# StateTrak local
Backend for fetching game info from demos

Uses [DemoInfoCs](https://github.com/markus-wa/demoinfocs-golang)

## Standalone usage

The whole app is one self-contained binary (UI embedded via `go:embed`,
demo parsing is local, data lives under `./controllers/data`). Anyone can run
it on their own machine — no server, no operator, no shared keys:

    make release          # pure-Go cross-compiled binaries (browser mode)
    make desktop          # Wails desktop app (.app / window; see below)
    ./statetrak           # or: ./statetrak -addr=:3007 -data=~/statetrak-data

## Building

Prerequisites:

- **Go 1.25+** (per `go.mod`)
- **Node 18+ with npm** — on a fresh clone run `npm install` inside `client/`
  once. Every `make` target builds the React client (`npm run build`) and
  embeds it into the binary via `web/dist` + `go:embed` — the client build is
  not committed
- **Wails CLI**, only for `make desktop` / `make dmg` — keep it in sync with
  the `wails/v2` version pinned in `go.mod`:

      go install github.com/wailsapp/wails/v2/cmd/wails@v2.16.0

Targets:

| Target | Produces | Notes |
| --- | --- | --- |
| `make` | `./statetrak` | server binary, browser mode, UI embedded |
| `make run` | — | `make` + restart on :3007 (no window/browser pops) |
| `make client` | `web/dist` | React build only, no Go |
| `make desktop` | `desktop/build/bin/StateTrak.app` | Wails app — native window; needs the wails CLI + platform webview SDK, build on the target OS |
| `make dmg` | `release/StateTrak.dmg` | `make desktop` + macOS disk image |
| `make release` | `release/` binaries | pure Go cross-compile (mac arm/intel, linux, windows), browser mode — no cgo needed |
| `make clean` | — | removes binary, `web/dist`, `release/`, `desktop/build/bin` |

Platform notes:

- **macOS desktop build** needs Xcode Command Line Tools (cgo/WKWebView).
- **Windows/Linux desktop builds** run natively on the target OS: WebView2
  (preinstalled on Win 10/11) resp. `libwebkit2gtk` dev packages on Linux;
  then `cd desktop && wails build`.
- A plain `go build .` works everywhere without cgo and serves the UI once
  `make client` has run (a tracked `web/dist/.gitkeep` keeps `go:embed`
  valid on a fresh clone).

### macOS packaging (`make dmg`)

`make dmg` builds the Wails app and wraps `StateTrak.app` into
`release/StateTrak.dmg` with macOS' own `hdiutil` (compressed UDZO image,
volume name “StateTrak”) — recipients mount it and drag the app to
/Applications. The target only exists on macOS (`hdiutil` is macOS-only).

Signing notes:

- `wails build` **ad-hoc self-signs** the app (it runs locally without
  prompts). An ad-hoc signature carries no identity, so Gatekeeper will
  still quarantine the dmg when downloaded from the internet — recipients
  right-click the app → *Open* once, or run
  `xattr -dr com.apple.quarantine StateTrak.app` after copying.
- For real distribution, sign with a Developer ID and notarize the dmg
  (requires an Apple Developer account; notarytool needs a stored app
  profile — see `xcrun notarytool store-credentials`):

      make desktop
      codesign --deep --force --options runtime \
        --sign "Developer ID Application: Your Name (TEAMID123)" \
        desktop/build/bin/StateTrak.app
      hdiutil create -volname StateTrak \
        -srcfolder desktop/build/bin/StateTrak.app \
        -ov -format UDZO release/StateTrak.dmg
      xcrun notarytool submit release/StateTrak.dmg \
        --keychain-profile notary-profile --wait
      xcrun stapler staple release/StateTrak.dmg

  Then the dmg opens cleanly on any Mac. Until then, `make dmg` output is
  perfect for yourself and people who trust where it came from.

## Desktop app (Wails) vs browser

`make desktop` packages the whole thing as a real desktop app
(`desktop/build/bin/StateTrak.app` on macOS — window, dock icon, closing it
quits; `make dmg` also produces `release/StateTrak.dmg`). The app runs the
same gin API + embedded React UI in-process through Wails' asset server —
the client keeps using plain `fetch()`, with every non-asset request falling
through to the gin router. Data goes to `~/Library/Application Support/StateTrak`
(`-data` overrides) since a Finder-launched app has no usable working dir.
The window is fullscreenable via the native ⤢ button or **F11** (the same
shortcut works in browser mode via the Fullscreen API). Toolchain
requirements live in **Building** above. `STATETRAK_NO_OPEN=1`
suppresses the browser in server mode (set in `make run` and
`deploy/statetrak.service`).

Each user brings their own
keys via the in-app **🔑 settings panel** (stored in the browser, sent per
request — never saved server-side):

- **TypeSafe (JEV)** key for `/demos/:id/analysis`
- **LLM** — any OpenAI-compatible endpoint (base URL + model + key), powering
  the “AI coach” review: OpenAI, a gateway, or a local Ollama at
  `http://localhost:11434/v1`

## Usage

Build & run (prerequisites and all targets in **Building**; listens on `:3007`):

    make

Parse a demo offline without starting the server (writes JSON to
`controllers/data/output/local`):

    go run . -parse=path/to/demo.dem

