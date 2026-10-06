package controllers

import (
	"bytes"
	"compress/gzip"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite" // pure-Go sqlite driver (no cgo — keeps cross-builds working)
)

// SQLite storage, replacing the per-demo JSON file tree. Design notes in
// docs/sqlite-refactor-plan.md:
//
//   - frames are stored once per round as a gzip blob of the exact round
//     JSON the old <round>.json files held — reads are byte-compatible with
//     the old files and the client needs no changes
//   - winner + economy + buys live in real columns so summaries, JEV and
//     the coach context never have to decompress a blob
//   - kills also get their own table (cheap to fill at parse time, unlocks
//     cross-demo queries later)
//   - derived analyses (JEV, post-plant) are cached in `caches` and
//     cascade away on re-parse or delete
//   - the .dem uploads stay on the filesystem as the recovery source —
//     any demo can be re-parsed from scratch if the db is ever lost

// Store wraps the sqlite database holding every parsed demo.
type Store struct {
	db *sql.DB
}

// db is the process-wide store, opened by Init (after SetDataDir had its
// chance to relocate the data directory). All controllers write through it;
// during the json->sqlite migration the JSON files are dual-written so the
// (still file-based) read paths keep working.
var db *Store

// DemoRow is one entry of the demo listing.
type DemoRow struct {
	Id     string
	Name   string
	Status string
	Error  string
}

// RoundMeta is the per-round metadata a summary or analysis needs — winner
// and the economy snapshot, without the frame blob.
type RoundMeta struct {
	Round   int
	Winner  string
	Economy *RoundEconomy
}

// Cache kinds stored in the caches table.
const (
	CacheAnalysis  = "analysis"
	CachePostPlant = "postplant"
)

// openStoreTimeout is how long sqlite waits for a lock before failing
// (WAL allows readers alongside the writer, but concurrent writers —
// PARSE_CONCURRENCY > 1 — need this).
const openStoreTimeoutMs = 5000

// OpenStore opens (creating if needed) the database at path and brings the
// schema to the current version. The pragmas ride along on the DSN so every
// pooled connection gets them (busy_timeout and foreign_keys are
// per-connection).
func OpenStore(path string) (*Store, error) {
	dsn := "file:" + filepath.ToSlash(path) +
		"?_pragma=busy_timeout(" + strconv.Itoa(openStoreTimeoutMs) + ")" +
		"&_pragma=journal_mode(WAL)" +
		"&_pragma=foreign_keys(1)" +
		"&_pragma=synchronous(NORMAL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	// A handful of connections is plenty: WAL gives concurrent readers,
	// writes are serialized by sqlite itself (one writer at a time).
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(4)

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

// Close closes the underlying database.
func (s *Store) Close() error {
	return s.db.Close()
}

// migrate brings the schema to user_version 1. Higher versions (from a
// newer binary) are refused rather than silently downgraded.
func (s *Store) migrate() error {
	var version int
	if err := s.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}
	if version > 1 {
		return fmt.Errorf("database schema v%d is newer than this build supports (v1)", version)
	}
	if version == 1 {
		return nil
	}
	const schema = `
CREATE TABLE demos (
  id               TEXT PRIMARY KEY, -- unix-nano timestamp, or "local" for -parse
  name             TEXT NOT NULL,    -- original .dem filename
  map              TEXT,
  frame_rate       INTEGER,
  round_count      INTEGER,
  game_start_frame INTEGER,
  game_end_frame   INTEGER,
  players_json     TEXT,             -- roster, as output.json holds it
  status           TEXT NOT NULL,    -- parsing | done | error
  error            TEXT,
  created_at       INTEGER NOT NULL
);

CREATE TABLE rounds (
  demo_id      TEXT NOT NULL REFERENCES demos(id) ON DELETE CASCADE,
  round        INTEGER NOT NULL,
  winner       TEXT,
  ct_buy       TEXT, -- heuristic buy type: pistol/eco/force/hero/kept/half/full
  t_buy        TEXT,
  economy_json TEXT, -- full RoundEconomy snapshot
  frames_gz    BLOB NOT NULL, -- gzip of the round JSON (exact old <round>.json bytes)
  PRIMARY KEY (demo_id, round)
);

CREATE TABLE kills (
  demo_id          TEXT NOT NULL REFERENCES demos(id) ON DELETE CASCADE,
  round            INTEGER NOT NULL,
  frame            INTEGER NOT NULL,
  time             REAL NOT NULL,
  attacker_steam_id TEXT,
  victim_steam_id   TEXT,
  assister_name     TEXT,
  weapon            TEXT,
  headshot          INTEGER NOT NULL DEFAULT 0,
  pos_x             REAL,
  pos_y             REAL
);
CREATE INDEX idx_kills_victim ON kills(victim_steam_id, demo_id);
CREATE INDEX idx_kills_attacker ON kills(attacker_steam_id, demo_id);

CREATE TABLE caches (
  demo_id TEXT NOT NULL REFERENCES demos(id) ON DELETE CASCADE,
  kind    TEXT NOT NULL, -- 'analysis' | 'postplant'
  data    BLOB NOT NULL, -- gzip(json)
  PRIMARY KEY (demo_id, kind)
);
`
	if _, err := s.db.Exec(schema); err != nil {
		return fmt.Errorf("create schema: %w", err)
	}
	if _, err := s.db.Exec("PRAGMA user_version = 1"); err != nil {
		return fmt.Errorf("set schema version: %w", err)
	}
	return nil
}

