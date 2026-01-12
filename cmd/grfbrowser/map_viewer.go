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
	locTexture      int32
	locLightmap     int32
	locSunStrength  int32

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

	// Camera (orbit style for Stage 1)
	rotationX float32
	rotationY float32
	distance  float32
	centerX   float32
	centerY   float32
	centerZ   float32

	// Lighting controls
	SunStrength float32

	// Map bounds
	minBounds [3]float32
	maxBounds [3]float32
}

// terrainVertex is the vertex format for terrain mesh.
type terrainVertex struct {
	Position    [3]float32
	Normal      [3]float32
	TexCoord    [2]float32
	LightmapUV  [2]float32
	Color       [4]float32
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
uniform float uSunStrength;

out vec4 FragColor;

void main() {
    vec4 texColor = texture(uTexture, vTexCoord);
    vec3 lightmapColor = texture(uLightmap, vLightmapUV).rgb;

    // Directional light component (sun) - controlled by uSunStrength
    float NdotL = max(dot(normalize(vNormal), normalize(uLightDir)), 0.0);
    vec3 directional = vec3(uSunStrength) * NdotL;

    // Combine: ambient + directional, modulated by lightmap and vertex color
    // Boost overall brightness for preview visibility
    vec3 lighting = (uAmbient + directional + vec3(0.2)) * (lightmapColor * 1.2 + vec3(0.5)) * vColor.rgb;

    vec3 color = texColor.rgb * lighting;
    FragColor = vec4(color, texColor.a * vColor.a);
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
		SunStrength:    2.0,
	}

	if err := mv.createFramebuffer(); err != nil {
		return nil, fmt.Errorf("creating framebuffer: %w", err)
	}

	if err := mv.createTerrainShader(); err != nil {
		return nil, fmt.Errorf("creating terrain shader: %w", err)
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
	mv.locTexture = gl.GetUniformLocation(mv.terrainProgram, gl.Str("uTexture\x00"))
	mv.locLightmap = gl.GetUniformLocation(mv.terrainProgram, gl.Str("uLightmap\x00"))
	mv.locSunStrength = gl.GetUniformLocation(mv.terrainProgram, gl.Str("uSunStrength\x00"))

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
func (mv *MapViewer) LoadMap(gnd *formats.GND, _ *formats.RSW, texLoader func(string) ([]byte, error)) error {
	// Clear old resources
	mv.clearTerrain()

	// Load ground textures
	mv.loadGroundTextures(gnd, texLoader)

	// Build lightmap atlas (Stage 2)
	mv.buildLightmapAtlas(gnd)

	// Build terrain mesh
	vertices, indices, groups := mv.buildTerrainMesh(gnd)
	mv.terrainGroups = groups

	// Upload to GPU
	mv.uploadTerrainMesh(vertices, indices)

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

	// Orbit camera position
	camPos := mv.calculateCameraPosition()
	center := math.Vec3{X: mv.centerX, Y: mv.centerY, Z: mv.centerZ}
	up := math.Vec3{X: 0, Y: 1, Z: 0}
	view := math.LookAt(camPos, center, up)

	viewProj := proj.Mul(view)

	// Use terrain shader
	gl.UseProgram(mv.terrainProgram)
	gl.UniformMatrix4fv(mv.locViewProj, 1, false, &viewProj[0])
	gl.Uniform3f(mv.locLightDir, 0.5, 1.0, 0.3)
	gl.Uniform3f(mv.locAmbient, 0.4, 0.4, 0.4)
	gl.Uniform1f(mv.locSunStrength, mv.SunStrength)
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
	gl.BindFramebuffer(gl.FRAMEBUFFER, 0)

	return mv.colorTexture
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

// HandleMouseWheel handles mouse scroll for zoom.
func (mv *MapViewer) HandleMouseWheel(delta float32) {
	mv.distance -= delta * mv.distance * 0.1
	if mv.distance < 50 {
		mv.distance = 50
	}
	if mv.distance > 5000 {
		mv.distance = 5000
	}
}

// Reset resets camera to default position.
func (mv *MapViewer) Reset() {
	mv.rotationX = 0.6
	mv.rotationY = 0.0
	// Distance is kept from fitCamera
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

// Destroy frees all GPU resources.
func (mv *MapViewer) Destroy() {
	mv.clearTerrain()

	if mv.fallbackTex != 0 {
		gl.DeleteTextures(1, &mv.fallbackTex)
	}
	if mv.terrainProgram != 0 {
		gl.DeleteProgram(mv.terrainProgram)
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
