# Terrain Rendering Research: Korangar Reference

## Summary

This document summarizes how Korangar renders RO terrain (GND files). All findings are based on Korangar source code analysis.

**Source Files:**
- `korangar/src/loaders/map/vertices.rs` - Terrain vertex generation
- `korangar/src/loaders/smoothing.rs` - Normal smoothing algorithm

---

## Key Findings

### 1. Height Array Mapping

GND stores 4 corner heights per tile (from `vertices.rs:358-365`):

```rust
Heights::SouthWest => tile.southwest_corner_height,  // [0]
Heights::SouthEast => tile.southeast_corner_height,  // [1]
Heights::NorthWest => tile.northwest_corner_height,  // [2]
Heights::NorthEast => tile.northeast_corner_height,  // [3]
```

Grid layout:
```
    NW[2]---NE[3]
      |       |
      |       |
    SW[0]---SE[1]
```

**Important:** Heights are negated when used (Y points down in Korangar's coordinate system).

---

### 2. Surface Types

Korangar processes three surface types per tile (`vertices.rs:38`):

| Surface Type | Description | Neighbor Connection |
|--------------|-------------|---------------------|
| `Top` | Ground surface | Uses current tile heights |
| `North` | Front wall | Connects to tile at y+1 |
| `East` | Right wall | Connects to tile at x+1 |

---

### 3. UV Coordinate Mapping

**Direct mapping** - UV indices match height indices (`vertices.rs:191-206`):

```rust
first_texture_coordinates  = ground_surface.u[0], ground_surface.v[0]  // SW corner
second_texture_coordinates = ground_surface.u[1], ground_surface.v[1]  // SE corner
third_texture_coordinates  = ground_surface.u[2], ground_surface.v[2]  // NW corner
fourth_texture_coordinates = ground_surface.u[3], ground_surface.v[3]  // NE corner
```

**Half-pixel inset** to prevent texture bleeding (`vertices.rs:188-189`):

```rust
let half_pixel_width = 0.5 / texture.width as f32;
let half_pixel_height = 0.5 / texture.height as f32;

// Formula for each UV coordinate:
uv_adjusted = half_pixel + uv * (1.0 - 2.0 * half_pixel)
```

For 256x256 textures: `half_pixel = 0.5/256 = 0.001953125`

---

### 4. Per-Triangle Normals (CRITICAL)

Korangar uses **6 vertices per quad** with **per-triangle normals** (`vertices.rs:83-84`):

```rust
let first_normal = NativeModelVertex::calculate_normal(first_position, second_position, third_position);
let second_normal = NativeModelVertex::calculate_normal(third_position, second_position, fourth_position);
```

**Triangle 1** (vertices 0,1,2): SW → SE → NW with `first_normal`
**Triangle 2** (vertices 2,1,3): NW → SE → NE with `second_normal`

Triangle layout (diagonal from SE to NW):
```
    NW---NE
    | \ |
    |  \|
    SW---SE

    Triangle 1: SW-SE-NW (bottom-left)
    Triangle 2: NW-SE-NE (top-right)
```

---

### 5. Normal Smoothing Algorithm

From `smoothing.rs:41-80`:

**Step 1: Detect artificial (wall) vertices**

Vertices connected by vertical edges (same X and Z, different Y) are marked as "artificial":

```rust
const EPSILON: f32 = 1e-6;

for vertex_index in 0..3 {
    let position0 = chunk[vertex_index].position;
    let position1 = chunk[(vertex_index + 1) % 3].position;

    // Vertical edge detection
    if (position0.x - position1.x).abs() < EPSILON
       && (position0.z - position1.z).abs() < EPSILON {
        artificial_vertices[index0] = true;
        artificial_vertices[index1] = true;
    }
}
```

**Step 2: Sum normals by position**

Non-artificial vertices at the same position have their normals accumulated:

```rust
for vertex in vertices.iter().filter(|v| !is_artificial) {
    let position = VertexPosition::new(vertex.position);
    *normals.entry(position).or_insert_with(Vector3::zero) += vertex.normal;
}
```

**Step 3: Normalize and apply**

```rust
for (_, normal) in normals.iter_mut() {
    *normal = normal.normalize();
}

for vertex in vertices.iter_mut() {
    if let Some(&smooth_normal) = normals.get(&position) {
        vertex.normal = smooth_normal;
    }
}
```

**Key insight:** Wall vertices keep their original normals; ground vertices get smoothed normals.

---

### 6. Neighbor Color Blending

Each corner of a tile can have a different color from neighboring tiles (`vertices.rs:91-107`):

```rust
let neighbor_color = |x_offset: usize, y_offset: usize| {
    let neighbor_tile = ground_tiles.get(tile_x + x_offset + (tile_y + y_offset) * width);
    let neighbor_surface_index = tile_surface_index(neighbor_tile, SurfaceType::Top);
    neighbor_surface.color.into()
};

// Corner colors:
let color_sw = ground_surface.color;                // Current tile (0, 0)
let color_se = neighbor_color(1, 0);                // East neighbor
let color_nw = neighbor_color(0, 1);                // North neighbor
let color_ne = neighbor_color(1, 1);                // Northeast neighbor
```

Corner assignment to vertices:
```
Triangle 1: SW → SE → NW
            color_sw → color_se → color_nw

Triangle 2: NW → SE → NE
            color_nw → color_se → color_ne
```

---

### 7. Tile Grid Visualization (Debug Feature)

From `vertices.rs:216-348`, Korangar renders tile grid for debugging:

**GAT tile size**: Defined as `GAT_TILE_SIZE` constant

**Height offset**: Tiles are rendered slightly above ground (`TILE_MESH_OFFSET = 0.9`)

**Tile position calculation:**
```rust
let offset = Vector2::new(x as f32 * GAT_TILE_SIZE, y as f32 * GAT_TILE_SIZE);

let first_position = Point3::new(offset.x, tile.southwest_corner_height + TILE_MESH_OFFSET, offset.y);
let second_position = Point3::new(offset.x + GAT_TILE_SIZE, tile.southeast_corner_height + TILE_MESH_OFFSET, offset.y);
let third_position = Point3::new(offset.x, tile.northwest_corner_height + TILE_MESH_OFFSET, offset.y + GAT_TILE_SIZE);
let fourth_position = Point3::new(offset.x + GAT_TILE_SIZE, tile.northeast_corner_height + TILE_MESH_OFFSET, offset.y + GAT_TILE_SIZE);
```

**Color coding**: Uses tile flags to determine tile type visualization.

---

### 8. Position Scaling

**Ground tiles**: Positions scaled by `GROUND_TILE_SIZE` (`vertices.rs:51-53`)
```rust
Point3::new(
    (tile_x + surface_offset.x) as f32 * GROUND_TILE_SIZE,
    -height,
    (tile_y + surface_offset.y) as f32 * GROUND_TILE_SIZE,
)
```

**GAT tiles**: Positions scaled by `GAT_TILE_SIZE` (typically half of ground tile size)

---

## Our Implementation Status

### ✅ Implemented (Matching Korangar)
1. 6 vertices per quad with per-triangle normals
2. Half-pixel UV inset (0.5/256)
3. Normal smoothing with vertical edge detection
4. Neighbor color blending

### ⚠️ Differences from Korangar
1. **UV mapping**: We use swapped mapping (UV[2]→corner[0]) that works for Prontera
   - Korangar uses direct mapping (UV[0]→corner[0])
   - May need investigation if other maps have issues

2. **Y coordinate direction**: Our Y increases north, Korangar's increases south
   - We use `y-1` for north neighbors instead of `y+1`

### 🔲 TODO
1. Implement tile grid debug visualization
2. Investigate height-related artifacts on sloped terrain (prt_fild)

---

## Reference Links

- **Korangar Source**: https://github.com/vE5li/korangar
- **Korangar Map Loader**: `korangar/src/loaders/map/`
