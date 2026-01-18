// 3D model viewer for RSM files (ADR-012 Stage 3).
package main

import (
	"bytes"
	"fmt"
	"image"
	gomath "math"
	"strings"
	"unsafe"

	"github.com/go-gl/gl/v4.1-core/gl"

	"github.com/Faultbox/midgard-ro/internal/engine/camera"
	"github.com/Faultbox/midgard-ro/internal/engine/texture"
	"github.com/Faultbox/midgard-ro/pkg/formats"
	"github.com/Faultbox/midgard-ro/pkg/math"
)

// ModelViewer handles 3D rendering of RSM models to an offscreen framebuffer.
type ModelViewer struct {
	// Framebuffer resources
	fbo          uint32
	colorTexture uint32
	depthRBO     uint32
	width        int32
	height       int32

	// Shader program
	shaderProgram uint32
	locModel      int32
	locView       int32
	locProjection int32
	locLightDir   int32
	locAmbient    int32
	locDiffuse    int32
	locTexture    int32

	// Mesh resources
	vao        uint32
	vbo        uint32
	ebo        uint32
	indexCount int32

	// Texture draw groups (for multi-texture support)
	textureGroups []textureDrawGroup

	// Model textures (OpenGL texture IDs)
	modelTextures []uint32

	// Fallback texture for missing textures
	fallbackTexture uint32

	// Camera state
	rotationX float32 // Pitch (vertical rotation)
	rotationY float32 // Yaw (horizontal rotation)
	distance  float32 // Distance from center
	centerX   float32
	centerY   float32
	centerZ   float32

	// Bounding box for auto-fit
	minBounds [3]float32
	maxBounds [3]float32

	// Animation state
	animPlaying bool
	animTime    float32 // Current animation time in milliseconds
	animSpeed   float32 // Animation speed multiplier (1.0 = normal)
	animLength  int32   // Total animation length in ms (from RSM)
	animLooping bool    // Whether animation loops

	// Cached data for animation rebuild
	currentRSM      *formats.RSM
	textureLoader   func(string) ([]byte, error)
	magentaKeyCache bool

	// Axis visualization
	showAxes    bool
	axisVAO     uint32
	axisVBO     uint32
	axisShader  uint32
	axisLocView int32
	axisLocProj int32

	// Rendering modes
	wireframeMode bool

	// Node visibility (for compound models)
	nodeVisibility map[string]bool // true = visible, false = hidden
}

// rsmVertex is the vertex format for RSM mesh.
type rsmVertex struct {
	Position [3]float32
	Normal   [3]float32
	TexCoord [2]float32
}

// textureDrawGroup represents a group of faces sharing the same texture.
type textureDrawGroup struct {
	textureIdx int   // Index into modelTextures array
	startIndex int32 // Starting index in EBO
	indexCount int32 // Number of indices to draw
}

const vertexShaderSource = `#version 410 core
layout (location = 0) in vec3 aPosition;
layout (location = 1) in vec3 aNormal;
layout (location = 2) in vec2 aTexCoord;

uniform mat4 uModel;
uniform mat4 uView;
uniform mat4 uProjection;

out vec3 vNormal;
out vec2 vTexCoord;

void main() {
    vNormal = mat3(uModel) * aNormal;
    vTexCoord = aTexCoord;
    gl_Position = uProjection * uView * uModel * vec4(aPosition, 1.0);
}
` + "\x00"

const fragmentShaderSource = `#version 410 core
in vec3 vNormal;
in vec2 vTexCoord;

uniform sampler2D uTexture;
uniform vec3 uLightDir;
uniform vec3 uAmbient;
uniform vec3 uDiffuse;

out vec4 FragColor;

void main() {
    vec3 normal = normalize(vNormal);
    vec3 lightDir = normalize(uLightDir);
    float diff = max(dot(normal, lightDir), 0.0);
    vec4 tex = texture(uTexture, vTexCoord);
    vec3 result = (uAmbient + diff * uDiffuse) * tex.rgb;
    FragColor = vec4(result, tex.a);
}
` + "\x00"

// Line shader for axis visualization
const lineVertexShader = `#version 410 core
layout (location = 0) in vec3 aPosition;
layout (location = 1) in vec3 aColor;

uniform mat4 uView;
uniform mat4 uProjection;

out vec3 vColor;

void main() {
    vColor = aColor;
    gl_Position = uProjection * uView * vec4(aPosition, 1.0);
}
` + "\x00"

const lineFragmentShader = `#version 410 core
in vec3 vColor;
out vec4 FragColor;

void main() {
    FragColor = vec4(vColor, 1.0);
}
` + "\x00"

// NewModelViewer creates a new 3D model viewer.
func NewModelViewer(width, height int32) (*ModelViewer, error) {
	mv := &ModelViewer{
		width:          width,
		height:         height,
		rotationX:      0.3,   // Slight downward angle
		rotationY:      0.5,   // Slight sideways angle
		distance:       100.0, // Default zoom
		animSpeed:      1.0,   // Normal animation speed
		animLooping:    true,  // Loop by default
		showAxes:       true,  // Show axes by default
		nodeVisibility: make(map[string]bool),
	}

	// Create framebuffer
	if err := mv.createFramebuffer(); err != nil {
		return nil, fmt.Errorf("framebuffer: %w", err)
	}

	// Create shader program
	if err := mv.createShaderProgram(); err != nil {
		mv.Destroy()
		return nil, fmt.Errorf("shader: %w", err)
	}

	// Create axis visualization
	if err := mv.createAxisVisualization(); err != nil {
		mv.Destroy()
		return nil, fmt.Errorf("axis viz: %w", err)
	}

	// Create fallback texture
	mv.createFallbackTexture()

	return mv, nil
}

