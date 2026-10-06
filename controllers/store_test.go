package controllers

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"
)

// openTestStore gives every test a fresh database in a temp dir.
func openTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := OpenStore(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// fixtureRound builds a round with a couple of frames, a kill and economy —
// enough shape to verify the blob path and the derived columns.
func fixtureRound(num int) Round {
	r := num
	two := 2
	player := PlayerState{
		Name:     "alice",
		SteamId:  "76561198000000001",
		Position: Vector{X: -332.5, Y: -754.25},
		Yaw:      -147.5,
		Team:     "CT",
		Firing:   true,
		Alive:    true,
		Weapon:   "M4A1-S",
		Health:   100,
	}
	return Round{
		Round:  &r,
		Winner: "CT",
		Frames: []FrameState{
			{Frame: 100, CTScore: 3, TScore: two, Time: 95.5, PlayerStates: []PlayerState{player}},
			{Frame: 101, CTScore: 3, TScore: two, Time: 95.6, PlayerStates: []PlayerState{player}},
		},
		Kills: []KillEvent{
			{
				Frame: 101, Time: 95.6, Attacker: "alice",
				AttackerSteamId: "76561198000000001", AttackerTeam: "CT",
				Victim: "bob", VictimSteamId: "76561198000000002", VictimTeam: "T",
				Weapon: "M4A1-S", Headshot: true, Position: Vector{X: 10, Y: -20},
			},
		},
		Economy: &RoundEconomy{
			CT: &TeamEconomy{
				Players: []PlayerEconomy{
					{Name: "alice", SteamId: "76561198000000001", StartMoney: 800, Bank: 500, Spent: 300, EquipValue: 1000, Weapon: "", Armor: 50, Helmet: false, DefuseKit: false},
				},
				AvgEquip: 1000, AvgSpent: 300, TotalSpent: 300, Type: "eco", Rifles: 0, Survivors: 1,
			},
			T: &TeamEconomy{
				Players: []PlayerEconomy{
					{Name: "bob", SteamId: "76561198000000002", StartMoney: 800, Bank: 400, Spent: 400, EquipValue: 900, Weapon: "Glock-18", Armor: 0, Helmet: false, DefuseKit: false},
				},
				AvgEquip: 900, AvgSpent: 400, TotalSpent: 400, Type: "eco", Rifles: 0, Survivors: 0,
			},
		},
	}
}

func fixtureGame(rounds int) Game {
	name := "alice"
	steam := "76561198000000001"
	return Game{
		Players:        []Player{{Name: &name, SteamID: &steam}},
		Map:            "de_dust2",
		GameStartFrame: 7202,
		GameEndFrame:   250000,
		FrameRate:      67,
		Rounds:         nil,
		RoundCount:     rounds,
	}
}

// seedDemo inserts a done demo with the given rounds, like ParseDemo would.
func seedDemo(t *testing.T, s *Store, id string, rounds []Round) {
	t.Helper()
	if err := s.CreateDemo(id, "test.dem"); err != nil {
		t.Fatalf("CreateDemo: %v", err)
	}
	if err := s.InsertRounds(id, rounds); err != nil {
		t.Fatalf("InsertRounds: %v", err)
	}
	if err := s.FinishDemo(id, fixtureGame(len(rounds))); err != nil {
		t.Fatalf("FinishDemo: %v", err)
	}
}

func TestStoreSchemaPragmas(t *testing.T) {
	s := openTestStore(t)
	var mode string
	if err := s.db.QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil {
		t.Fatalf("journal_mode: %v", err)
	}
	if mode != "wal" {
		t.Errorf("journal_mode = %q, want wal", mode)
	}
	var version int
	if err := s.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		t.Fatalf("user_version: %v", err)
	}
	if version != 1 {
		t.Errorf("user_version = %d, want 1", version)
	}
	// foreign_keys is per-connection — must be on via the DSN.
	var fk int
	if err := s.db.QueryRow("PRAGMA foreign_keys").Scan(&fk); err != nil {
		t.Fatalf("foreign_keys: %v", err)
	}
	if fk != 1 {
		t.Errorf("foreign_keys = %d, want 1", fk)
	}
}

