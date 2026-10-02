# AGENTS.md

Local web app that parses CS2 demo files (`.dem`) and replays them as 2D
radar-style playback in the browser. Go backend extracts per-frame game state
with demoinfocs-golang; React client renders it. Everything is embedded into a
single binary.

Read `readme.md` for the API surface and detailed gotchas. This file is the
agent-facing cheat sheet.

## Commands

```sh
make                 # build client -> copy to web/dist -> go build (portable binary, browser mode)
make run             # build, kill whatever holds :3007, run (STATETRAK_NO_OPEN=1 — nothing pops)
make client          # client build only (npm install first on a fresh clone)
make clean           # remove binary + web/dist + release/ + desktop/build/bin
make release         # pure-Go cross-compiled binaries into release/ (browser mode, gitignored)
make desktop         # Wails desktop app -> desktop/build/bin/StateTrak.app (needs the wails CLI)
make dmg             # make desktop + release/StateTrak.dmg

# CI (.github/workflows/build.yml): push/PR -> lint + portable cross-builds;
# tags (v*) / manual dispatch -> Wails desktop matrix (mac universal, win,
# linux webkit2_41) + GitHub release with all artifacts. No secrets needed.

go vet ./...         # static analysis
gofmt -l .           # must be empty before committing (tabs, gofmt style)

go run . -parse=test.dem   # offline parse, writes controllers/data/output/local/
go run . -jev=local        # JEV economy analysis of a parsed demo (TYPESAFE_API_KEY from env or .env; dry run without it)
./statetrak -addr=:3011 -data=/some/dir   # standalone run: custom port/data dir, opens the browser
```

Agents running inside pi get the `build_app` tool
(`.pi/extensions/build-app.ts`): build for a given OS with
`build_app(os=<macos|windows|linux>, target=<desktop|portable|dmg>, publish=<bool>)` —
local build when the OS matches the host; a per-OS CI dispatch
(`gh workflow run release-<os>.yml`) otherwise, where publish=true attaches
the artifact to the dispatched tag's release.

A plain `go build .` works on a fresh clone (a tracked `web/dist/.gitkeep`
keeps `go:embed` valid) but serves no UI until `make client` has run —
it builds the plain portable binary (browser mode). It listens on **:3007**
(`-addr` to override) and opens the default browser on startup unless
`STATETRAK_NO_OPEN=1`. The packaged desktop app is built separately with
`make desktop` (Wails).

## Verifying changes end-to-end

`test.dem` is a 266MB CS2 dust2 demo in the repo root (gitignored via `*.dem`,
but present locally). A full parse takes ~8s. Suggested workflow:

```sh
gofmt -l . && go vet ./... && go build -o /tmp/statetrak .
/tmp/statetrak -parse=test.dem                    # should print map + 17 rounds

/tmp/statetrak &                                   # or: go run . &
curl -s localhost:3007/ping
curl -s -F "file=@test.dem" localhost:3007/upload        # -> {"id":...}
curl -s localhost:3007/demos/<id>/status                 # progress 0-100, then done
curl -s localhost:3007/demos/<id>/rounds | head          # round/winner summaries
curl -s localhost:3007/demos/<id>/0 | head -c 300        # frame data

# restart the app, then confirm the demo still lists with its name (persistence):
curl -s localhost:3007/demos
curl -s -X DELETE localhost:3007/demos/<id>             # cleanup
```

