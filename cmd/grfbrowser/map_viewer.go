// 3D map viewer for GND/RSW files (ADR-013 Stage 1).
package main

import (
	"fmt"
	"image"
	"image/png"
	gomath "math"
	"os"
	"sort"
	"unsafe"

	"github.com/go-gl/gl/v4.1-core/gl"

	"github.com/Faultbox/midgard-ro/cmd/grfbrowser/shaders"
	"github.com/Faultbox/midgard-ro/internal/engine/camera"
	"github.com/Faultbox/midgard-ro/internal/engine/character"
	"github.com/Faultbox/midgard-ro/internal/engine/debug"
	"github.com/Faultbox/midgard-ro/internal/engine/lighting"
	rsmmodel "github.com/Faultbox/midgard-ro/internal/engine/model"
	"github.com/Faultbox/midgard-ro/internal/engine/picking"
	"github.com/Faultbox/midgard-ro/internal/engine/shader"
	"github.com/Faultbox/midgard-ro/internal/engine/shadow"
	"github.com/Faultbox/midgard-ro/internal/engine/sprite"
	"github.com/Faultbox/midgard-ro/internal/engine/terrain"
	"github.com/Faultbox/midgard-ro/internal/engine/water"
	"github.com/Faultbox/midgard-ro/internal/game/entity"
	"github.com/Faultbox/midgard-ro/pkg/formats"
	"github.com/Faultbox/midgard-ro/pkg/math"
)

// pointLightData stores extracted point light info for GPU upload.
type pointLightData struct {
	Position  [3]float32
	Color     [3]float32
	Range     float32
	Intensity float32
}

// MapDiagnostics tracks loading statistics for debugging.
type MapDiagnostics struct {
	// RSW model stats
	TotalModelsInRSW   int
	ModelsSkippedLimit int
	ModelsLoadFailed   int
	ModelsParseError   int
	ModelsNoNodes      int
	ModelsLoaded       int

	// RSM face stats
	TotalFaces    int
	TwoSidedFaces int
	TotalVertices int
	TotalNodes    int

	// Texture stats
	TexturesLoaded  int
	TexturesMissing int
	MissingTextures []string

	// Unique RSM files
	UniqueRSMFiles int

	// Failure details
	FailedModels []string
}

// MapModel represents a placed RSM model in the map.
type MapModel struct {
	vao        uint32
	vbo        uint32
	ebo        uint32
	indexCount int32
	textures   []uint32
	texGroups  []rsmmodel.TextureGroup
	position   [3]float32
	rotation   [3]float32
	scale      [3]float32
	// Debug info
	modelName  string
	bbox       [6]float32 // minX, minY, minZ, maxX, maxY, maxZ (after centering)
	instanceID int        // Unique instance ID for this model placement
	// Visibility and selection
	Visible bool // Whether this instance is rendered
	// Stats for debugging
	totalFaces   int
	twoSideFaces int
	// Extended debug info
	rsmVersion string
	nodeCount  int
	nodes      []rsmmodel.NodeDebugInfo
	// Animation support
	isAnimated bool              // Whether this model has keyframe animation
	rsm        *formats.RSM      // Reference to RSM for animation rebuild
	rswRef     *formats.RSWModel // Reference to RSW placement info
	animLength int32             // Animation length in ms
}

// ModelGroup represents a group of model instances sharing the same RSM.
type ModelGroup struct {
	RSMName    string // RSM filename (without path)
	Instances  []int  // Indices into MapViewer.models
	AllVisible bool   // Quick toggle for all instances
}

// CompositeFrame is an alias for character.CompositeFrame.
type CompositeFrame = character.CompositeFrame

// PlayerCharacter is an alias for character.Player.
type PlayerCharacter = character.Player

// saveAllDirectionsSheet saves all 8 direction composites into a single sprite sheet
func saveAllDirectionsSheet(
	bodySPR *formats.SPR, bodyACT *formats.ACT,
	headSPR *formats.SPR, headACT *formats.ACT,
	path string,
) {
	dirNames := []string{"S", "SW", "W", "NW", "N", "NE", "E", "SE"}

	// First pass: generate all composites and find max dimensions
	type dirComposite struct {
		pixels        []byte
		width, height int
	}
	composites := make([]dirComposite, 8)
	maxW, maxH := 0, 0

	for dir := 0; dir < 8; dir++ {
		result := sprite.CompositeSprites(bodySPR, bodyACT, headSPR, headACT, 0, dir, 0)
		composites[dir] = dirComposite{result.Pixels, result.Width, result.Height}
		if result.Width > maxW {
			maxW = result.Width
		}
		if result.Height > maxH {
			maxH = result.Height
		}
	}

	// Create combined image: 4 columns x 2 rows with labels
	padding := 10
	labelHeight := 20
	cellW := maxW + padding*2
	cellH := maxH + padding*2 + labelHeight
	sheetW := cellW * 4
	sheetH := cellH * 2

	// Create RGBA image with gray background
	sheet := image.NewRGBA(image.Rect(0, 0, sheetW, sheetH))
	// Fill with dark gray background
	for y := 0; y < sheetH; y++ {
		for x := 0; x < sheetW; x++ {
			idx := (y*sheetW + x) * 4
			sheet.Pix[idx] = 40    // R
			sheet.Pix[idx+1] = 40  // G
			sheet.Pix[idx+2] = 40  // B
			sheet.Pix[idx+3] = 255 // A
		}
	}

	// Layout: directions arranged as they appear when rotating camera
	// Top row: S(0), SE(7), E(6), NE(5)
	// Bottom row: SW(1), W(2), NW(3), N(4)
	layout := [][]int{
		{0, 7, 6, 5}, // Top row
		{1, 2, 3, 4}, // Bottom row
	}

	for row := 0; row < 2; row++ {
		for col := 0; col < 4; col++ {
			dir := layout[row][col]
			comp := composites[dir]

			// Calculate cell position
			cellX := col * cellW
			cellY := row * cellH

			// Center sprite in cell (below label area)
			spriteX := cellX + padding + (maxW-comp.width)/2
			spriteY := cellY + labelHeight + padding + (maxH-comp.height)/2

			// Copy sprite pixels
			if comp.pixels != nil {
				for py := 0; py < comp.height; py++ {
					for px := 0; px < comp.width; px++ {
						srcIdx := (py*comp.width + px) * 4
						dstX := spriteX + px
						dstY := spriteY + py
						if dstX >= 0 && dstX < sheetW && dstY >= 0 && dstY < sheetH {
							dstIdx := (dstY*sheetW + dstX) * 4
							// Copy with alpha
							sa := comp.pixels[srcIdx+3]
							if sa > 0 {
								sheet.Pix[dstIdx] = comp.pixels[srcIdx]
								sheet.Pix[dstIdx+1] = comp.pixels[srcIdx+1]
								sheet.Pix[dstIdx+2] = comp.pixels[srcIdx+2]
								sheet.Pix[dstIdx+3] = sa
							}
						}
					}
				}
			}

			// Draw direction label (simple pixel text)
			label := fmt.Sprintf("Dir %d (%s)", dir, dirNames[dir])
			drawSimpleText(sheet, cellX+padding, cellY+5, label)
		}
	}

	// Save the sheet
	f, err := os.Create(path)
	if err != nil {
		fmt.Printf("Error creating sprite sheet: %v\n", err)
		return
	}
	defer f.Close()
	if err := png.Encode(f, sheet); err != nil {
		fmt.Printf("Error encoding sprite sheet: %v\n", err)
		return
	}
	fmt.Printf("Saved all directions sprite sheet to %s\n", path)
}

// drawSimpleText draws text using a simple 5x7 pixel font
func drawSimpleText(img *image.RGBA, x, y int, text string) {
	// Simple 5x7 bitmap font for basic characters
	font := map[rune][]string{
		'D': {"####.", "#...#", "#...#", "#...#", "#...#", "#...#", "####."},
		'i': {"..#..", ".....", "..#..", "..#..", "..#..", "..#..", "..#.."},
		'r': {".....", ".....", ".###.", ".#...", ".#...", ".#...", ".#..."},
		' ': {".....", ".....", ".....", ".....", ".....", ".....", "....."},
		'0': {".###.", "#...#", "#..##", "#.#.#", "##..#", "#...#", ".###."},
		'1': {"..#..", ".##..", "..#..", "..#..", "..#..", "..#..", ".###."},
		'2': {".###.", "#...#", "....#", "..##.", ".#...", "#....", "#####"},
		'3': {".###.", "#...#", "....#", "..##.", "....#", "#...#", ".###."},
		'4': {"#...#", "#...#", "#...#", "#####", "....#", "....#", "....#"},
		'5': {"#####", "#....", "####.", "....#", "....#", "#...#", ".###."},
		'6': {".###.", "#....", "####.", "#...#", "#...#", "#...#", ".###."},
		'7': {"#####", "....#", "...#.", "..#..", ".#...", ".#...", ".#..."},
		'8': {".###.", "#...#", "#...#", ".###.", "#...#", "#...#", ".###."},
		'9': {".###.", "#...#", "#...#", ".####", "....#", "....#", ".###."},
		'(': {"..#..", ".#...", "#....", "#....", "#....", ".#...", "..#.."},
		')': {"..#..", "...#.", "....#", "....#", "....#", "...#.", "..#.."},
		'S': {".###.", "#....", ".###.", "....#", "....#", "#...#", ".###."},
		'W': {"#...#", "#...#", "#...#", "#.#.#", "#.#.#", "#.#.#", ".#.#."},
		'N': {"#...#", "##..#", "#.#.#", "#..##", "#...#", "#...#", "#...#"},
		'E': {"#####", "#....", "#....", "####.", "#....", "#....", "#####"},
	}

	curX := x
	for _, ch := range text {
		if glyph, ok := font[ch]; ok {
			for row, line := range glyph {
				for col, c := range line {
					if c == '#' {
						px := curX + col
						py := y + row
						if px >= 0 && px < img.Bounds().Dx() && py >= 0 && py < img.Bounds().Dy() {
							idx := (py*img.Bounds().Dx() + px) * 4
							img.Pix[idx] = 255   // R
							img.Pix[idx+1] = 255 // G
							img.Pix[idx+2] = 255 // B
							img.Pix[idx+3] = 255 // A
						}
					}
				}
			}
			curX += 6 // Character width + spacing
		} else {
			curX += 6 // Unknown char, just space
		}
	}
}

// MapViewer handles 3D rendering of complete RO maps.
type MapViewer struct {
	// Framebuffer resources
	fbo          uint32
	colorTexture uint32
	depthRBO     uint32
	width        int32
	height       int32

	// Terrain shader
	terrainProgram  uint32
	locViewProj     int32
	locLightDir     int32
	locAmbient      int32
	locDiffuse      int32
	locTexture      int32
	locLightmap     int32
	locBrightness   int32
	locLightOpacity int32
	locFogUse       int32
	locFogNear      int32
	locFogFar       int32
	locFogColor     int32

	// Model shader
	modelProgram     uint32
	locModelMVP      int32
	locModelLightDir int32
	locModelAmbient  int32
	locModelDiffuse  int32
	locModelTexture  int32
	locModelFogUse   int32
	locModelFogNear  int32
	locModelFogFar   int32
	locModelFogColor int32

	// Terrain mesh
	terrainVAO    uint32
	terrainVBO    uint32
	terrainEBO    uint32
	terrainGroups []terrain.TextureGroup

	// Ground textures and lightmap
	groundTextures   map[int]uint32
	fallbackTex      uint32
	lightmapAtlasTex uint32                 // GPU texture for lightmap atlas
	lightmapAtlas    *terrain.LightmapAtlas // Lightmap atlas metadata for UV calculation

	// Placed models
	models      []*MapModel
	ModelGroups []ModelGroup // Models grouped by RSM name
	MaxModels   int          // Maximum models to load (0 = unlimited)
	SelectedIdx int          // Currently selected model index (-1 = none)
	ModelFilter string       // Filter string for model names

	// Debug options
	ForceAllTwoSided bool // Force all faces to render as two-sided (debug)

	// Global scale multiplier for RSM models (buildings, props)
	ModelScale float32 // Multiplier applied to all model scales (default 1.0)

	// Diagnostics
	Diagnostics MapDiagnostics

	// Cameras
	OrbitCam  *camera.OrbitCamera       // For orbit/preview mode
	FollowCam *camera.ThirdPersonCamera // For play mode
	PlayMode  bool
	MoveSpeed float32

	// Player character (Play mode)
	Player            *PlayerCharacter
	spriteProgram     uint32 // Shader for billboard sprites
	locSpriteVP       int32  // viewProj uniform
	locSpritePos      int32  // world position uniform
	locSpriteSize     int32  // sprite size uniform
	locSpriteCamRight int32  // camera right vector for billboard
	locSpriteCamUp    int32  // camera up vector for billboard
	locSpriteTex      int32  // texture uniform
	locSpriteTint     int32  // color tint uniform

	// GAT data for terrain collision
	GAT *formats.GAT

	// Lighting from RSW
	lightDir     [3]float32 // Calculated from longitude/latitude
	ambientColor [3]float32 // From RSW.Light.Ambient
	diffuseColor [3]float32 // From RSW.Light.Diffuse
	lightOpacity float32    // Shadow opacity from RSW (affects ambient strength)
	Brightness   float32    // Terrain brightness multiplier (default 1.0)

	// Map bounds
	minBounds [3]float32
	maxBounds [3]float32

	// Map dimensions for coordinate conversion
	mapWidth  float32 // Width in world units (tiles * zoom)
	mapHeight float32 // Height in world units (tiles * zoom)

	// Terrain height data for model positioning (Stage 2 - ADR-014)
	terrainAltitudes [][]float32 // [x][y] -> average altitude at tile center
	terrainTileZoom  float32     // GND zoom factor
	terrainTilesX    int         // Number of tiles in X
	terrainTilesZ    int         // Number of tiles in Z

	// Water rendering (Stage 4 - ADR-014)
	waterProgram   uint32
	waterVAO       uint32
	waterVBO       uint32
	waterLevel     float32 // From RSW.Water.Level
	hasWater       bool    // Whether map has water
	locWaterMVP    int32
	locWaterColor  int32
	locWaterTime   int32
	locWaterTex    int32
	waterTime      float32  // Animation time
	waterTextures  []uint32 // Animated water texture frames
	waterAnimSpeed float32  // Animation speed from RSW
	waterFrame     int      // Current animation frame
	useWaterTex    bool     // Whether we have loaded water textures

	// Model animation (Stage 1 - ADR-014)
	modelAnimTime    float32     // Current animation time in ms
	modelAnimPlaying bool        // Whether model animations are playing
	animatedModels   []*MapModel // Models that need animation updates

	// Fog settings (Stage 4 - ADR-014) - public for UI controls
	FogEnabled bool
	FogNear    float32
	FogFar     float32
	FogColor   [3]float32

	// Shadow mapping (Enhanced Graphics)
	shadowMap                *shadow.Map
	shadowProgram            uint32
	ShadowsEnabled           bool  // Public for UI toggle
	ShadowResolution         int32 // Shadow map resolution (default 2048)
	lightViewProj            math.Mat4
	locShadowLightViewProj   int32 // Shadow shader uniform
	locShadowModel           int32 // Shadow shader model matrix
	locTerrainLightViewProj  int32 // Terrain shader shadow uniform
	locTerrainShadowMap      int32 // Terrain shader shadow map texture
	locTerrainShadowsEnabled int32 // Terrain shader shadow toggle
	locModelLightViewProj    int32 // Model shader shadow uniform
	locModelModel            int32 // Model shader model matrix
	locModelShadowMap        int32 // Model shader shadow map texture
	locModelShadowsEnabled   int32 // Model shader shadow toggle

	// Point lights from RSW (Enhanced Graphics Phase 3)
	pointLights         []pointLightData // Extracted from RSW
	PointLightsEnabled  bool             // Public for UI toggle
	PointLightIntensity float32          // Global intensity multiplier

	// Terrain shader point light uniforms
	locTerrainPointLightPositions   int32
	locTerrainPointLightColors      int32
	locTerrainPointLightRanges      int32
	locTerrainPointLightIntensities int32
	locTerrainPointLightCount       int32
	locTerrainPointLightsEnabled    int32

	// Model shader point light uniforms
	locModelPointLightPositions   int32
	locModelPointLightColors      int32
	locModelPointLightRanges      int32
	locModelPointLightIntensities int32
	locModelPointLightCount       int32
	locModelPointLightsEnabled    int32

	// Selection bounding box rendering
	bboxProgram  uint32
	bboxVAO      uint32
	bboxVBO      uint32
	locBboxMVP   int32
	locBboxColor int32

	// Cached matrices for picking
	lastViewProj math.Mat4
	lastView     math.Mat4
	lastProj     math.Mat4

	// Tile grid debug visualization
	tileGridProgram uint32
	tileGridVAO     uint32
	tileGridVBO     uint32
	tileGridEBO     uint32
	tileGridCount   int32          // Number of indices
	locTileGridMVP  int32          // MVP uniform location
	TileGridEnabled bool           // Public for UI toggle
	tileGrid        *terrain.TileGrid
}

