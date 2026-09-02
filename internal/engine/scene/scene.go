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
	Width              int32
	Height             int32
	ShadowResolution   int32
	ShadowsEnabled     bool
	PointLightsEnabled bool
	FogEnabled         bool
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
	shadowMap              *shadow.Map
	shadowProgram          uint32
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
	ShadowsEnabled bool
	lightViewProj  math.Mat4

	// Last computed view-projection matrix (set by RenderWithView).
	// Exposed for picking — see LastViewProj().
	lastViewProj math.Mat4

	// Map bounds
	MinBounds [3]float32
	MaxBounds [3]float32

	// Map dimensions
	MapWidth  float32
	MapHeight float32

	// Terrain height data
	terrainHeightmap *terrain.Heightmap
	terrainTileZoom  float32

	// HideModels suppresses map objects, leaving only the terrain. A
	// diagnostic aid for telling terrain artifacts from model ones.
	HideModels    bool
	terrainTilesX int
	terrainTilesZ int

	// GAT collision data
	GAT *formats.GAT

	// clearColor is what shows where the map has nothing: sky outdoors,
	// black indoors, as the original does. See SetClearColor.
	clearColor [3]float32

	// gnd is the map's ground, kept between BeginMap and EndMap for the water.
	gnd *formats.GND

	// Fallback texture
	fallbackTex uint32
}

