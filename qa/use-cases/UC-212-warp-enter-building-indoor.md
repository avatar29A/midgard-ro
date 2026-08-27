# UC-212: Enter and leave a building (indoor map)

## Description
The player walks into a Prontera building door, sees the loading screen for the moment `prt_in` takes to load — the original client's behaviour — and stands inside with the original's indoor camera: no orbiting, and black beyond the walls rather than water. Then walks back out. Covers the `indoorrswtable.txt` / `viewpointtable.txt` camera rules and the per-cell water mesh.

## Preconditions
- As UC-211; character in `prontera` near a building door, e.g. `174,218` (door warp `prt05` is at `177,221`, trigger box 5×5)
- Client started with `--trace=map`

## Test Steps
1. Log in and enter Prontera
2. Walk onto `177,221`
3. Once inside, right-drag to rotate the camera and scroll the wheel to zoom out fully
4. Walk onto the return warp (`prt_in 168,124`)
5. Walk to the fountain (`156,191`) and look at the water

## Expected Results
- Step 2: the loading screen appears as in UC-211, for well under a second, then the room: the character stands at `prt_in 168,128` and the room's NPCs appear within a second
- Step 3: the camera **does not rotate** (indoor maps disable orbital rotation); zoom stops at the table's normal range; **the space between the rooms is black — no water plane**; the F3 overlay shows `Indoor: yes`, the active camera limits, and `Water: 0 cells`
- Step 4: the character is back on the Prontera street at `174,218`; camera rotation works again; F3 shows `Indoor: no`
- Step 5: the fountain's water still renders and animates — outdoor water is unchanged by the per-cell mesh

## Priority
High

## Related
- Feature: docs/features/warp-portals/README.md
- Code: `internal/engine/camera/camera.go` (indoor limits), `internal/assets` (`indoorrswtable.txt`, `viewpointtable.txt`), `internal/engine/water/water.go`, `internal/engine/scene/water_renderer.go`
- Data: `data/indoorrswtable.txt` line 144 (`prt_in.rsw#`); roBrowser `src/Loaders/Ground.js:471-483` (the water rule)
