// 3D map viewer for GND/RSW files (ADR-013 Stage 1).
package main

import (
	"fmt"
	gomath "math"
	"sort"
	"unsafe"

	"github.com/go-gl/gl/v4.1-core/gl"

	"github.com/Faultbox/midgard-ro/pkg/formats"
	"github.com/Faultbox/midgard-ro/pkg/math"
)

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
// NodeDebugInfo stores debug information about an RSM node.
type NodeDebugInfo struct {
	Name         string
	Parent       string
	Offset       [3]float32
	Position     [3]float32
	Scale        [3]float32
	RotAngle     float32
	RotAxis      [3]float32
	Matrix       [9]float32
	HasRotKeys   bool
	HasPosKeys   bool
	HasScaleKeys bool
	// First rotation keyframe (if any)
	FirstRotQuat [4]float32
	RotKeyCount  int
}

// MapModel represents a placed RSM model in the map.
type MapModel struct {
	vao        uint32
	vbo        uint32
	ebo        uint32
	indexCount int32
	textures   []uint32
	texGroups  []modelTexGroup
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
	nodes      []NodeDebugInfo
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

// modelTexGroup groups faces by texture for rendering.
type modelTexGroup struct {
	texIdx     int
	startIndex int32
	indexCount int32
}

// Direction constants for 8-directional sprites (RO standard order)
const (
	DirS  = 0 // South (facing camera)
	DirSW = 1
	DirW  = 2
	DirNW = 3
	DirN  = 4 // North (away from camera)
	DirNE = 5
	DirE  = 6
	DirSE = 7
)

// Action type constants for character animations
const (
	ActionIdle = 0
	ActionWalk = 1
)

// calculateDirection returns 0-7 direction index from movement vector.
// RO directions: 0=S, 1=SW, 2=W, 3=NW, 4=N, 5=NE, 6=E, 7=SE
func calculateDirection(dx, dz float32) int {
	// Calculate angle in radians (atan2 gives -PI to PI)
	angle := gomath.Atan2(float64(dx), float64(dz))

	// Convert to 0-2*PI range
	if angle < 0 {
		angle += 2 * gomath.Pi
	}

	// Divide circle into 8 sectors (each 45 degrees = PI/4)
	// Add PI/8 offset to center each sector
	sector := int((angle + gomath.Pi/8) / (gomath.Pi / 4))
	if sector >= 8 {
		sector = 0
	}

	// Map sectors to RO direction order
	// angle=0 is +Z direction (South in RO terms = facing camera)
	// Clockwise: S(0), SE(7), E(6), NE(5), N(4), NW(3), W(2), SW(1)
	directionMap := []int{0, 7, 6, 5, 4, 3, 2, 1}
	return directionMap[sector]
}

// PlayerCharacter represents the player's character in Play Mode.
// CompositeFrame holds a pre-composited sprite frame (head + body merged).
type CompositeFrame struct {
	Texture uint32 // OpenGL texture ID
	Width   int    // Texture width in pixels
	Height  int    // Texture height in pixels
	OriginX int    // X offset from sprite origin to texture center
	OriginY int    // Y offset from sprite origin to texture center
}

type PlayerCharacter struct {
	// Position in world coordinates
	WorldX float32
	WorldY float32 // Altitude (follows terrain)
	WorldZ float32

	// Movement state
	IsMoving  bool
	Direction int     // 0-7: S, SW, W, NW, N, NE, E, SE
	MoveSpeed float32 // Units per second

	// Click-to-move destination
	DestX          float32 // Target X position
	DestZ          float32 // Target Z position
	HasDestination bool    // Whether moving to a destination

	// Animation state
	CurrentAction int     // 0=Idle, 1=Walk
	CurrentFrame  int     // Current frame within action
	FrameTime     float32 // Accumulated time for frame timing (ms)

	// Sprite data (body)
	SPR      *formats.SPR
	ACT      *formats.ACT
	Textures []uint32 // GPU textures for each SPR image

	// Head sprite data
	HeadSPR      *formats.SPR
	HeadACT      *formats.ACT
	HeadTextures []uint32 // GPU textures for head SPR images

	// Composite textures: [action*8+direction][frame] -> CompositeFrame
	// Pre-composited head+body for each animation frame
	CompositeFrames map[int][]CompositeFrame
	UseComposite    bool // Whether to use composite rendering

	// Billboard rendering
	VAO         uint32
	VBO         uint32
	SpriteScale float32 // Scale factor for sprite (default 1.0)

	// Shadow
	ShadowTex uint32 // Shadow texture (ellipse)
	ShadowVAO uint32
	ShadowVBO uint32
}

// compositeSprites creates a single RGBA image by compositing body and head sprites.
// It uses anchor points to correctly position the head relative to the body.
// Returns the composite image pixels, dimensions, and origin offset.
func compositeSprites(
	bodySPR *formats.SPR, bodyACT *formats.ACT,
	headSPR *formats.SPR, headACT *formats.ACT,
	action, direction, frame int,
) (pixels []byte, width, height, originX, originY int) {
	// Get body action/frame
	bodyActionIdx := action*8 + direction
	if bodyActionIdx >= len(bodyACT.Actions) {
		bodyActionIdx = direction % len(bodyACT.Actions)
	}
	bodyAction := &bodyACT.Actions[bodyActionIdx]
	if len(bodyAction.Frames) == 0 {
		return nil, 0, 0, 0, 0
	}
	bodyFrameIdx := frame % len(bodyAction.Frames)
	bodyFrame := &bodyAction.Frames[bodyFrameIdx]

	// Get head action/frame (always use frame 0 for stability)
	headActionIdx := action*8 + direction
	if headActionIdx >= len(headACT.Actions) {
		headActionIdx = direction % len(headACT.Actions)
	}
	headAction := &headACT.Actions[headActionIdx]
	if len(headAction.Frames) == 0 {
		return nil, 0, 0, 0, 0
	}
	headFrame := &headAction.Frames[0] // Always frame 0 for head

	// Find body layer bounds
	var bodyMinX, bodyMinY, bodyMaxX, bodyMaxY int
	bodyMinX, bodyMinY = 10000, 10000
	bodyMaxX, bodyMaxY = -10000, -10000

	for _, layer := range bodyFrame.Layers {
		if layer.SpriteID < 0 || int(layer.SpriteID) >= len(bodySPR.Images) {
			continue
		}
		img := &bodySPR.Images[layer.SpriteID]
		x, y := int(layer.X), int(layer.Y)
		w, h := int(img.Width), int(img.Height)

		// Layer position is center of sprite
		left := x - w/2
		top := y - h/2
		right := left + w
		bottom := top + h

		if left < bodyMinX {
			bodyMinX = left
		}
		if top < bodyMinY {
			bodyMinY = top
		}
		if right > bodyMaxX {
			bodyMaxX = right
		}
		if bottom > bodyMaxY {
			bodyMaxY = bottom
		}
	}

	// Get body anchor point (where head attaches)
	var bodyAnchorX, bodyAnchorY int
	if len(bodyFrame.AnchorPoints) > 0 {
		bodyAnchorX = int(bodyFrame.AnchorPoints[0].X)
		bodyAnchorY = int(bodyFrame.AnchorPoints[0].Y)
	}

	// Get head anchor point
	var headAnchorX, headAnchorY int
	if len(headFrame.AnchorPoints) > 0 {
		headAnchorX = int(headFrame.AnchorPoints[0].X)
		headAnchorY = int(headFrame.AnchorPoints[0].Y)
	}

	// Calculate head offset: head anchor aligns with body anchor
	headOffsetX := bodyAnchorX - headAnchorX
	headOffsetY := bodyAnchorY - headAnchorY

	// Find head layer bounds (relative to head origin + offset)
	var headMinX, headMinY, headMaxX, headMaxY int
	headMinX, headMinY = 10000, 10000
	headMaxX, headMaxY = -10000, -10000

	for _, layer := range headFrame.Layers {
		if layer.SpriteID < 0 || int(layer.SpriteID) >= len(headSPR.Images) {
			continue
		}
		img := &headSPR.Images[layer.SpriteID]
		x, y := int(layer.X)+headOffsetX, int(layer.Y)+headOffsetY
		w, h := int(img.Width), int(img.Height)

		left := x - w/2
		top := y - h/2
		right := left + w
		bottom := top + h

		if left < headMinX {
			headMinX = left
		}
		if top < headMinY {
			headMinY = top
		}
		if right > headMaxX {
			headMaxX = right
		}
		if bottom > headMaxY {
			headMaxY = bottom
		}
	}

	// Combine bounds
	minX := bodyMinX
	if headMinX < minX {
		minX = headMinX
	}
	minY := bodyMinY
	if headMinY < minY {
		minY = headMinY
	}
	maxX := bodyMaxX
	if headMaxX > maxX {
		maxX = headMaxX
	}
	maxY := bodyMaxY
	if headMaxY > maxY {
		maxY = headMaxY
	}

	// Handle empty sprites
	if minX >= maxX || minY >= maxY {
		return nil, 0, 0, 0, 0
	}

	// Create canvas
	width = maxX - minX
	height = maxY - minY
	originX = -minX // Offset from canvas origin to sprite origin
	originY = -minY
	pixels = make([]byte, width*height*4)

	// Helper to blit a sprite layer onto canvas
	blitLayer := func(spr *formats.SPR, layer *formats.Layer, offsetX, offsetY int) {
		if layer.SpriteID < 0 || int(layer.SpriteID) >= len(spr.Images) {
			return
		}
		img := &spr.Images[layer.SpriteID]
		imgW, imgH := int(img.Width), int(img.Height)

		// SPR images are already converted to RGBA format
		rgba := img.Pixels
		if rgba == nil || len(rgba) == 0 {
			return
		}

		// Layer center position + offset
		cx := int(layer.X) + offsetX + originX
		cy := int(layer.Y) + offsetY + originY

		// Blit with alpha blending
		for py := 0; py < imgH; py++ {
			for px := 0; px < imgW; px++ {
				dx := cx + px - imgW/2
				dy := cy + py - imgH/2
				if dx < 0 || dx >= width || dy < 0 || dy >= height {
					continue
				}

				srcIdx := (py*imgW + px) * 4
				dstIdx := (dy*width + dx) * 4

				// Source pixel
				sr, sg, sb, sa := rgba[srcIdx], rgba[srcIdx+1], rgba[srcIdx+2], rgba[srcIdx+3]
				if sa == 0 {
					continue // Fully transparent
				}

				// Alpha blend
				if sa == 255 {
					pixels[dstIdx] = sr
					pixels[dstIdx+1] = sg
					pixels[dstIdx+2] = sb
					pixels[dstIdx+3] = sa
				} else {
					// Simple alpha blend
					da := pixels[dstIdx+3]
					outA := sa + da*(255-sa)/255
					if outA > 0 {
						pixels[dstIdx] = byte((int(sr)*int(sa) + int(pixels[dstIdx])*int(da)*(255-int(sa))/255) / int(outA))
						pixels[dstIdx+1] = byte((int(sg)*int(sa) + int(pixels[dstIdx+1])*int(da)*(255-int(sa))/255) / int(outA))
						pixels[dstIdx+2] = byte((int(sb)*int(sa) + int(pixels[dstIdx+2])*int(da)*(255-int(sa))/255) / int(outA))
						pixels[dstIdx+3] = outA
					}
				}
			}
		}
	}

	// Draw body layers first (bottom)
	for _, layer := range bodyFrame.Layers {
		blitLayer(bodySPR, &layer, 0, 0)
	}

	// Draw head layers on top
	for _, layer := range headFrame.Layers {
		blitLayer(headSPR, &layer, headOffsetX, headOffsetY)
	}

	return pixels, width, height, originX, originY
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
	terrainGroups []terrainTextureGroup

	// Ground textures and lightmap
	groundTextures map[int]uint32
	fallbackTex    uint32
	lightmapAtlas  uint32
	atlasSize      int32 // Atlas dimensions (square)
	tilesPerRow    int32 // Number of lightmap tiles per row in atlas

	// Placed models
	models      []*MapModel
	ModelGroups []ModelGroup // Models grouped by RSM name
	MaxModels   int          // Maximum models to load (0 = unlimited)
	SelectedIdx int          // Currently selected model index (-1 = none)
	ModelFilter string       // Filter string for model names

	// Debug options
	ForceAllTwoSided bool // Force all faces to render as two-sided (debug)

	// Diagnostics
	Diagnostics MapDiagnostics

	// Camera - Orbit mode
	rotationX float32
	rotationY float32
	Distance  float32 // Public for zoom control
	centerX   float32
	centerY   float32
	centerZ   float32

	// Camera - Play mode (RO-style third-person)
	PlayMode  bool
	camPosX   float32
	camPosY   float32
	camPosZ   float32
	camYaw    float32 // Horizontal angle (radians)
	camPitch  float32 // Vertical angle (radians)
	MoveSpeed float32

	// Player character (Play mode)
	Player        *PlayerCharacter
	spriteProgram    uint32 // Shader for billboard sprites
	locSpriteVP      int32  // viewProj uniform
	locSpritePos     int32  // world position uniform
	locSpriteSize    int32  // sprite size uniform
	locSpriteCamRight int32 // camera right vector for billboard
	locSpriteCamUp    int32 // camera up vector for billboard
	locSpriteTex  int32  // texture uniform
	locSpriteTint int32  // color tint uniform

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
}

// terrainVertex is the vertex format for terrain mesh.
type terrainVertex struct {
	Position   [3]float32
	Normal     [3]float32
	TexCoord   [2]float32
	LightmapUV [2]float32
	Color      [4]float32
}

// terrainTextureGroup groups triangles by texture for batched rendering.
type terrainTextureGroup struct {
	textureID  int
	startIndex int32
	indexCount int32
}

const terrainVertexShader = `#version 410 core
layout (location = 0) in vec3 aPosition;
layout (location = 1) in vec3 aNormal;
layout (location = 2) in vec2 aTexCoord;
layout (location = 3) in vec2 aLightmapUV;
layout (location = 4) in vec4 aColor;

uniform mat4 uViewProj;

out vec3 vNormal;
out vec2 vTexCoord;
out vec2 vLightmapUV;
out vec4 vColor;

void main() {
    vNormal = aNormal;
    vTexCoord = aTexCoord;
    vLightmapUV = aLightmapUV;
    vColor = aColor;
    gl_Position = uViewProj * vec4(aPosition, 1.0);
}
`

const terrainFragmentShader = `#version 410 core
in vec3 vNormal;
in vec2 vTexCoord;
in vec2 vLightmapUV;
in vec4 vColor;

uniform sampler2D uTexture;
uniform sampler2D uLightmap;
uniform vec3 uLightDir;
uniform vec3 uAmbient;
uniform vec3 uDiffuse;
uniform float uBrightness;
uniform float uLightOpacity;

// Fog uniforms (roBrowser style)
uniform bool uFogUse;
uniform float uFogNear;
uniform float uFogFar;
uniform vec3 uFogColor;

out vec4 FragColor;

void main() {
    vec4 texColor = texture(uTexture, vTexCoord);

    // Discard transparent pixels (magenta key areas)
    if (texColor.a < 0.5) {
        discard;
    }

    // Lightmap: RGB = color tint, A = shadow intensity (0=shadow, 1=lit)
    vec4 lightmap = texture(uLightmap, vLightmapUV);
    float shadowIntensity = lightmap.a;  // 0.0 = full shadow, 1.0 = fully lit
    vec3 colorTint = lightmap.rgb;  // Color tint (0-255 normalized by GPU)

    // Directional light component (sun)
    vec3 normal = normalize(vNormal);
    vec3 lightDir = normalize(uLightDir);
    float NdotL = max(dot(normal, lightDir), 0.0);
    vec3 directional = uDiffuse * NdotL;

    // Lighting formula:
    // Ambient provides base illumination (not fully shadowed)
    // Directional light (sun) is affected by lightmap shadows
    // Opacity controls shadow visibility (higher = darker shadows)
    vec3 ambient = uAmbient;

    // Shadow affects directional light, ambient provides minimum illumination
    // Mix ambient shadow based on opacity (0 = no shadow effect, 1 = full shadow)
    float ambientShadow = mix(1.0, shadowIntensity, uLightOpacity);
    vec3 lighting = ambient * ambientShadow + directional * shadowIntensity;

    // Clamp lighting to [0, 1] range (prevents overbright)
    lighting = clamp(lighting, vec3(0.0), vec3(1.0));

    // Ensure vertex color doesn't cause black (default to white if black)
    vec3 vertColor = vColor.rgb;
    if (vertColor.r + vertColor.g + vertColor.b < 0.1) {
        vertColor = vec3(1.0);
    }

    // Final color: (texture * lighting * vertColor * brightness) + colorTint
    // roBrowser formula: texture * LightColor + ColorMap
    vec3 finalColor = texColor.rgb * lighting * vertColor * uBrightness + colorTint;

    // Apply fog (roBrowser formula using smoothstep)
    if (uFogUse) {
        float depth = gl_FragCoord.z / gl_FragCoord.w;
        float fogFactor = smoothstep(uFogNear, uFogFar, depth);
        finalColor = mix(finalColor, uFogColor, fogFactor);
    }

    FragColor = vec4(finalColor, texColor.a * vColor.a);
}
`

// Model vertex type (same as rsmVertex in model_viewer.go)
type modelVertex struct {
	Position [3]float32
	Normal   [3]float32
	TexCoord [2]float32
}

const modelVertexShader = `#version 410 core
layout (location = 0) in vec3 aPosition;
layout (location = 1) in vec3 aNormal;
layout (location = 2) in vec2 aTexCoord;

uniform mat4 uMVP;

out vec3 vNormal;
out vec2 vTexCoord;

void main() {
    vNormal = aNormal;
    vTexCoord = aTexCoord;
    gl_Position = uMVP * vec4(aPosition, 1.0);
}
`

const modelFragmentShader = `#version 410 core
in vec3 vNormal;
in vec2 vTexCoord;

uniform sampler2D uTexture;
uniform vec3 uLightDir;
uniform vec3 uAmbient;
uniform vec3 uDiffuse;

// Fog uniforms (roBrowser style)
uniform bool uFogUse;
uniform float uFogNear;
uniform float uFogFar;
uniform vec3 uFogColor;

out vec4 FragColor;

void main() {
    vec4 texColor = texture(uTexture, vTexCoord);

    // Discard transparent pixels (alpha set to 0 for magenta color key during texture load)
    if (texColor.a < 0.5) {
        discard;
    }

    // Simple lighting with shadow lift (roBrowser uses min 0.5 for models)
    float NdotL = max(dot(normalize(vNormal), normalize(uLightDir)), 0.5);
    vec3 lighting = uAmbient + uDiffuse * NdotL;

    vec3 color = texColor.rgb * lighting;

    // Apply fog (roBrowser formula using smoothstep)
    if (uFogUse) {
        float depth = gl_FragCoord.z / gl_FragCoord.w;
        float fogFactor = smoothstep(uFogNear, uFogFar, depth);
        color = mix(color, uFogColor, fogFactor);
    }

    FragColor = vec4(color, texColor.a);
}
`

// Water shader for semi-transparent water plane
const waterVertexShader = `#version 410 core
layout (location = 0) in vec3 aPosition;

uniform mat4 uMVP;

out vec3 vWorldPos;

void main() {
    vWorldPos = aPosition;
    gl_Position = uMVP * vec4(aPosition, 1.0);
}
`

const waterFragmentShader = `#version 410 core
in vec3 vWorldPos;

uniform vec4 uWaterColor;
uniform float uTime;
uniform float uScrollSpeed;
uniform sampler2D uWaterTex;
uniform int uUseTexture;

out vec4 FragColor;

// Hash function for pseudo-random noise (fallback)
float hash(vec2 p) {
    return fract(sin(dot(p, vec2(127.1, 311.7))) * 43758.5453);
}

float noise(vec2 p) {
    vec2 i = floor(p);
    vec2 f = fract(p);
    f = f * f * (3.0 - 2.0 * f);
    float a = hash(i);
    float b = hash(i + vec2(1.0, 0.0));
    float c = hash(i + vec2(0.0, 1.0));
    float d = hash(i + vec2(1.0, 1.0));
    return mix(mix(a, b, f.x), mix(c, d, f.x), f.y);
}

float fbm(vec2 p, float time) {
    float value = 0.0;
    float amplitude = 0.5;
    vec2 shift = vec2(time * 0.3, time * 0.2);
    for (int i = 0; i < 4; i++) {
        value += amplitude * noise(p + shift);
        p = p * 2.0 + vec2(1.7, 9.2);
        shift *= 1.1;
        amplitude *= 0.5;
    }
    return value;
}

void main() {
    // Scale world position for texture coordinates - tile the texture
    // RO tiles water texture approximately every 50-100 world units
    vec2 uv = vWorldPos.xz * 0.02; // Tiling scale

    if (uUseTexture == 1) {
        // Use loaded water texture - frame animation creates shimmering effect
        // No UV scrolling - just tile the texture
        vec2 tileUV = vWorldPos.xz * 0.004;
        vec4 texColor = texture(uWaterTex, tileUV);
        FragColor = vec4(texColor.rgb, 1.0);
    } else {
        // Fallback to procedural water
        vec2 procUV = vWorldPos.xz * 0.05;
        float pattern1 = fbm(procUV, uTime);
        float pattern2 = fbm(procUV * 1.5 + vec2(5.0), uTime * 0.8);
        float pattern = mix(pattern1, pattern2, 0.5);

        vec3 deepColor = vec3(0.12, 0.30, 0.45);
        vec3 midColor = vec3(0.20, 0.45, 0.55);
        vec3 lightColor = vec3(0.35, 0.60, 0.70);

        vec3 waterColor;
        if (pattern < 0.4) {
            waterColor = mix(deepColor, midColor, pattern / 0.4);
        } else {
            waterColor = mix(midColor, lightColor, (pattern - 0.4) / 0.6);
        }
        float caustic = pow(pattern, 2.5) * 0.4;
        waterColor += vec3(caustic * 0.5, caustic * 0.7, caustic);

        FragColor = vec4(waterColor, uWaterColor.a);
    }
}
`

// Bounding box shader - simple wireframe lines
const bboxVertexShader = `#version 410 core
layout (location = 0) in vec3 aPosition;

uniform mat4 uMVP;

void main() {
    gl_Position = uMVP * vec4(aPosition, 1.0);
}
`

const bboxFragmentShader = `#version 410 core
uniform vec4 uColor;

out vec4 FragColor;

void main() {
    FragColor = uColor;
}
`

// Sprite billboard shader for character rendering
const spriteVertexShader = `#version 410 core
layout (location = 0) in vec2 aPosition;
layout (location = 1) in vec2 aTexCoord;

uniform mat4 uViewProj;
uniform vec3 uWorldPos;
uniform vec2 uSpriteSize;
uniform vec3 uCamRight;  // Camera right vector for billboard
uniform vec3 uCamUp;     // Camera up vector for billboard

out vec2 vTexCoord;

void main() {
    // Camera-facing billboard: sprite always faces the camera
    // This creates the 3D illusion when combined with directional sprite frames
    vec3 pos = uWorldPos;
    pos += uCamRight * aPosition.x * uSpriteSize.x;
    pos += uCamUp * aPosition.y * uSpriteSize.y;

    vTexCoord = aTexCoord;
    gl_Position = uViewProj * vec4(pos, 1.0);
}
`

const spriteFragmentShader = `#version 410 core
in vec2 vTexCoord;

uniform sampler2D uTexture;
uniform vec4 uTint;

out vec4 FragColor;

void main() {
    vec4 texColor = texture(uTexture, vTexCoord);

    // Discard transparent pixels
    if (texColor.a < 0.1) {
        discard;
    }

    FragColor = texColor * uTint;
}
`

// NewMapViewer creates a new 3D map viewer.
func NewMapViewer(width, height int32) (*MapViewer, error) {
	mv := &MapViewer{
		width:          width,
		height:         height,
		groundTextures: make(map[int]uint32),
		rotationX:      0.5,
		rotationY:      0.0,
		Distance:       200.0,
		MoveSpeed:      5.0,
		MaxModels:      1500, // Default model limit
		Brightness:     1.0,  // Default terrain brightness multiplier
		SelectedIdx:    -1,   // No model selected initially
		// Default lighting (will be overwritten by RSW data)
		lightDir:     [3]float32{0.5, 0.866, 0.0}, // 60 degrees elevation
		ambientColor: [3]float32{0.3, 0.3, 0.3},
		diffuseColor: [3]float32{1.0, 1.0, 1.0},
		lightOpacity: 1.0, // Default shadow opacity
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
	// Compile vertex shader
	vertShader := gl.CreateShader(gl.VERTEX_SHADER)
	csource, free := gl.Strs(terrainVertexShader + "\x00")
	gl.ShaderSource(vertShader, 1, csource, nil)
	free()
	gl.CompileShader(vertShader)

	var status int32
	gl.GetShaderiv(vertShader, gl.COMPILE_STATUS, &status)
	if status == gl.FALSE {
		var logLen int32
		gl.GetShaderiv(vertShader, gl.INFO_LOG_LENGTH, &logLen)
		log := make([]byte, logLen)
		gl.GetShaderInfoLog(vertShader, logLen, nil, &log[0])
		return fmt.Errorf("vertex shader: %s", string(log))
	}

	// Compile fragment shader
	fragShader := gl.CreateShader(gl.FRAGMENT_SHADER)
	csource, free = gl.Strs(terrainFragmentShader + "\x00")
	gl.ShaderSource(fragShader, 1, csource, nil)
	free()
	gl.CompileShader(fragShader)

	gl.GetShaderiv(fragShader, gl.COMPILE_STATUS, &status)
	if status == gl.FALSE {
		var logLen int32
		gl.GetShaderiv(fragShader, gl.INFO_LOG_LENGTH, &logLen)
		log := make([]byte, logLen)
		gl.GetShaderInfoLog(fragShader, logLen, nil, &log[0])
		return fmt.Errorf("fragment shader: %s", string(log))
	}

	// Link program
	mv.terrainProgram = gl.CreateProgram()
	gl.AttachShader(mv.terrainProgram, vertShader)
	gl.AttachShader(mv.terrainProgram, fragShader)
	gl.LinkProgram(mv.terrainProgram)

	gl.GetProgramiv(mv.terrainProgram, gl.LINK_STATUS, &status)
	if status == gl.FALSE {
		var logLen int32
		gl.GetProgramiv(mv.terrainProgram, gl.INFO_LOG_LENGTH, &logLen)
		log := make([]byte, logLen)
		gl.GetProgramInfoLog(mv.terrainProgram, logLen, nil, &log[0])
		return fmt.Errorf("link: %s", string(log))
	}

	gl.DeleteShader(vertShader)
	gl.DeleteShader(fragShader)

	// Get uniform locations
	mv.locViewProj = gl.GetUniformLocation(mv.terrainProgram, gl.Str("uViewProj\x00"))
	mv.locLightDir = gl.GetUniformLocation(mv.terrainProgram, gl.Str("uLightDir\x00"))
	mv.locAmbient = gl.GetUniformLocation(mv.terrainProgram, gl.Str("uAmbient\x00"))
	mv.locDiffuse = gl.GetUniformLocation(mv.terrainProgram, gl.Str("uDiffuse\x00"))
	mv.locTexture = gl.GetUniformLocation(mv.terrainProgram, gl.Str("uTexture\x00"))
	mv.locLightmap = gl.GetUniformLocation(mv.terrainProgram, gl.Str("uLightmap\x00"))
	mv.locBrightness = gl.GetUniformLocation(mv.terrainProgram, gl.Str("uBrightness\x00"))
	mv.locLightOpacity = gl.GetUniformLocation(mv.terrainProgram, gl.Str("uLightOpacity\x00"))
	mv.locFogUse = gl.GetUniformLocation(mv.terrainProgram, gl.Str("uFogUse\x00"))
	mv.locFogNear = gl.GetUniformLocation(mv.terrainProgram, gl.Str("uFogNear\x00"))
	mv.locFogFar = gl.GetUniformLocation(mv.terrainProgram, gl.Str("uFogFar\x00"))
	mv.locFogColor = gl.GetUniformLocation(mv.terrainProgram, gl.Str("uFogColor\x00"))

	return nil
}

// createModelShader compiles the RSM model shader program.
func (mv *MapViewer) createModelShader() error {
	// Compile vertex shader
	vertShader := gl.CreateShader(gl.VERTEX_SHADER)
	csource, free := gl.Strs(modelVertexShader + "\x00")
	gl.ShaderSource(vertShader, 1, csource, nil)
	free()
	gl.CompileShader(vertShader)

	var status int32
	gl.GetShaderiv(vertShader, gl.COMPILE_STATUS, &status)
	if status == gl.FALSE {
		var logLen int32
		gl.GetShaderiv(vertShader, gl.INFO_LOG_LENGTH, &logLen)
		log := make([]byte, logLen)
		gl.GetShaderInfoLog(vertShader, logLen, nil, &log[0])
		return fmt.Errorf("model vertex shader: %s", string(log))
	}

	// Compile fragment shader
	fragShader := gl.CreateShader(gl.FRAGMENT_SHADER)
	csource, free = gl.Strs(modelFragmentShader + "\x00")
	gl.ShaderSource(fragShader, 1, csource, nil)
	free()
	gl.CompileShader(fragShader)

	gl.GetShaderiv(fragShader, gl.COMPILE_STATUS, &status)
	if status == gl.FALSE {
		var logLen int32
		gl.GetShaderiv(fragShader, gl.INFO_LOG_LENGTH, &logLen)
		log := make([]byte, logLen)
		gl.GetShaderInfoLog(fragShader, logLen, nil, &log[0])
		return fmt.Errorf("model fragment shader: %s", string(log))
	}

	// Link program
	mv.modelProgram = gl.CreateProgram()
	gl.AttachShader(mv.modelProgram, vertShader)
	gl.AttachShader(mv.modelProgram, fragShader)
	gl.LinkProgram(mv.modelProgram)

	gl.GetProgramiv(mv.modelProgram, gl.LINK_STATUS, &status)
	if status == gl.FALSE {
		var logLen int32
		gl.GetProgramiv(mv.modelProgram, gl.INFO_LOG_LENGTH, &logLen)
		log := make([]byte, logLen)
		gl.GetProgramInfoLog(mv.modelProgram, logLen, nil, &log[0])
		return fmt.Errorf("model shader link: %s", string(log))
	}

	gl.DeleteShader(vertShader)
	gl.DeleteShader(fragShader)

	// Get uniform locations
	mv.locModelMVP = gl.GetUniformLocation(mv.modelProgram, gl.Str("uMVP\x00"))
	mv.locModelLightDir = gl.GetUniformLocation(mv.modelProgram, gl.Str("uLightDir\x00"))
	mv.locModelAmbient = gl.GetUniformLocation(mv.modelProgram, gl.Str("uAmbient\x00"))
	mv.locModelDiffuse = gl.GetUniformLocation(mv.modelProgram, gl.Str("uDiffuse\x00"))
	mv.locModelTexture = gl.GetUniformLocation(mv.modelProgram, gl.Str("uTexture\x00"))
	mv.locModelFogUse = gl.GetUniformLocation(mv.modelProgram, gl.Str("uFogUse\x00"))
	mv.locModelFogNear = gl.GetUniformLocation(mv.modelProgram, gl.Str("uFogNear\x00"))
	mv.locModelFogFar = gl.GetUniformLocation(mv.modelProgram, gl.Str("uFogFar\x00"))
	mv.locModelFogColor = gl.GetUniformLocation(mv.modelProgram, gl.Str("uFogColor\x00"))

	// Compile water shader
	if err := mv.compileWaterShader(); err != nil {
		return fmt.Errorf("water shader: %w", err)
	}

	return nil
}

// createBboxShader compiles the bounding box wireframe shader.
func (mv *MapViewer) createBboxShader() error {
	// Compile vertex shader
	vertShader := gl.CreateShader(gl.VERTEX_SHADER)
	csource, free := gl.Strs(bboxVertexShader + "\x00")
	gl.ShaderSource(vertShader, 1, csource, nil)
	free()
	gl.CompileShader(vertShader)

	var status int32
	gl.GetShaderiv(vertShader, gl.COMPILE_STATUS, &status)
	if status == gl.FALSE {
		var logLen int32
		gl.GetShaderiv(vertShader, gl.INFO_LOG_LENGTH, &logLen)
		log := make([]byte, logLen)
		gl.GetShaderInfoLog(vertShader, logLen, nil, &log[0])
		return fmt.Errorf("bbox vertex shader: %s", string(log))
	}

	// Compile fragment shader
	fragShader := gl.CreateShader(gl.FRAGMENT_SHADER)
	csource, free = gl.Strs(bboxFragmentShader + "\x00")
	gl.ShaderSource(fragShader, 1, csource, nil)
	free()
	gl.CompileShader(fragShader)

	gl.GetShaderiv(fragShader, gl.COMPILE_STATUS, &status)
	if status == gl.FALSE {
		var logLen int32
		gl.GetShaderiv(fragShader, gl.INFO_LOG_LENGTH, &logLen)
		log := make([]byte, logLen)
		gl.GetShaderInfoLog(fragShader, logLen, nil, &log[0])
		return fmt.Errorf("bbox fragment shader: %s", string(log))
	}

	// Link program
	mv.bboxProgram = gl.CreateProgram()
	gl.AttachShader(mv.bboxProgram, vertShader)
	gl.AttachShader(mv.bboxProgram, fragShader)
	gl.LinkProgram(mv.bboxProgram)

	gl.GetProgramiv(mv.bboxProgram, gl.LINK_STATUS, &status)
	if status == gl.FALSE {
		var logLen int32
		gl.GetProgramiv(mv.bboxProgram, gl.INFO_LOG_LENGTH, &logLen)
		log := make([]byte, logLen)
		gl.GetProgramInfoLog(mv.bboxProgram, logLen, nil, &log[0])
		return fmt.Errorf("bbox shader link: %s", string(log))
	}

	gl.DeleteShader(vertShader)
	gl.DeleteShader(fragShader)

	// Get uniform locations
	mv.locBboxMVP = gl.GetUniformLocation(mv.bboxProgram, gl.Str("uMVP\x00"))
	mv.locBboxColor = gl.GetUniformLocation(mv.bboxProgram, gl.Str("uColor\x00"))

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

// createSpriteShader compiles the sprite billboard shader program.
func (mv *MapViewer) createSpriteShader() error {
	// Compile vertex shader
	vertShader := gl.CreateShader(gl.VERTEX_SHADER)
	csource, free := gl.Strs(spriteVertexShader + "\x00")
	gl.ShaderSource(vertShader, 1, csource, nil)
	free()
	gl.CompileShader(vertShader)

	var status int32
	gl.GetShaderiv(vertShader, gl.COMPILE_STATUS, &status)
	if status == gl.FALSE {
		var logLen int32
		gl.GetShaderiv(vertShader, gl.INFO_LOG_LENGTH, &logLen)
		log := make([]byte, logLen)
		gl.GetShaderInfoLog(vertShader, logLen, nil, &log[0])
		return fmt.Errorf("sprite vertex shader: %s", string(log))
	}

	// Compile fragment shader
	fragShader := gl.CreateShader(gl.FRAGMENT_SHADER)
	csource, free = gl.Strs(spriteFragmentShader + "\x00")
	gl.ShaderSource(fragShader, 1, csource, nil)
	free()
	gl.CompileShader(fragShader)

	gl.GetShaderiv(fragShader, gl.COMPILE_STATUS, &status)
	if status == gl.FALSE {
		var logLen int32
		gl.GetShaderiv(fragShader, gl.INFO_LOG_LENGTH, &logLen)
		log := make([]byte, logLen)
		gl.GetShaderInfoLog(fragShader, logLen, nil, &log[0])
		return fmt.Errorf("sprite fragment shader: %s", string(log))
	}

	// Link program
	mv.spriteProgram = gl.CreateProgram()
	gl.AttachShader(mv.spriteProgram, vertShader)
	gl.AttachShader(mv.spriteProgram, fragShader)
	gl.LinkProgram(mv.spriteProgram)

	gl.GetProgramiv(mv.spriteProgram, gl.LINK_STATUS, &status)
	if status == gl.FALSE {
		var logLen int32
		gl.GetProgramiv(mv.spriteProgram, gl.INFO_LOG_LENGTH, &logLen)
		log := make([]byte, logLen)
		gl.GetProgramInfoLog(mv.spriteProgram, logLen, nil, &log[0])
		return fmt.Errorf("sprite shader link: %s", string(log))
	}

	gl.DeleteShader(vertShader)
	gl.DeleteShader(fragShader)

	// Get uniform locations
	mv.locSpriteVP = gl.GetUniformLocation(mv.spriteProgram, gl.Str("uViewProj\x00"))
	mv.locSpritePos = gl.GetUniformLocation(mv.spriteProgram, gl.Str("uWorldPos\x00"))
	mv.locSpriteSize = gl.GetUniformLocation(mv.spriteProgram, gl.Str("uSpriteSize\x00"))
	mv.locSpriteTex = gl.GetUniformLocation(mv.spriteProgram, gl.Str("uTexture\x00"))
	mv.locSpriteTint = gl.GetUniformLocation(mv.spriteProgram, gl.Str("uTint\x00"))
	mv.locSpriteCamRight = gl.GetUniformLocation(mv.spriteProgram, gl.Str("uCamRight\x00"))
	mv.locSpriteCamUp = gl.GetUniformLocation(mv.spriteProgram, gl.Str("uCamUp\x00"))

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
	var status int32

	// Compile vertex shader
	vertShader := gl.CreateShader(gl.VERTEX_SHADER)
	csource, free := gl.Strs(waterVertexShader + "\x00")
	gl.ShaderSource(vertShader, 1, csource, nil)
	free()
	gl.CompileShader(vertShader)

	gl.GetShaderiv(vertShader, gl.COMPILE_STATUS, &status)
	if status == gl.FALSE {
		var logLen int32
		gl.GetShaderiv(vertShader, gl.INFO_LOG_LENGTH, &logLen)
		log := make([]byte, logLen)
		gl.GetShaderInfoLog(vertShader, logLen, nil, &log[0])
		return fmt.Errorf("water vertex shader: %s", string(log))
	}

	// Compile fragment shader
	fragShader := gl.CreateShader(gl.FRAGMENT_SHADER)
	csource, free = gl.Strs(waterFragmentShader + "\x00")
	gl.ShaderSource(fragShader, 1, csource, nil)
	free()
	gl.CompileShader(fragShader)

	gl.GetShaderiv(fragShader, gl.COMPILE_STATUS, &status)
	if status == gl.FALSE {
		var logLen int32
		gl.GetShaderiv(fragShader, gl.INFO_LOG_LENGTH, &logLen)
		log := make([]byte, logLen)
		gl.GetShaderInfoLog(fragShader, logLen, nil, &log[0])
		return fmt.Errorf("water fragment shader: %s", string(log))
	}

	// Link program
	mv.waterProgram = gl.CreateProgram()
	gl.AttachShader(mv.waterProgram, vertShader)
	gl.AttachShader(mv.waterProgram, fragShader)
	gl.LinkProgram(mv.waterProgram)

	gl.GetProgramiv(mv.waterProgram, gl.LINK_STATUS, &status)
	if status == gl.FALSE {
		var logLen int32
		gl.GetProgramiv(mv.waterProgram, gl.INFO_LOG_LENGTH, &logLen)
		log := make([]byte, logLen)
		gl.GetProgramInfoLog(mv.waterProgram, logLen, nil, &log[0])
		return fmt.Errorf("water shader link: %s", string(log))
	}

	gl.DeleteShader(vertShader)
	gl.DeleteShader(fragShader)

	// Get uniform locations
	mv.locWaterMVP = gl.GetUniformLocation(mv.waterProgram, gl.Str("uMVP\x00"))
	mv.locWaterColor = gl.GetUniformLocation(mv.waterProgram, gl.Str("uWaterColor\x00"))
	mv.locWaterTime = gl.GetUniformLocation(mv.waterProgram, gl.Str("uTime\x00"))
	mv.locWaterTex = gl.GetUniformLocation(mv.waterProgram, gl.Str("uWaterTex\x00"))

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
	mv.buildTerrainHeightMap(gnd)

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
		mv.lightDir = calculateSunDirection(rsw.Light.Longitude, rsw.Light.Latitude)

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
	}

	// Load ground textures
	mv.loadGroundTextures(gnd, texLoader)

	// Build lightmap atlas (Stage 2)
	mv.buildLightmapAtlas(gnd)

	// Build terrain mesh
	vertices, indices, groups := mv.buildTerrainMesh(gnd)
	mv.terrainGroups = groups

	// Upload to GPU
	mv.uploadTerrainMesh(vertices, indices)

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

	// Fit camera to map
	mv.fitCamera()

	// Override with preferred defaults
	mv.Distance = 340.0
	mv.modelAnimPlaying = true // Animation tracking enabled (rebuild disabled until fixed)

	return nil
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
	if mv.lightmapAtlas != 0 {
		gl.DeleteTextures(1, &mv.lightmapAtlas)
		mv.lightmapAtlas = 0
	}

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
	mv.centerX = worldX
	mv.centerY = worldY
	mv.centerZ = worldZ

	// Set reasonable zoom distance based on model bounding box
	bboxSize := gomath.Max(
		float64(model.bbox[3]-model.bbox[0]),
		gomath.Max(float64(model.bbox[4]-model.bbox[1]), float64(model.bbox[5]-model.bbox[2])),
	)
	mv.Distance = float32(gomath.Max(bboxSize*2, 50))
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
func (m *MapModel) GetNodes() []NodeDebugInfo {
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
	var vertices []modelVertex
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
		nodeMatrix := mv.buildNodeMatrix(node, rsm, 0) // Initial pose (time=0)

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
			normalVec := cross(e1, e2)

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
					pos := transformPoint(nodeMatrix, v)

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

					vertices = append(vertices, modelVertex{
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
	var groups []modelTexGroup
	for texIdx, idxs := range texGroups {
		if len(idxs) == 0 {
			continue
		}
		groups = append(groups, modelTexGroup{
			texIdx:     texIdx,
			startIndex: int32(len(indices)),
			indexCount: int32(len(idxs)),
		})
		indices = append(indices, idxs...)
	}

	// Smooth normals for models (reduces faceted appearance)
	smoothModelNormals(vertices)

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
	nodeDebugInfo := make([]NodeDebugInfo, len(rsm.Nodes))
	for i, node := range rsm.Nodes {
		info := NodeDebugInfo{
			Name:         node.Name,
			Parent:       node.Parent,
			Offset:       node.Offset,
			Position:     node.Position,
			Scale:        node.Scale,
			RotAngle:     node.RotAngle,
			RotAxis:      node.RotAxis,
			Matrix:       node.Matrix,
			HasRotKeys:   len(node.RotKeys) > 0,
			HasPosKeys:   len(node.PosKeys) > 0,
			HasScaleKeys: len(node.ScaleKeys) > 0,
			RotKeyCount:  len(node.RotKeys),
		}
		if len(node.RotKeys) > 0 {
			info.FirstRotQuat = node.RotKeys[0].Quaternion
		}
		nodeDebugInfo[i] = info
	}

	// Check if model has animation (any node with >1 keyframes AND animLength > 0)
	// Models with only 1 keyframe are static poses, not animations
	hasAnimation := false
	if rsm.AnimLength > 0 {
		for i := range rsm.Nodes {
			node := &rsm.Nodes[i]
			if len(node.RotKeys) > 1 || len(node.PosKeys) > 1 || len(node.ScaleKeys) > 1 {
				hasAnimation = true
				break
			}
		}
	}

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
	gl.BufferData(gl.ARRAY_BUFFER, len(vertices)*int(unsafe.Sizeof(modelVertex{})), gl.Ptr(vertices), gl.STATIC_DRAW)

	gl.GenBuffers(1, &model.ebo)
	gl.BindBuffer(gl.ELEMENT_ARRAY_BUFFER, model.ebo)
	gl.BufferData(gl.ELEMENT_ARRAY_BUFFER, len(indices)*4, gl.Ptr(indices), gl.STATIC_DRAW)

	model.indexCount = int32(len(indices))

	// Set vertex attributes (Position, Normal, TexCoord)
	stride := int32(unsafe.Sizeof(modelVertex{}))
	gl.EnableVertexAttribArray(0)
	gl.VertexAttribPointerWithOffset(0, 3, gl.FLOAT, false, stride, 0)
	gl.EnableVertexAttribArray(1)
	gl.VertexAttribPointerWithOffset(1, 3, gl.FLOAT, false, stride, 12)
	gl.EnableVertexAttribArray(2)
	gl.VertexAttribPointerWithOffset(2, 2, gl.FLOAT, false, stride, 24)

	gl.BindVertexArray(0)

	return model
}

// buildNodeMatrix builds the transformation matrix for an RSM node.
// Following roBrowser's approach: hierarchy matrix (inherited) + vertex transform (not inherited).
func (mv *MapViewer) buildNodeMatrix(node *formats.RSMNode, rsm *formats.RSM, animTimeMs float32) math.Mat4 {
	// Get hierarchy matrix (parent * Position * Rotation * Scale)
	visited := make(map[string]bool)
	hierarchyMatrix := mv.buildNodeHierarchyMatrix(node, rsm, animTimeMs, visited)

	// Add Offset and Mat3 for vertex transformation (NOT inherited by children)
	result := hierarchyMatrix
	result = result.Mul(math.Translate(node.Offset[0], node.Offset[1], node.Offset[2]))
	result = result.Mul(math.FromMat3x3(node.Matrix))

	return result
}

// buildNodeHierarchyMatrix returns the matrix that children inherit.
// This is: parent_hierarchy * Position * Rotation * Scale
// It does NOT include Offset or Mat3 (those are vertex-only transforms).
func (mv *MapViewer) buildNodeHierarchyMatrix(node *formats.RSMNode, rsm *formats.RSM, animTimeMs float32, visited map[string]bool) math.Mat4 {
	// Prevent infinite recursion
	if visited[node.Name] {
		return math.Identity()
	}
	visited[node.Name] = true

	// Check if node has rotation keyframes
	hasRotKeyframes := len(node.RotKeys) > 0

	// Build local hierarchy matrix: Position * Rotation * Scale
	localMatrix := math.Translate(node.Position[0], node.Position[1], node.Position[2])

	// Apply rotation (axis-angle OR keyframe, not both)
	if !hasRotKeyframes && node.RotAngle != 0 {
		axisLen := float32(gomath.Sqrt(float64(
			node.RotAxis[0]*node.RotAxis[0] +
				node.RotAxis[1]*node.RotAxis[1] +
				node.RotAxis[2]*node.RotAxis[2])))
		if axisLen > 1e-6 {
			normalizedAxis := [3]float32{
				node.RotAxis[0] / axisLen,
				node.RotAxis[1] / axisLen,
				node.RotAxis[2] / axisLen,
			}
			localMatrix = localMatrix.Mul(math.RotateAxis(normalizedAxis, node.RotAngle))
		}
	} else if hasRotKeyframes {
		rotQuat := mv.interpolateRotKeys(node.RotKeys, animTimeMs)
		localMatrix = localMatrix.Mul(rotQuat.ToMat4())
	}

	localMatrix = localMatrix.Mul(math.Scale(node.Scale[0], node.Scale[1], node.Scale[2]))

	// Apply animation scale if present
	if len(node.ScaleKeys) > 0 {
		scale := mv.interpolateScaleKeys(node.ScaleKeys, animTimeMs)
		localMatrix = localMatrix.Mul(math.Scale(scale[0], scale[1], scale[2]))
	}

	// If node has parent, get parent's hierarchy matrix first
	if node.Parent != "" && node.Parent != node.Name {
		parentNode := rsm.GetNodeByName(node.Parent)
		if parentNode != nil {
			parentHierarchy := mv.buildNodeHierarchyMatrix(parentNode, rsm, animTimeMs, visited)
			return parentHierarchy.Mul(localMatrix)
		}
	}

	return localMatrix
}

// transformPoint transforms a point by a 4x4 matrix.
func transformPoint(m math.Mat4, p [3]float32) [3]float32 {
	x := m[0]*p[0] + m[4]*p[1] + m[8]*p[2] + m[12]
	y := m[1]*p[0] + m[5]*p[1] + m[9]*p[2] + m[13]
	z := m[2]*p[0] + m[6]*p[1] + m[10]*p[2] + m[14]
	return [3]float32{x, y, z}
}

// --- Animation Interpolation Functions ---

// interpolateRotKeys interpolates rotation keyframes at the given time.
func (mv *MapViewer) interpolateRotKeys(keys []formats.RSMRotKeyframe, timeMs float32) math.Quat {
	if len(keys) == 0 {
		return math.QuatIdentity()
	}
	if len(keys) == 1 {
		k := keys[0]
		return math.Quat{X: k.Quaternion[0], Y: k.Quaternion[1], Z: k.Quaternion[2], W: k.Quaternion[3]}
	}

	// Find surrounding keyframes (assuming keys are sorted by frame)
	// RSM frame numbers need to be converted to time
	var prev, next int
	for i := range keys {
		if float32(keys[i].Frame) > timeMs {
			next = i
			break
		}
		prev = i
		next = i
	}

	// If at or past last frame, return last frame's rotation
	if prev == next {
		k := keys[prev]
		return math.Quat{X: k.Quaternion[0], Y: k.Quaternion[1], Z: k.Quaternion[2], W: k.Quaternion[3]}
	}

	// Interpolate between prev and next
	k0 := keys[prev]
	k1 := keys[next]
	t := float32(0)
	if k1.Frame != k0.Frame {
		t = (timeMs - float32(k0.Frame)) / float32(k1.Frame-k0.Frame)
	}

	q0 := math.Quat{X: k0.Quaternion[0], Y: k0.Quaternion[1], Z: k0.Quaternion[2], W: k0.Quaternion[3]}
	q1 := math.Quat{X: k1.Quaternion[0], Y: k1.Quaternion[1], Z: k1.Quaternion[2], W: k1.Quaternion[3]}
	return q0.Slerp(q1, t)
}

// interpolateScaleKeys interpolates scale keyframes at the given time.
func (mv *MapViewer) interpolateScaleKeys(keys []formats.RSMScaleKeyframe, timeMs float32) [3]float32 {
	if len(keys) == 0 {
		return [3]float32{1, 1, 1}
	}
	if len(keys) == 1 {
		return keys[0].Scale
	}

	var prev, next int
	for i := range keys {
		if float32(keys[i].Frame) > timeMs {
			next = i
			break
		}
		prev = i
		next = i
	}

	if prev == next {
		return keys[prev].Scale
	}

	k0 := keys[prev]
	k1 := keys[next]
	t := float32(0)
	if k1.Frame != k0.Frame {
		t = (timeMs - float32(k0.Frame)) / float32(k1.Frame-k0.Frame)
	}

	return [3]float32{
		k0.Scale[0] + t*(k1.Scale[0]-k0.Scale[0]),
		k0.Scale[1] + t*(k1.Scale[1]-k0.Scale[1]),
		k0.Scale[2] + t*(k1.Scale[2]-k0.Scale[2]),
	}
}

// buildAnimatedModelMesh builds vertices and indices for an animated model at a given time.
func (mv *MapViewer) buildAnimatedModelMesh(rsm *formats.RSM, ref *formats.RSWModel, animTimeMs float32) ([]modelVertex, []uint32, []modelTexGroup) {
	var vertices []modelVertex
	var indices []uint32
	texGroups := make(map[int][]uint32)

	// Process each node
	for i := range rsm.Nodes {
		node := &rsm.Nodes[i]

		// Build node transform with animation time
		nodeMatrix := mv.buildNodeMatrix(node, rsm, animTimeMs)

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
			normalVec := cross(e1, e2)
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
					transformedPos := transformPoint(nodeMatrix, pos)
					// Flip Y for RO coordinate system
					transformedPos[1] = -transformedPos[1]

					var uv [2]float32
					if int(texIDs[j]) < len(node.TexCoords) {
						tc := node.TexCoords[texIDs[j]]
						uv = [2]float32{tc.U, tc.V}
					}

					vertices = append(vertices, modelVertex{
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
	var groups []modelTexGroup
	for texIdx, idxs := range texGroups {
		startIdx := int32(len(indices))
		indices = append(indices, idxs...)
		groups = append(groups, modelTexGroup{
			texIdx:     texIdx,
			startIndex: startIdx,
			indexCount: int32(len(idxs)),
		})
	}

	return vertices, indices, groups
}

// buildLightmapAtlas creates a texture atlas from GND lightmaps.
func (mv *MapViewer) buildLightmapAtlas(gnd *formats.GND) {
	if len(gnd.Lightmaps) == 0 {
		// Create a simple white lightmap if none exist
		mv.atlasSize = 8
		mv.tilesPerRow = 1
		gl.GenTextures(1, &mv.lightmapAtlas)
		gl.BindTexture(gl.TEXTURE_2D, mv.lightmapAtlas)
		white := make([]uint8, 64*3) // 8x8 RGB
		for i := range white {
			white[i] = 255
		}
		gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGB, 8, 8, 0, gl.RGB, gl.UNSIGNED_BYTE, gl.Ptr(white))
		gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR)
		gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)
		return
	}

	// Calculate atlas size (square, power of 2)
	lmWidth := int(gnd.LightmapWidth)
	lmHeight := int(gnd.LightmapHeight)
	if lmWidth == 0 {
		lmWidth = 8
	}
	if lmHeight == 0 {
		lmHeight = 8
	}

	// Calculate how many lightmaps fit per row
	numLightmaps := len(gnd.Lightmaps)
	tilesPerRow := 1
	for tilesPerRow*tilesPerRow < numLightmaps {
		tilesPerRow *= 2
	}

	atlasSize := tilesPerRow * lmWidth
	// Round up to next power of 2
	pow2 := 64
	for pow2 < atlasSize {
		pow2 *= 2
	}
	atlasSize = pow2
	if atlasSize > 4096 {
		atlasSize = 4096
	}

	mv.atlasSize = int32(atlasSize)
	mv.tilesPerRow = int32(atlasSize / lmWidth)

	// Create RGBA atlas (4 bytes per pixel)
	atlasData := make([]byte, atlasSize*atlasSize*4)

	// Fill with default (white color, full brightness)
	for i := 0; i < len(atlasData); i += 4 {
		atlasData[i] = 255   // R
		atlasData[i+1] = 255 // G
		atlasData[i+2] = 255 // B
		atlasData[i+3] = 255 // A (brightness/shadow)
	}

	// Copy each lightmap into the atlas
	// GND lightmap format:
	// - Brightness: shadow/intensity (0=dark shadow, 255=fully lit)
	// - ColorRGB: color tint to add
	for i, lm := range gnd.Lightmaps {
		tileX := i % int(mv.tilesPerRow)
		tileY := i / int(mv.tilesPerRow)

		baseX := tileX * lmWidth
		baseY := tileY * lmHeight

		// Copy lightmap pixels directly (no Y flip - testing)
		for y := 0; y < lmHeight; y++ {
			for x := 0; x < lmWidth; x++ {
				srcIdx := y*lmWidth + x
				dstX := baseX + x
				dstY := baseY + y

				if dstX >= atlasSize || dstY >= atlasSize {
					continue
				}

				dstIdx := (dstY*atlasSize + dstX) * 4

				// Get brightness (shadow intensity) for alpha channel
				var brightness uint8 = 255
				if srcIdx < len(lm.Brightness) {
					brightness = lm.Brightness[srcIdx]
				}

				// Get RGB color tint
				var r, g, b uint8 = 0, 0, 0
				if srcIdx*3+2 < len(lm.ColorRGB) {
					r = lm.ColorRGB[srcIdx*3]
					g = lm.ColorRGB[srcIdx*3+1]
					b = lm.ColorRGB[srcIdx*3+2]
				}

				// Store: RGB = color tint, A = shadow intensity
				atlasData[dstIdx] = r
				atlasData[dstIdx+1] = g
				atlasData[dstIdx+2] = b
				atlasData[dstIdx+3] = brightness
			}
		}
	}

	// Upload RGBA atlas to GPU
	gl.GenTextures(1, &mv.lightmapAtlas)
	gl.BindTexture(gl.TEXTURE_2D, mv.lightmapAtlas)
	gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA, int32(atlasSize), int32(atlasSize), 0, gl.RGBA, gl.UNSIGNED_BYTE, gl.Ptr(atlasData))

	// Generate mipmaps for smooth lightmap at distance
	gl.GenerateMipmap(gl.TEXTURE_2D)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR_MIPMAP_LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
}

// calculateLightmapUV returns UV coordinates for a lightmap in the atlas.
// cornerIdx: 0=BL, 1=BR, 2=TL, 3=TR
//
// Uses 0.125/0.875 offsets (1/8 and 7/8 of tile size) to center UV sampling
// within the inner 75% of each lightmap tile, avoiding boundary bleeding
// that causes the chess board pattern. This matches roBrowser's approach.
func (mv *MapViewer) calculateLightmapUV(lightmapID int16, cornerIdx int, gnd *formats.GND) [2]float32 {
	if lightmapID < 0 || mv.tilesPerRow == 0 {
		return [2]float32{0.5, 0.5} // Center of first tile as fallback
	}

	lmWidth := int(gnd.LightmapWidth)
	lmHeight := int(gnd.LightmapHeight)
	if lmWidth == 0 {
		lmWidth = 8
	}
	if lmHeight == 0 {
		lmHeight = 8
	}

	// Position of lightmap tile in atlas
	tileX := int(lightmapID) % int(mv.tilesPerRow)
	tileY := int(lightmapID) / int(mv.tilesPerRow)

	// Calculate UV with 1/8 inset on each side to avoid edge bleeding
	// roBrowser uses 0.125 (1/8) and 0.875 (7/8) within each tile
	atlasSize := float32(mv.atlasSize)
	tileW := float32(lmWidth) / atlasSize
	tileH := float32(lmHeight) / atlasSize

	baseU := float32(tileX*lmWidth) / atlasSize
	baseV := float32(tileY*lmHeight) / atlasSize

	// Use full tile range with half-pixel inset to avoid edge bleeding
	halfPixelU := 0.5 / float32(mv.atlasSize)
	halfPixelV := 0.5 / float32(mv.atlasSize)
	innerU1 := baseU + halfPixelU
	innerU2 := baseU + tileW - halfPixelU
	innerV1 := baseV + halfPixelV
	innerV2 := baseV + tileH - halfPixelV

	// Corner UVs within the tile
	// GND UV order: [0]=BL, [1]=BR, [2]=TL, [3]=TR
	switch cornerIdx {
	case 0: // Bottom-left
		return [2]float32{innerU1, innerV2}
	case 1: // Bottom-right
		return [2]float32{innerU2, innerV2}
	case 2: // Top-left
		return [2]float32{innerU1, innerV1}
	case 3: // Top-right
		return [2]float32{innerU2, innerV1}
	}
	return [2]float32{0.5, 0.5}
}

// smoothModelNormals averages normals at shared vertex positions for models.
// This reduces faceted appearance on models (buildings, trees, etc).
func smoothModelNormals(vertices []modelVertex) {
	const epsilon float32 = 0.001

	// Group vertices by quantized position for O(n) lookup
	posMap := make(map[[3]int32][]int)
	for i := range vertices {
		key := [3]int32{
			int32(vertices[i].Position[0] / epsilon),
			int32(vertices[i].Position[1] / epsilon),
			int32(vertices[i].Position[2] / epsilon),
		}
		posMap[key] = append(posMap[key], i)
	}

	// Average normals for vertices at same position
	for _, indices := range posMap {
		if len(indices) < 2 {
			continue
		}

		var sum [3]float32
		for _, idx := range indices {
			sum[0] += vertices[idx].Normal[0]
			sum[1] += vertices[idx].Normal[1]
			sum[2] += vertices[idx].Normal[2]
		}

		avg := normalize(sum)

		for _, idx := range indices {
			vertices[idx].Normal = avg
		}
	}
}

// smoothTerrainNormals averages normals at shared vertex positions.
// This removes the visible "grid" lighting pattern between tiles by
// making lighting transition smoothly across tile boundaries.
func smoothTerrainNormals(vertices []terrainVertex) {
	const epsilon float32 = 0.001

	// Group vertices by quantized position for O(n) lookup
	posMap := make(map[[3]int32][]int)
	for i := range vertices {
		// Quantize position to grid for fast grouping
		key := [3]int32{
			int32(vertices[i].Position[0] / epsilon),
			int32(vertices[i].Position[1] / epsilon),
			int32(vertices[i].Position[2] / epsilon),
		}
		posMap[key] = append(posMap[key], i)
	}

	// Average normals for vertices at same position
	for _, indices := range posMap {
		if len(indices) < 2 {
			continue // No smoothing needed for isolated vertices
		}

		// Sum all normals at this position
		var sum [3]float32
		for _, idx := range indices {
			sum[0] += vertices[idx].Normal[0]
			sum[1] += vertices[idx].Normal[1]
			sum[2] += vertices[idx].Normal[2]
		}

		// Normalize the average
		avg := normalize(sum)

		// Apply averaged normal to all vertices at this position
		for _, idx := range indices {
			vertices[idx].Normal = avg
		}
	}
}

// buildTerrainMesh generates the terrain mesh from GND data.
func (mv *MapViewer) buildTerrainMesh(gnd *formats.GND) ([]terrainVertex, []uint32, []terrainTextureGroup) {
	var vertices []terrainVertex
	var indices []uint32

	// Map from texture ID to indices
	textureIndices := make(map[int][]uint32)

	tileSize := gnd.Zoom
	width := int(gnd.Width)
	height := int(gnd.Height)

	// Reset bounds
	mv.minBounds = [3]float32{1e10, 1e10, 1e10}
	mv.maxBounds = [3]float32{-1e10, -1e10, -1e10}

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			tile := gnd.GetTile(x, y)
			if tile == nil {
				continue
			}

			// Calculate world positions for tile corners
			// RO coordinate system: X=east, Y=up (negative=higher), Z=south
			baseX := float32(x) * tileSize
			baseZ := float32(y) * tileSize

			// Corner positions (in RO, altitude is negated for world Y)
			// GND corners: [0]=BL, [1]=BR, [2]=TL, [3]=TR
			corners := [4][3]float32{
				{baseX, -tile.Altitude[0], baseZ + tileSize},            // Bottom-left
				{baseX + tileSize, -tile.Altitude[1], baseZ + tileSize}, // Bottom-right
				{baseX, -tile.Altitude[2], baseZ},                       // Top-left
				{baseX + tileSize, -tile.Altitude[3], baseZ},            // Top-right
			}

			// Update bounds
			for _, c := range corners {
				mv.updateBounds(c)
			}

			// Top surface (horizontal quad)
			if tile.TopSurface >= 0 && int(tile.TopSurface) < len(gnd.Surfaces) {
				surface := &gnd.Surfaces[tile.TopSurface]
				texID := int(surface.TextureID)

				// Calculate normal (cross product of edges)
				edge1 := [3]float32{
					corners[1][0] - corners[0][0],
					corners[1][1] - corners[0][1],
					corners[1][2] - corners[0][2],
				}
				edge2 := [3]float32{
					corners[2][0] - corners[0][0],
					corners[2][1] - corners[0][1],
					corners[2][2] - corners[0][2],
				}
				normal := normalize(cross(edge1, edge2))

				// Vertex color from surface
				color := [4]float32{
					float32(surface.Color[2]) / 255.0, // R (stored as BGR)
					float32(surface.Color[1]) / 255.0, // G
					float32(surface.Color[0]) / 255.0, // B
					float32(surface.Color[3]) / 255.0, // A
				}

				// Calculate lightmap UVs
				lmUV0 := mv.calculateLightmapUV(surface.LightmapID, 0, gnd)
				lmUV1 := mv.calculateLightmapUV(surface.LightmapID, 1, gnd)
				lmUV2 := mv.calculateLightmapUV(surface.LightmapID, 2, gnd)
				lmUV3 := mv.calculateLightmapUV(surface.LightmapID, 3, gnd)

				// Create vertices for quad
				// Try swapped UV mapping: our corners are BL,BR,TL,TR but UV might be TL,TR,BL,BR
				baseIdx := uint32(len(vertices))
				vertices = append(vertices,
					terrainVertex{Position: corners[0], Normal: normal, TexCoord: [2]float32{surface.U[2], surface.V[2]}, LightmapUV: lmUV0, Color: color},
					terrainVertex{Position: corners[1], Normal: normal, TexCoord: [2]float32{surface.U[3], surface.V[3]}, LightmapUV: lmUV1, Color: color},
					terrainVertex{Position: corners[2], Normal: normal, TexCoord: [2]float32{surface.U[0], surface.V[0]}, LightmapUV: lmUV2, Color: color},
					terrainVertex{Position: corners[3], Normal: normal, TexCoord: [2]float32{surface.U[1], surface.V[1]}, LightmapUV: lmUV3, Color: color},
				)

				// Two triangles for quad (diagonal from BL to TR per RO spec)
				textureIndices[texID] = append(textureIndices[texID],
					baseIdx, baseIdx+1, baseIdx+2,
					baseIdx+2, baseIdx+1, baseIdx+3,
				)
			}

			// Front surface (vertical wall facing -Z) - fill gaps between tiles
			nextTile := gnd.GetTile(x, y+1)
			if nextTile != nil {
				heightDiff0 := absf(tile.Altitude[0] - nextTile.Altitude[2])
				heightDiff1 := absf(tile.Altitude[1] - nextTile.Altitude[3])
				if heightDiff0 > 0.001 || heightDiff1 > 0.001 {
					// Wall corners
					wallCorners := [4][3]float32{
						corners[0], // Top-left
						corners[1], // Top-right
						{baseX, -nextTile.Altitude[2], baseZ + tileSize},            // Bottom-left
						{baseX + tileSize, -nextTile.Altitude[3], baseZ + tileSize}, // Bottom-right
					}

					normal := [3]float32{0, 0, -1} // Facing -Z
					color := [4]float32{1.0, 1.0, 1.0, 1.0}
					var texID int
					var texU, texV [4]float32
					var lmID int16

					// Use front surface if available, otherwise use top surface
					if tile.FrontSurface >= 0 && int(tile.FrontSurface) < len(gnd.Surfaces) {
						surface := &gnd.Surfaces[tile.FrontSurface]
						texID = int(surface.TextureID)
						texU = surface.U
						texV = surface.V
						lmID = surface.LightmapID
					} else if tile.TopSurface >= 0 && int(tile.TopSurface) < len(gnd.Surfaces) {
						// Fallback to top surface texture for gap filling
						surface := &gnd.Surfaces[tile.TopSurface]
						texID = int(surface.TextureID)
						texU = [4]float32{0, 1, 0, 1}
						texV = [4]float32{0, 0, 1, 1}
						lmID = surface.LightmapID
					} else {
						continue
					}

					wlmUV0 := mv.calculateLightmapUV(lmID, 0, gnd)
					wlmUV1 := mv.calculateLightmapUV(lmID, 1, gnd)
					wlmUV2 := mv.calculateLightmapUV(lmID, 2, gnd)
					wlmUV3 := mv.calculateLightmapUV(lmID, 3, gnd)

					baseIdx := uint32(len(vertices))
					vertices = append(vertices,
						terrainVertex{Position: wallCorners[0], Normal: normal, TexCoord: [2]float32{texU[0], texV[0]}, LightmapUV: wlmUV0, Color: color},
						terrainVertex{Position: wallCorners[1], Normal: normal, TexCoord: [2]float32{texU[1], texV[1]}, LightmapUV: wlmUV1, Color: color},
						terrainVertex{Position: wallCorners[2], Normal: normal, TexCoord: [2]float32{texU[2], texV[2]}, LightmapUV: wlmUV2, Color: color},
						terrainVertex{Position: wallCorners[3], Normal: normal, TexCoord: [2]float32{texU[3], texV[3]}, LightmapUV: wlmUV3, Color: color},
					)

					textureIndices[texID] = append(textureIndices[texID],
						baseIdx, baseIdx+2, baseIdx+1,
						baseIdx+1, baseIdx+2, baseIdx+3,
					)
				}
			}

			// Right surface (vertical wall facing +X) - fill gaps between tiles
			rightNextTile := gnd.GetTile(x+1, y)
			if rightNextTile != nil {
				heightDiff0 := absf(tile.Altitude[1] - rightNextTile.Altitude[0])
				heightDiff1 := absf(tile.Altitude[3] - rightNextTile.Altitude[2])
				if heightDiff0 > 0.001 || heightDiff1 > 0.001 {
					// Wall corners
					wallCorners := [4][3]float32{
						corners[3], // Top-back
						corners[1], // Top-front
						{baseX + tileSize, -rightNextTile.Altitude[2], baseZ},            // Bottom-back
						{baseX + tileSize, -rightNextTile.Altitude[0], baseZ + tileSize}, // Bottom-front
					}

					normal := [3]float32{1, 0, 0} // Facing +X
					color := [4]float32{1.0, 1.0, 1.0, 1.0}
					var texID int
					var texU, texV [4]float32
					var lmID int16

					// Use right surface if available, otherwise use top surface
					if tile.RightSurface >= 0 && int(tile.RightSurface) < len(gnd.Surfaces) {
						surface := &gnd.Surfaces[tile.RightSurface]
						texID = int(surface.TextureID)
						texU = surface.U
						texV = surface.V
						lmID = surface.LightmapID
					} else if tile.TopSurface >= 0 && int(tile.TopSurface) < len(gnd.Surfaces) {
						// Fallback to top surface texture for gap filling
						surface := &gnd.Surfaces[tile.TopSurface]
						texID = int(surface.TextureID)
						texU = [4]float32{0, 1, 0, 1}
						texV = [4]float32{0, 0, 1, 1}
						lmID = surface.LightmapID
					} else {
						continue
					}

					// Calculate lightmap UVs for wall
					wlmUV0 := mv.calculateLightmapUV(lmID, 0, gnd)
					wlmUV1 := mv.calculateLightmapUV(lmID, 1, gnd)
					wlmUV2 := mv.calculateLightmapUV(lmID, 2, gnd)
					wlmUV3 := mv.calculateLightmapUV(lmID, 3, gnd)

					baseIdx := uint32(len(vertices))
					vertices = append(vertices,
						terrainVertex{Position: wallCorners[0], Normal: normal, TexCoord: [2]float32{texU[0], texV[0]}, LightmapUV: wlmUV0, Color: color},
						terrainVertex{Position: wallCorners[1], Normal: normal, TexCoord: [2]float32{texU[1], texV[1]}, LightmapUV: wlmUV1, Color: color},
						terrainVertex{Position: wallCorners[2], Normal: normal, TexCoord: [2]float32{texU[2], texV[2]}, LightmapUV: wlmUV2, Color: color},
						terrainVertex{Position: wallCorners[3], Normal: normal, TexCoord: [2]float32{texU[3], texV[3]}, LightmapUV: wlmUV3, Color: color},
					)

					textureIndices[texID] = append(textureIndices[texID],
						baseIdx, baseIdx+2, baseIdx+1,
						baseIdx+1, baseIdx+2, baseIdx+3,
					)
				}
			}
		}
	}

	// Build texture groups and final index buffer
	var groups []terrainTextureGroup
	for texID, texIndices := range textureIndices {
		if len(texIndices) == 0 {
			continue
		}
		groups = append(groups, terrainTextureGroup{
			textureID:  texID,
			startIndex: int32(len(indices)),
			indexCount: int32(len(texIndices)),
		})
		indices = append(indices, texIndices...)
	}

	// Smooth normals to eliminate hard edges between tiles
	smoothTerrainNormals(vertices)

	return vertices, indices, groups
}

// uploadTerrainMesh uploads mesh data to GPU.
func (mv *MapViewer) uploadTerrainMesh(vertices []terrainVertex, indices []uint32) {
	if len(vertices) == 0 {
		return
	}

	// Create VAO
	gl.GenVertexArrays(1, &mv.terrainVAO)
	gl.BindVertexArray(mv.terrainVAO)

	// Create VBO
	gl.GenBuffers(1, &mv.terrainVBO)
	gl.BindBuffer(gl.ARRAY_BUFFER, mv.terrainVBO)
	gl.BufferData(gl.ARRAY_BUFFER, len(vertices)*int(unsafe.Sizeof(terrainVertex{})), gl.Ptr(vertices), gl.STATIC_DRAW)

	// Create EBO
	gl.GenBuffers(1, &mv.terrainEBO)
	gl.BindBuffer(gl.ELEMENT_ARRAY_BUFFER, mv.terrainEBO)
	gl.BufferData(gl.ELEMENT_ARRAY_BUFFER, len(indices)*4, gl.Ptr(indices), gl.STATIC_DRAW)

	// Set vertex attributes
	// terrainVertex: Position(12) + Normal(12) + TexCoord(8) + LightmapUV(8) + Color(16) = 56 bytes
	stride := int32(unsafe.Sizeof(terrainVertex{}))

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

// fitCamera positions camera to view entire map.
func (mv *MapViewer) fitCamera() {
	// Calculate map center
	mv.centerX = (mv.minBounds[0] + mv.maxBounds[0]) / 2
	mv.centerY = (mv.minBounds[1] + mv.maxBounds[1]) / 2
	mv.centerZ = (mv.minBounds[2] + mv.maxBounds[2]) / 2

	// Calculate distance based on map size
	sizeX := mv.maxBounds[0] - mv.minBounds[0]
	sizeZ := mv.maxBounds[2] - mv.minBounds[2]
	maxSize := sizeX
	if sizeZ > maxSize {
		maxSize = sizeZ
	}

	// Default zoom distance (proportional to map size)
	mv.Distance = maxSize * 0.3
	if mv.Distance < 200 {
		mv.Distance = 200
	}

	mv.rotationX = 0.6 // Look down at ~35 degrees
	mv.rotationY = 0.0
}

// Render renders the map to the framebuffer and returns the texture ID.
func (mv *MapViewer) Render() uint32 {
	if mv.terrainVAO == 0 {
		return mv.colorTexture
	}

	// Bind framebuffer
	gl.BindFramebuffer(gl.FRAMEBUFFER, mv.fbo)
	gl.Viewport(0, 0, mv.width, mv.height)

	// Clear
	gl.ClearColor(0.4, 0.6, 0.9, 1.0) // Sky blue
	gl.Clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT)

	// Enable depth test
	gl.Enable(gl.DEPTH_TEST)
	gl.DepthFunc(gl.LESS)

	// Calculate view-projection matrix
	aspect := float32(mv.width) / float32(mv.height)
	proj := math.Perspective(45.0, aspect, 1.0, 10000.0)

	var view math.Mat4
	if mv.PlayMode && mv.Player != nil {
		// Play camera - RO-style third-person following player
		player := mv.Player

		// Camera distance (use Distance directly for more zoom range)
		camDistance := mv.Distance
		if camDistance < 100 {
			camDistance = 100
		}
		if camDistance > 800 {
			camDistance = 800
		}

		// Vertical angle (RO-style top-down view, ~45-50 degrees from vertical)
		pitch := float32(0.85) // ~48 degrees - more top-down like RO

		// Calculate camera offset from player using yaw for rotation
		offsetY := camDistance * float32(gomath.Sin(float64(pitch)))
		horizDist := camDistance * float32(gomath.Cos(float64(pitch)))
		offsetX := horizDist * float32(gomath.Sin(float64(mv.camYaw)))
		offsetZ := horizDist * float32(gomath.Cos(float64(mv.camYaw)))

		// Camera position: behind and above player
		camPos := math.Vec3{
			X: player.WorldX - offsetX,
			Y: player.WorldY + offsetY,
			Z: player.WorldZ - offsetZ,
		}

		// Store camera position for sprite direction calculation
		mv.camPosX = camPos.X
		mv.camPosY = camPos.Y
		mv.camPosZ = camPos.Z

		// Look at player position (slightly above feet)
		target := math.Vec3{
			X: player.WorldX,
			Y: player.WorldY + 30, // Look at character center, not feet
			Z: player.WorldZ,
		}

		up := math.Vec3{X: 0, Y: 1, Z: 0}
		view = math.LookAt(camPos, target, up)
	} else if mv.PlayMode {
		// Fallback if no player - use FPS-style camera
		camPos := math.Vec3{X: mv.camPosX, Y: mv.camPosY, Z: mv.camPosZ}
		dirX := float32(cosf(mv.camPitch) * sinf(mv.camYaw))
		dirY := float32(sinf(mv.camPitch))
		dirZ := float32(cosf(mv.camPitch) * cosf(mv.camYaw))
		target := math.Vec3{X: mv.camPosX + dirX, Y: mv.camPosY + dirY, Z: mv.camPosZ + dirZ}
		up := math.Vec3{X: 0, Y: 1, Z: 0}
		view = math.LookAt(camPos, target, up)
	} else {
		// Orbit camera - rotate around center point
		camPos := mv.calculateCameraPosition()
		center := math.Vec3{X: mv.centerX, Y: mv.centerY, Z: mv.centerZ}
		up := math.Vec3{X: 0, Y: 1, Z: 0}
		view = math.LookAt(camPos, center, up)
	}

	viewProj := proj.Mul(view)

	// Cache matrices for picking
	mv.lastView = view
	mv.lastProj = proj
	mv.lastViewProj = viewProj

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

	// Fog uniforms
	if mv.FogEnabled {
		gl.Uniform1i(mv.locFogUse, 1)
	} else {
		gl.Uniform1i(mv.locFogUse, 0)
	}
	gl.Uniform1f(mv.locFogNear, mv.FogNear)
	gl.Uniform1f(mv.locFogFar, mv.FogFar)
	gl.Uniform3f(mv.locFogColor, mv.FogColor[0], mv.FogColor[1], mv.FogColor[2])

	// Bind lightmap atlas to texture unit 1
	gl.ActiveTexture(gl.TEXTURE1)
	if mv.lightmapAtlas != 0 {
		gl.BindTexture(gl.TEXTURE_2D, mv.lightmapAtlas)
	} else {
		gl.BindTexture(gl.TEXTURE_2D, mv.fallbackTex)
	}

	// Bind terrain VAO
	gl.BindVertexArray(mv.terrainVAO)

	// Render each texture group
	gl.ActiveTexture(gl.TEXTURE0)
	for _, group := range mv.terrainGroups {
		tex, ok := mv.groundTextures[group.textureID]
		if !ok {
			tex = mv.fallbackTex
		}
		gl.BindTexture(gl.TEXTURE_2D, tex)
		gl.DrawElementsWithOffset(gl.TRIANGLES, group.indexCount, gl.UNSIGNED_INT, uintptr(group.startIndex*4))
	}

	gl.BindVertexArray(0)

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
	// Y-axis aligned billboard: rotates horizontally to face camera, stays upright
	dirX := mv.camPosX - player.WorldX
	dirZ := mv.camPosZ - player.WorldZ
	length := float32(gomath.Sqrt(float64(dirX*dirX + dirZ*dirZ)))
	if length > 0.001 {
		dirX /= length
		dirZ /= length
	} else {
		dirX = 0
		dirZ = 1
	}
	// Right vector perpendicular to camera direction in XZ plane
	camRight := [3]float32{-dirZ, 0, dirX}
	camUp := [3]float32{0, 1, 0} // World up

	// ========== STEP 2: Calculate visual direction using atan2 algorithm ==========
	// This is the key to the 3D illusion: sprite frame changes based on camera angle

	// Camera angle: from player to camera (in XZ plane)
	cameraAngle := float32(gomath.Atan2(float64(dirX), float64(dirZ)))

	// Player facing angle: convert direction 0-7 to radians
	// RO directions: 0=S, 1=SW, 2=W, 3=NW, 4=N, 5=NE, 6=E, 7=SE
	// S=0 means facing toward default camera (south), which is angle 0
	playerAngle := float32(player.Direction) * (gomath.Pi / 4.0)

	// Combine angles: camera angle + player facing
	combinedAngle := cameraAngle + playerAngle

	// Normalize to 0-2π
	for combinedAngle < 0 {
		combinedAngle += 2 * gomath.Pi
	}
	for combinedAngle >= 2*gomath.Pi {
		combinedAngle -= 2 * gomath.Pi
	}

	// Map to direction index (0-7) using 45° sectors with 22.5° offset
	sector := int((combinedAngle + gomath.Pi/8) / (gomath.Pi / 4))
	if sector >= 8 {
		sector = 0
	}
	visualDir := sector

	// ========== STEP 3: Use composite sprites if available ==========
	// Composite sprites have head+body pre-merged for solid appearance
	if player.UseComposite && player.CompositeFrames != nil {
		actionDirKey := player.CurrentAction*8 + visualDir
		if frames, ok := player.CompositeFrames[actionDirKey]; ok && len(frames) > 0 {
			// For idle action, always use frame 0 to prevent head bobbing
			// Walking uses animated frames
			frameIdx := 0
			if player.CurrentAction == ActionWalk && len(frames) > 1 {
				frameIdx = player.CurrentFrame % len(frames)
			}
			composite := frames[frameIdx]

			if composite.Texture != 0 && composite.Width > 0 && composite.Height > 0 {
				// Calculate sprite size in world units
				spriteWidth := float32(composite.Width) * player.SpriteScale
				spriteHeight := float32(composite.Height) * player.SpriteScale

				// Origin offset: originX/originY is distance from canvas top-left to sprite origin (0,0)
				// originYNorm = how far down from top the origin is (0-1)
				// In quad space: Y=0 is bottom, Y=1 is top
				// UV space: V=0 is top, V=1 is bottom
				// Origin at originYNorm from top means it's at quad Y = (1 - originYNorm)
				// We want origin at player feet (Y=0 of quad base at player.WorldY)
				// So shift DOWN by (1 - originYNorm) * spriteHeight
				originXNorm := float32(composite.OriginX) / float32(composite.Width)
				originYNorm := float32(composite.OriginY) / float32(composite.Height)

				// Vertical offset: shift DOWN so origin is at player feet
				// Origin is at (1-originYNorm) from bottom of quad, we want it at 0
				// Add small lift (10%) to keep feet above ground
				offsetY := -(1.0-originYNorm)*spriteHeight + spriteHeight*0.10

				// Mirror for left-facing directions
				mirrorScale := float32(1.0)
				if visualDir == DirSW || visualDir == DirW || visualDir == DirNW {
					mirrorScale = -1.0
					// When mirrored, origin at originXNorm from left appears at (1-originXNorm)
					// So use mirrored origin for offset calculation
					originXNorm = 1.0 - originXNorm
				}

				// Horizontal offset: shift so origin X is at center
				offsetX := (0.5 - originXNorm) * spriteWidth

				gl.Enable(gl.BLEND)
				gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)
				gl.UseProgram(mv.spriteProgram)

				// Apply offsets along billboard vectors
				posX := player.WorldX + offsetX*camRight[0]
				posY := player.WorldY + offsetY
				posZ := player.WorldZ + offsetX*camRight[2]

				gl.UniformMatrix4fv(mv.locSpriteVP, 1, false, &viewProj[0])
				gl.Uniform3f(mv.locSpritePos, posX, posY, posZ)
				gl.Uniform2f(mv.locSpriteSize, spriteWidth*mirrorScale, spriteHeight)
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
	if visualDir == DirSW || visualDir == DirW || visualDir == DirNW {
		spriteWidth = -spriteWidth
	}

	gl.Enable(gl.BLEND)
	gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)
	gl.UseProgram(mv.spriteProgram)

	gl.UniformMatrix4fv(mv.locSpriteVP, 1, false, &viewProj[0])
	gl.Uniform3f(mv.locSpritePos, player.WorldX, player.WorldY, player.WorldZ)
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

					if visualDir == DirSW || visualDir == DirW || visualDir == DirNW {
						headWidth = -headWidth
					}

					layerX := float32(headLayer.X) * player.SpriteScale
					layerY := float32(headLayer.Y) * player.SpriteScale

					totalOffsetX := offsetX + layerX

					headPosX := player.WorldX + totalOffsetX*camRight[0]
					headPosY := player.WorldY - (offsetY + layerY) + (bodyLayerY * player.SpriteScale * 0.35)
					headPosZ := player.WorldZ + totalOffsetX*camRight[2]

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
	shadowY := player.WorldY + 0.5

	// Set uniforms - position the shadow flat on the ground
	gl.UniformMatrix4fv(mv.locSpriteVP, 1, false, &viewProj[0])
	gl.Uniform3f(mv.locSpritePos, player.WorldX, shadowY, player.WorldZ)
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
	if mv.Player == nil {
		return
	}

	player := mv.Player

	// Procedural players don't have animation data
	if player.ACT == nil {
		return
	}

	// Determine action based on movement state
	newAction := ActionIdle
	if player.IsMoving {
		newAction = ActionWalk
	}

	// Reset frame when action changes
	if newAction != player.CurrentAction {
		player.CurrentAction = newAction
		player.CurrentFrame = 0
		player.FrameTime = 0
	}

	// Get current action
	actionIdx := player.CurrentAction*8 + player.Direction
	if actionIdx >= len(player.ACT.Actions) {
		actionIdx = 0
	}
	action := &player.ACT.Actions[actionIdx]
	if len(action.Frames) == 0 {
		return
	}

	// Get animation interval from ACT (default 150ms for smoother animation)
	interval := float32(150.0)
	if actionIdx < len(player.ACT.Intervals) && player.ACT.Intervals[actionIdx] > 0 {
		interval = player.ACT.Intervals[actionIdx]
		// ACT intervals can be very small, enforce minimum
		if interval < 50 {
			interval = 50
		}
	}

	// Accumulate time
	player.FrameTime += deltaMs
	if player.FrameTime >= interval {
		player.FrameTime -= interval
		player.CurrentFrame++
		if player.CurrentFrame >= len(action.Frames) {
			player.CurrentFrame = 0 // Loop animation
		}
	}
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
		// At speed 10, cycle through 32 frames in ~3 seconds
		frameTime := mv.waterTime * mv.waterAnimSpeed * 0.5
		mv.waterFrame = int(frameTime) % len(mv.waterTextures)

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
	worldX := model.position[0] + offsetX
	worldY := -model.position[1]
	worldZ := model.position[2] + offsetZ

	// Get model bounding box (local space, already centered)
	minX := model.bbox[0] * model.scale[0]
	minY := model.bbox[1] * model.scale[1]
	minZ := model.bbox[2] * model.scale[2]
	maxX := model.bbox[3] * model.scale[0]
	maxY := model.bbox[4] * model.scale[1]
	maxZ := model.bbox[5] * model.scale[2]

	// Handle negative scales
	if minX > maxX {
		minX, maxX = maxX, minX
	}
	if minY > maxY {
		minY, maxY = maxY, minY
	}
	if minZ > maxZ {
		minZ, maxZ = maxZ, minZ
	}

	// Expand box slightly for visibility
	pad := float32(1.0)
	minX -= pad
	minY -= pad
	minZ -= pad
	maxX += pad
	maxY += pad
	maxZ += pad

	// Transform to world space
	minX += worldX
	minY += worldY
	minZ += worldZ
	maxX += worldX
	maxY += worldY
	maxZ += worldZ

	// Build line vertices for 12 edges of the box
	vertices := []float32{
		// Bottom face
		minX, minY, minZ, maxX, minY, minZ,
		maxX, minY, minZ, maxX, minY, maxZ,
		maxX, minY, maxZ, minX, minY, maxZ,
		minX, minY, maxZ, minX, minY, minZ,
		// Top face
		minX, maxY, minZ, maxX, maxY, minZ,
		maxX, maxY, minZ, maxX, maxY, maxZ,
		maxX, maxY, maxZ, minX, maxY, maxZ,
		minX, maxY, maxZ, minX, maxY, minZ,
		// Vertical edges
		minX, minY, minZ, minX, maxY, minZ,
		maxX, minY, minZ, maxX, maxY, minZ,
		maxX, minY, maxZ, maxX, maxY, maxZ,
		minX, minY, maxZ, minX, maxY, maxZ,
	}

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

	// Convert screen coords to normalized device coords (-1 to 1)
	ndcX := (2.0*screenX/viewWidth - 1.0)
	ndcY := (1.0 - 2.0*screenY/viewHeight) // Flip Y

	// Create ray from camera through the click point
	// Unproject near and far points
	invViewProj := mv.lastViewProj.Inverse()

	nearPoint := math.Vec4{ndcX, ndcY, -1.0, 1.0}
	farPoint := math.Vec4{ndcX, ndcY, 1.0, 1.0}

	nearWorld := invViewProj.MulVec4(nearPoint)
	farWorld := invViewProj.MulVec4(farPoint)

	// Perspective divide
	if nearWorld[3] != 0 {
		nearWorld[0] /= nearWorld[3]
		nearWorld[1] /= nearWorld[3]
		nearWorld[2] /= nearWorld[3]
	}
	if farWorld[3] != 0 {
		farWorld[0] /= farWorld[3]
		farWorld[1] /= farWorld[3]
		farWorld[2] /= farWorld[3]
	}

	rayOrigin := [3]float32{nearWorld[0], nearWorld[1], nearWorld[2]}
	rayDir := [3]float32{
		farWorld[0] - nearWorld[0],
		farWorld[1] - nearWorld[1],
		farWorld[2] - nearWorld[2],
	}

	// Normalize ray direction
	rayLen := float32(gomath.Sqrt(float64(rayDir[0]*rayDir[0] + rayDir[1]*rayDir[1] + rayDir[2]*rayDir[2])))
	if rayLen > 0 {
		rayDir[0] /= rayLen
		rayDir[1] /= rayLen
		rayDir[2] /= rayLen
	}

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
		worldX := model.position[0] + offsetX
		worldY := -model.position[1]
		worldZ := model.position[2] + offsetZ

		minX := model.bbox[0]*model.scale[0] + worldX
		minY := model.bbox[1]*model.scale[1] + worldY
		minZ := model.bbox[2]*model.scale[2] + worldZ
		maxX := model.bbox[3]*model.scale[0] + worldX
		maxY := model.bbox[4]*model.scale[1] + worldY
		maxZ := model.bbox[5]*model.scale[2] + worldZ

		// Handle negative scales
		if minX > maxX {
			minX, maxX = maxX, minX
		}
		if minY > maxY {
			minY, maxY = maxY, minY
		}
		if minZ > maxZ {
			minZ, maxZ = maxZ, minZ
		}

		// Ray-AABB intersection test
		tmin := float32(-gomath.MaxFloat32)
		tmax := float32(gomath.MaxFloat32)

		// X slab
		if rayDir[0] != 0 {
			t1 := (minX - rayOrigin[0]) / rayDir[0]
			t2 := (maxX - rayOrigin[0]) / rayDir[0]
			if t1 > t2 {
				t1, t2 = t2, t1
			}
			if t1 > tmin {
				tmin = t1
			}
			if t2 < tmax {
				tmax = t2
			}
		} else if rayOrigin[0] < minX || rayOrigin[0] > maxX {
			continue
		}

		// Y slab
		if rayDir[1] != 0 {
			t1 := (minY - rayOrigin[1]) / rayDir[1]
			t2 := (maxY - rayOrigin[1]) / rayDir[1]
			if t1 > t2 {
				t1, t2 = t2, t1
			}
			if t1 > tmin {
				tmin = t1
			}
			if t2 < tmax {
				tmax = t2
			}
		} else if rayOrigin[1] < minY || rayOrigin[1] > maxY {
			continue
		}

		// Z slab
		if rayDir[2] != 0 {
			t1 := (minZ - rayOrigin[2]) / rayDir[2]
			t2 := (maxZ - rayOrigin[2]) / rayDir[2]
			if t1 > t2 {
				t1, t2 = t2, t1
			}
			if t1 > tmin {
				tmin = t1
			}
			if t2 < tmax {
				tmax = t2
			}
		} else if rayOrigin[2] < minZ || rayOrigin[2] > maxZ {
			continue
		}

		// Check if intersection is valid
		if tmax >= tmin && tmax >= 0 {
			hitDist := tmin
			if hitDist < 0 {
				hitDist = tmax
			}
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

	// Fog uniforms for models
	if mv.FogEnabled {
		gl.Uniform1i(mv.locModelFogUse, 1)
	} else {
		gl.Uniform1i(mv.locModelFogUse, 0)
	}
	gl.Uniform1f(mv.locModelFogNear, mv.FogNear)
	gl.Uniform1f(mv.locModelFogFar, mv.FogFar)
	gl.Uniform3f(mv.locModelFogColor, mv.FogColor[0], mv.FogColor[1], mv.FogColor[2])

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

		// Apply scale
		modelMatrix = modelMatrix.Mul(math.Scale(model.scale[0], model.scale[1], model.scale[2]))

		// Combine with view-projection
		mvp := viewProj.Mul(modelMatrix)
		gl.UniformMatrix4fv(mv.locModelMVP, 1, false, &mvp[0])

		gl.BindVertexArray(model.vao)

		// Render each texture group
		for _, group := range model.texGroups {
			tex := mv.fallbackTex
			if group.texIdx >= 0 && group.texIdx < len(model.textures) {
				tex = model.textures[group.texIdx]
			}
			gl.BindTexture(gl.TEXTURE_2D, tex)
			gl.DrawElementsWithOffset(gl.TRIANGLES, group.indexCount, gl.UNSIGNED_INT, uintptr(group.startIndex*4))
		}
	}

	gl.BindVertexArray(0)
}

// calculateCameraPosition computes camera position from orbit parameters.
func (mv *MapViewer) calculateCameraPosition() math.Vec3 {
	// Spherical to Cartesian
	x := mv.Distance * float32(cosf(mv.rotationX)*sinf(mv.rotationY))
	y := mv.Distance * float32(sinf(mv.rotationX))
	z := mv.Distance * float32(cosf(mv.rotationX)*cosf(mv.rotationY))

	return math.Vec3{
		X: mv.centerX + x,
		Y: mv.centerY + y,
		Z: mv.centerZ + z,
	}
}

// HandleMouseDrag handles mouse drag for camera rotation.
func (mv *MapViewer) HandleMouseDrag(deltaX, deltaY float32) {
	sensitivity := float32(0.005)

	if mv.PlayMode {
		// Play mode - rotate camera around player (horizontal only)
		mv.camYaw -= deltaX * sensitivity
		// Note: We don't adjust pitch in Play mode - fixed viewing angle
	} else {
		// Orbit mode - rotate around center
		mv.rotationY -= deltaX * sensitivity
		mv.rotationX += deltaY * sensitivity

		// Clamp pitch
		if mv.rotationX < 0.1 {
			mv.rotationX = 0.1
		}
		if mv.rotationX > 1.5 {
			mv.rotationX = 1.5
		}
	}
}

// HandleMouseWheel handles mouse scroll for zoom.
func (mv *MapViewer) HandleMouseWheel(delta float32) {
	// Both Play mode and Orbit mode use Distance for zoom
	mv.Distance -= delta * mv.Distance * 0.1
	if mv.Distance < 50 {
		mv.Distance = 50
	}
	if mv.Distance > 5000 {
		mv.Distance = 5000
	}
}

// HandlePlayMovement handles WASD movement in Play mode.
// forward/right are -1, 0, or 1 based on key presses.
func (mv *MapViewer) HandlePlayMovement(forward, right, up float32) {
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
	camDirX := float32(sinf(mv.camYaw))
	camDirZ := float32(cosf(mv.camYaw))

	// Right direction (perpendicular to forward) - negated for correct A/D
	camRightX := float32(-cosf(mv.camYaw))
	camRightZ := float32(sinf(mv.camYaw))

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
	mv.Player.Direction = calculateDirection(-moveX, -moveZ)

	// Set walk animation
	mv.Player.CurrentAction = ActionWalk
}

// UpdatePlayerMovement updates player position for click-to-move navigation.
// Called each frame to move player toward destination.
func (mv *MapViewer) UpdatePlayerMovement(deltaMs float32) {
	if !mv.PlayMode || mv.Player == nil || !mv.Player.HasDestination {
		return
	}

	player := mv.Player

	// Calculate direction to destination
	dx := player.DestX - player.WorldX
	dz := player.DestZ - player.WorldZ
	dist := float32(gomath.Sqrt(float64(dx*dx + dz*dz)))

	// Check if reached destination
	if dist < 1.0 {
		player.HasDestination = false
		player.IsMoving = false
		player.CurrentAction = ActionIdle
		return
	}

	// Normalize direction
	dx /= dist
	dz /= dist

	// Calculate movement amount
	moveAmount := player.MoveSpeed * deltaMs / 1000.0
	if moveAmount > dist {
		moveAmount = dist
	}

	// Calculate new position
	newX := player.WorldX + dx*moveAmount
	newZ := player.WorldZ + dz*moveAmount

	// Check if new position is walkable
	if mv.IsWalkable(newX, newZ) {
		player.WorldX = newX
		player.WorldZ = newZ
		player.WorldY = mv.GetInterpolatedTerrainHeight(newX, newZ)
	} else {
		// Stop if hit obstacle
		player.HasDestination = false
		player.IsMoving = false
		player.CurrentAction = ActionIdle
		return
	}

	// Update facing direction
	player.Direction = calculateDirection(dx, dz)
	player.IsMoving = true
	player.CurrentAction = ActionWalk
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

	// Set destination
	mv.Player.DestX = worldX
	mv.Player.DestZ = worldZ
	mv.Player.HasDestination = true
}

// ScreenToWorld converts screen coordinates to world XZ position by intersecting with ground plane.
// Uses proper matrix unprojection like PickModelAtScreen.
func (mv *MapViewer) ScreenToWorld(screenX, screenY, viewportW, viewportH float32) (worldX, worldZ float32, ok bool) {
	// Convert screen coords to normalized device coords (-1 to 1)
	ndcX := (2.0*screenX/viewportW - 1.0)
	ndcY := (1.0 - 2.0*screenY/viewportH) // Flip Y

	// Create ray from camera through the click point using inverse view-projection
	invViewProj := mv.lastViewProj.Inverse()

	nearPoint := math.Vec4{ndcX, ndcY, -1.0, 1.0}
	farPoint := math.Vec4{ndcX, ndcY, 1.0, 1.0}

	nearWorld := invViewProj.MulVec4(nearPoint)
	farWorld := invViewProj.MulVec4(farPoint)

	// Perspective divide
	if nearWorld[3] != 0 {
		nearWorld[0] /= nearWorld[3]
		nearWorld[1] /= nearWorld[3]
		nearWorld[2] /= nearWorld[3]
	}
	if farWorld[3] != 0 {
		farWorld[0] /= farWorld[3]
		farWorld[1] /= farWorld[3]
		farWorld[2] /= farWorld[3]
	}

	rayOrigin := [3]float32{nearWorld[0], nearWorld[1], nearWorld[2]}
	rayDir := [3]float32{
		farWorld[0] - nearWorld[0],
		farWorld[1] - nearWorld[1],
		farWorld[2] - nearWorld[2],
	}

	// Normalize ray direction
	rayLen := float32(gomath.Sqrt(float64(rayDir[0]*rayDir[0] + rayDir[1]*rayDir[1] + rayDir[2]*rayDir[2])))
	if rayLen > 0 {
		rayDir[0] /= rayLen
		rayDir[1] /= rayLen
		rayDir[2] /= rayLen
	}

	// Intersect with ground plane (Y = player height or terrain)
	groundY := float32(0)
	if mv.Player != nil {
		groundY = mv.Player.WorldY
	}

	// Ray: P = rayOrigin + t * rayDir
	// Plane: Y = groundY
	// Solve: rayOrigin.Y + t * rayDir.Y = groundY
	if gomath.Abs(float64(rayDir[1])) < 0.001 {
		return 0, 0, false // Ray parallel to ground
	}

	t := (groundY - rayOrigin[1]) / rayDir[1]
	if t < 0 {
		return 0, 0, false // Intersection behind camera
	}

	worldX = rayOrigin[0] + t*rayDir[0]
	worldZ = rayOrigin[2] + t*rayDir[2]

	fmt.Printf("Click: screen(%.0f,%.0f) -> world(%.1f, %.1f)\n", screenX, screenY, worldX, worldZ)

	return worldX, worldZ, true
}

// HandleOrbitMovement handles WASD movement in Orbit mode.
// Pans the camera's focal point (center).
// forward/right are -1, 0, or 1 based on key presses.
func (mv *MapViewer) HandleOrbitMovement(forward, right, up float32) {
	if mv.PlayMode {
		return
	}

	// Speed scales with distance for consistent feel
	speed := mv.Distance * 0.01

	// Calculate movement direction based on current camera rotation
	// Forward direction in world space (based on rotationY)
	dirX := float32(sinf(mv.rotationY))
	dirZ := float32(cosf(mv.rotationY))

	// Right direction (perpendicular to forward)
	rightX := float32(cosf(mv.rotationY))
	rightZ := float32(-sinf(mv.rotationY))

	// Apply movement to center point (negate forward so W moves "into" the scene)
	mv.centerX += (-dirX*forward + rightX*right) * speed
	mv.centerZ += (-dirZ*forward + rightZ*right) * speed
	mv.centerY += up * speed
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
	player := &PlayerCharacter{
		SPR:         spr,
		ACT:         act,
		SpriteScale: 0.28, // Scale down sprite pixels to world units
		MoveSpeed:   50.0, // World units per second // World units per second
		Direction:   DirS,  // Face south (camera) initially
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
	player := &PlayerCharacter{
		SPR:         spr,
		ACT:         act,
		SpriteScale: 0.28, // Scale down sprite pixels to world units
		MoveSpeed:   50.0, // World units per second
		Direction:   DirS,
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
		player.CompositeFrames = make(map[int][]CompositeFrame)

		// Generate composites for actions 0 (idle) and 1 (walk)
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

				// First, compute frame 0's reference origin for consistent positioning
				_, _, _, refOX, refOY := compositeSprites(spr, act, player.HeadSPR, player.HeadACT, action, dir, 0)

				frames := make([]CompositeFrame, numFrames)
				for frame := 0; frame < numFrames; frame++ {
					pixels, w, h, _, _ := compositeSprites(spr, act, player.HeadSPR, player.HeadACT, action, dir, frame)
					if pixels == nil || w == 0 || h == 0 {
						continue
					}

					// Use frame 0's origin for ALL frames to prevent position shifting
					// This keeps the character grounded consistently across all walk frames
					useOX, useOY := refOX, refOY

					// Create GPU texture for composite
					var tex uint32
					gl.GenTextures(1, &tex)
					gl.BindTexture(gl.TEXTURE_2D, tex)
					gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA8, int32(w), int32(h), 0,
						gl.RGBA, gl.UNSIGNED_BYTE, gl.Ptr(pixels))
					gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.NEAREST)
					gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.NEAREST)
					gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
					gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)

					frames[frame] = CompositeFrame{
						Texture: tex,
						Width:   w,
						Height:  h,
						OriginX: useOX,
						OriginY: useOY,
					}
				}
				player.CompositeFrames[actionDirKey] = frames
			}
		}
		player.UseComposite = true
		fmt.Printf("Generated %d composite frame sets\n", len(player.CompositeFrames))
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
	// Create circular shadow texture (24x24 pixels)
	size := 24
	pixels := make([]byte, size*size*4)

	center := float32(size) / 2
	radius := float32(size)/2 - 1

	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			idx := (y*size + x) * 4
			// Calculate distance from center (circle equation)
			dx := (float32(x) - center) / radius
			dy := (float32(y) - center) / radius
			dist := dx*dx + dy*dy

			if dist <= 1.0 {
				// Inside circle - light shadow with soft falloff
				alpha := (1.0 - dist) * 0.25 // Max 25% opacity, fading to edge
				pixels[idx+0] = 0            // R
				pixels[idx+1] = 0            // G
				pixels[idx+2] = 0            // B
				pixels[idx+3] = byte(alpha * 255)
			}
		}
	}

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
	shadowSize := float32(4.0) // Shadow size in world units (smaller, circular)
	shadowVerts := []float32{
		// Position (x, z)  TexCoord (u, v)
		-shadowSize, -shadowSize, 0.0, 0.0,
		shadowSize, -shadowSize, 1.0, 0.0,
		-shadowSize, shadowSize, 0.0, 1.0,
		shadowSize, shadowSize, 1.0, 1.0,
	}

	gl.BufferData(gl.ARRAY_BUFFER, len(shadowVerts)*4, gl.Ptr(shadowVerts), gl.STATIC_DRAW)
	gl.VertexAttribPointerWithOffset(0, 2, gl.FLOAT, false, 4*4, 0)
	gl.EnableVertexAttribArray(0)
	gl.VertexAttribPointerWithOffset(1, 2, gl.FLOAT, false, 4*4, 2*4)
	gl.EnableVertexAttribArray(1)
	gl.BindVertexArray(0)
}

