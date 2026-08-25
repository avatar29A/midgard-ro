// Package scene provides a reusable 3D scene rendering system.
package scene

import (
	"bytes"
	"fmt"
	"image"
	gomath "math"
	"strings"
	"time"
	"unsafe"

	"github.com/go-gl/gl/v4.1-core/gl"
	"go.uber.org/zap"

	rsmmodel "github.com/Faultbox/midgard-ro/internal/engine/model"
	"github.com/Faultbox/midgard-ro/internal/engine/scene/shaders"
	"github.com/Faultbox/midgard-ro/internal/engine/shader"
	"github.com/Faultbox/midgard-ro/internal/engine/shadow"
	"github.com/Faultbox/midgard-ro/internal/engine/texture"
	"github.com/Faultbox/midgard-ro/internal/trace"
	"github.com/Faultbox/midgard-ro/pkg/formats"
	"github.com/Faultbox/midgard-ro/pkg/math"
)

// MapModel represents a placed RSM model in the scene.
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
	modelName  string
	Visible    bool

	// Bounding sphere around the mesh as uploaded, before the instance's own
	// transform is applied.
	localCenter [3]float32
	localRadius float32

	// The same sphere in world space, for frustum culling. Models never move
	// once loaded, so this is computed once by computeWorldBounds rather than
	// rebuilt per frame.
	worldCenter [3]float32
	worldRadius float32

	// depthBias separates this instance from any it may overlap. See
	// assignDepthBiases.
	depthBias int
}

// ModelRenderer handles rendering of RSM models.
type ModelRenderer struct {
	// Shader
	program uint32

	// Uniform locations
	locMVP            int32
	locModel          int32
	locLightDir       int32
	locAmbient        int32
	locDiffuse        int32
	locTexture        int32
	locFogUse         int32
	locFogNear        int32
	locFogFar         int32
	locFogColor       int32
	locLightViewProj  int32
	locShadowMap      int32
	locShadowsEnabled int32

	// Point light uniforms
	locPointLightPositions   int32
	locPointLightColors      int32
	locPointLightRanges      int32
	locPointLightIntensities int32
	locPointLightCount       int32
	locPointLightsEnabled    int32

	// Models
	models []*MapModel

	// Map dimensions for coordinate conversion
	mapWidth  float32
	mapHeight float32

	// Fallback texture
	fallbackTex uint32

	// Force all faces to render as two-sided
	ForceAllTwoSided bool

	// Cull counters from the last Render, reported on the render trace.
	drawnModels    int
	culledModels   int
	cullTraceTimer time.Time
}

// NewModelRenderer creates a new model renderer.
func NewModelRenderer() (*ModelRenderer, error) {
	mr := &ModelRenderer{
		ForceAllTwoSided: true,
	}

	program, err := shader.CompileProgram(shaders.ModelVertexShader, shaders.ModelFragmentShader)
	if err != nil {
		return nil, fmt.Errorf("model shader: %w", err)
	}
	mr.program = program

	// Get uniform locations
	mr.locMVP = shader.GetUniform(program, "uMVP")
	mr.locModel = shader.GetUniform(program, "uModel")
	mr.locLightDir = shader.GetUniform(program, "uLightDir")
	mr.locAmbient = shader.GetUniform(program, "uAmbient")
	mr.locDiffuse = shader.GetUniform(program, "uDiffuse")
	mr.locTexture = shader.GetUniform(program, "uTexture")
	mr.locFogUse = shader.GetUniform(program, "uFogUse")
	mr.locFogNear = shader.GetUniform(program, "uFogNear")
	mr.locFogFar = shader.GetUniform(program, "uFogFar")
	mr.locFogColor = shader.GetUniform(program, "uFogColor")
	mr.locLightViewProj = shader.GetUniform(program, "uLightViewProj")
	mr.locShadowMap = shader.GetUniform(program, "uShadowMap")
	mr.locShadowsEnabled = shader.GetUniform(program, "uShadowsEnabled")

	// Point light uniforms
	mr.locPointLightPositions = shader.GetUniform(program, "uPointLightPositions")
	mr.locPointLightColors = shader.GetUniform(program, "uPointLightColors")
	mr.locPointLightRanges = shader.GetUniform(program, "uPointLightRanges")
	mr.locPointLightIntensities = shader.GetUniform(program, "uPointLightIntensities")
	mr.locPointLightCount = shader.GetUniform(program, "uPointLightCount")
	mr.locPointLightsEnabled = shader.GetUniform(program, "uPointLightsEnabled")

	return mr, nil
}

