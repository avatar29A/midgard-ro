// Package scene provides a reusable 3D scene rendering system.
package scene

import (
	"bytes"
	"fmt"
	"image"
	"strings"
	"unsafe"

	"github.com/go-gl/gl/v4.1-core/gl"

	"github.com/Faultbox/midgard-ro/internal/engine/scene/shaders"
	"github.com/Faultbox/midgard-ro/internal/engine/shader"
	"github.com/Faultbox/midgard-ro/internal/engine/shadow"
	"github.com/Faultbox/midgard-ro/internal/engine/terrain"
	"github.com/Faultbox/midgard-ro/internal/engine/texture"
	"github.com/Faultbox/midgard-ro/pkg/formats"
	"github.com/Faultbox/midgard-ro/pkg/math"
)

// TerrainRenderer handles rendering of terrain (GND) data.
type TerrainRenderer struct {
	// Shader
	program uint32

	// Uniform locations
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

	// Shadow uniforms
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

	// Terrain mesh
	vao    uint32
	vbo    uint32
	ebo    uint32
	groups []terrain.TextureGroup

	// Textures
	groundTextures   map[int]uint32
	lightmapAtlasTex uint32
	lightmapAtlas    *terrain.LightmapAtlas

	// Bounds
	MinBounds [3]float32
	MaxBounds [3]float32
}

// NewTerrainRenderer creates a new terrain renderer.
func NewTerrainRenderer() (*TerrainRenderer, error) {
	tr := &TerrainRenderer{
		groundTextures: make(map[int]uint32),
	}

	program, err := shader.CompileProgram(shaders.TerrainVertexShader, shaders.TerrainFragmentShader)
	if err != nil {
		return nil, fmt.Errorf("terrain shader: %w", err)
	}
	tr.program = program

	// Get uniform locations
	tr.locViewProj = shader.GetUniform(program, "uViewProj")
	tr.locLightDir = shader.GetUniform(program, "uLightDir")
	tr.locAmbient = shader.GetUniform(program, "uAmbient")
	tr.locDiffuse = shader.GetUniform(program, "uDiffuse")
	tr.locTexture = shader.GetUniform(program, "uTexture")
	tr.locLightmap = shader.GetUniform(program, "uLightmap")
	tr.locBrightness = shader.GetUniform(program, "uBrightness")
	tr.locLightOpacity = shader.GetUniform(program, "uLightOpacity")
	tr.locFogUse = shader.GetUniform(program, "uFogUse")
	tr.locFogNear = shader.GetUniform(program, "uFogNear")
	tr.locFogFar = shader.GetUniform(program, "uFogFar")
	tr.locFogColor = shader.GetUniform(program, "uFogColor")

	// Shadow uniforms
	tr.locLightViewProj = shader.GetUniform(program, "uLightViewProj")
	tr.locShadowMap = shader.GetUniform(program, "uShadowMap")
	tr.locShadowsEnabled = shader.GetUniform(program, "uShadowsEnabled")

	// Point light uniforms
	tr.locPointLightPositions = shader.GetUniform(program, "uPointLightPositions")
	tr.locPointLightColors = shader.GetUniform(program, "uPointLightColors")
	tr.locPointLightRanges = shader.GetUniform(program, "uPointLightRanges")
	tr.locPointLightIntensities = shader.GetUniform(program, "uPointLightIntensities")
	tr.locPointLightCount = shader.GetUniform(program, "uPointLightCount")
	tr.locPointLightsEnabled = shader.GetUniform(program, "uPointLightsEnabled")

	return tr, nil
}

// LoadTerrain loads terrain data from GND.
func (tr *TerrainRenderer) LoadTerrain(gnd *formats.GND, texLoader func(string) ([]byte, error), fallbackTex uint32) error {
	// Clear old resources
	tr.clearTerrain()

	// Load ground textures
	tr.loadGroundTextures(gnd, texLoader, fallbackTex)

	// Build lightmap atlas
	tr.lightmapAtlas = terrain.BuildLightmapAtlas(gnd)
	tr.uploadLightmapAtlas()

	// Build terrain mesh
	mesh := terrain.BuildMesh(gnd, tr.lightmapAtlas)
	tr.groups = mesh.Groups
	tr.MinBounds = mesh.Bounds.Min
	tr.MaxBounds = mesh.Bounds.Max

	// Upload to GPU
	tr.uploadTerrainMesh(mesh.Vertices, mesh.Indices)

	return nil
}

