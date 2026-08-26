# UC-208: HUD — Always-On Elements

## Description
The minimap, chat box, hotkey bar and click target square are present in game
without any interaction, alongside the Basic Info panel from #87.

## Preconditions
- Local rAthena stack running (`make server-up`)
- Character on `prontera` near the fountain (`156,191`)

## Test Steps

### Everything is on screen at once
1. Launch: `./build/midgard --autologin --trace=hud,net --screenshot-after 20s`
2. Verify the minimap is in the top-right corner
3. Verify the chat box is bottom-left
4. Verify the hotkey bar is along the bottom
5. Verify the Basic Info panel is still top-left and undisturbed
6. Verify nothing overlaps illegibly at 1280×720
7. Repeat at 1920×1080 (`--width 1920 --height 1080`)

### Minimap tracks the player
1. Walk several cells north
2. Verify the player dot moves in the same direction on the minimap
3. Verify the dot's position matches the coordinates in the status bar

### Minimap without an image
1. Enter a map with no pre-rendered minimap image
2. Verify a **warn** log names the path it tried
3. Verify the corner is empty rather than black or garbage
4. Verify no crash

### Chat shows server messages
1. On login, verify rAthena's welcome lines appear in the chat box
2. Verify lines are readable and wrap rather than being cut
3. Verify `hud.chat` appears in the trace once per line
4. Generate enough lines to overflow, and verify the box scrolls and keeps a bounded backlog

### Click target square
1. Move the cursor over a walkable cell
2. Verify a green square sits on that cell and follows the cursor
3. Click to move
4. Verify the square animates on the click
5. Verify it disappears when the character arrives
6. Verify `hud.target` reports the cell under the cursor

## Expected Results
- All four HUD elements visible simultaneously and legible at both resolutions
- Minimap dot matches the real position
- Chat shows server text and is bounded
- The target square tracks the cursor and animates on click