// LoadModels loads all RSM models from RSW.
func (mr *ModelRenderer) LoadModels(rsw *formats.RSW, texLoader func(string) ([]byte, error), fallbackTex uint32,
	mapWidth, mapHeight float32, terrainAltitudes [][]float32, terrainTileZoom float32, terrainTilesX, terrainTilesZ int) error {

	mr.clearModels()
	mr.fallbackTex = fallbackTex
	mr.mapWidth = mapWidth
	mr.mapHeight = mapHeight

	allModels := rsw.GetModels()

	// Limit models for performance
	maxModels := 1500
	models := allModels
	if len(models) > maxModels {
		models = models[:maxModels]
	}

	// Cache loaded RSM files
	rsmCache := make(map[string]*formats.RSM)

	for _, modelRef := range models {
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

		mapModel := mr.buildMapModel(rsm, modelRef, texLoader)
		if mapModel != nil {
			mr.models = append(mr.models, mapModel)
		}
	}

	mr.assignDepthBiases()
	mr.computeWorldBounds()

	return nil
}

// computeWorldBounds places each model's bounding sphere in world space.
//
// Rotation cannot change a sphere's radius, so only the scale matters, and a
// non-uniform scale stretches by at most its largest axis. Taking that largest
// axis keeps the sphere conservative — never smaller than the geometry.
func (mr *ModelRenderer) computeWorldBounds() {
	offsetX := mr.mapWidth / 2
	offsetZ := mr.mapHeight / 2

	for _, model := range mr.models {
		if model == nil {
			continue
		}
		matrix := mr.buildModelMatrix(model, offsetX, offsetZ)
		model.worldCenter = matrix.TransformPoint(model.localCenter)

		maxScale := float32(0)
		for _, axis := range model.scale {
			if s := float32(gomath.Abs(float64(axis))); s > maxScale {
				maxScale = s
			}
		}
		model.worldRadius = model.localRadius * maxScale
	}
}

// CullStats returns how many models were drawn and how many were skipped by
// the last Render.
func (mr *ModelRenderer) CullStats() (drawn, culled int) {
	return mr.drawnModels, mr.culledModels
}

// assignDepthBiases gives every model instance a depth bias such that no two
// instances close enough to overlap share one.
//
// Map data places instances that are exactly coplanar — Prontera's wall is a
// 30-unit segment repeated every 28.3 units at identical height and rotation,
// and elsewhere the same model appears twice at the very same coordinates.
// Coplanar surfaces have no depth difference to resolve, so which one shows is
// decided by floating-point noise and flips as the camera moves.
//
// Deriving the bias arithmetically from the instance index, its position, or a
// hash of either cannot fix this: with a small set of biases roughly one pair
// in N collides whatever the formula, and a set large enough to make that
// unlikely needs biases too large to stay invisible. Measured over five towns,
// an eight-value hash left 11-14% of overlapping pairs sharing a bias.
//
// Coloring the overlap graph removes the guesswork. Overlaps are local — the
// worst instance across those maps overlapped 22 others — so a greedy pass
// needs very few colors and separates every pair by construction rather than
// by probability.
func (mr *ModelRenderer) assignDepthBiases() {
	// Instances whose centers are closer than the largest model dimension may
	// share space. Being generous here only costs a color or two.
	const overlapRange = 30.0

	neighbors := make([][]int, len(mr.models))
	for a := range mr.models {
		for b := a + 1; b < len(mr.models); b++ {
			dx := float64(mr.models[a].position[0] - mr.models[b].position[0])
			dz := float64(mr.models[a].position[2] - mr.models[b].position[2])
			if gomath.Hypot(dx, dz) >= overlapRange {
				continue
			}
			neighbors[a] = append(neighbors[a], b)
			neighbors[b] = append(neighbors[b], a)
		}
	}

	for i := range mr.models {
		mr.models[i].depthBias = -1
	}

	taken := make(map[int]bool, 8)
	for i := range mr.models {
		for k := range taken {
			delete(taken, k)
		}
		for _, n := range neighbors[i] {
			if mr.models[n].depthBias >= 0 {
				taken[mr.models[n].depthBias] = true
			}
		}
		bias := 0
		for taken[bias] {
			bias++
		}
		mr.models[i].depthBias = bias
	}
}