func (tr *TerrainRenderer) loadGroundTextures(gnd *formats.GND, texLoader func(string) ([]byte, error), fallbackTex uint32) {
	for i, texPath := range gnd.Textures {
		fullPath := "data/texture/" + texPath

		data, err := texLoader(fullPath)
		if err != nil {
			// Try with backslash path format (GRF files use Windows paths)
			fullPath = "data\\texture\\" + texPath
			data, err = texLoader(fullPath)
		}
		if err != nil {
			tr.groundTextures[i] = fallbackTex
			continue
		}

		img, err := tr.decodeTexture(data, texPath)
		if err != nil {
			tr.groundTextures[i] = fallbackTex
			continue
		}

		tr.groundTextures[i] = tr.uploadTexture(img)
	}
}

func (tr *TerrainRenderer) decodeTexture(data []byte, path string) (*image.RGBA, error) {
	lowerPath := strings.ToLower(path)

	var img image.Image
	var err error

	if strings.HasSuffix(lowerPath, ".tga") {
		img, err = texture.DecodeTGA(data)
	} else {
		img, _, err = image.Decode(bytes.NewReader(data))
	}

	if err != nil {
		return nil, fmt.Errorf("decode %s: %w", path, err)
	}

	return texture.ImageToRGBA(img, true), nil
}

func (tr *TerrainRenderer) uploadTexture(img *image.RGBA) uint32 {
	var texID uint32
	gl.GenTextures(1, &texID)
	gl.BindTexture(gl.TEXTURE_2D, texID)

	gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA,
		int32(img.Bounds().Dx()), int32(img.Bounds().Dy()),
		0, gl.RGBA, gl.UNSIGNED_BYTE, unsafe.Pointer(&img.Pix[0]))

	gl.GenerateMipmap(gl.TEXTURE_2D)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR_MIPMAP_LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.REPEAT)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.REPEAT)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAX_LEVEL, 4)
	gl.TexParameterf(gl.TEXTURE_2D, gl.TEXTURE_MAX_ANISOTROPY, 8.0)

	return texID
}

func (tr *TerrainRenderer) uploadLightmapAtlas() {
	if tr.lightmapAtlas == nil || len(tr.lightmapAtlas.Data) == 0 {
		return
	}

	gl.GenTextures(1, &tr.lightmapAtlasTex)
	gl.BindTexture(gl.TEXTURE_2D, tr.lightmapAtlasTex)

	// LightmapAtlas.Size is the square atlas size
	gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA,
		tr.lightmapAtlas.Size, tr.lightmapAtlas.Size,
		0, gl.RGBA, gl.UNSIGNED_BYTE, unsafe.Pointer(&tr.lightmapAtlas.Data[0]))

	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
}

func (tr *TerrainRenderer) uploadTerrainMesh(vertices []terrain.Vertex, indices []uint32) {
	gl.GenVertexArrays(1, &tr.vao)
	gl.BindVertexArray(tr.vao)

	// VBO
	gl.GenBuffers(1, &tr.vbo)
	gl.BindBuffer(gl.ARRAY_BUFFER, tr.vbo)
	vertexSize := int(unsafe.Sizeof(terrain.Vertex{}))
	gl.BufferData(gl.ARRAY_BUFFER, len(vertices)*vertexSize, unsafe.Pointer(&vertices[0]), gl.STATIC_DRAW)

	// Position (location 0)
	gl.VertexAttribPointerWithOffset(0, 3, gl.FLOAT, false, int32(vertexSize), 0)
	gl.EnableVertexAttribArray(0)

	// Normal (location 1)
	gl.VertexAttribPointerWithOffset(1, 3, gl.FLOAT, false, int32(vertexSize), 3*4)
	gl.EnableVertexAttribArray(1)

	// TexCoord (location 2)
	gl.VertexAttribPointerWithOffset(2, 2, gl.FLOAT, false, int32(vertexSize), 6*4)
	gl.EnableVertexAttribArray(2)

	// LightmapUV (location 3)
	gl.VertexAttribPointerWithOffset(3, 2, gl.FLOAT, false, int32(vertexSize), 8*4)
	gl.EnableVertexAttribArray(3)

	// Color (location 4)
	gl.VertexAttribPointerWithOffset(4, 4, gl.FLOAT, false, int32(vertexSize), 10*4)
	gl.EnableVertexAttribArray(4)

	// EBO
	gl.GenBuffers(1, &tr.ebo)
	gl.BindBuffer(gl.ELEMENT_ARRAY_BUFFER, tr.ebo)
	gl.BufferData(gl.ELEMENT_ARRAY_BUFFER, len(indices)*4, unsafe.Pointer(&indices[0]), gl.STATIC_DRAW)

	gl.BindVertexArray(0)
}

