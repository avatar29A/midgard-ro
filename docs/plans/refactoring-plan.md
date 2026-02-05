# Refactoring Plan - GRF Browser & Engine Extraction

## Executive Summary

The codebase has grown significantly with the Play mode feature. This plan addresses:
- **5,470-line map_viewer.go** - needs decomposition
- **109 hardcoded magic numbers** - need constants
- **Complex functions** (up to 249 lines) - need splitting
- **Engine extraction opportunities** - move reusable components

---

## Phase 1: Constants & Magic Numbers (Priority: HIGH)

### 1.1 Create `cmd/grfbrowser/constants.go`

```go
package main

// Direction indices (RO convention: 0=S, clockwise)
const (
    DirS  = 0
    DirSW = 1
    DirW  = 2
    DirNW = 3
    DirN  = 4
    DirNE = 5
    DirE  = 6
    DirSE = 7
    NumDirections = 8
)

// Action types
const (
    ActionIdle = 0
    ActionWalk = 1
    ActionSit  = 2
    ActionPickUp = 3
)

// Zoom/Scale defaults
const (
    DefaultSpriteZoom     = 1.0
    DefaultMapZoom        = 1.0
    SpriteZoomIncrement   = 0.5
    TileZoomIncrement     = 0.25
    MinZoom               = 0.5
    MaxZoom               = 8.0
)

// Rendering defaults
const (
    DefaultMaxModels        = 1500
    DefaultTerrainBrightness = 1.0
    DefaultFogNear          = 150.0
    DefaultFogFar           = 1400.0
    DefaultMoveSpeed        = 50.0
    DefaultSpriteScale      = 0.28
)

// Direction hysteresis
const (
    DirectionHysteresis = math.Pi / 16  // ~11 degrees
    SectorSize          = math.Pi / 4   // 45 degrees
    SectorOffset        = math.Pi / 8   // 22.5 degrees
)

// Bounds constants (replace 10000/-10000)
const (
    BoundsMax = 100000
    BoundsMin = -100000
)
```

### 1.2 Create `cmd/grfbrowser/colors.go`

```go
package main

import "image/color"

// GAT cell type colors
var (
    GATColorWalkable    = color.RGBA{R: 100, G: 200, B: 100, A: 255}
    GATColorBlocked     = color.RGBA{R: 200, G: 80, B: 80, A: 255}
    GATColorWater       = color.RGBA{R: 80, G: 150, B: 220, A: 255}
    GATColorWaterWalk   = color.RGBA{R: 100, G: 180, B: 180, A: 255}
    GATColorCliff       = color.RGBA{R: 180, G: 140, B: 100, A: 255}
    GATColorUnknown     = color.RGBA{R: 128, G: 128, B: 128, A: 255}
)

// UI colors
var (
    BackgroundColor = [4]float32{0.1, 0.1, 0.12, 1.0}
)
```

### 1.3 Files to update
- [ ] `map_viewer.go` - Replace all magic numbers with constants
- [ ] `main.go` - Replace zoom values, colors
- [ ] `preview_map.go` - Replace GAT colors, list limits
- [ ] `model_viewer.go` - Replace rendering constants

**Estimated changes:** ~150 lines modified across 4 files

---

## Phase 2: Split map_viewer.go (Priority: CRITICAL)

Current: **5,470 lines** in one file
Target: **5-6 files** of ~800-1000 lines each

### 2.1 New file structure

```
cmd/grfbrowser/
├── map_viewer.go          (~800 lines) - Core MapViewer struct, initialization
├── map_viewer_render.go   (~1000 lines) - Render passes, shader setup
├── map_viewer_camera.go   (~400 lines) - Camera controls (orbit + play mode)
├── map_viewer_shaders.go  (~600 lines) - Shader source code, compilation
├── sprite_system.go       (~800 lines) - PlayerCharacter, sprite rendering
├── sprite_compositor.go   (~500 lines) - Composite sprite generation
└── constants.go           (~100 lines) - All constants (from Phase 1)
```

### 2.2 Extraction plan

#### `map_viewer_camera.go`
Extract:
- `HandleCameraInput()`
- `HandlePlayMovement()`
- `UpdateOrbitCamera()`
- Camera state fields from MapViewer struct
- Mouse drag handling

#### `map_viewer_render.go`
Extract:
- `Render()` main function
- `renderTerrain()`
- `renderModels()`
- `renderWater()`
- `renderBoundingBoxes()`
- Framebuffer management

#### `map_viewer_shaders.go`
Extract:
- All shader source strings (currently inline)
- `compileShaderProgram()` helper
- Uniform location caching

#### `sprite_system.go`
Extract:
- `PlayerCharacter` struct
- `renderPlayerCharacter()`
- `renderPlayerShadow()`
- `UpdatePlayerAnimation()`
- `UpdatePlayerMovement()`
- Direction calculation logic

#### `sprite_compositor.go`
Extract:
- `CompositeFrame` struct
- `compositeSprites()` function
- `saveAllDirectionsSheet()` debug function
- `saveDebugPNG()` helper
- `drawSimpleText()` bitmap font

### 2.3 Shared state approach

```go
// map_viewer.go - Core struct with embedded components
type MapViewer struct {
    // Core resources
    width, height int
    fbo, colorTex, depthTex uint32

    // Embedded components
    camera    *MapCamera
    terrain   *TerrainRenderer
    models    *ModelRenderer
    water     *WaterRenderer
    sprites   *SpriteSystem

    // Shared state
    viewProj  math.Mat4
    lightDir  [3]float32
}
```