func (mr *ModelRenderer) buildMapModel(rsm *formats.RSM, ref *formats.RSWModel, texLoader func(string) ([]byte, error)) *MapModel {
	if len(rsm.Nodes) == 0 {
		return nil
	}

	var vertices []rsmmodel.Vertex
	var indices []uint32
	texGroups := make(map[int][]uint32)

	// Load model textures
	modelTextures := make([]uint32, len(rsm.Textures))
	for i, texName := range rsm.Textures {
		texPath := "data/texture/" + texName
		data, err := texLoader(texPath)
		if err != nil {
			modelTextures[i] = mr.fallbackTex
			continue
		}
		img, err := mr.decodeTexture(data, texPath)
		if err != nil {
			modelTextures[i] = mr.fallbackTex
			continue
		}
		modelTextures[i] = mr.uploadTexture(img)
	}

	// Track bounding box
	var minX, minY, minZ float32 = 1e10, 1e10, 1e10
	var maxX, maxY, maxZ float32 = -1e10, -1e10, -1e10

	// Process each node
	reverseWinding := ref.Scale[0]*ref.Scale[1]*ref.Scale[2] < 0

	for i := range rsm.Nodes {
		node := &rsm.Nodes[i]
		nodeMatrix := rsmmodel.BuildNodeMatrix(node, rsm, 0)

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

			// Calculate normal
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

			addFaceVertices := func(reverseOrder bool, flipNormal bool) uint32 {
				faceBaseIdx := uint32(len(vertices))
				faceNormal := normal
				if flipNormal {
					faceNormal = [3]float32{-normal[0], -normal[1], -normal[2]}
				}

				var vertIDs, texIDs [3]uint16
				if reverseOrder {
					vertIDs = [3]uint16{face.VertexIDs[2], face.VertexIDs[1], face.VertexIDs[0]}
					texIDs = [3]uint16{face.TexCoordIDs[2], face.TexCoordIDs[1], face.TexCoordIDs[0]}
				} else {
					vertIDs = face.VertexIDs
					texIDs = face.TexCoordIDs
				}

				for j := 0; j < 3; j++ {
					vid := vertIDs[j]
					v := node.Vertices[vid]
					pos := rsmmodel.TransformPoint(nodeMatrix, v)
					pos[1] = -pos[1]

					// Track bounds
					if pos[0] < minX {
						minX = pos[0]
					}
					if pos[0] > maxX {
						maxX = pos[0]
					}
					if pos[1] < minY {
						minY = pos[1]
					}
					if pos[1] > maxY {
						maxY = pos[1]
					}
					if pos[2] < minZ {
						minZ = pos[2]
					}
					if pos[2] > maxZ {
						maxZ = pos[2]
					}

					var uv [2]float32
					if int(texIDs[j]) < len(node.TexCoords) {
						tc := node.TexCoords[texIDs[j]]
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

			faceBaseIdx := addFaceVertices(reverseWinding, false)
			globalTexIdx := 0
			if int(face.TextureID) < len(node.TextureIDs) {
				globalTexIdx = int(node.TextureIDs[face.TextureID])
			}
			texGroups[globalTexIdx] = append(texGroups[globalTexIdx], faceBaseIdx, faceBaseIdx+1, faceBaseIdx+2)

			// Add back face for two-sided
			if face.TwoSide != 0 || mr.ForceAllTwoSided {
				backFaceBaseIdx := addFaceVertices(!reverseWinding, true)
				texGroups[globalTexIdx] = append(texGroups[globalTexIdx], backFaceBaseIdx, backFaceBaseIdx+1, backFaceBaseIdx+2)
			}
		}
	}

	if len(vertices) == 0 {
		return nil
	}

	// Center model
	centerX := (minX + maxX) / 2
	centerZ := (minZ + maxZ) / 2
	for i := range vertices {
		vertices[i].Position[0] -= centerX
		vertices[i].Position[2] -= centerZ
	}

	// Derive the bounding sphere from the same vertices that get uploaded, so
	// it cannot drift from the geometry the way a separately tracked box can.
	localCenter, localRadius := boundingSphere(vertices)

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

	// Smooth normals
	rsmmodel.SmoothNormals(vertices)

	// Create GPU resources
	model := &MapModel{
		textures:    modelTextures,
		texGroups:   groups,
		position:    ref.Position,
		rotation:    ref.Rotation,
		scale:       ref.Scale,
		modelName:   ref.ModelName,
		Visible:     true,
		localCenter: localCenter,
		localRadius: localRadius,
	}

	// Upload mesh
	mr.uploadMesh(model, vertices, indices)

	return model
}

// boundingSphere returns the center and radius of a sphere enclosing every
// vertex. The center is the midpoint of the axis-aligned bounds and the radius
// the farthest vertex from it, which is tighter than taking the box diagonal
// and still encloses everything.
func boundingSphere(vertices []rsmmodel.Vertex) (center [3]float32, radius float32) {
	if len(vertices) == 0 {
		return center, 0
	}

	minP := vertices[0].Position
	maxP := vertices[0].Position
	for _, v := range vertices[1:] {
		for axis := 0; axis < 3; axis++ {
			if v.Position[axis] < minP[axis] {
				minP[axis] = v.Position[axis]
			}
			if v.Position[axis] > maxP[axis] {
				maxP[axis] = v.Position[axis]
			}
		}
	}

	for axis := 0; axis < 3; axis++ {
		center[axis] = (minP[axis] + maxP[axis]) / 2
	}

	var maxSq float32
	for _, v := range vertices {
		dx := v.Position[0] - center[0]
		dy := v.Position[1] - center[1]
		dz := v.Position[2] - center[2]
		if sq := dx*dx + dy*dy + dz*dz; sq > maxSq {
			maxSq = sq
		}
	}

	return center, float32(gomath.Sqrt(float64(maxSq)))
}

func (mr *ModelRenderer) uploadMesh(model *MapModel, vertices []rsmmodel.Vertex, indices []uint32) {
	gl.GenVertexArrays(1, &model.vao)
	gl.BindVertexArray(model.vao)

	gl.GenBuffers(1, &model.vbo)
	gl.BindBuffer(gl.ARRAY_BUFFER, model.vbo)
	vertexSize := int(unsafe.Sizeof(rsmmodel.Vertex{}))
	gl.BufferData(gl.ARRAY_BUFFER, len(vertices)*vertexSize, unsafe.Pointer(&vertices[0]), gl.STATIC_DRAW)

	// Position
	gl.VertexAttribPointerWithOffset(0, 3, gl.FLOAT, false, int32(vertexSize), 0)
	gl.EnableVertexAttribArray(0)
	// Normal
	gl.VertexAttribPointerWithOffset(1, 3, gl.FLOAT, false, int32(vertexSize), 3*4)
	gl.EnableVertexAttribArray(1)
	// TexCoord
	gl.VertexAttribPointerWithOffset(2, 2, gl.FLOAT, false, int32(vertexSize), 6*4)
	gl.EnableVertexAttribArray(2)

	gl.GenBuffers(1, &model.ebo)
	gl.BindBuffer(gl.ELEMENT_ARRAY_BUFFER, model.ebo)
	gl.BufferData(gl.ELEMENT_ARRAY_BUFFER, len(indices)*4, unsafe.Pointer(&indices[0]), gl.STATIC_DRAW)

	model.indexCount = int32(len(indices))
	gl.BindVertexArray(0)
}

func (mr *ModelRenderer) decodeTexture(data []byte, path string) (*image.RGBA, error) {
	lowerPath := strings.ToLower(path)
	var img image.Image
	var err error
	if strings.HasSuffix(lowerPath, ".tga") {
		img, err = texture.DecodeTGA(data)
	} else {
		img, _, err = image.Decode(bytes.NewReader(data))
	}
	if err != nil {
		return nil, err
	}
	return texture.ImageToRGBA(img, true), nil
}

func (mr *ModelRenderer) uploadTexture(img *image.RGBA) uint32 {
	var texID uint32
	gl.GenTextures(1, &texID)
	gl.BindTexture(gl.TEXTURE_2D, texID)
	gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA, int32(img.Bounds().Dx()), int32(img.Bounds().Dy()), 0, gl.RGBA, gl.UNSIGNED_BYTE, unsafe.Pointer(&img.Pix[0]))
	gl.GenerateMipmap(gl.TEXTURE_2D)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR_MIPMAP_LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.REPEAT)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.REPEAT)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAX_LEVEL, 4)
	gl.TexParameterf(gl.TEXTURE_2D, gl.TEXTURE_MAX_ANISOTROPY, 8.0)
	return texID
}

