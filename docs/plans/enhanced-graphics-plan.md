# Enhanced Graphics Implementation Plan

Branch: `feature/enhanced-graphics`

## Goal
Close the visual quality gap with Korangar by implementing:
1. Real-time shadow mapping
2. MSAA anti-aliasing
3. Point lights from RSW
4. FXAA post-processing

---

## Phase 1: Shadow Mapping (Highest Impact)

### 1.1 Shadow Map Framebuffer
**File:** `internal/engine/shadow/shadow_map.go`

```go
type ShadowMap struct {
    FBO           uint32
    DepthTexture  uint32
    Resolution    int32  // 2048 default
    LightViewProj math.Mat4
}

func NewShadowMap(resolution int32) *ShadowMap
func (s *ShadowMap) Bind()
func (s *ShadowMap) Unbind()
func (s *ShadowMap) BindTexture(unit uint32)
func (s *ShadowMap) Destroy()
```

**OpenGL Setup:**
```go
gl.GenFramebuffers(1, &s.FBO)
gl.GenTextures(1, &s.DepthTexture)
gl.BindTexture(gl.TEXTURE_2D, s.DepthTexture)
gl.TexImage2D(gl.TEXTURE_2D, 0, gl.DEPTH_COMPONENT24, resolution, resolution, 0,
              gl.DEPTH_COMPONENT, gl.FLOAT, nil)
gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_COMPARE_MODE, gl.COMPARE_REF_TO_TEXTURE)
gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_COMPARE_FUNC, gl.LEQUAL)
gl.FramebufferTexture2D(gl.FRAMEBUFFER, gl.DEPTH_ATTACHMENT, gl.TEXTURE_2D, s.DepthTexture, 0)
```

### 1.2 Shadow Pass Shader
**File:** `cmd/grfbrowser/shaders/shadow.vert`

```glsl
#version 410 core
layout (location = 0) in vec3 aPosition;

uniform mat4 uLightViewProj;
uniform mat4 uModel;

void main() {
    gl_Position = uLightViewProj * uModel * vec4(aPosition, 1.0);
}
```

**File:** `cmd/grfbrowser/shaders/shadow.frag`
```glsl
#version 410 core
void main() {
    // Depth is written automatically
}
```

### 1.3 Light Matrix Calculation
**File:** `internal/engine/shadow/light_matrix.go`

```go
// CalculateDirectionalLightMatrix computes view-projection for shadow map
func CalculateDirectionalLightMatrix(lightDir [3]float32, sceneBounds AABB) math.Mat4 {
    // 1. Create orthographic projection covering scene
    // 2. Look from light direction towards scene center
    center := sceneBounds.Center()
    radius := sceneBounds.Radius()

    lightPos := math.Vec3{
        X: center.X - lightDir[0] * radius * 2,
        Y: center.Y - lightDir[1] * radius * 2,
        Z: center.Z - lightDir[2] * radius * 2,
    }

    view := math.LookAt(lightPos, center, math.Vec3{X: 0, Y: 1, Z: 0})
    proj := math.Ortho(-radius, radius, -radius, radius, 0.1, radius*4)

    return proj.Mul(view)
}
```

### 1.4 Update Terrain Shader
**File:** `cmd/grfbrowser/shaders/terrain.frag` (modify)

```glsl
// Add uniforms
uniform sampler2DShadow uShadowMap;
uniform mat4 uLightViewProj;
uniform bool uShadowsEnabled;

// Add to fragment function
float calculateShadow(vec3 worldPos) {
    vec4 lightSpacePos = uLightViewProj * vec4(worldPos, 1.0);
    vec3 projCoords = lightSpacePos.xyz / lightSpacePos.w;
    projCoords = projCoords * 0.5 + 0.5;

    if (projCoords.z > 1.0) return 1.0;

    // PCF 3x3
    float shadow = 0.0;
    vec2 texelSize = 1.0 / textureSize(uShadowMap, 0);
    for (int x = -1; x <= 1; x++) {
        for (int y = -1; y <= 1; y++) {
            shadow += texture(uShadowMap, vec3(projCoords.xy + vec2(x, y) * texelSize, projCoords.z));
        }
    }
    return shadow / 9.0;
}
```

### 1.5 Render Pipeline Changes
**File:** `cmd/grfbrowser/map_viewer.go` (modify)

```go
// Add to MapViewer struct
shadowMap     *shadow.ShadowMap
shadowShader  uint32
shadowEnabled bool

// New render flow:
func (mv *MapViewer) Render() {
    // Pass 1: Shadow map
    if mv.shadowEnabled {
        mv.shadowMap.Bind()
        gl.Clear(gl.DEPTH_BUFFER_BIT)
        mv.renderSceneDepthOnly(mv.lightViewProj)
        mv.shadowMap.Unbind()
    }

    // Pass 2: Main render with shadows
    mv.bindMainFramebuffer()
    mv.shadowMap.BindTexture(gl.TEXTURE4)
    mv.renderScene()
}
```