// gzipJSON gzips data at the parse-time level (good ratio, still fast —
// see store_test.go benchmarks).
func gzipJSON(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	w, err := gzip.NewWriterLevel(&buf, gzip.DefaultCompression)
	if err != nil {
		return nil, err
	}
	if _, err := w.Write(data); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// gunzipJSON reverses gzipJSON.
func gunzipJSON(data []byte) ([]byte, error) {
	r, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}

// --- demos -----------------------------------------------------------------

// CreateDemo records a new demo in the "parsing" state.
func (s *Store) CreateDemo(id, name string) error {
	_, err := s.db.Exec(
		`INSERT INTO demos (id, name, status, created_at) VALUES (?, ?, 'parsing', ?)`,
		id, name, time.Now().UnixNano(),
	)
	return err
}

// FinishDemo stores the game-level metadata (output.json material) and marks
// the demo done. Rounds must already have been inserted by InsertRounds.
func (s *Store) FinishDemo(id string, g Game) error {
	players, err := json.Marshal(g.Players)
	if err != nil {
		return err
	}
	res, err := s.db.Exec(
		`UPDATE demos SET map = ?, frame_rate = ?, round_count = ?,
		 game_start_frame = ?, game_end_frame = ?, players_json = ?,
		 status = 'done', error = NULL
		 WHERE id = ?`,
		g.Map, g.FrameRate, g.RoundCount, g.GameStartFrame, g.GameEndFrame,
		string(players), id,
	)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("demo %q not found", id)
	}
	return nil
}

// FailDemo marks a demo errored.
func (s *Store) FailDemo(id, msg string) error {
	_, err := s.db.Exec(`UPDATE demos SET status = 'error', error = ? WHERE id = ?`, msg, id)
	return err
}

// SweepInterrupted flips "parsing" rows left behind by a crash to "error".
// Runs on startup, before anything is served.
func (s *Store) SweepInterrupted() error {
	_, err := s.db.Exec(
		`UPDATE demos SET status = 'error',
		 error = 'parsing interrupted — re-parse the demo to recover'
		 WHERE status = 'parsing'`,
	)
	return err
}

