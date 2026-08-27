package water

import (
	"testing"

	"github.com/Faultbox/midgard-ro/pkg/formats"
)

// A 2×2 map: the void (no surface), dry ground, a lake bed, and a shore whose
// one deep corner counts only once the wave height is allowed for.
func sampleGND() *formats.GND {
	flat := func(a float32) [4]float32 { return [4]float32{a, a, a, a} }
	return &formats.GND{
		Width: 2, Height: 2, Zoom: 10,
		Tiles: []formats.GNDTile{
			{Altitude: flat(100), TopSurface: -1},              // void: deep, but no ground
			{Altitude: flat(0), TopSurface: 0},                 // dry ground above the water
			{Altitude: flat(60), TopSurface: 0},                // lake bed below it
			{Altitude: [4]float32{0, 0, 0, 45}, TopSurface: 0}, // shore
		},
	}
}

func TestBuildCellsFollowsTheGroundRule(t *testing.T) {
	m := BuildCells(sampleGND(), 50, 0)
	if m.Cells != 1 {
		t.Fatalf("%d cells at level 50 with no waves, want just the lake bed", m.Cells)
	}
	if len(m.Vertices) != 6*3 {
		t.Fatalf("%d floats, want six vertices of three", len(m.Vertices))
	}
	// The lake bed is tile (0,1): x 0..10, z 10..20, at the water's height.
	if m.Vertices[0] != 0 || m.Vertices[1] != -50 || m.Vertices[2] != 10 {
		t.Fatalf("first vertex %v, want (0, -50, 10)", m.Vertices[:3])
	}

	m = BuildCells(sampleGND(), 50, 10)
	if m.Cells != 2 {
		t.Fatalf("%d cells with a wave height of 10, want the lake bed and the shore", m.Cells)
	}
}

func TestBuildCellsNeverFloodsTheVoid(t *testing.T) {
	// A surface above every corner: all the ground is under water.
	m := BuildCells(sampleGND(), -10, 0)
	if m.Cells != 3 {
		t.Fatalf("%d cells, want the three with ground and never the void", m.Cells)
	}

	// A surface below every corner: nothing is.
	if m := BuildCells(sampleGND(), 200, 0); m.Cells != 0 {
		t.Fatalf("%d cells with the water below all the ground, want none", m.Cells)
	}
}

func TestBuildCellsWithoutGround(t *testing.T) {
	if m := BuildCells(nil, 50, 0); m.Cells != 0 || len(m.Vertices) != 0 {
		t.Fatal("no GND, no water")
	}
}
