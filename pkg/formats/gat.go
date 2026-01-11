// Package formats provides parsers for Ragnarok Online file formats.
package formats

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
)

// GAT format errors.
var (
	ErrInvalidGATMagic       = errors.New("invalid GAT magic: expected 'GRAT'")
	ErrUnsupportedGATVersion = errors.New("unsupported GAT version")
	ErrTruncatedGATData      = errors.New("truncated GAT data")
)

// GATVersion represents the GAT file version.
type GATVersion struct {
	Major uint8
	Minor uint8
}

// String returns the version as "Major.Minor".
func (v GATVersion) String() string {
	return fmt.Sprintf("%d.%d", v.Major, v.Minor)
}

// GATCellType represents the walkability type of a cell.
type GATCellType uint32

// Cell type constants.
const (
	GATWalkable      GATCellType = 0 // Normal walkable ground
	GATBlocked       GATCellType = 1 // Cannot walk through
	GATWater         GATCellType = 2 // Water (walkable with certain skills)
	GATWalkableWater GATCellType = 3 // Shore/shallow water
	GATSnipeable     GATCellType = 4 // Can attack over but not walk (cliffs)
	GATBlockedSnipe  GATCellType = 5 // Blocked but can shoot over
)

// String returns a human-readable cell type name.
func (t GATCellType) String() string {
	switch t {
	case GATWalkable:
		return "Walkable"
	case GATBlocked:
		return "Blocked"
	case GATWater:
		return "Water"
	case GATWalkableWater:
		return "Walkable+Water"
	case GATSnipeable:
		return "Snipeable"
	case GATBlockedSnipe:
		return "Blocked+Snipe"
	default:
		return fmt.Sprintf("Unknown(%d)", t)
	}
}

// IsWalkable returns true if the cell type allows walking.
func (t GATCellType) IsWalkable() bool {
	return t == GATWalkable || t == GATWalkableWater
}

// IsBlocked returns true if the cell blocks movement.
func (t GATCellType) IsBlocked() bool {
	return t == GATBlocked || t == GATBlockedSnipe
}

// IsWater returns true if the cell contains water.
func (t GATCellType) IsWater() bool {
	return t == GATWater || t == GATWalkableWater
}

// IsSnipeable returns true if projectiles can pass over the cell.
func (t GATCellType) IsSnipeable() bool {
	return t == GATSnipeable || t == GATBlockedSnipe
}

// GATCell represents a single cell in the GAT grid.
type GATCell struct {
	// Heights contains the altitude of each corner:
	// [0] = bottom-left, [1] = bottom-right, [2] = top-left, [3] = top-right
	Heights [4]float32
	Type    GATCellType
}

// AverageHeight returns the average altitude of all four corners.
func (c *GATCell) AverageHeight() float32 {
	return (c.Heights[0] + c.Heights[1] + c.Heights[2] + c.Heights[3]) / 4.0
}

// GAT represents a parsed Ground Altitude Table file.
type GAT struct {
	Version GATVersion
	Width   uint32
	Height  uint32
	Cells   []GATCell
}

// GetCell returns the cell at the given coordinates.
// Returns nil if coordinates are out of bounds.
func (g *GAT) GetCell(x, y int) *GATCell {
	if x < 0 || y < 0 || x >= int(g.Width) || y >= int(g.Height) {
		return nil
	}
	return &g.Cells[y*int(g.Width)+x]
}

// IsWalkable checks if the cell at (x, y) is walkable.
func (g *GAT) IsWalkable(x, y int) bool {
	cell := g.GetCell(x, y)
	if cell == nil {
		return false
	}
	return cell.Type.IsWalkable()
}

// ParseGAT parses a GAT file from raw bytes.
func ParseGAT(data []byte) (*GAT, error) {
	if len(data) < 14 {
		return nil, ErrTruncatedGATData
	}

	// Check magic "GRAT"
	if string(data[0:4]) != "GRAT" {
		return nil, ErrInvalidGATMagic
	}

	// Version is stored as [minor, major]
	version := GATVersion{
		Major: data[5],
		Minor: data[4],
	}

	// Supported versions: 1.2, 1.3, 2.x, 3.x (cell format is identical)
	if version.Major < 1 || version.Major > 3 {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedGATVersion, version)
	}

	r := bytes.NewReader(data[6:])

	// Read dimensions
	var width, height uint32
	if err := binary.Read(r, binary.LittleEndian, &width); err != nil {
		return nil, fmt.Errorf("%w: reading width", ErrTruncatedGATData)
	}
	if err := binary.Read(r, binary.LittleEndian, &height); err != nil {
		return nil, fmt.Errorf("%w: reading height", ErrTruncatedGATData)
	}

	// Validate dimensions (maps can be up to ~512x512 cells typically, but allow larger)
	if width == 0 || height == 0 || width > 4096 || height > 4096 {
		return nil, fmt.Errorf("invalid GAT dimensions: %dx%d", width, height)
	}

	cellCount := int(width * height)
	gat := &GAT{
		Version: version,
		Width:   width,
		Height:  height,
		Cells:   make([]GATCell, cellCount),
	}

	// Read cells
	for i := 0; i < cellCount; i++ {
		cell, err := parseGATCell(r)
		if err != nil {
			return nil, fmt.Errorf("parsing cell %d: %w", i, err)
		}
		gat.Cells[i] = cell
	}

	return gat, nil
}

// parseGATCell parses a single GAT cell.
// Cell format is identical for all supported versions (1.x and 2.x).
func parseGATCell(r *bytes.Reader) (GATCell, error) {
	var cell GATCell

	// Read 4 corner heights
	for i := 0; i < 4; i++ {
		if err := binary.Read(r, binary.LittleEndian, &cell.Heights[i]); err != nil {
			return GATCell{}, fmt.Errorf("%w: reading height %d", ErrTruncatedGATData, i)
		}
	}

	// Read cell type
	if err := binary.Read(r, binary.LittleEndian, &cell.Type); err != nil {
		return GATCell{}, fmt.Errorf("%w: reading cell type", ErrTruncatedGATData)
	}

	return cell, nil
}

// ParseGATFile parses a GAT file from disk.
func ParseGATFile(path string) (*GAT, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading GAT file: %w", err)
	}
	return ParseGAT(data)
}

// CountByType returns the count of cells for each type.
func (g *GAT) CountByType() map[GATCellType]int {
	counts := make(map[GATCellType]int)
	for _, cell := range g.Cells {
		counts[cell.Type]++
	}
	return counts
}

// GetAltitudeRange returns the minimum and maximum altitude in the map.
func (g *GAT) GetAltitudeRange() (min, max float32) {
	if len(g.Cells) == 0 {
		return 0, 0
	}

	min = g.Cells[0].Heights[0]
	max = g.Cells[0].Heights[0]

	for _, cell := range g.Cells {
		for _, h := range cell.Heights {
			if h < min {
				min = h
			}
			if h > max {
				max = h
			}
		}
	}

	return min, max
}
