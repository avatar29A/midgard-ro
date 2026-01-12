# ADR-012: 3D Model Viewer

## Status
Proposed

## Context

Ragnarok Online uses RSM (Resource Model) files for 3D objects like buildings, trees, decorations, and effects placed on maps. These models are referenced by RSW (Resource World) files and rendered in the game world.

To provide complete map and asset preview capabilities in the GRF Browser, we need:
1. RSM format parser to read 3D model data
2. 3D rendering capability to visualize models
3. Texture loading from GRF archives

### Current State

- GRF Browser supports 2D previews (sprites, images, GAT/GND maps)
- RSW parser lists model references but cannot display them
- `cmd/grfbrowser/main.go` is 2772 lines and needs refactoring before adding new features

### RSM Format Overview

| Aspect | Details |
|--------|---------|
| Magic | "GRSM" (4 bytes) |
| Versions | 1.1-1.5 (common), 2.2-2.3 (newer) |
| Structure | Header → Textures → Node hierarchy → Mesh data → Animation |
| Features | Hierarchical nodes, keyframe animation, smooth shading |

## Decision

### 1. RSM File Format Specification

Based on roBrowser source code analysis:

```
Header:
  magic: char[4]           = "GRSM"
  version: uint8[2]        = [major, minor]
  anim_length: int32       = animation duration in ms
  shading_type: int32      = 0=NONE, 1=FLAT, 2=SMOOTH
  alpha: uint8             = transparency (v1.4+, /255)
  reserved: uint8[16]      = skip

Textures:
  count: int32
  names: char[40][]        = null-terminated texture paths

Root Node:
  name: char[40]           = root node name

Nodes:
  count: int32
  Node[count]:
    name: char[40]
    parent_name: char[40]
    texture_count: int32
    texture_ids: int32[]

    transform_matrix: float32[9]    = 3x3 rotation matrix
    offset: float32[3]              = pivot offset
    position: float32[3]            = translation
    rotation_angle: float32         = axis-angle rotation
    rotation_axis: float32[3]
    scale: float32[3]

    vertex_count: int32
    vertices: float32[3][]          = x, y, z positions

    texcoord_count: int32
    texcoords[]:
      color: uint8[4]               = RGBA (v1.2+)
      u, v: float32                 = texture coordinates

    face_count: int32
    faces[]:
      vertex_ids: uint16[3]         = triangle vertex indices
      texcoord_ids: uint16[3]       = texture coord indices
      texture_id: uint16
      padding: uint16
      two_side: int32               = double-sided flag
      smooth_group: int32           = smoothing group (v1.2+)

    pos_keyframes[]:                = position animation (v<1.5)
      frame: int32
      px, py, pz: float32

    rot_keyframes[]:                = rotation animation
      frame: int32
      qx, qy, qz, qw: float32       = quaternion

    scale_keyframes[]:              = scale animation (v1.5+)
      frame: int32
      sx, sy, sz: float32

Volume Boxes:
  count: int32
  boxes[]:
    size: float32[3]
    position: float32[3]
    rotation: float32[3]
    flag: int32                     = (v1.3+)
```

### 2. Implementation Stages

#### Stage 0: GRF Browser Refactoring (Pre-requisite)

**Goal**: Split `main.go` (2772 lines) into manageable modules

| File | Contents | ~Lines |
|------|----------|--------|
| `main.go` | App struct, main(), NewApp, Close, Run, OpenGRF | 150 |
| `file_tree.go` | buildFileTree, sortTree, renderFileTree | 200 |
| `ui.go` | render(), renderSearchAndFilter, renderStatusBar | 250 |
| `commands.go` | captureScreenshot, dumpState, executeCommand | 200 |
| `preview_sprite.go` | Sprite/ACT preview functions | 600 |
| `preview_image.go` | Image/Text/Hex preview functions | 200 |
| `preview_audio.go` | Audio preview functions | 150 |
| `preview_map.go` | GAT/GND/RSW preview functions | 500 |
| `utils.go` | Helper functions (euckrToUTF8, etc.) | 150 |

#### Stage 1: RSM Parser

**Goal**: Parse RSM files, expose model data

Deliverables:
- `pkg/formats/rsm.go` - RSM parser
- `pkg/formats/rsm_test.go` - Unit tests

```go
// Core types
type RSM struct {
    Version     RSMVersion
    AnimLength  int32
    Shading     RSMShadingType
    Alpha       float32
    Textures    []string
    RootNode    string
    Nodes       []RSMNode
    VolumeBoxes []RSMVolumeBox
}

type RSMNode struct {
    Name        string
    Parent      string
    TextureIDs  []int32
    Matrix      [9]float32
    Offset      [3]float32
    Position    [3]float32
    RotAngle    float32
    RotAxis     [3]float32
    Scale       [3]float32
    Vertices    [][3]float32
    TexCoords   []RSMTexCoord
    Faces       []RSMFace
    PosKeys     []RSMPosKeyframe
    RotKeys     []RSMRotKeyframe
    ScaleKeys   []RSMScaleKeyframe
}
```