func (mv *ModelViewer) createFramebuffer() error {
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
	if status := gl.CheckFramebufferStatus(gl.FRAMEBUFFER); status != gl.FRAMEBUFFER_COMPLETE {
		return fmt.Errorf("framebuffer incomplete: 0x%x", status)
	}

	gl.BindFramebuffer(gl.FRAMEBUFFER, 0)
	return nil
}

func (mv *ModelViewer) createShaderProgram() error {
	// Compile vertex shader
	vertexShader, err := compileModelShader(vertexShaderSource, gl.VERTEX_SHADER)
	if err != nil {
		return fmt.Errorf("vertex shader: %w", err)
	}
	defer gl.DeleteShader(vertexShader)

	// Compile fragment shader
	fragmentShader, err := compileModelShader(fragmentShaderSource, gl.FRAGMENT_SHADER)
	if err != nil {
		return fmt.Errorf("fragment shader: %w", err)
	}
	defer gl.DeleteShader(fragmentShader)

	// Link program
	mv.shaderProgram = gl.CreateProgram()
	gl.AttachShader(mv.shaderProgram, vertexShader)
	gl.AttachShader(mv.shaderProgram, fragmentShader)
	gl.LinkProgram(mv.shaderProgram)

	var status int32
	gl.GetProgramiv(mv.shaderProgram, gl.LINK_STATUS, &status)
	if status == gl.FALSE {
		var logLength int32
		gl.GetProgramiv(mv.shaderProgram, gl.INFO_LOG_LENGTH, &logLength)
		log := strings.Repeat("\x00", int(logLength+1))
		gl.GetProgramInfoLog(mv.shaderProgram, logLength, nil, gl.Str(log))
		return fmt.Errorf("link failed: %s", log)
	}

	// Get uniform locations
	mv.locModel = gl.GetUniformLocation(mv.shaderProgram, gl.Str("uModel\x00"))
	mv.locView = gl.GetUniformLocation(mv.shaderProgram, gl.Str("uView\x00"))
	mv.locProjection = gl.GetUniformLocation(mv.shaderProgram, gl.Str("uProjection\x00"))
	mv.locLightDir = gl.GetUniformLocation(mv.shaderProgram, gl.Str("uLightDir\x00"))
	mv.locAmbient = gl.GetUniformLocation(mv.shaderProgram, gl.Str("uAmbient\x00"))
	mv.locDiffuse = gl.GetUniformLocation(mv.shaderProgram, gl.Str("uDiffuse\x00"))
	mv.locTexture = gl.GetUniformLocation(mv.shaderProgram, gl.Str("uTexture\x00"))

	return nil
}

func compileModelShader(source string, shaderType uint32) (uint32, error) {
	shader := gl.CreateShader(shaderType)
	csources, free := gl.Strs(source)
	gl.ShaderSource(shader, 1, csources, nil)
	free()
	gl.CompileShader(shader)

	var status int32
	gl.GetShaderiv(shader, gl.COMPILE_STATUS, &status)
	if status == gl.FALSE {
		var logLength int32
		gl.GetShaderiv(shader, gl.INFO_LOG_LENGTH, &logLength)
		log := strings.Repeat("\x00", int(logLength+1))
		gl.GetShaderInfoLog(shader, logLength, nil, gl.Str(log))
		return 0, fmt.Errorf("compile failed: %s", log)
	}

	return shader, nil
}

func (mv *ModelViewer) createFallbackTexture() {
	// Create a simple white 1x1 texture
	gl.GenTextures(1, &mv.fallbackTexture)
	gl.BindTexture(gl.TEXTURE_2D, mv.fallbackTexture)
	white := []uint8{255, 255, 255, 255}
	gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA, 1, 1, 0, gl.RGBA, gl.UNSIGNED_BYTE, unsafe.Pointer(&white[0]))
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.NEAREST)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.NEAREST)
}

// LoadModel processes RSM data and uploads to GPU.
// magentaKey enables treating RGB(255,0,255) as transparent.
func (mv *ModelViewer) LoadModel(rsm *formats.RSM, texLoader func(string) ([]byte, error), magentaKey bool) error {
	// Clear previous model
	mv.clearModel()

	// Reset node visibility (all visible by default)
	mv.nodeVisibility = make(map[string]bool)

	// Store references for animation rebuild
	mv.currentRSM = rsm
	mv.textureLoader = texLoader
	mv.magentaKeyCache = magentaKey

	// Initialize animation state
	mv.animLength = rsm.AnimLength
	mv.animTime = 0
	mv.animPlaying = false // Start paused

	// Build mesh with current animation time (0 = base pose)
	vertices, indices := mv.buildMeshFromRSM(rsm, 0)
	if len(vertices) == 0 {
		return fmt.Errorf("no vertices in model")
	}

	// Upload to GPU
	mv.uploadMesh(vertices, indices)

	// Load textures
	mv.loadTextures(rsm, texLoader, magentaKey)

	// Reset camera to fit model
	mv.fitCamera()

	return nil
}