### Tasks for Phase 1
- [ ] Create `internal/engine/shadow/` package
- [ ] Implement ShadowMap struct with FBO
- [ ] Create shadow pass shaders
- [ ] Add light matrix calculation
- [ ] Modify terrain shader for shadow sampling
- [ ] Modify model shader for shadow sampling
- [ ] Add shadow pass to render pipeline
- [ ] Add UI toggle for shadows

---

## Phase 2: MSAA Anti-Aliasing

### 2.1 MSAA Framebuffer
**File:** `internal/engine/graphics/msaa_target.go`

```go
type MSAATarget struct {
    FBO            uint32
    ColorTexture   uint32  // Multisampled
    DepthRBO       uint32  // Multisampled
    ResolveFBO     uint32
    ResolveTexture uint32  // Regular texture for resolve
    Width, Height  int32
    Samples        int32   // 2, 4, 8
}

func NewMSAATarget(width, height, samples int32) *MSAATarget
func (m *MSAATarget) Bind()
func (m *MSAATarget) Resolve()  // Blit to resolve texture
func (m *MSAATarget) GetTexture() uint32
```

**OpenGL Setup:**
```go
// Multisampled color texture
gl.GenTextures(1, &m.ColorTexture)
gl.BindTexture(gl.TEXTURE_2D_MULTISAMPLE, m.ColorTexture)
gl.TexImage2DMultisample(gl.TEXTURE_2D_MULTISAMPLE, samples, gl.RGBA8, width, height, true)

// Multisampled depth renderbuffer
gl.GenRenderbuffers(1, &m.DepthRBO)
gl.BindRenderbuffer(gl.RENDERBUFFER, m.DepthRBO)
gl.RenderbufferStorageMultisample(gl.RENDERBUFFER, samples, gl.DEPTH_COMPONENT24, width, height)

// Resolve blit
gl.BindFramebuffer(gl.READ_FRAMEBUFFER, m.FBO)
gl.BindFramebuffer(gl.DRAW_FRAMEBUFFER, m.ResolveFBO)
gl.BlitFramebuffer(0, 0, width, height, 0, 0, width, height, gl.COLOR_BUFFER_BIT, gl.NEAREST)
```

### Tasks for Phase 2
- [ ] Create MSAATarget struct
- [ ] Add MSAA framebuffer creation
- [ ] Implement resolve (blit) step
- [ ] Integrate into render pipeline
- [ ] Add sample count setting (2x, 4x, 8x)
- [ ] Add UI dropdown for MSAA level

---

## Phase 3: Point Lights

### 3.1 Point Light Structure
**File:** `internal/engine/lighting/point_light.go`

```go
type PointLight struct {
    Position  [3]float32
    Color     [3]float32
    Range     float32
    Intensity float32
}

const MaxPointLights = 32

type PointLightBuffer struct {
    Lights [MaxPointLights]PointLight
    Count  int32
}
```

### 3.2 Extract Lights from RSW
**File:** `internal/engine/lighting/rsw_lights.go`

```go
func ExtractPointLights(rsw *formats.RSW) []PointLight {
    var lights []PointLight
    for _, obj := range rsw.Objects {
        if obj.Type == formats.RSWObjectLight {
            lights = append(lights, PointLight{
                Position:  obj.Position,
                Color:     obj.LightColor,
                Range:     obj.LightRange,
                Intensity: 1.0,
            })
        }
    }
    return lights
}
```

### 3.3 Update Shaders
**File:** `cmd/grfbrowser/shaders/terrain.frag` (modify)

```glsl
struct PointLight {
    vec3 position;
    vec3 color;
    float range;
    float intensity;
};

uniform PointLight uPointLights[32];
uniform int uPointLightCount;

vec3 calculatePointLights(vec3 worldPos, vec3 normal) {
    vec3 result = vec3(0.0);
    for (int i = 0; i < uPointLightCount; i++) {
        vec3 lightDir = uPointLights[i].position - worldPos;
        float distance = length(lightDir);
        lightDir = normalize(lightDir);

        float attenuation = 1.0 / (1.0 + distance * distance /
                           (uPointLights[i].range * uPointLights[i].range));
        float diffuse = max(dot(normal, lightDir), 0.0);

        result += uPointLights[i].color * uPointLights[i].intensity * diffuse * attenuation;
    }
    return result;
}
```

### Tasks for Phase 3
- [ ] Create PointLight struct
- [ ] Extract lights from RSW objects
- [ ] Add point light uniforms to shaders
- [ ] Implement attenuation calculation
- [ ] Upload light data to GPU
- [ ] Add to terrain and model shaders

