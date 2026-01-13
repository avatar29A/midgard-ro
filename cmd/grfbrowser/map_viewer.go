// 3D map viewer for GND/RSW files (ADR-013 Stage 1).
package main

import (
	"fmt"
	gomath "math"
	"unsafe"

	"github.com/go-gl/gl/v4.1-core/gl"

	"github.com/Faultbox/midgard-ro/pkg/formats"
	"github.com/Faultbox/midgard-ro/pkg/math"
)

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
}

// modelTexGroup groups faces by texture for rendering.
type modelTexGroup struct {
	texIdx     int
	startIndex int32
	indexCount int32
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
	terrainProgram uint32
	locViewProj    int32
	locLightDir    int32
	locAmbient     int32
	locDiffuse     int32
	locTexture     int32
	locLightmap    int32

	// Model shader
	modelProgram     uint32
	locModelMVP      int32
	locModelLightDir int32
	locModelAmbient  int32
	locModelDiffuse  int32
	locModelTexture  int32

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
	models []*MapModel

	// Camera - Orbit mode
	rotationX float32
	rotationY float32
	distance  float32
	centerX   float32
	centerY   float32
	centerZ   float32

	// Camera - FPS mode
	FPSMode   bool
	camPosX   float32
	camPosY   float32
	camPosZ   float32
	camYaw    float32 // Horizontal angle (radians)
	camPitch  float32 // Vertical angle (radians)
	MoveSpeed float32

	// Lighting from RSW
	lightDir     [3]float32 // Calculated from longitude/latitude
	ambientColor [3]float32 // From RSW.Light.Ambient
	diffuseColor [3]float32 // From RSW.Light.Diffuse

	// Map bounds
	minBounds [3]float32
	maxBounds [3]float32

	// Map dimensions for coordinate conversion
	mapWidth  float32 // Width in world units (tiles * zoom)
	mapHeight float32 // Height in world units (tiles * zoom)
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

out vec4 FragColor;

void main() {
    vec4 texColor = texture(uTexture, vTexCoord);
    vec3 lightmapColor = texture(uLightmap, vLightmapUV).rgb;

    // Proper lighting calculation using RSW data
    // Directional light component (sun)
    vec3 normal = normalize(vNormal);
    vec3 lightDir = normalize(uLightDir);
    float NdotL = max(dot(normal, lightDir), 0.0);
    vec3 directional = uDiffuse * NdotL;

    // Combine ambient + directional lighting
    vec3 lighting = uAmbient + directional;

    // Apply lightmap (pre-baked shadows and color from point lights)
    // Lightmaps in RO contain baked lighting - they should ADD to the scene
    // not darken it completely. Mix between full light and lightmap.
    vec3 adjustedLightmap = mix(vec3(0.5), lightmapColor, 0.8);

    // Combine: texture * dynamic lighting * baked lightmap * vertex color
    vec3 finalColor = texColor.rgb * lighting * adjustedLightmap * vColor.rgb;

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

out vec4 FragColor;

void main() {
    vec4 texColor = texture(uTexture, vTexCoord);

    // Simple lighting
    float NdotL = max(dot(normalize(vNormal), normalize(uLightDir)), 0.0);
    vec3 lighting = uAmbient + uDiffuse * NdotL;

    vec3 color = texColor.rgb * lighting;
    FragColor = vec4(color, texColor.a);
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
		distance:       500.0,
		MoveSpeed:      5.0,
		// Default lighting (will be overwritten by RSW data)
		lightDir:     [3]float32{0.5, 0.866, 0.0}, // 60 degrees elevation
		ambientColor: [3]float32{0.3, 0.3, 0.3},
		diffuseColor: [3]float32{1.0, 1.0, 1.0},
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

// LoadMap loads a GND/RSW map for rendering.
func (mv *MapViewer) LoadMap(gnd *formats.GND, rsw *formats.RSW, texLoader func(string) ([]byte, error)) error {
	// Clear old resources
	mv.clearTerrain()

	// Store map dimensions for coordinate conversion (RSW positions are centered)
	mv.mapWidth = float32(gnd.Width) * gnd.Zoom
	mv.mapHeight = float32(gnd.Height) * gnd.Zoom

	// Extract lighting data from RSW (Stage 1: Correct Lighting - ADR-014)
	if rsw != nil {
		// Calculate sun direction from spherical coordinates
		mv.lightDir = calculateSunDirection(rsw.Light.Longitude, rsw.Light.Latitude)

		// Use RSW ambient and diffuse colors
		// Note: RSW values are often quite low, we apply a minimum floor
		// to prevent completely dark scenes
		mv.ambientColor = rsw.Light.Ambient
		mv.diffuseColor = rsw.Light.Diffuse

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

	// Fit camera to map
	mv.fitCamera()

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

		// Decode texture (reuse existing decoders)
		img, err := decodeModelTexture(data, fullPath, false)
		if err != nil {
			continue
		}

		// Upload to GPU
		texID := uploadModelTexture(img)
		mv.groundTextures[i] = texID
	}
}

// loadModels loads RSM models from RSW object list.
func (mv *MapViewer) loadModels(rsw *formats.RSW, texLoader func(string) ([]byte, error)) {
	models := rsw.GetModels()

	// Limit number of models to avoid performance issues
	maxModels := 500
	if len(models) > maxModels {
		models = models[:maxModels]
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
				continue
			}
			rsm, err = formats.ParseRSM(data)
			if err != nil {
				continue
			}
			rsmCache[rsmPath] = rsm
		}

		// Build map model from RSM
		mapModel := mv.buildMapModel(rsm, modelRef, texLoader)
		if mapModel != nil {
			mv.models = append(mv.models, mapModel)
		}
	}
}

// buildMapModel creates a MapModel from RSM data with world transform.
func (mv *MapViewer) buildMapModel(rsm *formats.RSM, ref *formats.RSWModel, texLoader func(string) ([]byte, error)) *MapModel {
	if len(rsm.Nodes) == 0 {
		return nil
	}

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
			continue
		}
		img, err := decodeModelTexture(data, texPath, true) // Use magenta key
		if err != nil {
			modelTextures[i] = mv.fallbackTex
			continue
		}
		modelTextures[i] = uploadModelTexture(img)
	}

	// Track bounding box for centering
	var minVertX, minVertY, minVertZ float32 = 1e10, 1e10, 1e10
	var maxVertX, maxVertY, maxVertZ float32 = -1e10, -1e10, -1e10

	// Process each node
	for i := range rsm.Nodes {
		node := &rsm.Nodes[i]
		baseIdx := uint32(len(vertices))

		// Build node transform matrix (with parent hierarchy)
		nodeMatrix := mv.buildNodeMatrix(node, rsm)

		// Process faces
		for _, face := range node.Faces {
			// Get face normal
			normal := [3]float32{0, 1, 0}
			if len(face.VertexIDs) >= 3 && int(face.VertexIDs[0]) < len(node.Vertices) &&
				int(face.VertexIDs[1]) < len(node.Vertices) && int(face.VertexIDs[2]) < len(node.Vertices) {
				// Calculate face normal from vertices
				v0 := node.Vertices[face.VertexIDs[0]]
				v1 := node.Vertices[face.VertexIDs[1]]
				v2 := node.Vertices[face.VertexIDs[2]]
				e1 := [3]float32{v1[0] - v0[0], v1[1] - v0[1], v1[2] - v0[2]}
				e2 := [3]float32{v2[0] - v0[0], v2[1] - v0[1], v2[2] - v0[2]}
				normal = normalize(cross(e1, e2))
			}

			// Add vertices for this face
			faceBaseIdx := uint32(len(vertices))
			for i, vid := range face.VertexIDs {
				if int(vid) >= len(node.Vertices) {
					continue
				}
				v := node.Vertices[vid]

				// Transform vertex position by node matrix
				pos := transformPoint(nodeMatrix, v)

				// Flip Y for RO coordinate system (same as model_viewer.go)
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
				if i < len(face.TexCoordIDs) && int(face.TexCoordIDs[i]) < len(node.TexCoords) {
					tc := node.TexCoords[face.TexCoordIDs[i]]
					uv = [2]float32{tc.U, tc.V}
				}

				vertices = append(vertices, modelVertex{
					Position: pos,
					Normal:   normal,
					TexCoord: uv,
				})
			}

			// Add indices for triangles (face can have 3+ vertices)
			texIdx := int(face.TextureID)
			for i := 2; i < len(face.VertexIDs); i++ {
				texGroups[texIdx] = append(texGroups[texIdx],
					faceBaseIdx,
					faceBaseIdx+uint32(i-1),
					faceBaseIdx+uint32(i),
				)
			}
		}
		_ = baseIdx // Silence unused warning
	}

	if len(vertices) == 0 {
		return nil
	}

	// Center all models based on bounding box (standard RO approach)
	// Subtract centerX/Z for horizontal centering, minY to put base at ground
	centerX := (minVertX + maxVertX) / 2
	centerZ := (minVertZ + maxVertZ) / 2
	for i := range vertices {
		vertices[i].Position[0] -= centerX
		vertices[i].Position[1] -= minVertY
		vertices[i].Position[2] -= centerZ
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

	// Create GPU resources
	model := &MapModel{
		textures:  modelTextures,
		texGroups: groups,
		position:  ref.Position,
		rotation:  ref.Rotation,
		scale:     ref.Scale,
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
// This matches the model_viewer.go implementation for consistency.
func (mv *MapViewer) buildNodeMatrix(node *formats.RSMNode, rsm *formats.RSM) math.Mat4 {
	return mv.buildNodeMatrixRecursive(node, rsm, make(map[string]bool))
}

func (mv *MapViewer) buildNodeMatrixRecursive(node *formats.RSMNode, rsm *formats.RSM, visited map[string]bool) math.Mat4 {
	// Prevent infinite recursion
	if visited[node.Name] {
		return math.Identity()
	}
	visited[node.Name] = true

	result := math.Identity()

	// Apply scale
	result = result.Mul(math.Scale(node.Scale[0], node.Scale[1], node.Scale[2]))

	// Apply rotation from 3x3 matrix (not axis-angle)
	rotMat := math.FromMat3x3(node.Matrix)
	result = result.Mul(rotMat)

	// Apply offset (pivot point) - note the negative sign
	result = result.Mul(math.Translate(-node.Offset[0], -node.Offset[1], -node.Offset[2]))

	// Apply position
	result = result.Mul(math.Translate(node.Position[0], node.Position[1], node.Position[2]))

	// If node has parent, multiply by parent's matrix
	if node.Parent != "" && node.Parent != node.Name {
		parentNode := rsm.GetNodeByName(node.Parent)
		if parentNode != nil {
			parentMatrix := mv.buildNodeMatrixRecursive(parentNode, rsm, visited)
			result = parentMatrix.Mul(result)
		}
	}

	return result
}

// transformPoint transforms a point by a 4x4 matrix.
func transformPoint(m math.Mat4, p [3]float32) [3]float32 {
	x := m[0]*p[0] + m[4]*p[1] + m[8]*p[2] + m[12]
	y := m[1]*p[0] + m[5]*p[1] + m[9]*p[2] + m[13]
	z := m[2]*p[0] + m[6]*p[1] + m[10]*p[2] + m[14]
	return [3]float32{x, y, z}
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
	// Round up to power of 2
	for p := 64; p < atlasSize; p *= 2 {
		atlasSize = p * 2
	}
	if atlasSize < 64 {
		atlasSize = 64
	}
	if atlasSize > 2048 {
		atlasSize = 2048
	}

	mv.atlasSize = int32(atlasSize)
	mv.tilesPerRow = int32(atlasSize / lmWidth)

	// Create atlas texture data (RGB)
	atlasData := make([]uint8, atlasSize*atlasSize*3)

	// Fill with white default
	for i := range atlasData {
		atlasData[i] = 255
	}

	// Copy each lightmap into the atlas
	for i, lm := range gnd.Lightmaps {
		tileX := i % int(mv.tilesPerRow)
		tileY := i / int(mv.tilesPerRow)

		baseX := tileX * lmWidth
		baseY := tileY * lmHeight

		// Copy lightmap pixels
		for y := 0; y < lmHeight; y++ {
			for x := 0; x < lmWidth; x++ {
				srcIdx := y*lmWidth + x
				dstX := baseX + x
				dstY := baseY + y

				if dstX >= atlasSize || dstY >= atlasSize {
					continue
				}

				dstIdx := (dstY*atlasSize + dstX) * 3

				// Combine brightness with color
				brightness := float32(1.0)
				if srcIdx < len(lm.Brightness) {
					brightness = float32(lm.Brightness[srcIdx]) / 255.0
				}

				// Get RGB color
				var r, g, b uint8 = 255, 255, 255
				if srcIdx*3+2 < len(lm.ColorRGB) {
					r = lm.ColorRGB[srcIdx*3]
					g = lm.ColorRGB[srcIdx*3+1]
					b = lm.ColorRGB[srcIdx*3+2]
				}

				// Apply brightness to color
				atlasData[dstIdx] = uint8(float32(r) * brightness)
				atlasData[dstIdx+1] = uint8(float32(g) * brightness)
				atlasData[dstIdx+2] = uint8(float32(b) * brightness)
			}
		}
	}

	// Upload atlas to GPU
	gl.GenTextures(1, &mv.lightmapAtlas)
	gl.BindTexture(gl.TEXTURE_2D, mv.lightmapAtlas)
	gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGB, int32(atlasSize), int32(atlasSize), 0, gl.RGB, gl.UNSIGNED_BYTE, gl.Ptr(atlasData))
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
}

// calculateLightmapUV returns UV coordinates for a lightmap in the atlas.
// cornerIdx: 0=BL, 1=BR, 2=TL, 3=TR
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

	// Calculate UV with small inset to avoid edge bleeding
	atlasSize := float32(mv.atlasSize)
	tileW := float32(lmWidth) / atlasSize
	tileH := float32(lmHeight) / atlasSize

	baseU := float32(tileX*lmWidth) / atlasSize
	baseV := float32(tileY*lmHeight) / atlasSize

	// Small inset (half pixel)
	inset := 0.5 / atlasSize

	// Corner UVs within the tile (with inset)
	// GND UV order: [0]=BL, [1]=BR, [2]=TL, [3]=TR
	switch cornerIdx {
	case 0: // Bottom-left
		return [2]float32{baseU + inset, baseV + tileH - inset}
	case 1: // Bottom-right
		return [2]float32{baseU + tileW - inset, baseV + tileH - inset}
	case 2: // Top-left
		return [2]float32{baseU + inset, baseV + inset}
	case 3: // Top-right
		return [2]float32{baseU + tileW - inset, baseV + inset}
	}
	return [2]float32{0.5, 0.5}
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
				baseIdx := uint32(len(vertices))
				vertices = append(vertices,
					terrainVertex{Position: corners[0], Normal: normal, TexCoord: [2]float32{surface.U[0], surface.V[0]}, LightmapUV: lmUV0, Color: color},
					terrainVertex{Position: corners[1], Normal: normal, TexCoord: [2]float32{surface.U[1], surface.V[1]}, LightmapUV: lmUV1, Color: color},
					terrainVertex{Position: corners[2], Normal: normal, TexCoord: [2]float32{surface.U[2], surface.V[2]}, LightmapUV: lmUV2, Color: color},
					terrainVertex{Position: corners[3], Normal: normal, TexCoord: [2]float32{surface.U[3], surface.V[3]}, LightmapUV: lmUV3, Color: color},
				)

				// Two triangles for quad
				textureIndices[texID] = append(textureIndices[texID],
					baseIdx, baseIdx+1, baseIdx+2,
					baseIdx+1, baseIdx+3, baseIdx+2,
				)
			}

			// Front surface (vertical wall facing -Z)
			// Only render if there's actual height difference between tiles
			if tile.FrontSurface >= 0 && int(tile.FrontSurface) < len(gnd.Surfaces) {
				surface := &gnd.Surfaces[tile.FrontSurface]
				texID := int(surface.TextureID)

				// Get neighboring tile for bottom edge
				nextTile := gnd.GetTile(x, y+1)
				if nextTile != nil {
					// Check if there's a height difference (skip flat connections)
					heightDiff0 := absf(tile.Altitude[0] - nextTile.Altitude[2])
					heightDiff1 := absf(tile.Altitude[1] - nextTile.Altitude[3])
					if heightDiff0 > 1.0 || heightDiff1 > 1.0 {
						// Wall corners
						wallCorners := [4][3]float32{
							corners[0], // Top-left
							corners[1], // Top-right
							{baseX, -nextTile.Altitude[2], baseZ + tileSize},            // Bottom-left
							{baseX + tileSize, -nextTile.Altitude[3], baseZ + tileSize}, // Bottom-right
						}

						normal := [3]float32{0, 0, -1} // Facing -Z
						color := [4]float32{1.0, 1.0, 1.0, 1.0}

						// Calculate lightmap UVs for wall
						wlmUV0 := mv.calculateLightmapUV(surface.LightmapID, 0, gnd)
						wlmUV1 := mv.calculateLightmapUV(surface.LightmapID, 1, gnd)
						wlmUV2 := mv.calculateLightmapUV(surface.LightmapID, 2, gnd)
						wlmUV3 := mv.calculateLightmapUV(surface.LightmapID, 3, gnd)

						baseIdx := uint32(len(vertices))
						vertices = append(vertices,
							terrainVertex{Position: wallCorners[0], Normal: normal, TexCoord: [2]float32{surface.U[0], surface.V[0]}, LightmapUV: wlmUV0, Color: color},
							terrainVertex{Position: wallCorners[1], Normal: normal, TexCoord: [2]float32{surface.U[1], surface.V[1]}, LightmapUV: wlmUV1, Color: color},
							terrainVertex{Position: wallCorners[2], Normal: normal, TexCoord: [2]float32{surface.U[2], surface.V[2]}, LightmapUV: wlmUV2, Color: color},
							terrainVertex{Position: wallCorners[3], Normal: normal, TexCoord: [2]float32{surface.U[3], surface.V[3]}, LightmapUV: wlmUV3, Color: color},
						)

						textureIndices[texID] = append(textureIndices[texID],
							baseIdx, baseIdx+2, baseIdx+1,
							baseIdx+1, baseIdx+2, baseIdx+3,
						)
					}
				}
			}

			// Right surface (vertical wall facing +X)
			// Only render if there's actual height difference between tiles
			if tile.RightSurface >= 0 && int(tile.RightSurface) < len(gnd.Surfaces) {
				surface := &gnd.Surfaces[tile.RightSurface]
				texID := int(surface.TextureID)

				// Get neighboring tile for right edge
				nextTile := gnd.GetTile(x+1, y)
				if nextTile != nil {
					// Check if there's a height difference (skip flat connections)
					heightDiff0 := absf(tile.Altitude[1] - nextTile.Altitude[0])
					heightDiff1 := absf(tile.Altitude[3] - nextTile.Altitude[2])
					if heightDiff0 > 1.0 || heightDiff1 > 1.0 {
						// Wall corners
						wallCorners := [4][3]float32{
							corners[3], // Top-back
							corners[1], // Top-front
							{baseX + tileSize, -nextTile.Altitude[2], baseZ},            // Bottom-back
							{baseX + tileSize, -nextTile.Altitude[0], baseZ + tileSize}, // Bottom-front
						}

						normal := [3]float32{1, 0, 0} // Facing +X
						color := [4]float32{1.0, 1.0, 1.0, 1.0}

						// Calculate lightmap UVs for wall
						wlmUV0 := mv.calculateLightmapUV(surface.LightmapID, 0, gnd)
						wlmUV1 := mv.calculateLightmapUV(surface.LightmapID, 1, gnd)
						wlmUV2 := mv.calculateLightmapUV(surface.LightmapID, 2, gnd)
						wlmUV3 := mv.calculateLightmapUV(surface.LightmapID, 3, gnd)

						baseIdx := uint32(len(vertices))
						vertices = append(vertices,
							terrainVertex{Position: wallCorners[0], Normal: normal, TexCoord: [2]float32{surface.U[0], surface.V[0]}, LightmapUV: wlmUV0, Color: color},
							terrainVertex{Position: wallCorners[1], Normal: normal, TexCoord: [2]float32{surface.U[1], surface.V[1]}, LightmapUV: wlmUV1, Color: color},
							terrainVertex{Position: wallCorners[2], Normal: normal, TexCoord: [2]float32{surface.U[2], surface.V[2]}, LightmapUV: wlmUV2, Color: color},
							terrainVertex{Position: wallCorners[3], Normal: normal, TexCoord: [2]float32{surface.U[3], surface.V[3]}, LightmapUV: wlmUV3, Color: color},
						)

						textureIndices[texID] = append(textureIndices[texID],
							baseIdx, baseIdx+2, baseIdx+1,
							baseIdx+1, baseIdx+2, baseIdx+3,
						)
					}
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

	mv.distance = maxSize * 0.7
	if mv.distance < 100 {
		mv.distance = 100
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
	if mv.FPSMode {
		// FPS camera - look from position in direction of yaw/pitch
		camPos := math.Vec3{X: mv.camPosX, Y: mv.camPosY, Z: mv.camPosZ}
		// Calculate look direction from yaw and pitch
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

	// Use terrain shader with RSW lighting data
	gl.UseProgram(mv.terrainProgram)
	gl.UniformMatrix4fv(mv.locViewProj, 1, false, &viewProj[0])
	gl.Uniform3f(mv.locLightDir, mv.lightDir[0], mv.lightDir[1], mv.lightDir[2])
	gl.Uniform3f(mv.locAmbient, mv.ambientColor[0], mv.ambientColor[1], mv.ambientColor[2])
	gl.Uniform3f(mv.locDiffuse, mv.diffuseColor[0], mv.diffuseColor[1], mv.diffuseColor[2])
	gl.Uniform1i(mv.locTexture, 0)
	gl.Uniform1i(mv.locLightmap, 1)

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

	gl.BindFramebuffer(gl.FRAMEBUFFER, 0)

	return mv.colorTexture
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

	gl.ActiveTexture(gl.TEXTURE0)

	// RSW positions are centered at map origin (0,0,0)
	// GND terrain spans from (0,0) to (mapWidth, mapHeight)
	// Convert by adding map center offset
	offsetX := mv.mapWidth / 2
	offsetZ := mv.mapHeight / 2

	for _, model := range mv.models {
		if model.vao == 0 || model.indexCount == 0 {
			continue
		}

		// Convert RSW position to GND world coordinates:
		// - RSW X (0 = center) -> World X = rswX + mapWidth/2
		// - RSW Y (altitude) -> World Y = -rswY (same as GND altitude convention)
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
	x := mv.distance * float32(cosf(mv.rotationX)*sinf(mv.rotationY))
	y := mv.distance * float32(sinf(mv.rotationX))
	z := mv.distance * float32(cosf(mv.rotationX)*cosf(mv.rotationY))

	return math.Vec3{
		X: mv.centerX + x,
		Y: mv.centerY + y,
		Z: mv.centerZ + z,
	}
}

// HandleMouseDrag handles mouse drag for camera rotation.
func (mv *MapViewer) HandleMouseDrag(deltaX, deltaY float32) {
	sensitivity := float32(0.005)

	if mv.FPSMode {
		// FPS mode - adjust yaw and pitch
		mv.camYaw -= deltaX * sensitivity
		mv.camPitch -= deltaY * sensitivity

		// Clamp pitch to avoid gimbal lock
		if mv.camPitch < -1.5 {
			mv.camPitch = -1.5
		}
		if mv.camPitch > 1.5 {
			mv.camPitch = 1.5
		}
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
	if mv.FPSMode {
		// In FPS mode, scroll adjusts move speed
		mv.MoveSpeed += delta * 0.5
		if mv.MoveSpeed < 1.0 {
			mv.MoveSpeed = 1.0
		}
		if mv.MoveSpeed > 50.0 {
			mv.MoveSpeed = 50.0
		}
	} else {
		// Orbit mode - zoom in/out
		mv.distance -= delta * mv.distance * 0.1
		if mv.distance < 50 {
			mv.distance = 50
		}
		if mv.distance > 5000 {
			mv.distance = 5000
		}
	}
}

// HandleFPSMovement handles WASD movement in FPS mode.
// forward/right are -1, 0, or 1 based on key presses.
func (mv *MapViewer) HandleFPSMovement(forward, right, up float32) {
	if !mv.FPSMode {
		return
	}

	// Calculate forward direction (horizontal only for walking)
	dirX := float32(sinf(mv.camYaw))
	dirZ := float32(cosf(mv.camYaw))

	// Right direction (perpendicular to forward) - negated for correct A/D mapping
	rightX := float32(-cosf(mv.camYaw))
	rightZ := float32(sinf(mv.camYaw))

	// Apply movement
	mv.camPosX += (dirX*forward + rightX*right) * mv.MoveSpeed
	mv.camPosZ += (dirZ*forward + rightZ*right) * mv.MoveSpeed
	mv.camPosY += up * mv.MoveSpeed
}

// ToggleFPSMode toggles between orbit and FPS camera modes.
func (mv *MapViewer) ToggleFPSMode() {
	mv.FPSMode = !mv.FPSMode

	if mv.FPSMode {
		// Initialize FPS camera at map center, slightly above ground level
		mv.camPosX = mv.centerX
		mv.camPosY = mv.centerY + 50 // Slightly above center height
		mv.camPosZ = mv.centerZ

		// Look forward (towards +Z direction)
		mv.camYaw = 0
		mv.camPitch = 0
	}
}

// Reset resets camera to default position.
func (mv *MapViewer) Reset() {
	if mv.FPSMode {
		// Reset FPS camera to map center
		mv.camPosX = mv.centerX
		mv.camPosY = mv.centerY + 50
		mv.camPosZ = mv.centerZ
		mv.camYaw = 0
		mv.camPitch = 0
		mv.MoveSpeed = 5.0
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
