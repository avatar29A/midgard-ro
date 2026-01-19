// Package scene provides a reusable 3D scene rendering system for Ragnarok Online maps.
// It handles terrain, models, water, sprites, and lighting.
package scene

import (
	"fmt"

	"github.com/go-gl/gl/v4.1-core/gl"

	"github.com/Faultbox/midgard-ro/internal/engine/camera"
	"github.com/Faultbox/midgard-ro/internal/engine/framebuffer"
	"github.com/Faultbox/midgard-ro/internal/engine/lighting"
	"github.com/Faultbox/midgard-ro/internal/engine/scene/shaders"
	"github.com/Faultbox/midgard-ro/internal/engine/shader"
	"github.com/Faultbox/midgard-ro/internal/engine/shadow"
	"github.com/Faultbox/midgard-ro/internal/engine/terrain"
	"github.com/Faultbox/midgard-ro/pkg/formats"
	"github.com/Faultbox/midgard-ro/pkg/math"
)

// MaxPointLights is the maximum number of point lights supported.
const MaxPointLights = 32

// PointLight represents a point light source in the scene.
type PointLight struct {
	Position  [3]float32
	Color     [3]float32
	Range     float32
	Intensity float32
}

// Config contains scene configuration options.
type Config struct {
	Width            int32
	Height           int32
	ShadowResolution int32
	ShadowsEnabled   bool
	PointLightsEnabled bool
	FogEnabled       bool
}

// DefaultConfig returns a default scene configuration.
func DefaultConfig() Config {
	return Config{
		Width:              1280,
		Height:             720,
		ShadowResolution:   shadow.DefaultResolution,
		ShadowsEnabled:     true,
		PointLightsEnabled: true,
		FogEnabled:         false,
	}
}

// Scene manages a complete 3D scene with terrain, models, water, and lighting.
type Scene struct {
	// Configuration
	config Config

	// Framebuffer for offscreen rendering
	framebuffer *framebuffer.Framebuffer

	// Renderers
	terrainRenderer *TerrainRenderer
	modelRenderer   *ModelRenderer
	waterRenderer   *WaterRenderer
	spriteRenderer  *SpriteRenderer

	// Shadow mapping
	shadowMap     *shadow.Map
	shadowProgram uint32
	locShadowLightViewProj int32
	locShadowModel         int32

	// Lighting
	LightDir     [3]float32
	AmbientColor [3]float32
	DiffuseColor [3]float32
	LightOpacity float32
	Brightness   float32

	// Point lights
	PointLights         []PointLight
	PointLightsEnabled  bool
	PointLightIntensity float32

	// Fog settings
	FogEnabled bool
	FogNear    float32
	FogFar     float32
	FogColor   [3]float32

	// Shadows
	ShadowsEnabled   bool
	lightViewProj    math.Mat4

	// Map bounds
	MinBounds [3]float32
	MaxBounds [3]float32

	// Map dimensions
	MapWidth  float32
	MapHeight float32

	// Terrain height data
	terrainAltitudes [][]float32
	terrainTileZoom  float32
	terrainTilesX    int
	terrainTilesZ    int

	// GAT collision data
	GAT *formats.GAT

	// Fallback texture
	fallbackTex uint32
}

