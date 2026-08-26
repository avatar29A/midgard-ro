# Feature: NPC dialog and interaction

**Branch:** `feature/npc-dialog` · **Issue:** #86 · **Parent:** #49 (MVP scope), #54 (Track E, task **E3**)
**Status:** Planned · **Created:** 2026-08-26

## Goal

Click a Prontera NPC and talk to it: the Kafra greets you, the Guide offers a list
of places, `Next` advances the script, a menu choice branches it, and `Close`
ends it cleanly. This completes **Track E3** of #54 — E1 (entity system) and E2
(spawn/move/vanish packets) landed in PR #80, and everything that made those
units appear is now waiting on a way to interact with them.

Advances the MVP's **NPCs** item. Quests, shops and storage are all built on this
same conversation flow, so this is the piece they wait on — but none of them are
in this feature.

## The one thing to know first

NPC behaviour is **server-side**. rAthena runs the scripts (`docker/rathena/build/rathena/npc/**/*.txt`);
the client never executes anything. There is no Lua interpreter to write and no
quest logic to port. What the client owes is a **conversation state machine plus
a dialog window**, driven by seven packets. That is the whole feature.

## Reference (original client)

### Buttons — extracted from `data.grf`

![grf-btn-close — the Close button, three-state sheet, basic_interface/btn_close.bmp](./grf-btn-close.png)
![grf-btn-next — the Next button, login_interface/btn_next.bmp](./grf-btn-next.png)

1. **Close button** — `basic_interface/btn_close.bmp`, **42×20**, with `_a` (hover)
   and `_b` (pressed) siblings. Ends the conversation.
2. **Next button** — `btn_next.bmp`, **42×20**, but it lives in **`login_interface/`**,
   not `basic_interface/`, and ships **no `_a`/`_b` hover art**. Main already
   handles that case (`1a3bda6 feat(ui): shade skin buttons that ship no hover art`).

Both measure 42×20 in the archive, which is exactly what roBrowser's stylesheet
declares (`NpcBox.css:5` — `width:42px; height:20px`). Two independent sources
agreeing is the confirmation that this is the original size.

### Layout — measured from roBrowser