func TestStoreReopenKeepsData(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")
	s, err := OpenStore(path)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	seedDemo(t, s, "1", []Round{fixtureRound(0)})
	s.Close()

	s2, err := OpenStore(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer s2.Close()
	demos, err := s2.ListDemos()
	if err != nil || len(demos) != 1 || demos[0].Id != "1" {
		t.Fatalf("reopen ListDemos = %+v, err %v", demos, err)
	}
	data, ok, err := s2.GetRoundJSON("1", 0)
	if err != nil || !ok {
		t.Fatalf("reopen GetRoundJSON ok=%v err=%v", ok, err)
	}
	var r Round
	if json.Unmarshal(data, &r) != nil || r.Winner != "CT" {
		t.Fatalf("reopened round data wrong: %s", data)
	}
}

func TestStoreRoundTrip(t *testing.T) {
	s := openTestStore(t)
	rounds := []Round{fixtureRound(0), fixtureRound(1)}
	rounds[1].Winner = "T" // second round differs
	seedDemo(t, s, "123", rounds)

	// Blob is valid gzip; JSON is byte-identical to marshaling the struct.
	blob, ok, err := s.GetRoundBlob("123", 0)
	if err != nil || !ok {
		t.Fatalf("GetRoundBlob ok=%v err=%v", ok, err)
	}
	zr, err := gzip.NewReader(bytes.NewReader(blob))
	if err != nil {
		t.Fatalf("blob not gzip: %v", err)
	}
	zr.Close()

	data, ok, err := s.GetRoundJSON("123", 0)
	if err != nil || !ok {
		t.Fatalf("GetRoundJSON ok=%v err=%v", ok, err)
	}
	want, _ := json.Marshal(rounds[0])
	if !bytes.Equal(data, want) {
		t.Errorf("round JSON differs from the old file format\n got: %s\nwant: %s", data, want)
	}
	// Structural round trip: what comes back parses into the same round.
	var got Round
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !reflect.DeepEqual(got, rounds[0]) {
		t.Errorf("round struct mismatch: %+v != %+v", got, rounds[0])
	}

	// Missing round / demo
	if _, ok, _ := s.GetRoundJSON("123", 5); ok {
		t.Error("round 5 should not exist")
	}
	if _, ok, _ := s.GetRoundJSON("nope", 0); ok {
		t.Error("unknown demo should not have rounds")
	}
}

func TestStoreSummaries(t *testing.T) {
	s := openTestStore(t)
	seedDemo(t, s, "123", []Round{fixtureRound(0), fixtureRound(1)})

	summaries, err := s.GetRoundSummaries("123")
	if err != nil {
		t.Fatalf("GetRoundSummaries: %v", err)
	}
	if len(summaries) != 2 {
		t.Fatalf("got %d summaries, want 2", len(summaries))
	}
	if summaries[0].Winner != "CT" || summaries[0].CTBuy != "eco" || summaries[0].TBuy != "eco" {
		t.Errorf("summary[0] = %+v", summaries[0])
	}
	if !reflect.DeepEqual(summaries[0].CTSteamIds, []string{"76561198000000001"}) {
		t.Errorf("CTSteamIds = %v", summaries[0].CTSteamIds)
	}
	if !reflect.DeepEqual(summaries[0].TSteamIds, []string{"76561198000000002"}) {
		t.Errorf("TSteamIds = %v", summaries[0].TSteamIds)
	}
	// Same JSON the old loadRoundSummaries produced.
	data, _ := json.Marshal(summaries)
	if !bytes.Contains(data, []byte(`"round":0,"winner":"CT","ctBuy":"eco"`)) {
		t.Errorf("summary JSON shape changed: %s", data)
	}

	// Metas carry the full economy for JEV.
	metas, err := s.GetRoundMetas("123")
	if err != nil || len(metas) != 2 {
		t.Fatalf("GetRoundMetas = %+v, err %v", metas, err)
	}
	if metas[0].Economy == nil || metas[0].Economy.CT.AvgSpent != 300 {
		t.Errorf("economy did not round-trip: %+v", metas[0].Economy)
	}
}

func TestStoreOutputJSON(t *testing.T) {
	s := openTestStore(t)
	seedDemo(t, s, "123", []Round{fixtureRound(0)})
	data, ok, err := s.GetDemoOutput("123")
	if err != nil || !ok {
		t.Fatalf("GetDemoOutput ok=%v err=%v", ok, err)
	}
	// Must be byte-identical to the old output.json marshal of the same game.
	want, _ := json.Marshal(fixtureGame(1))
	if !bytes.Equal(data, want) {
		t.Errorf("output.json differs\n got: %s\nwant: %s", data, want)
	}
	// Not-done and unknown demos report not-found.
	if _, ok, _ := s.GetDemoOutput("unknown"); ok {
		t.Error("unknown demo should not have output")
	}
	s2 := openTestStore(t)
	s2.CreateDemo("parsingdemo", "x.dem")
	if _, ok, _ := s2.GetDemoOutput("parsingdemo"); ok {
		t.Error("parsing demo should not have output")
	}
}

func TestStoreDeleteCascade(t *testing.T) {
	s := openTestStore(t)
	seedDemo(t, s, "123", []Round{fixtureRound(0)})
	s.SetCache("123", CachePostPlant, []byte(`{"rounds":[]}`))

	deleted, err := s.DeleteDemo("123")
	if err != nil || !deleted {
		t.Fatalf("DeleteDemo deleted=%v err=%v", deleted, err)
	}
	for _, table := range []string{"rounds", "kills", "caches"} {
		var n int
		if err := s.db.QueryRow("SELECT COUNT(*) FROM " + table + " WHERE demo_id = '123'").Scan(&n); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		if n != 0 {
			t.Errorf("%s rows survived the cascade: %d", table, n)
		}
	}
	if deleted, _ := s.DeleteDemo("123"); deleted {
		t.Error("second delete should find nothing")
	}
}

func TestStoreClearRoundsKeepsDemo(t *testing.T) {
	s := openTestStore(t)
	seedDemo(t, s, "123", []Round{fixtureRound(0)})
	s.SetCache("123", CacheAnalysis, []byte(`{"rounds":[]}`))

	if err := s.ClearDemoRounds("123"); err != nil {
		t.Fatalf("ClearDemoRounds: %v", err)
	}
	demos, _ := s.ListDemos()
	if len(demos) != 1 || demos[0].Status != "done" {
		t.Errorf("demo row should survive a re-parse clear: %+v", demos)
	}
	var n int
	s.db.QueryRow("SELECT COUNT(*) FROM rounds WHERE demo_id = '123'").Scan(&n)
	if n != 0 {
		t.Errorf("rounds survived clear: %d", n)
	}
	s.db.QueryRow("SELECT COUNT(*) FROM caches WHERE demo_id = '123'").Scan(&n)
	if n != 0 {
		t.Errorf("caches survived clear: %d", n)
	}
}

func TestStoreCaches(t *testing.T) {
	s := openTestStore(t)
	seedDemo(t, s, "123", []Round{fixtureRound(0)})

	if _, ok, _ := s.GetCache("123", CacheAnalysis); ok {
		t.Error("cache should be empty first")
	}
	if err := s.SetCache("123", CacheAnalysis, []byte(`{"model":"jev"}`)); err != nil {
		t.Fatalf("SetCache: %v", err)
	}
	data, ok, err := s.GetCache("123", CacheAnalysis)
	if err != nil || !ok || !bytes.Equal(data, []byte(`{"model":"jev"}`)) {
		t.Fatalf("GetCache = %s ok=%v err=%v", data, ok, err)
	}
	// Overwrite.
	if err := s.SetCache("123", CacheAnalysis, []byte(`{"model":"jev2"}`)); err != nil {
		t.Fatalf("SetCache overwrite: %v", err)
	}
	data, _, _ = s.GetCache("123", CacheAnalysis)
	if !bytes.Equal(data, []byte(`{"model":"jev2"}`)) {
		t.Errorf("cache overwrite failed: %s", data)
	}
}

func TestStoreStatusLifecycle(t *testing.T) {
	s := openTestStore(t)
	s.CreateDemo("123", "x.dem")
	if err := s.FailDemo("123", "boom"); err != nil {
		t.Fatalf("FailDemo: %v", err)
	}
	demos, _ := s.ListDemos()
	if demos[0].Status != "error" || demos[0].Error != "boom" {
		t.Errorf("failed demo = %+v", demos[0])
	}

	// Crash sweep: a stale "parsing" row flips to error.
	s.CreateDemo("456", "y.dem")
	if err := s.SweepInterrupted(); err != nil {
		t.Fatalf("SweepInterrupted: %v", err)
	}
	for _, d := range mustList(s, t) {
		if d.Id == "456" && d.Status != "error" {
			t.Errorf("interrupted demo not swept: %+v", d)
		}
	}
}

func mustList(s *Store, t *testing.T) []DemoRow {
	t.Helper()
	demos, err := s.ListDemos()
	if err != nil {
		t.Fatalf("ListDemos: %v", err)
	}
	return demos
}

func TestStoreListOrder(t *testing.T) {
	s := openTestStore(t)
	s.CreateDemo("100", "a.dem")
	s.CreateDemo("200", "b.dem")
	s.CreateDemo("300", "c.dem")
	demos := mustList(s, t)
	if demos[0].Id != "300" || demos[1].Id != "200" || demos[2].Id != "100" {
		t.Errorf("listing not newest-first: %+v", demos)
	}
}

// TestStoreConcurrentReadDuringWrite is the WAL smoke test: readers must
// keep working (on pre-commit data) while a big insert transaction runs.
func TestStoreConcurrentReadDuringWrite(t *testing.T) {
	s := openTestStore(t)
	s.CreateDemo("1", "writer.dem")
	seedDemo(t, s, "2", []Round{fixtureRound(0)})

	var wg sync.WaitGroup
	readErr := make(chan error, 50)
	stop := make(chan struct{})

	// Readers: list + summaries in a hot loop.
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				if _, err := s.ListDemos(); err != nil {
					readErr <- err
					return
				}
				if _, err := s.GetRoundSummaries("2"); err != nil {
					readErr <- err
					return
				}
			}
		}()
	}

	// Writer: 200 rounds in one transaction.
	wg.Add(1)
	go func() {
		defer wg.Done()
		rounds := make([]Round, 200)
		for i := range rounds {
			rounds[i] = fixtureRound(i)
		}
		if err := s.InsertRounds("1", rounds); err != nil {
			readErr <- err
		}
		close(stop)
	}()

	wg.Wait()
	close(readErr)
	for err := range readErr {
		t.Errorf("concurrent read/write error: %v", err)
	}

	summaries, err := s.GetRoundSummaries("1")
	if err != nil {
		t.Fatalf("post-commit summaries: %v", err)
	}
	if len(summaries) != 200 {
		t.Errorf("got %d rounds after commit, want 200", len(summaries))
	}
}

