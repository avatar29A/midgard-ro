# UC-203: Game Loop Exit and Cleanup

## Description
Tests graceful exit from the game loop and proper cleanup of all resources when the game shuts down.

## Preconditions
- Game is initialized and running

## Test Steps

### Exit via ESC Key
1. Start game
2. Press ESC key
3. Verify game sets `running = false`
4. Verify game loop exits
5. Verify `game.Close()` is called
6. Verify all resources are cleaned up

### Exit via Window Close Button
1. Start game
2. Click window close button (X)
3. Verify quit event is detected
4. Verify game loop exits
5. Verify cleanup occurs

### Resource Cleanup Verification
After exit, verify:
1. Renderer is closed (shaders, VAOs, VBOs deleted)
2. Window is destroyed
3. SDL2 is shut down
4. OpenGL context is destroyed
5. Logger is synced and flushed
6. No memory leaks
7. No zombie processes

## Expected Results
- Game exits cleanly without crashes
- All OpenGL resources are released
- SDL2 is properly shut down
- Window disappears from screen
- Process terminates normally (exit code 0)
- No error messages during shutdown
- No memory leaks (can verify with tools)

## Priority
High

## Related
- PRD Section: 6.2 Main Game Loop
- ADR: ADR-002-architecture.md
- Code: `/Users/borisglebov/git/Faultbox/midgard-ro/internal/game/game.go::Close()`
- Code: `/Users/borisglebov/git/Faultbox/midgard-ro/cmd/client/main.go` (defer cleanup)
- Test: None (integration test)

## Cleanup Order
IMPORTANT - cleanup must happen in reverse order of initialization:
1. Renderer cleanup (OpenGL resources)
2. Window destruction (destroys OpenGL context)
3. SDL2 shutdown
4. Logger sync

## Failure Cases
- If cleanup crashes → could indicate resource corruption
- If process doesn't terminate → possible deadlock
- If window remains visible → SDL2 not shut down properly