func (mv *ModelViewer) buildMeshFromRSM(rsm *formats.RSM, animTimeMs float32) ([]rsmVertex, []uint32) {
	// Group faces by global texture index for multi-texture support
	type faceData struct {
		vertices [3]rsmVertex
		texIdx   int // Global texture index into rsm.Textures
	}

	// Map from global texture index to list of faces
	facesByTexture := make(map[int][]faceData)

	// Initialize bounding box
	mv.minBounds = [3]float32{1e10, 1e10, 1e10}
	mv.maxBounds = [3]float32{-1e10, -1e10, -1e10}

	for nodeIdx := range rsm.Nodes {
		node := &rsm.Nodes[nodeIdx]

		// Skip hidden nodes (for compound model inspection)
		if !mv.GetNodeVisibility(node.Name) {
			continue
		}

		// Build node transformation matrix with animation
		nodeMatrix := mv.buildNodeMatrix(node, rsm, animTimeMs)

		for _, face := range node.Faces {
			// Bounds check for vertex indices
			if int(face.VertexIDs[0]) >= len(node.Vertices) ||
				int(face.VertexIDs[1]) >= len(node.Vertices) ||
				int(face.VertexIDs[2]) >= len(node.Vertices) {
				continue // Skip invalid faces
			}

			// Get vertices for this face
			v0 := node.Vertices[face.VertexIDs[0]]
			v1 := node.Vertices[face.VertexIDs[1]]
			v2 := node.Vertices[face.VertexIDs[2]]

			// Transform vertices (flip Y for RO coordinate system)
			tv0 := nodeMatrix.TransformPoint(v0)
			tv1 := nodeMatrix.TransformPoint(v1)
			tv2 := nodeMatrix.TransformPoint(v2)
			tv0[1] = -tv0[1]
			tv1[1] = -tv1[1]
			tv2[1] = -tv2[1]

			// Update bounding box
			mv.updateBounds(tv0)
			mv.updateBounds(tv1)
			mv.updateBounds(tv2)

			// Calculate face normal
			edge1 := math.Vec3{X: tv1[0] - tv0[0], Y: tv1[1] - tv0[1], Z: tv1[2] - tv0[2]}
			edge2 := math.Vec3{X: tv2[0] - tv0[0], Y: tv2[1] - tv0[1], Z: tv2[2] - tv0[2]}
			normal := edge1.Cross(edge2).Normalize()

			// Get texture coordinates
			var tc0, tc1, tc2 [2]float32
			if int(face.TexCoordIDs[0]) < len(node.TexCoords) {
				tc := node.TexCoords[face.TexCoordIDs[0]]
				tc0 = [2]float32{tc.U, tc.V}
			}
			if int(face.TexCoordIDs[1]) < len(node.TexCoords) {
				tc := node.TexCoords[face.TexCoordIDs[1]]
				tc1 = [2]float32{tc.U, tc.V}
			}
			if int(face.TexCoordIDs[2]) < len(node.TexCoords) {
				tc := node.TexCoords[face.TexCoordIDs[2]]
				tc2 = [2]float32{tc.U, tc.V}
			}

			// Determine global texture index
			// face.TextureID is index into node.TextureIDs
			// node.TextureIDs[i] is index into rsm.Textures
			globalTexIdx := 0
			if int(face.TextureID) < len(node.TextureIDs) {
				globalTexIdx = int(node.TextureIDs[face.TextureID])
			}

			// Store face data grouped by texture
			fd := faceData{
				vertices: [3]rsmVertex{
					{Position: tv0, Normal: [3]float32{normal.X, normal.Y, normal.Z}, TexCoord: tc0},
					{Position: tv1, Normal: [3]float32{normal.X, normal.Y, normal.Z}, TexCoord: tc1},
					{Position: tv2, Normal: [3]float32{normal.X, normal.Y, normal.Z}, TexCoord: tc2},
				},
				texIdx: globalTexIdx,
			}
			facesByTexture[globalTexIdx] = append(facesByTexture[globalTexIdx], fd)
		}
	}

	// Build final vertex and index arrays, grouped by texture
	var vertices []rsmVertex
	var indices []uint32
	mv.textureGroups = nil

	// Sort texture indices for consistent ordering
	var texIndices []int
	for texIdx := range facesByTexture {
		texIndices = append(texIndices, texIdx)
	}
	// Simple sort (bubble sort is fine for small arrays)
	for i := 0; i < len(texIndices); i++ {
		for j := i + 1; j < len(texIndices); j++ {
			if texIndices[i] > texIndices[j] {
				texIndices[i], texIndices[j] = texIndices[j], texIndices[i]
			}
		}
	}

	// Build vertices and indices for each texture group
	for _, texIdx := range texIndices {
		faces := facesByTexture[texIdx]
		startIdx := int32(len(indices))

		for _, fd := range faces {
			baseIdx := uint32(len(vertices))
			vertices = append(vertices, fd.vertices[0], fd.vertices[1], fd.vertices[2])
			indices = append(indices, baseIdx, baseIdx+1, baseIdx+2)
		}

		// Record this texture group
		mv.textureGroups = append(mv.textureGroups, textureDrawGroup{
			textureIdx: texIdx,
			startIndex: startIdx,
			indexCount: int32(len(indices)) - startIdx,
		})
	}
	return vertices, indices
}