`controllers/data/` (uploads + output) is gitignored — safe to create/delete
freely during testing. Leave it as you found it (the repo has some pre-existing
`controllers/data/output/*.json` from old manual runs; don't touch those).

## Architecture (request flow)

```
POST /upload -> saves .dem to controllers/data/uploads/, writes
                controllers/data/output/<id>/meta.json (original filename),
                spawns goroutine running ParseDemo()
ParseDemo()   -> demoinfocs event handlers accumulate FrameState per round,
                writes <round>.json + output.json per demo,
                reports byte-based progress via updateProgress()
client        -> fetches /demos/:id/output (metadata), /demos/:id/:round
                (frames), renders on radar images from client/src/maps/
```

- `controllers/game.go` — parser, data model (`FrameState`, `Round`, `KillEvent`,
  `Game`), JSON output. Round JSON contains `frames`, `winner`, `kills`
  (per-kill attacker/victim/team/weapon/headshot/death-position), and an
  `economy` snapshot per round (per-team buy captured just after freeze-time
  end, with a heuristic pistol/eco/force/hero/kept/half/full baseline classification
  driven by held rifles (strength) plus money spent (intent): full = 4+/5
  players hold rifle-class weapons bought or kept; kept = full rifle strength
  with < $1500 spent (top-ups stay kept); hero = exactly one buyer of a real
  weapon (>= $1500) while the rest save; half =
  only 2-3 rifles held).
  Demo id `"local"` is used by the `-parse` flag.
- `controllers/jev.go` — JEV (TypeSafe System One) client + round-economy
  analyzer, shared by two entry points: the `-jev` CLI flag (prints a
  baseline vs JEV comparison) and `GET /demos/:id/analysis` (runs the same
  analysis, caches it as `analysis.json` in the demo's output dir so JEV is
  billed once per demo, single-flighted per demo id). The key is
  bring-your-own: `X-TypeSafe-Key` request header (🔑 client settings) wins,
  `TYPESAFE_API_KEY` env is the fallback; a bad key degrades stickily to
  baseline-only results (by design).
- `controllers/postplant.go` — deterministic post-plant analysis
  (`/demos/:id/postplant`, no key needed): per planted round, T setup
  positions at plant+5s (spread, distance to bomb), movement +5s→+15s,
  CT retake entry (first CT within 500u of the bomb, how grouped they
  entered) and the outcome (defused/exploded/eliminated); computed from the
  round JSON files on first request, cached as `postplant.json`. Phantom
  plants are detected by the planted-run shape — RoundEnd resets
  `bombState`, so a real plant's `planted:true` run is always followed by
  `planted:false` frames, while a phantom run reaches the round's last
  frame. Don't use the frame `phase` for this: the LIVE heuristic
  (`GamePhase()==2`) never fires in the second half of CS2 demos.
- `controllers/coach.go` — "AI coach" (`POST /demos/:id/coach`): the client
  sends its own OpenAI-compatible LLM config (base URL + model + key, from
  the 🔑 settings in localStorage — works with OpenAI, gateways, local
  Ollama), the Go backend assembles a compact context (map, rounds/buys/rosters,
  post-plant analysis, player sides) and proxies to
  `{baseUrl}/chat/completions`. Keys are per-request only — never logged,
  cached or persisted. Bad LLM key → 502 with the provider's error.
- `controllers/demo.go` — upload/list/status/delete routes, in-memory
  `demoStatus` map, `meta.json` persistence, `loadPersistedDemos()` on
  startup. No package `init()`: `main` calls `controllers.Init()` after
  flag parsing, so `-data`/`SetDataDir` can relocate the upload/output dirs
  first (defaults keep `./controllers/data`).
- `controllers/round.go` — round-serving routes; `/demos/:id/rounds`
  summaries include each team's roster steam ids (from the round's economy
  snapshot) so the client knows which side "my" player was on per round
  (teams swap at halftime).
- `web/web.go` — `go:embed` of `web/dist`: `Dist()` fs for asset servers + SPA fallback route.
- `server/router.go` — the gin router (API routes + SPA fallback), shared by
  the portable binary (`main.go`) and the Wails desktop app (`desktop/`).
- `desktop/` — Wails v2 desktop app: native window running the same router
  in-process via the asset server (`make desktop`, `make dmg`).
- `client/` — Create React App (`react-scripts`, plain JS, no TS). Components
  in `src/components/`, weapon icons in `src/utils/weapons.js`, radar maps +
  per-map config in `src/maps/`. The viewer focuses on the demo owner's team:
  `src/utils/me.js` holds "my" Steam ID (localStorage, empty until set in
  the ⚙ panel),
  scoreboard/kill feed/radar/round bar orient around it (YOUR TEAM first,
  win/loss round colors) — falls back to neutral coloring
  when the steam id isn't in the demo or the demo predates rosters.
  The post-plant panel adds a deterministic good/bad-pattern narrative for
  a chosen player's team (`src/utils/postplantText.js`): sides per round
  come from the round rosters (position-snapshot fallback for old demos),
  bullets fire only on clear data patterns — keep it conservative, don't
  invent claims to fill space.

## Gotchas that will bite you

