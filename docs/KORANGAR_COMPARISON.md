# Korangar vs Midgard Graphics Comparison

Analysis of rendering differences between Korangar and our implementation.

## Screenshot Analysis

![Korangar Screenshot](../data/Korangar/Screenshot%202026-01-17%20at%207.35.44%20PM.png)

Korangar uses **Metal** backend (via wgpu) and shows noticeably better graphics quality.

---

## Key Differences

### 1. Shadow System

| Feature | Korangar | Midgard |
|---------|----------|---------|
| **Shadow Maps** | SDSM with cascades (2048-4096px) | Lightmaps only (baked) |
| **Shadow Methods** | Hard, PCF, PCSS | None (no real-time) |
| **Point Light Shadows** | Yes (cubemap shadows) | No |
| **Shadow Resolution** | Normal/Ultra/Insane presets | N/A |

**Korangar Implementation:**
- Sample Distribution Shadow Maps (SDSM) with partitions
- Multiple shadow cascade levels for distance-based quality
- PCF (Percentage Closer Filtering) for soft shadows
- PCSS (Percentage Closer Soft Shadows) for variable penumbra
- Shadow translucency support

**Impact:** Real-time shadows give depth and grounding to characters/objects.

---

### 2. Anti-Aliasing

| Feature | Korangar | Midgard |
|---------|----------|---------|
| **MSAA** | x2, x4, x8, x16 | None |
| **SSAA** | x2, x3, x4 | None |
| **FXAA** | Yes | None |

**Impact:** Jagged edges on our terrain/models vs smooth edges in Korangar.

---

### 3. Texture Filtering

| Feature | Korangar | Midgard |
|---------|----------|---------|
| **Sampler Types** | Nearest, Linear, Anisotropic | Linear only |
| **Anisotropic** | x4, x8, x16 | x8 (fixed) |
| **Mipmap Generation** | Yes | Yes |

**Impact:** Korangar textures look sharper at oblique angles.

---

### 4. Lighting System

| Feature | Korangar | Midgard |
|---------|----------|---------|
| **Ambient Light** | Configurable | From RSW |
| **Directional Light** | Real-time with shadows | Basic diffuse |
| **Point Lights** | Tiled light culling, shadows | None |
| **Light Count** | Many (tiled deferred) | 1 directional |

**Korangar Implementation:**
```
// Per-tile light culling
let tile_index = tile_y * tile_count_x + tile_x;
let light_count = light_count_texture.Load(...);

for (var index = 0; index < light_count; index++) {
    let light = point_lights[tile_light_indices[tile_index].indices[index]];
    // Calculate contribution with attenuation and shadows
}
```

**Impact:** RSW light sources (torches, etc.) actually emit light in Korangar.

---

### 5. Transparency Handling

| Feature | Korangar | Midgard |
|---------|----------|---------|
| **Method** | WBOIT (Weighted Blended OIT) | Simple alpha blend |
| **Alpha-to-Coverage** | Yes | No |
| **Order Independence** | Yes | No (depth sorting) |

**Korangar Implementation:**
```
// Weighted Blended Order-Independent Transparency
let weight = clamp(pow(min(1.0, alpha * 10.0) + 0.01, 3.0) * 1e8 *
             pow(view_z * 0.9, 3.0), 1e-2, 3e3);
output.accumulation = fragment_color * weight;
output.revealage = fragment_color.a;
```

**Impact:** Transparent objects (water, effects) render correctly regardless of order.

---

### 6. Vertex Features

| Feature | Korangar | Midgard |
|---------|----------|---------|
| **Vertex Colors** | Yes | No |
| **Wind Animation** | Yes (vertex shader) | No |

**Wind Implementation:**
```
let wind_position = world_position + float4(animation_timer);
let offset = float4(sin(wind_position.x), 0.0, sin(wind_position.z), 0.0) * wind_affinity;
let final_world_position = world_position + offset;
```

**Impact:** Foliage sways naturally in Korangar.

---

### 7. Post-Processing

| Feature | Korangar | Midgard |
|---------|----------|---------|
| **FXAA** | Yes | No |
| **Color Balance** | Yes | No |
| **Effect Rendering** | Dedicated pass | None |

---

## Priority Improvements for Midgard

### High Priority (Biggest Visual Impact)

1. **Real-time Shadows**
   - Add directional shadow mapping
   - Start with simple hard shadows, then PCF
   - Resolution: 2048x2048 minimum

2. **MSAA**
   - Add 4x MSAA support
   - Smooths terrain and model edges significantly

3. **Point Lights**
   - Parse RSW light objects
   - Basic point light contribution (no shadows initially)

### Medium Priority

4. **Better Transparency**
   - Implement alpha-to-coverage for vegetation
   - Consider WBOIT for complex scenes

5. **FXAA Post-Process**
   - Simple to implement
   - Good AA for UI and remaining jaggies

6. **Vertex Colors**
   - RSM models have per-vertex colors
   - Add to vertex format and shader

### Lower Priority

7. **Wind Animation**
   - Add wind_affinity to model vertices
   - Simple sin-based displacement

8. **PCSS Soft Shadows**
   - More complex, needs blocker search
   - Big quality improvement for shadows

---

## Implementation Roadmap

### Phase 1: Shadows
```
1. Add shadow map framebuffer (2048x2048 depth)
2. Render scene from light's perspective
3. Sample shadow map in terrain/model shaders
4. Add PCF for soft edges
```

### Phase 2: Lighting
```
1. Parse RSW light objects
2. Add point light uniform array
3. Calculate attenuation: 1.0 / (1.0 + dist * dist / (range * range))
4. Accumulate in fragment shader
```

### Phase 3: Anti-Aliasing
```
1. Create MSAA framebuffer
2. Resolve to screen texture
3. Add FXAA post-process pass (optional)
```

---

## Technical Notes

### Korangar Tech Stack
- **Language:** Rust
- **Graphics API:** wgpu (Vulkan/Metal/DX12)
- **Shader Language:** Slang (compiles to SPIR-V/WGSL)
- **Rendering:** Forward+ with tiled light culling

### Our Tech Stack
- **Language:** Go
- **Graphics API:** OpenGL 4.1 Core
- **Shader Language:** GLSL 410
- **Rendering:** Forward (single pass)

### Shader Complexity Comparison
| Shader | Korangar LOC | Midgard LOC |
|--------|-------------|-------------|
| Model Fragment | ~225 | ~30 |
| Terrain Fragment | Similar | ~60 |

The difference is primarily shadow sampling and point light loops.

---

## Conclusion

The main visual quality gap comes from:
1. **No real-time shadows** (50% of the difference)
2. **No MSAA** (25% of the difference)
3. **No point lights** (15% of the difference)
4. **No WBOIT** (10% for transparency quality)

Adding shadow mapping and MSAA would close most of the visual gap.