func (mv *ModelViewer) buildNodeMatrix(node *formats.RSMNode, rsm *formats.RSM, animTimeMs float32) math.Mat4 {
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
func (mv *ModelViewer) buildNodeHierarchyMatrix(node *formats.RSMNode, rsm *formats.RSM, animTimeMs float32, visited map[string]bool) math.Mat4 {
	// Prevent infinite recursion from circular references
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

// interpolateRotKeys interpolates rotation keyframes at the given time.
func (mv *ModelViewer) interpolateRotKeys(keys []formats.RSMRotKeyframe, timeMs float32) math.Quat {
	if len(keys) == 0 {
		return math.QuatIdentity()
	}
	if len(keys) == 1 {
		k := keys[0]
		return math.Quat{X: k.Quaternion[0], Y: k.Quaternion[1], Z: k.Quaternion[2], W: k.Quaternion[3]}
	}

	// Find surrounding keyframes
	// RSM uses frame numbers, convert time to frame (assume 1000ms = 1000 frames for simplicity)
	frame := timeMs

	// Find the keyframes that bracket this frame
	var k0, k1 formats.RSMRotKeyframe
	k0 = keys[0]
	k1 = keys[0]

	for i := 0; i < len(keys)-1; i++ {
		if float32(keys[i].Frame) <= frame && float32(keys[i+1].Frame) > frame {
			k0 = keys[i]
			k1 = keys[i+1]
			break
		}
	}

	// If frame is past all keyframes, use last keyframe
	if frame >= float32(keys[len(keys)-1].Frame) {
		k := keys[len(keys)-1]
		return math.Quat{X: k.Quaternion[0], Y: k.Quaternion[1], Z: k.Quaternion[2], W: k.Quaternion[3]}
	}

	// If frame is before first keyframe, use first
	if frame <= float32(keys[0].Frame) {
		k := keys[0]
		return math.Quat{X: k.Quaternion[0], Y: k.Quaternion[1], Z: k.Quaternion[2], W: k.Quaternion[3]}
	}

	// Interpolate
	frameDiff := float32(k1.Frame - k0.Frame)
	if frameDiff <= 0 {
		k := k0
		return math.Quat{X: k.Quaternion[0], Y: k.Quaternion[1], Z: k.Quaternion[2], W: k.Quaternion[3]}
	}

	t := (frame - float32(k0.Frame)) / frameDiff
	q0 := math.Quat{X: k0.Quaternion[0], Y: k0.Quaternion[1], Z: k0.Quaternion[2], W: k0.Quaternion[3]}
	q1 := math.Quat{X: k1.Quaternion[0], Y: k1.Quaternion[1], Z: k1.Quaternion[2], W: k1.Quaternion[3]}

	return q0.Slerp(q1, t)
}

// interpolateScaleKeys interpolates scale keyframes at the given time.
func (mv *ModelViewer) interpolateScaleKeys(keys []formats.RSMScaleKeyframe, timeMs float32) [3]float32 {
	if len(keys) == 0 {
		return [3]float32{1, 1, 1}
	}
	if len(keys) == 1 {
		return keys[0].Scale
	}

	frame := timeMs

	// Find surrounding keyframes
	var k0, k1 formats.RSMScaleKeyframe
	k0 = keys[0]
	k1 = keys[0]

	for i := 0; i < len(keys)-1; i++ {
		if float32(keys[i].Frame) <= frame && float32(keys[i+1].Frame) > frame {
			k0 = keys[i]
			k1 = keys[i+1]
			break
		}
	}

	if frame >= float32(keys[len(keys)-1].Frame) {
		return keys[len(keys)-1].Scale
	}
	if frame <= float32(keys[0].Frame) {
		return keys[0].Scale
	}

	frameDiff := float32(k1.Frame - k0.Frame)
	if frameDiff <= 0 {
		return k0.Scale
	}

	t := (frame - float32(k0.Frame)) / frameDiff
	return math.LerpVec3(k0.Scale, k1.Scale, t)
}

func (mv *ModelViewer) updateBounds(p [3]float32) {
	for i := 0; i < 3; i++ {
		if p[i] < mv.minBounds[i] {
			mv.minBounds[i] = p[i]
		}
		if p[i] > mv.maxBounds[i] {
			mv.maxBounds[i] = p[i]
		}
	}
}

func (mv *ModelViewer) uploadMesh(vertices []rsmVertex, indices []uint32) {
	// Create VAO
	gl.GenVertexArrays(1, &mv.vao)
	gl.BindVertexArray(mv.vao)

	// Create VBO
	gl.GenBuffers(1, &mv.vbo)
	gl.BindBuffer(gl.ARRAY_BUFFER, mv.vbo)
	gl.BufferData(gl.ARRAY_BUFFER, len(vertices)*int(unsafe.Sizeof(rsmVertex{})), unsafe.Pointer(&vertices[0]), gl.STATIC_DRAW)

	// Create EBO
	gl.GenBuffers(1, &mv.ebo)
	gl.BindBuffer(gl.ELEMENT_ARRAY_BUFFER, mv.ebo)
	gl.BufferData(gl.ELEMENT_ARRAY_BUFFER, len(indices)*4, unsafe.Pointer(&indices[0]), gl.STATIC_DRAW)

	// Position attribute (location = 0)
	gl.VertexAttribPointerWithOffset(0, 3, gl.FLOAT, false, int32(unsafe.Sizeof(rsmVertex{})), 0)
	gl.EnableVertexAttribArray(0)

	// Normal attribute (location = 1)
	gl.VertexAttribPointerWithOffset(1, 3, gl.FLOAT, false, int32(unsafe.Sizeof(rsmVertex{})), 12)
	gl.EnableVertexAttribArray(1)

	// TexCoord attribute (location = 2)
	gl.VertexAttribPointerWithOffset(2, 2, gl.FLOAT, false, int32(unsafe.Sizeof(rsmVertex{})), 24)
	gl.EnableVertexAttribArray(2)

	gl.BindVertexArray(0)

	mv.indexCount = int32(len(indices))
}

func (mv *ModelViewer) loadTextures(rsm *formats.RSM, loader func(string) ([]byte, error), magentaKey bool) {
	mv.modelTextures = make([]uint32, len(rsm.Textures))

	for i, texPath := range rsm.Textures {
		// Build full GRF path
		fullPath := "data/texture/" + texPath

		data, err := loader(fullPath)
		if err != nil {
			mv.modelTextures[i] = mv.fallbackTexture
			continue
		}

		// Decode image
		img, err := decodeModelTexture(data, texPath, magentaKey)
		if err != nil {
			mv.modelTextures[i] = mv.fallbackTexture
			continue
		}

		// Upload to OpenGL
		mv.modelTextures[i] = uploadModelTexture(img)
	}
}

func decodeModelTexture(data []byte, path string, magentaKey bool) (*image.RGBA, error) {
	lowerPath := strings.ToLower(path)

	var img image.Image
	var err error

	if strings.HasSuffix(lowerPath, ".tga") {
		// TGA needs special handling
		img, err = texture.DecodeTGA(data)
	} else {
		// BMP, PNG, JPG - use standard decoder
		img, _, err = image.Decode(bytes.NewReader(data))
	}

	if err != nil {
		return nil, fmt.Errorf("decode %s: %w", path, err)
	}

	// Convert to RGBA with optional magenta key transparency
	return texture.ImageToRGBA(img, magentaKey), nil
}

func uploadModelTexture(img *image.RGBA) uint32 {
	var texID uint32
	gl.GenTextures(1, &texID)
	gl.BindTexture(gl.TEXTURE_2D, texID)

	gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA,
		int32(img.Bounds().Dx()), int32(img.Bounds().Dy()),
		0, gl.RGBA, gl.UNSIGNED_BYTE, unsafe.Pointer(&img.Pix[0]))

	// Generate mipmaps for smooth rendering at distance
	gl.GenerateMipmap(gl.TEXTURE_2D)

	// Use trilinear filtering (LINEAR between mipmap levels) for smooth appearance
	// This matches roBrowser and korangar approach
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR_MIPMAP_LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.REPEAT)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.REPEAT)

	// Limit max mipmap level to reduce texture bleeding at lowest LODs
	// For 256x256 textures, level 4 is 16x16 which is good enough
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAX_LEVEL, 4)

	// Enable anisotropic filtering for better quality at oblique angles (8x)
	gl.TexParameterf(gl.TEXTURE_2D, gl.TEXTURE_MAX_ANISOTROPY, 8.0)

	return texID
}