// NewMapViewer creates a new 3D map viewer.
func NewMapViewer(width, height int32) (*MapViewer, error) {
	mv := &MapViewer{
		width:          width,
		height:         height,
		groundTextures: make(map[int]uint32),
		OrbitCam:       camera.NewOrbitCamera(),
		FollowCam:      camera.NewThirdPersonCamera(),
		MoveSpeed:      5.0,
		MaxModels:      1500, // Default model limit
		Brightness:     1.0,  // Default terrain brightness multiplier
		ModelScale:     1.0,  // Default model scale (1.0 = original size)
		SelectedIdx:    -1,   // No model selected initially
		// Default lighting (will be overwritten by RSW data)
		lightDir:     [3]float32{0.5, 0.866, 0.0}, // 60 degrees elevation
		ambientColor: [3]float32{0.3, 0.3, 0.3},
		diffuseColor: [3]float32{1.0, 1.0, 1.0},
		lightOpacity: 1.0, // Default shadow opacity
		// Shadow mapping defaults
		ShadowsEnabled:   true,
		ShadowResolution: shadow.DefaultResolution,
		// Point light defaults
		PointLightsEnabled:  true,
		PointLightIntensity: 1.0,
		// Render quality defaults
		ForceAllTwoSided: true, // Many RO models have missing back faces
	}

	if err := mv.createFramebuffer(); err != nil {
		return nil, fmt.Errorf("creating framebuffer: %w", err)
	}

	if err := mv.createTerrainShader(); err != nil {
		return nil, fmt.Errorf("creating terrain shader: %w", err)
	}

	if err := mv.createModelShader(); err != nil {
		return nil, fmt.Errorf("creating model shader: %w", err)
	}

	if err := mv.createBboxShader(); err != nil {
		return nil, fmt.Errorf("creating bbox shader: %w", err)
	}

	if err := mv.createSpriteShader(); err != nil {
		return nil, fmt.Errorf("creating sprite shader: %w", err)
	}

	if err := mv.createShadowShader(); err != nil {
		return nil, fmt.Errorf("creating shadow shader: %w", err)
	}

	if err := mv.createTileGridShader(); err != nil {
		return nil, fmt.Errorf("creating tile grid shader: %w", err)
	}

	// Initialize shadow map
	mv.shadowMap = shadow.NewMap(mv.ShadowResolution)
	if mv.shadowMap == nil {
		// Shadow mapping not critical - continue without it
		mv.ShadowsEnabled = false
	}

	mv.createFallbackTexture()

	return mv, nil
}

// createFramebuffer sets up the offscreen render target.
func (mv *MapViewer) createFramebuffer() error {
	// Create framebuffer
	gl.GenFramebuffers(1, &mv.fbo)
	gl.BindFramebuffer(gl.FRAMEBUFFER, mv.fbo)

	// Create color texture
	gl.GenTextures(1, &mv.colorTexture)
	gl.BindTexture(gl.TEXTURE_2D, mv.colorTexture)
	gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA8, mv.width, mv.height, 0, gl.RGBA, gl.UNSIGNED_BYTE, nil)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)
	gl.FramebufferTexture2D(gl.FRAMEBUFFER, gl.COLOR_ATTACHMENT0, gl.TEXTURE_2D, mv.colorTexture, 0)

	// Create depth renderbuffer
	gl.GenRenderbuffers(1, &mv.depthRBO)
	gl.BindRenderbuffer(gl.RENDERBUFFER, mv.depthRBO)
	gl.RenderbufferStorage(gl.RENDERBUFFER, gl.DEPTH_COMPONENT24, mv.width, mv.height)
	gl.FramebufferRenderbuffer(gl.FRAMEBUFFER, gl.DEPTH_ATTACHMENT, gl.RENDERBUFFER, mv.depthRBO)

	// Check completeness
	if gl.CheckFramebufferStatus(gl.FRAMEBUFFER) != gl.FRAMEBUFFER_COMPLETE {
		return fmt.Errorf("framebuffer not complete")
	}

	gl.BindFramebuffer(gl.FRAMEBUFFER, 0)
	return nil
}

// Resize updates the framebuffer size if dimensions changed.
func (mv *MapViewer) Resize(width, height int32) {
	if width == mv.width && height == mv.height {
		return
	}
	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = 1
	}

	mv.width = width
	mv.height = height

	// Resize color texture
	gl.BindTexture(gl.TEXTURE_2D, mv.colorTexture)
	gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA8, mv.width, mv.height, 0, gl.RGBA, gl.UNSIGNED_BYTE, nil)

	// Resize depth renderbuffer
	gl.BindRenderbuffer(gl.RENDERBUFFER, mv.depthRBO)
	gl.RenderbufferStorage(gl.RENDERBUFFER, gl.DEPTH_COMPONENT24, mv.width, mv.height)
}

// createTerrainShader compiles the terrain shader program.
func (mv *MapViewer) createTerrainShader() error {
	program, err := shader.CompileProgram(shaders.TerrainVertexShader, shaders.TerrainFragmentShader)
	if err != nil {
		return fmt.Errorf("terrain shader: %w", err)
	}
	mv.terrainProgram = program

	// Get uniform locations
	mv.locViewProj = shader.GetUniform(program, "uViewProj")
	mv.locLightDir = shader.GetUniform(program, "uLightDir")
	mv.locAmbient = shader.GetUniform(program, "uAmbient")
	mv.locDiffuse = shader.GetUniform(program, "uDiffuse")
	mv.locTexture = shader.GetUniform(program, "uTexture")
	mv.locLightmap = shader.GetUniform(program, "uLightmap")
	mv.locBrightness = shader.GetUniform(program, "uBrightness")
	mv.locLightOpacity = shader.GetUniform(program, "uLightOpacity")
	mv.locFogUse = shader.GetUniform(program, "uFogUse")
	mv.locFogNear = shader.GetUniform(program, "uFogNear")
	mv.locFogFar = shader.GetUniform(program, "uFogFar")
	mv.locFogColor = shader.GetUniform(program, "uFogColor")

	// Shadow mapping uniforms
	mv.locTerrainLightViewProj = shader.GetUniform(program, "uLightViewProj")
	mv.locTerrainShadowMap = shader.GetUniform(program, "uShadowMap")
	mv.locTerrainShadowsEnabled = shader.GetUniform(program, "uShadowsEnabled")

	// Point light uniforms
	mv.locTerrainPointLightPositions = shader.GetUniform(program, "uPointLightPositions")
	mv.locTerrainPointLightColors = shader.GetUniform(program, "uPointLightColors")
	mv.locTerrainPointLightRanges = shader.GetUniform(program, "uPointLightRanges")
	mv.locTerrainPointLightIntensities = shader.GetUniform(program, "uPointLightIntensities")
	mv.locTerrainPointLightCount = shader.GetUniform(program, "uPointLightCount")
	mv.locTerrainPointLightsEnabled = shader.GetUniform(program, "uPointLightsEnabled")

	return nil
}

// createModelShader compiles the RSM model shader program.
func (mv *MapViewer) createModelShader() error {
	program, err := shader.CompileProgram(shaders.ModelVertexShader, shaders.ModelFragmentShader)
	if err != nil {
		return fmt.Errorf("model shader: %w", err)
	}
	mv.modelProgram = program

	// Get uniform locations
	mv.locModelMVP = shader.GetUniform(program, "uMVP")
	mv.locModelLightDir = shader.GetUniform(program, "uLightDir")
	mv.locModelAmbient = shader.GetUniform(program, "uAmbient")
	mv.locModelDiffuse = shader.GetUniform(program, "uDiffuse")
	mv.locModelTexture = shader.GetUniform(program, "uTexture")
	mv.locModelFogUse = shader.GetUniform(program, "uFogUse")
	mv.locModelFogNear = shader.GetUniform(program, "uFogNear")
	mv.locModelFogFar = shader.GetUniform(program, "uFogFar")
	mv.locModelFogColor = shader.GetUniform(program, "uFogColor")

	// Shadow mapping uniforms
	mv.locModelLightViewProj = shader.GetUniform(program, "uLightViewProj")
	mv.locModelModel = shader.GetUniform(program, "uModel")
	mv.locModelShadowMap = shader.GetUniform(program, "uShadowMap")
	mv.locModelShadowsEnabled = shader.GetUniform(program, "uShadowsEnabled")

	// Point light uniforms
	mv.locModelPointLightPositions = shader.GetUniform(program, "uPointLightPositions")
	mv.locModelPointLightColors = shader.GetUniform(program, "uPointLightColors")
	mv.locModelPointLightRanges = shader.GetUniform(program, "uPointLightRanges")
	mv.locModelPointLightIntensities = shader.GetUniform(program, "uPointLightIntensities")
	mv.locModelPointLightCount = shader.GetUniform(program, "uPointLightCount")
	mv.locModelPointLightsEnabled = shader.GetUniform(program, "uPointLightsEnabled")

	// Compile water shader
	if err := mv.compileWaterShader(); err != nil {
		return fmt.Errorf("water shader: %w", err)
	}

	return nil
}

// createShadowShader compiles the shadow pass shader program.
func (mv *MapViewer) createShadowShader() error {
	program, err := shader.CompileProgram(shaders.ShadowVertexShader, shaders.ShadowFragmentShader)
	if err != nil {
		return fmt.Errorf("shadow shader: %w", err)
	}
	mv.shadowProgram = program

	// Get uniform locations
	mv.locShadowLightViewProj = shader.GetUniform(program, "uLightViewProj")
	mv.locShadowModel = shader.GetUniform(program, "uModel")

	return nil
}

// createBboxShader compiles the bounding box wireframe shader.
func (mv *MapViewer) createBboxShader() error {
	program, err := shader.CompileProgram(shaders.BboxVertexShader, shaders.BboxFragmentShader)
	if err != nil {
		return fmt.Errorf("bbox shader: %w", err)
	}
	mv.bboxProgram = program

	// Get uniform locations
	mv.locBboxMVP = shader.GetUniform(program, "uMVP")
	mv.locBboxColor = shader.GetUniform(program, "uColor")

	// Create VAO/VBO for bounding box (12 lines = 24 vertices)
	gl.GenVertexArrays(1, &mv.bboxVAO)
	gl.GenBuffers(1, &mv.bboxVBO)

	gl.BindVertexArray(mv.bboxVAO)
	gl.BindBuffer(gl.ARRAY_BUFFER, mv.bboxVBO)
	// Allocate space for 24 vertices (12 lines), will be updated per-frame
	gl.BufferData(gl.ARRAY_BUFFER, 24*3*4, nil, gl.DYNAMIC_DRAW)

	// Position attribute
	gl.VertexAttribPointerWithOffset(0, 3, gl.FLOAT, false, 3*4, 0)
	gl.EnableVertexAttribArray(0)

	gl.BindVertexArray(0)

	return nil
}

// createTileGridShader compiles the tile grid debug visualization shader.
func (mv *MapViewer) createTileGridShader() error {
	program, err := shader.CompileProgram(shaders.TileGridVertexShader, shaders.TileGridFragmentShader)
	if err != nil {
		return fmt.Errorf("tile grid shader: %w", err)
	}
	mv.tileGridProgram = program

	// Get uniform locations
	mv.locTileGridMVP = shader.GetUniform(program, "uMVP")

	return nil
}

// uploadTileGrid uploads the tile grid mesh to the GPU.
func (mv *MapViewer) uploadTileGrid() {
	if mv.tileGrid == nil || len(mv.tileGrid.Vertices) == 0 {
		return
	}

	// Clean up old resources
	if mv.tileGridVAO != 0 {
		gl.DeleteVertexArrays(1, &mv.tileGridVAO)
		gl.DeleteBuffers(1, &mv.tileGridVBO)
		gl.DeleteBuffers(1, &mv.tileGridEBO)
	}

	// Create VAO
	gl.GenVertexArrays(1, &mv.tileGridVAO)
	gl.GenBuffers(1, &mv.tileGridVBO)
	gl.GenBuffers(1, &mv.tileGridEBO)

	gl.BindVertexArray(mv.tileGridVAO)

	// Upload vertex data
	// TileGridVertex: Position [3]float32, Color [4]float32 = 28 bytes
	gl.BindBuffer(gl.ARRAY_BUFFER, mv.tileGridVBO)
	vertexSize := int(unsafe.Sizeof(terrain.TileGridVertex{}))
	gl.BufferData(gl.ARRAY_BUFFER, len(mv.tileGrid.Vertices)*vertexSize,
		unsafe.Pointer(&mv.tileGrid.Vertices[0]), gl.STATIC_DRAW)

	// Position attribute (location 0)
	gl.VertexAttribPointerWithOffset(0, 3, gl.FLOAT, false, int32(vertexSize), 0)
	gl.EnableVertexAttribArray(0)

	// Color attribute (location 1)
	gl.VertexAttribPointerWithOffset(1, 4, gl.FLOAT, false, int32(vertexSize), 3*4)
	gl.EnableVertexAttribArray(1)

	// Upload index data
	gl.BindBuffer(gl.ELEMENT_ARRAY_BUFFER, mv.tileGridEBO)
	gl.BufferData(gl.ELEMENT_ARRAY_BUFFER, len(mv.tileGrid.Indices)*4,
		unsafe.Pointer(&mv.tileGrid.Indices[0]), gl.STATIC_DRAW)

	mv.tileGridCount = int32(len(mv.tileGrid.Indices))

	gl.BindVertexArray(0)
}

// renderTileGrid renders the tile grid debug overlay.
// Uses robust GL state management to ensure grid is always visible on terrain.
func (mv *MapViewer) renderTileGrid(viewProj math.Mat4) {
	if mv.tileGridVAO == 0 || mv.tileGridCount == 0 {
		return
	}

	// Save current GL state
	var prevDepthFunc int32
	var cullFaceEnabled bool
	gl.GetIntegerv(gl.DEPTH_FUNC, &prevDepthFunc)
	cullFaceEnabled = gl.IsEnabled(gl.CULL_FACE)

	// Set up state for grid rendering:
	// 1. LEQUAL depth test - grid at same depth wins over terrain
	// 2. Disable backface culling - ensures grid visible from all angles
	// 3. Polygon offset - additional depth bias for reliability
	gl.DepthFunc(gl.LEQUAL)
	gl.Disable(gl.CULL_FACE)
	gl.Enable(gl.BLEND)
	gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)
	gl.Enable(gl.POLYGON_OFFSET_FILL)
	gl.PolygonOffset(-2.0, -2.0) // Negative values bring closer to camera

	// Use tile grid shader
	gl.UseProgram(mv.tileGridProgram)
	gl.UniformMatrix4fv(mv.locTileGridMVP, 1, false, &viewProj[0])

	// Draw filled tiles
	gl.BindVertexArray(mv.tileGridVAO)
	gl.DrawElements(gl.TRIANGLES, mv.tileGridCount, gl.UNSIGNED_INT, nil)

	// Draw black grid lines (wireframe)
	gl.Disable(gl.POLYGON_OFFSET_FILL)
	gl.Enable(gl.POLYGON_OFFSET_LINE)
	gl.PolygonOffset(-4.0, -4.0) // Push lines even closer to camera

	// Use bbox shader for solid black lines
	gl.UseProgram(mv.bboxProgram)
	gl.UniformMatrix4fv(mv.locBboxMVP, 1, false, &viewProj[0])
	gl.Uniform4f(mv.locBboxColor, 0.0, 0.0, 0.0, 0.9) // Black with slight transparency

	gl.PolygonMode(gl.FRONT_AND_BACK, gl.LINE)
	gl.LineWidth(1.0)
	gl.DrawElements(gl.TRIANGLES, mv.tileGridCount, gl.UNSIGNED_INT, nil)
	gl.PolygonMode(gl.FRONT_AND_BACK, gl.FILL)

	gl.BindVertexArray(0)

	// Restore all GL state
	gl.Disable(gl.POLYGON_OFFSET_LINE)
	gl.Disable(gl.BLEND)
	gl.DepthFunc(uint32(prevDepthFunc))
	if cullFaceEnabled {
		gl.Enable(gl.CULL_FACE)
	}
}

// createSpriteShader compiles the sprite billboard shader program.
func (mv *MapViewer) createSpriteShader() error {
	program, err := shader.CompileProgram(shaders.SpriteVertexShader, shaders.SpriteFragmentShader)
	if err != nil {
		return fmt.Errorf("sprite shader: %w", err)
	}
	mv.spriteProgram = program

	// Get uniform locations
	mv.locSpriteVP = shader.GetUniform(program, "uViewProj")
	mv.locSpritePos = shader.GetUniform(program, "uWorldPos")
	mv.locSpriteSize = shader.GetUniform(program, "uSpriteSize")
	mv.locSpriteTex = shader.GetUniform(program, "uTexture")
	mv.locSpriteTint = shader.GetUniform(program, "uTint")
	mv.locSpriteCamRight = shader.GetUniform(program, "uCamRight")
	mv.locSpriteCamUp = shader.GetUniform(program, "uCamUp")

	return nil
}

// createFallbackTexture creates a simple white texture for missing textures.
func (mv *MapViewer) createFallbackTexture() {
	gl.GenTextures(1, &mv.fallbackTex)
	gl.BindTexture(gl.TEXTURE_2D, mv.fallbackTex)

	white := []uint8{255, 255, 255, 255}
	gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA, 1, 1, 0, gl.RGBA, gl.UNSIGNED_BYTE, gl.Ptr(white))
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.NEAREST)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.NEAREST)
}

// compileWaterShader compiles the water rendering shader.
func (mv *MapViewer) compileWaterShader() error {
	program, err := shader.CompileProgram(shaders.WaterVertexShader, shaders.WaterFragmentShader)
	if err != nil {
		return fmt.Errorf("water shader: %w", err)
	}
	mv.waterProgram = program

	// Get uniform locations
	mv.locWaterMVP = shader.GetUniform(program, "uMVP")
	mv.locWaterColor = shader.GetUniform(program, "uWaterColor")
	mv.locWaterTime = shader.GetUniform(program, "uTime")
	mv.locWaterTex = shader.GetUniform(program, "uWaterTex")

	return nil
}

