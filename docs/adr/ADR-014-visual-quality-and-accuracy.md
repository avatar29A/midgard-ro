# ADR-014: Visual Quality and Rendering Accuracy

## Status
Proposed

## Context

### The Problem

After completing ADR-013 (3D Map Viewer), we have a technically functional map renderer that produces "90s low budget" visuals compared to the actual Ragnarok Online client. Three critical issues:

1. **Guessing-based development**: Model positioning fixes involve trial-and-error with random changes, wasting significant verification time. We lack systematic understanding of how RO actually calculates transforms.

2. **Visual artifacts**: Small rendering glitches across models indicate fundamental issues with our rendering approach (alpha handling, culling, z-fighting).

3. **Missing atmosphere**: The game has warmth, depth, and visual appeal. Our viewer renders geometry correctly but lacks the visual qualities that make maps feel alive.

### Root Cause Analysis

**What we parse but don't use correctly:**

| Data | Source | Current Usage | Correct Usage |
|------|--------|---------------|---------------|
| Sun direction | RSW.Light.Longitude/Latitude | Hardcoded (0.5, 1.0, 0.3) | Spherical→cartesian conversion |
| Ambient color | RSW.Light.Ambient | Hardcoded (0.4, 0.4, 0.4) | Use actual RGB values |
| Diffuse color | RSW.Light.Diffuse | Hardcoded (0.8, 0.8, 0.7) | Use actual RGB values |
| Point lights | RSW.Lights[] | Not rendered | **Already baked in lightmaps** |
| Lightmaps | GND.Lightmaps | Blended with brightness hacks | Proper multiplicative blending |
| Water level | RSW.Water.Level | Ignored | Render water plane |
| Model alpha | RSM.Alpha | Ignored | Apply per-model transparency |
| Two-sided flag | RSM.Face.TwoSide | Ignored | Disable backface culling |
| Shading type | RSM.Shading | Ignored | Flat vs smooth normals |

**Critical insight from research**: Point lights in RSW are NOT meant to be rendered dynamically. Their lighting is pre-baked into GND lightmaps. We should NOT implement real-time point lighting.

### Reference Implementations