// New creates a new scene with the given configuration.
func New(cfg Config) (*Scene, error) {
	s := &Scene{
		config: cfg,
		// Default lighting
		LightDir:     [3]float32{0.5, 0.866, 0.0},
		AmbientColor: [3]float32{0.3, 0.3, 0.3},
		DiffuseColor: [3]float32{1.0, 1.0, 1.0},
		LightOpacity: 1.0,
		Brightness:   1.0,
		// Shadow/light settings
		ShadowsEnabled:     cfg.ShadowsEnabled,
		PointLightsEnabled: cfg.PointLightsEnabled,
		PointLightIntensity: 1.0,
		FogEnabled:         cfg.FogEnabled,
	}

	// Create framebuffer
	var err error
	s.framebuffer, err = framebuffer.New(cfg.Width, cfg.Height)
	if err != nil {
		return nil, fmt.Errorf("creating framebuffer: %w", err)
	}

	// Create shadow map
	s.shadowMap = shadow.NewMap(cfg.ShadowResolution)
	if s.shadowMap == nil {
		s.ShadowsEnabled = false
	}

	// Create shadow shader
	if err := s.createShadowShader(); err != nil {
		s.Destroy()
		return nil, fmt.Errorf("creating shadow shader: %w", err)
	}

	// Create renderers
	s.terrainRenderer, err = NewTerrainRenderer()
	if err != nil {
		s.Destroy()
		return nil, fmt.Errorf("creating terrain renderer: %w", err)
	}

	s.modelRenderer, err = NewModelRenderer()
	if err != nil {
		s.Destroy()
		return nil, fmt.Errorf("creating model renderer: %w", err)
	}

	s.waterRenderer, err = NewWaterRenderer()
	if err != nil {
		s.Destroy()
		return nil, fmt.Errorf("creating water renderer: %w", err)
	}

	s.spriteRenderer, err = NewSpriteRenderer()
	if err != nil {
		s.Destroy()
		return nil, fmt.Errorf("creating sprite renderer: %w", err)
	}

	// Create fallback texture
	s.createFallbackTexture()

	return s, nil
}

func (s *Scene) createShadowShader() error {
	program, err := shader.CompileProgram(shaders.ShadowVertexShader, shaders.ShadowFragmentShader)
	if err != nil {
		return fmt.Errorf("shadow shader: %w", err)
	}
	s.shadowProgram = program
	s.locShadowLightViewProj = shader.GetUniform(program, "uLightViewProj")
	s.locShadowModel = shader.GetUniform(program, "uModel")
	return nil
}

func (s *Scene) createFallbackTexture() {
	gl.GenTextures(1, &s.fallbackTex)
	gl.BindTexture(gl.TEXTURE_2D, s.fallbackTex)
	white := []uint8{255, 255, 255, 255}
	gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA, 1, 1, 0, gl.RGBA, gl.UNSIGNED_BYTE, gl.Ptr(white))
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.NEAREST)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.NEAREST)
}

// LoadMap loads terrain data from GND and RSW.
func (s *Scene) LoadMap(gnd *formats.GND, rsw *formats.RSW, texLoader func(string) ([]byte, error)) error {
	// Store map dimensions
	s.MapWidth = float32(gnd.Width) * gnd.Zoom
	s.MapHeight = float32(gnd.Height) * gnd.Zoom

	// Build heightmap for terrain height queries
	hm := terrain.BuildHeightmap(gnd)
	s.terrainAltitudes = hm.Altitudes
	s.terrainTilesX = hm.TilesX
	s.terrainTilesZ = hm.TilesZ
	s.terrainTileZoom = hm.TileZoom

	// Load GAT for collision
	if rsw != nil && rsw.GndFile != "" {
		gatPath := "data/" + rsw.GndFile
		if len(gatPath) > 4 {
			gatPath = gatPath[:len(gatPath)-4] + ".gat"
		}
		gatData, err := texLoader(gatPath)
		if err == nil {
			s.GAT, _ = formats.ParseGAT(gatData)
		}
	}

	// Extract lighting from RSW
	if rsw != nil {
		s.LightDir = lighting.SunDirection(rsw.Light.Longitude, rsw.Light.Latitude)
		s.AmbientColor = rsw.Light.Ambient
		s.DiffuseColor = rsw.Light.Diffuse
		s.LightOpacity = rsw.Light.Opacity
		if s.LightOpacity <= 0 {
			s.LightOpacity = 1.0
		}

		// Ensure minimum ambient
		minAmbient := float32(0.3)
		for i := 0; i < 3; i++ {
			if s.AmbientColor[i] < minAmbient {
				s.AmbientColor[i] = minAmbient
			}
		}

		// Extract point lights
		s.extractPointLights(rsw)
	}

	// Load terrain
	if err := s.terrainRenderer.LoadTerrain(gnd, texLoader, s.fallbackTex); err != nil {
		return fmt.Errorf("loading terrain: %w", err)
	}

	// Get bounds from terrain
	s.MinBounds = s.terrainRenderer.MinBounds
	s.MaxBounds = s.terrainRenderer.MaxBounds

	// Load models
	if rsw != nil {
		if err := s.modelRenderer.LoadModels(rsw, texLoader, s.fallbackTex, s.MapWidth, s.MapHeight, s.terrainAltitudes, s.terrainTileZoom, s.terrainTilesX, s.terrainTilesZ); err != nil {
			return fmt.Errorf("loading models: %w", err)
		}
	}

	// Load water
	if rsw != nil && rsw.Water.Level > 0 {
		s.waterRenderer.SetupWater(rsw.Water.Level, s.MinBounds, s.MaxBounds, texLoader)
	}

	return nil
}

