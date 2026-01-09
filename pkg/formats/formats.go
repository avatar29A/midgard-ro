// Package formats provides parsers for Ragnarok Online file formats.
package formats

// GAT represents ground altitude and tile data.
// Used for collision detection and pathfinding.
type GAT struct {
	Width  int
	Height int
	Cells  []GATCell
}

// GATCell represents a single cell in the GAT grid.
type GATCell struct {
	Heights  [4]float32 // Corner heights
	CellType uint8      // 0=walkable, 1=non-walkable, etc.
}

// IsWalkable checks if a cell at the given coordinates is walkable.
func (g *GAT) IsWalkable(x, y int) bool {
	if g == nil || x < 0 || y < 0 || x >= g.Width || y >= g.Height {
		return false
	}
	idx := y*g.Width + x
	if idx >= len(g.Cells) {
		return false
	}
	return g.Cells[idx].CellType == 0
}

// GND represents ground mesh data.
// Contains textures and geometry for the map surface.
type GND struct {
	Width    int
	Height   int
	Textures []string
}

// RSW represents resource/world data.
// Contains objects, lights, sounds, and effects placed on the map.
type RSW struct {
	MapName string
	Objects []RSWObject
}

// RSWObject represents an object placed in the world.
type RSWObject struct {
	Name     string
	Position [3]float32
	Rotation [3]float32
	Scale    [3]float32
}