// TestStoreImportLegacyOutput builds the old directory layout in a temp
// dir and imports it, twice (idempotence).
func TestStoreImportLegacyOutput(t *testing.T) {
	s := openTestStore(t)
	legacy := t.TempDir()

	// Demo one: complete (meta + output + one round + both caches).
	dir := filepath.Join(legacy, "111")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dir, "meta.json"), []byte(`{"name":"match-one.dem"}`), 0644)
	outJson, _ := json.Marshal(fixtureGame(1))
	os.WriteFile(filepath.Join(dir, "output.json"), outJson, 0644)
	roundJson, _ := json.Marshal(fixtureRound(0))
	os.WriteFile(filepath.Join(dir, "0.json"), roundJson, 0644)
	os.WriteFile(filepath.Join(dir, "analysis.json"), []byte(`{"model":"jev"}`), 0644)
	os.WriteFile(filepath.Join(dir, "postplant.json"), []byte(`{"rounds":[]}`), 0644)

	// Demo two: incomplete (no output.json) -> error status, like before.
	dir2 := filepath.Join(legacy, "222")
	if err := os.MkdirAll(dir2, 0755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dir2, "meta.json"), []byte(`{"name":"match-two.dem"}`), 0644)

	n, err := s.ImportLegacyOutput(legacy)
	if err != nil {
		t.Fatalf("ImportLegacyOutput: %v", err)
	}
	if n != 2 {
		t.Fatalf("imported %d demos, want 2", n)
	}

	demos := mustList(s, t)
	if len(demos) != 2 {
		t.Fatalf("got %d demos, want 2", len(demos))
	}
	var one, two DemoRow
	for _, d := range demos {
		switch d.Id {
		case "111":
			one = d
		case "222":
			two = d
		}
	}
	if one.Name != "match-one.dem" || one.Status != "done" {
		t.Errorf("demo 111 = %+v", one)
	}
	if two.Status != "error" {
		t.Errorf("incomplete demo should be error, got %+v", two)
	}

	// Blob is the gzipped raw file — byte-identical, no re-marshal.
	data, ok, err := s.GetRoundJSON("111", 0)
	if err != nil || !ok {
		t.Fatalf("imported GetRoundJSON ok=%v err=%v", ok, err)
	}
	if !bytes.Equal(data, roundJson) {
		t.Error("imported round bytes differ from the source file")
	}

	// Metadata carried over.
	out, ok, _ := s.GetDemoOutput("111")
	if !ok || !bytes.Equal(out, outJson) {
		t.Errorf("imported output.json mismatch: %s", out)
	}

	// Caches carried over.
	if c, ok, _ := s.GetCache("111", CacheAnalysis); !ok || !bytes.Equal(c, []byte(`{"model":"jev"}`)) {
		t.Errorf("imported analysis cache = %s ok=%v", c, ok)
	}

	// Second run imports nothing and doesn't duplicate.
	n, err = s.ImportLegacyOutput(legacy)
	if err != nil {
		t.Fatalf("second ImportLegacyOutput: %v", err)
	}
	if n != 0 {
		t.Errorf("second run imported %d, want 0", n)
	}
	if demos := mustList(s, t); len(demos) != 2 {
		t.Errorf("demos duplicated: %d", len(demos))
	}

	// Missing dir is fine.
	if n, err := s.ImportLegacyOutput(filepath.Join(t.TempDir(), "nope")); err != nil || n != 0 {
		t.Errorf("missing legacy dir: n=%d err=%v", n, err)
	}
}