// loadWaterTextures loads water textures from GRF based on water type.
func (mv *MapViewer) loadWaterTextures(_ int32, texLoader func(string) ([]byte, error)) {
	// RO water textures are in data/texture/워터/ folder
	// Format: water%03d.jpg (000-031 for 32 frames)

	var textures []uint32

	// Load 32 frames of water animation
	for frame := 0; frame < 32; frame++ {
		path := fmt.Sprintf("data/texture/워터/water%03d.jpg", frame)

		data, err := texLoader(path)
		if err != nil {
			if frame == 0 {
				fmt.Printf("Water texture not found: %s\n", path)
			}
			continue
		}

		// Decode and upload texture
		img, err := decodeModelTexture(data, path, false)
		if err != nil {
			fmt.Printf("Failed to decode water texture: %s\n", path)
			continue
		}

		texID := uploadModelTexture(img)
		textures = append(textures, texID)
	}

	if len(textures) > 0 {
		mv.waterTextures = textures
		mv.useWaterTex = true
		fmt.Printf("Loaded %d water texture frames\n", len(textures))
	} else {
		fmt.Println("No water textures found, using procedural water")
		mv.useWaterTex = false
	}
}

// LoadMap loads a GND/RSW map for rendering.
func (mv *MapViewer) LoadMap(gnd *formats.GND, rsw *formats.RSW, texLoader func(string) ([]byte, error)) error {
	// Clear old resources
	mv.clearTerrain()

	// Store map dimensions for coordinate conversion (RSW positions are centered)
	mv.mapWidth = float32(gnd.Width) * gnd.Zoom
	mv.mapHeight = float32(gnd.Height) * gnd.Zoom

	// Store terrain height data for model positioning (Stage 2 - ADR-014)
	hm := terrain.BuildHeightmap(gnd)
	mv.terrainAltitudes = hm.Altitudes
	mv.terrainTilesX = hm.TilesX
	mv.terrainTilesZ = hm.TilesZ
	mv.terrainTileZoom = hm.TileZoom

	// Load GAT file for collision data (Play mode)
	if rsw != nil && rsw.GndFile != "" {
		// Derive GAT path from GND path (replace .gnd with .gat)
		// GndFile is like "prontera.gnd", need "data/prontera.gat"
		gatPath := "data/" + rsw.GndFile
		if len(gatPath) > 4 {
			gatPath = gatPath[:len(gatPath)-4] + ".gat"
		}
		gatData, err := texLoader(gatPath)
		if err == nil {
			gat, err := formats.ParseGAT(gatData)
			if err == nil {
				mv.GAT = gat
			} else {
				fmt.Printf("Warning: Failed to parse GAT: %v\n", err)
			}
		} else {
			fmt.Printf("Warning: GAT file not found: %s\n", gatPath)
		}
	}

	// Extract lighting data from RSW (Stage 1: Correct Lighting - ADR-014)
	if rsw != nil {
		// Calculate sun direction from spherical coordinates
		mv.lightDir = lighting.SunDirection(rsw.Light.Longitude, rsw.Light.Latitude)

		// Use RSW ambient and diffuse colors
		// Note: RSW values are often quite low, we apply a minimum floor
		// to prevent completely dark scenes
		mv.ambientColor = rsw.Light.Ambient
		mv.diffuseColor = rsw.Light.Diffuse

		// Shadow opacity from RSW (affects how strong ambient is relative to shadows)
		mv.lightOpacity = rsw.Light.Opacity
		if mv.lightOpacity <= 0 {
			mv.lightOpacity = 1.0 // Default if not set
		}

		// Ensure minimum ambient to prevent totally dark scenes
		// Reference implementations typically boost ambient
		minAmbient := float32(0.3)
		for i := 0; i < 3; i++ {
			if mv.ambientColor[i] < minAmbient {
				mv.ambientColor[i] = minAmbient
			}
		}

		// Extract point lights from RSW (Enhanced Graphics Phase 3)
		mv.extractPointLights(rsw)
	}

	// Load ground textures
	mv.loadGroundTextures(gnd, texLoader)

	// Build lightmap atlas (Stage 2)
	mv.lightmapAtlas = terrain.BuildLightmapAtlas(gnd)
	mv.uploadLightmapAtlas()

	// Build terrain mesh
	mesh := terrain.BuildMesh(gnd, mv.lightmapAtlas)
	mv.terrainGroups = mesh.Groups
	mv.minBounds = mesh.Bounds.Min
	mv.maxBounds = mesh.Bounds.Max

	// Upload to GPU
	mv.uploadTerrainMesh(mesh.Vertices, mesh.Indices)

	// Load RSM models from RSW (Stage 4)
	if rsw != nil {
		mv.loadModels(rsw, texLoader)
	}

	// Create water plane (Stage 4 - ADR-014)
	if rsw != nil && rsw.Water.Level != 0 {
		mv.createWaterPlane(gnd, rsw.Water.Level)
		mv.loadWaterTextures(rsw.Water.Type, texLoader)
		mv.waterAnimSpeed = float32(rsw.Water.AnimSpeed)
		if mv.waterAnimSpeed == 0 {
			mv.waterAnimSpeed = 30.0 // Fast animation speed for shimmering effect
		}
	}

	// Set up fog (Stage 4 - ADR-014)
	mv.FogEnabled = true
	mv.FogNear = 150.0
	mv.FogFar = 1400.0
	mv.FogColor = [3]float32{0.95, 0.90, 0.85} // Very subtle warm tint (barely visible)

	// Build tile grid from GAT (debug visualization - Korangar style)
	if mv.GAT != nil {
		// Grid at exact terrain position - LEQUAL depth test handles z-fighting
		const tileOffset float32 = 0.0
		mv.tileGrid = terrain.BuildTileGrid(mv.GAT, gnd, tileOffset)
		mv.uploadTileGrid()
	}

	// Fit camera to map
	mv.fitCamera()

	// Override with preferred defaults
	mv.OrbitCam.Distance = 340.0
	mv.modelAnimPlaying = true // Animation tracking enabled (rebuild disabled until fixed)

	return nil
}

// extractPointLights extracts point lights from RSW for GPU upload.
func (mv *MapViewer) extractPointLights(rsw *formats.RSW) {
	mv.pointLights = nil
	if rsw == nil {
		return
	}

	rswLights := rsw.GetLights()
	if len(rswLights) == 0 {
		return
	}

	// Limit to max supported lights
	count := len(rswLights)
	if count > lighting.MaxPointLights {
		count = lighting.MaxPointLights
	}

	mv.pointLights = make([]pointLightData, count)
	for i := 0; i < count; i++ {
		rswLight := rswLights[i]

		// RSW positions are centered; same coordinate system as terrain
		mv.pointLights[i] = pointLightData{
			Position: [3]float32{
				rswLight.Position[0],
				rswLight.Position[1],
				rswLight.Position[2],
			},
			Color: [3]float32{
				clampf(rswLight.Color[0], 0, 1),
				clampf(rswLight.Color[1], 0, 1),
				clampf(rswLight.Color[2], 0, 1),
			},
			Range:     rswLight.Range,
			Intensity: mv.PointLightIntensity,
		}

		// Ensure range is positive
		if mv.pointLights[i].Range <= 0 {
			mv.pointLights[i].Range = 100.0
		}
	}

	fmt.Printf("Extracted %d point lights from RSW\n", len(mv.pointLights))
}

// clampf clamps a float32 to [min, max].
func clampf(v, min, max float32) float32 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

// uploadPointLightsToShader uploads point light data to the currently bound shader.
func (mv *MapViewer) uploadPointLightsToShader(
	locPositions, locColors, locRanges, locIntensities, locCount, locEnabled int32,
) {
	// Set enabled flag
	if mv.PointLightsEnabled && len(mv.pointLights) > 0 {
		gl.Uniform1i(locEnabled, 1)
	} else {
		gl.Uniform1i(locEnabled, 0)
		gl.Uniform1i(locCount, 0)
		return
	}

	// Set light count
	count := int32(len(mv.pointLights))
	gl.Uniform1i(locCount, count)

	// Upload light arrays
	// Positions: vec3 array
	positions := make([]float32, lighting.MaxPointLights*3)
	for i, light := range mv.pointLights {
		positions[i*3+0] = light.Position[0]
		positions[i*3+1] = light.Position[1]
		positions[i*3+2] = light.Position[2]
	}
	gl.Uniform3fv(locPositions, lighting.MaxPointLights, &positions[0])

	// Colors: vec3 array
	colors := make([]float32, lighting.MaxPointLights*3)
	for i, light := range mv.pointLights {
		colors[i*3+0] = light.Color[0]
		colors[i*3+1] = light.Color[1]
		colors[i*3+2] = light.Color[2]
	}
	gl.Uniform3fv(locColors, lighting.MaxPointLights, &colors[0])

	// Ranges: float array
	ranges := make([]float32, lighting.MaxPointLights)
	for i, light := range mv.pointLights {
		ranges[i] = light.Range
	}
	gl.Uniform1fv(locRanges, lighting.MaxPointLights, &ranges[0])

	// Intensities: float array (apply global intensity multiplier)
	intensities := make([]float32, lighting.MaxPointLights)
	for i, light := range mv.pointLights {
		intensities[i] = light.Intensity * mv.PointLightIntensity
	}
	gl.Uniform1fv(locIntensities, lighting.MaxPointLights, &intensities[0])
}

// clearTerrain frees terrain GPU resources.
func (mv *MapViewer) clearTerrain() {
	if mv.terrainVAO != 0 {
		gl.DeleteVertexArrays(1, &mv.terrainVAO)
		mv.terrainVAO = 0
	}
	if mv.terrainVBO != 0 {
		gl.DeleteBuffers(1, &mv.terrainVBO)
		mv.terrainVBO = 0
	}
	if mv.terrainEBO != 0 {
		gl.DeleteBuffers(1, &mv.terrainEBO)
		mv.terrainEBO = 0
	}
	for _, tex := range mv.groundTextures {
		gl.DeleteTextures(1, &tex)
	}
	mv.groundTextures = make(map[int]uint32)
	mv.terrainGroups = nil
	if mv.lightmapAtlasTex != 0 {
		gl.DeleteTextures(1, &mv.lightmapAtlasTex)
		mv.lightmapAtlasTex = 0
	}
	mv.lightmapAtlas = nil

	// Clear models
	for _, model := range mv.models {
		if model.vao != 0 {
			gl.DeleteVertexArrays(1, &model.vao)
		}
		if model.vbo != 0 {
			gl.DeleteBuffers(1, &model.vbo)
		}
		if model.ebo != 0 {
			gl.DeleteBuffers(1, &model.ebo)
		}
		for _, tex := range model.textures {
			gl.DeleteTextures(1, &tex)
		}
	}
	mv.models = nil
	mv.animatedModels = nil // Clear animated models list too
	mv.modelAnimTime = 0    // Reset animation time
}

// loadGroundTextures loads textures from GRF.
func (mv *MapViewer) loadGroundTextures(gnd *formats.GND, texLoader func(string) ([]byte, error)) {
	for i, texPath := range gnd.Textures {
		// Build full path
		fullPath := "data/texture/" + texPath

		data, err := texLoader(fullPath)
		if err != nil {
			continue
		}

		// Decode texture with magenta key enabled
		// Some terrain textures (like Yuno railings) use magenta for transparency
		img, err := decodeModelTexture(data, fullPath, true)
		if err != nil {
			continue
		}

		// Upload to GPU
		texID := uploadModelTexture(img)
		mv.groundTextures[i] = texID
	}
}

// DebugModelPositioning enables debug output for model positioning issues.
var DebugModelPositioning = false

// loadModels loads RSM models from RSW object list.
func (mv *MapViewer) loadModels(rsw *formats.RSW, texLoader func(string) ([]byte, error)) {
	allModels := rsw.GetModels()

	// Reset diagnostics
	mv.Diagnostics = MapDiagnostics{
		TotalModelsInRSW: len(allModels),
	}

	// Limit number of models to avoid performance issues
	// Use configured MaxModels, default to 1500 if not set
	maxModels := mv.MaxModels
	if maxModels <= 0 {
		maxModels = 1500 // Default limit
	}
	models := allModels
	if len(models) > maxModels {
		mv.Diagnostics.ModelsSkippedLimit = len(models) - maxModels
		models = models[:maxModels]
	}

	if DebugModelPositioning {
		fmt.Printf("Loading %d models (max %d)\n", len(models), maxModels)
	}

	// Cache loaded RSM files to avoid reloading
	rsmCache := make(map[string]*formats.RSM)

	for _, modelRef := range models {
		// Load RSM if not cached
		rsmPath := "data/model/" + modelRef.ModelName
		rsm, ok := rsmCache[rsmPath]
		if !ok {
			data, err := texLoader(rsmPath)
			if err != nil {
				mv.Diagnostics.ModelsLoadFailed++
				mv.Diagnostics.FailedModels = append(mv.Diagnostics.FailedModels, modelRef.ModelName+" (load: "+err.Error()+")")
				continue
			}
			rsm, err = formats.ParseRSM(data)
			if err != nil {
				mv.Diagnostics.ModelsParseError++
				mv.Diagnostics.FailedModels = append(mv.Diagnostics.FailedModels, modelRef.ModelName+" (parse: "+err.Error()+")")
				continue
			}
			rsmCache[rsmPath] = rsm
		}

		// Build map model from RSM
		mapModel := mv.buildMapModel(rsm, modelRef, texLoader)
		if mapModel != nil {
			mapModel.instanceID = len(mv.models)
			mv.models = append(mv.models, mapModel)
			mv.Diagnostics.ModelsLoaded++
			// Track animated models for animation updates
			if mapModel.isAnimated {
				mv.animatedModels = append(mv.animatedModels, mapModel)
			}
		} else {
			mv.Diagnostics.ModelsNoNodes++
		}
	}

	mv.Diagnostics.UniqueRSMFiles = len(rsmCache)

	// Build model groups for scene tree
	mv.buildModelGroups()
}

// buildModelGroups creates groups of model instances by RSM name.
func (mv *MapViewer) buildModelGroups() {
	groupMap := make(map[string][]int)

	for i, model := range mv.models {
		if model == nil {
			continue
		}
		groupMap[model.modelName] = append(groupMap[model.modelName], i)
	}

	// Convert to slice and sort by name
	mv.ModelGroups = make([]ModelGroup, 0, len(groupMap))
	for name, indices := range groupMap {
		mv.ModelGroups = append(mv.ModelGroups, ModelGroup{
			RSMName:    name,
			Instances:  indices,
			AllVisible: true,
		})
	}

	// Sort groups alphabetically
	sort.Slice(mv.ModelGroups, func(i, j int) bool {
		return mv.ModelGroups[i].RSMName < mv.ModelGroups[j].RSMName
	})
}

// SetGroupVisibility sets visibility for all instances in a model group.
func (mv *MapViewer) SetGroupVisibility(groupIdx int, visible bool) {
	if groupIdx < 0 || groupIdx >= len(mv.ModelGroups) {
		return
	}
	mv.ModelGroups[groupIdx].AllVisible = visible
	for _, modelIdx := range mv.ModelGroups[groupIdx].Instances {
		if modelIdx >= 0 && modelIdx < len(mv.models) && mv.models[modelIdx] != nil {
			mv.models[modelIdx].Visible = visible
		}
	}
}

// SetAllModelsVisible sets visibility for all models.
func (mv *MapViewer) SetAllModelsVisible(visible bool) {
	for i := range mv.ModelGroups {
		mv.ModelGroups[i].AllVisible = visible
	}
	for _, model := range mv.models {
		if model != nil {
			model.Visible = visible
		}
	}
}

// GetModel returns a model by index, or nil if invalid.
func (mv *MapViewer) GetModel(idx int) *MapModel {
	if idx < 0 || idx >= len(mv.models) {
		return nil
	}
	return mv.models[idx]
}

// GetVisibleCount returns the number of visible models.
func (mv *MapViewer) GetVisibleCount() int {
	count := 0
	for _, model := range mv.models {
		if model != nil && model.Visible {
			count++
		}
	}
	return count
}

// FocusOnModel moves the camera to focus on a specific model.
func (mv *MapViewer) FocusOnModel(idx int) {
	model := mv.GetModel(idx)
	if model == nil {
		return
	}

	// Calculate world position of model
	offsetX := mv.mapWidth / 2
	offsetZ := mv.mapHeight / 2
	worldX := model.position[0] + offsetX
	worldY := -model.position[1]
	worldZ := model.position[2] + offsetZ

	// Set camera center to model position
	mv.OrbitCam.SetCenter(worldX, worldY, worldZ)

	// Set reasonable zoom distance based on model bounding box
	bboxSize := gomath.Max(
		float64(model.bbox[3]-model.bbox[0]),
		gomath.Max(float64(model.bbox[4]-model.bbox[1]), float64(model.bbox[5]-model.bbox[2])),
	)
	mv.OrbitCam.Distance = float32(gomath.Max(bboxSize*2, 50))
}

// HasNegativeScale returns true if the model has a negative scale determinant.
func (m *MapModel) HasNegativeScale() bool {
	return m.scale[0]*m.scale[1]*m.scale[2] < 0
}

