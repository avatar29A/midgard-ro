// Package debug provides debug visualization utilities.
package debug

import (
	"github.com/Faultbox/midgard-ro/pkg/formats"
)

// TileGridRenderer generates debug visualization for the tile grid.
type TileGridRenderer struct {
	gat      *formats.GAT
	tileSize float32
}

// NewTileGridRenderer creates a new tile grid renderer.
func NewTileGridRenderer(gat *formats.GAT, tileSize float32) *TileGridRenderer {
	if gat == nil {
		return nil
	}
	return &TileGridRenderer{
		gat:      gat,
		tileSize: tileSize,
	}
}

// TileVertex represents a vertex for tile grid rendering.
type TileVertex struct {
	X, Y, Z float32 // Position
	R, G, B float32 // Color
}

// GenerateGridLines generates line vertices for the tile grid.
// Returns vertices in [x, y, z, r, g, b] format for each point.
func (t *TileGridRenderer) GenerateGridLines(minX, minY, maxX, maxY int, height float32) []TileVertex {
	if t.gat == nil {
		return nil
	}

	width := int(t.gat.Width)
	gridHeight := int(t.gat.Height)

	// Clamp bounds
	if minX < 0 {
		minX = 0
	}
	if minY < 0 {
		minY = 0
	}
	if maxX > width {
		maxX = width
	}
	if maxY > gridHeight {
		maxY = gridHeight
	}

	var vertices []TileVertex
	gridColor := [3]float32{0.5, 0.5, 0.5}

	// Vertical lines
	for x := minX; x <= maxX; x++ {
		worldX := float32(x) * t.tileSize
		vertices = append(vertices,
			TileVertex{worldX, height, float32(minY) * t.tileSize, gridColor[0], gridColor[1], gridColor[2]},
			TileVertex{worldX, height, float32(maxY) * t.tileSize, gridColor[0], gridColor[1], gridColor[2]},
		)
	}

	// Horizontal lines
	for y := minY; y <= maxY; y++ {
		worldZ := float32(y) * t.tileSize
		vertices = append(vertices,
			TileVertex{float32(minX) * t.tileSize, height, worldZ, gridColor[0], gridColor[1], gridColor[2]},
			TileVertex{float32(maxX) * t.tileSize, height, worldZ, gridColor[0], gridColor[1], gridColor[2]},
		)
	}

	return vertices
}

// GenerateWalkableOverlay generates colored quads for walkability visualization.
// Returns vertices in [x, y, z, r, g, b] format, 6 vertices per tile (2 triangles).
func (t *TileGridRenderer) GenerateWalkableOverlay(minX, minY, maxX, maxY int, height float32) []TileVertex {
	if t.gat == nil {
		return nil
	}

	width := int(t.gat.Width)
	gridHeight := int(t.gat.Height)

	// Clamp bounds
	if minX < 0 {
		minX = 0
	}
	if minY < 0 {
		minY = 0
	}
	if maxX > width {
		maxX = width
	}
	if maxY > gridHeight {
		maxY = gridHeight
	}

	var vertices []TileVertex

	for y := minY; y < maxY; y++ {
		for x := minX; x < maxX; x++ {
			// Get cell type
			walkable := t.gat.IsWalkable(x, y)

			var color [3]float32
			if walkable {
				color = [3]float32{0.0, 0.5, 0.0} // Green for walkable
			} else {
				color = [3]float32{0.5, 0.0, 0.0} // Red for blocked
			}

			// Quad corners
			x0 := float32(x) * t.tileSize
			z0 := float32(y) * t.tileSize
			x1 := float32(x+1) * t.tileSize
			z1 := float32(y+1) * t.tileSize

			// Triangle 1
			vertices = append(vertices,
				TileVertex{x0, height, z0, color[0], color[1], color[2]},
				TileVertex{x1, height, z0, color[0], color[1], color[2]},
				TileVertex{x1, height, z1, color[0], color[1], color[2]},
			)

			// Triangle 2
			vertices = append(vertices,
				TileVertex{x0, height, z0, color[0], color[1], color[2]},
				TileVertex{x1, height, z1, color[0], color[1], color[2]},
				TileVertex{x0, height, z1, color[0], color[1], color[2]},
			)
		}
	}

	return vertices
}