The viewer orients around **your team**: the settings panel (⚙) stores your
Steam ID in localStorage (defaulting to the owner's account,
`client/src/utils/me.js`). With it set, the scoreboard puts your team first
("YOUR TEAM" / "ENEMY") and highlights your row, the radar draws a halo
around your own dot, the kill feed
and timeline mark your team's kills green and teammate deaths red, and the
round bar colors rounds win/loss from your perspective. Demos parsed before
the roster field shipped lack per-round team info and fall back to the old
neutral coloring — re-upload them to get it.

Analyze a parsed demo's round economies with [JEV](https://typesafe.ai)
(TypeSafe's "System One" decision model) — classifies each team's buy per
round (pistol / eco / force / half / full) and prints where JEV's semantic
judgment disagrees with the parser's deterministic threshold baseline:

    TYPESAFE_API_KEY=... go run . -jev=local   # live analysis (key: console.typesafe.ai)
    go run . -jev=local                        # dry run: baseline + example request only

The key can also live in a `.env` file in the repo root (`TYPESAFE_API_KEY=...`)
— it's gitignored, and real environment variables take precedence over it.

## API

| Route | Description |
| --- | --- |
| `POST /upload` | upload a `.dem`, kicks off async parsing, returns `{id}` |
| `GET /demos` | list demos (id, name, status) |
| `GET /demos/:id/status` | parse status incl. live progress |
| `DELETE /demos/:id` | delete upload + output |
| `GET /demos/:id/output` | game metadata (map, players, frame rate) |
| `GET /demos/:id/rounds` | round list with winners, per-team buy types and per-team rosters (steam ids, from the round's economy snapshot) |
| `GET /demos/:id/analysis` | JEV buy-type analysis per round (cached after the first call; key bring-your-own via `X-TypeSafe-Key` header, server env as fallback) |
| `POST /demos/:id/coach` | AI coach review for one player's team — server assembles demo context (rounds, buys, post-plant) and calls the OpenAI-compatible LLM from the request body; key never persisted |
| `GET /demos/:id/postplant` | post-plant positioning per planted round — T setups (spread, distance to bomb, movement), CT retake entries (timing, grouping), and how each correlates with the outcome (cached after the first call). The panel also writes out good/bad pattern bullets for a chosen player's team (`client/src/utils/postplantText.js`) |
| `GET /demos/:id/:round` | full per-frame data for one round |
| `GET /ping` | health check |

Everything else serves the embedded React app (SPA fallback).

## Configuration

All optional, via environment or `.env` in the working directory (loaded at
startup, real env vars win):

| Variable | Default | Description |
| --- | --- | --- |
| `TYPESAFE_API_KEY` | — | JEV analysis key (https://console.typesafe.ai). Server-side fallback for `-jev` and `/demos/:id/analysis` — a per-request `X-TypeSafe-Key` header (🔑 app settings) wins |
| `STATETRAK_NO_OPEN` | — | set to `1` to not open the browser on startup (headless servers; set in `deploy/statetrak.service`) |
| `UPLOAD_MAX_BYTES` | `1073741824` | max `.dem` upload size |
| `PARSE_CONCURRENCY` | `1` | concurrent demo parses (each parse is RAM-heavy) |
| `ALLOWED_ORIGIN` | — | enable CORS for one origin; the embedded client is same-origin and needs nothing |

## Deploying

Single binary behind a reverse proxy (nginx or Caddy: TLS + basic auth —
the app itself has no auth). Human runbook: `deploy/README.md`; there is
also an agent-executable runbook in `AGENTS.md` ("Server deployment") for
letting a coding agent on the VPS do the setup.

**Known limitation — user support is planned.** Auth is proxy-level basic
auth only: everyone who logs in shares full access (any friend can delete
any demo), there's no per-user attribution beyond nginx logs, and revoking
someone means editing the htpasswd. Proper user support — per-user accounts,
per-demo ownership, owner-only deletes — belongs in the app eventually.
Until then the proxy auth must stay enabled.

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
metadata. Round files also contain an `economy` object per round — each
team's buy captured just after freeze time ends: per-player start money,
spent, bank, primary weapon held, equipment value, armor/helmet/defuse, plus team aggregates
(average equipment value, average/total spent), a count of players who
survived the previous round (weapons carry over for them — the `kept` buy
type), and a heuristic pistol/eco/force/hero/kept/half/full classification. That
is the deterministic baseline for the JEV round analysis (`-jev` flag,
`controllers/jev.go`), which sends the economy state to TypeSafe's System One
and compares its judgment against the baseline.

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
- **CS2 economy properties lie in two ways** (see `snapshotTeamEconomy` in
  `game.go`): `Player.MoneySpentThisRound()` does *not* reset between rounds,
  and `Player.EquipmentValueFreezetimeEnd()` read inside the
  `RoundFreezetimeEnd` handler returns the *previous* round's snapshot. The
  economy is therefore captured one frame after freeze time ends, using
  live equipment values and start money snapshotted on `RoundStart` (spent
  = start money − bank). Pistol-round equipment values can also carry
  warmup leftovers; pistol detection uses start money ($800), which is
  reliable.
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
main.go              server entry: flags (-parse/-jev/-addr/-data), browser auto-open
server/router.go     the gin router (API routes + SPA fallback) — shared with the desktop app
web/web.go           embeds client build (web/dist): Dist() fs + SPA handler
desktop/             Wails desktop app (main.go, wails.json, build/ scaffold)
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