// GetRSMVersion returns the RSM version string.
func (m *MapModel) GetRSMVersion() string {
	return m.rsmVersion
}

// GetNodeCount returns the number of nodes in the RSM.
func (m *MapModel) GetNodeCount() int {
	return m.nodeCount
}

// GetNodes returns the node debug info for this model.
func (m *MapModel) GetNodes() []rsmmodel.NodeDebugInfo {
	return m.nodes
}

// GetModelWorldPosition returns the world position of the model.
func (mv *MapViewer) GetModelWorldPosition(idx int) (float32, float32, float32) {
	model := mv.GetModel(idx)
	if model == nil {
		return 0, 0, 0
	}
	offsetX := mv.mapWidth / 2
	offsetZ := mv.mapHeight / 2
	return model.position[0] + offsetX, -model.position[1], model.position[2] + offsetZ
}

// buildMapModel creates a MapModel from RSM data with world transform.
func (mv *MapViewer) buildMapModel(rsm *formats.RSM, ref *formats.RSWModel, texLoader func(string) ([]byte, error)) *MapModel {
	if len(rsm.Nodes) == 0 {
		return nil
	}

	// Track nodes
	mv.Diagnostics.TotalNodes += len(rsm.Nodes)

	// Build mesh from all RSM nodes
	var vertices []rsmmodel.Vertex
	var indices []uint32
	texGroups := make(map[int][]uint32)

	// Load model textures
	modelTextures := make([]uint32, len(rsm.Textures))
	for i, texName := range rsm.Textures {
		texPath := "data/texture/" + texName
		data, err := texLoader(texPath)
		if err != nil {
			modelTextures[i] = mv.fallbackTex
			mv.Diagnostics.TexturesMissing++
			// Only add unique missing textures
			found := false
			for _, t := range mv.Diagnostics.MissingTextures {
				if t == texName {
					found = true
					break
				}
			}
			if !found {
				mv.Diagnostics.MissingTextures = append(mv.Diagnostics.MissingTextures, texName)
			}
			continue
		}
		img, err := decodeModelTexture(data, texPath, true) // Use magenta key
		if err != nil {
			modelTextures[i] = mv.fallbackTex
			mv.Diagnostics.TexturesMissing++
			continue
		}
		modelTextures[i] = uploadModelTexture(img)
		mv.Diagnostics.TexturesLoaded++
	}

	// Track bounding box for centering
	var minVertX, minVertY, minVertZ float32 = 1e10, 1e10, 1e10
	var maxVertX, maxVertY, maxVertZ float32 = -1e10, -1e10, -1e10

	// Process each node
	for i := range rsm.Nodes {
		node := &rsm.Nodes[i]
		baseIdx := uint32(len(vertices))

		// Build node transform matrix (with parent hierarchy)
		nodeMatrix := rsmmodel.BuildNodeMatrix(node, rsm, 0) // Initial pose (time=0)

		// Check if we need to reverse winding due to negative scale
		reverseWinding := ref.Scale[0]*ref.Scale[1]*ref.Scale[2] < 0

		// Process faces
		for _, face := range node.Faces {
			// Track face stats
			mv.Diagnostics.TotalFaces++
			isTwoSided := face.TwoSide != 0
			if isTwoSided {
				mv.Diagnostics.TwoSidedFaces++
			}

			// Skip faces with insufficient vertices
			if len(face.VertexIDs) < 3 {
				continue
			}

			// Bounds check vertex IDs
			validFace := true
			for _, vid := range face.VertexIDs {
				if int(vid) >= len(node.Vertices) {
					validFace = false
					break
				}
			}
			if !validFace {
				continue
			}

			// Calculate face normal from first 3 vertices
			v0 := node.Vertices[face.VertexIDs[0]]
			v1 := node.Vertices[face.VertexIDs[1]]
			v2 := node.Vertices[face.VertexIDs[2]]
			e1 := [3]float32{v1[0] - v0[0], v1[1] - v0[1], v1[2] - v0[2]}
			e2 := [3]float32{v2[0] - v0[0], v2[1] - v0[1], v2[2] - v0[2]}
			normalVec := rsmmodel.Cross(e1, e2)

			// Degenerate triangle detection - skip if normal is too small
			normalMag := float32(gomath.Sqrt(float64(normalVec[0]*normalVec[0] + normalVec[1]*normalVec[1] + normalVec[2]*normalVec[2])))
			if normalMag < 1e-5 {
				continue // Skip degenerate triangle
			}

			normal := [3]float32{normalVec[0] / normalMag, normalVec[1] / normalMag, normalVec[2] / normalMag}

			// Helper to add face vertices
			addFaceVertices := func(reverseOrder bool, flipNormal bool) uint32 {
				faceBaseIdx := uint32(len(vertices))
				faceNormal := normal
				if flipNormal {
					faceNormal = [3]float32{-normal[0], -normal[1], -normal[2]}
				}

				// Determine vertex order (RSM faces are always triangles = 3 vertices)
				var vertIDs [3]uint16
				var texIDs [3]uint16
				if reverseOrder {
					// Reverse vertex order for back face: 0,1,2 -> 2,1,0
					vertIDs = [3]uint16{face.VertexIDs[2], face.VertexIDs[1], face.VertexIDs[0]}
					texIDs = [3]uint16{face.TexCoordIDs[2], face.TexCoordIDs[1], face.TexCoordIDs[0]}
				} else {
					vertIDs = face.VertexIDs
					texIDs = face.TexCoordIDs
				}

				for i := 0; i < 3; i++ {
					vid := vertIDs[i]
					v := node.Vertices[vid]

					// Transform vertex position by node matrix
					pos := rsmmodel.TransformPoint(nodeMatrix, v)

					// Flip Y for RO coordinate system
					pos[1] = -pos[1]

					// Track bounding box
					if pos[0] < minVertX {
						minVertX = pos[0]
					}
					if pos[0] > maxVertX {
						maxVertX = pos[0]
					}
					if pos[1] < minVertY {
						minVertY = pos[1]
					}
					if pos[1] > maxVertY {
						maxVertY = pos[1]
					}
					if pos[2] < minVertZ {
						minVertZ = pos[2]
					}
					if pos[2] > maxVertZ {
						maxVertZ = pos[2]
					}

					// Get texture coordinates
					var uv [2]float32
					if int(texIDs[i]) < len(node.TexCoords) {
						tc := node.TexCoords[texIDs[i]]
						uv = [2]float32{tc.U, tc.V}
					}

					vertices = append(vertices, rsmmodel.Vertex{
						Position: pos,
						Normal:   faceNormal,
						TexCoord: uv,
					})
				}
				return faceBaseIdx
			}

			// Add front face (with winding reversal if negative scale)
			faceBaseIdx := addFaceVertices(reverseWinding, false)

			// Add indices for triangle (RSM faces are always triangles)
			// face.TextureID is index into node.TextureIDs
			// node.TextureIDs[i] is the global index into rsm.Textures
			globalTexIdx := 0
			if int(face.TextureID) < len(node.TextureIDs) {
				globalTexIdx = int(node.TextureIDs[face.TextureID])
			}
			texGroups[globalTexIdx] = append(texGroups[globalTexIdx],
				faceBaseIdx,
				faceBaseIdx+1,
				faceBaseIdx+2,
			)

			// If TwoSide (or ForceAllTwoSided debug flag), add back face
			if isTwoSided || mv.ForceAllTwoSided {
				backFaceBaseIdx := addFaceVertices(!reverseWinding, true)
				texGroups[globalTexIdx] = append(texGroups[globalTexIdx],
					backFaceBaseIdx,
					backFaceBaseIdx+1,
					backFaceBaseIdx+2,
				)
			}
		}
		_ = baseIdx // Silence unused warning
	}

	// Track total vertices
	mv.Diagnostics.TotalVertices += len(vertices)

	if len(vertices) == 0 {
		return nil
	}

	// Center models horizontally (X/Z) but NOT vertically (Y)
	// Preserving Y offset fixes positioning of decorative elements attached to structures
	centerX := (minVertX + maxVertX) / 2
	centerZ := (minVertZ + maxVertZ) / 2
	for i := range vertices {
		vertices[i].Position[0] -= centerX
		// Don't center Y - preserve original vertical offset from RSM
		vertices[i].Position[2] -= centerZ
	}

	// Store bounding box after centering (for debug visualization)
	bboxAfter := [6]float32{
		minVertX - centerX, minVertY, minVertZ - centerZ,
		maxVertX - centerX, maxVertY, maxVertZ - centerZ,
	}

	// Debug: log model centering info
	if DebugModelPositioning {
		height := maxVertY - minVertY
		fmt.Printf("Model: %s | RSW pos: (%.1f,%.1f,%.1f) rot: (%.1f,%.1f,%.1f) | BBox: (%.1f,%.1f,%.1f)-(%.1f,%.1f,%.1f) | Height: %.1f\n",
			ref.ModelName,
			ref.Position[0], ref.Position[1], ref.Position[2],
			ref.Rotation[0], ref.Rotation[1], ref.Rotation[2],
			minVertX, minVertY, minVertZ, maxVertX, maxVertY, maxVertZ, height)
	}

	// Build texture groups
	var groups []rsmmodel.TextureGroup
	for texIdx, idxs := range texGroups {
		if len(idxs) == 0 {
			continue
		}
		groups = append(groups, rsmmodel.TextureGroup{
			TextureIdx: texIdx,
			StartIndex: int32(len(indices)),
			IndexCount: int32(len(idxs)),
		})
		indices = append(indices, idxs...)
	}

	// Smooth normals for models (reduces faceted appearance)
	rsmmodel.SmoothNormals(vertices)

	// Count total and two-sided faces for this model
	modelTotalFaces := 0
	modelTwoSideFaces := 0
	for i := range rsm.Nodes {
		for _, face := range rsm.Nodes[i].Faces {
			modelTotalFaces++
			if face.TwoSide != 0 {
				modelTwoSideFaces++
			}
		}
	}

	// Build node debug info
	nodeDebugInfo := rsmmodel.BuildNodeDebugInfo(rsm)

	// Check if model has animation
	hasAnimation := rsmmodel.HasAnimation(rsm)

	// Create GPU resources
	model := &MapModel{
		textures:     modelTextures,
		texGroups:    groups,
		position:     ref.Position,
		rotation:     ref.Rotation,
		scale:        ref.Scale,
		modelName:    ref.ModelName,
		bbox:         bboxAfter,
		Visible:      true, // Visible by default
		totalFaces:   modelTotalFaces,
		twoSideFaces: modelTwoSideFaces,
		rsmVersion:   rsm.Version.String(),
		nodeCount:    len(rsm.Nodes),
		nodes:        nodeDebugInfo,
		// Animation support
		isAnimated: hasAnimation,
		animLength: rsm.AnimLength,
	}

	// Store RSM reference for animated models (needed for mesh rebuild)
	if hasAnimation {
		model.rsm = rsm
		model.rswRef = ref
	}

	// Upload mesh to GPU
	gl.GenVertexArrays(1, &model.vao)
	gl.BindVertexArray(model.vao)

	gl.GenBuffers(1, &model.vbo)
	gl.BindBuffer(gl.ARRAY_BUFFER, model.vbo)
	gl.BufferData(gl.ARRAY_BUFFER, len(vertices)*int(unsafe.Sizeof(rsmmodel.Vertex{})), gl.Ptr(vertices), gl.STATIC_DRAW)

	gl.GenBuffers(1, &model.ebo)
	gl.BindBuffer(gl.ELEMENT_ARRAY_BUFFER, model.ebo)
	gl.BufferData(gl.ELEMENT_ARRAY_BUFFER, len(indices)*4, gl.Ptr(indices), gl.STATIC_DRAW)

	model.indexCount = int32(len(indices))

	// Set vertex attributes (Position, Normal, TexCoord)
	stride := int32(unsafe.Sizeof(rsmmodel.Vertex{}))
	gl.EnableVertexAttribArray(0)
	gl.VertexAttribPointerWithOffset(0, 3, gl.FLOAT, false, stride, 0)
	gl.EnableVertexAttribArray(1)
	gl.VertexAttribPointerWithOffset(1, 3, gl.FLOAT, false, stride, 12)
	gl.EnableVertexAttribArray(2)
	gl.VertexAttribPointerWithOffset(2, 2, gl.FLOAT, false, stride, 24)

	gl.BindVertexArray(0)

	return model
}

// buildAnimatedModelMesh builds vertices and indices for an animated model at a given time.
func (mv *MapViewer) buildAnimatedModelMesh(rsm *formats.RSM, ref *formats.RSWModel, animTimeMs float32) ([]rsmmodel.Vertex, []uint32, []rsmmodel.TextureGroup) {
	var vertices []rsmmodel.Vertex
	var indices []uint32
	texGroups := make(map[int][]uint32)

	// Process each node
	for i := range rsm.Nodes {
		node := &rsm.Nodes[i]

		// Build node transform with animation time
		nodeMatrix := rsmmodel.BuildNodeMatrix(node, rsm, animTimeMs)

		// Check if we need to reverse winding
		reverseWinding := ref.Scale[0]*ref.Scale[1]*ref.Scale[2] < 0

		// Process faces
		for _, face := range node.Faces {
			if len(face.VertexIDs) < 3 {
				continue
			}

			// Bounds check
			validFace := true
			for _, vid := range face.VertexIDs {
				if int(vid) >= len(node.Vertices) {
					validFace = false
					break
				}
			}
			if !validFace {
				continue
			}

			// Calculate face normal
			v0 := node.Vertices[face.VertexIDs[0]]
			v1 := node.Vertices[face.VertexIDs[1]]
			v2 := node.Vertices[face.VertexIDs[2]]
			e1 := [3]float32{v1[0] - v0[0], v1[1] - v0[1], v1[2] - v0[2]}
			e2 := [3]float32{v2[0] - v0[0], v2[1] - v0[1], v2[2] - v0[2]}
			normalVec := rsmmodel.Cross(e1, e2)
			normalMag := float32(gomath.Sqrt(float64(normalVec[0]*normalVec[0] + normalVec[1]*normalVec[1] + normalVec[2]*normalVec[2])))
			if normalMag < 1e-5 {
				continue
			}
			normal := [3]float32{normalVec[0] / normalMag, normalVec[1] / normalMag, normalVec[2] / normalMag}

			// Helper to add face vertices
			addFaceVerts := func(reverseOrder bool, flipNormal bool) uint32 {
				faceBaseIdx := uint32(len(vertices))
				faceNormal := normal
				if flipNormal {
					faceNormal = [3]float32{-normal[0], -normal[1], -normal[2]}
				}

				var vertIDs [3]uint16
				var texIDs [3]uint16
				if reverseOrder {
					vertIDs = [3]uint16{face.VertexIDs[2], face.VertexIDs[1], face.VertexIDs[0]}
					texIDs = [3]uint16{face.TexCoordIDs[2], face.TexCoordIDs[1], face.TexCoordIDs[0]}
				} else {
					vertIDs = face.VertexIDs
					texIDs = face.TexCoordIDs
				}

				for j := 0; j < 3; j++ {
					pos := node.Vertices[vertIDs[j]]
					transformedPos := rsmmodel.TransformPoint(nodeMatrix, pos)
					// Flip Y for RO coordinate system
					transformedPos[1] = -transformedPos[1]

					var uv [2]float32
					if int(texIDs[j]) < len(node.TexCoords) {
						tc := node.TexCoords[texIDs[j]]
						uv = [2]float32{tc.U, tc.V}
					}

					vertices = append(vertices, rsmmodel.Vertex{
						Position: transformedPos,
						Normal:   faceNormal,
						TexCoord: uv,
					})
				}
				return faceBaseIdx
			}

			// Add front face
			faceBaseIdx := addFaceVerts(reverseWinding, false)

			// Get global texture index
			globalTexIdx := 0
			if int(face.TextureID) < len(node.TextureIDs) {
				globalTexIdx = int(node.TextureIDs[face.TextureID])
			}
			texGroups[globalTexIdx] = append(texGroups[globalTexIdx],
				faceBaseIdx, faceBaseIdx+1, faceBaseIdx+2)

			// Add back face if two-sided
			if face.TwoSide != 0 || mv.ForceAllTwoSided {
				backIdx := addFaceVerts(!reverseWinding, true)
				texGroups[globalTexIdx] = append(texGroups[globalTexIdx],
					backIdx, backIdx+1, backIdx+2)
			}
		}
	}

	// Build indices and groups
	var groups []rsmmodel.TextureGroup
	for texIdx, idxs := range texGroups {
		startIdx := int32(len(indices))
		indices = append(indices, idxs...)
		groups = append(groups, rsmmodel.TextureGroup{
			TextureIdx: texIdx,
			StartIndex: startIdx,
			IndexCount: int32(len(idxs)),
		})
	}

	return vertices, indices, groups
}