// uploadTerrainTexture uploads a texture with 1-pixel padding to prevent
// texture bleeding at mipmap boundaries (roBrowser atlas style).
// This creates a 258x258 texture from a 256x256 source, with edge pixels
// duplicated into the padding region.
func uploadTerrainTexture(img *image.RGBA) uint32 {
	srcBounds := img.Bounds()
	srcW := srcBounds.Dx()
	srcH := srcBounds.Dy()

	// Create padded image (1 pixel padding on each side)
	paddedW := srcW + 2
	paddedH := srcH + 2
	padded := image.NewRGBA(image.Rect(0, 0, paddedW, paddedH))

	// Copy source image to center (offset by 1,1)
	for y := 0; y < srcH; y++ {
		for x := 0; x < srcW; x++ {
			c := img.RGBAAt(srcBounds.Min.X+x, srcBounds.Min.Y+y)
			padded.SetRGBA(x+1, y+1, c)
		}
	}

	// Duplicate edges into padding region

	// Top and bottom edges
	for x := 0; x < srcW; x++ {
		// Top edge (y=0 in padded = copy from y=0 in source)
		c := img.RGBAAt(srcBounds.Min.X+x, srcBounds.Min.Y)
		padded.SetRGBA(x+1, 0, c)

		// Bottom edge (y=paddedH-1 in padded = copy from y=srcH-1 in source)
		c = img.RGBAAt(srcBounds.Min.X+x, srcBounds.Min.Y+srcH-1)
		padded.SetRGBA(x+1, paddedH-1, c)
	}

	// Left and right edges
	for y := 0; y < srcH; y++ {
		// Left edge (x=0 in padded = copy from x=0 in source)
		c := img.RGBAAt(srcBounds.Min.X, srcBounds.Min.Y+y)
		padded.SetRGBA(0, y+1, c)

		// Right edge (x=paddedW-1 in padded = copy from x=srcW-1 in source)
		c = img.RGBAAt(srcBounds.Min.X+srcW-1, srcBounds.Min.Y+y)
		padded.SetRGBA(paddedW-1, y+1, c)
	}

	// Corners
	padded.SetRGBA(0, 0, img.RGBAAt(srcBounds.Min.X, srcBounds.Min.Y))                               // Top-left
	padded.SetRGBA(paddedW-1, 0, img.RGBAAt(srcBounds.Min.X+srcW-1, srcBounds.Min.Y))                // Top-right
	padded.SetRGBA(0, paddedH-1, img.RGBAAt(srcBounds.Min.X, srcBounds.Min.Y+srcH-1))                // Bottom-left
	padded.SetRGBA(paddedW-1, paddedH-1, img.RGBAAt(srcBounds.Min.X+srcW-1, srcBounds.Min.Y+srcH-1)) // Bottom-right

	// Upload padded texture to GPU
	var texID uint32
	gl.GenTextures(1, &texID)
	gl.BindTexture(gl.TEXTURE_2D, texID)

	gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA,
		int32(paddedW), int32(paddedH),
		0, gl.RGBA, gl.UNSIGNED_BYTE, unsafe.Pointer(&padded.Pix[0]))

	// Generate mipmaps for smooth rendering at distance
	gl.GenerateMipmap(gl.TEXTURE_2D)

	// Use trilinear filtering for smooth appearance
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR_MIPMAP_LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)

	// Enable anisotropic filtering for better quality at oblique angles (8x)
	gl.TexParameterf(gl.TEXTURE_2D, gl.TEXTURE_MAX_ANISOTROPY, 8.0)

	return texID
}

func (mv *ModelViewer) fitCamera() {
	// Use camera package to calculate fitting parameters
	fit := camera.FitBoundsToView(mv.minBounds, mv.maxBounds, 2.0, 10.0)
	mv.centerX = fit.CenterX
	mv.centerY = fit.CenterY
	mv.centerZ = fit.CenterZ
	mv.distance = fit.Distance
}

