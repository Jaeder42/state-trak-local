# Design pass — improvement suggestions

A review of the client UI as it stands (post-sqlite, pre-v0.0.3), read
component-by-component. Organized by impact; each item names the files
involved. Nothing here is blocking the release — this is a menu, not a
todo list.

## What's already working well

- **The core loop is right**: pick demo → pick round → press play. The
  bottom round bar (win/loss colors, buy chips, side chips) is a genuine
  differentiator and reads well at a glance.
- **Team-oriented coloring** (my kills green, teammate deaths red, YOUR
  TEAM pills, win/loss round buttons) is consistently threaded through
  scoreboard, kill feed, timeline, kill markers. The identity holds.
- Dark near-black canvas with colored dots is the correct register for a
  radar replay tool; kill feed + focus HUD are well-placed overlays.
- Pan/zoom/pinch + click-to-focus is solid, with sensible click-vs-drag
  disambiguation (`Frame.jsx`).
- Time-accurate playback from frame timestamps, auto-advance between
  rounds, keyboard shortcuts (Space, ←/→, `,`/`.`, F11).

## P0 — quick wins (hours each, big perceived polish) — **DONE (v0.0.3)**

All ten shipped in one pass: Escape layering, clickable timeline markers,
drag & drop + floating progress, money column, pretty demo names, round-bar
loading shell, toast kinds, shortcuts help (? + Home/End/Shift-arrows),
9-10px type floor, aria-labels.

1. **Escape should close panels** — today it only clears the focus player
   (`Games.jsx` onKeyDown) while analysis/post-plant/coach panels close
   only on outside-click. Make Escape close the topmost panel first.
2. **Make timeline kill markers clickable** — `.timeline-markers` is
   `pointer-events: none`; for a review tool, click-a-kill-to-jump is the
   most natural seek gesture there is. The markers already carry the kill
   data for tooltips (`RoundSelector.jsx`).
3. **Drag-and-drop upload + always-visible progress** — the upload button
   hides inside the ☰ menu, and parse progress renders only while the menu
   is open (`DemoMenu.jsx`). Accept `.dem` drops anywhere on the window
   (empty state especially) and hoist the progress bar out of the menu.
   The empty state currently says "Upload or select a demo to begin" with
   no visible upload affordance.
4. **Show the money** — the parser captures per-player `startMoney`,
   `spent`, `bank`, `equipValue` (`PlayerEconomy`), but the scoreboard only
   surfaces the team average (`ScoreBoard.jsx` `avgEquip`). A bank/$ column
   on the team tables (CS2 scoreboard style) is one of the most useful
   review facts and the data is already in `output.economy`.
5. **Prettify demo names** — the select shows raw filenames like
   `2025-11-03_20-39-21_-1_de_ancient_team_Cayeen_vs_team_HunkDaddy.dem`.
   Strip `.dem`, split date/map/teams into a structured row (P1 turns this
   into a real list panel).
6. **Round bar shell during loads** — `RoundSelector` returns `null` while
   a round loads, so the bottom bar (and its 120px) vanishes and the whole
   layout jumps. Render the shell with a spinner instead. Same for the
   `…loading` empty-state swap in `Games.jsx`.
7. **Toast variants** — `.toast` is styled as an error (red) for every
   message, including info. Add `info`/`success`/`error` kinds.
8. **Shortcut discoverability** — the keyboard shortcuts are good but
   invisible. A tiny `?` overlay listing them; also worth adding Home/End
   (round start/end) and Shift+←/→ for 5s jumps.
9. **Minimum legible type** — buy chips are `font-size: 7px`, side chips
   7px, outcomes 9px (`index.css`). Nothing under 9-10px; round buttons
   have room (48px tall) for 10px chips.
10. **Icon buttons need labels** — ☰, ⚙, ▶ are hover-titled only and mean
    different things to different people; the three analysis actions
    (JEV, post-plant, coach) hide behind ☰ where nobody will find them.
    Give the menu buttons `aria-label`s and text labels where they fit —
    P1 addresses the placement properly.

## P1 — structural (a day or two each)

