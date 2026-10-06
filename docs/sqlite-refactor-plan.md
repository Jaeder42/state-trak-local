# Refactor plan: JSON files → SQLite

## Why

Current storage is one directory per demo full of hand-rolled JSON files
(`meta.json`, `output.json`, `<round>.json`, `analysis.json`, `postplant.json`).
That works, but:

- **Size** — one 266MB demo parses to **1.1GB of JSON**. Round files are
  17–30MB each, served raw over HTTP.
- **Redundant reads** — `GET /demos/:id/rounds` reads *every full round file*
  just to summarize winner + buys (measured: **1.29s** for 17 rounds on
  `local`). Same for post-plant and JEV (they re-read full rounds from disk).
- **Fragile persistence** — the demo list survives restarts by directory
  scanning + `meta.json`/`output.json` presence heuristics; interrupted
  parses are only detected by a missing `output.json`.
- **Delete** is `os.RemoveAll` of a directory tree; no integrity story.
- **No queryability** — anything cross-round/cross-demo means re-parsing
  JSON blobs.

Measured facts that drive the design (from `test.dem` / demo `local`):

| Fact | Value |
| --- | --- |
| JSON per demo | ~1.1 GB |
| Round file | ~17–30 MB, 4.8k–6.5k frames × 10 player states |
| gzip of one round file | 17.7 MB → **0.66 MB (~27:1)** |
| Reads per round | whole-file only — no per-frame queries anywhere |
| Client | HTTP API only (`/demos/:id/:round` etc.) — zero filesystem coupling |
| Builds | portable binaries are pure Go, no cgo; Wails desktop shares the router |

## Goals

1. One SQLite database file per data dir replacing all JSON output.
2. API responses **byte-identical** — client needs zero changes.
3. Cheap round summaries (`/rounds` from columns, not 30MB blobs).
4. Shrinks stored size ~25× (gzip) and per-round HTTP transfer ~27× (if we
   serve the stored gzip directly).
5. Honest persistence: demo rows with status/error, no directory heuristics.
6. Path for future cross-demo queries (kills table).

## Non-goals

- No client changes, no API shape changes.
- No per-frame SQL rows — the access pattern is whole-round reads; normalizing
  ~1.1M player-frame rows per demo would balloon size and slow reads for
  nothing we query today. Revisit only if we ever need positional queries.
- Uploads (`.dem`) stay on the filesystem — they're the recovery source;
  stuffing 266MB blobs into the DB bloats it for no query value.

## Key decisions

### 1. `modernc.org/sqlite` (pure Go), not `mattn/go-sqlite3`

The portable binaries advertise "no cgo, cross-compile anywhere" and the
Makefile cross-builds 4 platforms from one host — a cgo dep breaks that and
the Wails builds. `modernc.org/sqlite` is slower than CGo SQLite on heavy
queries, but our workload is blob writes/reads + tiny metadata queries, well
within its league. Cost: ~10–15MB binary growth and a new dep family in
`THIRD_PARTY_NOTICES.md` (must be regenerated — see Risks).

### 2. Hybrid schema: normalized metadata + compressed round blobs

Access pattern is strictly "give me round N of demo X, whole". So:

- `rounds.frames_gz` = **gzip of the exact current round JSON** (the
  `Round` struct marshaled as today). Byte-compat with the client for free,
  and the blob doubles as the HTTP payload (see decision 4).
- Winner + buy classification + economy live in real columns so
  `/rounds` summaries, JEV and coach never touch the blobs.
- Kills also in a proper table — near-free to populate during parse,
  unlocks "query my deaths across demos" later.

### 3. Single shared DB file: `<dataDir>/statetrak.db`

Not one DB per demo: the demo *list* is a query, deletes are one
transaction, and `-data` keeps working (`SetDataDir` already runs before
`Init()`). WAL mode: concurrent readers with the one writer (the parse
goroutine); set `busy_timeout` for the `PARSE_CONCURRENCY > 1` case.

### 4. Optional gzip passthrough on `/demos/:id/:round`

Serve `frames_gz` with `Content-Encoding: gzip` — browsers and both desktop
webviews (WKWebView/WebView2) transparently decompress `fetch()` responses.
Turns a 30MB response into 0.7MB with zero marshal cost. Phase 6, behind a
fall-back (plain gunzip serve) if any target misbehaves.

## Schema

