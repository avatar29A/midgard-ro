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
			// RO coordinate system: X=east, Y=up (negative=higher), Z=north
			// Korangar-style: South corners at lower Z, North corners at higher Z
			baseX := float32(x) * tileSize
			baseZ := float32(y) * tileSize

			// Corner positions (in RO, altitude is negated for world Y)
			// GND corners: [0]=SW, [1]=SE, [2]=NW, [3]=NE
			// SW/SE at baseZ (lower Z = South), NW/NE at baseZ+tileSize (higher Z = North)
			corners := [4][3]float32{
				{baseX, -tile.Altitude[0], baseZ},                       // SW (South-West)
				{baseX + tileSize, -tile.Altitude[1], baseZ},            // SE (South-East)
				{baseX, -tile.Altitude[2], baseZ + tileSize},            // NW (North-West)
				{baseX + tileSize, -tile.Altitude[3], baseZ + tileSize}, // NE (North-East)
			}

			// Update bounds
			for _, c := range corners {
				updateBounds(&bounds, c)
			}

			// Top surface (horizontal quad)
			// Korangar-style: 6 vertices (3 per triangle) with per-triangle normals
			// Triangles: SW-SE-NW and NW-SE-NE (diagonal from SE to NW)
			if tile.TopSurface >= 0 && int(tile.TopSurface) < len(gnd.Surfaces) {
				surface := &gnd.Surfaces[tile.TopSurface]
				texID := int(surface.TextureID)

				// Calculate per-triangle normals (Korangar style)
				// Triangle 1: SW, SE, NW
				normal1 := calcTriangleNormal(corners[0], corners[1], corners[2])
				// Triangle 2: NW, SE, NE
				normal2 := calcTriangleNormal(corners[2], corners[1], corners[3])

				// Vertex colors from surface and neighbors (Korangar style)
				// East = x+1, North = y+1
				colorSW := surfaceColor(surface)
				colorSE := getNeighborColor(gnd, x+1, y, surface)   // East neighbor
				colorNW := getNeighborColor(gnd, x, y+1, surface)   // North neighbor
				colorNE := getNeighborColor(gnd, x+1, y+1, surface) // NorthEast neighbor

				// Calculate lightmap UVs
				lmUV0 := CalculateLightmapUV(atlas, surface.LightmapID, 0)
				lmUV1 := CalculateLightmapUV(atlas, surface.LightmapID, 1)
				lmUV2 := CalculateLightmapUV(atlas, surface.LightmapID, 2)
				lmUV3 := CalculateLightmapUV(atlas, surface.LightmapID, 3)

				// UV coordinates - direct mapping
				u0, v0 := uvInset(surface.U[0], surface.V[0]) // SW
				u1, v1 := uvInset(surface.U[1], surface.V[1]) // SE
				u2, v2 := uvInset(surface.U[2], surface.V[2]) // NW
				u3, v3 := uvInset(surface.U[3], surface.V[3]) // NE

				// Create 6 vertices with per-triangle normals (Korangar style)
				// Triangle 1: SW, SE, NW (all with normal1)
				baseIdx := uint32(len(vertices))
				vertices = append(vertices,
					Vertex{Position: corners[0], Normal: normal1, TexCoord: [2]float32{u0, v0}, LightmapUV: lmUV0, Color: colorSW}, // SW
					Vertex{Position: corners[1], Normal: normal1, TexCoord: [2]float32{u1, v1}, LightmapUV: lmUV1, Color: colorSE}, // SE
					Vertex{Position: corners[2], Normal: normal1, TexCoord: [2]float32{u2, v2}, LightmapUV: lmUV2, Color: colorNW}, // NW
				)
				textureIndices[texID] = append(textureIndices[texID],
					baseIdx, baseIdx+1, baseIdx+2,
				)

				// Triangle 2: NW, SE, NE (all with normal2)
				baseIdx = uint32(len(vertices))
				vertices = append(vertices,
					Vertex{Position: corners[2], Normal: normal2, TexCoord: [2]float32{u2, v2}, LightmapUV: lmUV2, Color: colorNW}, // NW
					Vertex{Position: corners[1], Normal: normal2, TexCoord: [2]float32{u1, v1}, LightmapUV: lmUV1, Color: colorSE}, // SE
					Vertex{Position: corners[3], Normal: normal2, TexCoord: [2]float32{u3, v3}, LightmapUV: lmUV3, Color: colorNE}, // NE
				)
				textureIndices[texID] = append(textureIndices[texID],
					baseIdx, baseIdx+1, baseIdx+2,
				)
			}

			// North surface (vertical wall at higher Z, connecting to y+1 neighbor)
			// Korangar: North wall connects current tile's NW/NE to neighbor's SW/SE
			northNeighbor := gnd.GetTile(x, y+1)
			if northNeighbor != nil {
				// Compare current tile's NW/NE with neighbor's SW/SE
				heightDiff0 := absf(tile.Altitude[2] - northNeighbor.Altitude[0])
				heightDiff1 := absf(tile.Altitude[3] - northNeighbor.Altitude[1])
				if heightDiff0 > 0.001 || heightDiff1 > 0.001 {
					buildWallNorth(gnd, atlas, tile, northNeighbor, corners, baseX, baseZ, tileSize,
						&vertices, textureIndices)
				}
			}

			// East surface (vertical wall at higher X, connecting to x+1 neighbor)
			// Korangar: East wall connects current tile's SE/NE to neighbor's SW/NW
			eastNeighbor := gnd.GetTile(x+1, y)
			if eastNeighbor != nil {
				// Compare current tile's SE/NE with neighbor's SW/NW
				heightDiff0 := absf(tile.Altitude[1] - eastNeighbor.Altitude[0])
				heightDiff1 := absf(tile.Altitude[3] - eastNeighbor.Altitude[2])
				if heightDiff0 > 0.001 || heightDiff1 > 0.001 {
					buildWallEast(gnd, atlas, tile, eastNeighbor, corners, baseX, baseZ, tileSize,
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

// buildWallNorth builds a double-sided wall on the North edge (higher Z).
// Connects current tile's NW/NE to neighbor's SW/SE (Korangar style).
func buildWallNorth(gnd *formats.GND, atlas *LightmapAtlas, tile, neighborTile *formats.GNDTile,
	corners [4][3]float32, baseX, baseZ, tileSize float32,
	vertices *[]Vertex, textureIndices map[int][]uint32) {

	// Wall at Z = baseZ + tileSize (north edge)
	// Current tile's NW (corners[2]) and NE (corners[3]) are at the top of the wall
	// Neighbor tile's SW and SE altitudes form the bottom of the wall (at same X,Z)
	wallCorners := [4][3]float32{
		corners[2], // Current tile NW (top-left of wall)
		corners[3], // Current tile NE (top-right of wall)
		{baseX, -neighborTile.Altitude[0], baseZ + tileSize},            // Neighbor SW (bottom-left)
		{baseX + tileSize, -neighborTile.Altitude[1], baseZ + tileSize}, // Neighbor SE (bottom-right)
	}

	normalNorth := [3]float32{0, 0, 1}  // Facing +Z (north, away from current tile)
	normalSouth := [3]float32{0, 0, -1} // Facing -Z (south, toward current tile)
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

	// North-facing vertices (facing +Z)
	*vertices = append(*vertices,
		Vertex{Position: wallCorners[0], Normal: normalNorth, TexCoord: [2]float32{texU[0], texV[0]}, LightmapUV: wlmUV0, Color: color},
		Vertex{Position: wallCorners[1], Normal: normalNorth, TexCoord: [2]float32{texU[1], texV[1]}, LightmapUV: wlmUV1, Color: color},
		Vertex{Position: wallCorners[2], Normal: normalNorth, TexCoord: [2]float32{texU[2], texV[2]}, LightmapUV: wlmUV2, Color: color},
		Vertex{Position: wallCorners[3], Normal: normalNorth, TexCoord: [2]float32{texU[3], texV[3]}, LightmapUV: wlmUV3, Color: color},
	)

	// South-facing vertices (facing -Z)
	*vertices = append(*vertices,
		Vertex{Position: wallCorners[0], Normal: normalSouth, TexCoord: [2]float32{texU[0], texV[0]}, LightmapUV: wlmUV0, Color: color},
		Vertex{Position: wallCorners[1], Normal: normalSouth, TexCoord: [2]float32{texU[1], texV[1]}, LightmapUV: wlmUV1, Color: color},
		Vertex{Position: wallCorners[2], Normal: normalSouth, TexCoord: [2]float32{texU[2], texV[2]}, LightmapUV: wlmUV2, Color: color},
		Vertex{Position: wallCorners[3], Normal: normalSouth, TexCoord: [2]float32{texU[3], texV[3]}, LightmapUV: wlmUV3, Color: color},
	)

	// North face triangles (CCW winding when viewed from +Z)
	textureIndices[texID] = append(textureIndices[texID],
		baseIdx, baseIdx+1, baseIdx+2,
		baseIdx+1, baseIdx+3, baseIdx+2,
	)

	// South face triangles (CCW winding when viewed from -Z)
	textureIndices[texID] = append(textureIndices[texID],
		baseIdx+4, baseIdx+6, baseIdx+5,
		baseIdx+5, baseIdx+6, baseIdx+7,
	)
}

// buildWallEast builds a double-sided wall on the East edge (higher X).
// Connects current tile's SE/NE to neighbor's SW/NW (Korangar style).
func buildWallEast(gnd *formats.GND, atlas *LightmapAtlas, tile, neighborTile *formats.GNDTile,
	corners [4][3]float32, baseX, baseZ, tileSize float32,
	vertices *[]Vertex, textureIndices map[int][]uint32) {

	// Wall at X = baseX + tileSize (east edge)
	// Current tile's NE (corners[3]) and SE (corners[1]) are at the top of the wall
	// Neighbor tile's NW and SW altitudes form the bottom of the wall (at same X,Z)
	// Korangar order: NE, SE for current tile; NW, SW for neighbor (maintains correct orientation)
	wallCorners := [4][3]float32{
		corners[3], // Current tile NE (top-back, higher Z)
		corners[1], // Current tile SE (top-front, lower Z)
		{baseX + tileSize, -neighborTile.Altitude[2], baseZ + tileSize}, // Neighbor NW (bottom-back)
		{baseX + tileSize, -neighborTile.Altitude[0], baseZ},            // Neighbor SW (bottom-front)
	}

	normalEast := [3]float32{1, 0, 0}  // Facing +X (east, away from current tile)
	normalWest := [3]float32{-1, 0, 0} // Facing -X (west, toward current tile)
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

	// East-facing vertices (facing +X)
	*vertices = append(*vertices,
		Vertex{Position: wallCorners[0], Normal: normalEast, TexCoord: [2]float32{texU[0], texV[0]}, LightmapUV: wlmUV0, Color: color},
		Vertex{Position: wallCorners[1], Normal: normalEast, TexCoord: [2]float32{texU[1], texV[1]}, LightmapUV: wlmUV1, Color: color},
		Vertex{Position: wallCorners[2], Normal: normalEast, TexCoord: [2]float32{texU[2], texV[2]}, LightmapUV: wlmUV2, Color: color},
		Vertex{Position: wallCorners[3], Normal: normalEast, TexCoord: [2]float32{texU[3], texV[3]}, LightmapUV: wlmUV3, Color: color},
	)

	// West-facing vertices (facing -X)
	*vertices = append(*vertices,
		Vertex{Position: wallCorners[0], Normal: normalWest, TexCoord: [2]float32{texU[0], texV[0]}, LightmapUV: wlmUV0, Color: color},
		Vertex{Position: wallCorners[1], Normal: normalWest, TexCoord: [2]float32{texU[1], texV[1]}, LightmapUV: wlmUV1, Color: color},
		Vertex{Position: wallCorners[2], Normal: normalWest, TexCoord: [2]float32{texU[2], texV[2]}, LightmapUV: wlmUV2, Color: color},
		Vertex{Position: wallCorners[3], Normal: normalWest, TexCoord: [2]float32{texU[3], texV[3]}, LightmapUV: wlmUV3, Color: color},
	)

	// East face triangles (CCW winding when viewed from +X)
	textureIndices[texID] = append(textureIndices[texID],
		baseIdx, baseIdx+1, baseIdx+2,
		baseIdx+1, baseIdx+3, baseIdx+2,
	)

	// West face triangles (CCW winding when viewed from -X)
	textureIndices[texID] = append(textureIndices[texID],
		baseIdx+4, baseIdx+6, baseIdx+5,
		baseIdx+5, baseIdx+6, baseIdx+7,
	)
}

// SmoothNormals averages normals at shared vertex positions (Korangar-style).
// This eliminates hard edges between tiles for a smoother appearance.
// Excludes "artificial vertices" (wall edges) from smoothing.
func SmoothNormals(vertices []Vertex) {
	const epsilon float32 = 1e-6

	// Detect artificial vertices (Korangar style):
	// Vertices that connect to an edge parallel to Y axis (walls).
	// Such edges only occur in wall structures.
	artificialVertices := make([]bool, len(vertices))

	// Process in chunks of 3 (triangles)
	for chunkIdx := 0; chunkIdx+2 < len(vertices); chunkIdx += 3 {
		for vertIdx := 0; vertIdx < 3; vertIdx++ {
			idx0 := chunkIdx + vertIdx
			idx1 := chunkIdx + (vertIdx+1)%3

			pos0 := vertices[idx0].Position
			pos1 := vertices[idx1].Position

			// If edge is vertical (X and Z are same), it's a wall edge
			if absf(pos0[0]-pos1[0]) < epsilon && absf(pos0[2]-pos1[2]) < epsilon {
				artificialVertices[idx0] = true
				artificialVertices[idx1] = true
			}
		}
	}

	// Use full 3D position for grouping with high precision (Korangar style)
	posKey := func(p [3]float32) [3]int64 {
		return [3]int64{
			int64((p[0] / epsilon) + 0.5),
			int64((p[1] / epsilon) + 0.5),
			int64((p[2] / epsilon) + 0.5),
		}
	}

	// First pass: sum normals at each position (excluding artificial vertices)
	normalSums := make(map[[3]int64][3]float32)

	for i := range vertices {
		if artificialVertices[i] {
			continue // Skip wall vertices
		}
		key := posKey(vertices[i].Position)
		sum := normalSums[key]
		sum[0] += vertices[i].Normal[0]
		sum[1] += vertices[i].Normal[1]
		sum[2] += vertices[i].Normal[2]
		normalSums[key] = sum
	}

	// Normalize the sums
	for key, sum := range normalSums {
		normalSums[key] = normalize(sum)
	}

	// Second pass: apply smoothed normals to all non-artificial vertices
	for i := range vertices {
		key := posKey(vertices[i].Position)
		if smoothNormal, ok := normalSums[key]; ok {
			vertices[i].Normal = smoothNormal
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

// calcTriangleNormal calculates the face normal for a triangle.
// Uses edge2 x edge1 to get upward-pointing normal for CCW winding.
func calcTriangleNormal(p0, p1, p2 [3]float32) [3]float32 {
	edge1 := [3]float32{
		p1[0] - p0[0],
		p1[1] - p0[1],
		p1[2] - p0[2],
	}
	edge2 := [3]float32{
		p2[0] - p0[0],
		p2[1] - p0[1],
		p2[2] - p0[2],
	}
	// Swap order: cross(edge2, edge1) gives upward-pointing normal for terrain
	return normalize(cross(edge2, edge1))
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
	for range 10 {
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

// uvInset applies half-pixel inset to prevent texture bleeding (Korangar style).
// Formula: half_pixel + uv * (1.0 - 2.0 * half_pixel)
// Assumes 256x256 textures.
func uvInset(u, v float32) (float32, float32) {
	const halfPixel = 0.5 / 256.0 // Korangar uses 0.5 / texture_dimension
	const scale = 1.0 - 2.0*halfPixel
	return halfPixel + u*scale, halfPixel + v*scale
}

// surfaceColor extracts RGBA color from a GND surface.
func surfaceColor(surface *formats.GNDSurface) [4]float32 {
	return [4]float32{
		float32(surface.Color[2]) / 255.0, // R (stored as BGR)
		float32(surface.Color[1]) / 255.0, // G
		float32(surface.Color[0]) / 255.0, // B
		float32(surface.Color[3]) / 255.0, // A
	}
}

// getNeighborColor gets the surface color from a neighbor tile (Korangar style).
// Falls back to the current surface color if neighbor doesn't exist.
func getNeighborColor(gnd *formats.GND, nx, ny int, fallback *formats.GNDSurface) [4]float32 {
	neighborTile := gnd.GetTile(nx, ny)
	if neighborTile == nil || neighborTile.TopSurface < 0 {
		return surfaceColor(fallback)
	}
	if int(neighborTile.TopSurface) >= len(gnd.Surfaces) {
		return surfaceColor(fallback)
	}
	return surfaceColor(&gnd.Surfaces[neighborTile.TopSurface])
}

// BuildTileGrid creates a tile grid mesh from GAT and GND data for debug visualization.
// The grid shows walkability with color-coded tiles (Korangar-style debug feature).
// Uses GND heights for accurate terrain alignment, GAT for walkability colors.
// tileOffset is a small Y offset to render the grid slightly above the terrain.
func BuildTileGrid(gat *formats.GAT, gnd *formats.GND, tileOffset float32) *TileGrid {
	if gat == nil || gnd == nil {
		return nil
	}

	// Use GND dimensions and tile size
	width := int(gnd.Width)
	height := int(gnd.Height)
	tileSize := gnd.Zoom

	// Pre-allocate
	vertices := make([]TileGridVertex, 0, width*height*4)
	indices := make([]uint32, 0, width*height*6)

	for y := range height {
		for x := range width {
			// Get GND tile for height data
			tile := gnd.GetTile(x, y)
			if tile == nil {
				continue
			}

			// Get GAT cell for walkability color
			// GAT may have same or different dimensions - map accordingly
			gatX := x * int(gat.Width) / width
			gatY := y * int(gat.Height) / height
			cell := gat.GetCell(gatX, gatY)

			// Determine color based on cell type
			var color [4]float32
			if cell != nil {
				color = tileColor(cell.Type)
			} else {
				color = [4]float32{0.5, 0.5, 0.5, 0.5} // Gray for missing
			}

			// Calculate corner positions using GND coordinates and heights
			// Korangar-style: South corners at lower Z, North corners at higher Z
			baseX := float32(x) * tileSize
			baseZ := float32(y) * tileSize

			// Use GND heights (negated) with offset above terrain
			// SW/SE at baseZ (lower Z = South), NW/NE at baseZ+tileSize (higher Z = North)
			corners := [4][3]float32{
				{baseX, -tile.Altitude[0] + tileOffset, baseZ},                       // SW (South-West)
				{baseX + tileSize, -tile.Altitude[1] + tileOffset, baseZ},            // SE (South-East)
				{baseX, -tile.Altitude[2] + tileOffset, baseZ + tileSize},            // NW (North-West)
				{baseX + tileSize, -tile.Altitude[3] + tileOffset, baseZ + tileSize}, // NE (North-East)
			}

			baseIdx := uint32(len(vertices))

			// 4 vertices per tile
			vertices = append(vertices,
				TileGridVertex{Position: corners[0], Color: color},
				TileGridVertex{Position: corners[1], Color: color},
				TileGridVertex{Position: corners[2], Color: color},
				TileGridVertex{Position: corners[3], Color: color},
			)

			// 2 triangles (6 indices): SW-SE-NW, NW-SE-NE
			indices = append(indices,
				baseIdx, baseIdx+1, baseIdx+2,
				baseIdx+2, baseIdx+1, baseIdx+3,
			)
		}
	}

	return &TileGrid{
		Vertices: vertices,
		Indices:  indices,
	}
}

// tileColor returns a color based on GAT cell type (Korangar-style debug colors).
func tileColor(cellType formats.GATCellType) [4]float32 {
	// More visible colors for debugging
	const alpha = 0.7

	switch cellType {
	case formats.GATWalkable:
		// Bright green for walkable
		return [4]float32{0.0, 1.0, 0.0, alpha}
	case formats.GATBlocked:
		// Bright red for blocked
		return [4]float32{1.0, 0.0, 0.0, alpha}
	case formats.GATWater:
		// Bright blue for water
		return [4]float32{0.0, 0.5, 1.0, alpha}
	case formats.GATWalkableWater:
		// Cyan for walkable water (shallow)
		return [4]float32{0.0, 1.0, 1.0, alpha}
	case formats.GATSnipeable:
		// Yellow for snipeable (can shoot over)
		return [4]float32{1.0, 1.0, 0.0, alpha}
	case formats.GATBlockedSnipe:
		// Orange for blocked but snipeable
		return [4]float32{1.0, 0.5, 0.0, alpha}
	default:
		// Magenta for unknown (very visible)
		return [4]float32{1.0, 0.0, 1.0, alpha}
	}
}