11. **Design tokens: one palette, one place.** CT blue `#68a3e5` is
    hardcoded 10× in `index.css` and 3× in the canvas code (`Frame.jsx`);
    win green `#4caf50` ~8×; radii wander across 4/6/8/10/20/22px; four
    near-identical button classes duplicate each other. Add a CSS custom
    properties layer (`--ct`, `--t`, `--win`, `--loss`, `--panel`,
    `--border`, `--radius-*`, a 5-step type scale) and a small JS palette
    module so canvas dots, timeline, and CSS can never drift apart. This
    is the prerequisite that makes every later visual change cheap.
12. **Radar dot upgrade** (`Frame.jsx` `draw`): today a 4px circle + a
    1×15px view line. Suggested: CS2-style filled direction wedge with
    the player's number inside, dead players as a gray × rather than a
    fading gray dot, and the firing indicator as a brief muzzle flash on
    the dot edge instead of a 30px line. Numbers-in-dots would also make
    "click dot ↔ scoreboard row" matching instant.
13. **Bomb site labels** — render A/B markers from a per-map `sites` array
    in `maps/config.js`. For a tool whose headline analysis is post-plant
    positioning, reading "setup at A" needs A on the map.
14. **Grenade feedback bloom** — smokes pop in at full opacity instantly;
    flashes too (`drawSmoke`/`drawFlash`). Fade smokes in over ~0.5s with
    a feathered edge, flash a decaying white bloom. Cheap, huge perceived
    quality.
15. **Surface the review features** — post-plant is the marquee analysis
    and lives in a menu behind a hamburger. Options: a "Review" button on
    the round bar, or a right drawer with the three analyses as tabs
    (Economy / Post-plant / Coach) with one-line descriptions of what
    each does. Also show a hint when no 🔑 key is set.
16. **Focus-cam affordance** — when following a player there's no visual
    indication the view is attached; add a small "Following <name> —
    click map or Esc to release" chip near the FocusHud.
17. **Responsive layout** — `main-layout` is `height: calc(100vh - 60px)`
    while the round bar is ~120px and the title ~40px; `Frame` also
    falls back to `innerHeight - 200`. On short/narrow windows the radar
    clips and the round buttons squash 17 ways. Make the page a
    grid (header / stage / bar) so the canvas sizes from its real box,
    and let the round bar scroll horizontally below ~900px. Below ~1000px
    the scoreboard could become a toggleable overlay instead of a column.

## P2 — features worth building (bigger)

18. **Clutch detection on the timeline** — computable client-side from
    kill order + alive counts: mark rounds where the last player of a team
    is alive vs 2+ enemies (won/lost). Review tools live and die by
    "jump to the interesting moments"; this is the cheapest interesting
    moment to detect. Badges on round buttons + a timeline marker.
19. **Round intro card** — on round switch, flash a 1.5s card:
    "Round 12 — CT FULL vs T ECO — CT lead 7:4 — you played CT". All the
    data is already in `rounds` + `output.economy`; it gives context
    before the frames roll.
20. **Match economy graph** — team money/equip per round as a small line
    chart (the economy snapshots exist per round). Answers "when did we
    break their economy" at a glance; fits the future Review drawer.
21. **ADR/KAST in the scoreboard** — `damage` is cumulative per player;
    ADR is one division away. Modest but standard review vocabulary.
22. **Timeline hover preview** — hovering the scrubber shows the game
    time + alive counts at that point (frames are all client-side).
23. **Per-player trail filter** — once dots have numbers (P1 #12), allow
    "trails only for focused player".

## Nitpicks

- `tooglePlay` typo (`Games.jsx`) — harmless, rename when touching it.
- The 15%-opacity logo backdrop behind the map (`map-backdrop`) adds
  noise in the margins; consider empty-state-only.
- Legend row could fade out after ~10s of playback — it's static
  information occupying timeline space.
- `.analysis-panel` min-widths (520/720px) will fight narrow desktop
  windows; they have max-width guards, verify at 800×600.

## Suggested order

Ship v0.0.3 as-is. Then: #11 tokens first (it makes everything else
cheap), then the P0 batch (1–10) as a "polish" release, then #12–14
radar quality, then #15–17 structure, and pull from P2 by demand —
#18 clutch detection is the one I'd bet on for word-of-mouth.