// Render renders all visible models.
// Depth bias applied per model instance, to break ties between map geometry
// that is exactly coplanar.
//
// glPolygonOffset computes factor*m + r*units, where m is the polygon's
// screen-space depth slope and r is the smallest resolvable depth step. Both
// halves are needed. The units term separates surfaces seen face-on; the
// factor term scales with slope, which is what covers them at grazing angles.
//
// Biasing on units alone left the joins stable while walking and glitching
// across every segment while turning, because turning is what sweeps these
// walls through grazing angles, where the interpolation error between two
// coplanar surfaces grows with the slope and swamps a constant offset.
//
// The values cycle so a large map cannot accumulate a bias big enough to see;
// instances that overlap are neighbors in the world file, so their indices
// differ by far less than the cycle length.
const (
	coplanarSlopeStep = 0.25
	coplanarUnitsStep = 2.0
)

func (mr *ModelRenderer) Render(viewProj math.Mat4, lightDir, ambient, diffuse [3]float32,
	shadowsEnabled bool, lightViewProj math.Mat4, shadowMap *shadow.Map,
	pointLightsEnabled bool, pointLights []PointLight, pointLightIntensity float32,
	fogEnabled bool, fogNear, fogFar float32, fogColor [3]float32) {

	if len(mr.models) == 0 {
		return
	}

	gl.UseProgram(mr.program)

	// Set light uniforms
	gl.Uniform3f(mr.locLightDir, lightDir[0], lightDir[1], lightDir[2])
	gl.Uniform3f(mr.locAmbient, ambient[0], ambient[1], ambient[2])
	gl.Uniform3f(mr.locDiffuse, diffuse[0], diffuse[1], diffuse[2])

	// Fog
	if fogEnabled {
		gl.Uniform1i(mr.locFogUse, 1)
		gl.Uniform1f(mr.locFogNear, fogNear)
		gl.Uniform1f(mr.locFogFar, fogFar)
		gl.Uniform3f(mr.locFogColor, fogColor[0], fogColor[1], fogColor[2])
	} else {
		gl.Uniform1i(mr.locFogUse, 0)
	}

	// Shadows
	if shadowsEnabled && shadowMap != nil {
		gl.Uniform1i(mr.locShadowsEnabled, 1)
		gl.UniformMatrix4fv(mr.locLightViewProj, 1, false, &lightViewProj[0])
		gl.ActiveTexture(gl.TEXTURE1)
		gl.BindTexture(gl.TEXTURE_2D, shadowMap.DepthTexture)
		gl.Uniform1i(mr.locShadowMap, 1)
	} else {
		gl.Uniform1i(mr.locShadowsEnabled, 0)
	}

	// Point lights
	if pointLightsEnabled && len(pointLights) > 0 {
		gl.Uniform1i(mr.locPointLightsEnabled, 1)
		count := len(pointLights)
		if count > MaxPointLights {
			count = MaxPointLights
		}
		gl.Uniform1i(mr.locPointLightCount, int32(count))

		positions := make([]float32, count*3)
		colors := make([]float32, count*3)
		ranges := make([]float32, count)
		intensities := make([]float32, count)

		for i := 0; i < count; i++ {
			positions[i*3] = pointLights[i].Position[0]
			positions[i*3+1] = pointLights[i].Position[1]
			positions[i*3+2] = pointLights[i].Position[2]
			colors[i*3] = pointLights[i].Color[0]
			colors[i*3+1] = pointLights[i].Color[1]
			colors[i*3+2] = pointLights[i].Color[2]
			ranges[i] = pointLights[i].Range
			intensities[i] = pointLights[i].Intensity * pointLightIntensity
		}

		gl.Uniform3fv(mr.locPointLightPositions, int32(count), &positions[0])
		gl.Uniform3fv(mr.locPointLightColors, int32(count), &colors[0])
		gl.Uniform1fv(mr.locPointLightRanges, int32(count), &ranges[0])
		gl.Uniform1fv(mr.locPointLightIntensities, int32(count), &intensities[0])
	} else {
		gl.Uniform1i(mr.locPointLightsEnabled, 0)
	}

	gl.ActiveTexture(gl.TEXTURE0)
	gl.Uniform1i(mr.locTexture, 0)

	// Render each model
	offsetX := mr.mapWidth / 2
	offsetZ := mr.mapHeight / 2

	// Map data places model instances that are exactly coplanar with each
	// other. Prontera's wall is built from a 30-unit segment repeated every
	// 28.3 units at identical height and rotation, so each adjacent pair
	// shares a 1.7-unit strip lying in the same plane.
	//
	// Surfaces in the same plane have no depth difference to resolve, so which
	// one is in front is decided by floating-point noise in each instance's
	// transform, and it flips as the camera moves — the flicker at wall joins.
	// No amount of depth precision fixes that; there is nothing to resolve.
	// More precision makes it worse, by resolving the noise itself.
	//
	// A per-instance depth bias breaks the tie deterministically instead, on
	// both the constant and the slope-dependent term — see the constants above
	// for why the slope term matters.
	gl.Enable(gl.POLYGON_OFFSET_FILL)
	defer gl.Disable(gl.POLYGON_OFFSET_FILL)
	defer gl.PolygonOffset(0, 0)

	// Most of a town is behind the camera or off to the sides at any moment,
	// and scene rendering is nearly all of the frame. Testing a bounding
	// sphere against the six clip planes costs a handful of dot products and
	// skips the matrix build, uniform uploads and draw calls behind it.
	frustum := FrustumFromViewProj(viewProj)
	mr.drawnModels, mr.culledModels = 0, 0

	for _, model := range mr.models {
		if model == nil || !model.Visible || model.vao == 0 {
			continue
		}

		if !frustum.ContainsSphere(model.worldCenter, model.worldRadius) {
			mr.culledModels++
			continue
		}
		mr.drawnModels++

		// Wrapped so a large map cannot accumulate a bias big enough to show.
		// Instances that overlap are neighbors in the world file, so their
		// indices differ by far less than the wrap.
		bias := float32(model.depthBias)
		gl.PolygonOffset(bias*coplanarSlopeStep, bias*coplanarUnitsStep)

		// Build model matrix
		modelMatrix := mr.buildModelMatrix(model, offsetX, offsetZ)
		mvp := viewProj.Mul(modelMatrix)

		gl.UniformMatrix4fv(mr.locMVP, 1, false, &mvp[0])
		gl.UniformMatrix4fv(mr.locModel, 1, false, &modelMatrix[0])

		gl.BindVertexArray(model.vao)

		// Draw each texture group
		for _, group := range model.texGroups {
			tex := mr.fallbackTex
			if group.TextureIdx >= 0 && group.TextureIdx < len(model.textures) && model.textures[group.TextureIdx] != 0 {
				tex = model.textures[group.TextureIdx]
			}
			gl.BindTexture(gl.TEXTURE_2D, tex)
			gl.DrawElementsWithOffset(gl.TRIANGLES, group.IndexCount, gl.UNSIGNED_INT, uintptr(group.StartIndex*4))
		}
	}

	gl.BindVertexArray(0)

	mr.traceCullStats()
}