// uploadTerrainMesh uploads mesh data to GPU.
func (mv *MapViewer) uploadTerrainMesh(vertices []terrain.Vertex, indices []uint32) {
	if len(vertices) == 0 {
		return
	}

	// Create VAO
	gl.GenVertexArrays(1, &mv.terrainVAO)
	gl.BindVertexArray(mv.terrainVAO)

	// Create VBO
	gl.GenBuffers(1, &mv.terrainVBO)
	gl.BindBuffer(gl.ARRAY_BUFFER, mv.terrainVBO)
	gl.BufferData(gl.ARRAY_BUFFER, len(vertices)*int(unsafe.Sizeof(terrain.Vertex{})), gl.Ptr(vertices), gl.STATIC_DRAW)

	// Create EBO
	gl.GenBuffers(1, &mv.terrainEBO)
	gl.BindBuffer(gl.ELEMENT_ARRAY_BUFFER, mv.terrainEBO)
	gl.BufferData(gl.ELEMENT_ARRAY_BUFFER, len(indices)*4, gl.Ptr(indices), gl.STATIC_DRAW)

	// Set vertex attributes
	// terrain.Vertex: Position(12) + Normal(12) + TexCoord(8) + LightmapUV(8) + Color(16) = 56 bytes
	stride := int32(unsafe.Sizeof(terrain.Vertex{}))

	// Position (location 0) - offset 0
	gl.EnableVertexAttribArray(0)
	gl.VertexAttribPointerWithOffset(0, 3, gl.FLOAT, false, stride, 0)

	// Normal (location 1) - offset 12
	gl.EnableVertexAttribArray(1)
	gl.VertexAttribPointerWithOffset(1, 3, gl.FLOAT, false, stride, 12)

	// TexCoord (location 2) - offset 24
	gl.EnableVertexAttribArray(2)
	gl.VertexAttribPointerWithOffset(2, 2, gl.FLOAT, false, stride, 24)

	// LightmapUV (location 3) - offset 32
	gl.EnableVertexAttribArray(3)
	gl.VertexAttribPointerWithOffset(3, 2, gl.FLOAT, false, stride, 32)

	// Color (location 4) - offset 40
	gl.EnableVertexAttribArray(4)
	gl.VertexAttribPointerWithOffset(4, 4, gl.FLOAT, false, stride, 40)

	gl.BindVertexArray(0)
}

// uploadLightmapAtlas uploads the lightmap atlas texture to GPU.
func (mv *MapViewer) uploadLightmapAtlas() {
	if mv.lightmapAtlas == nil {
		return
	}

	gl.GenTextures(1, &mv.lightmapAtlasTex)
	gl.BindTexture(gl.TEXTURE_2D, mv.lightmapAtlasTex)
	gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA, mv.lightmapAtlas.Size, mv.lightmapAtlas.Size, 0,
		gl.RGBA, gl.UNSIGNED_BYTE, gl.Ptr(mv.lightmapAtlas.Data))

	// Generate mipmaps for smooth lightmap at distance
	gl.GenerateMipmap(gl.TEXTURE_2D)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR_MIPMAP_LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
}

// renderShadowPass renders the scene to the shadow map for shadow calculations.
func (mv *MapViewer) renderShadowPass() {
	if mv.shadowMap == nil || !mv.shadowMap.IsValid() {
		return
	}

	// Bind shadow map framebuffer
	mv.shadowMap.Bind()

	// Use shadow shader
	gl.UseProgram(mv.shadowProgram)
	gl.UniformMatrix4fv(mv.locShadowLightViewProj, 1, false, &mv.lightViewProj[0])

	// Render terrain to shadow map (terrain is at origin, identity model matrix)
	identityMatrix := math.Identity()
	gl.UniformMatrix4fv(mv.locShadowModel, 1, false, &identityMatrix[0])

	gl.BindVertexArray(mv.terrainVAO)
	for _, group := range mv.terrainGroups {
		gl.DrawElementsWithOffset(gl.TRIANGLES, group.IndexCount, gl.UNSIGNED_INT, uintptr(group.StartIndex*4))
	}

	// Render models to shadow map
	offsetX := mv.mapWidth / 2
	offsetZ := mv.mapHeight / 2

	for _, model := range mv.models {
		if model.vao == 0 || model.indexCount == 0 || !model.Visible {
			continue
		}

		// Build model matrix (same as in renderModels)
		worldX := model.position[0] + offsetX
		worldY := -model.position[1]
		worldZ := model.position[2] + offsetZ

		modelMatrix := math.Identity()
		modelMatrix = modelMatrix.Mul(math.Translate(worldX, worldY, worldZ))
		modelMatrix = modelMatrix.Mul(math.RotateY(model.rotation[1] * gomath.Pi / 180))
		modelMatrix = modelMatrix.Mul(math.RotateX(model.rotation[0] * gomath.Pi / 180))
		modelMatrix = modelMatrix.Mul(math.RotateZ(model.rotation[2] * gomath.Pi / 180))
		// Apply per-model scale multiplied by global ModelScale
		modelMatrix = modelMatrix.Mul(math.Scale(
			model.scale[0]*mv.ModelScale,
			model.scale[1]*mv.ModelScale,
			model.scale[2]*mv.ModelScale,
		))

		gl.UniformMatrix4fv(mv.locShadowModel, 1, false, &modelMatrix[0])

		gl.BindVertexArray(model.vao)
		for _, group := range model.texGroups {
			gl.DrawElementsWithOffset(gl.TRIANGLES, group.IndexCount, gl.UNSIGNED_INT, uintptr(group.StartIndex*4))
		}
	}

	// Unbind shadow map framebuffer
	mv.shadowMap.Unbind()
	gl.BindVertexArray(0)
}

// fitCamera positions camera to view entire map.
func (mv *MapViewer) fitCamera() {
	mv.OrbitCam.FitToBounds(
		mv.minBounds[0], mv.minBounds[1], mv.minBounds[2],
		mv.maxBounds[0], mv.maxBounds[1], mv.maxBounds[2],
	)
}

// Render renders the map to the framebuffer and returns the texture ID.
func (mv *MapViewer) Render() uint32 {
	if mv.terrainVAO == 0 {
		return mv.colorTexture
	}

	// Calculate view-projection matrix first (needed for shadow pass too)
	aspect := float32(mv.width) / float32(mv.height)
	proj := math.Perspective(45.0, aspect, 1.0, 10000.0)

	var view math.Mat4
	if mv.PlayMode && mv.Player != nil {
		player := mv.Player
		view = mv.FollowCam.ViewMatrix(player.WorldX, player.WorldY, player.WorldZ)
	} else if mv.PlayMode {
		view = mv.OrbitCam.ViewMatrix()
	} else {
		view = mv.OrbitCam.ViewMatrix()
	}

	viewProj := proj.Mul(view)

	// Cache matrices for picking
	mv.lastView = view
	mv.lastProj = proj
	mv.lastViewProj = viewProj

	// Calculate light view-projection for shadow mapping
	if mv.ShadowsEnabled && mv.shadowMap != nil && mv.shadowMap.IsValid() {
		sceneBounds := shadow.AABB{
			Min: mv.minBounds,
			Max: mv.maxBounds,
		}
		mv.lightViewProj = shadow.CalculateDirectionalLightMatrix(mv.lightDir, sceneBounds)

		// Render shadow pass
		mv.renderShadowPass()
	}

	// Bind main framebuffer
	gl.BindFramebuffer(gl.FRAMEBUFFER, mv.fbo)
	gl.Viewport(0, 0, mv.width, mv.height)

	// Clear
	gl.ClearColor(0.4, 0.6, 0.9, 1.0) // Sky blue
	gl.Clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT)

	// Enable depth test
	gl.Enable(gl.DEPTH_TEST)
	gl.DepthFunc(gl.LESS)

	// Use terrain shader with RSW lighting data
	gl.UseProgram(mv.terrainProgram)
	gl.UniformMatrix4fv(mv.locViewProj, 1, false, &viewProj[0])
	gl.Uniform3f(mv.locLightDir, mv.lightDir[0], mv.lightDir[1], mv.lightDir[2])
	gl.Uniform3f(mv.locAmbient, mv.ambientColor[0], mv.ambientColor[1], mv.ambientColor[2])
	gl.Uniform3f(mv.locDiffuse, mv.diffuseColor[0], mv.diffuseColor[1], mv.diffuseColor[2])
	gl.Uniform1i(mv.locTexture, 0)
	gl.Uniform1i(mv.locLightmap, 1)
	gl.Uniform1f(mv.locBrightness, mv.Brightness)
	gl.Uniform1f(mv.locLightOpacity, mv.lightOpacity)

	// Shadow mapping uniforms for terrain
	gl.UniformMatrix4fv(mv.locTerrainLightViewProj, 1, false, &mv.lightViewProj[0])
	gl.Uniform1i(mv.locTerrainShadowMap, 2) // Shadow map on texture unit 2
	if mv.ShadowsEnabled && mv.shadowMap != nil && mv.shadowMap.IsValid() {
		gl.Uniform1i(mv.locTerrainShadowsEnabled, 1)
	} else {
		gl.Uniform1i(mv.locTerrainShadowsEnabled, 0)
	}

	// Fog uniforms
	if mv.FogEnabled {
		gl.Uniform1i(mv.locFogUse, 1)
	} else {
		gl.Uniform1i(mv.locFogUse, 0)
	}
	gl.Uniform1f(mv.locFogNear, mv.FogNear)
	gl.Uniform1f(mv.locFogFar, mv.FogFar)
	gl.Uniform3f(mv.locFogColor, mv.FogColor[0], mv.FogColor[1], mv.FogColor[2])

	// Point light uniforms (terrain shader)
	mv.uploadPointLightsToShader(
		mv.locTerrainPointLightPositions,
		mv.locTerrainPointLightColors,
		mv.locTerrainPointLightRanges,
		mv.locTerrainPointLightIntensities,
		mv.locTerrainPointLightCount,
		mv.locTerrainPointLightsEnabled,
	)

	// Bind lightmap atlas to texture unit 1
	gl.ActiveTexture(gl.TEXTURE1)
	if mv.lightmapAtlasTex != 0 {
		gl.BindTexture(gl.TEXTURE_2D, mv.lightmapAtlasTex)
	} else {
		gl.BindTexture(gl.TEXTURE_2D, mv.fallbackTex)
	}

	// Bind shadow map to texture unit 2
	if mv.shadowMap != nil && mv.shadowMap.IsValid() {
		mv.shadowMap.BindTexture(gl.TEXTURE2)
	}

	// Bind terrain VAO
	gl.BindVertexArray(mv.terrainVAO)

	// Render each texture group
	gl.ActiveTexture(gl.TEXTURE0)
	for _, group := range mv.terrainGroups {
		tex, ok := mv.groundTextures[group.TextureID]
		if !ok {
			tex = mv.fallbackTex
		}
		gl.BindTexture(gl.TEXTURE_2D, tex)
		gl.DrawElementsWithOffset(gl.TRIANGLES, group.IndexCount, gl.UNSIGNED_INT, uintptr(group.StartIndex*4))
	}

	gl.BindVertexArray(0)

	// Render tile grid (debug visualization)
	if mv.TileGridEnabled && mv.tileGridVAO != 0 {
		mv.renderTileGrid(viewProj)
	}

	// Render placed models
	mv.renderModels(viewProj)

	// Render player character (in Play mode)
	if mv.PlayMode && mv.Player != nil {
		// Update animation (assuming ~60fps = 16ms per frame)
		mv.UpdatePlayerAnimation(16.0)
		mv.renderPlayerCharacter(viewProj)
	}

	// Render water (last, with transparency)
	mv.renderWater(viewProj)

	// Render selection bounding box (on top of everything)
	mv.renderSelectionBbox(viewProj)

	gl.BindFramebuffer(gl.FRAMEBUFFER, 0)

	return mv.colorTexture
}

// renderPlayerCharacter renders the player sprite as a billboard in the 3D scene.
// Uses camera-facing billboard + directional sprite selection for 3D illusion.
func (mv *MapViewer) renderPlayerCharacter(viewProj math.Mat4) {
	player := mv.Player
	if player == nil || player.VAO == 0 {
		return
	}

	// Render shadow first (below character)
	mv.renderPlayerShadow(viewProj)

	// ========== STEP 1: Calculate camera-facing billboard vectors ==========
	// Use render position for smooth visual appearance
	camRight, camUp := character.BillboardVectors(mv.FollowCam.PosX, mv.FollowCam.PosZ, player.RenderX, player.RenderZ)

	// ========== STEP 2: Calculate visual direction with hysteresis ==========
	cameraAngle := character.CameraAngleToPlayer(mv.FollowCam.PosX, mv.FollowCam.PosZ, player.RenderX, player.RenderZ)
	visualDir, newSector := character.CalculateVisualDirection(cameraAngle, player.Direction, player.LastVisualDir)
	player.LastVisualDir = newSector

	// ========== STEP 3: Use composite sprites if available ==========
	// Composite sprites have head+body pre-merged for solid appearance
	if player.UseComposite && player.CompositeFrames != nil {
		actionDirKey := player.CurrentAction*8 + visualDir
		if frames, ok := player.CompositeFrames[actionDirKey]; ok && len(frames) > 0 {
			// For idle action, always use frame 0 to prevent head bobbing
			// Walking uses animated frames
			frameIdx := 0
			if player.CurrentAction == entity.ActionWalk && len(frames) > 1 {
				frameIdx = player.CurrentFrame % len(frames)
			}
			composite := frames[frameIdx]

			if composite.Texture != 0 && composite.Width > 0 && composite.Height > 0 {
				// All composites are now padded to same dimensions
				spriteWidth := float32(composite.Width) * player.SpriteScale
				spriteHeight := float32(composite.Height) * player.SpriteScale

				gl.Enable(gl.BLEND)
				gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)
				gl.UseProgram(mv.spriteProgram)

				// Position sprite centered on player, lift to align feet with ground
				// Use render position for smooth interpolated movement
				posX := player.RenderX
				posY := player.RenderY + spriteHeight*0.12
				posZ := player.RenderZ

				gl.UniformMatrix4fv(mv.locSpriteVP, 1, false, &viewProj[0])
				gl.Uniform3f(mv.locSpritePos, posX, posY, posZ)
				gl.Uniform2f(mv.locSpriteSize, spriteWidth, spriteHeight)
				gl.Uniform4f(mv.locSpriteTint, 1.0, 1.0, 1.0, 1.0)
				gl.Uniform3f(mv.locSpriteCamRight, camRight[0], camRight[1], camRight[2])
				gl.Uniform3f(mv.locSpriteCamUp, camUp[0], camUp[1], camUp[2])

				gl.ActiveTexture(gl.TEXTURE0)
				gl.BindTexture(gl.TEXTURE_2D, composite.Texture)
				gl.Uniform1i(mv.locSpriteTex, 0)

				gl.BindVertexArray(player.VAO)
				gl.DrawArrays(gl.TRIANGLE_STRIP, 0, 4)
				gl.BindVertexArray(0)

				gl.Disable(gl.BLEND)
				return // Done - composite rendered
			}
		}
	}

	// ========== FALLBACK: Render body + head separately ==========
	if len(player.Textures) == 0 {
		return
	}

	var spriteID int
	var spriteWidth, spriteHeight float32
	var tint [4]float32

	if player.ACT != nil && player.SPR != nil {
		// Get animation frame using visualDir for the 3D illusion
		actionIdx := player.CurrentAction*8 + visualDir
		if actionIdx >= len(player.ACT.Actions) {
			actionIdx = 0
		}
		action := &player.ACT.Actions[actionIdx]
		if len(action.Frames) == 0 {
			return
		}

		frameIdx := player.CurrentFrame % len(action.Frames)
		frame := &action.Frames[frameIdx]
		if len(frame.Layers) == 0 {
			return
		}

		// Get the first valid layer
		var layer *formats.Layer
		for i := range frame.Layers {
			if frame.Layers[i].SpriteID >= 0 && int(frame.Layers[i].SpriteID) < len(player.Textures) {
				layer = &frame.Layers[i]
				break
			}
		}
		if layer == nil {
			return
		}

		spriteID = int(layer.SpriteID)
		if spriteID >= len(player.Textures) {
			return
		}

		// Get sprite dimensions
		sprImg := &player.SPR.Images[spriteID]
		layerScaleX := layer.ScaleX
		layerScaleY := layer.ScaleY
		if layerScaleX == 0 {
			layerScaleX = 1.0
		}
		if layerScaleY == 0 {
			layerScaleY = 1.0
		}
		spriteWidth = float32(sprImg.Width) * player.SpriteScale * layerScaleX
		spriteHeight = float32(sprImg.Height) * player.SpriteScale * layerScaleY
		tint = [4]float32{1.0, 1.0, 1.0, 1.0}
	} else {
		spriteID = 0
		spriteWidth = 32 * player.SpriteScale
		spriteHeight = 64 * player.SpriteScale
		tint = [4]float32{1.0, 1.0, 1.0, 1.0}
	}

	// Mirror for left-facing directions
	if visualDir == entity.DirSW || visualDir == entity.DirW || visualDir == entity.DirNW {
		spriteWidth = -spriteWidth
	}

	gl.Enable(gl.BLEND)
	gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)
	gl.UseProgram(mv.spriteProgram)

	gl.UniformMatrix4fv(mv.locSpriteVP, 1, false, &viewProj[0])
	// Use render position for smooth interpolated movement
	gl.Uniform3f(mv.locSpritePos, player.RenderX, player.RenderY, player.RenderZ)
	gl.Uniform2f(mv.locSpriteSize, spriteWidth, spriteHeight)
	gl.Uniform4f(mv.locSpriteTint, tint[0], tint[1], tint[2], tint[3])
	gl.Uniform3f(mv.locSpriteCamRight, camRight[0], camRight[1], camRight[2])
	gl.Uniform3f(mv.locSpriteCamUp, camUp[0], camUp[1], camUp[2])

	gl.ActiveTexture(gl.TEXTURE0)
	gl.BindTexture(gl.TEXTURE_2D, player.Textures[spriteID])
	gl.Uniform1i(mv.locSpriteTex, 0)

	gl.BindVertexArray(player.VAO)
	gl.DrawArrays(gl.TRIANGLE_STRIP, 0, 4)
	gl.BindVertexArray(0)

	// Render head separately if no composite available
	if player.HeadSPR != nil && player.HeadACT != nil && len(player.HeadTextures) > 0 && player.ACT != nil {
		bodyActionIdx := player.CurrentAction*8 + visualDir
		if bodyActionIdx >= len(player.ACT.Actions) {
			bodyActionIdx = 0
		}
		bodyAction := &player.ACT.Actions[bodyActionIdx]
		bodyFrameIdx := 0
		if bodyFrameIdx >= len(bodyAction.Frames) {
			bodyFrameIdx = 0
		}
		bodyFrame := &bodyAction.Frames[bodyFrameIdx]

		if len(bodyFrame.AnchorPoints) > 0 {
			bodyAnchorX := float32(bodyFrame.AnchorPoints[0].X)
			bodyAnchorY := float32(bodyFrame.AnchorPoints[0].Y)

			var bodyLayerY float32
			for _, bl := range bodyFrame.Layers {
				if bl.SpriteID >= 0 {
					bodyLayerY = float32(bl.Y)
					break
				}
			}

			headActionIdx := player.CurrentAction*8 + visualDir
			if headActionIdx >= len(player.HeadACT.Actions) {
				headActionIdx = visualDir
			}
			if headActionIdx >= len(player.HeadACT.Actions) {
				headActionIdx = 0
			}

			headAction := &player.HeadACT.Actions[headActionIdx]
			if len(headAction.Frames) > 0 {
				headFrameIdx := 0
				headFrame := &headAction.Frames[headFrameIdx]

				var headAnchorX, headAnchorY float32
				if len(headFrame.AnchorPoints) > 0 {
					headAnchorX = float32(headFrame.AnchorPoints[0].X)
					headAnchorY = float32(headFrame.AnchorPoints[0].Y)
				}

				offsetX := (bodyAnchorX - headAnchorX) * player.SpriteScale
				offsetY := (bodyAnchorY - headAnchorY) * player.SpriteScale

				for _, headLayer := range headFrame.Layers {
					headSpriteID := int(headLayer.SpriteID)
					if headSpriteID < 0 || headSpriteID >= len(player.HeadTextures) {
						continue
					}

					headImg := &player.HeadSPR.Images[headSpriteID]
					headScaleX := headLayer.ScaleX
					headScaleY := headLayer.ScaleY
					if headScaleX == 0 {
						headScaleX = 1.0
					}
					if headScaleY == 0 {
						headScaleY = 1.0
					}

					headWidth := float32(headImg.Width) * player.SpriteScale * headScaleX
					headHeight := float32(headImg.Height) * player.SpriteScale * headScaleY

					if visualDir == entity.DirSW || visualDir == entity.DirW || visualDir == entity.DirNW {
						headWidth = -headWidth
					}

					layerX := float32(headLayer.X) * player.SpriteScale
					layerY := float32(headLayer.Y) * player.SpriteScale

					totalOffsetX := offsetX + layerX

					// Use render position for smooth interpolated movement
					headPosX := player.RenderX + totalOffsetX*camRight[0]
					headPosY := player.RenderY - (offsetY + layerY) + (bodyLayerY * player.SpriteScale * 0.35)
					headPosZ := player.RenderZ + totalOffsetX*camRight[2]

					gl.Disable(gl.DEPTH_TEST)
					gl.Uniform3f(mv.locSpritePos, headPosX, headPosY, headPosZ)
					gl.Uniform2f(mv.locSpriteSize, headWidth, headHeight)
					gl.BindTexture(gl.TEXTURE_2D, player.HeadTextures[headSpriteID])
					gl.BindVertexArray(player.VAO)
					gl.DrawArrays(gl.TRIANGLE_STRIP, 0, 4)
					gl.BindVertexArray(0)
					gl.Enable(gl.DEPTH_TEST)
				}
			}
		}
	}

	gl.Disable(gl.BLEND)
}

