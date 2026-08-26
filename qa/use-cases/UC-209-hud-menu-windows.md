# UC-209: HUD — The Four Menu Windows

## Description
The first row of menu buttons opens Info (stats), Skills, Items and Map, each
toggling and reflecting its state in the button art.

## Preconditions
- As UC-208

## Test Steps

### Each button opens its window
1. Launch with `--autologin --trace=hud`
2. Click **Info** — verify a stats window opens and `hud.toggle` reports it open
3. Click **Skills** — verify a skill list opens
4. Click **Items** — verify an inventory list opens
5. Click **Map** — verify the full map opens
6. Verify each button shows its `_on` art while its window is open

### Each button closes its window
1. Click each button a second time
2. Verify the window closes and the button returns to `_off`
3. Verify `hud.toggle` reports it closed

### Stats window contents
1. Open Info
2. Verify STR, AGI, VIT, INT, DEX and LUK are each shown with their value
3. Verify bonuses are shown where the server sends them (`ZC_COUPLESTATUS`)
4. Verify the values match what F3 reports

### Stats update live
1. Leave Info open
2. Cause a stat change (level up, or a server-side stat command)
3. Verify the window updates without being reopened
4. Verify `hud.stat` names the stat and its old and new values

### Skills window contents
1. Open Skills on a Novice
2. Verify Basic Skill appears with its level
3. Verify the count matches `hud.skills` in the trace

### Items window contents
1. Open Items
2. Verify carried items are listed with their counts
3. Verify an empty inventory shows an empty list, not a broken window

### Map window
1. Open Map
2. Verify the full Prontera map image is shown with the player marked
3. Verify the marker agrees with the minimap

### Windows coexist
1. Open all four at once
2. Verify each is independently movable and closable
3. Verify clicking a window does not also click through to the map underneath

## Expected Results
- All four buttons toggle their windows and reflect state
- Stats, skills and items show real server data
- Live stat updates land without reopening
- No click falls through a window to the world
