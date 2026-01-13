# ADR-013: 3D Map Viewer

## Status
Proposed

## Context

Ragnarok Online maps are composed of three file formats:
- **GND** (Ground) - 3D terrain mesh with textures and lightmaps
- **RSW** (Resource World) - World settings, object placements (RSM models), lighting, water
- **GAT** (Ground Altitude Table) - Walkability data (already implemented in ADR-011)

With ADR-012 complete (3D RSM model viewer), we now have all the components needed to render complete 3D maps in the GRF Browser. This provides:
1. Visual map inspection without running the game client
2. Understanding of map structure for client development
3. Foundation for future full map rendering in the game client

### Current State

| Component | Status | Location |
|-----------|--------|----------|
| GAT parser | ✅ Complete | `pkg/formats/gat.go` |
| GND parser | ✅ Complete | `pkg/formats/gnd.go` |
| RSW parser | ✅ Complete | `pkg/formats/rsw.go` |
| RSM parser | ✅ Complete | `pkg/formats/rsm.go` |
| RSM 3D viewer | ✅ Complete | `cmd/grfbrowser/model_viewer.go` |
| 2D map previews | ✅ Complete | `cmd/grfbrowser/preview_map.go` |
| **3D map viewer** | ❌ Not started | This ADR |

## Decision

### Architecture

Create a new `MapViewer` component that renders complete 3D maps by:
1. Building terrain mesh from GND data
2. Loading and placing RSM models at RSW positions
3. Providing FPS-style camera navigation

#### New File: `cmd/grfbrowser/map_viewer.go`

```go
type MapViewer struct {
    // Framebuffer resources
    fbo          uint32
    colorTexture uint32
    depthRBO     uint32
    width, height int32

    // Shaders
    terrainProgram uint32  // GND terrain shader
    modelProgram   uint32  // RSM model shader

    // Terrain mesh (GND)
    terrainVAO     uint32
    terrainVBO     uint32
    terrainEBO     uint32
    terrainGroups  []terrainTextureGroup

    // Ground textures and lightmaps
    groundTextures map[int]uint32
    lightmapAtlas  uint32

    // Model cache and instances
    modelCache     map[string]*CachedModel
    modelInstances []ModelInstance

    // FPS Camera
    cameraPos   [3]float32
    cameraYaw   float32
    cameraPitch float32
    moveSpeed   float32

    // Map data
    mapWidth, mapHeight int
    waterLevel float32
}
```

### Implementation Stages

#### Stage 1: GND Terrain Mesh
**Goal**: Render terrain as 3D mesh with textures

**Deliverables**:
- Terrain mesh generation from GND tiles
- Ground texture loading from GRF
- Basic terrain shader (diffuse only initially)
- Orbit camera for viewing

**Terrain Vertex Format**:
```go
type terrainVertex struct {
    Position  [3]float32  // World position
    Normal    [3]float32  // Surface normal
    TexCoord  [2]float32  // Diffuse texture UV
    Color     [4]float32  // Vertex color
}
```

**Mesh Generation**:
- Each GND tile has 4 corner altitudes
- TopSurface creates horizontal quad (2 triangles)
- FrontSurface creates vertical wall facing -Z
- RightSurface creates vertical wall facing +X
- Group triangles by texture ID for batched rendering

**Coordinate System**:
- RO: X=east, Y=up (negative values = higher), Z=south
- Tile size = GND.Zoom (typically 10.0 units)
- Map origin at corner, extends in +X and +Z

#### Stage 2: Lightmap Integration
**Goal**: Add lightmap blending for realistic lighting

**Deliverables**:
- Lightmap atlas texture generation
- UV calculation for atlas lookup
- Shader modification for lightmap blending

**Lightmap Atlas**:
- GND stores individual lightmaps (8x8 pixels each)
- Pack into single atlas texture (1024x1024)
- Calculate atlas UV during mesh generation

**Shader Update**:
```glsl
uniform sampler2D uDiffuse;
uniform sampler2D uLightmap;

void main() {
    vec4 diffuse = texture(uDiffuse, vTexCoord);
    vec3 light = texture(uLightmap, vLightmapUV).rgb;
    FragColor = vec4(diffuse.rgb * light, diffuse.a);
}
```

