# ADR-011: Map Format Parsers (GAT/GND/RSW)

## Status
Proposed

## Context

Ragnarok Online maps consist of three interrelated file formats:
1. **GAT** (Ground Altitude Table) - Walkability and collision data
2. **GND** (Ground) - Terrain mesh, textures, and lightmaps
3. **RSW** (Resource World) - World settings, object placements, lighting, water, sound

To provide map preview in the GRF Browser and eventually render full maps in the client, we need parsers for all three formats.

### Format Overview

| Format | Purpose | Complexity | Dependencies |
|--------|---------|------------|--------------|
| GAT | 2D collision grid | Low | None |
| GND | 3D terrain mesh | Medium | Textures (BMP/TGA) |
| RSW | World container | High | GND, GAT, RSM models |

### Use Cases

1. **GRF Browser**: Preview map data (walkability, terrain)
2. **Client**: Full map rendering with 3D terrain and objects
3. **Development**: Debug pathfinding, collision, spawn points

## Decision

### 1. File Format Specifications

#### 1.1 GAT Format (Ground Altitude Table)

```
Header:
  magic: char[4]       = "GRAT"
  version: uint8[2]    = [major, minor] (1.2 common)
  width: uint32        = map width in cells
  height: uint32       = map height in cells

Cells[width * height]:
  altitude[4]: float32 = corner heights (bottom-left, bottom-right, top-left, top-right)
  type: uint32         = cell type flags
```

**Cell Types:**
| Value | Type | Description |
|-------|------|-------------|
| 0 | Walkable | Normal ground |
| 1 | Blocked | Cannot walk |
| 2 | Water | Water surface (walkable with aqua skill) |
| 3 | Walkable+Water | Shore area |
| 4 | Cliff/Snipeable | Can attack over but not walk |
| 5 | Blocked+Snipeable | Blocked but can shoot over |

#### 1.2 GND Format (Ground)

```
Header:
  magic: char[4]          = "GRGN"
  version: uint8[2]       = [major, minor] (1.7 common)
  width: uint32           = terrain width in tiles
  height: uint32          = terrain height in tiles
  zoom: float32           = scale factor
  texture_count: uint32
  texture_name_length: uint32

Textures[texture_count]:
  name: char[texture_name_length]  = null-terminated texture path

Lightmaps:
  lightmap_count: uint32
  lightmap_width: uint32   = typically 8
  lightmap_height: uint32  = typically 8
  lightmap_cells: uint32   = typically 1
  Lightmaps[lightmap_count]:
    brightness: uint8[w*h]
    color_rgb: uint8[w*h*3]

Surfaces:
  surface_count: uint32
  Surfaces[surface_count]:
    u[4], v[4]: float32   = texture coordinates
    texture_id: int16     = -1 = no texture
    lightmap_id: int16
    color: uint8[4]       = BGRA vertex color

Tiles[width * height]:
  altitude[4]: float32    = corner heights
  surface_ids[2]: int32   = top surface, front surface (-1 = none)
  side_surface_id: int32  = right surface (-1 = none)
```

#### 1.3 RSW Format (Resource World)

```
Header:
  magic: char[4]       = "GRSW"
  version: uint8[2]    = [major, minor] (2.1 common)
  ini_file: char[40]   = settings file reference
  gnd_file: char[40]   = ground file name
  gat_file: char[40]   = altitude file name (v1.4+)
  src_file: char[40]   = source file (v1.4+)

Water (v1.3+):
  water_level: float32
  water_type: int32
  wave_height: float32
  wave_speed: float32
  wave_pitch: float32
  water_anim_speed: int32

Light (v1.5+):
  longitude: int32     = sun direction
  latitude: int32
  diffuse: float32[3]  = RGB
  ambient: float32[3]  = RGB

Ground (v1.6+):
  top: int32, bottom: int32, left: int32, right: int32  = view bounds

Objects:
  object_count: uint32
  Objects[object_count]:
    type: int32
    -- Type 1: Model (RSM) --
    name: char[40]
    anim_type: int32
    anim_speed: float32
    block_type: int32
    model_name: char[80]
    node_name: char[80]
    position: float32[3]
    rotation: float32[3]
    scale: float32[3]
    -- Type 2: Light Source --
    name: char[80]
    position: float32[3]
    color: float32[3]
    range: float32
    -- Type 3: Sound Source --
    name: char[80]
    file: char[80]
    position: float32[3]
    volume: float32
    width: int32, height: int32
    range: float32
    cycle: float32 (v2.0+)
    -- Type 4: Effect --
    name: char[80]
    position: float32[3]
    effect_id: int32
    delay: float32
    param: float32[4]

Quadtree (v2.1+):
  range: float32[4][4]  = AABB for scene partitioning
```

### 2. Implementation Stages

#### Stage 1: GAT Parser + 2D Viewer ✅ PRIORITY
**Goal**: Parse GAT, visualize walkability in GRF Browser

Deliverables:
- `pkg/formats/gat.go` - GAT parser
- `pkg/formats/gat_test.go` - Unit tests
- GRF Browser: 2D heatmap viewer for GAT files