// Render draws the model to the framebuffer and returns the texture ID.
func (mv *ModelViewer) Render() uint32 {
	if mv.vao == 0 || mv.indexCount == 0 {
		return mv.colorTexture
	}

	// Save current OpenGL state
	var prevFBO int32
	gl.GetIntegerv(gl.FRAMEBUFFER_BINDING, &prevFBO)
	var prevViewport [4]int32
	gl.GetIntegerv(gl.VIEWPORT, &prevViewport[0])

	// Bind our framebuffer
	gl.BindFramebuffer(gl.FRAMEBUFFER, mv.fbo)
	gl.Viewport(0, 0, mv.width, mv.height)

	// Clear - use light background for wireframe mode
	if mv.wireframeMode {
		gl.ClearColor(0.7, 0.7, 0.75, 1.0) // Light gray for wireframe visibility
	} else {
		gl.ClearColor(0.15, 0.15, 0.2, 1.0) // Dark blue-gray for normal mode
	}
	gl.Clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT)

	// Enable depth testing
	gl.Enable(gl.DEPTH_TEST)
	gl.DepthFunc(gl.LESS)

	// Enable alpha blending for transparent textures
	gl.Enable(gl.BLEND)
	gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)

	// Use shader
	gl.UseProgram(mv.shaderProgram)

	// Calculate matrices
	aspect := float32(mv.width) / float32(mv.height)
	projection := math.Perspective(0.785398, aspect, 0.1, 10000.0) // 45 degrees FOV

	// Camera position (orbiting)
	eye := mv.calculateCameraPosition()
	center := math.Vec3{X: mv.centerX, Y: mv.centerY, Z: mv.centerZ}
	up := math.Vec3{X: 0, Y: 1, Z: 0}
	view := math.LookAt(eye, center, up)

	model := math.Identity()

	// Set uniforms
	gl.UniformMatrix4fv(mv.locProjection, 1, false, projection.Ptr())
	gl.UniformMatrix4fv(mv.locView, 1, false, view.Ptr())
	gl.UniformMatrix4fv(mv.locModel, 1, false, model.Ptr())

	// Lighting
	gl.Uniform3f(mv.locLightDir, 0.5, 1.0, 0.5)
	gl.Uniform3f(mv.locAmbient, 0.4, 0.4, 0.4)
	gl.Uniform3f(mv.locDiffuse, 0.6, 0.6, 0.6)

	// Set wireframe mode if enabled
	if mv.wireframeMode {
		gl.PolygonMode(gl.FRONT_AND_BACK, gl.LINE)
	}

	// Draw each texture group
	gl.ActiveTexture(gl.TEXTURE0)
	gl.Uniform1i(mv.locTexture, 0)
	gl.BindVertexArray(mv.vao)

	for _, group := range mv.textureGroups {
		// Bind appropriate texture for this group
		texID := mv.fallbackTexture
		if group.textureIdx >= 0 && group.textureIdx < len(mv.modelTextures) && mv.modelTextures[group.textureIdx] != 0 {
			texID = mv.modelTextures[group.textureIdx]
		}
		gl.BindTexture(gl.TEXTURE_2D, texID)

		// Draw this group's triangles
		//nolint:govet // Valid OpenGL offset pointer usage
		gl.DrawElements(gl.TRIANGLES, group.indexCount, gl.UNSIGNED_INT, unsafe.Pointer(uintptr(group.startIndex*4)))
	}

	gl.BindVertexArray(0)

	// Restore fill mode
	if mv.wireframeMode {
		gl.PolygonMode(gl.FRONT_AND_BACK, gl.FILL)
	}

	// Draw axes overlay if enabled
	mv.renderAxes(view, projection)

	// Restore state
	gl.BindFramebuffer(gl.FRAMEBUFFER, uint32(prevFBO))
	gl.Viewport(prevViewport[0], prevViewport[1], prevViewport[2], prevViewport[3])

	return mv.colorTexture
}

func (mv *ModelViewer) calculateCameraPosition() math.Vec3 {
	// Spherical to Cartesian conversion
	cosX := float32(gomath.Cos(float64(mv.rotationX)))
	sinX := float32(gomath.Sin(float64(mv.rotationX)))
	cosY := float32(gomath.Cos(float64(mv.rotationY)))
	sinY := float32(gomath.Sin(float64(mv.rotationY)))

	x := mv.distance * cosX * sinY
	y := mv.distance * sinX
	z := mv.distance * cosX * cosY

	return math.Vec3{
		X: mv.centerX + x,
		Y: mv.centerY + y,
		Z: mv.centerZ + z,
	}
}

// HandleMouseDrag updates rotation based on mouse movement.
func (mv *ModelViewer) HandleMouseDrag(deltaX, deltaY float32) {
	mv.rotationY += deltaX * 0.01
	mv.rotationX += deltaY * 0.01

	// Clamp vertical rotation
	if mv.rotationX > 1.5 {
		mv.rotationX = 1.5
	}
	if mv.rotationX < -1.5 {
		mv.rotationX = -1.5
	}
}

// HandleMouseWheel updates zoom level.
func (mv *ModelViewer) HandleMouseWheel(delta float32) {
	mv.distance -= delta
	if mv.distance < 1 {
		mv.distance = 1
	}
	if mv.distance > 10000 {
		mv.distance = 10000
	}
}

// Reset resets camera to default position.
func (mv *ModelViewer) Reset() {
	mv.rotationX = 0.3
	mv.rotationY = 0.5
	mv.fitCamera()
}

// Animation control methods

// UpdateAnimation advances animation time by deltaMs milliseconds.
// Should be called each frame when animation is playing.
// Returns true if mesh was rebuilt.
func (mv *ModelViewer) UpdateAnimation(deltaMs float32) bool {
	if !mv.animPlaying || mv.currentRSM == nil || mv.animLength <= 0 {
		return false
	}

	// Advance time
	mv.animTime += deltaMs * mv.animSpeed

	// Handle looping
	if mv.animLooping {
		for mv.animTime >= float32(mv.animLength) {
			mv.animTime -= float32(mv.animLength)
		}
		for mv.animTime < 0 {
			mv.animTime += float32(mv.animLength)
		}
	} else {
		// Clamp to range
		if mv.animTime >= float32(mv.animLength) {
			mv.animTime = float32(mv.animLength)
			mv.animPlaying = false
		}
		if mv.animTime < 0 {
			mv.animTime = 0
		}
	}

	// Rebuild mesh with new animation time
	mv.rebuildMesh()
	return true
}

