package formats

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// createTestGAT creates a minimal valid GAT file for testing.
func createTestGAT(width, height uint32, cellTypes []GATCellType) []byte {
	buf := new(bytes.Buffer)

	// Magic "GRAT"
	buf.WriteString("GRAT")

	// Version 1.2 (stored as minor, major)
	buf.WriteByte(2) // minor
	buf.WriteByte(1) // major

	// Dimensions
	binary.Write(buf, binary.LittleEndian, width)
	binary.Write(buf, binary.LittleEndian, height)

	// Cells
	cellCount := int(width * height)
	for i := 0; i < cellCount; i++ {
		// Heights (4 floats)
		for j := 0; j < 4; j++ {
			binary.Write(buf, binary.LittleEndian, float32(0.0))
		}
		// Cell type
		cellType := GATWalkable
		if i < len(cellTypes) {
			cellType = cellTypes[i]
		}
		binary.Write(buf, binary.LittleEndian, uint32(cellType))
	}

	return buf.Bytes()
}

func TestParseGAT_ValidFile(t *testing.T) {
	data := createTestGAT(4, 4, nil)

	gat, err := ParseGAT(data)
	if err != nil {
		t.Fatalf("ParseGAT failed: %v", err)
	}

	if gat.Version.Major != 1 || gat.Version.Minor != 2 {
		t.Errorf("expected version 1.2, got %s", gat.Version)
	}

	if gat.Width != 4 {
		t.Errorf("expected width 4, got %d", gat.Width)
	}

	if gat.Height != 4 {
		t.Errorf("expected height 4, got %d", gat.Height)
	}

	if len(gat.Cells) != 16 {
		t.Errorf("expected 16 cells, got %d", len(gat.Cells))
	}
}

func TestParseGAT_CellTypes(t *testing.T) {
	cellTypes := []GATCellType{
		GATWalkable,
		GATBlocked,
		GATWater,
		GATWalkableWater,
		GATSnipeable,
		GATBlockedSnipe,
	}
	data := createTestGAT(3, 2, cellTypes)

	gat, err := ParseGAT(data)
	if err != nil {
		t.Fatalf("ParseGAT failed: %v", err)
	}

	for i, expected := range cellTypes {
		if gat.Cells[i].Type != expected {
			t.Errorf("cell %d: expected type %v, got %v", i, expected, gat.Cells[i].Type)
		}
	}
}

func TestParseGAT_InvalidMagic(t *testing.T) {
	data := []byte("XXXX\x02\x01\x04\x00\x00\x00\x04\x00\x00\x00")

	_, err := ParseGAT(data)
	if err == nil {
		t.Error("expected error for invalid magic")
	}
}

func TestParseGAT_TruncatedData(t *testing.T) {
	_, err := ParseGAT([]byte("GRAT"))
	if err == nil {
		t.Error("expected error for truncated data")
	}
}

func TestGATCellType_IsWalkable(t *testing.T) {
	tests := []struct {
		cellType GATCellType
		expected bool
	}{
		{GATWalkable, true},
		{GATBlocked, false},
		{GATWater, false},
		{GATWalkableWater, true},
		{GATSnipeable, false},
		{GATBlockedSnipe, false},
	}

	for _, tc := range tests {
		if tc.cellType.IsWalkable() != tc.expected {
			t.Errorf("%v.IsWalkable() = %v, expected %v", tc.cellType, tc.cellType.IsWalkable(), tc.expected)
		}
	}
}

func TestGATCellType_IsBlocked(t *testing.T) {
	tests := []struct {
		cellType GATCellType
		expected bool
	}{
		{GATWalkable, false},
		{GATBlocked, true},
		{GATWater, false},
		{GATWalkableWater, false},
		{GATSnipeable, false},
		{GATBlockedSnipe, true},
	}

	for _, tc := range tests {
		if tc.cellType.IsBlocked() != tc.expected {
			t.Errorf("%v.IsBlocked() = %v, expected %v", tc.cellType, tc.cellType.IsBlocked(), tc.expected)
		}
	}
}