// Render renders the terrain.
func (tr *TerrainRenderer) Render(viewProj math.Mat4, lightDir, ambient, diffuse [3]float32, brightness, lightOpacity float32,
	shadowsEnabled bool, lightViewProj math.Mat4, shadowMap *shadow.Map,
	pointLightsEnabled bool, pointLights []PointLight, pointLightIntensity float32,
	fogEnabled bool, fogNear, fogFar float32, fogColor [3]float32) {

	if tr.vao == 0 {
		return
	}

	gl.UseProgram(tr.program)

	// Set uniforms
	gl.UniformMatrix4fv(tr.locViewProj, 1, false, &viewProj[0])
	gl.Uniform3f(tr.locLightDir, lightDir[0], lightDir[1], lightDir[2])
	gl.Uniform3f(tr.locAmbient, ambient[0], ambient[1], ambient[2])
	gl.Uniform3f(tr.locDiffuse, diffuse[0], diffuse[1], diffuse[2])
	gl.Uniform1f(tr.locBrightness, brightness)
	gl.Uniform1f(tr.locLightOpacity, lightOpacity)

	// Fog uniforms
	if fogEnabled {
		gl.Uniform1i(tr.locFogUse, 1)
		gl.Uniform1f(tr.locFogNear, fogNear)
		gl.Uniform1f(tr.locFogFar, fogFar)
		gl.Uniform3f(tr.locFogColor, fogColor[0], fogColor[1], fogColor[2])
	} else {
		gl.Uniform1i(tr.locFogUse, 0)
	}

	// Shadow uniforms
	if shadowsEnabled && shadowMap != nil {
		gl.Uniform1i(tr.locShadowsEnabled, 1)
		gl.UniformMatrix4fv(tr.locLightViewProj, 1, false, &lightViewProj[0])
		gl.ActiveTexture(gl.TEXTURE2)
		gl.BindTexture(gl.TEXTURE_2D, shadowMap.DepthTexture)
		gl.Uniform1i(tr.locShadowMap, 2)
	} else {
		gl.Uniform1i(tr.locShadowsEnabled, 0)
	}

	// Point light uniforms
	if pointLightsEnabled && len(pointLights) > 0 {
		gl.Uniform1i(tr.locPointLightsEnabled, 1)
		count := len(pointLights)
		if count > MaxPointLights {
			count = MaxPointLights
		}
		gl.Uniform1i(tr.locPointLightCount, int32(count))

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

		gl.Uniform3fv(tr.locPointLightPositions, int32(count), &positions[0])
		gl.Uniform3fv(tr.locPointLightColors, int32(count), &colors[0])
		gl.Uniform1fv(tr.locPointLightRanges, int32(count), &ranges[0])
		gl.Uniform1fv(tr.locPointLightIntensities, int32(count), &intensities[0])
	} else {
		gl.Uniform1i(tr.locPointLightsEnabled, 0)
	}

	// Bind lightmap
	gl.ActiveTexture(gl.TEXTURE1)
	gl.BindTexture(gl.TEXTURE_2D, tr.lightmapAtlasTex)
	gl.Uniform1i(tr.locLightmap, 1)

	// Draw each texture group
	gl.BindVertexArray(tr.vao)
	gl.ActiveTexture(gl.TEXTURE0)
	gl.Uniform1i(tr.locTexture, 0)

	for _, group := range tr.groups {
		tex, ok := tr.groundTextures[group.TextureID]
		if !ok {
			continue
		}
		gl.BindTexture(gl.TEXTURE_2D, tex)
		gl.DrawElementsWithOffset(gl.TRIANGLES, group.IndexCount, gl.UNSIGNED_INT, uintptr(group.StartIndex*4))
	}

	gl.BindVertexArray(0)
}

// RenderShadow renders the terrain to the shadow map.
func (tr *TerrainRenderer) RenderShadow() {
	if tr.vao == 0 {
		return
	}

	gl.BindVertexArray(tr.vao)
	var totalIndices int32
	for _, group := range tr.groups {
		totalIndices += group.IndexCount
	}
	gl.DrawElements(gl.TRIANGLES, totalIndices, gl.UNSIGNED_INT, nil)
	gl.BindVertexArray(0)
}

func (tr *TerrainRenderer) clearTerrain() {
	if tr.vao != 0 {
		gl.DeleteVertexArrays(1, &tr.vao)
		tr.vao = 0
	}
	if tr.vbo != 0 {
		gl.DeleteBuffers(1, &tr.vbo)
		tr.vbo = 0
	}
	if tr.ebo != 0 {
		gl.DeleteBuffers(1, &tr.ebo)
		tr.ebo = 0
	}
	for _, tex := range tr.groundTextures {
		if tex != 0 {
			gl.DeleteTextures(1, &tex)
		}
	}
	tr.groundTextures = make(map[int]uint32)
	if tr.lightmapAtlasTex != 0 {
		gl.DeleteTextures(1, &tr.lightmapAtlasTex)
		tr.lightmapAtlasTex = 0
	}
}

// Destroy releases all resources.
func (tr *TerrainRenderer) Destroy() {
	tr.clearTerrain()
	if tr.program != 0 {
		gl.DeleteProgram(tr.program)
		tr.program = 0
	}
}