#### Stage 3: FPS Camera Navigation
**Goal**: WASD + mouse look camera for map exploration

**Controls**:
- W/S: Forward/backward
- A/D: Strafe left/right
- Space/Shift: Up/down
- Mouse drag: Look around
- Scroll: Adjust movement speed

**Implementation**:
```go
func (mv *MapViewer) HandleKeyboard(keys KeyState, deltaTime float32)
func (mv *MapViewer) HandleMouseLook(deltaX, deltaY float32)
func (mv *MapViewer) calculateViewMatrix() math.Mat4
```

#### Stage 4: RSM Model Placement
**Goal**: Load and render RSM models at RSW positions

**Process**:
1. Parse RSW to get model list
2. For each unique model path:
   - Load RSM from GRF
   - Build GPU mesh (reuse model_viewer.go patterns)
   - Store in cache
3. For each RSW placement:
   - Compute world transform from Position/Rotation/Scale
   - Store instance reference

**Transform Calculation**:
```go
func computeModelTransform(model *formats.RSWModel, gnd *formats.GND) math.Mat4 {
    tileSize := gnd.Zoom
    mapCenterX := float32(gnd.Width) * tileSize / 2.0
    mapCenterZ := float32(gnd.Height) * tileSize / 2.0

    worldX := model.Position[0] + mapCenterX
    worldY := -model.Position[1]  // Flip Y
    worldZ := model.Position[2] + mapCenterZ

    T := math.Translate(worldX, worldY, worldZ)
    R := math.RotateY(model.Rotation[1]) // Y rotation primary
    S := math.Scale(model.Scale[0], model.Scale[1], model.Scale[2])

    return T.Mul(R).Mul(S)
}
```

#### Stage 5: UI Integration
**Goal**: Add 3D view mode to RSW preview

**Changes**:
- Add "View 3D" button to RSW preview panel
- Toggle between 2D info panel and 3D viewer
- Show controls help overlay
- Add map statistics (models, textures, FPS)

**UI Layout**:
```
+----------------------------------+
| [2D Info] [3D View] [Reset]      |
+----------------------------------+
|                                  |
|      3D Rendered Map View        |
|                                  |
+----------------------------------+
| WASD: Move | Mouse: Look | ESC: Back |
+----------------------------------+
```

#### Stage 6: Optimizations (Future)
**Goal**: Handle large maps efficiently

**Optimizations**:
- Frustum culling for models
- Distance-based model LOD/culling
- Texture streaming for large maps
- Hardware instancing for repeated models

### Technical Considerations

#### Memory Budget
- Terrain mesh: ~5-20MB for large maps
- Ground textures: ~10-50MB (compressed)
- Model cache: ~50-200MB depending on map
- Lightmap atlas: ~4MB (1024x1024 RGBA)

#### Performance Targets
- 60 FPS on typical hardware
- Load time < 5 seconds for medium maps
- Support maps up to 512x512 tiles

### File Structure

```
cmd/grfbrowser/
├── main.go              # Add MapViewer to App
├── map_viewer.go        # NEW: 3D map renderer (~1000 lines)
├── preview_map.go       # Modify: Add 3D view toggle
├── model_viewer.go      # Existing: Extract shared utilities
└── utils.go             # Add transform helpers
```

## Consequences

### Positive
- Complete map visualization in GRF Browser
- Visual debugging for map development
- Foundation for game client map rendering
- Better understanding of RO map format

### Negative
- Significant OpenGL code (~1000 lines)
- Memory usage for large maps
- Coordinate system complexity

### Mitigations
- Stage 1 provides value without models
- Reuse patterns from model_viewer.go
- Progressive loading for large maps

## References

- [ADR-011: Map Format Parsers](./ADR-011-map-format-parsers.md)
- [ADR-012: 3D Model Viewer](./ADR-012-3d-model-viewer.md)
- [roBrowser Map Renderer](https://github.com/vthibault/roBrowser)
- [OpenKore Map Loader](https://github.com/OpenKore/openkore)
