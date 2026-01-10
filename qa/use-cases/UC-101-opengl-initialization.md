# UC-101: OpenGL Initialization

## Description
Tests OpenGL initialization and context setup. Verifies that the correct OpenGL version is available and initialized properly.

## Preconditions
- SDL2 window with OpenGL context is created
- OpenGL 4.1+ drivers available

## Test Steps
1. Create window with OpenGL context
2. Create renderer with `renderer.New(config)`
3. Verify no error is returned
4. Check that OpenGL is initialized (gl.Init() succeeded)
5. Verify OpenGL version is 4.1 or higher
6. Verify renderer name is reported correctly
7. Verify default OpenGL state is set (depth test enabled, clear color set)

## Expected Results
- OpenGL initializes without errors
- Version string contains "4.1" or higher
- Renderer name is logged (e.g., "AMD Radeon", "Intel HD", "Apple M1")
- Depth test is enabled
- Clear color is set to dark blue-gray (0.1, 0.1, 0.15, 1.0)
- Viewport is set correctly

## Priority
Critical

## Related
- PRD Section: 7.1 Milestone 1 (Window & Triangle)
- ADR: ADR-001-graphics-stack.md
- Code: `/Users/borisglebov/git/Faultbox/midgard-ro/internal/engine/renderer/renderer.go::New()`
- Test: None (requires OpenGL context)

## Manual Verification Required
- Check log output for OpenGL version
- Check log output for renderer name
- Verify no OpenGL errors in console

## Platform Differences
- macOS: Maximum OpenGL 4.1 Core Profile
- Linux: May support OpenGL 4.5+
- Windows: May support OpenGL 4.6+
