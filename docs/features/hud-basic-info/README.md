# Feature: Basic Info HUD (name, job, HP/SP, levels)

**Branch:** `feature/hud-basic-info` · **Issue:** #85 · **Parent:** #49 (MVP scope)
**Status:** Planned · **Created:** 2026-08-26

## Goal

Standing in Prontera, the player sees the original client's Basic Info panel in the top-left: their character's name and job, HP and SP as coloured gauges with `current / max` printed on them, and Base/Job level with experience bars. The values track the server — taking damage moves the HP gauge, sitting refills it — rather than being drawn once and going stale. This advances the **UI** item of the six-feature MVP, and is the anchor the rest of the HUD (chat, inventory, skills, quick-spell) positions itself against.

## Reference (original client)

![grf-basewin-bg2 — the Basic Info panel background, 220×135, extracted from data.grf (`basic_interface/basewin_bg2.bmp`)](./grf-basewin-bg2.png)

1. **Title bar** — the same 17px gradient bar the window chrome uses, with the system buttons at each end (11×11).
2. **Name / Job rows** — plain text on the white body; nothing is painted for them, so both are drawn.
3. **Gauge troughs** — the two rounded wells at y≈53 and y≈68 are **painted into this bitmap**. Only the coloured fill is drawn on top; we do not draw the trough.
4. **Info block** — the flat grey panel below the gauges is where Base/Job level and the experience bars sit.
5. **Footer strip** — the striped band at the bottom, below which the menu buttons hang.

![grf-btn-info1 — one of the five menu buttons, 54×18 (`basic_interface/info1.bmp`)](./grf-btn-info1.png)

6. **Menu buttons** — 54×18 each. The archive carries **ten** of them, each in three states (`info1/2/3`, `skill1/2/3`, `item*`, `map*`, `party*`, `guild*`, `quest*`, `option*`, `booking*`, `recruit*`), which matches the screenshot's grid rather than roBrowser's row of five. Four per row × 54 = 216, against a 220-wide panel. This feature draws them but wires none of them.

**Transparency:** the panel's left and right edges are magenta (255,0,255) colour-key, not visible pixels — the loader must key them out or the panel gets pink borders.

**Real client, supplied in review** (kRO, classic skin, windowed — posted on this issue). What it shows, and the names the plan uses:

7. **Panel header** — reads `Basic Info`, with a **single** button at the right (minimize). There is no close button: this panel is always present.
8. **Name / job** — `TankJr` over `Novice`, left-aligned on the white body.
9. **HP / SP rows** — label, then a filled gauge, then the percentage on the right: `HP [====] 54 / 54 100%`. The numbers sit **on** the gauge, the percentage outside it.
10. **Base Lv. / Job Lv.** — each with a thin experience bar to its right.
11. **Weight and Zeny** — `Weight: 50/2150   Zeny: 100,000` on one line under the levels, inside the panel.
12. **Menu buttons** — a **grid of ten** below the panel, four per row: `info skill item map` / `party guild quest option` / `booking rec` + the game's logo. Not the single row of five that roBrowser's older markup describes.

Also visible but **not this feature**: the minimap (top-right), the tabbed chat box (bottom-left, `Regular Chat` / `Battle Log`), and the HP bar under the character.

The client's own bitmaps below remain the measurement source; the screenshot is what confirms which elements exist and how they read.