// createProceduralPlayer creates a simple colored player marker when no sprite is available.
func (mv *MapViewer) createProceduralPlayer() error {
	// Create a simple 32x64 colored texture (blue player marker)
	width, height := 32, 64
	pixels := make([]byte, width*height*4)

	// Fill with semi-transparent blue color
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			idx := (y*width + x) * 4
			// Create a simple humanoid shape
			centerX := width / 2
			distFromCenter := x - centerX
			if distFromCenter < 0 {
				distFromCenter = -distFromCenter
			}

			// Head (top 1/4)
			if y < height/4 && distFromCenter < width/4 {
				pixels[idx+0] = 100  // R
				pixels[idx+1] = 150  // G
				pixels[idx+2] = 255  // B
				pixels[idx+3] = 255  // A
			} else if y >= height/4 && y < height*3/4 && distFromCenter < width/3 {
				// Body (middle half)
				pixels[idx+0] = 50   // R
				pixels[idx+1] = 100  // G
				pixels[idx+2] = 200  // B
				pixels[idx+3] = 255  // A
			} else if y >= height*3/4 && distFromCenter < width/4 {
				// Legs (bottom quarter)
				pixels[idx+0] = 50  // R
				pixels[idx+1] = 80  // G
				pixels[idx+2] = 150 // B
				pixels[idx+3] = 255 // A
			} else {
				// Transparent
				pixels[idx+3] = 0
			}
		}
	}

	// Create player character
	player := &PlayerCharacter{
		SpriteScale: 0.4, // Reasonable scale for character size (~13x26 world units)
		MoveSpeed:   50.0, // World units per second
		Direction:   DirS,
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

	// Set player position
	mv.Player.WorldX = centerX
	mv.Player.WorldZ = centerZ
	mv.Player.WorldY = mv.GetInterpolatedTerrainHeight(centerX, centerZ)

	fmt.Printf("Player spawned at (%.0f, %.0f, %.0f)\n", mv.Player.WorldX, mv.Player.WorldY, mv.Player.WorldZ)
}

