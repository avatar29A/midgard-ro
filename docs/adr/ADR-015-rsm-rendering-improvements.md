# ADR-015: RSM Model Rendering Improvements

## Status
Proposed

## Date
2026-01-14

## Context

During ADR-014 visual improvements implementation, we encountered RSM model rendering issues where some faces appear missing or broken (particularly visible on wooden crates/boxes). Investigation revealed that our current approach may be incomplete.

Deep research was conducted on two established open-source RO clients:
- **korangar** (Rust) - Modern, well-architected implementation
- **roBrowser** (JavaScript/WebGL) - Battle-tested web implementation

## Research Findings

### 1. Two-Sided Face Handling

**korangar approach:**
- Parses `TwoSide` flag from RSM face data
- At load time, generates **duplicate faces with reversed winding order and flipped normals**
- Pre-calculates back face count: `two_sided_face_count = faces.filter(f => f.two_sided != 0).count()`
- Allocates: `total_vertices = (face_count + two_sided_face_count) * 3`

```rust
// korangar implementation
if face.two_sided != 0 {
    Self::add_vertices(
        &mut vertices[back_face_index..],
        &vertex_positions,
        !reverse_order,  // Flip winding
        true,            // Reverse normal
    );
}
```

**roBrowser approach:**
- Parses `TwoSide` flag but **does NOT use it**
- Always renders single-sided with standard backface culling
- May cause same missing face issues we observed

**Our current approach:**
- Not handling `TwoSide` flag at all

### 2. Degenerate Triangle Handling

**korangar:**
- Uses epsilon check (`1e-5`) for normal magnitude
- Skips triangles that produce invalid normals (degenerate)
- Returns `None` for invalid normals, parent code skips adding vertices

```rust
const DEGENERATE_EPSILON: f32 = 1e-5;
let normal = delta1.cross(delta2);
match normal.magnitude() > DEGENERATE_EPSILON {
    true => Some(normal.normalize()),
    false => None,  // Skip degenerate
}
```

**Our current approach:**
- No degenerate triangle detection

### 3. Negative Scale Detection

**korangar:**
- Checks if model scale produces negative determinant
- If `scale.x * scale.y * scale.z < 0`, reverses winding order

```rust
let reverse_order = scale.iter().fold(1.0, |a, b| a * b).is_sign_negative();
```

### 4. Texture UV Clamping (Anti-Seaming)

**korangar:**
- Insets UV coordinates by half-pixel to prevent texture bleeding

```rust
let half_pixel = 0.5 / texture_width;
let u = half_pixel + original_u * (1.0 - 2.0 * half_pixel);
```

**roBrowser:**
- Uses `* 0.98 + 0.01` scaling for UVs

### 5. Three-Pass Transparency Rendering

**korangar implements:**
1. **Opaque Pass** - Fully opaque models
2. **Semi-Opaque Pass** - Opaque parts of models with transparent textures
3. **Transparent Pass** - Transparent parts (sorted back-to-front)

### 6. Z-Fighting Prevention

**korangar:**
- Adds per-node offset to sort order: `offset = node_index * 1.1920929e-4`
- Prevents z-fighting on overlapping nodes from same model

### 7. Vertex Deduplication

**korangar:**
- Post-load deduplication using HashMap before GPU upload
- Reduces bandwidth and improves cache efficiency

## Decision

Implement the following improvements in order of priority:

### Phase 1: Core Rendering Fixes (High Priority)
1. **TwoSide face handling** - Generate back faces at load time
2. **Degenerate triangle detection** - Skip invalid triangles
3. **Negative scale detection** - Reverse winding when needed

### Phase 2: Quality Improvements (Medium Priority)
4. **UV clamping** - Prevent texture seaming
5. **Z-fighting prevention** - Per-node depth offsets
6. **Vertex deduplication** - Optimize mesh data

### Phase 3: Advanced Rendering (Lower Priority)
7. **Three-pass transparency** - Proper alpha sorting
8. **Model sorting** - Back-to-front for transparent models

## Implementation Plan

### Phase 1 Changes (`cmd/grfbrowser/map_viewer.go`)

```go
// 1. TwoSide handling in buildMapModel
for _, face := range node.Faces {
    // Add front face
    addFaceVertices(face, vertices, false)

    // If TwoSide, add back face with reversed winding
    if face.TwoSide != 0 {
        addFaceVertices(face, vertices, true) // reversed
    }
}

// 2. Degenerate triangle check
func isValidNormal(v0, v1, v2 [3]float32) bool {
    e1 := sub(v1, v0)
    e2 := sub(v2, v0)
    normal := cross(e1, e2)
    return length(normal) > 1e-5
}

// 3. Negative scale detection
func shouldReverseWinding(scale [3]float32) bool {
    return scale[0] * scale[1] * scale[2] < 0
}
```

## Consequences

### Positive
- Missing faces on boxes/crates will be fixed
- Better visual parity with original RO client
- More robust handling of edge cases
- Performance optimization through deduplication

### Negative
- Increased vertex count for two-sided faces
- More complex loading code
- Need to test with variety of RSM models

## References

- korangar source: `~/git/RagnarokClients/korangar/`
  - Model loading: `korangar/src/loaders/model/mod.rs`
  - Node hierarchy: `korangar/src/world/model/node.rs`
  - Vertex structure: `korangar/src/graphics/vertices/model.rs`

- roBrowser source: `~/git/RagnarokClients/roBrowser/`
  - Model loading: `src/Loaders/Model.js`
  - Model rendering: `src/Renderer/Map/Models.js`

## Follow-up ADRs
- ADR-016: Autonomous Visual QA Pipeline
