# UC-100: SDL2 Window Creation

## Description
Tests SDL2 window creation with OpenGL context. This is the foundation for all rendering operations.

## Preconditions
- SDL2 libraries installed on system
- Display available (not headless)
- OpenGL 4.1+ drivers available

## Test Steps
1. Create `window.Config` with title="Test", width=800, height=600
2. Call `window.New(config)`
3. Verify no error is returned
4. Verify `window.sdlWindow` is not nil
5. Verify `window.glContext` is not nil
6. Call `window.GetSize()` and verify returns (800, 600)
7. Call `window.Close()` and verify clean shutdown

## Expected Results
- Window is created successfully
- Window appears on screen with correct dimensions
- OpenGL context is created (4.1 Core Profile on macOS)
- VSync setting is applied
- Window can be closed without errors
- SDL2 is properly shut down

## Priority
Critical

## Related
- PRD Section: 7.1 Milestone 1 (Window & Triangle)
- ADR: ADR-001-graphics-stack.md
- Code: `/Users/borisglebov/git/Faultbox/midgard-ro/internal/engine/window/window.go::New()`
- Test: None (requires display)

## Manual Verification Required
- Window appears on screen
- Window has correct title
- Window is resizable (WINDOW_RESIZABLE flag)
- Window has correct initial size

## Edge Cases
- Fullscreen mode
- Different window sizes
- Multiple monitors
- High DPI displays