// rebuildMesh rebuilds the mesh with current animation time.
func (mv *ModelViewer) rebuildMesh() {
	if mv.currentRSM == nil {
		return
	}

	// Delete old buffers
	if mv.vao != 0 {
		gl.DeleteVertexArrays(1, &mv.vao)
		mv.vao = 0
	}
	if mv.vbo != 0 {
		gl.DeleteBuffers(1, &mv.vbo)
		mv.vbo = 0
	}
	if mv.ebo != 0 {
		gl.DeleteBuffers(1, &mv.ebo)
		mv.ebo = 0
	}

	// Rebuild with current animation time
	vertices, indices := mv.buildMeshFromRSM(mv.currentRSM, mv.animTime)
	if len(vertices) > 0 {
		mv.uploadMesh(vertices, indices)
	}
}

// PlayAnimation starts or resumes animation playback.
func (mv *ModelViewer) PlayAnimation() {
	mv.animPlaying = true
}

// PauseAnimation pauses animation playback.
func (mv *ModelViewer) PauseAnimation() {
	mv.animPlaying = false
}

// ToggleAnimation toggles between play and pause.
func (mv *ModelViewer) ToggleAnimation() {
	mv.animPlaying = !mv.animPlaying
}

// IsAnimationPlaying returns true if animation is playing.
func (mv *ModelViewer) IsAnimationPlaying() bool {
	return mv.animPlaying
}

// SetAnimationTime sets the animation time in milliseconds.
func (mv *ModelViewer) SetAnimationTime(timeMs float32) {
	mv.animTime = timeMs
	if mv.animTime < 0 {
		mv.animTime = 0
	}
	if mv.animTime > float32(mv.animLength) {
		mv.animTime = float32(mv.animLength)
	}
	mv.rebuildMesh()
}

// GetAnimationTime returns the current animation time in milliseconds.
func (mv *ModelViewer) GetAnimationTime() float32 {
	return mv.animTime
}

// GetAnimationLength returns the animation length in milliseconds.
func (mv *ModelViewer) GetAnimationLength() int32 {
	return mv.animLength
}

// SetAnimationSpeed sets the animation speed multiplier.
func (mv *ModelViewer) SetAnimationSpeed(speed float32) {
	mv.animSpeed = speed
}

// GetAnimationSpeed returns the current animation speed multiplier.
func (mv *ModelViewer) GetAnimationSpeed() float32 {
	return mv.animSpeed
}

// SetAnimationLooping sets whether animation should loop.
func (mv *ModelViewer) SetAnimationLooping(loop bool) {
	mv.animLooping = loop
}

// IsAnimationLooping returns true if animation loops.
func (mv *ModelViewer) IsAnimationLooping() bool {
	return mv.animLooping
}

// HasAnimation returns true if the loaded model has animation keyframes.
func (mv *ModelViewer) HasAnimation() bool {
	return mv.currentRSM != nil && mv.currentRSM.HasAnimation()
}

// GetCenter returns the model's center point (X, Y, Z).
func (mv *ModelViewer) GetCenter() [3]float32 {
	return [3]float32{mv.centerX, mv.centerY, mv.centerZ}
}

// GetBounds returns the model's bounding box (minX, minY, minZ, maxX, maxY, maxZ).
func (mv *ModelViewer) GetBounds() (min, max [3]float32) {
	return mv.minBounds, mv.maxBounds
}

// GetRootNodeOffset returns the root node's offset (pivot point) if available.
func (mv *ModelViewer) GetRootNodeOffset() [3]float32 {
	if mv.currentRSM == nil || len(mv.currentRSM.Nodes) == 0 {
		return [3]float32{0, 0, 0}
	}
	root := mv.currentRSM.GetRootNode()
	if root != nil {
		return root.Offset
	}
	return mv.currentRSM.Nodes[0].Offset
}

// createAxisVisualization creates the shader and geometry for axis visualization.
func (mv *ModelViewer) createAxisVisualization() error {
	// Compile line vertex shader
	vertexShader, err := compileModelShader(lineVertexShader, gl.VERTEX_SHADER)
	if err != nil {
		return fmt.Errorf("line vertex shader: %w", err)
	}
	defer gl.DeleteShader(vertexShader)

	// Compile line fragment shader
	fragmentShader, err := compileModelShader(lineFragmentShader, gl.FRAGMENT_SHADER)
	if err != nil {
		return fmt.Errorf("line fragment shader: %w", err)
	}
	defer gl.DeleteShader(fragmentShader)

	// Link program
	mv.axisShader = gl.CreateProgram()
	gl.AttachShader(mv.axisShader, vertexShader)
	gl.AttachShader(mv.axisShader, fragmentShader)
	gl.LinkProgram(mv.axisShader)

	var status int32
	gl.GetProgramiv(mv.axisShader, gl.LINK_STATUS, &status)
	if status == gl.FALSE {
		var logLength int32
		gl.GetProgramiv(mv.axisShader, gl.INFO_LOG_LENGTH, &logLength)
		log := strings.Repeat("\x00", int(logLength+1))
		gl.GetProgramInfoLog(mv.axisShader, logLength, nil, gl.Str(log))
		return fmt.Errorf("link failed: %s", log)
	}

	// Get uniform locations
	mv.axisLocView = gl.GetUniformLocation(mv.axisShader, gl.Str("uView\x00"))
	mv.axisLocProj = gl.GetUniformLocation(mv.axisShader, gl.Str("uProjection\x00"))

	// Create axis line geometry
	// Each line: 2 vertices with position (3 floats) + color (3 floats) = 6 floats per vertex
	// 3 axes x 2 vertices = 6 vertices total
	axisLength := float32(50.0) // World units
	axisData := []float32{
		// X axis (red) - from origin to positive X
		0, 0, 0, 1, 0, 0,
		axisLength, 0, 0, 1, 0, 0,
		// Y axis (green) - from origin to positive Y
		0, 0, 0, 0, 1, 0,
		0, axisLength, 0, 0, 1, 0,
		// Z axis (blue) - from origin to positive Z
		0, 0, 0, 0, 0, 1,
		0, 0, axisLength, 0, 0, 1,
	}

	// Create VAO for axes
	gl.GenVertexArrays(1, &mv.axisVAO)
	gl.BindVertexArray(mv.axisVAO)

	// Create VBO
	gl.GenBuffers(1, &mv.axisVBO)
	gl.BindBuffer(gl.ARRAY_BUFFER, mv.axisVBO)
	gl.BufferData(gl.ARRAY_BUFFER, len(axisData)*4, unsafe.Pointer(&axisData[0]), gl.STATIC_DRAW)

	// Position attribute (location = 0)
	gl.VertexAttribPointerWithOffset(0, 3, gl.FLOAT, false, 24, 0) // 6 floats * 4 bytes = 24 stride
	gl.EnableVertexAttribArray(0)

	// Color attribute (location = 1)
	gl.VertexAttribPointerWithOffset(1, 3, gl.FLOAT, false, 24, 12) // Offset 3 floats * 4 = 12
	gl.EnableVertexAttribArray(1)

	gl.BindVertexArray(0)

	return nil
}