func (s *Scene) extractPointLights(rsw *formats.RSW) {
	s.PointLights = nil
	lights := rsw.GetLights()
	for _, light := range lights {
		pl := PointLight{
			Position:  light.Position,
			Color:     light.Color,
			Range:     light.Range,
			Intensity: 1.0,
		}
		// Convert RSW coordinates to world coordinates
		pl.Position[0] = pl.Position[0] + s.MapWidth/2
		pl.Position[2] = pl.Position[2] + s.MapHeight/2
		s.PointLights = append(s.PointLights, pl)
	}
}

// Render renders the scene to the framebuffer using an OrbitCamera.
func (s *Scene) Render(cam *camera.OrbitCamera) uint32 {
	return s.RenderWithView(cam.ViewMatrix())
}

// RenderWithThirdPerson renders the scene using a ThirdPersonCamera following a target.
func (s *Scene) RenderWithThirdPerson(cam *camera.ThirdPersonCamera, targetX, targetY, targetZ float32) uint32 {
	return s.RenderWithView(cam.ViewMatrix(targetX, targetY, targetZ))
}

// RenderWithView renders the scene with a pre-computed view matrix.
func (s *Scene) RenderWithView(view math.Mat4) uint32 {
	// Calculate view/projection matrices
	aspect := float32(s.config.Width) / float32(s.config.Height)
	proj := math.Perspective(0.785398, aspect, 1.0, 10000.0) // 45 degrees FOV
	viewProj := proj.Mul(view)

	// Calculate light view projection for shadows
	if s.ShadowsEnabled && s.shadowMap != nil {
		sceneBounds := shadow.AABB{
			Min: s.MinBounds,
			Max: s.MaxBounds,
		}
		s.lightViewProj = shadow.CalculateDirectionalLightMatrix(s.LightDir, sceneBounds)
	}

	// Render shadow pass
	if s.ShadowsEnabled && s.shadowMap != nil {
		s.renderShadowPass()
	}

	// Bind main framebuffer
	restore := s.framebuffer.BindWithViewport()
	defer restore()

	// Clear
	s.framebuffer.Clear(0.15, 0.15, 0.2, 1.0)

	// Enable depth testing
	gl.Enable(gl.DEPTH_TEST)
	gl.DepthFunc(gl.LESS)

	// Enable alpha blending
	gl.Enable(gl.BLEND)
	gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)

	// Render terrain
	s.terrainRenderer.Render(viewProj, s.LightDir, s.AmbientColor, s.DiffuseColor, s.Brightness, s.LightOpacity,
		s.ShadowsEnabled, s.lightViewProj, s.shadowMap,
		s.PointLightsEnabled, s.PointLights, s.PointLightIntensity,
		s.FogEnabled, s.FogNear, s.FogFar, s.FogColor)

	// Render models
	s.modelRenderer.Render(viewProj, s.LightDir, s.AmbientColor, s.DiffuseColor,
		s.ShadowsEnabled, s.lightViewProj, s.shadowMap,
		s.PointLightsEnabled, s.PointLights, s.PointLightIntensity,
		s.FogEnabled, s.FogNear, s.FogFar, s.FogColor)

	// Render water
	if s.waterRenderer.HasWater() {
		s.waterRenderer.Render(viewProj)
	}

	return s.framebuffer.ColorTexture()
}

