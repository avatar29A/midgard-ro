package formats

import "testing"

func TestGAT_IsWalkable(t *testing.T) {
	// Create test GAT with 3x3 grid
	gat := &GAT{
		Width:  3,
		Height: 3,
		Cells: []GATCell{
			{CellType: 0}, {CellType: 1}, {CellType: 0}, // Row 0
			{CellType: 1}, {CellType: 0}, {CellType: 1}, // Row 1
			{CellType: 0}, {CellType: 0}, {CellType: 0}, // Row 2
		},
	}

	tests := []struct {
		name     string
		x, y     int
		expected bool
	}{
		{"walkable top-left", 0, 0, true},
		{"non-walkable top-middle", 1, 0, false},
		{"walkable top-right", 2, 0, true},
		{"non-walkable middle-left", 0, 1, false},
		{"walkable center", 1, 1, true},
		{"non-walkable middle-right", 2, 1, false},
		{"walkable bottom row", 0, 2, true},
		{"out of bounds negative x", -1, 0, false},
		{"out of bounds negative y", 0, -1, false},
		{"out of bounds x too large", 3, 0, false},
		{"out of bounds y too large", 0, 3, false},
		{"out of bounds both", 10, 10, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := gat.IsWalkable(tt.x, tt.y)
			if result != tt.expected {
				t.Errorf("IsWalkable(%d, %d) = %v, expected %v", tt.x, tt.y, result, tt.expected)
			}
		})
	}
}

func TestGAT_IsWalkable_Nil(t *testing.T) {
	var gat *GAT
	if gat.IsWalkable(0, 0) {
		t.Error("nil GAT should return false for IsWalkable")
	}
}

func TestGAT_IsWalkable_EmptyCells(t *testing.T) {
	gat := &GAT{
		Width:  5,
		Height: 5,
		Cells:  []GATCell{}, // Empty cells slice
	}

	// Should return false since cells array is empty
	if gat.IsWalkable(0, 0) {
		t.Error("GAT with empty cells should return false")
	}
}

func TestGAT_IsWalkable_InsufficientCells(t *testing.T) {
	gat := &GAT{
		Width:  3,
		Height: 3,
		Cells:  []GATCell{{CellType: 0}}, // Only 1 cell, but should have 9
	}

	// Accessing (2, 2) would require index 8, but we only have 1 cell
	if gat.IsWalkable(2, 2) {
		t.Error("accessing beyond cells array bounds should return false")
	}

	// (0, 0) should work
	if !gat.IsWalkable(0, 0) {
		t.Error("first cell should be walkable")
	}
}

func TestGATCell_Types(t *testing.T) {
	tests := []struct {
		cellType uint8
		walkable bool
	}{
		{0, true},  // Type 0 is walkable
		{1, false}, // Type 1 is non-walkable
		{2, false}, // Type 2 is non-walkable
		{3, false}, // Type 3 is non-walkable
		{4, false}, // Type 4 is non-walkable
		{5, false}, // Type 5 is non-walkable
	}

	for _, tt := range tests {
		t.Run(string(rune(tt.cellType+'0')), func(t *testing.T) {
			gat := &GAT{
				Width:  1,
				Height: 1,
				Cells:  []GATCell{{CellType: tt.cellType}},
			}

			result := gat.IsWalkable(0, 0)
			if result != tt.walkable {
				t.Errorf("cell type %d: IsWalkable = %v, expected %v", tt.cellType, result, tt.walkable)
			}
		})
	}
}

func TestGATCell_Heights(t *testing.T) {
	// Test that heights are stored correctly
	cell := GATCell{
		Heights:  [4]float32{1.0, 2.0, 3.0, 4.0},
		CellType: 0,
	}

	if cell.Heights[0] != 1.0 {
		t.Errorf("expected height[0] = 1.0, got %f", cell.Heights[0])
	}
	if cell.Heights[3] != 4.0 {
		t.Errorf("expected height[3] = 4.0, got %f", cell.Heights[3])
	}
}

func TestGND_Basic(t *testing.T) {
	gnd := &GND{
		Width:    100,
		Height:   100,
		Textures: []string{"texture1.bmp", "texture2.bmp"},
	}

	if gnd.Width != 100 {
		t.Errorf("expected width 100, got %d", gnd.Width)
	}
	if len(gnd.Textures) != 2 {
		t.Errorf("expected 2 textures, got %d", len(gnd.Textures))
	}
}

func TestRSW_Basic(t *testing.T) {
	rsw := &RSW{
		MapName: "prontera",
		Objects: []RSWObject{
			{
				Name:     "building01",
				Position: [3]float32{100, 0, 200},
				Rotation: [3]float32{0, 45, 0},
				Scale:    [3]float32{1, 1, 1},
			},
		},
	}

	if rsw.MapName != "prontera" {
		t.Errorf("expected map name 'prontera', got %s", rsw.MapName)
	}
	if len(rsw.Objects) != 1 {
		t.Errorf("expected 1 object, got %d", len(rsw.Objects))
	}
	if rsw.Objects[0].Name != "building01" {
		t.Errorf("expected object name 'building01', got %s", rsw.Objects[0].Name)
	}
}

func TestRSWObject_Coordinates(t *testing.T) {
	obj := RSWObject{
		Name:     "test_object",
		Position: [3]float32{10.5, 20.3, 30.7},
		Rotation: [3]float32{0, 90, 0},
		Scale:    [3]float32{2.0, 1.5, 2.0},
	}

	// Test position
	if obj.Position[0] != 10.5 {
		t.Errorf("expected position X = 10.5, got %f", obj.Position[0])
	}
	if obj.Position[1] != 20.3 {
		t.Errorf("expected position Y = 20.3, got %f", obj.Position[1])
	}
	if obj.Position[2] != 30.7 {
		t.Errorf("expected position Z = 30.7, got %f", obj.Position[2])
	}

	// Test rotation
	if obj.Rotation[1] != 90 {
		t.Errorf("expected rotation Y = 90, got %f", obj.Rotation[1])
	}

	// Test scale
	if obj.Scale[0] != 2.0 {
		t.Errorf("expected scale X = 2.0, got %f", obj.Scale[0])
	}
}