// ListDemos returns all demos, newest id first (ids are unix-nano).
func (s *Store) ListDemos() ([]DemoRow, error) {
	rows, err := s.db.Query(`SELECT id, name, status, COALESCE(error, '') FROM demos ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var demos []DemoRow
	for rows.Next() {
		var d DemoRow
		if err := rows.Scan(&d.Id, &d.Name, &d.Status, &d.Error); err != nil {
			return nil, err
		}
		demos = append(demos, d)
	}
	return demos, rows.Err()
}

// DemoExists reports whether a demo id is in the database.
func (s *Store) DemoExists(id string) (bool, error) {
	var one int
	err := s.db.QueryRow(`SELECT 1 FROM demos WHERE id = ?`, id).Scan(&one)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// DeleteDemo removes a demo and everything derived from it (rounds, kills,
// caches cascade). Returns false if the demo was unknown.
func (s *Store) DeleteDemo(id string) (bool, error) {
	res, err := s.db.Exec(`DELETE FROM demos WHERE id = ?`, id)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// ClearDemoRounds drops a demo's rounds, kills and cached analyses — the
// storage-side of a re-parse, which then re-inserts fresh data.
func (s *Store) ClearDemoRounds(id string) error {
	if _, err := s.db.Exec(`DELETE FROM rounds WHERE demo_id = ?`, id); err != nil {
		return err
	}
	if _, err := s.db.Exec(`DELETE FROM kills WHERE demo_id = ?`, id); err != nil {
		return err
	}
	if _, err := s.db.Exec(`DELETE FROM caches WHERE demo_id = ?`, id); err != nil {
		return err
	}
	return nil
}

// GetDemoOutput reassembles the output.json document from the demos row —
// byte-compatible with the old file (same Game struct marshal, Rounds nil).
func (s *Store) GetDemoOutput(id string) ([]byte, bool, error) {
	var (
		mapName                      string
		frameRate, roundCount        int
		gameStartFrame, gameEndFrame int
		playersJson                  []byte
	)
	err := s.db.QueryRow(
		`SELECT map, frame_rate, round_count, game_start_frame, game_end_frame, players_json
		 FROM demos WHERE id = ? AND status = 'done'`, id,
	).Scan(&mapName, &frameRate, &roundCount, &gameStartFrame, &gameEndFrame, &playersJson)
	if err == sql.ErrNoRows {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	var players []Player
	if err := json.Unmarshal(playersJson, &players); err != nil {
		return nil, false, fmt.Errorf("bad players_json for %q: %w", id, err)
	}
	g := Game{
		Players:        players,
		Map:            mapName,
		GameStartFrame: gameStartFrame,
		GameEndFrame:   gameEndFrame,
		FrameRate:      frameRate,
		Rounds:         nil, // as the old output.json (frames live per-round)
		RoundCount:     roundCount,
	}
	out, err := json.Marshal(g)
	if err != nil {
		return nil, false, err
	}
	return out, true, nil
}

// --- rounds ----------------------------------------------------------------

// InsertRounds stores every round of a demo in one transaction, filling the
// kills table on the way. The frame blob is the exact JSON the old per-round
// files held, gzipped.
func (s *Store) InsertRounds(demoId string, rounds []Round) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	roundStmt, err := tx.Prepare(
		`INSERT INTO rounds (demo_id, round, winner, ct_buy, t_buy, economy_json, frames_gz)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer roundStmt.Close()
	killStmt, err := tx.Prepare(
		`INSERT INTO kills (demo_id, round, frame, time, attacker_steam_id,
		 victim_steam_id, assister_name, weapon, headshot, pos_x, pos_y)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer killStmt.Close()

	for _, r := range rounds {
		roundJson, err := json.Marshal(r)
		if err != nil {
			return fmt.Errorf("marshal round %d: %w", derefInt(r.Round), err)
		}
		framesGz, err := gzipJSON(roundJson)
		if err != nil {
			return fmt.Errorf("gzip round %d: %w", derefInt(r.Round), err)
		}
		var economyJson any // nil -> NULL
		var ctBuy, tBuy any
		if r.Economy != nil {
			if r.Economy.CT != nil {
				ctBuy = r.Economy.CT.Type
			}
			if r.Economy.T != nil {
				tBuy = r.Economy.T.Type
			}
			data, err := json.Marshal(r.Economy)
			if err != nil {
				return fmt.Errorf("marshal economy of round %d: %w", derefInt(r.Round), err)
			}
			economyJson = string(data)
		}
		roundNum := derefInt(r.Round)
		if _, err := roundStmt.Exec(demoId, roundNum, r.Winner, ctBuy, tBuy, economyJson, framesGz); err != nil {
			return err
		}
		for _, k := range r.Kills {
			if _, err := killStmt.Exec(demoId, roundNum, k.Frame, k.Time,
				k.AttackerSteamId, k.VictimSteamId, k.Assister, k.Weapon,
				k.Headshot, k.Position.X, k.Position.Y); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

// derefInt is r.Round without the nil dance.
func derefInt(p *int) int {
	if p != nil {
		return *p
	}
	return -1
}

// GetRoundBlob returns the gzipped round JSON as stored (for direct HTTP
// passthrough with Content-Encoding: gzip).
func (s *Store) GetRoundBlob(demoId string, round int) ([]byte, bool, error) {
	var blob []byte
	err := s.db.QueryRow(`SELECT frames_gz FROM rounds WHERE demo_id = ? AND round = ?`,
		demoId, round).Scan(&blob)
	if err == sql.ErrNoRows {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return blob, true, nil
}

// GetRoundJSON returns the decompressed round JSON — same bytes the old
// <round>.json file held.
func (s *Store) GetRoundJSON(demoId string, round int) ([]byte, bool, error) {
	blob, ok, err := s.GetRoundBlob(demoId, round)
	if err != nil || !ok {
		return nil, ok, err
	}
	data, err := gunzipJSON(blob)
	if err != nil {
		return nil, false, fmt.Errorf("gunzip round %d of %q: %w", round, demoId, err)
	}
	return data, true, nil
}

// GetRoundMetas returns per-round winner + economy for all rounds, in
// order — what summaries, JEV and the coach context need, without touching
// the frame blobs.
func (s *Store) GetRoundMetas(demoId string) ([]RoundMeta, error) {
	rows, err := s.db.Query(
		`SELECT round, COALESCE(winner, ''), economy_json FROM rounds
		 WHERE demo_id = ? ORDER BY round`, demoId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var metas []RoundMeta
	for rows.Next() {
		var m RoundMeta
		var economyJson []byte
		if err := rows.Scan(&m.Round, &m.Winner, &economyJson); err != nil {
			return nil, err
		}
		if len(economyJson) > 0 {
			if err := json.Unmarshal(economyJson, &m.Economy); err != nil {
				return nil, fmt.Errorf("bad economy_json for %q round %d: %w", demoId, m.Round, err)
			}
		}
		metas = append(metas, m)
	}
	return metas, rows.Err()
}

// GetRoundSummaries builds the /rounds response from the round columns —
// the old version read every 30MB round file for this.
func (s *Store) GetRoundSummaries(demoId string) ([]RoundSummary, error) {
	metas, err := s.GetRoundMetas(demoId)
	if err != nil {
		return nil, err
	}
	var summaries []RoundSummary
	for _, m := range metas {
		summary := RoundSummary{Round: m.Round, Winner: m.Winner}
		if m.Economy != nil {
			if m.Economy.CT != nil {
				summary.CTBuy = m.Economy.CT.Type
				for _, p := range m.Economy.CT.Players {
					summary.CTSteamIds = append(summary.CTSteamIds, p.SteamId)
				}
			}
			if m.Economy.T != nil {
				summary.TBuy = m.Economy.T.Type
				for _, p := range m.Economy.T.Players {
					summary.TSteamIds = append(summary.TSteamIds, p.SteamId)
				}
			}
		}
		summaries = append(summaries, summary)
	}
	return summaries, nil
}

// --- caches ----------------------------------------------------------------

// GetCache returns a cached analysis document, decompressed.
func (s *Store) GetCache(demoId, kind string) ([]byte, bool, error) {
	var blob []byte
	err := s.db.QueryRow(`SELECT data FROM caches WHERE demo_id = ? AND kind = ?`,
		demoId, kind).Scan(&blob)
	if err == sql.ErrNoRows {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	data, err := gunzipJSON(blob)
	if err != nil {
		return nil, false, fmt.Errorf("gunzip %s cache of %q: %w", kind, demoId, err)
	}
	return data, true, nil
}

// SetCache stores a cached analysis document (overwrites).
func (s *Store) SetCache(demoId, kind string, data []byte) error {
	blob, err := gzipJSON(data)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(
		`INSERT INTO caches (demo_id, kind, data) VALUES (?, ?, ?)
		 ON CONFLICT (demo_id, kind) DO UPDATE SET data = excluded.data`,
		demoId, kind, blob)
	return err
}

// --- legacy import ----------------------------------------------------------

// ImportLegacyOutput imports the old per-demo output/<id>/ directory tree
// into the database (one-shot migration at startup). Demos already in the
// database are skipped, so it is safe to run repeatedly. Round blobs are
// the gzipped bytes of the old files — no re-parse, byte-identical.
// Returns how many demos were imported.
func (s *Store) ImportLegacyOutput(outputDir string) (int, error) {
	entries, err := os.ReadDir(outputDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	imported := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		id := entry.Name()
		if exists, err := s.DemoExists(id); err != nil {
			return imported, err
		} else if exists {
			continue
		}
		if err := s.importLegacyDemo(filepath.Join(outputDir, id), id); err != nil {
			return imported, fmt.Errorf("import %q: %w", id, err)
		}
		imported++
	}
	return imported, nil
}

// importLegacyDemo imports one legacy demo directory.
func (s *Store) importLegacyDemo(dir, id string) error {
	name := id
	if data, err := os.ReadFile(filepath.Join(dir, "meta.json")); err == nil {
		var meta demoMeta
		if json.Unmarshal(data, &meta) == nil && meta.Name != "" {
			name = meta.Name
		}
	}
	if err := s.CreateDemo(id, name); err != nil {
		return err
	}

	// output.json is the marker for a completed parse, same as before.
	outData, err := os.ReadFile(filepath.Join(dir, "output.json"))
	if err != nil {
		return s.FailDemo(id, "parsing incomplete (output.json missing)")
	}
	var g Game
	if err := json.Unmarshal(outData, &g); err != nil {
		return s.FailDemo(id, "bad output.json: "+err.Error())
	}

	// Round files -> rounds + kills. The blob is the raw file gzipped so
	// the imported rounds are byte-identical to a fresh parse.
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	var rounds []Round
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".json") {
			continue
		}
		num, err := strconv.Atoi(strings.TrimSuffix(name, ".json"))
		if err != nil {
			continue // output.json, meta.json, analysis.json, postplant.json…
		}
		raw, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return err
		}
		var r Round
		if err := json.Unmarshal(raw, &r); err != nil {
			return fmt.Errorf("bad round file %s: %w", name, err)
		}
		if r.Round == nil {
			r.Round = &num
		}
		rounds = append(rounds, r)
	}
	sort.Slice(rounds, func(i, j int) bool { return derefInt(rounds[i].Round) < derefInt(rounds[j].Round) })
	if err := s.InsertRounds(id, rounds); err != nil {
		return err
	}
	if err := s.FinishDemo(id, g); err != nil {
		return err
	}

	// Cached analyses carry over so JEV is not re-billed for old demos.
	for kind, file := range map[string]string{
		CacheAnalysis:  "analysis.json",
		CachePostPlant: "postplant.json",
	} {
		if data, err := os.ReadFile(filepath.Join(dir, file)); err == nil {
			if err := s.SetCache(id, kind, data); err != nil {
				return err
			}
		}
	}
	return nil
}
