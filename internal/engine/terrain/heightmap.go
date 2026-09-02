package terrain

import (
	"math"

	"github.com/Faultbox/midgard-ro/pkg/formats"
)

// BuildHeightmap creates a heightmap from GND data, for standing things on
// the ground.
//
// Every tile's four corners are kept rather than averaged. What the query has
// to agree with is the mesh, and the mesh puts each corner at its own height.
func BuildHeightmap(gnd *formats.GND) *Heightmap {
	tilesX := int(gnd.Width)
	tilesZ := int(gnd.Height)

	corners := make([][][4]float32, tilesX)
	for x := range tilesX {
		corners[x] = make([][4]float32, tilesZ)
		for z := range tilesZ {
			if tile := gnd.GetTile(x, z); tile != nil {
				corners[x][z] = [4]float32{
					tile.Altitude[0], tile.Altitude[1],
					tile.Altitude[2], tile.Altitude[3],
				}
			}
		}
	}

	return &Heightmap{
		Corners:  corners,
		TilesX:   tilesX,
		TilesZ:   tilesZ,
		TileZoom: gnd.Zoom,
	}
}

// Probe reports what a height query is working from: which tile the position
// lands in, where inside it, and that tile's four corner heights as world Y.
//
// For telling a wrong height apart from a wrong sprite. The corners say
// whether the ground there is a slope at all, and the fractions say where on
// it the query is reading.
func (h *Heightmap) Probe(worldX, worldZ float32) (tileX, tileZ int, u, v float32, corners [4]float32) {
	if h == nil || h.TileZoom <= 0 || len(h.Corners) == 0 {
		return 0, 0, 0, 0, corners
	}

	tileFX := worldX / h.TileZoom
	tileFZ := worldZ / h.TileZoom

	tileX = int(math.Floor(float64(tileFX)))
	tileZ = int(math.Floor(float64(tileFZ)))

	if tileX < 0 || tileZ < 0 || tileX >= h.TilesX || tileZ >= h.TilesZ {
		return tileX, tileZ, 0, 0, corners
	}

	u = tileFX - float32(tileX)
	v = tileFZ - float32(tileZ)

	for i, alt := range h.Corners[tileX][tileZ] {
		corners[i] = -alt
	}

	return tileX, tileZ, u, v, corners
}

// HeightAt is the ground's world Y at a position, read off the same surface
// the mesh draws.
//
// Bilinear across the tile's four corners, which is what the mesh's own quad
// is. Sampling a tile as one height instead put a character on a staircase:
// it held one altitude the whole way across a tile and then jumped to the
// next, so on any slope it walked half a tile sunk into the ground and half a
// tile above it.
//
// Off the map is zero, which is what a flat map would give.
func (h *Heightmap) HeightAt(worldX, worldZ float32) float32 {
	if h == nil || h.TileZoom <= 0 || len(h.Corners) == 0 {
		return 0
	}

	tileFX := worldX / h.TileZoom
	tileFZ := worldZ / h.TileZoom

	tileX := int(math.Floor(float64(tileFX)))
	tileZ := int(math.Floor(float64(tileFZ)))

	if tileX < 0 || tileZ < 0 || tileX >= h.TilesX || tileZ >= h.TilesZ {
		return 0
	}

	u := tileFX - float32(tileX)
	v := tileFZ - float32(tileZ)

	c := h.Corners[tileX][tileZ]

	// Along the southern edge, then the northern one, then between them.
	south := c[0] + (c[1]-c[0])*u
	north := c[2] + (c[3]-c[2])*u

	// The mesh puts a corner's world Y at minus its stored altitude, so the
	// query has to negate it too or everything stands under the map.
	return -(south + (north-south)*v)
}

// GetInterpolatedHeight returns the interpolated terrain height at a world position.
// Uses GAT data for precise height lookup with bilinear interpolation.
func GetInterpolatedHeight(gat *formats.GAT, worldX, worldZ float32) float32 {
	if gat == nil {
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
	if cellX >= int(gat.Width)-1 {
		cellX = int(gat.Width) - 2
	}
	if cellZ >= int(gat.Height)-1 {
		cellZ = int(gat.Height) - 2
	}

	// Get fractional position within cell (0-1)
	fracX := cellFX - float32(cellX)
	fracZ := cellFZ - float32(cellZ)
	fracX = clampf(fracX, 0, 1)
	fracZ = clampf(fracZ, 0, 1)

	// Get cell heights (corners: 0=SW, 1=SE, 2=NW, 3=NE)
	cell := gat.GetCell(cellX, cellZ)
	if cell == nil {
		return 0
	}

	// Bilinear interpolation (Korangar style)
	// South edge (lower Z): lerp between SW and SE
	south := cell.Heights[0]*(1-fracX) + cell.Heights[1]*fracX
	// North edge (higher Z): lerp between NW and NE
	north := cell.Heights[2]*(1-fracX) + cell.Heights[3]*fracX
	// Final: lerp between south and north edges based on Z position
	height := south*(1-fracZ) + north*fracZ

	// GAT heights are typically negative (lower = higher in RO coordinate system)
	return -height
}

// IsWalkable checks if a world position is walkable according to GAT data.
func IsWalkable(gat *formats.GAT, worldX, worldZ float32) bool {
	if gat == nil {
		return true // No GAT data, allow movement
	}

	// Convert world coordinates to GAT cell coordinates
	cellSize := float32(5.0)
	cellX := int(worldX / cellSize)
	cellZ := int(worldZ / cellSize)

	// Check bounds
	if cellX < 0 || cellZ < 0 || cellX >= int(gat.Width) || cellZ >= int(gat.Height) {
		return false
	}

	cell := gat.GetCell(cellX, cellZ)
	if cell == nil {
		return false
	}

	// Cell type 0 = walkable, 1 = not walkable, 5 = water walkable
	return cell.Type == 0 || cell.Type == 5
}

func clampf(v, min, max float32) float32 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
