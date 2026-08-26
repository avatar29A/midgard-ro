# UC-205: Basic Info panel renders with the character's values

## Description
The original client's Basic Info panel is drawn in the top-left in game, showing
the character's name, job and levels, and can be reduced and moved.

## Preconditions
- rAthena is running (`make server-up`) and the test account can reach Prontera
- `config.yaml` points at a `data.grf` containing
  `data/texture/유저인터페이스/basic_interface/basewin_bg2.bmp`
- A character exists on the account

## Test Steps
1. `go run ./cmd/client --config config.yaml --screenshot-after 12s`
2. Wait for the map to load and the screenshot to be written
3. Press Ctrl+V, then F12
4. Drag the panel by its title bar to the middle of the screen, then press F12
5. Press Ctrl+V again, then F12

## Expected Results
- Step 2: `data/Screenshots/latest.png` shows the 220x135 panel in the top-left
  with no magenta edges; name and job read as the selected character's; Base Lv.
  and Job Lv. show the server's values, not zero
- Step 3: the panel is replaced by the reduced 220x53 form
- Step 4: the panel stays where it was dropped and does not snap back on the
  next frame
- Step 5: the panel returns to the large form, in the position it was dragged to
- No `basic info` warning appears in the log

## Priority
High

## Related
- PRD Section: docs/prd/PRD.md - UI
- Feature: docs/features/hud-basic-info/README.md
- Code: internal/game/ui/hud_basic_info.go
- Test: internal/game/ui/hud_basic_info_test.go
