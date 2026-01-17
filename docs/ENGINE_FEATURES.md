# Midgard RO Engine Features

This document summarizes the engine packages and features extracted from the GRF Browser tool, ready for use in the game client.

## Overview

The refactoring effort extracted ~1500+ lines of reusable code from `cmd/grfbrowser/` into modular engine packages following the project's layered architecture.

---

## 1. File Format Parsers (`pkg/formats/`)

Ragnarok Online file format parsers - pure Go, no external dependencies.

| Format | File | Description |
|--------|------|-------------|
| **SPR** | `spr.go` | Sprite images (indexed + RGBA) with palette support |
| **ACT** | `act.go` | Animation data (keyframes, layers, timing) |
| **GAT** | `gat.go` | Ground altitude/walkability data |
| **GND** | `gnd.go` | Ground mesh (tiles, textures, lightmaps) |
| **RSW** | `rsw.go` | World/map data (models, lights, sounds, water, fog) |
| **RSM** | `rsm.go` | 3D model data (nodes, meshes, animations) |
| **action_names.go** | Action/direction naming conventions for sprites |

### Action Names
```go
formats.DirectionNames      // ["S", "SW", "W", "NW", "N", "NE", "E", "SE"]
formats.MonsterActionNames  // ["Idle", "Walk", "Attack", "Damage", "Die", ...]
formats.PlayerActionNames   // ["Idle", "Walk", "Sit", "Pick Up", "Standby", ...]
formats.GetActionName(index, totalActions) string
formats.GetDirectionName(direction) string
```

---

## 2. Terrain System (`internal/engine/terrain/`)

Ground mesh generation and rendering utilities.

| File | Features |
|------|----------|
| **types.go** | `TerrainVertex`, `TerrainTile` structs |
| **mesh.go** | `GenerateTerrainMesh()` - converts GND to renderable vertices |
| **lightmap.go** | `BuildLightmapAtlas()` - packs lightmaps into texture atlas |
| **heightmap.go** | `GetHeightAt()` - terrain height sampling for collision |

### Key Functions
```go
terrain.GenerateTerrainMesh(gnd *formats.GND) ([]TerrainVertex, []uint32)
terrain.BuildLightmapAtlas(gnd *formats.GND, cellsPerRow int) ([]byte, int)
terrain.GetHeightAt(gnd *formats.GND, worldX, worldZ float32) float32
```

---

## 3. Model System (`internal/engine/model/`)

RSM 3D model processing for props and buildings.

| File | Features |
|------|----------|
| **types.go** | `ModelVertex`, `ModelInstance` structs |
| **mesh.go** | `BuildModelMesh()` - converts RSM nodes to vertices |
| **matrix.go** | `BuildNodeMatrix()` - hierarchical transforms |
| **animation.go** | `InterpolateRotKeys()`, `InterpolateScaleKeys()` |
| **math.go** | Axis-angle rotation, quaternion helpers |

### Key Functions
```go
model.BuildModelMesh(rsm *formats.RSM, animTimeMs float32) ([]ModelVertex, []uint32)
model.BuildNodeMatrix(node *formats.RSMNode, rsm *formats.RSM, animTimeMs float32) math.Mat4
model.InterpolateRotKeys(keys []formats.RSMRotKeyframe, timeMs float32) math.Quat
```

---

## 4. Water System (`internal/engine/water/`)

Animated water plane generation.

| File | Features |
|------|----------|
| **water.go** | Water mesh generation with wave animation |

### Key Functions
```go
water.GenerateWaterMesh(rsw *formats.RSW, gnd *formats.GND) []WaterVertex
water.CalculateWaveOffset(time, x, z, amplitude, frequency float32) float32
```

---

## 5. Character System (`internal/engine/character/`)

Player/NPC character handling with RO-style sprite animation.

| File | Features |
|------|----------|
| **types.go** | `Player` struct, `CompositeFrame`, `TerrainQuery` interface |
| **animation.go** | `UpdateAnimation()` - ACT-based frame advancement |
| **movement.go** | `UpdateMovement()` - click-to-move pathfinding |
| **direction.go** | `CalculateVisualDirection()` - hysteresis for smooth sprite facing |

### Key Features
- **Click-to-move**: Smooth movement towards destination
- **Direction hysteresis**: Prevents sprite flickering when near direction boundaries
- **ACT animation**: Proper frame timing from animation data
- **Terrain integration**: Height sampling via `TerrainQuery` interface

```go
character.UpdateAnimation(player *Player, act *formats.ACT, deltaMs float32)
character.UpdateMovement(player *Player, terrain TerrainQuery, deltaMs float32)
character.CalculateVisualDirection(cameraAngle float32, playerDir, lastSector int) (int, int)
```

---

## 6. Sprite System (`internal/engine/sprite/`)

Sprite rendering utilities for 2D billboards.

| File | Features |
|------|----------|
| **composite.go** | Multi-layer sprite compositing (body + head) |
| **shadow.go** | Shadow texture generation, billboard quads |