// GetInterpolatedTerrainHeight returns the terrain height at a world position.
// Uses bilinear interpolation between GAT cell corner heights.
func (mv *MapViewer) GetInterpolatedTerrainHeight(worldX, worldZ float32) float32 {
	if mv.GAT == nil {
		return 0
	}

	// Convert world coordinates to GAT cell coordinates
	// GAT cells are 5x5 world units (half of GND tile size which is 10)
	cellSize := float32(5.0)
	cellFX := worldX / cellSize
	cellFZ := worldZ / cellSize

	cellX := int(cellFX)
	cellZ := int(cellFZ)

	// Clamp to valid range
	if cellX < 0 {
		cellX = 0
	}
	if cellZ < 0 {
		cellZ = 0
	}
	if cellX >= int(mv.GAT.Width)-1 {
		cellX = int(mv.GAT.Width) - 2
	}
	if cellZ >= int(mv.GAT.Height)-1 {
		cellZ = int(mv.GAT.Height) - 2
	}

	// Get fractional position within cell (0-1)
	fracX := cellFX - float32(cellX)
	fracZ := cellFZ - float32(cellZ)
	if fracX < 0 {
		fracX = 0
	}
	if fracX > 1 {
		fracX = 1
	}
	if fracZ < 0 {
		fracZ = 0
	}
	if fracZ > 1 {
		fracZ = 1
	}

	// Get cell heights (corners: 0=BL, 1=BR, 2=TL, 3=TR)
	cell := mv.GAT.GetCell(cellX, cellZ)
	if cell == nil {
		return 0
	}

	// Bilinear interpolation
	// Bottom edge: lerp between bottom-left and bottom-right
	bottom := cell.Heights[0]*(1-fracX) + cell.Heights[1]*fracX
	// Top edge: lerp between top-left and top-right
	top := cell.Heights[2]*(1-fracX) + cell.Heights[3]*fracX
	// Final: lerp between bottom and top edges
	height := bottom*(1-fracZ) + top*fracZ

	// GAT heights are typically negative (lower = higher in RO coordinate system)
	return -height
}