// New creates a new scene with the given configuration.
func New(cfg Config) (*Scene, error) {
	s := &Scene{
		config:     cfg,
		clearColor: SkyClearColor,
		// Default lighting
		LightDir:     [3]float32{0.5, 0.866, 0.0},
		AmbientColor: [3]float32{0.3, 0.3, 0.3},
		DiffuseColor: [3]float32{1.0, 1.0, 1.0},
		LightOpacity: 1.0,
		Brightness:   1.0,
		// Shadow/light settings
		ShadowsEnabled:      cfg.ShadowsEnabled,
		PointLightsEnabled:  cfg.PointLightsEnabled,
		PointLightIntensity: 1.0,
		FogEnabled:          cfg.FogEnabled,
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

// A map is loaded in phases so a loading screen can draw between them:
// BeginMap, LoadTerrain, BeginModels, LoadModelRange (as many times as it
// takes), EndMap. LoadMap runs them back to back for a caller that does not
// need frames in between. Every phase runs on the GL thread.

// BeginMap takes the map's dimensions, height data, walkability and lighting
// from the parsed files. It does no GPU work.
func (s *Scene) BeginMap(gnd *formats.GND, rsw *formats.RSW, texLoader func(string) ([]byte, error)) {
	s.gnd = gnd
	s.MapWidth = float32(gnd.Width) * gnd.Zoom
	s.MapHeight = float32(gnd.Height) * gnd.Zoom

	// Build heightmap for terrain height queries
	hm := terrain.BuildHeightmap(gnd)
	s.terrainHeightmap = hm
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
}

// LoadTerrain uploads the ground: its textures, lightmap atlas and mesh.
func (s *Scene) LoadTerrain(gnd *formats.GND, texLoader func(string) ([]byte, error)) error {
	if err := s.terrainRenderer.LoadTerrain(gnd, texLoader, s.fallbackTex); err != nil {
		return fmt.Errorf("loading terrain: %w", err)
	}

	// Get bounds from terrain
	s.MinBounds = s.terrainRenderer.MinBounds
	s.MaxBounds = s.terrainRenderer.MaxBounds
	return nil
}

// TerrainGroups reports how many texture groups the ground mesh has.
func (s *Scene) TerrainGroups() int {
	if s.terrainRenderer == nil {
		return 0
	}
	return len(s.terrainRenderer.groups)
}

// ModelCount is how many of a map's model instances LoadModelRange will
// accept: the RSW's count, capped.
func (s *Scene) ModelCount(rsw *formats.RSW) int {
	if rsw == nil {
		return 0
	}
	return s.modelRenderer.ModelCount(rsw)
}

// BeginModels drops any previous models and prepares for LoadModelRange.
func (s *Scene) BeginModels(rsw *formats.RSW) {
	if rsw == nil {
		return
	}
	s.modelRenderer.BeginModels(s.fallbackTex, s.MapWidth, s.MapHeight)
}

// LoadModelRange builds and uploads the model instances with indices in
// [from, to).
func (s *Scene) LoadModelRange(rsw *formats.RSW, texLoader func(string) ([]byte, error), from, to int) {
	if rsw == nil {
		return
	}
	s.modelRenderer.LoadModelRange(rsw, texLoader, from, to)
}

// EndMap finishes the models — depth biases against overlap, world bounds for
// culling — and sets up the water.
func (s *Scene) EndMap(rsw *formats.RSW, texLoader func(string) ([]byte, error)) {
	if rsw == nil {
		return
	}
	s.modelRenderer.EndModels()

	if rsw.Water.Level > 0 {
		s.waterRenderer.SetupWater(s.gnd, rsw.Water, texLoader)
	}
}

// SkyClearColor is the sky blue that shows beyond an outdoor map's edge
// (matches grfbrowser); IndoorClearColor is the black the original shows
// between an indoor map's rooms.
var (
	SkyClearColor    = [3]float32{0.4, 0.6, 0.9}
	IndoorClearColor = [3]float32{0, 0, 0}
)

// SetClearColor sets what is drawn where the map has nothing.
func (s *Scene) SetClearColor(c [3]float32) {
	s.clearColor = c
}

// WaterCells reports how many cells of the map carry water.
func (s *Scene) WaterCells() int {
	if s.waterRenderer == nil {
		return 0
	}
	return s.waterRenderer.Cells()
}

// LoadedModels reports how many model instances the map placed.
func (s *Scene) LoadedModels() int {
	if s.modelRenderer == nil {
		return 0
	}
	return len(s.modelRenderer.models)
}

// LoadMap loads a map in one call: every phase, back to back.
func (s *Scene) LoadMap(gnd *formats.GND, rsw *formats.RSW, texLoader func(string) ([]byte, error)) error {
	s.BeginMap(gnd, rsw, texLoader)
	if err := s.LoadTerrain(gnd, texLoader); err != nil {
		return err
	}
	s.BeginModels(rsw)
	s.LoadModelRange(rsw, texLoader, 0, s.ModelCount(rsw))
	s.EndMap(rsw, texLoader)
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
	return s.RenderWithViewExtras(cam.ViewMatrix(targetX, targetY, targetZ), nil)
}

// RenderWithThirdPersonExtras is RenderWithThirdPerson plus an extras callback
// that runs inside the scene framebuffer (after world rendering, before
// unbind) — use this to draw billboards/overlays that need to appear in the
// composited scene texture.
func (s *Scene) RenderWithThirdPersonExtras(cam *camera.ThirdPersonCamera, targetX, targetY, targetZ float32, extras func(viewProj math.Mat4)) uint32 {
	return s.RenderWithViewExtras(cam.ViewMatrix(targetX, targetY, targetZ), extras)
}

// RenderWithView renders the scene with a pre-computed view matrix.
func (s *Scene) RenderWithView(view math.Mat4) uint32 {
	return s.RenderWithViewExtras(view, nil)
}

// AdvanceModels moves map model animation forward by deltaMs.
//
// Must be called on the GL thread: animated models are rebuilt and re-uploaded
// here rather than skinned on the GPU, because so few of them move that the
// simpler path costs less than the machinery to avoid it.
func (s *Scene) AdvanceModels(deltaMs float32) {
	if s.modelRenderer != nil {
		s.modelRenderer.Advance(deltaMs)
	}
}

// LastViewProj returns the most recently used view-projection matrix.
// Useful for screen-to-world ray casting (picking).
func (s *Scene) LastViewProj() math.Mat4 {
	return s.lastViewProj
}

// RenderWithViewExtras renders the scene with a pre-computed view matrix and
// an optional extras callback that runs in the scene framebuffer just before
// it is unbound, so callers can draw additional content (e.g. player sprite,
// effects) into the composited scene texture.
func (s *Scene) RenderWithViewExtras(view math.Mat4, extras func(viewProj math.Mat4)) uint32 {
	// Calculate view/projection matrices
	aspect := float32(s.config.Width) / float32(s.config.Height)
	proj := math.Perspective(0.785398, aspect, 1.0, 10000.0) // 45 degrees FOV
	viewProj := proj.Mul(view)
	s.lastViewProj = viewProj

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

	s.framebuffer.Clear(s.clearColor[0], s.clearColor[1], s.clearColor[2], 1.0)

	// Enable depth testing
	gl.Enable(gl.DEPTH_TEST)
	gl.DepthFunc(gl.LESS)

	// Enable alpha blending
	gl.Enable(gl.BLEND)
	gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)

	// Disable face culling for terrain (winding order varies)
	gl.Disable(gl.CULL_FACE)

	// Render terrain
	s.terrainRenderer.Render(viewProj, s.LightDir, s.AmbientColor, s.DiffuseColor, s.Brightness, s.LightOpacity,
		s.ShadowsEnabled, s.lightViewProj, s.shadowMap,
		s.PointLightsEnabled, s.PointLights, s.PointLightIntensity,
		s.FogEnabled, s.FogNear, s.FogFar, s.FogColor)

	// Render models. Skipping them is a diagnostic aid: with the models gone,
	// anything still wrong on screen belongs to the terrain mesh, which
	// otherwise takes a lot of guessing to tell apart from map objects sitting
	// flush against it.
	if !s.HideModels {
		s.modelRenderer.Render(viewProj, s.LightDir, s.AmbientColor, s.DiffuseColor,
			s.ShadowsEnabled, s.lightViewProj, s.shadowMap,
			s.PointLightsEnabled, s.PointLights, s.PointLightIntensity,
			s.FogEnabled, s.FogNear, s.FogFar, s.FogColor)
	}

	// Render water
	if s.waterRenderer.HasWater() {
		s.waterRenderer.Render(viewProj)
	}

	// Run extras (e.g. player billboard) inside the framebuffer.
	if extras != nil {
		extras(viewProj)
	}

	// Force a GL flush before returning so that any writes made by world
	// renderers OR by the extras callback are committed to the FBO's
	// color texture before the imgui display step samples it.
	//
	// On macOS, OpenGL is layered on Metal which is deferred/tiled. Our
	// FBO writes can sit in a command queue past the end of this function
	// and not be visible when imgui samples the texture in the same
	// frame — the symptom is the texture appearing to contain pre-write
	// content (the world without the extras' contribution). Verified
	// empirically: a sprite render in extras was invisible until we
	// added a flush; with it the sprite shows correctly.
	gl.Flush()

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

// FramebufferSize returns the scene framebuffer dimensions in pixels.
// Used by the debug overlay.
func (s *Scene) FramebufferSize() (width, height int32) {
	return s.config.Width, s.config.Height
}

// FramebufferID returns the underlying GL framebuffer object ID.
// Used by diagnostics to verify the correct FB is bound.
func (s *Scene) FramebufferID() uint32 {
	if s.framebuffer == nil {
		return 0
	}
	return s.framebuffer.FBO()
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
	return s.terrainHeightmap.HeightAt(worldX, worldZ)
}

// HasGAT reports whether the map came with collision data.
func (s *Scene) HasGAT() bool { return s.GAT != nil }

// GatHeight is the walkable surface's height at a position, as the collision
// map gives it.
//
// The collision map is not the ground mesh. A town's raised plaza is built
// out of map models standing on flat ground, so the mesh says one height for
// the whole square while the collision map carries the height you actually
// walk at.
func (s *Scene) GatHeight(worldX, worldZ float32) float32 {
	return terrain.GetInterpolatedHeight(s.GAT, worldX, worldZ)
}

// TerrainProbe reports what the ground query is working from at a position:
// the tile it lands in, where inside that tile, and the tile's four corner
// heights.
//
// For telling a wrong height apart from a wrong sprite. A character that rises
// and falls as it walks is either standing on a surface the query has got
// wrong or being drawn off the surface it is standing on, and the two look
// alike from outside.
func (s *Scene) TerrainProbe(worldX, worldZ float32) (tileX, tileZ int, u, v float32, corners [4]float32) {
	return s.terrainHeightmap.Probe(worldX, worldZ)
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
