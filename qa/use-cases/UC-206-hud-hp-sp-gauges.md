# UC-206: HP and SP gauges follow the server

## Description
The HP and SP gauges fill in proportion to the character's current and maximum
values and update as those change, rather than being drawn once at spawn.

## Preconditions
- UC-205 passes
- A monster is reachable on the map, or a GM command is available to change HP

## Test Steps
1. `go run ./cmd/client --config config.yaml --trace=status --screenshot-after 12s`
2. Read the HP and SP readouts on the panel and the F3 overlay
3. Take damage (attack a monster and let it hit back), then press F12
4. Sit until HP regenerates, then press F12
5. Inspect the trace output for `status.change` events

## Expected Results
- Step 2: both gauges are filled proportionally, each printing `current / max`
  over the bar and a percentage to its right; the panel and F3 agree
- Step 3: the red gauge is visibly shorter and the printed numbers are lower
- Step 4: the gauge grows back toward full
- Step 5: `status.change` events appear with var ids 5/6 (HP/MaxHP) and 7/8
  (SP/MaxSP); no `status.unknown` for those ids
- A character with `max = 0` (should not occur in play) draws an empty gauge
  rather than crashing — covered by the unit test

## Priority
High

## Related
- PRD Section: docs/prd/PRD.md - UI
- Feature: docs/features/hud-basic-info/README.md
- Code: internal/game/ui/hud_basic_info.go, internal/network/packets
- Test: internal/game/ui/hud_basic_info_test.go