// GenerateCellTypeOverlay generates colored quads based on GAT cell types.
// Returns vertices in [x, y, z, r, g, b] format.
func (t *TileGridRenderer) GenerateCellTypeOverlay(minX, minY, maxX, maxY int, height float32) []TileVertex {
	if t.gat == nil {
		return nil
	}

	width := int(t.gat.Width)
	gridHeight := int(t.gat.Height)

	// Clamp bounds
	if minX < 0 {
		minX = 0
	}
	if minY < 0 {
		minY = 0
	}
	if maxX > width {
		maxX = width
	}
	if maxY > gridHeight {
		maxY = gridHeight
	}

	var vertices []TileVertex

	for y := minY; y < maxY; y++ {
		for x := minX; x < maxX; x++ {
			cell := t.gat.GetCell(x, y)
			if cell == nil {
				continue
			}
			color := cellTypeColor(cell.Type)

			// Quad corners
			x0 := float32(x) * t.tileSize
			z0 := float32(y) * t.tileSize
			x1 := float32(x+1) * t.tileSize
			z1 := float32(y+1) * t.tileSize

			// Triangle 1
			vertices = append(vertices,
				TileVertex{x0, height, z0, color[0], color[1], color[2]},
				TileVertex{x1, height, z0, color[0], color[1], color[2]},
				TileVertex{x1, height, z1, color[0], color[1], color[2]},
			)

			// Triangle 2
			vertices = append(vertices,
				TileVertex{x0, height, z0, color[0], color[1], color[2]},
				TileVertex{x1, height, z1, color[0], color[1], color[2]},
				TileVertex{x0, height, z1, color[0], color[1], color[2]},
			)
		}
	}

	return vertices
}

// GeneratePathOverlay generates colored quads for a path visualization.
// Returns vertices in [x, y, z, r, g, b] format.
func (t *TileGridRenderer) GeneratePathOverlay(path [][2]int, height float32) []TileVertex {
	if len(path) == 0 {
		return nil
	}

	var vertices []TileVertex

	for i, tile := range path {
		// Color gradient from start (blue) to end (green)
		progress := float32(i) / float32(len(path)-1)
		color := [3]float32{
			0.2,
			0.3 + 0.5*progress,
			1.0 - 0.8*progress,
		}

		x := tile[0]
		y := tile[1]

		// Slightly smaller quad (inset by 10%)
		inset := t.tileSize * 0.1
		x0 := float32(x)*t.tileSize + inset
		z0 := float32(y)*t.tileSize + inset
		x1 := float32(x+1)*t.tileSize - inset
		z1 := float32(y+1)*t.tileSize - inset

		// Triangle 1
		vertices = append(vertices,
			TileVertex{x0, height + 0.1, z0, color[0], color[1], color[2]},
			TileVertex{x1, height + 0.1, z0, color[0], color[1], color[2]},
			TileVertex{x1, height + 0.1, z1, color[0], color[1], color[2]},
		)

		// Triangle 2
		vertices = append(vertices,
			TileVertex{x0, height + 0.1, z0, color[0], color[1], color[2]},
			TileVertex{x1, height + 0.1, z1, color[0], color[1], color[2]},
			TileVertex{x0, height + 0.1, z1, color[0], color[1], color[2]},
		)
	}

	return vertices
}

// cellTypeColor returns a color for a GAT cell type.
func cellTypeColor(cellType formats.GATCellType) [3]float32 {
	switch cellType {
	case formats.GATWalkable:
		return [3]float32{0.0, 0.5, 0.0} // Green
	case formats.GATBlocked:
		return [3]float32{0.5, 0.0, 0.0} // Red
	case formats.GATWater:
		return [3]float32{0.0, 0.3, 0.6} // Blue
	case formats.GATWalkableWater:
		return [3]float32{0.0, 0.5, 0.5} // Cyan
	case formats.GATSnipeable:
		return [3]float32{0.5, 0.5, 0.0} // Yellow
	case formats.GATBlockedSnipe:
		return [3]float32{0.5, 0.3, 0.0} // Orange
	default:
		return [3]float32{0.3, 0.3, 0.3} // Gray for unknown
	}
}

// TileInfo contains information about a specific tile.
type TileInfo struct {
	X, Y     int
	Type     formats.GATCellType
	Walkable bool
	Heights  [4]float32 // Corner heights
}

// GetTileInfo returns information about a specific tile.
func (t *TileGridRenderer) GetTileInfo(tileX, tileY int) *TileInfo {
	if t.gat == nil {
		return nil
	}

	cell := t.gat.GetCell(tileX, tileY)
	if cell == nil {
		return nil
	}

	return &TileInfo{
		X:        tileX,
		Y:        tileY,
		Type:     cell.Type,
		Walkable: cell.Type.IsWalkable(),
		Heights:  cell.Heights,
	}
}