roBrowser transcribes the original window pixel-for-pixel, so its CSS is a
measurement source (the approach PR #81 used for the login window):

| Element | Size | Source |
|---------|------|--------|
| Dialog window | 276 × 176 | `NpcBox.css:1` |
| Inner border | 264 × 164, 1px `#c1c6c2` | `NpcBox.css:2` |
| Text area | 254 × 130, `#eff4f0`, `pre-wrap`, scrolls vertically | `NpcBox.css:3` |
| Buttons | 42 × 20 | `NpcBox.css:5` |
| Menu window | 276 × 116, list rows 260 wide | `NpcMenu.css:4-33` |

The menu's OK/cancel buttons and its scrollbar are visible in the captures above
but are not in this table — `NpcMenu.css` will need re-reading for their
geometry when Step 5 gets there.

Two behaviours fall out of that CSS and are easy to miss:

3. **`white-space: pre-wrap`** — NPC text carries its own line breaks and they
   are significant. Re-flowing the text as one paragraph would mangle every
   script that formats a list.
4. **`overflow-y: auto`** — the text area scrolls. A long message does not grow
   the window.

### Real client, supplied in review

![ref-01 — [Kris]: the text window and the menu window side by side](./ref-01-text-and-menu.png)

**ref-01 — text and menu together.** `[Kris]` on the first line of the text
window, the message below it, and a separate menu window to its right with
`Yes.` / `Nah, I'm a pro~` / `Cancel.`, the first row filled edge to edge with a
blue highlight, and **OK** / **cancel** at its bottom right. The text window
shows no button of its own while the menu is up.

![ref-02 — [Shopkeeper]: the Next button in place, and colored text in the message](./ref-02-next-button.jpg)

**ref-02 — the Next button in situ.** `next`, lowercase, at the **bottom right
inside the text window**. Also the first sight of **colored text in an NPC
message**: `Clana Nemieri` is blue while the rest of the line is black.

![ref-03 — Kafra Employee, the whole screen](./ref-03-kafra-fullscreen.jpg)

**ref-03 — a whole screen.** Text window upper right, menu window below and left
of it (`Save`, `Use Storage`, `Use Teleport Service`, `Rent a Pushcart`), both
floating over the map beside the chat box and the minimap.

Two more captures were supplied in chat but are not files, so they are not in
the repo: a `[Genius Skyler]` dialog with the menu *overlapping* the text window
rather than beside it, and a warp list — `Last Warp [pay_dun01]`, `~ Towns`,
`~ Fields`, `~ Dungeons` — with a **vertical scrollbar and arrow buttons** down
the right edge. The scrollbar claim in ⑩ rests on that second one.

#### Measured against roBrowser

ref-01 is a lossless capture at native size — its 1px borders are single crisp
pixels, not resampled — so it can be measured rather than eyeballed:

| | Measured in ref-01 | roBrowser |
|---|---|---|
| Text window, outer | 278 × 178 | 276 × 176 |
| Menu window, outer width | 278 | 276 |
| Text area | 260 wide, rows 131–270 | 254 × 130 |
| Border | 1px `#c5c5c5` | `#c1c6c2` |
| Text area fill | `#f7f7f7` | `#eff4f0` |

**roBrowser is confirmed.** Its 276 × 176 is a CSS content box; add the 1px
border it also declares and you get the 278 × 178 measured here, in both
dimensions and for both windows. The two colors differ in the last step or two
of each channel — the measured ones are what the original actually draws, so
prefer them.

What they establish, in the order it matters:

7. **Neither window has a title bar.** Both are plain light panels: a thin grey
   border, softly rounded corners, a slight drop shadow, and nothing else. This
   contradicts the "reuse `win_msgbox.bmp` — no new chrome" line in the table
   below: that bitmap nine-slices with a 24px gradient title bar, which these
   windows do not have. roBrowser agrees — its `NpcBox.css` is a 1px `#c1c6c2`
   border around an `#eff4f0` fill, a plain box. **Step 3 must not open a titled
   window.**
8. **The NPC's name is part of the message,** not a window title —
   `[Genius Skyler]`, `[Kafra Employee]`, written by the script as its first
   line. Nothing parses a name out of anything; the bracket text is just text.
   This follows from ⑦ and is why it can.
9. **The menu is a window with OK and cancel,** not a list that acts on a
   click. A row is *selected* — full-width blue highlight, first row by default
   — and **OK** confirms it. That is two steps, and the plan described one.
10. **A long list scrolls,** with a real scrollbar and arrow buttons (ref-02).
    The menu window does not grow to fit its items.
11. **Text and menu are shown at the same time,** as two separate windows, and
    the text window keeps its content while the menu is up. In both captures
    that show a menu, the text window shows **no Next or Close button** — which
    matches roBrowser revealing those only when the server asks for them.

Not established, so not assumed: whether double-clicking a row also confirms it
(likely — it is the convention, and `ui2d.DoubleClickedIn` now exists — but no
capture shows it), the exact fonts and padding, and where the windows open. The
three captures disagree on position, which is itself the answer to Open
question 2: it is not a fixed spot.

### Current state (ours)

![current-npcs — Prontera fountain, our client, 17 units tracked and 12 drawn](./current-npcs.jpg)

5. **NPCs render and stand on shadows** — 17 tracked, 12 drawn (the 5 undrawable
   are guild flags, job 722, which are `.gr2` 3D models rather than sprites).
6. **Nothing is clickable.** A click anywhere, including squarely on an NPC,
   is a move request.

## What exists today

| Area | What exists | Status |
|------|-------------|--------|
| `internal/game/entity/entity.go` | `Entity` (id, name, job, type, `Body`), `Manager` keyed by AID | ✅ from #80 |
| `internal/game/states/entities.go` | `upsertUnit`, `removeUnit`, `unitSpec`, fade in/out | ✅ from #80 |
| `internal/engine/picking/ray.go:25` | `ScreenToRay` | ✅ used for click-to-move |
| `internal/engine/picking/ray.go:90` | `IntersectAABB` | 🟡 **exists, unused** — this is the seam for entity picking |
| `internal/game/game.go:883` | click → `ScreenToTile` → `RequestMove` | 🟡 needs to try NPCs first |
| `internal/engine/ui2d/context.go:154` | `BeginWindowEx`, draggable, nine-sliced | ✅ |
| `internal/engine/ui2d/context.go:317,339,618` | `Button`, `Label`, `Selectable` | ✅ |
| `internal/game/ui/window_skin.go:48` | `win_msgbox.bmp` frame already loaded | ❌ **not this window** — it has a title bar and the NPC dialog does not (see ⑦) |
| `internal/engine/ui2d/context.go` | `CaptureMouse` / `MouseCaptured` | ✅ from #87 — this is most of Step 0a |
| `internal/engine/ui2d/context.go` | `DoubleClickedIn` | ✅ from #87 — for confirming a menu row |
| `internal/engine/ui2d/widgets_at.go` | `ButtonOptions{Silent}` | ✅ from #87 |
| `internal/network/packets/` | none of the seven dialog packets | ❌ |
| `internal/game/states/ingame.go:531` | `registerPacketHandlers` — entity handlers only | 🟡 extend |
| Tests | entity/packet tests from #80; nothing for dialog | ❌ |

**In flight:** nothing. `git worktree list` shows `midgard-ro-audio` (merged) and
`midgard-ro-entities` (merged as #80). No open PR touches these files.

## Reference implementations

| Source | Where | Approach |
|--------|-------|----------|
| roBrowser | `src/UI/Components/NpcBox/`, `NpcMenu/` | Two separate windows. `NpcBox` shows the text and reveals a `Next` or `Close` button as the server asks for one (`NpcBox.js:148,160`); `NpcMenu` is a list whose selection sends `index + 1`, and cancel sends `255` (`NpcMenu.js:186,195`). The window is not recreated per message — text accumulates in one scrolling box. |
| korangar | `korangar/src/interface/windows/dialog.rs` | Same flow in a modern immediate-mode UI; useful as a structural check, but its look is deliberately not the original. |

They agree on the flow. We follow roBrowser for layout because it is a
transcription of the original; korangar only as a sanity check on packet handling.

## Assets

| Asset | GRF path | Exists? |
|-------|----------|---------|
| Close button | `data/texture/유저인터페이스/basic_interface/btn_close.bmp` (+`_a`,`_b`) | ✅ 42×20, all three states |
| Next button | `data/texture/유저인터페이스/login_interface/btn_next.bmp` | ✅ 42×20, **no hover states, different folder** |
| Window frame | `data/texture/유저인터페이스/win_msgbox.bmp` | ✅ already loaded by `window_skin.go:48` |

The `dialog_*.bmp` family in `basic_interface` (600×24 strips) is the **chat bar**,
not this window — checked and rejected, so nobody re-checks it later.

## Protocol

All IDs and offsets derived from the server's own headers at PACKETVER 20211103
via `tools/packetlen/layout.py`, not from a wiki.

| Packet | ID | Dir | Layout | Source |
|--------|----|-----|--------|--------|
| `CZ_CONTACTNPC` | `0x0090` | C→S | `type(2) AID(4) kind(1)` = 7 | `packets_struct.hpp:5412` |
| `ZC_SAY_DIALOG` | `0x00B4` | S→C | `type(2) len(2) npcId(4) message[]` | `packets_struct.hpp:5486` |
| `ZC_WAIT_DIALOG` | `0x00B5` | S→C | `type(2) npcId(4)` = 6 — show **Next** | `clif_packetdb.hpp` |
| `ZC_CLOSE_DIALOG` | `0x00B6` | S→C | `type(2) npcId(4)` = 6 — show **Close** | `clif_packetdb.hpp` |
| `ZC_MENU_LIST` | `0x00B7` | S→C | `type(2) len(2) npcId(4) menu[]` | `packets_struct.hpp` |
| `CZ_CHOOSE_MENU` | `0x00B8` | C→S | `type(2) npcId(4) select(1)` = 7 | `clif_packetdb.hpp:62` |
| `CZ_REQ_NEXT_SCRIPT` | `0x00B9` | C→S | `type(2) npcId(4)` = 6 | `clif_packetdb.hpp:63` |
| `CZ_CLOSE_DIALOG` | `0x0146` | C→S | `type(2) npcId(4)` = 6 | `clif_packetdb.hpp:136` |

`0x00B8` and `0x00B9` are declared with `parseable_packet(...)`, which is why they
are **absent from `internal/network/packets/lengths.go`** — that table is generated
from server→client packets only. Outgoing packets are built by hand, so this
costs nothing, but do not expect `packets.Length()` to know them.

### Three landmines

1. **An invalid menu index disconnects you.** `clif_parse_NpcSelectMenu`
   (`clif.cpp`) calls `clif_GM_kick` when `select == 0` or `select > npc_menu`.
   Valid values are **1..n**, plus **255 for cancel**. A zero-based index is not
   a display bug — it is a kick.
2. **The menu is one colon-separated string.** `clif_scriptmenu` copies the
   script's text verbatim (`safestrncpy(packet->menu, mes, ...)`), NUL-terminated.
   The client splits on `:`, and the item's position in that list — 1-based — is
   what goes back in `CZ_CHOOSE_MENU`.
3. **NPC text carries color codes.** ref-02 shows `Clana Nemieri` in blue
   inside an otherwise black line. Scripts write `^RRGGBB` inline — `^0000FF`
   to set, `^000000` to return to black. They are not stripped by the server:
   whatever the script wrote arrives verbatim. Rendering the string as-is would
   print `^0000FFClana Nemieri^000000` on screen. At minimum they must be
   parsed out of the text; drawing them in color is a small extra step once the
   parser exists, since the text is already drawn run by run.
4. **The talking NPC may not be an entity we know.** `clif_scriptmenu` calls
   `clif_sendfakenpc` when the npcId is not a real unit near the player. The
   dialog must key off the id in the packet and never assume `Manager.Get(npcId)`
   returns anything.

## Step 0 — Prerequisites & tooling

### 0a. Refactoring

- [ ] `internal/game/game.go` — the click handler goes straight from
      `ScreenToTile` to `RequestMove`, with no seam for "did this click hit
      something". Extract the decision so a click can be offered to entities
      first and fall through to movement. Minimal change; no new package.
      **Smaller than planned since #87**: click-to-move is already gated on
      `uiBackend.MouseCaptured()`, so the dialog windows only have to claim
      their own rects — what is left is the entity hit test itself.
- [ ] No ADR. This adds a state machine inside an existing package and a window
      to an existing UI layer — it crosses no layer boundary in `CLAUDE.md`.

### 0b. Debug tooling & tests

- [ ] **Trace channel `npc`** in `internal/trace` — events: `npc.click`
      (aid, name, screen xy), `npc.contact` (aid sent), `npc.say` (npcId, bytes,
      text length), `npc.menu` (npcId, item count), `npc.choose` (npcId, index),
      `npc.close`. The `net` channel already shows the raw packets; this one
      shows the conversation as a sequence, which is what makes a stuck dialog
      readable.
- [ ] **F3 overlay fields:** talking-to (npc id + name or `—`), dialog state
      (`idle` / `text` / `waiting-next` / `menu`), last menu size. A stuck
      conversation should be diagnosable from a single screenshot.
- [ ] **Screenshot scenario:** `./build/midgard --autologin --trace=npc,net --screenshot-after 20s`
      standing at the Prontera fountain (`prontera 156,191`). `latest.png` must
      show the dialog window over the map with the Kafra's greeting in it.
- [ ] **Logs:** a menu index outside 1..n must be refused client-side and logged
      at **warn** with the index and the item count — never sent. Getting kicked
      by our own packet is the failure this prevents.
- [ ] **Tests:** `internal/network/packets/npc_test.go` — round-trip each packet
      against bytes written by hand from the layout table, and a menu-splitting
      table (empty, one item, trailing colon, embedded newlines, 255-cancel).
      `internal/game/states/npcdialog_test.go` — the state machine, driven by
      decoded packets with no GL or network.
- [ ] **Use cases:** UC-205 (talk and close), UC-206 (menu choice), UC-207
      (cancel and out-of-range refusal).

## Steps

### Step 1 — Decode the dialog packets
- **Changes:** `internal/network/packets/npc.go`, `npc_test.go`
- **Done when:** all seven ids are constants with encoders/decoders; the menu
  string splits into items; index 0 and >n are rejected by the encoder.
- **Proved by:** `go test ./internal/network/packets/` — round-trips against
  hand-written bytes, and a table for menu splitting.

### Step 2 — Click an NPC and tell the server
- **Changes:** `internal/engine/picking`, `internal/game/states/ingame.go`, `internal/game/game.go`
- **Done when:** clicking an NPC sends `CZ_CONTACTNPC` and does **not** walk you
  there; clicking the ground still walks.
- **Proved by:** `--trace=npc,net` shows `npc.click` → `npc.contact` → a
  `ZC_SAY_DIALOG` arriving. No `net.send` of a move request in the same frame.
- **Reference:** current-npcs ⑥

### Step 3 — Show what the NPC says
- **Changes:** `internal/game/states/npcdialog.go`, `internal/game/ui/`
- **Done when:** the greeting appears in a window over the map, with its own
  line breaks intact, scrolling when longer than the box, and `^RRGGBB` codes
  parsed out rather than printed.
- **Proved by:** screenshot scenario; UC-205; a table test for the color-code
  parser (no codes, one code, unterminated code, a code at the very end).
- **Reference:** ref-01 ⑦⑧, ref-02, measurements above
- **Chrome:** a plain panel — 278 × 178, 1px `#c5c5c5` border, `#f7f7f7` fill —
  **not** `BeginWindow`, which would give it a title bar it does not have.

### Step 4 — Next and Close
- **Changes:** `internal/game/states/npcdialog.go`, `internal/game/ui/`
- **Done when:** `ZC_WAIT_DIALOG` shows a Next button that advances the script;
  `ZC_CLOSE_DIALOG` shows Close, which ends the conversation and returns control.
- **Proved by:** UC-205 end to end against Prontera's Guide; `npc` trace shows
  the full sequence and a clean `npc.close`.
- **Reference:** grf-btn-close ①, grf-btn-next ②, ref-02 for placement — the
  button sits at the bottom right *inside* the text window.

### Step 5 — Menus
- **Changes:** `internal/game/states/npcdialog.go`, `internal/game/ui/`
- **Done when:** `ZC_MENU_LIST` shows the choices; picking one sends its 1-based
  index and the script branches; cancel sends 255.
- **Proved by:** UC-206, UC-207. Talking to the Guide and choosing a destination
  branches correctly, and cancel closes without a kick.
- **Reference:** roBrowser `NpcMenu.css` measurements

### Step 6 — Docs
- [ ] `docs/ENGINE_FEATURES.md` if a package was added
- [ ] Session log `docs/sessions/2026-08-DD-npc-dialog.md`
- [ ] Close #54's **E3** checklist items

## Done when (feature)

- Clicking Prontera's Kafra opens her dialog with her greeting
- `Next` advances a multi-page script; `Close` ends it and gives back control
- The Guide's destination menu appears, a choice branches the script, cancel backs out
- Clicking the ground still walks; clicking an NPC never does both
- No disconnect from any menu interaction, including cancel and rapid clicking
- `--trace=npc` reads as a conversation from click to close

## Out of scope

- Shops, storage and the Kafra's warp menu as **custom windows** — they arrive
  through the generic dialog and will look generic until they get their own feature
- Quest log, dialogue history, NPC name tooltips on hover
- Monster clicking and combat (Track F)
- `SECURE_NPCTIMEOUT` — rAthena can time a dialog out server-side; not enabled
  in our config, and not handled here

## Open questions

1. ~~No original screenshot of the dialog in situ.~~ **Answered and
   committed** — `ref-01`…`ref-03`, and roBrowser's measurements confirmed
   against them.
2. ~~Where should the window open?~~ **Partly answered** — the three captures
   disagree on position, so it is not a fixed spot. Remaining question: should
   it be remembered between conversations, the way camera zoom is?
3. **Text encoding.** rAthena's English scripts are ASCII, so this does not bite
   today, but Korean scripts would arrive as EUC-KR and our font atlas is built
   from the ASCII range. Worth confirming we only care about English scripts.

## Investigation notes

- The `dialog_*.bmp` family (600×24) is the chat input bar, not this window.
- `basic_interface` and `login_interface` are separate folders in the archive and
  the two buttons this feature needs live in different ones.
- `internal/network/packets/lengths.go` covers server→client packets only, because
  the generator reads `packet(...)` declarations and the client→server ones are
  declared with `parseable_packet(...)`.

## Revision log

- 2026-08-26 — three captures committed as `ref-01`…`ref-03`. Measuring the
  lossless one confirms roBrowser to the pixel (276 × 176 content box + 1px
  border = the 278 × 178 measured). Two findings the earlier round missed: the
  Next button sits bottom right *inside* the text window, and NPC text carries
  `^RRGGBB` color codes that would otherwise print literally.
- 2026-08-26 — rebased onto `main` after #87 merged, which brings
  `CaptureMouse`/`MouseCaptured`, `DoubleClickedIn` and `ButtonOptions` — all of
  which this feature wants. Three real-client captures supplied in review:
  neither window has a title bar (so `win_msgbox.bmp` is the wrong chrome), the
  NPC's name is part of the message text, and the menu is a window with OK,
  cancel and a scrollbar rather than a click-to-choose list.

- 2026-08-26 — created