// renderPlayerShadow renders the shadow ellipse on the ground under the player.
func (mv *MapViewer) renderPlayerShadow(viewProj math.Mat4) {
	player := mv.Player
	if player == nil || player.ShadowVAO == 0 || player.ShadowTex == 0 {
		return
	}

	// Enable blending for semi-transparent shadow
	gl.Enable(gl.BLEND)
	gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)

	// Disable depth write so shadow doesn't occlude terrain
	gl.DepthMask(false)

	// Use sprite shader (reuse for simplicity)
	gl.UseProgram(mv.spriteProgram)

	// Shadow position slightly above ground to avoid z-fighting
	// Use render position for smooth interpolated movement
	shadowY := player.RenderY + 0.5

	// Set uniforms - position the shadow flat on the ground
	gl.UniformMatrix4fv(mv.locSpriteVP, 1, false, &viewProj[0])
	gl.Uniform3f(mv.locSpritePos, player.RenderX, shadowY, player.RenderZ)
	gl.Uniform2f(mv.locSpriteSize, 1.0, 1.0) // Size is baked into VBO
	gl.Uniform4f(mv.locSpriteTint, 1.0, 1.0, 1.0, 1.0)
	// Shadow is flat on ground (XZ plane), not camera-facing
	gl.Uniform3f(mv.locSpriteCamRight, 1.0, 0.0, 0.0) // X axis
	gl.Uniform3f(mv.locSpriteCamUp, 0.0, 0.0, 1.0)    // Z axis (flat)

	// Bind shadow texture
	gl.ActiveTexture(gl.TEXTURE0)
	gl.BindTexture(gl.TEXTURE_2D, player.ShadowTex)
	gl.Uniform1i(mv.locSpriteTex, 0)

	// Draw shadow quad
	gl.BindVertexArray(player.ShadowVAO)
	gl.DrawArrays(gl.TRIANGLE_STRIP, 0, 4)
	gl.BindVertexArray(0)

	// Restore depth write
	gl.DepthMask(true)
}

// UpdatePlayerAnimation advances player animation frame based on time.
func (mv *MapViewer) UpdatePlayerAnimation(deltaMs float32) {
	character.UpdateAnimation(mv.Player, deltaMs)
}

// renderWater renders the water plane with transparency.
func (mv *MapViewer) renderWater(viewProj math.Mat4) {
	if !mv.hasWater || mv.waterVAO == 0 {
		return
	}

	// Update water animation time
	mv.waterTime += 0.016

	// Enable blending for transparency
	gl.Enable(gl.BLEND)
	gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)

	// Use water shader
	gl.UseProgram(mv.waterProgram)
	gl.UniformMatrix4fv(mv.locWaterMVP, 1, false, &viewProj[0])

	// Water color: fully opaque when using texture
	gl.Uniform4f(mv.locWaterColor, 0.2, 0.4, 0.7, 1.0)

	// Pass animation time and scroll speed
	gl.Uniform1f(mv.locWaterTime, mv.waterTime)
	locScrollSpeed := gl.GetUniformLocation(mv.waterProgram, gl.Str("uScrollSpeed\x00"))
	gl.Uniform1f(locScrollSpeed, mv.waterAnimSpeed)

	// Set up texture if we have water textures loaded
	if mv.useWaterTex && len(mv.waterTextures) > 0 {
		// Update animation frame based on time and speed
		mv.waterFrame = water.CalculateAnimFrame(mv.waterTime, mv.waterAnimSpeed, len(mv.waterTextures))

		// Bind water texture
		gl.ActiveTexture(gl.TEXTURE0)
		gl.BindTexture(gl.TEXTURE_2D, mv.waterTextures[mv.waterFrame])
		gl.Uniform1i(mv.locWaterTex, 0)

		// Tell shader to use texture
		locUseTexture := gl.GetUniformLocation(mv.waterProgram, gl.Str("uUseTexture\x00"))
		gl.Uniform1i(locUseTexture, 1)
	} else {
		// Tell shader to use procedural water
		locUseTexture := gl.GetUniformLocation(mv.waterProgram, gl.Str("uUseTexture\x00"))
		gl.Uniform1i(locUseTexture, 0)
	}

	// Render water quad
	gl.BindVertexArray(mv.waterVAO)
	gl.DrawArrays(gl.TRIANGLE_FAN, 0, 4)
	gl.BindVertexArray(0)

	// Disable blending
	gl.Disable(gl.BLEND)
}

// renderSelectionBbox draws a wireframe bounding box around the selected model.
func (mv *MapViewer) renderSelectionBbox(viewProj math.Mat4) {
	if mv.SelectedIdx < 0 || mv.bboxVAO == 0 {
		return
	}

	model := mv.GetModel(mv.SelectedIdx)
	if model == nil || !model.Visible {
		return
	}

	// Calculate world position
	offsetX := mv.mapWidth / 2
	offsetZ := mv.mapHeight / 2
	worldPos := [3]float32{
		model.position[0] + offsetX,
		-model.position[1],
		model.position[2] + offsetZ,
	}

	// Generate wireframe vertices using debug package
	vertices := debug.GenerateBBoxWireframeFromAABB(model.bbox, worldPos, model.scale, debug.DefaultBBoxPadding)

	// Update VBO
	gl.BindBuffer(gl.ARRAY_BUFFER, mv.bboxVBO)
	gl.BufferSubData(gl.ARRAY_BUFFER, 0, len(vertices)*4, unsafe.Pointer(&vertices[0]))

	// Disable depth test to draw on top
	gl.Disable(gl.DEPTH_TEST)
	gl.LineWidth(2.0)

	// Draw
	gl.UseProgram(mv.bboxProgram)
	gl.UniformMatrix4fv(mv.locBboxMVP, 1, false, &viewProj[0])
	gl.Uniform4f(mv.locBboxColor, 1.0, 0.0, 1.0, 1.0) // Purple/Magenta

	gl.BindVertexArray(mv.bboxVAO)
	gl.DrawArrays(gl.LINES, 0, 24)
	gl.BindVertexArray(0)

	// Re-enable depth test
	gl.Enable(gl.DEPTH_TEST)
	gl.LineWidth(1.0)
}

// PickModelAtScreen returns the index of the model at screen coordinates, or -1 if none.
func (mv *MapViewer) PickModelAtScreen(screenX, screenY, viewWidth, viewHeight float32) int {
	if len(mv.models) == 0 {
		return -1
	}

	// Create ray from screen coordinates
	ray := picking.ScreenToRay(screenX, screenY, viewWidth, viewHeight, mv.lastViewProj.Inverse())

	// Test intersection with each visible model's bounding box
	offsetX := mv.mapWidth / 2
	offsetZ := mv.mapHeight / 2

	bestIdx := -1
	bestDist := float32(gomath.MaxFloat32)

	for i, model := range mv.models {
		if model == nil || !model.Visible {
			continue
		}

		// Calculate world-space bounding box
		worldPos := [3]float32{
			model.position[0] + offsetX,
			-model.position[1],
			model.position[2] + offsetZ,
		}
		box := picking.TransformAABB(model.bbox, worldPos, model.scale)

		// Ray-AABB intersection test
		if hitDist, hit := ray.IntersectAABB(box); hit {
			if hitDist < bestDist {
				bestDist = hitDist
				bestIdx = i
			}
		}
	}

	return bestIdx
}

// renderModels renders all placed RSM models.
func (mv *MapViewer) renderModels(viewProj math.Mat4) {
	if len(mv.models) == 0 {
		return
	}

	gl.UseProgram(mv.modelProgram)
	gl.Uniform3f(mv.locModelLightDir, mv.lightDir[0], mv.lightDir[1], mv.lightDir[2])
	gl.Uniform3f(mv.locModelAmbient, mv.ambientColor[0], mv.ambientColor[1], mv.ambientColor[2])
	gl.Uniform3f(mv.locModelDiffuse, mv.diffuseColor[0], mv.diffuseColor[1], mv.diffuseColor[2])
	gl.Uniform1i(mv.locModelTexture, 0)

	// Shadow mapping uniforms for models
	gl.UniformMatrix4fv(mv.locModelLightViewProj, 1, false, &mv.lightViewProj[0])
	gl.Uniform1i(mv.locModelShadowMap, 2) // Shadow map on texture unit 2
	if mv.ShadowsEnabled && mv.shadowMap != nil && mv.shadowMap.IsValid() {
		gl.Uniform1i(mv.locModelShadowsEnabled, 1)
		mv.shadowMap.BindTexture(gl.TEXTURE2)
	} else {
		gl.Uniform1i(mv.locModelShadowsEnabled, 0)
	}

	// Fog uniforms for models
	if mv.FogEnabled {
		gl.Uniform1i(mv.locModelFogUse, 1)
	} else {
		gl.Uniform1i(mv.locModelFogUse, 0)
	}
	gl.Uniform1f(mv.locModelFogNear, mv.FogNear)
	gl.Uniform1f(mv.locModelFogFar, mv.FogFar)
	gl.Uniform3f(mv.locModelFogColor, mv.FogColor[0], mv.FogColor[1], mv.FogColor[2])

	// Point light uniforms (model shader)
	mv.uploadPointLightsToShader(
		mv.locModelPointLightPositions,
		mv.locModelPointLightColors,
		mv.locModelPointLightRanges,
		mv.locModelPointLightIntensities,
		mv.locModelPointLightCount,
		mv.locModelPointLightsEnabled,
	)

	gl.ActiveTexture(gl.TEXTURE0)

	// RSW positions are centered at map origin (0,0,0)
	// GND terrain spans from (0,0) to (mapWidth, mapHeight)
	// Convert by adding map center offset
	offsetX := mv.mapWidth / 2
	offsetZ := mv.mapHeight / 2

	for _, model := range mv.models {
		if model.vao == 0 || model.indexCount == 0 || !model.Visible {
			continue
		}

		// Convert RSW position to GND world coordinates:
		// - RSW X (0 = center) -> World X = rswX + mapWidth/2
		// - RSW Y (altitude) -> World Y = -rswY (same convention as GND: positive = lower)
		// - RSW Z (0 = center) -> World Z = rswZ + mapHeight/2
		worldX := model.position[0] + offsetX
		worldY := -model.position[1]
		worldZ := model.position[2] + offsetZ

		// Build model matrix: translate first, then apply rotation and scale
		// Order: T * Ry * Rx * Rz * BaseRot * S (applied right-to-left)
		modelMatrix := math.Identity()

		// Apply translation to world position
		modelMatrix = modelMatrix.Mul(math.Translate(worldX, worldY, worldZ))

		// Apply RSW rotations (in degrees)
		// Note: RSW stores rotation as [X, Y, Z] in degrees
		modelMatrix = modelMatrix.Mul(math.RotateY(model.rotation[1] * gomath.Pi / 180))
		modelMatrix = modelMatrix.Mul(math.RotateX(model.rotation[0] * gomath.Pi / 180))
		modelMatrix = modelMatrix.Mul(math.RotateZ(model.rotation[2] * gomath.Pi / 180))

		// Apply per-model scale multiplied by global ModelScale
		modelMatrix = modelMatrix.Mul(math.Scale(
			model.scale[0]*mv.ModelScale,
			model.scale[1]*mv.ModelScale,
			model.scale[2]*mv.ModelScale,
		))

		// Combine with view-projection
		mvp := viewProj.Mul(modelMatrix)
		gl.UniformMatrix4fv(mv.locModelMVP, 1, false, &mvp[0])
		gl.UniformMatrix4fv(mv.locModelModel, 1, false, &modelMatrix[0])

		gl.BindVertexArray(model.vao)

		// Render each texture group
		for _, group := range model.texGroups {
			tex := mv.fallbackTex
			if group.TextureIdx >= 0 && group.TextureIdx < len(model.textures) {
				tex = model.textures[group.TextureIdx]
			}
			gl.BindTexture(gl.TEXTURE_2D, tex)
			gl.DrawElementsWithOffset(gl.TRIANGLES, group.IndexCount, gl.UNSIGNED_INT, uintptr(group.StartIndex*4))
		}
	}

	gl.BindVertexArray(0)
}

// HandleMouseDrag handles mouse drag for camera rotation.
func (mv *MapViewer) HandleMouseDrag(deltaX, deltaY float32) {
	if mv.PlayMode {
		// Play mode - rotate camera around player (horizontal only)
		mv.FollowCam.HandleYaw(deltaX)
	} else {
		// Orbit mode - rotate around center
		mv.OrbitCam.HandleDrag(deltaX, deltaY)
	}
}

// HandleMouseWheel handles mouse scroll for zoom.
func (mv *MapViewer) HandleMouseWheel(delta float32) {
	if mv.PlayMode {
		mv.FollowCam.HandleZoom(delta)
	} else {
		mv.OrbitCam.HandleZoom(delta)
	}
}