Visualization:
```
Color coding:
- Green:  Walkable (type 0)
- Red:    Blocked (type 1)
- Blue:   Water (type 2, 3)
- Yellow: Snipeable (type 4, 5)
- Gray:   Unknown
```

#### Stage 2: GND Parser + Wireframe Viewer
**Goal**: Parse GND, display terrain wireframe

Deliverables:
- `pkg/formats/gnd.go` - GND parser
- `pkg/formats/gnd_test.go` - Unit tests
- GRF Browser: 3D wireframe terrain viewer (simple OpenGL)

#### Stage 3: GND Textured Rendering
**Goal**: Render terrain with textures and lightmaps

Deliverables:
- Texture loading from GRF
- Lightmap blending
- GRF Browser: Full textured terrain preview

#### Stage 4: RSW Parser
**Goal**: Parse world file, extract object positions

Deliverables:
- `pkg/formats/rsw.go` - RSW parser
- `pkg/formats/rsw_test.go` - Unit tests
- Object list in properties panel

#### Stage 5: RSM Parser + Model Viewer (Future)
**Goal**: Parse 3D models, render on terrain

Deliverables:
- `pkg/formats/rsm.go` - RSM parser
- Model preview in GRF Browser
- Objects placed on terrain

### 3. Package Structure

```
pkg/formats/
├── gat.go           # GAT parser
├── gat_test.go
├── gnd.go           # GND parser
├── gnd_test.go
├── rsw.go           # RSW parser
├── rsw_test.go
└── rsm.go           # RSM parser (future)
```

### 4. Data Structures

```go
// GAT structures
type GAT struct {
    Version  GATVersion
    Width    uint32
    Height   uint32
    Cells    []GATCell
}

type GATCell struct {
    Heights [4]float32  // Corner altitudes
    Type    GATCellType // Walkability flags
}

type GATCellType uint32

const (
    GATWalkable      GATCellType = 0
    GATBlocked       GATCellType = 1
    GATWater         GATCellType = 2
    GATWalkableWater GATCellType = 3
    GATSnipeable     GATCellType = 4
    GATBlockedSnipe  GATCellType = 5
)

// GND structures
type GND struct {
    Version      GNDVersion
    Width        uint32
    Height       uint32
    Zoom         float32
    Textures     []string
    Lightmaps    []GNDLightmap
    Surfaces     []GNDSurface
    Tiles        []GNDTile
}

type GNDTile struct {
    Heights    [4]float32
    TopSurface int32
    FrontSurface int32
    RightSurface int32
}

// RSW structures
type RSW struct {
    Version    RSWVersion
    GNDFile    string
    GATFile    string
    Water      RSWWater
    Light      RSWLight
    Objects    []RSWObject
}

type RSWObject struct {
    Type     RSWObjectType
    Name     string
    Position [3]float32
    Rotation [3]float32
    Scale    [3]float32
    // Type-specific fields...
}
```

### 5. GRF Browser Integration

#### GAT Viewer (Stage 1)
```
┌─────────────────────────────────────┐
│ prontera.gat                        │
├─────────────────────────────────────┤
│ Size: 400 x 400 cells               │
│ Version: 1.2                        │
├─────────────────────────────────────┤
│ ┌─────────────────────────────────┐ │
│ │ [2D Grid Visualization]         │ │
│ │  ████████████████████████████   │ │
│ │  ██  ██████  ████  ████  ████   │ │
│ │  ██  ██████  ████  ████  ████   │ │
│ │  ████████████████████████████   │ │
│ └─────────────────────────────────┘ │
│ Legend: ■Walkable ■Blocked ■Water   │
│ Zoom: [-] 100% [+]                  │
└─────────────────────────────────────┘
```

#### Controls
- Mouse drag: Pan view
- Scroll: Zoom in/out
- Hover: Show cell info (x, y, type, altitude)

### 6. Technical Considerations

#### Performance
- Large maps: 400x400 = 160,000 cells
- Render to texture, display as image
- Only re-render on zoom/pan

#### Memory
- GAT: ~2.5MB for 400x400 map (16 bytes/cell)
- GND: Variable, textures loaded on demand
- RSW: Small header, objects indexed

#### Version Handling
- Support GAT 1.2 (most common)
- Support GND 1.5-1.8
- Support RSW 1.2-2.1
- Graceful fallback for unknown versions

## Consequences

### Positive
- Complete map data access for development
- Visual debugging for pathfinding/collision
- Foundation for full map rendering in client
- Modding tool capabilities (map editing)

### Negative
- Complex formats with many versions
- 3D rendering requires significant effort
- RSM models are separate complex format

### Mitigations
- Stage 1 (GAT only) provides immediate value
- 2D visualization avoids 3D complexity initially
- Can skip RSM and still have useful terrain viewer

## References

- [RO Map Format Documentation](http://mist.in/gratia/doc/gnd.html)
- [OpenKore GAT Parser](https://github.com/OpenKore/openkore)
- [roBrowser Map Loader](https://github.com/nickstelter/robmern-web)
- [Borf's GRF Tool](https://github.com/Borf/broern)
- [ADR-006: GRF Archive Reader](./ADR-006-grf-archive-reader.md)
- [ADR-009: GRF Browser Tool](./ADR-009-grf-browser-tool.md)
