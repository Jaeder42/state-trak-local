# StateTrak local
Backend for fetching game info from demos

Uses [DemoInfoCs](https://github.com/markus-wa/demoinfocs-golang)

## Usage

Build & run (compiles the React client into the binary, listens on `:3001`):

    make

`make` runs `npm run build` in `client/` and copies the output into `web/dist/`,
which gets embedded into the Go binary via `go:embed`. The client build is
**not** committed, so on a fresh clone you must build the client first —
a plain `go build` works (a `web/dist/.gitkeep` keeps the embed valid) but the
server will serve no UI until you run `make` (requires `npm install` in
`client/` on a fresh clone).

Parse a demo offline without starting the server (writes JSON to
`controllers/data/output/local`):

    go run . -parse=path/to/demo.dem

## API

| Route | Description |
| --- | --- |
| `POST /upload` | upload a `.dem`, kicks off async parsing, returns `{id}` |
| `GET /demos` | list demos (id, name, status) |
| `GET /demos/:id/status` | parse status incl. live progress |
| `DELETE /demos/:id` | delete upload + output |
| `GET /demos/:id/output` | game metadata (map, players, frame rate) |
| `GET /demos/:id/rounds` | round list with winners |
| `GET /demos/:id/:round` | full per-frame data for one round |
| `GET /ping` | health check |

Everything else serves the embedded React app (SPA fallback).

## How parsing works

`controllers/game.go:ParseDemo()` walks the demo with demoinfocs event
handlers (frame done, round start/end, smoke/flash/HE explode, weapon fire,
bomb planted, kill, ...) and accumulates per-round `FrameState`s: player
positions, yaw, health, weapons, scoreboard, smokes/flashes/HEs/fires, grenade
projectiles, bomb state. `bombState` tracks the C4 through all its states:
carried (with the carrier's steam id + position), dropped (last position on
the ground), and planted. Output is written per round (`<round>.json`), which
contains the frames, the round winner, and a `kills` array (`KillEvent`:
attacker/victim names, steam ids, teams, weapon, headshot flag, and the
victim's death position — the client uses it for the kill feed, kill markers,
timeline notches, and fading dead dots), plus a `output.json` with game
metadata.

## Gotchas & decisions worth remembering

- **CS2 players can still plant the bomb after `RoundEnd`**, during the
  round-over period. That "phantom" plant is a real event and leaves a few
  planted frames at the end of the finished round — all transient state
  (bomb, smokes, flashes, HEs) is therefore reset on **`RoundStart`**, not
  just `RoundEnd`, so it can't leak into the next round's frames.
- **CS2 demos don't expose a frame count until the end of the file** — the
  demoinfocs `Progress()` call is based on header playback frames, which for
  CS2 are only known once the `CDemoFileInfo` message at the very end is
  reached. `Progress()` therefore always returns 0 mid-parse. Parse progress
  is instead tracked by bytes consumed through a `countingReader` wrapper
  (`game.go`), reported as `progress`/`total` (percent) on
  `GET /demos/:id/status`.
- **Demo names/statuses are persisted in `controllers/data/output/<id>/meta.json`**
  (written at upload time) and reloaded by `loadPersistedDemos()` on startup.
  The in-memory `demoStatus` map would otherwise be lost on restart. Demos
  whose `output.json` is missing (e.g. server killed mid-parse) show up as
  `error` after a restart.
- **Upload/parse status lives only in memory during a run** — there is no
  cross-process coordination. `-parse` runs are not tracked in the status map.
- **File-serving routes sanitize the id/round params with `filepath.Base`**
  (`demoDir()` in `demo.go`) — don't build paths from route params without it.
- **Round/frame data is 2D (x/y only)** — the client renders on radar images
  (`client/src/maps/`), so z is dropped.
- `FrameRate` in `output.json` is hardcoded to 60; the demoinfocs header that
  would give the real value isn't available for CS2 (see first bullet).
- The knife/warmup round before `MatchStart` is discarded; round counting
  starts from there (`currentRound` in `game.go`).

## Project layout

```
main.go              Gin server, routes, -parse flag
controllers/
  demo.go            upload/list/status/delete, status persistence
  game.go            ParseDemo(): demo -> JSON round files
  round.go           round/rounds endpoints
web/web.go           embeds client build (web/dist) + SPA fallback
client/              React viewer (Create React App)
  src/components/    Games, RoundSelector, ScoreBoard, Controls, ...
  src/maps/          radar images + per-map config
controllers/data/    uploads + parse output (gitignored)
```