### Key Functions
```go
sprite.GenerateCircularShadow(size int, maxOpacity float32) []byte
sprite.GenerateShadowQuadVertices(shadowSize float32) []float32
sprite.GenerateBillboardQuadVertices() []float32
sprite.GenerateProceduralPlayer(width, height int) []byte
```

---

## 7. Camera System (`internal/engine/camera/`)

Camera implementations for 3D rendering.

| File | Features |
|------|----------|
| **camera.go** | `OrbitCamera`, `ThirdPersonCamera`, `FitBoundsToView()` |

### Camera Types
- **OrbitCamera**: Editor-style camera orbiting around a point
- **ThirdPersonCamera**: RO-style follow camera with fixed pitch

```go
camera.NewOrbitCamera() *OrbitCamera
camera.NewThirdPersonCamera() *ThirdPersonCamera
camera.FitBoundsToView(minBounds, maxBounds [3]float32, multiplier, minDist float32) FitResult
```

---

## 8. Picking System (`internal/engine/picking/`)

3D object selection via ray casting.

| File | Features |
|------|----------|
| **ray.go** | `Ray`, `AABB` structs, intersection tests |

### Key Functions
```go
picking.ScreenToRay(screenX, screenY, viewportW, viewportH float32, invViewProj math.Mat4) Ray
ray.IntersectPlaneY(planeY float32) (x, z float32, ok bool)
ray.IntersectAABB(box AABB) (t float32, hit bool)
picking.TransformAABB(localBbox [6]float32, position, scale [3]float32) AABB
```

---

## 9. Texture System (`internal/engine/texture/`)

Image decoding and processing.

| File | Features |
|------|----------|
| **tga.go** | TGA decoder (uncompressed + RLE), magenta key transparency |

### Key Functions
```go
texture.DecodeTGA(data []byte) (image.Image, error)
texture.IsMagentaKey(r, g, b uint8) bool
texture.ApplyMagentaKey(img *image.RGBA)
texture.ImageToRGBA(img image.Image, applyMagentaKey bool) *image.RGBA
```

---

## 10. Lighting System (`internal/engine/lighting/`)

Lighting utilities for 3D rendering.

| File | Features |
|------|----------|
| **sun.go** | Sun direction from RSW longitude/latitude |

```go
lighting.SunDirection(longitude, latitude int32) [3]float32
```

---

## 11. Debug Utilities (`internal/engine/debug/`)

Debug visualization helpers.

| File | Features |
|------|----------|
| **bbox.go** | Bounding box wireframe generation |

```go
debug.GenerateBBoxWireframeVertices(minX, minY, minZ, maxX, maxY, maxZ float32) []float32
debug.GenerateBBoxWireframeFromAABB(bbox [6]float32, pos, scale [3]float32, padding float32) []float32
```

---

## 12. Core Engine (`internal/engine/`)

Foundation packages for rendering.

| Package | Description |
|---------|-------------|
| **window/** | SDL2 window management |
| **input/** | Keyboard/mouse input handling |
| **renderer/** | OpenGL rendering abstraction |
| **shader/** | Shader compilation utilities |

---

## 13. Game Layer (`internal/game/`)

High-level game systems.

| Package | Description |
|---------|-------------|
| **entity/** | Entity definitions (`Character`, etc.) |
| **states/** | Game state machine |
| **world/** | World/map management |
| **game.go** | Main game loop |

---

## 14. Math Utilities (`pkg/math/`)

Vector, matrix, and quaternion math.

| Type | Operations |
|------|------------|
| **Vec2** | Add, Sub, Scale, Normalize, Dot, Length |
| **Vec3** | Add, Sub, Scale, Normalize, Dot, Cross, Length |
| **Mat4** | Identity, Translate, Scale, Rotate, Perspective, LookAt, Mul, TransformPoint |
| **Quat** | Normalize, Slerp, ToMat4 |

---

## Architecture Summary

```
cmd/grfbrowser/     → GRF Browser tool (uses all packages)
cmd/client/         → Game client entry point

internal/engine/    → Reusable engine components
  ├── camera/       → Camera systems
  ├── character/    → Character animation/movement
  ├── debug/        → Debug visualization
  ├── lighting/     → Lighting utilities
  ├── model/        → RSM model processing
  ├── picking/      → 3D selection
  ├── sprite/       → 2D sprite rendering
  ├── terrain/      → Ground mesh generation
  ├── texture/      → Image processing
  └── water/        → Water rendering

internal/game/      → Game-specific logic
  ├── entity/       → Game entities
  ├── states/       → State machine
  └── world/        → World management

pkg/                → Reusable libraries (no internal imports)
  ├── formats/      → RO file format parsers
  ├── grf/          → GRF archive reader
  └── math/         → Math utilities
```

---

## Refactoring Statistics

| Metric | Value |
|--------|-------|
| Engine packages created | 12 |
| Lines extracted from grfbrowser | ~1500+ |
| File format parsers | 6 (SPR, ACT, GAT, GND, RSW, RSM) |
| Commits | 6 refactoring commits |
