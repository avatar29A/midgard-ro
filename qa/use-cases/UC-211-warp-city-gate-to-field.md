# UC-211: Walk through a city gate into a field map

## Description
The player walks onto Prontera's south gate warp and arrives in `prt_fild08` after a loading screen, with the field's NPCs and monsters present. Covers the server-driven map change (`ZC_NPCACK_MAPMOVE`), the loading screen with real progress, the `CZ_NOTIFY_ACTORINIT` handshake and the entity refresh.

## Preconditions
- Local rAthena stack running (`make server-up`); account `midgard-test` with one character
- Character standing in `prontera` near the south gate, e.g. `156,30` (`--autologin` from `config.yaml`; position can be set with `UPDATE \`char\` SET last_map='prontera', last_x=156, last_y=30` while offline)
- `vsync: false` in the local `config.yaml` for unattended runs
- Client started with `--trace=map,net`

## Test Steps
1. Log in and enter Prontera; note the F3 overlay shows `Map: prontera.gat`
2. Click the ground just past the gate, e.g. tile `156,22`, and let the character walk onto it
3. Observe the screen while the map changes
4. Wait until the field is visible and walk a few tiles
5. Read the `map` trace in the log
6. **Any map:** log out; set `last_map`/`last_x`/`last_y` in the DB to `geffen 119,59`, then `morocc 156,93`, then `prt_castle 102,20`; log in each time

## Expected Results
- The moment the character reaches the gate, movement stops and the **loading screen** appears: one of `loading01..10.jpg` filling the window, a progress bar that grows in steps, no black frame with the old map behind it
- The loading screen stays up until `prt_fild08` is ready; the client sends `0x007D` only after the map has loaded (trace: `map.change` → `map.load.phase` × N → `map.ready` → `net.recv 0x09FF`…)
- The character stands at `prt_fild08 170,375` facing away from the gate; no residue of Prontera's units (F3 `Entities:` restarts from 0 and grows as spawns arrive)
- Field NPCs and monsters appear within a second of the map showing; clicking the ground walks normally
- The BGM changes to the field's track; the NPC dialog window (if it was open) is closed
- Walking back onto `prt_fild08 170,378` returns to `prontera 156,26` the same way
- Step 6: each map loads through the same loading screen and lands the character at the given tile; `prt_castle` reports `Indoor: yes`, the other two `Indoor: no`; no map-specific code path is involved

## Priority
Critical

## Related
- Feature: docs/features/warp-portals/README.md
- Code: `internal/game/states/loading.go`, `internal/game/states/ingame.go` (`handleMapChange`), `internal/network/packets/`
- Server: `docker/rathena/build/rathena/npc/warps/cities/prontera.txt:24` (`prt001`), `npc/re/warps/fields/prontera_fild.txt:69` (`prtf004`)