// TestStoreImportLegacySkipsExisting verifies a demo already in the db
// (e.g. "local" re-parsed before the import ran) is not re-imported.
func TestStoreImportLegacySkipsExisting(t *testing.T) {
	s := openTestStore(t)
	legacy := t.TempDir()
	dir := filepath.Join(legacy, "local")
	os.MkdirAll(dir, 0755)
	os.WriteFile(filepath.Join(dir, "meta.json"), []byte(`{"name":"old.dem"}`), 0644)

	// "local" parsed fresh (new path) before the import sees the old dir.
	seedDemo(t, s, "local", []Round{fixtureRound(0)})

	n, err := s.ImportLegacyOutput(legacy)
	if err != nil || n != 0 {
		t.Fatalf("import should skip existing demo, n=%d err=%v", n, err)
	}
	demos := mustList(s, t)
	if len(demos) != 1 || demos[0].Name != "test.dem" {
		t.Errorf("existing demo clobbered by import: %+v", demos[0])
	}
}

// TestGzipBenchmark documents the parse-time cost and ratio on a real
// round file — run with -short it is skipped.
func TestGzipBenchmark(t *testing.T) {
	const path = "data/output/local/0.json"
	raw, err := os.ReadFile(path)
	if err != nil || testing.Short() {
		t.Skip("no local round fixture (or -short)")
	}
	start := time.Now()
	blob, err := gzipJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	dur := time.Since(start)
	start = time.Now()
	if _, err := gunzipJSON(blob); err != nil {
		t.Fatal(err)
	}
	t.Logf("gzip: %d -> %d bytes (%.1f:1) in %v; gunzip in %v",
		len(raw), len(blob), float64(len(raw))/float64(len(blob)), dur, time.Since(start))
	if dur > 2*time.Second {
		t.Errorf("gzip too slow for parse time: %v", dur)
	}
}
