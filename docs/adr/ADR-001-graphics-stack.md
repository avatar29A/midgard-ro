# ADR-001: Graphics Stack Selection

**Status**: Accepted (Updated)
**Date**: 2025-01-09
**Updated**: 2025-01-09 (SDL3 → SDL2)
**Decision Makers**: Boris (CEO), Ilon (CTO)

## Context

We need to choose a graphics stack for rendering the Ragnarok Online client. The original RO uses DirectX 7/8, but we're building a cross-platform modern client.

### Options Considered

1. **SDL3 GPU API (New!)**
   - SDL3's new cross-platform GPU abstraction
   - Backends: Vulkan, Metal, D3D12
   - Still experimental

2. **OpenGL 4.1+ with SDL3**
   - Mature, well-documented
   - Works on macOS (4.1 is max), Linux, Windows
   - Large learning resource base
   - go-sdl3 bindings are pure Go but less mature

3. **OpenGL 4.1+ with SDL2**
   - Battle-tested, stable
   - go-sdl2 bindings are mature and well-maintained
   - Requires CGO and system SDL2 library

4. **Vulkan with SDL3**
   - Modern, high performance
   - Extremely verbose
   - Steep learning curve

5. **raylib**
   - Simple, beginner-friendly
   - Limited control over rendering
   - Not ideal for 2.5D isometric

## Decision

**We choose OpenGL 4.1+ with SDL2 (veandco/go-sdl2 bindings)**

### Rationale

1. **Educational Value**: OpenGL teaches fundamental graphics concepts (shaders, buffers, state machine) that transfer to any graphics API.

2. **macOS Support**: macOS deprecated OpenGL but still supports 4.1. This version has all features we need (shaders, VAOs, instancing).

3. **Stability**: SDL2 is mature and battle-tested. The `go-sdl2` bindings are well-maintained with extensive documentation.

4. **Documentation**: Extensive tutorials and examples for both OpenGL and SDL2.

5. **Scope Appropriate**: For a 2.5D isometric game with sprite-based graphics, OpenGL is more than sufficient. Vulkan's complexity isn't justified.

### Why SDL2 over SDL3

Initially we chose SDL3 with go-sdl3 (pure Go, no CGO). However:
- go-sdl3 is still experimental and had version resolution issues
- SDL2 is more mature and stable
- go-sdl2 has better documentation and community support
- The CGO requirement is acceptable for this project

## Consequences

### Positive
- Faster development due to simpler, stable API
- More learning resources available
- Cross-platform without extra work
- Mature, well-tested bindings

### Negative
- Requires CGO (need C compiler and SDL2 dev libraries)
- No macOS Metal performance optimizations
- OpenGL is "legacy" on macOS (but stable)

### System Requirements
- macOS: `brew install sdl2`
- Linux: `apt install libsdl2-dev`
- Windows: Download SDL2 development libraries

## Technical Notes

### go-sdl2 Setup
```go
import (
    "github.com/veandco/go-sdl2/sdl"
)

func main() {
    if err := sdl.Init(sdl.INIT_VIDEO); err != nil {
        panic(err)
    }
    defer sdl.Quit()

    // Set OpenGL attributes BEFORE creating window
    sdl.GLSetAttribute(sdl.GL_CONTEXT_MAJOR_VERSION, 4)
    sdl.GLSetAttribute(sdl.GL_CONTEXT_MINOR_VERSION, 1)
    sdl.GLSetAttribute(sdl.GL_CONTEXT_PROFILE_MASK, sdl.GL_CONTEXT_PROFILE_CORE)

    // Create OpenGL window
    window, err := sdl.CreateWindow("Midgard RO",
        sdl.WINDOWPOS_CENTERED, sdl.WINDOWPOS_CENTERED,
        1280, 720,
        sdl.WINDOW_OPENGL|sdl.WINDOW_RESIZABLE)
    if err != nil {
        panic(err)
    }
    defer window.Destroy()

    // Create OpenGL context
    ctx, err := window.GLCreateContext()
    if err != nil {
        panic(err)
    }
    defer sdl.GLDeleteContext(ctx)
}
```

### OpenGL Loading
We use `github.com/go-gl/gl/v4.1-core/gl` for OpenGL function loading.

## References
- [go-sdl2 GitHub](https://github.com/veandco/go-sdl2)
- [go-gl OpenGL bindings](https://github.com/go-gl/gl)
- [LearnOpenGL tutorials](https://learnopengl.com)
- [SDL2 Documentation](https://wiki.libsdl.org/SDL2)
