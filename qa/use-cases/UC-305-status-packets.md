# UC-305: Character status packets are parsed

## Description
ZC_PAR_CHANGE (0x00B0) and ZC_LONGPAR_CHANGE (0x00B1) are decoded and update the
player's stats, including var ids the client does not map.

## Preconditions
- rAthena is running (`make server-up`)
- The client can log in and enter a map

## Test Steps
1. `go test ./internal/network/packets/`
2. `go run ./cmd/client --config config.yaml --trace=status,net`
3. Enter the map and watch the trace for the login burst of status packets
4. Take damage or gain experience and watch for further events

## Expected Results
- Step 1: round-trip tests pass for both packets, decoding known bytes into the
  documented `<var id>.W <value>.L` shape, and an unmapped var id is reported
  rather than silently dropped
- Step 3: `status.change` events carry ids from rAthena's `enum _sp` — 5 (HP),
  6 (MaxHP), 7 (SP), 8 (MaxSP), 11 (BaseLevel), 55 (JobLevel)
- Step 4: further `status.change` events arrive as values change
- Any id the client does not map logs `status.unknown` once, not every packet

## Priority
High

## Related
- PRD Section: docs/prd/PRD.md - Network
- Feature: docs/features/hud-basic-info/README.md
- Code: internal/network/packets, internal/game/states/ingame.go
- Test: internal/network/packets/status_test.go