---

## Phase 4: FXAA Post-Processing

### 4.1 Post-Process Pipeline
**File:** `internal/engine/postprocess/fxaa.go`

```go
type FXAAPass struct {
    Shader     uint32
    FullscreenVAO uint32
    FullscreenVBO uint32
}

func NewFXAAPass() *FXAAPass
func (f *FXAAPass) Apply(inputTexture uint32, outputFBO uint32)
```

### 4.2 FXAA Shader
**File:** `cmd/grfbrowser/shaders/fxaa.frag`

```glsl
#version 410 core

uniform sampler2D uInputTexture;
uniform vec2 uTexelSize;

in vec2 vTexCoord;
out vec4 FragColor;

// FXAA 3.11 implementation
void main() {
    // Luma at current fragment
    vec3 rgbM = texture(uInputTexture, vTexCoord).rgb;
    float lumaM = dot(rgbM, vec3(0.299, 0.587, 0.114));

    // Luma at neighbors
    float lumaNW = dot(textureOffset(uInputTexture, vTexCoord, ivec2(-1, -1)).rgb, vec3(0.299, 0.587, 0.114));
    float lumaNE = dot(textureOffset(uInputTexture, vTexCoord, ivec2( 1, -1)).rgb, vec3(0.299, 0.587, 0.114));
    float lumaSW = dot(textureOffset(uInputTexture, vTexCoord, ivec2(-1,  1)).rgb, vec3(0.299, 0.587, 0.114));
    float lumaSE = dot(textureOffset(uInputTexture, vTexCoord, ivec2( 1,  1)).rgb, vec3(0.299, 0.587, 0.114));

    // ... FXAA algorithm
    FragColor = vec4(rgbM, 1.0); // Simplified
}
```

### Tasks for Phase 4
- [ ] Create fullscreen quad for post-processing
- [ ] Implement FXAA shader
- [ ] Add post-process pass to pipeline
- [ ] Add UI toggle for FXAA

---

## Implementation Order

```
Week 1: Shadow Mapping
├── Day 1-2: Shadow map FBO and shaders
├── Day 3-4: Light matrix calculation
└── Day 5: Integrate into render pipeline

Week 2: MSAA + Polish
├── Day 1-2: MSAA framebuffer
├── Day 3: Resolve and integration
└── Day 4-5: Testing and bug fixes

Week 3: Point Lights + FXAA
├── Day 1-2: Point light extraction and shaders
├── Day 3: FXAA post-process
└── Day 4-5: UI controls and final polish
```

---

## Files to Create

```
internal/engine/shadow/
├── shadow_map.go       # Shadow map FBO management
└── light_matrix.go     # Directional light matrix calculation

internal/engine/graphics/
└── msaa_target.go      # MSAA framebuffer

internal/engine/lighting/
├── point_light.go      # Point light struct
└── rsw_lights.go       # Extract lights from RSW

internal/engine/postprocess/
└── fxaa.go             # FXAA pass

cmd/grfbrowser/shaders/
├── shadow.vert         # Shadow pass vertex
├── shadow.frag         # Shadow pass fragment
└── fxaa.frag           # FXAA post-process
```

---

## Files to Modify

```
cmd/grfbrowser/shaders/
├── terrain.vert        # Add shadow coord output
├── terrain.frag        # Add shadow sampling, point lights
├── model.vert          # Add shadow coord output
└── model.frag          # Add shadow sampling, point lights

cmd/grfbrowser/
├── map_viewer.go       # Multi-pass rendering
└── preview_map.go      # UI controls for new features
```

---

## Testing Checklist

### Shadow Mapping
- [ ] Shadows render on terrain
- [ ] Shadows render on models
- [ ] Shadow map resolution change works
- [ ] PCF softness looks correct
- [ ] No shadow acne artifacts
- [ ] Peter-panning minimized

### MSAA
- [ ] MSAA 2x/4x/8x all work
- [ ] No performance regression at 4x
- [ ] Edges are noticeably smoother
- [ ] Resolve step works correctly

### Point Lights
- [ ] Lights extracted from RSW
- [ ] Attenuation looks natural
- [ ] Multiple lights accumulate correctly
- [ ] Performance acceptable with 32 lights

### FXAA
- [ ] Reduces remaining aliasing
- [ ] No excessive blur
- [ ] Toggle works correctly

---

## Performance Targets

| Feature | Target FPS Impact |
|---------|-------------------|
| Shadow Map (2048) | -5 to -10 FPS |
| MSAA 4x | -10 to -15 FPS |
| Point Lights (32) | -2 to -5 FPS |
| FXAA | -1 to -2 FPS |

Baseline: 60 FPS on integrated GPU
Target: 40+ FPS with all features on integrated GPU