func TestGATCellType_IsWater(t *testing.T) {
	tests := []struct {
		cellType GATCellType
		expected bool
	}{
		{GATWalkable, false},
		{GATBlocked, false},
		{GATWater, true},
		{GATWalkableWater, true},
		{GATSnipeable, false},
		{GATBlockedSnipe, false},
	}

	for _, tc := range tests {
		if tc.cellType.IsWater() != tc.expected {
			t.Errorf("%v.IsWater() = %v, expected %v", tc.cellType, tc.cellType.IsWater(), tc.expected)
		}
	}
}

func TestGAT_GetCell(t *testing.T) {
	data := createTestGAT(4, 4, nil)
	gat, _ := ParseGAT(data)

	// Valid cell
	cell := gat.GetCell(2, 3)
	if cell == nil {
		t.Error("GetCell(2, 3) returned nil for valid coordinates")
	}

	// Out of bounds
	if gat.GetCell(-1, 0) != nil {
		t.Error("GetCell(-1, 0) should return nil")
	}
	if gat.GetCell(0, -1) != nil {
		t.Error("GetCell(0, -1) should return nil")
	}
	if gat.GetCell(4, 0) != nil {
		t.Error("GetCell(4, 0) should return nil")
	}
	if gat.GetCell(0, 4) != nil {
		t.Error("GetCell(0, 4) should return nil")
	}
}

func TestGAT_IsWalkable(t *testing.T) {
	cellTypes := []GATCellType{
		GATWalkable,  // (0,0)
		GATBlocked,   // (1,0)
		GATWater,     // (0,1)
		GATSnipeable, // (1,1)
	}
	data := createTestGAT(2, 2, cellTypes)
	gat, _ := ParseGAT(data)

	if !gat.IsWalkable(0, 0) {
		t.Error("(0,0) should be walkable")
	}
	if gat.IsWalkable(1, 0) {
		t.Error("(1,0) should not be walkable")
	}
	if gat.IsWalkable(0, 1) {
		t.Error("(0,1) water should not be walkable")
	}
	if gat.IsWalkable(-1, 0) {
		t.Error("out of bounds should not be walkable")
	}
}

func TestGAT_CountByType(t *testing.T) {
	cellTypes := []GATCellType{
		GATWalkable, GATWalkable, GATWalkable,
		GATBlocked, GATBlocked,
		GATWater,
	}
	data := createTestGAT(3, 2, cellTypes)
	gat, _ := ParseGAT(data)

	counts := gat.CountByType()

	if counts[GATWalkable] != 3 {
		t.Errorf("expected 3 walkable, got %d", counts[GATWalkable])
	}
	if counts[GATBlocked] != 2 {
		t.Errorf("expected 2 blocked, got %d", counts[GATBlocked])
	}
	if counts[GATWater] != 1 {
		t.Errorf("expected 1 water, got %d", counts[GATWater])
	}
}

func TestGATCell_AverageHeight(t *testing.T) {
	cell := GATCell{
		Heights: [4]float32{10.0, 20.0, 30.0, 40.0},
	}

	avg := cell.AverageHeight()
	if avg != 25.0 {
		t.Errorf("expected average 25.0, got %f", avg)
	}
}

func TestGATCellType_String(t *testing.T) {
	tests := []struct {
		cellType GATCellType
		expected string
	}{
		{GATWalkable, "Walkable"},
		{GATBlocked, "Blocked"},
		{GATWater, "Water"},
		{GATWalkableWater, "Walkable+Water"},
		{GATSnipeable, "Snipeable"},
		{GATBlockedSnipe, "Blocked+Snipe"},
		{GATCellType(99), "Unknown(99)"},
	}

	for _, tc := range tests {
		if tc.cellType.String() != tc.expected {
			t.Errorf("%d.String() = %q, expected %q", tc.cellType, tc.cellType.String(), tc.expected)
		}
	}
}
