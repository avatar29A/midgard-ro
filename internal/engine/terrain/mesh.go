package terrain

import (
	"github.com/Faultbox/midgard-ro/pkg/formats"
)

// BuildMesh creates a terrain mesh from GND data.
// The atlas parameter provides lightmap UV calculation data.
func BuildMesh(gnd *formats.GND, atlas *LightmapAtlas) *Mesh {
	var vertices []Vertex
	var indices []uint32

	// Map from texture ID to indices
	textureIndices := make(map[int][]uint32)

	tileSize := gnd.Zoom
	width := int(gnd.Width)
	height := int(gnd.Height)

	// Initialize bounds
	bounds := Bounds{
		Min: [3]float32{1e10, 1e10, 1e10},
		Max: [3]float32{-1e10, -1e10, -1e10},
	}

	for y := range height {
		for x := range width {
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
				updateBounds(&bounds, c)
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
				lmUV0 := CalculateLightmapUV(atlas, surface.LightmapID, 0)
				lmUV1 := CalculateLightmapUV(atlas, surface.LightmapID, 1)
				lmUV2 := CalculateLightmapUV(atlas, surface.LightmapID, 2)
				lmUV3 := CalculateLightmapUV(atlas, surface.LightmapID, 3)

				// Create vertices for quad
				baseIdx := uint32(len(vertices))
				vertices = append(vertices,
					Vertex{Position: corners[0], Normal: normal, TexCoord: [2]float32{surface.U[2], surface.V[2]}, LightmapUV: lmUV0, Color: color},
					Vertex{Position: corners[1], Normal: normal, TexCoord: [2]float32{surface.U[3], surface.V[3]}, LightmapUV: lmUV1, Color: color},
					Vertex{Position: corners[2], Normal: normal, TexCoord: [2]float32{surface.U[0], surface.V[0]}, LightmapUV: lmUV2, Color: color},
					Vertex{Position: corners[3], Normal: normal, TexCoord: [2]float32{surface.U[1], surface.V[1]}, LightmapUV: lmUV3, Color: color},
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
					buildWallFront(gnd, atlas, tile, nextTile, corners, baseX, baseZ, tileSize,
						&vertices, textureIndices)
				}
			}

			// Right surface (vertical wall facing +X) - fill gaps between tiles
			rightNextTile := gnd.GetTile(x+1, y)
			if rightNextTile != nil {
				heightDiff0 := absf(tile.Altitude[1] - rightNextTile.Altitude[0])
				heightDiff1 := absf(tile.Altitude[3] - rightNextTile.Altitude[2])
				if heightDiff0 > 0.001 || heightDiff1 > 0.001 {
					buildWallRight(gnd, atlas, tile, rightNextTile, corners, baseX, baseZ, tileSize,
						&vertices, textureIndices)
				}
			}
		}
	}

	// Build texture groups and final index buffer
	var groups []TextureGroup
	for texID, texIndices := range textureIndices {
		if len(texIndices) == 0 {
			continue
		}
		groups = append(groups, TextureGroup{
			TextureID:  texID,
			StartIndex: int32(len(indices)),
			IndexCount: int32(len(texIndices)),
		})
		indices = append(indices, texIndices...)
	}

	// Smooth normals to eliminate hard edges between tiles
	SmoothNormals(vertices)

	return &Mesh{
		Vertices: vertices,
		Indices:  indices,
		Groups:   groups,
		Bounds:   bounds,
	}
}

