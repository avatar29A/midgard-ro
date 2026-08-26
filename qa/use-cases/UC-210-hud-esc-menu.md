# UC-210: HUD — ESC Menu and Leaving the Game

## Description
ESC opens the system menu; Return to character select and Quit both leave the
game cleanly through the correct packet.

## Preconditions
- As UC-208

## Test Steps

### The menu toggles
1. Launch with `--autologin --trace=hud,net`
2. Press ESC
3. Verify a menu appears with Return to character select, Quit and Cancel
4. Press ESC again — verify it closes
5. Verify the game keeps running and rendering behind it while open

### Cancel
1. Press ESC, click Cancel
2. Verify the menu closes and control returns to the character
3. Verify no packet was sent

### Return to character select
1. Press ESC, click Return to character select
2. Verify `CZ_RESTART` (`0x00B2`) is sent with type byte **1**
3. Verify the client reaches the character select screen
4. Verify a character can be re-entered without restarting the client
5. Verify no error in the log and no orphaned connection

### Quit
1. Press ESC, click Quit
2. Verify `CZ_REQ_DISCONNECT` (`0x018A`) is sent
3. Verify the server answers `ZC_ACK_REQ_DISCONNECT` (`0x018B`)
4. Verify the client exits cleanly (`game closed normally` in the log, exit 0)
5. Verify the server log shows a clean logout, not a timeout

### The menu does not swallow the game
1. Open the menu while walking
2. Verify the character finishes its current move
3. Verify clicks on the world behind the menu do not start new moves

## Expected Results
- ESC toggles the menu; Cancel is inert
- Character select and Quit each send the right packet and land cleanly
- No timeout, hang, or orphaned connection on the way out