// traceCullStats reports how much of the map the camera is skipping. Rate
// limited to once a second so the trace can be left on while playing, matching
// how frame costs are reported.
func (mr *ModelRenderer) traceCullStats() {
	if !trace.On(trace.Render) || time.Since(mr.cullTraceTimer) < time.Second {
		return
	}
	mr.cullTraceTimer = time.Now()

	total := mr.drawnModels + mr.culledModels
	var fraction float64
	if total > 0 {
		fraction = float64(mr.culledModels) / float64(total)
	}

	trace.Emit(trace.Render, "cull",
		zap.Int("drawn", mr.drawnModels),
		zap.Int("culled", mr.culledModels),
		zap.Int("total", total),
		zap.Float64("culledFraction", fraction))
}

func (mr *ModelRenderer) buildModelMatrix(model *MapModel, offsetX, offsetZ float32) math.Mat4 {
	// Translation: RSW position + map center offset
	worldX := model.position[0] + offsetX
	worldY := -model.position[1]
	worldZ := model.position[2] + offsetZ

	result := math.Translate(worldX, worldY, worldZ)

	// Rotation (RSW uses degrees, XYZ order)
	rotX := model.rotation[0] * (gomath.Pi / 180.0)
	rotY := model.rotation[1] * (gomath.Pi / 180.0)
	rotZ := model.rotation[2] * (gomath.Pi / 180.0)

	result = result.Mul(math.RotateY(float32(rotY)))
	result = result.Mul(math.RotateX(float32(rotX)))
	result = result.Mul(math.RotateZ(float32(rotZ)))

	// Scale
	result = result.Mul(math.Scale(model.scale[0], model.scale[1], model.scale[2]))

	return result
}