// buildWallFront builds a front-facing wall between tiles.
func buildWallFront(gnd *formats.GND, atlas *LightmapAtlas, tile, nextTile *formats.GNDTile,
	corners [4][3]float32, baseX, baseZ, tileSize float32,
	vertices *[]Vertex, textureIndices map[int][]uint32) {

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
		surface := &gnd.Surfaces[tile.TopSurface]
		texID = int(surface.TextureID)
		texU = [4]float32{0, 1, 0, 1}
		texV = [4]float32{0, 0, 1, 1}
		lmID = surface.LightmapID
	} else {
		return
	}

	wlmUV0 := CalculateLightmapUV(atlas, lmID, 0)
	wlmUV1 := CalculateLightmapUV(atlas, lmID, 1)
	wlmUV2 := CalculateLightmapUV(atlas, lmID, 2)
	wlmUV3 := CalculateLightmapUV(atlas, lmID, 3)

	baseIdx := uint32(len(*vertices))
	*vertices = append(*vertices,
		Vertex{Position: wallCorners[0], Normal: normal, TexCoord: [2]float32{texU[0], texV[0]}, LightmapUV: wlmUV0, Color: color},
		Vertex{Position: wallCorners[1], Normal: normal, TexCoord: [2]float32{texU[1], texV[1]}, LightmapUV: wlmUV1, Color: color},
		Vertex{Position: wallCorners[2], Normal: normal, TexCoord: [2]float32{texU[2], texV[2]}, LightmapUV: wlmUV2, Color: color},
		Vertex{Position: wallCorners[3], Normal: normal, TexCoord: [2]float32{texU[3], texV[3]}, LightmapUV: wlmUV3, Color: color},
	)

	textureIndices[texID] = append(textureIndices[texID],
		baseIdx, baseIdx+2, baseIdx+1,
		baseIdx+1, baseIdx+2, baseIdx+3,
	)
}

// buildWallRight builds a right-facing wall between tiles.
func buildWallRight(gnd *formats.GND, atlas *LightmapAtlas, tile, rightNextTile *formats.GNDTile,
	corners [4][3]float32, baseX, baseZ, tileSize float32,
	vertices *[]Vertex, textureIndices map[int][]uint32) {

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
		surface := &gnd.Surfaces[tile.TopSurface]
		texID = int(surface.TextureID)
		texU = [4]float32{0, 1, 0, 1}
		texV = [4]float32{0, 0, 1, 1}
		lmID = surface.LightmapID
	} else {
		return
	}

	wlmUV0 := CalculateLightmapUV(atlas, lmID, 0)
	wlmUV1 := CalculateLightmapUV(atlas, lmID, 1)
	wlmUV2 := CalculateLightmapUV(atlas, lmID, 2)
	wlmUV3 := CalculateLightmapUV(atlas, lmID, 3)

	baseIdx := uint32(len(*vertices))
	*vertices = append(*vertices,
		Vertex{Position: wallCorners[0], Normal: normal, TexCoord: [2]float32{texU[0], texV[0]}, LightmapUV: wlmUV0, Color: color},
		Vertex{Position: wallCorners[1], Normal: normal, TexCoord: [2]float32{texU[1], texV[1]}, LightmapUV: wlmUV1, Color: color},
		Vertex{Position: wallCorners[2], Normal: normal, TexCoord: [2]float32{texU[2], texV[2]}, LightmapUV: wlmUV2, Color: color},
		Vertex{Position: wallCorners[3], Normal: normal, TexCoord: [2]float32{texU[3], texV[3]}, LightmapUV: wlmUV3, Color: color},
	)

	textureIndices[texID] = append(textureIndices[texID],
		baseIdx, baseIdx+2, baseIdx+1,
		baseIdx+1, baseIdx+2, baseIdx+3,
	)
}

// SmoothNormals averages normals at shared vertex positions.
// This eliminates hard edges between tiles for a smoother appearance.
func SmoothNormals(vertices []Vertex) {
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

// Helper functions

func updateBounds(b *Bounds, p [3]float32) {
	if p[0] < b.Min[0] {
		b.Min[0] = p[0]
	}
	if p[1] < b.Min[1] {
		b.Min[1] = p[1]
	}
	if p[2] < b.Min[2] {
		b.Min[2] = p[2]
	}
	if p[0] > b.Max[0] {
		b.Max[0] = p[0]
	}
	if p[1] > b.Max[1] {
		b.Max[1] = p[1]
	}
	if p[2] > b.Max[2] {
		b.Max[2] = p[2]
	}
}

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
	// Fast inverse square root approximation not needed - use standard
	return float32(sqrt64(float64(x)))
}

func sqrt64(x float64) float64 {
	// Use math.Sqrt via import
	if x <= 0 {
		return 0
	}
	z := x
	for i := 0; i < 10; i++ {
		z = (z + x/z) / 2
	}
	return z
}

func absf(x float32) float32 {
	if x < 0 {
		return -x
	}
	return x
}