```sql
PRAGMA journal_mode = WAL;
PRAGMA synchronous  = NORMAL;   -- crash-safe enough: uploads can always re-parse
PRAGMA foreign_keys = ON;
PRAGMA user_version = 1;         -- schema version for future migrations

CREATE TABLE demos (
  id            TEXT PRIMARY KEY,   -- unix-nano, or "local" for -parse
  name          TEXT NOT NULL,      -- original filename
  map           TEXT,
  frame_rate    INTEGER,
  round_count   INTEGER,
  game_start_frame INTEGER,
  game_end_frame  INTEGER,
  players_json  TEXT,               -- roster (small, output.json material)
  status        TEXT NOT NULL,      -- parsing | done | error
  error         TEXT,
  created_at    INTEGER NOT NULL
);

CREATE TABLE rounds (
  demo_id      TEXT NOT NULL REFERENCES demos(id) ON DELETE CASCADE,
  round        INTEGER NOT NULL,
  winner       TEXT,
  ct_buy       TEXT,               -- economy classification (pistol/eco/...)
  t_buy        TEXT,
  economy_json TEXT,               -- full RoundEconomy snapshot
  frames_gz    BLOB NOT NULL,      -- gzip(json of Round) — exact current shape
  PRIMARY KEY (demo_id, round)
);

CREATE TABLE kills (
  demo_id  TEXT NOT NULL REFERENCES demos(id) ON DELETE CASCADE,
  round    INTEGER NOT NULL,
  frame    INTEGER NOT NULL,
  time     REAL NOT NULL,
  attacker_steam_id TEXT, victim_steam_id TEXT,
  assister_name     TEXT,
  weapon  TEXT, headshot INTEGER NOT NULL DEFAULT 0,
  pos_x   REAL, pos_y   REAL
);
CREATE INDEX idx_kills_victim ON kills(victim_steam_id, demo_id);
CREATE INDEX idx_kills_attacker ON kills(attacker_steam_id, demo_id);

CREATE TABLE caches (
  demo_id TEXT NOT NULL REFERENCES demos(id) ON DELETE CASCADE,
  kind    TEXT NOT NULL,          -- 'analysis' | 'postplant'
  data    BLOB NOT NULL,          -- gzip(json)
  PRIMARY KEY (demo_id, kind)
);
```

Why duplicated data is fine: `winner`/`economy_json` inside `frames_gz` AND
in columns costs ~2KB per round; summaries stop costing 30MB reads.

## Touchpoint mapping (every current file → store call)

New file `controllers/store.go` (package `controllers`, no new package —
avoids an import cycle dance and keeps the diff small).

| Today (file) | Becomes |
| --- | --- |
| `output/<id>/meta.json` | `demos.name` column |
| `output.json` presence check (loadPersistedDemos) | `SELECT status FROM demos`; startup flips crashed `parsing` rows → `error` ("interrupted, reparse available") |
| `output.json` write (ParseDemo) | `UPDATE demos SET ...` after rounds are inserted |
| `<round>.json` write (ParseDemo loop) | one `INSERT` per round inside one tx per demo (frames_gz = gzip of the same marshal) |
| `GetRound` → `c.File` | `SELECT frames_gz` → serve (plain, or gzip passthrough in phase 6) |
| `loadRoundSummaries` (reads every full round!) | `SELECT round, winner, ct_buy, t_buy, economy_json FROM rounds ORDER BY round` — ~ms instead of ~1.3s |
| `analysis.json` (jev.go) | `caches (kind='analysis')`; JEV reads round economy from `rounds.economy_json`, never blobs |
| `postplant.json` (postplant.go) | `caches (kind='postplant')`; the analysis itself reads `frames_gz` blobs + decompresses |
| `output.json` reads (jev.go, coach.go) | `SELECT` demo row → assemble the same JSON |
| `ListDemos` ReadDir scan | `SELECT id, name, status FROM demos ORDER BY id DESC` |
| `DeleteDemo` RemoveAll | `DELETE FROM demos WHERE id=?` (CASCADE) + remove `uploads/<id>.dem` |
| re-parse (`-reparse`, fresh upload) | `DELETE FROM rounds/kills/caches WHERE demo_id=?` then insert — replaces the "remove analysis.json/postplant.json" dance |
| in-memory `demoStatus` map | stays (progress ticks per frame are too hot for the DB); only status *transitions* persist |

## Migration (existing demos keep working)

One-shot importer at the end of `Init()`:

1. Scan legacy `output/` for demo dirs not present in the DB.
2. For each: insert `demos` row (name from `meta.json`, status `done`),
   each `<n>.json` → `rounds` row with `frames_gz = gzip(raw file bytes)`
   (byte-identical, no re-marshal), winner/economy parsed for columns;
   `analysis.json`/`postplant.json` → `caches` if present.
3. Legacy files are left in place untouched as the backup — listing serves
   from the DB only. A `-drop-legacy` flag (or just manual deletion) removes
   them later once the DB has proven itself.

~1.1GB of gzip ≈ 10s, once. `-reparse=<id>` remains the sledgehammer
fallback for anything odd (uploads are retained).

## Implementation phases

Each phase leaves the app green and is one commit.

**Phase 1 — store layer** (`controllers/store.go`, `store_test.go`)
Open/init with pragmas, schema v1, CRUD methods, all behind a small
interface. First real Go tests in the repo: temp-DB CRUD, delete-cascade,
migration importer against fixture JSON, WAL concurrent read-during-write
smoke. No callers yet.

