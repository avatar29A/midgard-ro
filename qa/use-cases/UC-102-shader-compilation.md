# UC-102: Shader Compilation and Linking

## Description
Tests GLSL shader compilation and program linking. Shaders are essential for all rendering operations.

## Preconditions
- OpenGL context is initialized
- Renderer is created

## Test Steps
1. Renderer creates vertex shader from source
2. Verify vertex shader compiles without errors
3. Renderer creates fragment shader from source
4. Verify fragment shader compiles without errors
5. Renderer links shader program
6. Verify program links without errors
7. Verify program ID is non-zero
8. Verify shaders are cleaned up after linking

## Expected Results
- Vertex shader compiles successfully (GLSL 410 core)
- Fragment shader compiles successfully
- Shader program links successfully
- No compilation or link errors
- Shader program can be used for rendering
- Individual shaders are deleted after linking

## Priority
Critical

## Related
- PRD Section: 7.1 Milestone 1 (Window & Triangle)
- ADR: ADR-001-graphics-stack.md
- Code: `/Users/borisglebov/git/Faultbox/midgard-ro/internal/engine/renderer/renderer.go::createShaderProgram()`
- Test: None (requires OpenGL context)

## Shader Specifications
- Version: GLSL 410 core (OpenGL 4.1)
- Vertex shader: Transforms vertices, passes color to fragment shader
- Fragment shader: Outputs final pixel color

## Error Cases to Test
- Invalid GLSL syntax (should error with helpful message)
- Version mismatch (e.g., using GLSL 330 features)
- Undefined variables or functions
