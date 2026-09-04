# AGENTS.md

Local web app that parses CS2 demo files (`.dem`) and replays them as 2D
radar-style playback in the browser. Go backend extracts per-frame game state
with demoinfocs-golang; React client renders it. Everything is embedded into a
single binary.

Read `readme.md` for the API surface and detailed gotchas. This file is the
agent-facing cheat sheet.

## Commands

```sh
make                 # build client -> copy to web/dist -> go build (binary: ./statetrak)
make run             # build, kill whatever holds :3001, run
make client          # client build only (npm install first on a fresh clone)
make clean           # remove binary + web/dist

go vet ./...         # static analysis
gofmt -l .           # must be empty before committing (tabs, gofmt style)

go run . -parse=test.dem   # offline parse, writes controllers/data/output/local/
```

A plain `go build .` works on a fresh clone (a tracked `web/dist/.gitkeep`
keeps `go:embed` valid) but serves no UI until `make client` has run.
The server listens on **:3001**.

## Verifying changes end-to-end

`test.dem` is a 266MB CS2 dust2 demo in the repo root (gitignored via `*.dem`,
but present locally). A full parse takes ~8s. Suggested workflow:

```sh
gofmt -l . && go vet ./... && go build -o /tmp/statetrak .
/tmp/statetrak -parse=test.dem                    # should print map + 17 rounds

/tmp/statetrak &                                   # or: go run . &
curl -s localhost:3001/ping
curl -s -F "file=@test.dem" localhost:3001/upload        # -> {"id":...}
curl -s localhost:3001/demos/<id>/status                 # progress 0-100, then done
curl -s localhost:3001/demos/<id>/rounds | head          # round/winner summaries
curl -s localhost:3001/demos/<id>/0 | head -c 300        # frame data

# restart the server, then confirm the demo still lists with its name (persistence):
curl -s localhost:3001/demos
curl -s -X DELETE localhost:3001/demos/<id>             # cleanup
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

- `controllers/game.go` — parser, data model (`FrameState`, `Round`, `Game`),
  JSON output. Demo id `"local"` is used by the `-parse` flag.
- `controllers/demo.go` — upload/list/status/delete routes, in-memory
  `demoStatus` map, `meta.json` persistence, `loadPersistedDemos()` on startup.
- `controllers/round.go` — round-serving routes.
- `web/web.go` — `go:embed` of `web/dist` + SPA fallback route.
- `client/` — Create React App (`react-scripts`, plain JS, no TS). Components
  in `src/components/`, weapon icons in `src/utils/weapons.js`, radar maps +
  per-map config in `src/maps/`.

## Gotchas that will bite you

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
  test route changes against a running server; gin panics at startup on bad
  route trees.
- **Round indexing**: warmup/knife before `MatchStart` is discarded; rounds
  start at 0 after that. Round files are named by round number, `output.json`
  is the game-level metadata (excluded from the rounds listing).
- `FrameRate` in `output.json` is hardcoded 60 (real value unavailable for
  CS2, same header issue as above). Client playback speed depends on it.
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