**Estimated effort:** 4-6 hours

---

## Phase 3: Extract to Engine (Priority: MEDIUM)

### 3.1 Candidates for `internal/engine/`

#### 3.1.1 Shader utilities → `internal/engine/shader/`

```go
// internal/engine/shader/shader.go
package shader

type Program struct {
    ID uint32
    uniforms map[string]int32
}

func Compile(vertSrc, fragSrc string) (*Program, error)
func (p *Program) Use()
func (p *Program) SetMat4(name string, m *[16]float32)
func (p *Program) SetVec3(name string, v [3]float32)
func (p *Program) SetFloat(name string, f float32)
func (p *Program) SetInt(name string, i int32)
```

#### 3.1.2 Sprite system → `internal/engine/sprite/`

```go
// internal/engine/sprite/sprite.go
package sprite

type Billboard struct {
    Texture  uint32
    Width    float32
    Height   float32
    Position [3]float32
}

type Renderer struct {
    program *shader.Program
    vao, vbo uint32
}

func NewRenderer() (*Renderer, error)
func (r *Renderer) Draw(b *Billboard, camRight, camUp [3]float32)
```

#### 3.1.3 Camera system → `internal/engine/camera/`

```go
// internal/engine/camera/orbit.go
package camera

type OrbitCamera struct {
    Target   [3]float32
    Distance float32
    Yaw, Pitch float32
}

func (c *OrbitCamera) GetViewMatrix() math.Mat4
func (c *OrbitCamera) HandleMouseDrag(dx, dy float32)
func (c *OrbitCamera) HandleMouseWheel(delta float32)
```

### 3.2 Benefits
- Reusable in future game client (`cmd/client`)
- Cleaner separation of concerns
- Easier testing
- Follows existing architecture (see `docs/adr/ADR-002-architecture.md`)

**Estimated effort:** 6-8 hours

---

## Phase 4: Simplify Complex Functions (Priority: MEDIUM)

### 4.1 Split `compositeSprites()` (249 lines → 3 functions)

```go
// Before: one 249-line function
func compositeSprites(...) (pixels []byte, width, height int)

// After: three focused functions
func calculateCompositeBounds(body, head *SpriteFrame) (bounds Rect, bodyAnchor, headAnchor Point)
func renderComposite(bounds Rect, body, head *SpriteFrame, bodyAnchor, headAnchor Point) []byte
func saveCompositeDebug(pixels []byte, width, height int, direction int)
```

### 4.2 Extract shader sources

Move inline shader strings to separate files or embed:

```go
//go:embed shaders/terrain.vert
var terrainVertShader string

//go:embed shaders/terrain.frag
var terrainFragShader string
```

Or keep in `map_viewer_shaders.go` as const blocks.

### 4.3 Simplify NewMapViewer()

```go
// Before: massive initialization function
func NewMapViewer(w, h int) (*MapViewer, error) {
    // 300+ lines of setup
}

// After: builder pattern
func NewMapViewer(w, h int) (*MapViewer, error) {
    mv := &MapViewer{width: w, height: h}

    if err := mv.initFramebuffer(); err != nil {
        return nil, err
    }
    if err := mv.initShaders(); err != nil {
        return nil, err
    }
    if err := mv.initTerrain(); err != nil {
        return nil, err
    }
    // etc.

    return mv, nil
}
```

---

## Phase 5: Code Quality (Priority: LOW)

### 5.1 Replace fmt.Printf with structured logging

```go
// Before
fmt.Printf("Loaded %d models\n", count)
fmt.Fprintf(os.Stderr, "Error: %v\n", err)

// After
slog.Info("loaded models", "count", count)
slog.Error("loading failed", "error", err)
```

**109 occurrences** to update across grfbrowser files.

### 5.2 Add documentation

- Document all exported functions
- Add package-level documentation
- Document complex algorithms (direction calculation, sprite compositing)

---

## Implementation Order

| Phase | Priority | Effort | Dependencies |
|-------|----------|--------|--------------|
| 1. Constants | HIGH | 2h | None |
| 2. Split map_viewer.go | CRITICAL | 6h | Phase 1 |
| 3. Extract to engine | MEDIUM | 8h | Phase 2 |
| 4. Simplify functions | MEDIUM | 3h | Phase 2 |
| 5. Code quality | LOW | 4h | Phase 1-4 |

**Total estimated effort:** ~23 hours

---

## Success Criteria

- [ ] No file over 1,200 lines in `cmd/grfbrowser/`
- [ ] All magic numbers replaced with named constants
- [ ] No function over 100 lines
- [ ] Shader, camera, and sprite systems extracted to engine
- [ ] All tests passing
- [ ] Linter passing

---

## Files NOT requiring changes

These are well-structured and appropriately sized:
- `internal/engine/input/input.go` (120 lines) ✓
- `internal/engine/window/window.go` (136 lines) ✓
- `pkg/formats/*.go` (200-600 lines each) ✓

---

## Questions for Review

1. **Phase 3 scope:** Should we extract to engine now, or defer to when building `cmd/client`?
2. **Shader embedding:** Use `//go:embed` or keep inline strings in Go files?
3. **Logging:** Use `slog` (stdlib) or external logger like `zap`?