// IsWalkable checks if a world position is walkable.
func (mv *MapViewer) IsWalkable(worldX, worldZ float32) bool {
	if mv.GAT == nil {
		return true // No GAT data, allow movement
	}

	// Convert world coordinates to GAT cell coordinates
	cellSize := float32(5.0)
	cellX := int(worldX / cellSize)
	cellZ := int(worldZ / cellSize)

	return mv.GAT.IsWalkable(cellX, cellZ)
}

// TogglePlayMode toggles between orbit and play camera modes.
func (mv *MapViewer) TogglePlayMode() {
	mv.PlayMode = !mv.PlayMode

	if mv.PlayMode {
		// Set appropriate zoom distance for Play mode (RO-style)
		mv.Distance = 145 // Good starting distance for third-person

		// Reset camera yaw (rotation around player)
		mv.camYaw = 0

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
		mv.camYaw = 0
		mv.Distance = 145 // Default Play mode distance

		// Reset player to map center
		if mv.Player != nil {
			mv.initializePlayerPosition()
		}
	} else {
		mv.rotationX = 0.6
		mv.rotationY = 0.0
	}
}

// updateBounds expands bounds to include point.
func (mv *MapViewer) updateBounds(p [3]float32) {
	for i := 0; i < 3; i++ {
		if p[i] < mv.minBounds[i] {
			mv.minBounds[i] = p[i]
		}
		if p[i] > mv.maxBounds[i] {
			mv.maxBounds[i] = p[i]
		}
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
	gl.BufferData(gl.ARRAY_BUFFER, len(vertices)*int(unsafe.Sizeof(modelVertex{})), gl.Ptr(vertices), gl.DYNAMIC_DRAW)

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

// Helper functions

func cross(a, b [3]float32) [3]float32 {
	return [3]float32{
		a[1]*b[2] - a[2]*b[1],
		a[2]*b[0] - a[0]*b[2],
		a[0]*b[1] - a[1]*b[0],
	}
}

func normalize(v [3]float32) [3]float32 {
	len := sqrtf(v[0]*v[0] + v[1]*v[1] + v[2]*v[2])
	if len < 0.0001 {
		return [3]float32{0, 1, 0}
	}
	return [3]float32{v[0] / len, v[1] / len, v[2] / len}
}

func sqrtf(x float32) float32 {
	return float32(gomath.Sqrt(float64(x)))
}

func cosf(x float32) float64 {
	return gomath.Cos(float64(x))
}

func sinf(x float32) float64 {
	return gomath.Sin(float64(x))
}

func absf(x float32) float32 {
	if x < 0 {
		return -x
	}
	return x
}

// buildTerrainHeightMap builds a lookup table of terrain heights for model positioning.
func (mv *MapViewer) buildTerrainHeightMap(gnd *formats.GND) {
	mv.terrainTilesX = int(gnd.Width)
	mv.terrainTilesZ = int(gnd.Height)
	mv.terrainTileZoom = gnd.Zoom

	// Allocate 2D array for terrain heights
	mv.terrainAltitudes = make([][]float32, mv.terrainTilesX)
	for x := 0; x < mv.terrainTilesX; x++ {
		mv.terrainAltitudes[x] = make([]float32, mv.terrainTilesZ)
		for z := 0; z < mv.terrainTilesZ; z++ {
			tile := gnd.GetTile(x, z)
			if tile != nil {
				// Average of 4 corners
				avgAlt := (tile.Altitude[0] + tile.Altitude[1] + tile.Altitude[2] + tile.Altitude[3]) / 4.0
				mv.terrainAltitudes[x][z] = avgAlt
			}
		}
	}
}

// createWaterPlane creates a water surface plane at the specified height.
func (mv *MapViewer) createWaterPlane(_ *formats.GND, waterLevel float32) {
	// Delete old water if exists
	if mv.waterVAO != 0 {
		gl.DeleteVertexArrays(1, &mv.waterVAO)
		gl.DeleteBuffers(1, &mv.waterVBO)
	}

	// Water level in RSW is typically positive for below ground level
	// Convert to our Y-up coordinate system
	waterY := -waterLevel

	// Create a large water plane covering the map
	// Extend slightly beyond map bounds
	padding := float32(50.0)
	minX := mv.minBounds[0] - padding
	maxX := mv.maxBounds[0] + padding
	minZ := mv.minBounds[2] - padding
	maxZ := mv.maxBounds[2] + padding

	// Simple quad vertices (position only)
	vertices := []float32{
		minX, waterY, minZ,
		maxX, waterY, minZ,
		maxX, waterY, maxZ,
		minX, waterY, maxZ,
	}

	// Create VAO/VBO
	gl.GenVertexArrays(1, &mv.waterVAO)
	gl.GenBuffers(1, &mv.waterVBO)

	gl.BindVertexArray(mv.waterVAO)
	gl.BindBuffer(gl.ARRAY_BUFFER, mv.waterVBO)
	gl.BufferData(gl.ARRAY_BUFFER, len(vertices)*4, gl.Ptr(vertices), gl.STATIC_DRAW)

	// Position attribute
	gl.EnableVertexAttribArray(0)
	gl.VertexAttribPointerWithOffset(0, 3, gl.FLOAT, false, 3*4, 0)

	gl.BindVertexArray(0)

	mv.waterLevel = waterLevel
	mv.hasWater = true
}

// calculateSunDirection converts RSW spherical coordinates (longitude, latitude in degrees)
// to a directional light vector. This matches how the RO client interprets the sun position.
// Longitude: azimuth angle (0-360), horizontal rotation around Y axis
// Latitude: elevation angle (0-90), 0 = horizon, 90 = directly overhead
func calculateSunDirection(longitude, latitude int32) [3]float32 {
	// Convert degrees to radians
	lonRad := float64(longitude) * gomath.Pi / 180.0
	latRad := float64(latitude) * gomath.Pi / 180.0

	// Spherical to Cartesian conversion
	// The sun direction points FROM the sun TO the surface (towards origin)
	// Latitude: 0 = horizon, 90 = directly overhead
	// Longitude: angle around Y axis
	x := float32(gomath.Cos(latRad) * gomath.Sin(lonRad))
	y := float32(gomath.Sin(latRad))
	z := float32(gomath.Cos(latRad) * gomath.Cos(lonRad))

	return [3]float32{x, y, z}
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
