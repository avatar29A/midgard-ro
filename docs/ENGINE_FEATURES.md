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

Water geometry, built per cell by the original client's rule.

| File | Features |
|------|----------|
| **water.go** | `BuildCells`: a quad for every GND cell that has ground with a corner below the water level (roBrowser `Ground.js:471-483`); cells with no ground get none, so an indoor map's void stays black. `BuildPlane` is the older map-sized quad, kept for grfbrowser. |

### Key Functions
```go
water.BuildCells(gnd *formats.GND, level, waveHeight float32) *water.Mesh   // Vertices, Cells
water.BuildPlaneWithPadding(minX, maxX, minZ, maxZ, level, padding float32) *water.Plane
water.CalculateAnimFrame(time, speed float32, numFrames int) int
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
| **states/** | Game state machine, and the player's live stats (`player_stats.go`) |
| **ui/** | The 2D interface: the HUD (`hud_*.go`) and the menu windows (`win_*.go`) |
| **world/** | World/map management |
| **game.go** | Main game loop |

### The interface is drawn natively, not with ImGui

Every widget is drawn by `internal/engine/ui2d` against the archive's own
bitmaps. ImGui remains only as the platform layer — window, GL context and
raw input — behind `ui2d_backend.go`, and `git grep -l cimgui-go --
internal/game/ui` returning anything else means a widget has crept back.

What that layer can and cannot do is worth knowing before adding to it:

- **Drawn later is on top.** Images, solids and text all go into one command
  list in the order they are asked for, and the flush walks it in that order.
  There is no rule about primitive types to remember and no layer to opt into.
- There is no scissor. Text that must not overflow its box is trimmed to fit
  rather than clipped.
- A window's remembered state outlives the window. Closing or minimizing sets
  a flag that `BeginWindow` refuses on, so anything offering a way to reopen
  one has to clear it (`OpenWindow`), and a false return has to be read with
  `WindowClosed` rather than assumed to mean closed.

The first of those was not always true, and the cost of it not being true is
worth remembering. The renderer used to batch by primitive type and flush
images, then solids, then text, so a solid always covered an image and text
covered both, whatever order the calls were made in. That produced five bugs
that looked unrelated — an invisible window background, invisible skill and
item icons, health bars drawn over a map, and every label on the HUD floating
above every window — and four of them were worked around one at a time before
the fifth turned out to be unworkaroundable. The command list replaced three
mechanisms (`FillImageLayer`, `WindowOptions.BitmapBody` and a manual layer
flush) with one rule.

The cursor is still drawn in its own pass above everything, which is a
statement about the cursor rather than an exception to the rule.

### The HUD and its windows

Always on: the minimap, the chat box, the hotkey bar, the bars under the
character and the click marker. Behind the menu buttons: Status, Skill Tree,
Inventory and Map. Escape opens the game menu, which leaves the game through
the protocol rather than by exiting the process.

Names the packets no longer carry — skills and items — are generated from the
server's own database into `internal/game/skills` and `internal/game/items`,
because this archive's tables are Korean. See `tools/skillnames` and
`tools/itemnames`.

### NPC conversations (`internal/game/states/npcdialog.go`, `internal/game/ui/npcdialog.go`, `npcmenu.go`)

Click an NPC and talk to it. The server owns the conversation entirely — it
sends text, then asks for a Next, a Close or a choice from a menu — so the
client is a state machine (`DialogPhase`) and two windows. Neither has a title
bar; both are plain panels, both drag, and both scroll with the original's
`scroll0*` scrollbar.

What the NPC says arrives one `mes` line per packet and accumulates, with a
blank line at each page boundary. The text carries inline markup the server
passes through untouched: `^RRGGBB` color codes and `<NAVI>[label]<INFO>…</INFO></NAVI>`
navigation links, both handled in `npctext.go` — without that, the raw codes
print on screen.

A menu selection is one-based and counts only non-empty entries; an index
outside that range is refused before it is sent, because rAthena kicks the
connection for it rather than reporting an error. `--trace=npc` shows a
conversation as a sequence.

### Basic Info HUD (`internal/game/ui/hud_basic_info.go`)

The original client's top-left panel: name, job, HP and SP gauges, Base and
Job level with experience bars, weight and Zeny, and the ten menu buttons.
Drawn over `basic_interface/basewin_bg2.bmp`, which has the chrome painted in
— every piece of text is ours, the window caption included.

Ctrl+V or the title bar's system button folds it to its reduced form, which is
the same bitmap clipped to its top 53 rows. The panel drags by its title bar.

The values come from `states.PlayerStats`, seeded from the character list and
kept current by the server's parameter packets — `0x00B0`, `0x00B1` and, for
experience at `PACKETVER >= 20170830`, `0x0ACB`. `--trace=status` shows each
update as it arrives.

---

### Map loading (`internal/game/states/maploader.go`, `internal/engine/scene/scene.go`)

A map is loaded in phases — GAT, GND, RSW, prepare, terrain, models in
chunks, finish — by a `MapLoader` that `InGameState` steps once per frame with
a 24 ms budget, so the loading screen (`internal/game/ui/loading_screen.go`:
one of the archive's `loading01..10.jpg`, the original's 240×15 bar) draws
between phases with a bar that moves because work was done. `Scene` exposes
the phases (`BeginMap`, `LoadTerrain`, `BeginModels`, `LoadModelRange`,
`EndMap`); `LoadMap` runs them back to back. The state survives a map change:
`ZC_NPCACK_MAPMOVE` drops the units, the dialog and the walk, loads the new
map, and only then sends `CZ_NOTIFY_ACTORINIT`. Prontera loads in ~1.3 s.

```go
l := states.NewMapLoader("prontera", load, scene)   // scene satisfies MapSink
for !l.Step() { /* draw the loading screen; l.Progress(), l.Phase() */ }
l.Err(); l.GAT(); l.TimingSummary()
```

### Warp portals (`internal/engine/scene/portal.go`)

Class-45 NPCs are drawn as the original's portal effect (`EF_WARPZONE2`)
rather than the sprite the client's job table names: a twenty-sided tube
wrapped in `data/texture/effect/ring_blue.tga`, spun a quarter degree per
millisecond, tinted blue, over a soft disc. `PortalRenderer` belongs to
`InGameState` alongside the player renderer.

```go
pr, _ := scene.NewPortalRenderer()
pr.LoadTextures(load)
pr.Render(viewProj, x, y, z, timeMs, alpha)
```

### Map camera rules (`pkg/formats/maptables.go`, `internal/engine/camera`, `internal/game/states/maprules.go`)

`data/indoorrswtable.txt` lists the maps where the original disables orbital
rotation; `data/viewpointtable.txt` gives a few maps an arc and an entry
angle. `formats.ParseIndoorRSWTable` / `ParseViewpointTable` read them,
`Manager.MapRules()` loads them once per session, and `InGameState` applies
them to the `ThirdPersonCamera` as `camera.Limits` on every map. Indoor maps
also clear to black instead of sky.

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