| Project | Language | Rendering Quality | Source |
|---------|----------|-------------------|--------|
| [Korangar](https://github.com/vE5li/korangar) | Rust/Vulkan | Excellent (modern) | Best reference for quality |
| [RagnarokRebuild](https://github.com/Doddler/RagnarokRebuild) | C#/Unity | Good (faithful) | Best reference for accuracy |
| [roBrowser](https://github.com/nicop83/roBrowserLegacy) | JavaScript/WebGL | Good | Well-documented |
| [Ragnarok Research Lab](https://ragnarokresearchlab.github.io/) | Documentation | N/A | Authoritative format docs |

## Decision

### Philosophy: Reference-Driven Development

**Stop guessing. Start comparing.**

Every rendering change must be:
1. Based on studying reference implementation code
2. Validated against in-game screenshots
3. Implemented incrementally with visual A/B comparison

### Architecture: Staged Visual Improvements

```
Stage 1: Correct Lighting (Foundation)
    ↓
Stage 2: Model Positioning Fix (Systematic)
    ↓
Stage 3: Artifact Elimination (Polish)
    ↓
Stage 4: Atmosphere (Water, Fog)
    ↓
Stage 5: Validation Framework (Prevent Regression)
```

---

## Implementation Stages

### Stage 1: Correct Lighting (3-4 hours)

**Goal**: Use actual RSW lighting data instead of hardcoded values.

#### 1.1 Sun Direction from Spherical Coordinates

RSW stores sun position as longitude (azimuth) and latitude (elevation) in degrees:

```go
// Convert RSW spherical coordinates to directional light vector
func calculateSunDirection(longitude, latitude float32) [3]float32 {
    // Convert degrees to radians
    lonRad := longitude * math.Pi / 180.0
    latRad := latitude * math.Pi / 180.0

    // Spherical to Cartesian conversion
    // Latitude: 0° = horizon, 90° = directly overhead
    // Longitude: angle around Y axis
    x := float32(math.Cos(float64(latRad)) * math.Sin(float64(lonRad)))
    y := float32(math.Sin(float64(latRad)))
    z := float32(math.Cos(float64(latRad)) * math.Cos(float64(lonRad)))

    return [3]float32{x, y, z}
}
```

#### 1.2 Use RSW Ambient/Diffuse Colors

```go
// In LoadMap(), after parsing RSW:
mv.ambientColor = [3]float32{
    rsw.Light.Ambient[0],
    rsw.Light.Ambient[1],
    rsw.Light.Ambient[2],
}
mv.diffuseColor = [3]float32{
    rsw.Light.Diffuse[0],
    rsw.Light.Diffuse[1],
    rsw.Light.Diffuse[2],
}
mv.lightDir = calculateSunDirection(rsw.Light.Longitude, rsw.Light.Latitude)
```

#### 1.3 Remove Brightness Hacks

Current terrain shader has artificial boosts:
```glsl
// REMOVE THESE HACKS:
vec3 lighting = (uAmbient + directional + vec3(0.2)) *
                (lightmapColor * 1.2 + vec3(0.5)) * vColor.rgb;
```

Replace with proper formula:
```glsl
// CORRECT:
vec3 lighting = uAmbient + uDiffuse * max(dot(normal, uLightDir), 0.0);
vec3 finalColor = texColor.rgb * lightmapColor * lighting * vColor.rgb;
```

#### 1.4 Validation

- Screenshot comparison: Alberta with our lighting vs game screenshot
- Lighting should feel similar in direction and warmth
- No artificial brightness or washed-out appearance

---

### Stage 2: Model Positioning Fix (4-6 hours)

**Goal**: Systematic, reference-based approach to model transforms.

#### 2.1 Study Reference Implementation

Before writing any code, extract the exact algorithm from RagnarokRebuild:

```
Task: Read RagnarokRebuild/Assets/Scripts/MapEditor/Editor/RagnarokModelLoader.cs
Focus: Line ~340 where models are centered
Extract: Exact bounding box calculation and centering formula
```

#### 2.2 Implement Debug Visualization

Create visual debugging tools instead of guessing:

```go
// Add to MapViewer
type ModelDebugInfo struct {
    Name          string
    RSWPosition   [3]float32  // Raw from RSW
    RSWRotation   [3]float32  // Raw from RSW
    BoundingBox   BoundingBox // Calculated
    CenterOffset  [3]float32  // Applied offset
    FinalPosition [3]float32  // World position
}

func (mv *MapViewer) RenderDebugOverlay() {
    // Draw bounding boxes
    // Draw position markers
    // Show transform chain
}
```

#### 2.3 Correct Transform Order

Reference implementations use specific transform order. Document and implement exactly:

```go
// Transform application order (from reference study):
// 1. Scale
// 2. Rotation (specific axis order - determine from reference)
// 3. Translation (including bounding box centering)
// 4. Parent node transform (if applicable)

func computeModelWorldTransform(rsw *formats.RSWModel, rsm *formats.RSM, gnd *formats.GND) Mat4 {
    // Step 1: Calculate model bounding box in local space
    bbox := calculateModelBounds(rsm)

    // Step 2: Centering offset (from reference)
    centerX := (bbox.Max[0] + bbox.Min[0]) / 2.0
    centerZ := (bbox.Max[2] + bbox.Min[2]) / 2.0
    groundY := bbox.Min[1]  // Put base on ground

    // Step 3: Build transform (order matters!)
    // ... exact order from reference
}
```

#### 2.4 Validation

- Log problematic models: Name, expected position, actual position
- Compare specific models (lamp posts, trees) with reference screenshots
- No floating, underground, or upside-down models

---

### Stage 3: Artifact Elimination (2-3 hours)

**Goal**: Fix rendering glitches.

#### 3.1 Alpha Handling

```go
// Apply RSM.Alpha per model
if rsm.Alpha < 255 {
    gl.Enable(gl.BLEND)
    gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)
    // Pass alpha to shader uniform
}
```

#### 3.2 Double-Sided Rendering

```go
// Check face.TwoSide flag during mesh build
if face.TwoSide != 0 {
    // Mark this face group for two-sided rendering
    // In render loop: gl.Disable(gl.CULL_FACE) for these
}
```

#### 3.3 Z-Fighting Prevention

```go
// Add small polygon offset for overlapping surfaces
gl.Enable(gl.POLYGON_OFFSET_FILL)
gl.PolygonOffset(1.0, 1.0)
```

#### 3.4 Texture Filtering

```go
// Ensure proper texture filtering
gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR_MIPMAP_LINEAR)
gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)
gl.GenerateMipmap(gl.TEXTURE_2D)
```

---

### Stage 4: Atmosphere (4-5 hours)

**Goal**: Add depth and visual richness.

#### 4.1 Water Rendering

Simple water plane at RSW.Water.Level:

```go
type WaterRenderer struct {
    vao, vbo uint32
    texture  uint32
    level    float32
    // Animation
    waveTime   float32
    waveHeight float32
    waveSpeed  float32
}

func (w *WaterRenderer) Render(viewProj Mat4) {
    // Render semi-transparent plane
    // Animate UV coordinates for wave effect
    // Simple reflection (optional): flip scene, render below water
}
```

Water shader:
```glsl
uniform float uTime;
uniform float uWaveHeight;

void main() {
    // Simple wave animation
    vec2 animatedUV = vTexCoord + vec2(sin(uTime * 0.5), cos(uTime * 0.3)) * 0.02;
    vec4 waterColor = texture(uTexture, animatedUV);

    // Semi-transparent with depth fade
    float alpha = 0.7;
    FragColor = vec4(waterColor.rgb, alpha);
}
```

#### 4.2 Distance Fog

Add atmospheric depth:

```glsl
// In fragment shader
uniform vec3 uFogColor;
uniform float uFogStart;
uniform float uFogEnd;

void main() {
    // Calculate fog factor
    float dist = length(vViewPos);
    float fogFactor = clamp((uFogEnd - dist) / (uFogEnd - uFogStart), 0.0, 1.0);

    // Blend with fog
    vec3 finalColor = mix(uFogColor, litColor, fogFactor);
    FragColor = vec4(finalColor, 1.0);
}
```

Fog parameters from map settings or defaults:
```go
mv.fogColor = [3]float32{0.8, 0.85, 0.9}  // Light blue-gray
mv.fogStart = 200.0  // Start fading
mv.fogEnd = 500.0    // Fully fogged
```

#### 4.3 Sky Background

Simple gradient or skybox:

```go
func (mv *MapViewer) RenderSky() {
    // Option 1: Clear with gradient (top to horizon)
    // Option 2: Simple skybox texture
    // Match fog color at horizon for seamless blend
}
```

---

### Stage 5: Validation Framework (2-3 hours)

**Goal**: Prevent regressions, ensure accuracy.

#### 5.1 Screenshot Comparison Tool

```go
// cmd/grfbrowser/validation.go
type ValidationResult struct {
    MapName    string
    Screenshot string  // Path to our render
    Reference  string  // Path to game screenshot
    Score      float64 // Similarity metric
    Issues     []string
}

func CompareWithReference(mapName string) ValidationResult {
    // Render map at fixed camera position
    // Load reference screenshot
    // Calculate visual similarity
    // Flag significant differences
}
```

#### 5.2 Test Maps

Curated set of maps covering edge cases:

| Map | Tests |
|-----|-------|
| prontera | Large map, many models, complex lighting |
| alberta | Known problematic models (trees, decorations) |
| prt_fild01 | Open field, atmospheric depth |
| prt_castle | Indoor lighting, shadows |
| iz_dun00 | Dark map, point light baking verification |

#### 5.3 Visual Diff in CI (Future)

```yaml
# .github/workflows/visual-test.yml
- name: Render test maps
  run: ./grfbrowser --render-test prontera,alberta
- name: Compare with baselines
  run: ./grfbrowser --visual-diff baseline/ output/
```

---

## File Changes

```
cmd/grfbrowser/
├── map_viewer.go       # Lighting, transforms, water, fog
├── model_debug.go      # NEW: Debug visualization for positioning
├── water_renderer.go   # NEW: Water plane rendering
├── validation.go       # NEW: Screenshot comparison tools
└── shaders/            # NEW: External shader files (easier iteration)
    ├── terrain.vert
    ├── terrain.frag
    ├── model.vert
    ├── model.frag
    ├── water.vert
    └── water.frag

docs/
├── reference-screenshots/  # NEW: Game screenshots for comparison
│   ├── alberta.png
│   ├── prontera.png
│   └── ...
└── investigations/
    └── rsm-model-positioning.md  # Update with findings
```

---

## Success Criteria

### Visual Quality
- [ ] Maps look warm and atmospheric, not flat and harsh
- [ ] Lighting direction matches game (sun position correct)
- [ ] Colors match reference screenshots (no washed out appearance)
- [ ] Fog provides depth perception
- [ ] Water planes render with animation

### Model Accuracy
- [ ] No floating objects
- [ ] No upside-down models
- [ ] No underground objects
- [ ] Correct orientation for all model types

### Artifact-Free
- [ ] No z-fighting flickering
- [ ] Transparent objects render correctly
- [ ] No backface culling issues on two-sided geometry

### Process Quality
- [ ] Every change justified by reference implementation
- [ ] Visual comparison before/after each change
- [ ] No "guessing" - systematic debugging with visualization tools

---

## Consequences

### Positive
- Systematic approach eliminates guessing
- Visual quality matches player expectations
- Foundation for future game client rendering
- Debugging tools speed up future development

### Negative
- Significant refactoring of existing shaders
- Need to collect reference screenshots
- More complex rendering pipeline

### Risks
- Reference implementations may differ from each other
- Some visual effects may require more research
- Performance impact from additional effects

### Mitigations
- Start with RagnarokRebuild as primary reference (faithful to original)
- Profile performance after each stage
- Make fog/water optional via settings

---

## References

- [Ragnarok Research Lab - Rendering Overview](https://ragnarokresearchlab.github.io/rendering/)
- [Ragnarok Research Lab - RSW Format](https://ragnarokresearchlab.github.io/file-formats/rsw/)
- [RagnarokFileFormats - RSM.MD](https://github.com/Duckwhale/RagnarokFileFormats/blob/master/RSM.MD)
- [Korangar - Rust RO Client](https://github.com/vE5li/korangar)
- [RagnarokRebuild - Unity Client](https://github.com/Doddler/RagnarokRebuild)
- [LearnOpenGL - Basic Lighting](https://learnopengl.com/Lighting/Basic-Lighting)