func (s *Scene) renderShadowPass() {
	if s.shadowMap == nil {
		return
	}

	s.shadowMap.Bind()
	gl.Clear(gl.DEPTH_BUFFER_BIT)

	gl.UseProgram(s.shadowProgram)
	gl.UniformMatrix4fv(s.locShadowLightViewProj, 1, false, &s.lightViewProj[0])

	// Render terrain to shadow map
	identity := math.Identity()
	gl.UniformMatrix4fv(s.locShadowModel, 1, false, &identity[0])
	s.terrainRenderer.RenderShadow()

	// Render models to shadow map
	s.modelRenderer.RenderShadow(s.shadowProgram, s.locShadowModel)

	s.shadowMap.Unbind()
}

// RenderSprite renders a sprite at the given world position.
func (s *Scene) RenderSprite(viewProj math.Mat4, camRight, camUp math.Vec3, worldPos [3]float32, width, height float32, textureID uint32, tint [4]float32) {
	s.spriteRenderer.Render(viewProj, camRight, camUp, worldPos, width, height, textureID, tint)
}

// Resize updates the scene dimensions.
func (s *Scene) Resize(width, height int32) {
	if width == s.config.Width && height == s.config.Height {
		return
	}
	s.config.Width = width
	s.config.Height = height
	s.framebuffer.Resize(width, height)
}

// GetTerrainHeight returns the terrain height at the given world coordinates.
func (s *Scene) GetTerrainHeight(worldX, worldZ float32) float32 {
	if s.terrainAltitudes == nil {
		return 0
	}

	// Convert world coords to tile coords
	tileX := int(worldX / s.terrainTileZoom)
	tileZ := int(worldZ / s.terrainTileZoom)

	if tileX < 0 || tileX >= s.terrainTilesX || tileZ < 0 || tileZ >= s.terrainTilesZ {
		return 0
	}

	return s.terrainAltitudes[tileX][tileZ]
}

// IsWalkable returns whether the given tile coordinates are walkable.
func (s *Scene) IsWalkable(tileX, tileY int) bool {
	if s.GAT == nil {
		return true
	}
	return s.GAT.IsWalkable(tileX, tileY)
}

// FallbackTexture returns the fallback texture ID.
func (s *Scene) FallbackTexture() uint32 {
	return s.fallbackTex
}

// ColorTexture returns the rendered color texture.
func (s *Scene) ColorTexture() uint32 {
	return s.framebuffer.ColorTexture()
}

// CaptureImage captures the current rendered scene as RGBA pixel data.
// Returns the pixel data and dimensions. Pixels are in correct orientation (top-to-bottom).
func (s *Scene) CaptureImage() ([]byte, int32, int32) {
	width, height := s.framebuffer.Size()
	pixels := s.framebuffer.ReadPixels()

	// Flip vertically (OpenGL has origin at bottom-left, we need top-left)
	rowSize := int(width) * 4
	flipped := make([]byte, len(pixels))
	for y := 0; y < int(height); y++ {
		srcRow := (int(height) - 1 - y) * rowSize
		dstRow := y * rowSize
		copy(flipped[dstRow:dstRow+rowSize], pixels[srcRow:srcRow+rowSize])
	}

	return flipped, width, height
}

// Destroy releases all resources.
func (s *Scene) Destroy() {
	if s.terrainRenderer != nil {
		s.terrainRenderer.Destroy()
	}
	if s.modelRenderer != nil {
		s.modelRenderer.Destroy()
	}
	if s.waterRenderer != nil {
		s.waterRenderer.Destroy()
	}
	if s.spriteRenderer != nil {
		s.spriteRenderer.Destroy()
	}
	if s.shadowMap != nil {
		s.shadowMap.Destroy()
	}
	if s.shadowProgram != 0 {
		gl.DeleteProgram(s.shadowProgram)
	}
	if s.framebuffer != nil {
		s.framebuffer.Destroy()
	}
	if s.fallbackTex != 0 {
		gl.DeleteTextures(1, &s.fallbackTex)
	}
}