Behaviour confirmed from documentation rather than a capture ([StrategyWiki](https://strategywiki.org/wiki/Ragnarok_Online/Basic_Info), [iRO Wiki](https://irowiki.org/wiki/Basic_Game_Control)): the window shows name, class, Base/Job level with EXP as a percentage, HP, SP, weight and Zeny, and **Ctrl+V toggles between the large and reduced form**.

**Current state (ours):** nothing related renders — the in-game HUD is a debug overlay and a bottom status bar (`internal/game/ui/ui2d_backend.go:856` `RenderInGameUI`), so there is no "before" screenshot to show.

## Layout (from roBrowser, `src/UI/Components/BasicInfo/BasicInfo.css`)

Coordinates are relative to the panel's top-left. Cross-checked against the bitmap: the panel is 220×135 and the buttons are 54×18 in both.

| Element | Position | Size |
|---|---|---|
| Panel (large) | — | 220 × 135 |
| Panel (reduced) | — | 220 × 53 |
| System button, left / right | (4, 3) / (right 2, 3) | 11 × 11 |
| Title | (18, 2) | — |
| Name | (10, 20) | — |
| Job | (10, 33) | — |
| "HP" / "SP" labels | (15, 50) / (15, 65) | — |
| HP gauge / SP gauge | (35, 53) / (35, 68) | 135 × 8 |
| `cur / max` on gauge | centred over the gauge | 127 wide, 10px text |
| HP % / SP % | (right 20, 50) / (right 20, 65) | — |
| Base Lv. / Job Lv. | (15, 86) / (15, 97) | — |
| Base EXP / Job EXP bar | (84, 89) / (84, 101) | 110 × 4 |
| Menu buttons | grid at y = 135, four per row | 54 × 18 each |

## What exists today

| Area | What exists | Status |
|------|-------------|--------|
| `internal/game/ui/ui2d_backend.go:856` | `RenderInGameUI` draws the scene texture, the F3 debug window, an error window and a bottom status bar. No HUD. | ❌ missing |
| `internal/game/ui/backend.go:129` | `InGameUIState` **already declares** `PlayerHP/MaxHP`, `PlayerSP/MaxSP`, `PlayerLevel`, `PlayerJobLevel` | 🟡 declared |
| `internal/game/game.go:657` | builds `InGameUIState` — and **never sets any of those six fields**, so they are always zero | ❌ missing |
| `internal/network/packets/lengths.go:63` | `0x00B0` has a length (8) and nothing else: no struct, no handler, no constant | 🟡 stub |
| `internal/game/states/ingame.go:532` | handlers registered for entity/move/map packets only — no status packets | ❌ missing |
| `internal/game/states/state.go:60` | `Session.Char` carries name, class, HP/SP and stats from character select — the panel's initial values | ✅ done |
| `internal/engine/ui2d` | window chrome, 3-slice drawing, skinned buttons with hover shading, `DrawImageUV`, text with measured metrics — all from #82 | ✅ done |
| `internal/game/ui/window_skin.go` | `LoadNativeWindowFrame` + `TextureCache` — the pattern this panel's loader follows | ✅ done |
| `cmd/grfbrowser` | previews bitmaps already (`preview_image.go`); no new viewer needed for this feature | ✅ done |
| Tests | none for the HUD; `internal/network/packets/lengths_test.go:45` already asserts `0x00B0` is 8 bytes | 🟡 partial |
| Docs | no ADR or plan mentions the HUD | ❌ missing |

**In flight:** only dependabot PRs (#76, #48). Nothing touches these files.

## Reference implementations

| Source | Where | Approach |
|--------|-------|----------|
| roBrowser | `src/UI/Components/BasicInfo/BasicInfo.html`, `.css`, `.js` | Transcribes the original panel: absolute positions inside a 220×135 background, gauges built from `gzered_*`/`gzeblue_*` three-slice art over the trough painted in the background, and a reduced 53px form toggled with Ctrl+V. This is our measurement source. |
| korangar | `korangar/src/interface/` | Its own interface system, deliberately not the original look — useful for how state reaches the UI, not for layout. |
| midgarts | — | No HUD. |

## Assets

All confirmed present with `grftool search` against `data.grf`.

| Asset | GRF path | Size | Exists? |
|-------|----------|------|---------|
| Panel (large) | `…/basic_interface/basewin_bg2.bmp` | 220×135 | ✅ |
| Panel (reduced) | `…/basic_interface/basewin_mini.bmp` | — | ✅ |
| HP gauge | `…/basic_interface/gzered_left/mid/right.bmp` | 4×8, 1×8, 4×8 | ✅ |
| SP gauge | `…/basic_interface/gzeblue_left/mid/right.bmp` | 4×8, 1×8, 4×8 | ✅ |
| Gauge trough | `…/basic_interface/gze_bg_left/mid/right.bmp` | 9px tall | ✅ |
| Menu buttons (ten, three states each) | `…/basic_interface/{info,skill,item,map,party,guild,quest,option,booking,recruit}{1,2,3}.bmp` | 54×18 | ✅ |

The gauges are three-slice horizontally: fixed 4px caps and a 1px middle stretched to the fill width.

## Protocol

The server pushes each value as it changes; there is no "give me my stats" request.

| Packet | ID | Direction | Source |
|--------|----|-----------|--------|
| `ZC_PAR_CHANGE` | `0x00B0` — `<var id>.W <value>.L`, 8 bytes | S→C | `docker/rathena/build/rathena/src/map/clif.cpp:3547` |
| `ZC_LONGPAR_CHANGE` | `0x00B1` — same shape, for values that outgrow a short | S→C | `clif.cpp:3558` |

Var ids come from `enum _sp` in `docker/rathena/build/rathena/src/map/map.hpp:497`:

| Field | `var id` | | Field | `var id` |
|---|---|---|---|---|
| `SP_HP` | 5 | | `SP_BASELEVEL` | 11 |
| `SP_MAXHP` | 6 | | `SP_JOBLEVEL` | 55 |
| `SP_SP` | 7 | | `SP_BASEEXP` | 1 |
| `SP_MAXSP` | 8 | | `SP_NEXTBASEEXP` | 22 |
| `SP_ZENY` | 20 | | `SP_WEIGHT` / `SP_MAXWEIGHT` | 24 / 25 |

`clif_updatestatus` (`clif.cpp:3635`) is the dispatcher that decides which of the two packets carries a given parameter.

## Step 0 — Prerequisites & tooling

### 0a. Refactoring (only what this feature needs)

- [ ] `internal/game/ui/ui2d_backend.go` is ~950 lines and already holds login, char-select, connecting, loading and in-game rendering. The HUD goes in its own file (`hud_basic_info.go`) beside `charselect_native.go`, rather than growing it further. No interface changes, so **no ADR**.
- [ ] `internal/game/game.go:657` — the `InGameUIState` literal is where the six unset fields are filled; the player's live stats need somewhere to live first (Step 1).

### 0b. Debug tooling & tests

- [ ] Trace channel `status` in `internal/trace` — events: `status.change` (var id, value), `status.unknown` (an id we do not map, logged once per id).
- [ ] F3 overlay fields: `HP cur/max`, `SP cur/max`, `Base/Job Lv` — so a single screenshot proves whether the values arrived, independent of whether the panel draws them.
- [ ] Screenshot scenario: `go run ./cmd/client --config config.yaml --trace=status,net --screenshot-after 12s` → `latest.png` shows the panel top-left with non-zero HP/SP.
- [ ] Logs: an unmapped `var id` warns once; a missing panel bitmap warns and falls back to no HUD, never a swallowed error.
- [ ] Tests: `internal/network/packets/status_test.go` — table-driven round-trip of `0x00B0`/`0x00B1` from known bytes, including a var id we do not map; `internal/game/ui/hud_basic_info_test.go` — gauge fill width for 0%, 50%, 100% and the `max = 0` case.
- [ ] Use cases: UC-205 (panel renders with live values), UC-206 (HP gauge tracks damage), UC-305 (status packets parsed).

## Steps

### Step 1 — Parse status packets, verify the CharInfo layout, keep the player's stats
- **Changes:** `internal/network/packets` (constants + struct), `internal/game/states/ingame.go` (handler), `internal/game/game.go` (fill the six `InGameUIState` fields)
- **Done when:** `--trace=status` prints `status.change` with sensible ids and values on login and when taking damage; F3 shows HP/SP/levels matching the server
- **Proved by:** `go test ./internal/network/packets/` (round-trips), F3 screenshot, UC-305
- **Reference:** Protocol table above

  **Layout investigation (per review).** Character select reports `Job 40`,
  `Lv. 0` and `HP 11/11` next to `SP 40/40` for the test account. 40 is not a
  base job, and those three disagreeing is the signature of a struct read at the
  wrong offsets: `packets.go:151` defines `CharInfo` with `CharInfoSize = 175`
  (eAthena) while noting rAthena/Hercules uses 155, so the same bytes can be
  read two different ways. This step checks the layout against
  `docker/rathena/build/rathena/src/map/packets.hpp` for our PACKETVER and fixes
  the offsets before the HUD trusts them — a panel fed from a misread struct
  shows wrong numbers convincingly. The `0x00B0` values are the cross-check: if
  HP from the packet disagrees with HP from `CharInfo`, the struct is still
  wrong.

### Step 2 — Draw the panel with name, job and levels
- **Changes:** `internal/game/ui/hud_basic_info.go` (new), `internal/game/ui/ui2d_backend.go` (call it from `RenderInGameUI`)
- **Done when:** the panel is in the top-left with the character's name, job and Base/Job level; magenta edges are keyed out
- **Proved by:** screenshot scenario, UC-205
- **Reference:** grf-basewin-bg2 ①②④

### Step 3 — HP and SP gauges
- **Changes:** `internal/game/ui/hud_basic_info.go`
- **Done when:** both gauges fill in proportion with `current / max` printed on them and the percentage to the right; a gauge with `max = 0` draws empty rather than dividing by zero
- **Proved by:** `go test ./internal/game/ui/` (fill widths), UC-206 — sit to regenerate and watch the bar move
- **Reference:** grf-basewin-bg2 ③

### Step 4 — Experience bars, weight and Zeny, and the menu grid
- **Changes:** `internal/game/ui/hud_basic_info.go`
- **Done when:** Base/Job EXP bars fill against the next-level values; `Weight: cur/max` and `Zeny: n` read the server's values; the ten 54×18 buttons draw in a four-per-row grid using their own hover and pressed art
- **Proved by:** screenshot scenario; buttons are drawn but inert (see Out of scope)
- **Reference:** ref screenshot ⑩⑪⑫, grf-basewin-bg2 ④⑤, grf-btn-info1 ⑥

### Step 5 — Reduced form and dragging
- **Changes:** `internal/game/ui/hud_basic_info.go`, input handling for Ctrl+V
- **Done when:** Ctrl+V switches between the 220×135 and 220×53 forms, and the panel can be dragged by its title bar and stays where it is put
- **Proved by:** screenshot scenario in both forms, UC-205
- **Reference:** `basewin_mini.bmp`

### Step 6 — Docs
- [ ] `docs/ENGINE_FEATURES.md` — the HUD entry
- [ ] Session log `docs/sessions/2026-08-26-hud-basic-info.md`

## Done when (feature)

- The Basic Info panel is in the top-left in game, drawn from the original bitmaps.
- Name, job, Base level and Job level are the character's real values.
- HP and SP gauges fill proportionally, show `current / max` and a percentage, and follow the server as they change.
- Base/Job EXP bars fill against the next-level thresholds.
- Ctrl+V switches to the reduced panel and back; the panel drags and stays put.
- Nothing regresses when the archive lacks a bitmap: the HUD is skipped with a warning and the game still runs.

## Out of scope

- The menu buttons **open nothing** — inventory, skills, map, options and guild windows are their own features.
- Nothing about the minimap, the chat box or entity HP bars, all visible in the reference but each its own feature.
- Party/guild HP bars, and HP bars over other entities.
- The status window (STR/AGI/VIT/INT/DEX/LUK) — a separate window, despite arriving in the same packets.

## Open questions

_All three from the first round are answered — see the Revision log. One follow-up:_

1. **The reference image lives on the issue, not in the repo.** It was supplied
   in review rather than fetched (ResearchGate blocks automated download), so
   `docs/features/hud-basic-info/` holds only the client's own bitmaps. Attach
   the PNG to this issue if you want it in the permanent record, and I will
   commit it as `ref-01` with the legend above.

## Revision log

- 2026-08-26 — created
- 2026-08-26 — real-client screenshot supplied in review: the menu buttons are a
  grid of ten (three states each, all present in the archive), not roBrowser's
  row of five; weight and Zeny move into scope since they arrive as status vars
  (`SP_ZENY` 20, `SP_WEIGHT` 24, `SP_MAXWEIGHT` 25) rather than needing
  inventory; the panel has a minimize button and no close button.
- 2026-08-26 — Step 1 now also verifies the `CharInfo` layout against rAthena
  before the HUD trusts its values (per comment 2). Job names stay as they are;
  `Unknown (N)` is acceptable (per comment 3). The reference screenshot is still
  outstanding: the linked ResearchGate image is not fetchable from here
  (per comment 1).