**Phase 2 — write path** (game.go, demo.go, main.go) — **DONE**
`ParseDemo` inserts rounds/rows via one tx; upload flow inserts the
`demos` row (status `parsing`) and finalizes to `done`/`error`; delete goes
through the DB. Keep writing JSON files in parallel (dual-write) during this
phase so the old read path stays intact — flip flag-guarded.

Verified: dual-write byte-identical (blob == round file), upload/delete/
re-parse flows against the real test.dem, cross-builds still cgo-free
(windows/linux).

**Phase 3 — read path** (round.go, jev.go, postplant.go, coach.go) — **DONE**
All readers switch to the store; `/rounds` summaries from columns.
Byte-diff harness: run old and new build against the same demo, `diff`
every endpoint's response (they must match exactly).

The legacy importer (was phase 5) was wired into Init here — without it,
file-only demos would 404 between phases 3 and 4, breaking the
each-phase-must-be-green rule. It ran on first boot and imported the
pre-existing ancient demo (~20s, one-time). Dual-write stays on until
phase 4 flips listing/status, so every commit remains green.

Verified byte-identical (old build vs store build): /ping, /demos,
/demos/:id/status, /output, /rounds, three 30MB round payloads,
/postplant, and every error path (404s, coach 404) — plus the *imported*
legacy demo (importer serves the raw file bytes, no re-marshal).
`/rounds` latency: **1.35s → 11ms**.

**Phase 4 — status & listing** (demo.go, main.go)
`ListDemos`/`loadPersistedDemos` equivalents from the DB; startup
`parsing`→`error` sweep; remove `meta.json` machinery. Update `-parse`
help text ("writes to the database").

**Phase 5 — migration importer** — **DONE, absorbed into phase 3**
Imported on first boot against the pre-existing demos (including the
roster-less ancient demo). Remaining decision: when to drop the legacy
`output/` trees (a `-drop-legacy` flag or manual deletion once the store
has proven itself).

**Phase 6 — gzip passthrough + docs** — `Content-Encoding: gzip` on
`/demos/:id/:round` (plain-gunzip fallback kept behind a check), readme +
AGENTS.md storage sections, regenerate `THIRD_PARTY_NOTICES.md`, release
as `v0.0.3`.

Verification at every phase: `gofmt -l . && go vet ./...`, `make`, parse
`test.dem` (17 rounds), full curl sweep from AGENTS.md, restart
persistence, delete, `make desktop` (webview + WAL in-process sanity),
`make release` (pure-Go cross-build proof).

## Risks & mitigations

- **modernc.org/sqlite binary size** (+10–15MB) — acceptable; notices file
  must be regenerated (it's hand/`go-licenses`-generated, 78 entries today;
  modernc pulls ~8 more modules). Do it in phase 1, not at the end.
- **Concurrency** — `database/sql` is goroutine-safe; one writer (parse,
  capped by `parseSem`) + many readers under WAL. Set `busy_timeout=5000`.
  Keep `SetMaxOpenConns` modest (e.g. 4) — modernc connections are cheap but
  not free.
- **DB corruption / single file of truth** — uploads are never deleted, so
  any demo can be re-parsed from source; worst case delete `statetrak.db`
  and re-parse the library (~8s/demo). Consider printing a hint to that
  effect on startup errors.
- **WAL sidecar files** (`-wal`/`-shm`) — fine, but `.gitignore` hygiene and
  `make clean` should not touch them; document that copying a live DB needs
  a checkpoint (or just stop the app first).
- **gzip passthrough webviews** — WKWebView and WebView2 both honor
  `Content-Encoding` on XHR/fetch, but this is the one phase with
  per-platform risk → ship behind a runtime check with plain-serve fallback.
- **`local` demo id** — `-parse` writes id `local`; importer must treat a
  pre-existing `output/local` dir + a `local` DB row as one demo (overwrite,
  not duplicate).

## Open questions (decide before phase 1)

1. Keep the legacy dual-write phase 2 at all, or go straight read+write
   in one phase? (Dual-write is safer for a public app; adds a throwaway flag.)
2. gzip level: default (fast, ~27:1 measured with python defaults) vs
   `gzip.BestCompression` at parse time (parse is 8s anyway; check the real
   numbers when implementing). — resolved: Go default level, 24.5:1 at
   45ms per round (~3s per demo).
3. Trim `kills` table columns (I kept names out — steam ids only) or keep
   full name strings for human-readable offline queries?
4. Notices regeneration: confirm how the current file was produced
   (`go-licenses save`?) so the new entries match its format exactly. —
   resolved: entries are hand-assembled "module (version) + verbatim
   license" blocks; phase 1 appended the 7 newly-linked modules in the
   same format (incl. libc/sqlite third-party notices verbatim).