// RenderShadow renders all models to the shadow map.
//
// Deliberately not frustum culled. The shadow pass is drawn from the light,
// not the camera, and a building standing behind the camera can still cast
// onto ground that is in view — culling here by the camera's frustum would
// delete those shadows. Culling it would need the light's own frustum, which
// on a directional light covers the whole map and so saves little.
func (mr *ModelRenderer) RenderShadow(shadowProgram uint32, locModel int32) {
	offsetX := mr.mapWidth / 2
	offsetZ := mr.mapHeight / 2

	for _, model := range mr.models {
		if model == nil || !model.Visible || model.vao == 0 {
			continue
		}

		modelMatrix := mr.buildModelMatrix(model, offsetX, offsetZ)
		gl.UniformMatrix4fv(locModel, 1, false, &modelMatrix[0])

		gl.BindVertexArray(model.vao)
		gl.DrawElements(gl.TRIANGLES, model.indexCount, gl.UNSIGNED_INT, nil)
	}
	gl.BindVertexArray(0)
}

func (mr *ModelRenderer) clearModels() {
	for _, model := range mr.models {
		if model == nil {
			continue
		}
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
			if tex != 0 && tex != mr.fallbackTex {
				gl.DeleteTextures(1, &tex)
			}
		}
	}
	mr.models = nil
}

// Destroy releases all resources.
func (mr *ModelRenderer) Destroy() {
	mr.clearModels()
	if mr.program != 0 {
		gl.DeleteProgram(mr.program)
		mr.program = 0
	}
}