- **CS2 players can plant the bomb after `RoundEnd`**, during the round-over
  period. That phantom plant is a real event (it shows in the finished
  round's last frames). All transient state (bomb, smokes, flashes, HEs) is
  reset on **`RoundStart`** as well as `RoundEnd` — without the `RoundStart`
  reset, the next round inherits `planted: true` for its entire duration.
  If you touch the round lifecycle handlers, re-verify with a demo that has
  a post-round plant (compare `bombState.planted` transitions per round).
  The post-plant analysis (`controllers/postplant.go`) depends on the
  RoundEnd reset to tell real plants from phantom ones — re-check that
  endpoint too if the reset logic changes.
- `bombState` in frames has three states: planted (event-tracked), carried
  (`GameState().Bomb().Carrier()`, position follows the carrier), dropped
  (bomb's last position on the ground). Players legitimately toss the C4
  around at round start — the carrier changing is real, not a bug.

- **CS2 demos have no frame count until the end of the file.** demoinfocs
  `Parser.Progress()` (header-based) is therefore 0 for the whole parse.
  Progress is tracked by bytes consumed via the `countingReader` wrapper in
  `game.go` and surfaced as `progress`/`total` (percent) on the status route.
  Don't "simplify" this back to `p.Progress()`.
- **Route params are used to build file paths** — always go through
  `demoDir()` / `filepath.Base` (`demo.go`). Never interpolate `c.Param()`
  directly into a filesystem path.
- **Status map is in-memory only**; names + done/error state survive restarts
  solely via `meta.json` + `output.json` presence (see `loadPersistedDemos`).
- **Gin serves static and param siblings** on `/demos/:id/...`
  (`rounds`/`status`/`output` vs `:round`) — this works on current gin, but
  test route changes against the running app; gin panics at startup on bad
  route trees.
- **Round indexing**: warmup/knife before `MatchStart` is discarded; rounds
  start at 0 after that. Round files are named by round number, `output.json`
  is the game-level metadata (excluded from the rounds listing).
- **CS2 economy properties are traps** (see `snapshotTeamEconomy`):
  `Player.MoneySpentThisRound()` never resets between rounds, and
  `Player.EquipmentValueFreezetimeEnd()` read inside the `RoundFreezetimeEnd`
  handler returns the previous round's snapshot. Economy is therefore
  captured one frame *after* freeze time ends (live equipment values), with
  start money snapshotted on `RoundStart`; spent = start money − bank.
  Pistol rounds are detected via start money ($800), not equipment values
  (which carry warmup leftovers in round 0).
- `FrameRate` in `output.json` is hardcoded 60 (real value unavailable for
  CS2, same header issue as above). Client playback speed depends on it.
- **The desktop app is a Wails build in `desktop/`** — same module, its own
  `main` package. It plugs the shared `server.NewRouter()` into Wails'
  asset server (`Assets: web.Dist()`, `Handler: router`): UI assets come
  from the embedded build, every other request falls through to the gin
  routes, so the React client's `fetch()` calls work unchanged (no Wails
  bindings). Keep the wails CLI version in sync with the library
  (go.mod pins `wails/v2 v2.16.0`; CLI: `go install .../cmd/wails@v2.16.0`).
  The desktop app defaults its data dir to `~/Library/Application
  Support/StateTrak` (`-data` overrides) because a Finder-launched app has
  cwd=/. The `desktop/build/` scaffold (appicon, Info.plist) is committed;
  `desktop/build/bin/` is gitignored.
- **`Mac: &mac.Options{}` in the wails options must stay non-nil** — wails
  v2.16 initializes the darwin `zoomable` flag only inside its `Mac != nil`
  branch; with a nil Mac it stays 0 and the frontend disables the green
  maximize/fullscreen button (`standardWindowButton:NSWindowZoomButton
  setEnabled:NO`). Don't "clean up" that empty-looking option away.
- Data model is 2D: positions keep x/y only, z is dropped by design.

## Conventions

- Go: gofmt-formatted (tabs), errors from marshal/write paths panic via
  `log.Panic` (they're recovered by the upload goroutine and surfaced as
  `status: "error"`). Handlers keep the existing 404/400 JSON shapes — the
  client doesn't have sophisticated error handling.
- Commits: short, lowercase, imperative ("cleanup: ...", "make usable").
- Never commit build artifacts: `client/build/`, `web/dist/*` (except
  `.gitkeep`), `statetrak`, `state-trak-local`, `*.dem`, `controllers/data/`,
  `.DS_Store` — all gitignored, keep it that way.