// renderAxes draws the XYZ axis visualization.
func (mv *ModelViewer) renderAxes(view, projection math.Mat4) {
	if !mv.showAxes || mv.axisVAO == 0 {
		return
	}

	// Use line shader
	gl.UseProgram(mv.axisShader)

	// Set uniforms
	gl.UniformMatrix4fv(mv.axisLocView, 1, false, view.Ptr())
	gl.UniformMatrix4fv(mv.axisLocProj, 1, false, projection.Ptr())

	// Draw thicker lines for visibility
	gl.LineWidth(2.0)

	// Disable depth test so axes are always visible
	gl.Disable(gl.DEPTH_TEST)

	gl.BindVertexArray(mv.axisVAO)
	gl.DrawArrays(gl.LINES, 0, 6) // 6 vertices = 3 lines
	gl.BindVertexArray(0)

	// Re-enable depth test
	gl.Enable(gl.DEPTH_TEST)
}

// SetShowAxes toggles axis visualization.
func (mv *ModelViewer) SetShowAxes(show bool) {
	mv.showAxes = show
}

// ShowAxes returns whether axis visualization is enabled.
func (mv *ModelViewer) ShowAxes() bool {
	return mv.showAxes
}

// SetWireframeMode enables or disables wireframe rendering.
func (mv *ModelViewer) SetWireframeMode(enabled bool) {
	mv.wireframeMode = enabled
}

// WireframeMode returns whether wireframe rendering is enabled.
func (mv *ModelViewer) WireframeMode() bool {
	return mv.wireframeMode
}

// SetNodeVisibility sets visibility for a specific node.
func (mv *ModelViewer) SetNodeVisibility(nodeName string, visible bool) {
	if mv.nodeVisibility == nil {
		mv.nodeVisibility = make(map[string]bool)
	}
	mv.nodeVisibility[nodeName] = visible
	// Rebuild mesh with new visibility settings
	mv.rebuildMesh()
}

// GetNodeVisibility returns whether a specific node is visible.
// Returns true if node is not in the map (visible by default).
func (mv *ModelViewer) GetNodeVisibility(nodeName string) bool {
	if mv.nodeVisibility == nil {
		return true // Visible by default
	}
	visible, ok := mv.nodeVisibility[nodeName]
	if !ok {
		return true // Visible by default if not explicitly set
	}
	return visible
}

// SetAllNodesVisible sets all nodes to visible.
func (mv *ModelViewer) SetAllNodesVisible() {
	mv.nodeVisibility = make(map[string]bool)
	mv.rebuildMesh()
}

// GetNodeNames returns the names of all nodes in the current model.
func (mv *ModelViewer) GetNodeNames() []string {
	if mv.currentRSM == nil {
		return nil
	}
	names := make([]string, len(mv.currentRSM.Nodes))
	for i := range mv.currentRSM.Nodes {
		names[i] = mv.currentRSM.Nodes[i].Name
	}
	return names
}

func (mv *ModelViewer) clearModel() {
	if mv.vao != 0 {
		gl.DeleteVertexArrays(1, &mv.vao)
		mv.vao = 0
	}
	if mv.vbo != 0 {
		gl.DeleteBuffers(1, &mv.vbo)
		mv.vbo = 0
	}
	if mv.ebo != 0 {
		gl.DeleteBuffers(1, &mv.ebo)
		mv.ebo = 0
	}

	// Delete model textures (but not fallback)
	for _, tex := range mv.modelTextures {
		if tex != 0 && tex != mv.fallbackTexture {
			gl.DeleteTextures(1, &tex)
		}
	}
	mv.modelTextures = nil
	mv.indexCount = 0
}

// Destroy releases all OpenGL resources.
func (mv *ModelViewer) Destroy() {
	mv.clearModel()

	if mv.fallbackTexture != 0 {
		gl.DeleteTextures(1, &mv.fallbackTexture)
	}
	if mv.shaderProgram != 0 {
		gl.DeleteProgram(mv.shaderProgram)
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
	// Clean up axis visualization
	if mv.axisShader != 0 {
		gl.DeleteProgram(mv.axisShader)
	}
	if mv.axisVAO != 0 {
		gl.DeleteVertexArrays(1, &mv.axisVAO)
	}
	if mv.axisVBO != 0 {
		gl.DeleteBuffers(1, &mv.axisVBO)
	}
}
