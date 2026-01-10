# UC-104: Window Resize Handling

## Description
Tests proper handling of window resize events. The viewport must be updated when the window size changes.

## Preconditions
- Window is created and visible
- Renderer is initialized
- Application is running

## Test Steps
1. Launch application with default window size
2. Manually resize window by dragging edge or corner
3. Verify resize event is detected by input system
4. Verify `renderer.Resize()` is called with new dimensions
5. Verify OpenGL viewport is updated via `gl.Viewport()`
6. Verify rendered content scales correctly to new size

## Expected Results
- Resize events are detected immediately
- Renderer is notified of new size
- OpenGL viewport matches new window size
- Triangle (or other content) scales/repositions correctly
- No rendering artifacts during resize
- No crashes or errors

## Priority
High

## Related
- PRD Section: 7.1 Milestone 1
- ADR: ADR-001-graphics-stack.md
- Code: `/Users/borisglebov/git/Faultbox/midgard-ro/internal/engine/renderer/renderer.go::Resize()`
- Code: `/Users/borisglebov/git/Faultbox/midgard-ro/internal/game/game.go::Run()` (event handling)
- Test: None (manual interaction required)

## Manual Verification Required
1. Resize window horizontally
2. Resize window vertically
3. Resize to very small size (e.g., 100x100)
4. Resize to very large size (e.g., 2560x1440)
5. Maximize window
6. Restore from maximized

All cases should render correctly without artifacts.
