package formats

import "testing"

// Note: GAT tests are in gat_test.go
// Note: GND tests are in gnd_test.go

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

	// Test name
	if obj.Name != "test_object" {
		t.Errorf("expected name 'test_object', got %s", obj.Name)
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
