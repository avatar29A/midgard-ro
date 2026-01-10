# UC-103: Triangle Rendering (Test Geometry)

## Description
Tests rendering of a colored triangle, the "Hello World" of graphics programming. Verifies the entire rendering pipeline is functional.

## Preconditions
- Window is created
- OpenGL is initialized
- Shaders are compiled and linked
- Triangle geometry is created (VAO/VBO)

## Test Steps
1. Create renderer (which creates test triangle)
2. Verify triangle VAO is non-zero
3. Verify triangle VBO is non-zero
4. In render loop: call `renderer.Begin()`
5. Call `renderer.DrawTriangle()`
6. Call `renderer.End()`
7. Swap buffers to display

## Expected Results
- Triangle appears on screen
- Triangle has three vertices forming a triangle
- Top vertex is red
- Bottom-left vertex is green
- Bottom-right vertex is blue
- Colors are interpolated across triangle surface (gradient effect)
- No rendering errors

## Priority
Critical

## Related
- PRD Section: 7.1 Milestone 1 (Render a colored triangle)
- ADR: ADR-001-graphics-stack.md
- Code: `/Users/borisglebov/git/Faultbox/midgard-ro/internal/engine/renderer/renderer.go::createTriangle()`, `DrawTriangle()`
- Test: None (visual verification required)

## Manual Verification Required
VISUAL CHECK:
- Window shows triangle on dark blue-gray background
- Triangle is centered
- Colors are correct (RGB at vertices)
- Smooth color gradient across triangle
- No flickering or artifacts
- Triangle persists across frames

## Triangle Geometry
```
Position:        Color:
Top: (0, 0.5)    Red (1, 0, 0)
BL: (-0.5, -0.5) Green (0, 1, 0)
BR: (0.5, -0.5)  Blue (0, 0, 1)
```
