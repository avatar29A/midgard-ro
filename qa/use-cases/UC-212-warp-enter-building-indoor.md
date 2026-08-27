# UC-212: Enter and leave a building (indoor map)

## Description
The player walks into a Prontera building door and is inside `prt_in` at once — no loading screen — with the camera locked the way the original client locks it on indoor maps, then walks back out. Covers the cached/preloaded map switch and the `indoorrswtable.txt` camera rules.

## Preconditions
- As UC-211; character in `prontera` near a building door, e.g. `174,218` (door warp `prt05` is at `177,221`, trigger box 5×5)
- Client started with `--trace=map`

## Test Steps
1. Log in and enter Prontera; wait for the `map.preload` trace line for `prt_in`
2. Walk onto `177,221`
3. Once inside, right-drag to rotate the camera and scroll the wheel to zoom out fully
4. Walk onto the return warp (`prt_in 168,124`)
5. Repeat step 2 immediately after logging in, before any preload has finished

## Expected Results
- Step 2: the view cuts from the street to the room in a single frame (trace `map.cache hit`, switch time < 50 ms); no loading screen, no progress bar
- The character stands at `prt_in 168,128`; the room's NPCs appear within a second
- Step 3: the camera **does not rotate** (indoor maps disable orbital rotation); zooming out stops at the indoor limit, so the room's outside walls and the void beyond them never come into view; the F3 overlay shows `Indoor: yes` and the active camera limits
- Step 4: the character is back on the Prontera street at `174,218`, camera rotation works again
- Step 5: with nothing cached the ordinary loading screen appears (as UC-211) — the feature degrades to the original's behaviour rather than freezing

## Priority
High

## Related
- Feature: docs/features/warp-portals/README.md
- Code: `internal/engine/camera/camera.go` (indoor limits), `internal/game/states/` (map cache), `internal/assets` (`indoorrswtable.txt`)
- Data: `data/indoorrswtable.txt` line 144 (`prt_in.rsw#`)
