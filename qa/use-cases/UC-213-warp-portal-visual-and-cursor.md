# UC-213: Warp portals look like portals, and the cursor says so

## Description
Every server warp (NPC class 45) is drawn as the original's blue swirling portal; hovering it shows the door cursor; clicking it walks onto it rather than "talking" to it; hidden warps (class 139) are not drawn at all.

## Preconditions
- As UC-211; character in `prontera` at `136,219` (beside door warp `prt04` at `134,221`)
- Client started with `--trace=render,npc`

## Test Steps
1. Log in and look at the door two tiles north-west
2. Hover the mouse over the portal
3. Move the mouse onto an ordinary NPC, then back onto the portal
4. Click the portal
5. Press F12 with the cursor over the portal

## Expected Results
- Step 1: a translucent blue column of rising light spikes stands in the doorway, slowly rotating, with a soft blue disc on the ground; **no** `1_ETC_01` NPC sprite, no name label, no shadow; the `render` trace shows the warp drawn as an effect, not a sprite sheet
- Step 2: the cursor becomes the animated **door** (`cursors.act` action 7, ~10 frames looping at 100 ms)
- Step 3: talk cursor over the NPC, door cursor over the portal, default cursor over the ground — each switch immediate
- Step 4: the character **walks** to the portal's tile and is warped (UC-212); no `CZ_CONTACTNPC` is sent (`npc` trace shows no `npc.contact`)
- Step 5: `latest.png` shows the portal and the door cursor together
- Standing at `prontera 156,191` (fountain) nothing is drawn for warps that are hidden (class 139) — none in view, none in the entity count

## Priority
High

## Related
- Feature: docs/features/warp-portals/README.md
- Code: `internal/game/states/entities.go`, `internal/engine/cursor/cursor.go` (`StateWarp`), `internal/game/game.go` (`updateCursor`), `internal/engine/scene/` (portal effect)
- Data: `data/texture/effect/ring_blue.tga`, `data/sprite/cursors.spr`/`.act`