// HandlePlayMovement handles WASD movement in Play mode.
// forward/right are -1, 0, or 1 based on key presses.
func (mv *MapViewer) HandlePlayMovement(forward, right, _ float32) {
	if !mv.PlayMode || mv.Player == nil {
		return
	}

	// Check if any movement input
	if forward == 0 && right == 0 {
		mv.Player.IsMoving = false
		return
	}

	mv.Player.IsMoving = true

	// Calculate movement direction based on camera yaw
	// Forward is toward camera direction, right is perpendicular
	camDirX, camDirZ := mv.FollowCam.ForwardDirection()
	camRightX, camRightZ := mv.FollowCam.RightDirection()

	// Combined movement direction
	moveX := camDirX*forward + camRightX*right
	moveZ := camDirZ*forward + camRightZ*right

	// Normalize if diagonal
	length := float32(gomath.Sqrt(float64(moveX*moveX + moveZ*moveZ)))
	if length > 0 {
		moveX /= length
		moveZ /= length
	}

	// Calculate new position
	speed := mv.Player.MoveSpeed * 0.016 // ~60fps delta
	newX := mv.Player.WorldX + moveX*speed
	newZ := mv.Player.WorldZ + moveZ*speed

	// Check if new position is walkable
	if mv.IsWalkable(newX, newZ) {
		mv.Player.WorldX = newX
		mv.Player.WorldZ = newZ
		// Update Y to follow terrain
		mv.Player.WorldY = mv.GetInterpolatedTerrainHeight(newX, newZ)
	}

	// Calculate 8-direction facing from movement (negate to face movement direction)
	mv.Player.Direction = entity.CalculateDirection(-moveX, -moveZ)

	// Set walk animation
	mv.Player.CurrentAction = entity.ActionWalk
}

// UpdatePlayerMovement updates player position for click-to-move navigation.
// Called each frame to move player toward destination.
func (mv *MapViewer) UpdatePlayerMovement(deltaMs float32) {
	if !mv.PlayMode {
		return
	}
	character.UpdateMovement(mv.Player, deltaMs, mv)
}

// HandlePlayModeClick handles mouse click in Play mode for click-to-move.
// screenX, screenY are mouse coordinates, viewportW, viewportH are viewport dimensions.
func (mv *MapViewer) HandlePlayModeClick(screenX, screenY, viewportW, viewportH float32) {
	if !mv.PlayMode || mv.Player == nil {
		return
	}

	// Convert screen coordinates to world position using terrain intersection
	worldX, worldZ, ok := mv.ScreenToWorld(screenX, screenY, viewportW, viewportH)
	if !ok {
		return
	}

	// Set destination using character package
	character.SetDestination(mv.Player, worldX, worldZ)
}

// ScreenToWorld converts screen coordinates to world XZ position by intersecting with ground plane.
func (mv *MapViewer) ScreenToWorld(screenX, screenY, viewportW, viewportH float32) (worldX, worldZ float32, ok bool) {
	// Create ray from screen coordinates
	ray := picking.ScreenToRay(screenX, screenY, viewportW, viewportH, mv.lastViewProj.Inverse())

	// Intersect with ground plane (Y = player height or terrain)
	groundY := float32(0)
	if mv.Player != nil {
		groundY = mv.Player.WorldY
	}

	worldX, worldZ, ok = ray.IntersectPlaneY(groundY)
	if ok {
		fmt.Printf("Click: screen(%.0f,%.0f) -> world(%.1f, %.1f)\n", screenX, screenY, worldX, worldZ)
	}
	return worldX, worldZ, ok
}

// HandleOrbitMovement handles WASD movement in Orbit mode.
// Pans the camera's focal point (center).
// forward/right are -1, 0, or 1 based on key presses.
func (mv *MapViewer) HandleOrbitMovement(forward, right, up float32) {
	if mv.PlayMode {
		return
	}
	mv.OrbitCam.HandleMovement(forward, right, up)
}

// LoadPlayerCharacter loads the Novice sprite for Play Mode.
func (mv *MapViewer) LoadPlayerCharacter(texLoader func(string) ([]byte, error)) error {
	if mv.Player != nil {
		return nil // Already loaded
	}

	// Try multiple sprite paths (different GRF versions have different paths)
	// Note: In RO, body and head are separate sprites that can be customized
	// For simplicity, we use complete character sprites like b_novice or monsters
	spritePaths := []struct {
		spr string
		act string
	}{
		// Baby Novice (complete sprite without separate head)
		{"data/sprite/몬스터/b_novice.spr", "data/sprite/몬스터/b_novice.act"},
		// Korean Novice male body (would need head separately)
		{"data/sprite/인간족/몸통/남/초보자_남.spr", "data/sprite/인간족/몸통/남/초보자_남.act"},
		// English paths
		{"data/sprite/human/body/male/novice_m.spr", "data/sprite/human/body/male/novice_m.act"},
		// Poring as fallback (should exist in most GRFs)
		{"data/sprite/몬스터/poring.spr", "data/sprite/몬스터/poring.act"},
		{"data/sprite/monster/poring.spr", "data/sprite/monster/poring.act"},
	}

	var sprData, actData []byte
	var sprPath, actPath string
	var err error

	// Try each path until one works
	for _, paths := range spritePaths {
		sprData, err = texLoader(paths.spr)
		if err != nil {
			continue
		}
		actData, err = texLoader(paths.act)
		if err != nil {
			continue
		}
		sprPath = paths.spr
		actPath = paths.act
		break
	}

	if sprData == nil || actData == nil {
		// Create a simple colored marker as fallback
		fmt.Println("No sprite found, creating procedural player marker")
		return mv.createProceduralPlayer()
	}

	fmt.Printf("Using sprite: %s\n", sprPath)

	// Parse SPR file
	spr, err := formats.ParseSPR(sprData)
	if err != nil {
		return fmt.Errorf("parsing player sprite %s: %w", sprPath, err)
	}

	// Parse ACT file
	act, err := formats.ParseACT(actData)
	if err != nil {
		return fmt.Errorf("parsing player animation %s: %w", actPath, err)
	}

	// Create player character
	char := entity.NewCharacter(0, 0, 0)
	char.MoveSpeed = 50.0 // World units per second
	player := &PlayerCharacter{
		Character:   char,
		SPR:         spr,
		ACT:         act,
		SpriteScale: DefaultSpriteScale,
	}

	// Create GPU textures for each sprite image
	player.Textures = make([]uint32, len(spr.Images))
	for i, img := range spr.Images {
		var tex uint32
		gl.GenTextures(1, &tex)
		gl.BindTexture(gl.TEXTURE_2D, tex)

		gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA8,
			int32(img.Width), int32(img.Height), 0,
			gl.RGBA, gl.UNSIGNED_BYTE, gl.Ptr(img.Pixels))

		gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.NEAREST)
		gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.NEAREST)
		gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
		gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)

		player.Textures[i] = tex
	}

	// Create billboard VAO/VBO
	gl.GenVertexArrays(1, &player.VAO)
	gl.GenBuffers(1, &player.VBO)

	gl.BindVertexArray(player.VAO)
	gl.BindBuffer(gl.ARRAY_BUFFER, player.VBO)

	// Billboard quad vertices: position (x,y) + texcoord (u,v)
	// Positioned at feet, extending upward
	vertices := []float32{
		// Position (x, y)  TexCoord (u, v)
		-0.5, 1.0, 0.0, 0.0, // Top-left (sprite top at Y=1)
		0.5, 1.0, 1.0, 0.0, // Top-right
		-0.5, 0.0, 0.0, 1.0, // Bottom-left (sprite bottom at Y=0)
		0.5, 0.0, 1.0, 1.0, // Bottom-right
	}

	gl.BufferData(gl.ARRAY_BUFFER, len(vertices)*4, gl.Ptr(vertices), gl.STATIC_DRAW)

	// Position attribute (location 0)
	gl.VertexAttribPointerWithOffset(0, 2, gl.FLOAT, false, 4*4, 0)
	gl.EnableVertexAttribArray(0)

	// TexCoord attribute (location 1)
	gl.VertexAttribPointerWithOffset(1, 2, gl.FLOAT, false, 4*4, 2*4)
	gl.EnableVertexAttribArray(1)

	gl.BindVertexArray(0)

	// Create shadow
	mv.createPlayerShadow(player)

	mv.Player = player

	// Initialize player position to map center
	mv.initializePlayerPosition()

	fmt.Printf("Loaded player sprite: %d images, %d actions\n", len(spr.Images), len(act.Actions))

	return nil
}

// LoadPlayerCharacterFromPath loads a player sprite from specific paths.
// Used when the sprite path is found by searching the archive.
func (mv *MapViewer) LoadPlayerCharacterFromPath(texLoader func(string) ([]byte, error), sprPath, actPath, headSprPath, headActPath string) error {
	if mv.Player != nil {
		return nil // Already loaded
	}

	fmt.Printf("Loading sprite from: %s\n", sprPath)

	sprData, err := texLoader(sprPath)
	if err != nil {
		return fmt.Errorf("loading sprite %s: %w", sprPath, err)
	}

	actData, err := texLoader(actPath)
	if err != nil {
		return fmt.Errorf("loading animation %s: %w", actPath, err)
	}

	// Parse SPR file
	spr, err := formats.ParseSPR(sprData)
	if err != nil {
		return fmt.Errorf("parsing sprite %s: %w", sprPath, err)
	}

	// Parse ACT file
	act, err := formats.ParseACT(actData)
	if err != nil {
		return fmt.Errorf("parsing animation %s: %w", actPath, err)
	}

	// Create player character
	char := entity.NewCharacter(0, 0, 0)
	char.MoveSpeed = 50.0 // World units per second
	player := &PlayerCharacter{
		Character:   char,
		SPR:         spr,
		ACT:         act,
		SpriteScale: DefaultSpriteScale,
	}

	// Load head sprite if path provided
	if headSprPath != "" {
		fmt.Printf("Loading head sprite from: %s\n", headSprPath)
		headSprData, err := texLoader(headSprPath)
		if err != nil {
			fmt.Printf("Warning: could not load head sprite: %v\n", err)
		} else {
			headActData, err := texLoader(headActPath)
			if err != nil {
				fmt.Printf("Warning: could not load head animation: %v\n", err)
			} else {
				headSpr, err := formats.ParseSPR(headSprData)
				if err != nil {
					fmt.Printf("Warning: could not parse head sprite: %v\n", err)
				} else {
					headAct, err := formats.ParseACT(headActData)
					if err != nil {
						fmt.Printf("Warning: could not parse head animation: %v\n", err)
					} else {
						player.HeadSPR = headSpr
						player.HeadACT = headAct
						fmt.Printf("Loaded head sprite: %d images, %d actions\n", len(headSpr.Images), len(headAct.Actions))
						// Debug: check anchor points and layer positions
						if len(headAct.Actions) > 0 && len(headAct.Actions[0].Frames) > 0 {
							frame := &headAct.Actions[0].Frames[0]
							fmt.Printf("  Head action 0 frame 0: %d layers, %d anchors\n", len(frame.Layers), len(frame.AnchorPoints))
							for i, ap := range frame.AnchorPoints {
								fmt.Printf("    Anchor %d: X=%d Y=%d Attr=%d\n", i, ap.X, ap.Y, ap.Attribute)
							}
							for i, layer := range frame.Layers {
								fmt.Printf("    Layer %d: SpriteID=%d X=%d Y=%d\n", i, layer.SpriteID, layer.X, layer.Y)
							}
						}
					}
				}
			}
		}
	}

	// Create GPU textures for each sprite image
	player.Textures = make([]uint32, len(spr.Images))
	for i, img := range spr.Images {
		var tex uint32
		gl.GenTextures(1, &tex)
		gl.BindTexture(gl.TEXTURE_2D, tex)

		gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA8,
			int32(img.Width), int32(img.Height), 0,
			gl.RGBA, gl.UNSIGNED_BYTE, gl.Ptr(img.Pixels))

		gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.NEAREST)
		gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.NEAREST)
		gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
		gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)

		player.Textures[i] = tex
	}

	// Create GPU textures for head sprite images
	if player.HeadSPR != nil {
		player.HeadTextures = make([]uint32, len(player.HeadSPR.Images))
		for i, img := range player.HeadSPR.Images {
			var tex uint32
			gl.GenTextures(1, &tex)
			gl.BindTexture(gl.TEXTURE_2D, tex)

			gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA8,
				int32(img.Width), int32(img.Height), 0,
				gl.RGBA, gl.UNSIGNED_BYTE, gl.Ptr(img.Pixels))

			gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.NEAREST)
			gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.NEAREST)
			gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
			gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)

			player.HeadTextures[i] = tex
		}

		// Generate composite textures (head+body merged) for each action/direction/frame
		// This creates proper head-body alignment using anchor points
		fmt.Println("Generating composite sprites...")

		// Debug: print body and head anchors for each direction
		fmt.Println("Body anchors per direction (action 0):")
		for dir := 0; dir < 8 && dir < len(act.Actions); dir++ {
			ba := &act.Actions[dir]
			if len(ba.Frames) > 0 {
				bf := &ba.Frames[0]
				if len(bf.AnchorPoints) > 0 {
					fmt.Printf("  Dir %d: body anchor(%d,%d)\n", dir, bf.AnchorPoints[0].X, bf.AnchorPoints[0].Y)
				}
			}
		}
		fmt.Println("Head anchors per direction:")
		for dir := 0; dir < 8 && dir < len(player.HeadACT.Actions); dir++ {
			ha := &player.HeadACT.Actions[dir]
			if len(ha.Frames) > 0 {
				hf := &ha.Frames[0]
				if len(hf.AnchorPoints) > 0 {
					fmt.Printf("  Dir %d: head anchor(%d,%d)\n", dir, hf.AnchorPoints[0].X, hf.AnchorPoints[0].Y)
				}
			}
		}

		player.CompositeFrames = make(map[int][]CompositeFrame)
		player.CompositeMaxWidth = 0
		player.CompositeMaxHeight = 0

		// First pass: find max dimensions across all composites
		for action := 0; action < 2; action++ {
			for dir := 0; dir < 8; dir++ {
				actionIdx := action*8 + dir
				if actionIdx >= len(act.Actions) {
					continue
				}
				actAction := &act.Actions[actionIdx]
				for frame := 0; frame < len(actAction.Frames); frame++ {
					result := sprite.CompositeSprites(spr, act, player.HeadSPR, player.HeadACT, action, dir, frame)
					if result.Width > player.CompositeMaxWidth {
						player.CompositeMaxWidth = result.Width
					}
					if result.Height > player.CompositeMaxHeight {
						player.CompositeMaxHeight = result.Height
					}
				}
			}
		}
		fmt.Printf("Composite max dimensions: %dx%d\n", player.CompositeMaxWidth, player.CompositeMaxHeight)

		// Second pass: generate composites padded to max dimensions
		for action := 0; action < 2; action++ {
			for dir := 0; dir < 8; dir++ {
				actionDirKey := action*8 + dir
				actionIdx := action*8 + dir
				if actionIdx >= len(act.Actions) {
					continue
				}
				actAction := &act.Actions[actionIdx]
				numFrames := len(actAction.Frames)
				if numFrames == 0 {
					continue
				}

				frames := make([]CompositeFrame, numFrames)
				for frame := 0; frame < numFrames; frame++ {
					result := sprite.CompositeSprites(spr, act, player.HeadSPR, player.HeadACT, action, dir, frame)
					if result.Pixels == nil || result.Width == 0 || result.Height == 0 {
						continue
					}

					// Pad to max dimensions (center horizontally, align bottom for feet)
					paddedW := player.CompositeMaxWidth
					paddedH := player.CompositeMaxHeight
					paddedPixels := make([]byte, paddedW*paddedH*4)

					// Calculate offset to center horizontally and align feet at bottom
					offsetX := (paddedW - result.Width) / 2
					offsetY := paddedH - result.Height // Align bottom (feet)

					// Copy original pixels to padded canvas
					for py := 0; py < result.Height; py++ {
						for px := 0; px < result.Width; px++ {
							srcIdx := (py*result.Width + px) * 4
							dstX := offsetX + px
							dstY := offsetY + py
							dstIdx := (dstY*paddedW + dstX) * 4
							paddedPixels[dstIdx] = result.Pixels[srcIdx]
							paddedPixels[dstIdx+1] = result.Pixels[srcIdx+1]
							paddedPixels[dstIdx+2] = result.Pixels[srcIdx+2]
							paddedPixels[dstIdx+3] = result.Pixels[srcIdx+3]
						}
					}

					// Create GPU texture for padded composite
					var tex uint32
					gl.GenTextures(1, &tex)
					gl.BindTexture(gl.TEXTURE_2D, tex)
					gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA8, int32(paddedW), int32(paddedH), 0,
						gl.RGBA, gl.UNSIGNED_BYTE, gl.Ptr(paddedPixels))
					gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.NEAREST)
					gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.NEAREST)
					gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
					gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)

					frames[frame] = CompositeFrame{
						Texture: tex,
						Width:   paddedW,
						Height:  paddedH,
						OriginX: offsetX,
						OriginY: offsetY,
					}
				}
				player.CompositeFrames[actionDirKey] = frames
			}
		}
		player.UseComposite = true
		fmt.Printf("Generated %d composite frame sets\n", len(player.CompositeFrames))

		// Save all directions to a single sprite sheet for debugging
		saveAllDirectionsSheet(spr, act, player.HeadSPR, player.HeadACT, "/tmp/all_directions.png")
	}

	// Create billboard VAO/VBO
	gl.GenVertexArrays(1, &player.VAO)
	gl.GenBuffers(1, &player.VBO)

	gl.BindVertexArray(player.VAO)
	gl.BindBuffer(gl.ARRAY_BUFFER, player.VBO)

	vertices := []float32{
		-0.5, 1.0, 0.0, 0.0,
		0.5, 1.0, 1.0, 0.0,
		-0.5, 0.0, 0.0, 1.0,
		0.5, 0.0, 1.0, 1.0,
	}

	gl.BufferData(gl.ARRAY_BUFFER, len(vertices)*4, gl.Ptr(vertices), gl.STATIC_DRAW)
	gl.VertexAttribPointerWithOffset(0, 2, gl.FLOAT, false, 4*4, 0)
	gl.EnableVertexAttribArray(0)
	gl.VertexAttribPointerWithOffset(1, 2, gl.FLOAT, false, 4*4, 2*4)
	gl.EnableVertexAttribArray(1)
	gl.BindVertexArray(0)

	// Create shadow
	mv.createPlayerShadow(player)

	mv.Player = player
	mv.initializePlayerPosition()

	fmt.Printf("Loaded player sprite: %d images, %d actions\n", len(spr.Images), len(act.Actions))
	// Debug: check body anchor points and layer positions
	if len(act.Actions) > 0 && len(act.Actions[0].Frames) > 0 {
		frame := &act.Actions[0].Frames[0]
		fmt.Printf("  Body action 0 frame 0: %d layers, %d anchors\n", len(frame.Layers), len(frame.AnchorPoints))
		for i, ap := range frame.AnchorPoints {
			fmt.Printf("    Anchor %d: X=%d Y=%d Attr=%d\n", i, ap.X, ap.Y, ap.Attribute)
		}
		for i, layer := range frame.Layers {
			fmt.Printf("    Layer %d: SpriteID=%d X=%d Y=%d\n", i, layer.SpriteID, layer.X, layer.Y)
		}
	}
	return nil
}

