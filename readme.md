# StateTrak

**Watch your Counter-Strike 2 demos from above.** StateTrak parses `.dem`
files and replays them as a 2D radar playback in the browser or a native
desktop window — oriented around *your* team, with round-economy analysis,
post-plant positioning stats, and an AI coach review powered by your own
LLM.

Parsing is done with
[DemoInfoCs](https://github.com/markus-wa/demoinfocs-golang); everything
(demo files, positions, keys) stays on your machine.

<!-- ![playback](docs/screenshot.png) -->

## Features

- **Radar replay** — smooth playback at 0.25–4× with pan/zoom/pinch and a
  click-to-focus follow cam; movement trails, kill feed + kill markers,
  health bars, smokes/flashes/HEs, and full bomb tracking (carried, dropped,
  planted — with a bomb-timer pill)
- **Your team first** — set your Steam ID once and the whole viewer orients
  around you: your team on top of the scoreboard ("YOUR TEAM" / "ENEMY"),
  your row and your dot highlighted, your kills green and teammate deaths
  red, round buttons colored win/loss — correct through halftime side swaps
- **Round economy** — per-round buy snapshots with a heuristic
  pistol/eco/force/hero/kept/half/full classification, plus optional
  [JEV](https://typesafe.ai) analysis (TypeSafe's "System One") that judges
  every buy semantically and flags where it disagrees with the baseline
- **Post-plant positioning** — T setup spread/distance/movement, CT retake
  entry timing and grouping, correlated with outcomes (defused / exploded /
  eliminated), per-round and aggregate radar maps, watch-from-plant jumps,
  and a deterministic good/bad-pattern writeup for any chosen player's team
- **AI coach** — a written review of your demo from *your own*
  OpenAI-compatible LLM (OpenAI, a gateway, or a local Ollama); the app
  assembles the context, your key never leaves the request
- **Bring your own keys** — the TypeSafe (JEV) key and the LLM config live in
  your browser (🔑 settings), are sent per request, and are never stored
  by the app
- **Standalone** — one self-contained binary (UI embedded via `go:embed`),
  or a native desktop window via Wails

## Try it

Grab a binary from [Releases](https://github.com/Jaeder42/state-trak-local/releases)
and run it — the default browser opens at `localhost:3007`, upload a `.dem`,
and press play:

    ./statetrak                       # listens on :3007, opens the browser
    ./statetrak -addr=:3011 -data=~/statetrak-data

- **macOS desktop app** — `StateTrak-macos.dmg` (universal): drag to
  /Applications; closing the window quits the app; data lives in
  `~/Library/Application Support/StateTrak`
- **Portable binaries** — `statetrak-macos-arm64/-intel`, `statetrak-linux-amd64`,
  `statetrak-windows-amd64.exe`: same app without the window — run it and
  it opens in your default browser. No cgo, no install, works everywhere
- Downloaded dmgs are ad-hoc signed: right-click → *Open* on first launch
  (see [macOS packaging](#macos-packaging-make-dmg) for notarization)

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
| `make` | `./statetrak` | portable binary, browser mode, UI embedded |
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
requirements live in **Building** above. `STATETRAK_NO_OPEN=1` suppresses
the browser in portable mode (set in `make run`).

## Usage

Build & run (prerequisites and all targets in **Building**; listens on `:3007`):

    make

Parse a demo offline without starting the app (writes JSON to
`controllers/data/output/local`):

    go run . -parse=path/to/demo.dem

Refresh a previously uploaded demo's output with the current parser
(e.g. after new fields like rosters/economy were added — derived caches are
invalidated automatically):

    go run . -reparse=<demo id>

**Your team.** The ⚙ panel stores your Steam ID in localStorage (empty
until you set it — `client/src/utils/me.js` holds the logic). With no ID
set the viewer stays neutral. Demos parsed before the roster field shipped
lack per-round team info and fall back to neutral coloring — refresh them
with `go run . -reparse=<demo id>`.

**JEV economy analysis** (TypeSafe's "System One" decision model) —
classifies each team's buy per round (pistol / eco / force / half / full)
and prints where JEV's semantic judgment disagrees with the parser's
deterministic threshold baseline:

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
| `GET /demos/:id/analysis` | JEV buy-type analysis per round (cached after the first call; key bring-your-own via `X-TypeSafe-Key` header, app env as fallback) |
| `POST /demos/:id/coach` | AI coach review for one player's team — the app assembles demo context (rounds, buys, post-plant) and calls the OpenAI-compatible LLM from the request body; key never persisted |
| `GET /demos/:id/postplant` | post-plant positioning per planted round — T setups (spread, distance to bomb, movement), CT retake entries (timing, grouping), and how each correlates with the outcome (cached after the first call). The panel also writes out good/bad pattern bullets for a chosen player's team (`client/src/utils/postplantText.js`) |
| `GET /demos/:id/:round` | full per-frame data for one round |
| `GET /ping` | health check |

Everything else serves the embedded React app (SPA fallback).

## Configuration

All optional, via environment or `.env` in the working directory (loaded at
startup, real env vars win):

| Variable | Default | Description |
| --- | --- | --- |
| `TYPESAFE_API_KEY` | — | JEV analysis key (https://console.typesafe.ai). Fallback from the environment or `.env` for `-jev` and `/demos/:id/analysis` — a per-request `X-TypeSafe-Key` header (🔑 app settings) wins |
| `STATETRAK_NO_OPEN` | — | set to `1` to not open the browser on startup in portable mode (set in `make run`) |
| `UPLOAD_MAX_BYTES` | `1073741824` | max `.dem` upload size |
| `PARSE_CONCURRENCY` | `1` | concurrent demo parses (each parse is RAM-heavy) |

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

Deeper implementation gotchas and design decisions are collected in
[AGENTS.md](AGENTS.md).

## Project layout

```
main.go              portable binary entry: flags (-parse/-jev/-addr/-data), browser auto-open
server/router.go     the gin router (API routes + SPA fallback) — shared with the desktop app
web/web.go           embeds client build (web/dist): Dist() fs + SPA handler
desktop/             Wails desktop app (main.go, wails.json, build/ scaffold)
controllers/
  demo.go            upload/list/status/delete, status persistence
  game.go            ParseDemo(): demo -> JSON round files
  round.go           round/rounds endpoints
client/              React viewer (Create React App)
  src/components/    Games, RoundSelector, ScoreBoard, Controls, ...
  src/maps/          radar images + per-map config
controllers/data/    uploads + parse output (gitignored)
```

## Agent tooling (`build_app`)

For coding agents working in this repo with
[pi](https://github.com/earendil-works/pi-coding-agent): `.pi/extensions/build-app.ts`
registers a `build_app` tool with parameters `os` (macos/windows/linux),
`target` (desktop/portable/dmg) and an optional `publish` (CI dispatches).
Portable binaries cross-compile locally from any host; desktop builds run
locally when `os` matches the host and otherwise dispatch that OS's release
workflow, `release-<os>.yml` (needs `gh` authenticated), and return the run
URL — with `publish=true` the artifact attaches to the dispatched tag's
release. Agents should prefer the tool over hand-rolling make/wails/gh
command chains.

## Release notes

Convention: every tagged release gets a short section here (and the GitHub
release body can lift it). Sections append newest-first.

### v0.1.0 — first tagged release

The complete app as it stands: demo parsing, radar playback, team-oriented
review, three analysis layers, standalone distribution with bring-your-own
keys, and a native desktop build.

**Playback**
- CS2 demo parsing (demoinfocs) → per-round JSON: player positions/yaw/health,
  scoreboard, kills, smokes/flashes/HEs/fires, and full bomb tracking —
  carried (carrier ring + C4 badge), dropped (blinking) and planted (pulsing)
- 2D radar replay: 0.25–4× speeds (time-accurate — timing measured from
  the demo's own ~67 fps tick rate), pan/zoom/pinch, click-to-focus follow cam,
  movement trails, kill feed + kill markers, health bars, round bar with
  per-team buy chips and a bomb-timer pill

**Your team**
- The viewer orients around your team from your Steam ID (⚙ panel): your
  team listed first ("YOUR TEAM"/"ENEMY"), your row highlighted, a white halo
  on your dot, kill feed/timeline/radar colored from your perspective
  (green = enemy down, red = teammate down), and round buttons colored
  win/loss — correct through halftime side swaps via per-round rosters

**Analysis**
- Per-round economy snapshots with heuristic buy classification
  (pistol/eco/force/hero/kept/half/full)
- **JEV analysis** — TypeSafe System One judging every team's buy against the
  baseline, cached per demo, bring-your-own key (🔑)
- **Post-plant positioning** — T setups 5s after the plant (spread, distance
  to the bomb, hold-vs-push movement), CT retake entries (timing, grouping),
  outcomes (defused/exploded/eliminated), phantom plants excluded, per-round
  radar maps, aggregate radars colored by outcome, watch-from-plant jumps,
  and a deterministic good/bad team-pattern narrative for any chosen player
- **AI coach** — a written review of your team's demo from your own
  OpenAI-compatible LLM (OpenAI, gateways, local Ollama), app-assembled
  context, keys per-request only

**Standalone + desktop**
- One self-contained binary; browser auto-opens; `-addr`/`-data` flags;
  every user brings their own JEV/LLM keys (stored in the browser, never stored by the app)
- **Wails desktop app** (`make desktop`/`make dmg`): native window with the
  same UI and API in-process, green-button fullscreen, per-user data dir,
  drag-to-Applications dmg
- `make release` cross-compiles pure-Go browser-mode binaries; GitHub
  Actions builds everything on tags (see AGENTS.md)

**Known limitations**
- Playback timing assumes a uniform frame rate per round; the sparse
  round-over segments play slightly compressed (the round clock stays
  exact)
- Demos parsed before the economy/roster fields shipped fall back to neutral
  coloring and lack per-round buys — re-upload to refresh them
- No automated test suite — CI covers gofmt/vet, builds, and a boot smoke
  test; real parsing is verified manually against a local demo
- The macOS dmg is ad-hoc signed (right-click → Open on first launch);
  notarization is documented but requires an Apple Developer account

## License

MIT — see [LICENSE](LICENSE). Third-party notices for the bundled Go/npm
dependencies: [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md) (also served
by the app at `/THIRD_PARTY_NOTICES.md`).