#### Stage 2: RSM Info Panel

**Goal**: Display model metadata in GRF Browser

Preview panel shows:
- Version, animation length, shading type
- Texture list with paths
- Node hierarchy tree
- Vertex/face counts per node
- Volume box count

```
┌─────────────────────────────────────┐
│ prontera_building.rsm               │
├─────────────────────────────────────┤
│ Version: 1.5                        │
│ Animation: 0 ms                     │
│ Shading: SMOOTH                     │
│ Alpha: 1.0                          │
├─────────────────────────────────────┤
│ Textures (3):                       │
│   wall.bmp                          │
│   roof.bmp                          │
│   window.bmp                        │
├─────────────────────────────────────┤
│ Nodes (2):                          │
│ ▼ building_main                     │
│   ├─ Vertices: 128                  │
│   ├─ Faces: 64                      │
│   └─ child_node                     │
│       ├─ Vertices: 32               │
│       └─ Faces: 16                  │
├─────────────────────────────────────┤
│ Volume Boxes: 1                     │
└─────────────────────────────────────┘
```

#### Stage 3: 3D Model Viewer

**Goal**: Render 3D models with textures

Architecture:
```go
// cmd/grfbrowser/model_viewer.go

type ModelViewer struct {
    // OpenGL resources
    framebuffer   uint32
    colorTexture  uint32
    depthBuffer   uint32

    // Shaders
    shaderProgram uint32

    // Mesh data
    vao, vbo, ebo uint32
    indexCount    int32

    // Camera
    rotationX, rotationY float32
    distance             float32

    // Model textures
    textures []uint32
}

func NewModelViewer() (*ModelViewer, error)
func (mv *ModelViewer) LoadModel(rsm *formats.RSM, texLoader TextureLoader) error
func (mv *ModelViewer) Render() uint32  // Returns texture ID for ImGui
func (mv *ModelViewer) HandleInput(drag bool, deltaX, deltaY, scroll float32)
func (mv *ModelViewer) Destroy()
```

Rendering approach:
1. Create offscreen framebuffer (512x512 or configurable)
2. Render model with OpenGL 4.1 shaders
3. Display framebuffer texture in ImGui panel
4. Update on mouse drag (rotation) / scroll (zoom)

Shader features:
- MVP matrix transformation
- Texture sampling
- Basic directional + ambient lighting
- Optional: smooth normals based on shading type

#### Stage 4: Animation Support (Future)

**Goal**: Animate models using keyframe data

Features:
- Play/pause/stop controls
- Timeline scrubber
- Frame-by-frame stepping
- Loop toggle

### 3. Technical Considerations

#### Texture Loading
- Textures referenced by path in RSM (e.g., "texture/wall.bmp")
- Load from GRF using path transformation
- Support BMP, TGA formats (existing decoders)
- Cache loaded textures to avoid re-loading

#### Node Hierarchy
- Nodes form parent-child tree
- Transforms cascade from parent to child
- Must compute world-space transforms for rendering

#### Performance
- Typical model: 100-1000 vertices
- Use indexed rendering (EBO)
- Single draw call per texture batch
- Render to texture, display result

#### Memory
- RSM data: ~10-100KB per model
- GPU buffers: Similar size
- Textures: Largest cost, cache and share

### 4. Package Structure

```
pkg/formats/
├── rsm.go           # RSM parser (NEW)
└── rsm_test.go      # Tests (NEW)

cmd/grfbrowser/
├── main.go          # Core App (refactored)
├── file_tree.go     # File tree (refactored)
├── ui.go            # UI rendering (refactored)
├── commands.go      # Commands (refactored)
├── preview_sprite.go
├── preview_image.go
├── preview_audio.go
├── preview_map.go   # GAT/GND/RSW (refactored)
├── preview_model.go # RSM preview (NEW)
├── model_viewer.go  # 3D renderer (NEW)
└── utils.go         # Helpers (refactored)
```

## Consequences

### Positive
- Complete asset preview in GRF Browser
- Visual inspection of 3D models without external tools
- Foundation for full map rendering with objects
- Better understanding of RSM format for client development

### Negative
- Significant OpenGL code for 3D rendering
- Texture loading adds complexity
- Animation support is complex

### Mitigations
- Stage 0 (refactoring) makes code maintainable
- Stage 2 (info panel) provides value without 3D
- Render-to-texture approach isolates GL code
- Animation deferred to Stage 4

## References

- [roBrowser RSM Loader](https://github.com/vthibault/roBrowser) - JavaScript implementation
- [RagnarokFileFormats RSM.MD](https://github.com/rdw-archive/RagnarokFileFormats) - Format documentation
- [ADR-011: Map Format Parsers](./ADR-011-map-format-parsers.md) - Related formats
- [ADR-009: GRF Browser Tool](./ADR-009-grf-browser-tool.md) - Browser architecture