// createPlayerShadow creates a shadow ellipse texture and VAO for the player.
func (mv *MapViewer) createPlayerShadow(player *PlayerCharacter) {
	// Generate circular shadow texture pixels
	size := sprite.DefaultShadowSize
	pixels := sprite.GenerateCircularShadow(size, sprite.DefaultShadowOpacity)

	// Create shadow texture
	gl.GenTextures(1, &player.ShadowTex)
	gl.BindTexture(gl.TEXTURE_2D, player.ShadowTex)
	gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA8, int32(size), int32(size), 0,
		gl.RGBA, gl.UNSIGNED_BYTE, gl.Ptr(pixels))
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)

	// Create shadow VAO/VBO (flat on ground)
	gl.GenVertexArrays(1, &player.ShadowVAO)
	gl.GenBuffers(1, &player.ShadowVBO)

	gl.BindVertexArray(player.ShadowVAO)
	gl.BindBuffer(gl.ARRAY_BUFFER, player.ShadowVBO)

	// Shadow quad on XZ plane (Y=0), centered at origin
	shadowVerts := sprite.GenerateShadowQuadVertices(sprite.DefaultShadowWorldSize)

	gl.BufferData(gl.ARRAY_BUFFER, len(shadowVerts)*4, gl.Ptr(shadowVerts), gl.STATIC_DRAW)
	gl.VertexAttribPointerWithOffset(0, 2, gl.FLOAT, false, 4*4, 0)
	gl.EnableVertexAttribArray(0)
	gl.VertexAttribPointerWithOffset(1, 2, gl.FLOAT, false, 4*4, 2*4)
	gl.EnableVertexAttribArray(1)
	gl.BindVertexArray(0)
}

// createProceduralPlayer creates a simple colored player marker when no sprite is available.
func (mv *MapViewer) createProceduralPlayer() error {
	// Generate procedural player texture
	width, height := sprite.DefaultProceduralWidth, sprite.DefaultProceduralHeight
	pixels := sprite.GenerateProceduralPlayer(width, height)

	// Create player character
	char := entity.NewCharacter(0, 0, 0)
	char.MoveSpeed = 50.0 // World units per second
	player := &PlayerCharacter{
		Character:   char,
		SpriteScale: sprite.DefaultProceduralScale,
	}

	// Create single texture
	player.Textures = make([]uint32, 1)
	var tex uint32
	gl.GenTextures(1, &tex)
	gl.BindTexture(gl.TEXTURE_2D, tex)
	gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA8, int32(width), int32(height), 0,
		gl.RGBA, gl.UNSIGNED_BYTE, gl.Ptr(pixels))
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.NEAREST)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.NEAREST)
	player.Textures[0] = tex

	// Create billboard VAO/VBO
	gl.GenVertexArrays(1, &player.VAO)
	gl.GenBuffers(1, &player.VBO)

	gl.BindVertexArray(player.VAO)
	gl.BindBuffer(gl.ARRAY_BUFFER, player.VBO)

	vertices := sprite.GenerateBillboardQuadVertices()

	gl.BufferData(gl.ARRAY_BUFFER, len(vertices)*4, gl.Ptr(vertices), gl.STATIC_DRAW)
	gl.VertexAttribPointerWithOffset(0, 2, gl.FLOAT, false, 4*4, 0)
	gl.EnableVertexAttribArray(0)
	gl.VertexAttribPointerWithOffset(1, 2, gl.FLOAT, false, 4*4, 2*4)
	gl.EnableVertexAttribArray(1)
	gl.BindVertexArray(0)

	mv.Player = player

	// Create shadow for the player
	mv.createPlayerShadow(player)

	// Initialize player position to map center
	mv.initializePlayerPosition()

	fmt.Println("Created procedural player marker")
	return nil
}

// initializePlayerPosition places the player at the map center.
func (mv *MapViewer) initializePlayerPosition() {
	if mv.Player == nil {
		return
	}

	// Calculate map center from bounds
	centerX := (mv.minBounds[0] + mv.maxBounds[0]) / 2
	centerZ := (mv.minBounds[2] + mv.maxBounds[2]) / 2

	// If bounds aren't set, use center from map dimensions
	if centerX == 0 && centerZ == 0 && mv.mapWidth > 0 {
		centerX = mv.mapWidth / 2
		centerZ = mv.mapHeight / 2
	}

	// Set player position (both world and render to prevent lerp on spawn)
	mv.Player.WorldX = centerX
	mv.Player.WorldZ = centerZ
	mv.Player.WorldY = mv.GetInterpolatedTerrainHeight(centerX, centerZ)
	// Sync render position to avoid sprite interpolating from origin
	mv.Player.RenderX = mv.Player.WorldX
	mv.Player.RenderY = mv.Player.WorldY
	mv.Player.RenderZ = mv.Player.WorldZ

	fmt.Printf("Player spawned at (%.0f, %.0f, %.0f)\n", mv.Player.WorldX, mv.Player.WorldY, mv.Player.WorldZ)
}

// GetInterpolatedTerrainHeight returns the terrain height at a world position.
// Delegates to terrain package for bilinear interpolation between GAT cell heights.
func (mv *MapViewer) GetInterpolatedTerrainHeight(worldX, worldZ float32) float32 {
	return terrain.GetInterpolatedHeight(mv.GAT, worldX, worldZ)
}

// GetHeight implements character.TerrainQuery interface.
// Returns the terrain height at a world position.
func (mv *MapViewer) GetHeight(worldX, worldZ float32) float32 {
	return terrain.GetInterpolatedHeight(mv.GAT, worldX, worldZ)
}

// IsWalkable checks if a world position is walkable.
// Delegates to terrain package for GAT-based walkability check.
// Also implements character.TerrainQuery interface.
func (mv *MapViewer) IsWalkable(worldX, worldZ float32) bool {
	return terrain.IsWalkable(mv.GAT, worldX, worldZ)
}

// TogglePlayMode toggles between orbit and play camera modes.
func (mv *MapViewer) TogglePlayMode() {
	mv.PlayMode = !mv.PlayMode

	if mv.PlayMode {
		// Set appropriate zoom distance for Play mode (RO-style)
		mv.FollowCam.Distance = 145 // Good starting distance for third-person

		// Reset camera yaw (rotation around player)
		mv.FollowCam.Yaw = 0

		// Player position should already be initialized by LoadPlayerCharacter
		// If not, initialize now
		if mv.Player != nil && mv.Player.WorldX == 0 && mv.Player.WorldZ == 0 {
			mv.initializePlayerPosition()
		}
	}
}

// Reset resets camera to default position.
func (mv *MapViewer) Reset() {
	if mv.PlayMode {
		// Reset play camera
		mv.FollowCam.Yaw = 0
		mv.FollowCam.Distance = 145 // Default Play mode distance

		// Reset player to map center
		if mv.Player != nil {
			mv.initializePlayerPosition()
		}
	} else {
		mv.OrbitCam.RotationX = 0.6
		mv.OrbitCam.RotationY = 0.0
	}
}

// GetLightDir returns the current light direction vector (from RSW data).
func (mv *MapViewer) GetLightDir() [3]float32 {
	return mv.lightDir
}

// GetWaterAnimSpeed returns the current water animation speed.
func (mv *MapViewer) GetWaterAnimSpeed() float32 {
	return mv.waterAnimSpeed
}

// SetWaterAnimSpeed sets the water animation speed.
func (mv *MapViewer) SetWaterAnimSpeed(speed float32) {
	mv.waterAnimSpeed = speed
}

// HasWater returns whether the map has water.
func (mv *MapViewer) HasWater() bool {
	return mv.hasWater
}

// --- Model Animation Functions ---

// UpdateModelAnimation advances model animation time and rebuilds animated models.
// Returns true if any models were updated.
func (mv *MapViewer) UpdateModelAnimation(deltaMs float32) bool {
	if !mv.modelAnimPlaying || len(mv.animatedModels) == 0 {
		return false
	}

	mv.modelAnimTime += deltaMs

	// Rebuild all animated models with new time
	for _, model := range mv.animatedModels {
		if model.rsm != nil && model.Visible {
			mv.rebuildAnimatedModel(model, mv.modelAnimTime)
		}
	}

	return true
}

// rebuildAnimatedModel rebuilds a single model's mesh at the given animation time.
func (mv *MapViewer) rebuildAnimatedModel(model *MapModel, animTimeMs float32) {
	if model.rsm == nil || model.rswRef == nil {
		return
	}

	// Loop animation time based on model's animation length
	loopedTime := animTimeMs
	if model.animLength > 0 {
		loopedTime = float32(int(animTimeMs) % int(model.animLength))
	}

	// Build vertices and indices at current animation time
	vertices, indices, groups := mv.buildAnimatedModelMesh(model.rsm, model.rswRef, loopedTime)
	if len(vertices) == 0 {
		return
	}

	// Update GPU buffers
	gl.BindVertexArray(model.vao)

	gl.BindBuffer(gl.ARRAY_BUFFER, model.vbo)
	gl.BufferData(gl.ARRAY_BUFFER, len(vertices)*int(unsafe.Sizeof(rsmmodel.Vertex{})), gl.Ptr(vertices), gl.DYNAMIC_DRAW)

	gl.BindBuffer(gl.ELEMENT_ARRAY_BUFFER, model.ebo)
	gl.BufferData(gl.ELEMENT_ARRAY_BUFFER, len(indices)*4, gl.Ptr(indices), gl.DYNAMIC_DRAW)

	model.indexCount = int32(len(indices))
	model.texGroups = groups

	gl.BindVertexArray(0)
}

// PlayModelAnimation starts model animations.
func (mv *MapViewer) PlayModelAnimation() {
	mv.modelAnimPlaying = true
}

// PauseModelAnimation pauses model animations.
func (mv *MapViewer) PauseModelAnimation() {
	mv.modelAnimPlaying = false
}

// ToggleModelAnimation toggles model animation playback.
func (mv *MapViewer) ToggleModelAnimation() {
	mv.modelAnimPlaying = !mv.modelAnimPlaying
}

// IsModelAnimationPlaying returns whether model animations are playing.
func (mv *MapViewer) IsModelAnimationPlaying() bool {
	return mv.modelAnimPlaying
}

// GetAnimatedModelCount returns the number of animated models.
func (mv *MapViewer) GetAnimatedModelCount() int {
	return len(mv.animatedModels)
}

// GetModelCount returns the number of loaded models.
func (mv *MapViewer) GetModelCount() int {
	return len(mv.models)
}

// GetModelInfo returns debug info for a specific model by index.
func (mv *MapViewer) GetModelInfo(idx int) (name string, pos, rot, scale [3]float32, bbox [6]float32) {
	if idx < 0 || idx >= len(mv.models) {
		return "", [3]float32{}, [3]float32{}, [3]float32{}, [6]float32{}
	}
	m := mv.models[idx]
	return m.modelName, m.position, m.rotation, m.scale, m.bbox
}

// SetDebugMode enables/disables debug output for model positioning.
func SetDebugMode(enabled bool) {
	DebugModelPositioning = enabled
}

// GetAmbientColor returns the current ambient light color (from RSW data).
func (mv *MapViewer) GetAmbientColor() [3]float32 {
	return mv.ambientColor
}

// GetDiffuseColor returns the current diffuse light color (from RSW data).
func (mv *MapViewer) GetDiffuseColor() [3]float32 {
	return mv.diffuseColor
}

// Destroy frees all GPU resources.
func (mv *MapViewer) Destroy() {
	mv.clearTerrain()

	if mv.fallbackTex != 0 {
		gl.DeleteTextures(1, &mv.fallbackTex)
	}
	if mv.terrainProgram != 0 {
		gl.DeleteProgram(mv.terrainProgram)
	}
	if mv.modelProgram != 0 {
		gl.DeleteProgram(mv.modelProgram)
	}
	if mv.fbo != 0 {
		gl.DeleteFramebuffers(1, &mv.fbo)
	}
	if mv.colorTexture != 0 {
		gl.DeleteTextures(1, &mv.colorTexture)
	}
	if mv.depthRBO != 0 {
		gl.DeleteRenderbuffers(1, &mv.depthRBO)
	}
}

// createWaterPlane creates a water surface plane at the specified height.
func (mv *MapViewer) createWaterPlane(_ *formats.GND, waterLevel float32) {
	// Delete old water if exists
	if mv.waterVAO != 0 {
		gl.DeleteVertexArrays(1, &mv.waterVAO)
		gl.DeleteBuffers(1, &mv.waterVBO)
	}

	// Build water plane geometry using water package
	plane := water.BuildPlaneWithPadding(
		mv.minBounds[0], mv.maxBounds[0],
		mv.minBounds[2], mv.maxBounds[2],
		waterLevel, water.DefaultPadding,
	)

	// Create VAO/VBO
	gl.GenVertexArrays(1, &mv.waterVAO)
	gl.GenBuffers(1, &mv.waterVBO)

	gl.BindVertexArray(mv.waterVAO)
	gl.BindBuffer(gl.ARRAY_BUFFER, mv.waterVBO)
	gl.BufferData(gl.ARRAY_BUFFER, len(plane.Vertices)*4, gl.Ptr(plane.Vertices), gl.STATIC_DRAW)

	// Position attribute
	gl.EnableVertexAttribArray(0)
	gl.VertexAttribPointerWithOffset(0, 3, gl.FLOAT, false, 3*4, 0)

	gl.BindVertexArray(0)

	mv.waterLevel = waterLevel
	mv.hasWater = true
}

// PrintDiagnostics outputs map loading diagnostics to console.
func (mv *MapViewer) PrintDiagnostics() {
	d := mv.Diagnostics
	fmt.Println("\n=== Map Loading Diagnostics ===")
	fmt.Println("Models:")
	fmt.Printf("  Total in RSW:    %d\n", d.TotalModelsInRSW)
	fmt.Printf("  Skipped (limit): %d\n", d.ModelsSkippedLimit)
	fmt.Printf("  Load failed:     %d\n", d.ModelsLoadFailed)
	fmt.Printf("  Parse error:     %d\n", d.ModelsParseError)
	fmt.Printf("  No nodes:        %d\n", d.ModelsNoNodes)
	fmt.Printf("  Loaded OK:       %d\n", d.ModelsLoaded)
	fmt.Printf("  Unique RSM files:%d\n", d.UniqueRSMFiles)

	fmt.Println("\nGeometry:")
	fmt.Printf("  Total nodes:     %d\n", d.TotalNodes)
	fmt.Printf("  Total faces:     %d\n", d.TotalFaces)
	fmt.Printf("  Two-sided faces: %d (%.1f%%)\n", d.TwoSidedFaces, float64(d.TwoSidedFaces)*100/float64(max(d.TotalFaces, 1)))
	fmt.Printf("  Total vertices:  %d\n", d.TotalVertices)

	fmt.Println("\nTextures:")
	fmt.Printf("  Loaded:          %d\n", d.TexturesLoaded)
	fmt.Printf("  Missing:         %d\n", d.TexturesMissing)

	if len(d.MissingTextures) > 0 {
		fmt.Println("\nMissing textures (first 10):")
		for i, tex := range d.MissingTextures {
			if i >= 10 {
				fmt.Printf("  ... and %d more\n", len(d.MissingTextures)-10)
				break
			}
			fmt.Printf("  - %s\n", tex)
		}
	}

	if len(d.FailedModels) > 0 {
		fmt.Println("\nFailed models (first 10):")
		for i, model := range d.FailedModels {
			if i >= 10 {
				fmt.Printf("  ... and %d more\n", len(d.FailedModels)-10)
				break
			}
			fmt.Printf("  - %s\n", model)
		}
	}

	fmt.Println("\nLighting:")
	fmt.Printf("  Light Dir:       (%.2f, %.2f, %.2f)\n", mv.lightDir[0], mv.lightDir[1], mv.lightDir[2])
	fmt.Printf("  Ambient:         (%.2f, %.2f, %.2f)\n", mv.ambientColor[0], mv.ambientColor[1], mv.ambientColor[2])
	fmt.Printf("  Diffuse:         (%.2f, %.2f, %.2f)\n", mv.diffuseColor[0], mv.diffuseColor[1], mv.diffuseColor[2])
	fmt.Printf("  Light Opacity:   %.2f\n", mv.lightOpacity)
	fmt.Printf("  Brightness:      %.2f\n", mv.Brightness)

	fmt.Println("================================")
}
