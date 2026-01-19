package terrain

import (
	"github.com/Faultbox/midgard-ro/pkg/formats"
)

// BuildHeightmap creates a heightmap from GND data for model positioning.
func BuildHeightmap(gnd *formats.GND) *Heightmap {
	tilesX := int(gnd.Width)
	tilesZ := int(gnd.Height)

	altitudes := make([][]float32, tilesX)
	for x := range tilesX {
		altitudes[x] = make([]float32, tilesZ)
		for z := range tilesZ {
			tile := gnd.GetTile(x, z)
			if tile != nil {
				// Average of 4 corners
				// Negate because GND altitudes are negative (lower = higher in RO coordinate system)
				avgAlt := (tile.Altitude[0] + tile.Altitude[1] + tile.Altitude[2] + tile.Altitude[3]) / 4.0
				altitudes[x][z] = -avgAlt
			}
		}
	}

	return &Heightmap{
		Altitudes: altitudes,
		TilesX:    tilesX,
		TilesZ:    tilesZ,
		TileZoom:  gnd.Zoom,
	}
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
