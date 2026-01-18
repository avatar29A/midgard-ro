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
			// Korangar style: 6 vertices per quad (2 triangles), each triangle gets its own normal
			if tile.TopSurface >= 0 && int(tile.TopSurface) < len(gnd.Surfaces) {
				surface := &gnd.Surfaces[tile.TopSurface]
				texID := int(surface.TextureID)

				// Calculate per-triangle normals (Korangar style)
				// Triangle 1: corners 0, 1, 2 (SW, SE, NW)
				normal1 := calcTriangleNormal(corners[0], corners[1], corners[2])
				// Triangle 2: corners 2, 1, 3 (NW, SE, NE)
				normal2 := calcTriangleNormal(corners[2], corners[1], corners[3])

				// Vertex colors from surface and neighbors (Korangar style)
				// This creates smooth color blending between tiles
				// Note: In our coords, y-1 = north (smaller Z), y+1 = south (larger Z)
				colorSW := surfaceColor(surface)                     // Current tile color for SW
				colorSE := getNeighborColor(gnd, x+1, y, surface)    // East neighbor for SE
				colorNW := getNeighborColor(gnd, x, y-1, surface)    // North neighbor for NW
				colorNE := getNeighborColor(gnd, x+1, y-1, surface)  // NE neighbor for NE

				// Calculate lightmap UVs
				lmUV0 := CalculateLightmapUV(atlas, surface.LightmapID, 0)
				lmUV1 := CalculateLightmapUV(atlas, surface.LightmapID, 1)
				lmUV2 := CalculateLightmapUV(atlas, surface.LightmapID, 2)
				lmUV3 := CalculateLightmapUV(atlas, surface.LightmapID, 3)

				// UV coordinates - swapped mapping (works for Prontera pattern)
				u0, v0 := uvInset(surface.U[2], surface.V[2])
				u1, v1 := uvInset(surface.U[3], surface.V[3])
				u2, v2 := uvInset(surface.U[0], surface.V[0])
				u3, v3 := uvInset(surface.U[1], surface.V[1])

				// Create 6 vertices (3 per triangle) with per-triangle normals
				// This matches Korangar's approach for proper smoothing
				baseIdx := uint32(len(vertices))

				// Triangle 1: SW(0), SE(1), NW(2)
				vertices = append(vertices,
					Vertex{Position: corners[0], Normal: normal1, TexCoord: [2]float32{u0, v0}, LightmapUV: lmUV0, Color: colorSW},
					Vertex{Position: corners[1], Normal: normal1, TexCoord: [2]float32{u1, v1}, LightmapUV: lmUV1, Color: colorSE},
					Vertex{Position: corners[2], Normal: normal1, TexCoord: [2]float32{u2, v2}, LightmapUV: lmUV2, Color: colorNW},
				)

				// Triangle 2: NW(2), SE(1), NE(3)
				vertices = append(vertices,
					Vertex{Position: corners[2], Normal: normal2, TexCoord: [2]float32{u2, v2}, LightmapUV: lmUV2, Color: colorNW},
					Vertex{Position: corners[1], Normal: normal2, TexCoord: [2]float32{u1, v1}, LightmapUV: lmUV1, Color: colorSE},
					Vertex{Position: corners[3], Normal: normal2, TexCoord: [2]float32{u3, v3}, LightmapUV: lmUV3, Color: colorNE},
				)

				// Indices for 2 triangles (now just sequential since vertices are already per-triangle)
				textureIndices[texID] = append(textureIndices[texID],
					baseIdx, baseIdx+1, baseIdx+2,
					baseIdx+3, baseIdx+4, baseIdx+5,
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

// SmoothNormals averages normals at shared vertex positions (Korangar-style).
// This eliminates hard edges between tiles for a smoother appearance.
// Wall vertices (those forming vertical edges) are excluded from smoothing.
func SmoothNormals(vertices []Vertex) {
	const epsilon float32 = 1e-6

	// Identify "artificial" vertices - those that form vertical edges (walls).
	// Korangar detects this by checking if an edge has same X and Z (only Y differs).
	artificialVertex := make([]bool, len(vertices))

	// Process vertices in chunks of 3 (triangles)
	for chunkIdx := 0; chunkIdx < len(vertices)/3; chunkIdx++ {
		baseIdx := chunkIdx * 3
		if baseIdx+2 >= len(vertices) {
			break
		}

		// Check each edge of the triangle
		for vertIdx := range 3 {
			idx0 := baseIdx + vertIdx
			idx1 := baseIdx + (vertIdx+1)%3

			p0 := vertices[idx0].Position
			p1 := vertices[idx1].Position

			// If X and Z are the same (edge is vertical/parallel to Y axis), mark as artificial
			if absf(p0[0]-p1[0]) < epsilon && absf(p0[2]-p1[2]) < epsilon {
				artificialVertex[idx0] = true
				artificialVertex[idx1] = true
			}
		}
	}

	// Sum normals at each position (excluding artificial vertices)
	const scale float32 = 1000000.0
	normalSums := make(map[[3]int32][3]float32)

	for i := range vertices {
		if artificialVertex[i] {
			continue
		}
		key := [3]int32{
			int32(vertices[i].Position[0] * scale),
			int32(vertices[i].Position[1] * scale),
			int32(vertices[i].Position[2] * scale),
		}
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

	// Apply smoothed normals to ALL vertices at matching positions
	// (Korangar applies to all, not just non-artificial)
	for i := range vertices {
		key := [3]int32{
			int32(vertices[i].Position[0] * scale),
			int32(vertices[i].Position[1] * scale),
			int32(vertices[i].Position[2] * scale),
		}
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

// calcTriangleNormal calculates the face normal for a triangle (Korangar style).
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
	return normalize(cross(edge1, edge2))
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

// BuildTileGrid creates a tile grid mesh from GAT data for debug visualization.
// The grid shows walkability with color-coded tiles (Korangar-style debug feature).
// Heights are negated to match the terrain coordinate system.
// tileOffset is a small Y offset to render the grid slightly above the terrain.
func BuildTileGrid(gat *formats.GAT, tileZoom float32, tileOffset float32) *TileGrid {
	if gat == nil {
		return nil
	}

	width := int(gat.Width)
	height := int(gat.Height)

	// GAT tiles are half the size of GND tiles (2 GAT cells per GND tile)
	// In RO, GAT_TILE_SIZE = GROUND_TILE_SIZE / 2
	gatTileSize := tileZoom / 2.0

	// Pre-allocate
	vertices := make([]TileGridVertex, 0, width*height*4)
	indices := make([]uint32, 0, width*height*6)

	for y := range height {
		for x := range width {
			cell := gat.GetCell(x, y)
			if cell == nil {
				continue
			}

			// Determine color based on cell type
			color := tileColor(cell.Type)

			// Calculate corner positions
			offsetX := float32(x) * gatTileSize
			offsetZ := float32(y) * gatTileSize

			// Heights are negated (like in Korangar) and offset slightly above terrain
			// Z coordinate: Z + tileSize = south (bottom), Z = north (top)
			// This matches the GND coordinate system
			corners := [4][3]float32{
				{offsetX, -cell.Heights[0] + tileOffset, offsetZ + gatTileSize},                             // SW (bottom-left)
				{offsetX + gatTileSize, -cell.Heights[1] + tileOffset, offsetZ + gatTileSize},               // SE (bottom-right)
				{offsetX, -cell.Heights[2] + tileOffset, offsetZ},                                           // NW (top-left)
				{offsetX + gatTileSize, -cell.Heights[3] + tileOffset, offsetZ},                             // NE (top-right)
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
