package world

import (
	"testing"

	"github.com/Faultbox/midgard-ro/pkg/formats"
)

// mockGAT creates a simple GAT for testing.
func mockGAT(width, height int, blocked [][2]int) *formats.GAT {
	gat := &formats.GAT{
		Width:  uint32(width),
		Height: uint32(height),
		Cells:  make([]formats.GATCell, width*height),
	}

	// Mark all cells as walkable by default
	for i := range gat.Cells {
		gat.Cells[i].Type = formats.GATWalkable
	}

	// Mark blocked cells
	for _, b := range blocked {
		idx := b[1]*width + b[0]
		if idx >= 0 && idx < len(gat.Cells) {
			gat.Cells[idx].Type = formats.GATBlocked
		}
	}

	return gat
}

func TestPathFinder_FindPath_Simple(t *testing.T) {
	// 5x5 grid, no obstacles
	gat := mockGAT(5, 5, nil)
	pf := NewPathFinder(gat)

	path := pf.FindPath(0, 0, 4, 4)
	if path == nil {
		t.Fatal("expected path, got nil")
	}

	// Path should start at (0,0) and end at (4,4)
	if path[0][0] != 0 || path[0][1] != 0 {
		t.Errorf("path should start at (0,0), got (%d,%d)", path[0][0], path[0][1])
	}

	lastIdx := len(path) - 1
	if path[lastIdx][0] != 4 || path[lastIdx][1] != 4 {
		t.Errorf("path should end at (4,4), got (%d,%d)", path[lastIdx][0], path[lastIdx][1])
	}
}

func TestPathFinder_FindPath_WithObstacle(t *testing.T) {
	// 5x5 grid with wall in the middle
	blocked := [][2]int{
		{2, 0}, {2, 1}, {2, 2}, {2, 3},
	}
	gat := mockGAT(5, 5, blocked)
	pf := NewPathFinder(gat)

	path := pf.FindPath(0, 2, 4, 2)
	if path == nil {
		t.Fatal("expected path around obstacle, got nil")
	}

	// Verify path doesn't go through blocked cells
	for _, p := range path {
		if p[0] == 2 && p[1] < 4 {
			t.Errorf("path went through blocked cell at (%d,%d)", p[0], p[1])
		}
	}
}

func TestPathFinder_FindPath_NoPath(t *testing.T) {
	// 5x5 grid with complete wall
	blocked := [][2]int{
		{2, 0}, {2, 1}, {2, 2}, {2, 3}, {2, 4},
	}
	gat := mockGAT(5, 5, blocked)
	pf := NewPathFinder(gat)

	path := pf.FindPath(0, 2, 4, 2)
	if path != nil {
		t.Errorf("expected no path, got %v", path)
	}
}

func TestPathFinder_FindPath_SameStartGoal(t *testing.T) {
	gat := mockGAT(5, 5, nil)
	pf := NewPathFinder(gat)

	path := pf.FindPath(2, 2, 2, 2)
	if path == nil || len(path) == 0 {
		t.Fatal("expected path with single node")
	}

	if len(path) != 1 {
		t.Errorf("expected path length 1, got %d", len(path))
	}
}

func TestPathFinder_FindPath_OutOfBounds(t *testing.T) {
	gat := mockGAT(5, 5, nil)
	pf := NewPathFinder(gat)

	// Start out of bounds
	path := pf.FindPath(-1, 0, 4, 4)
	if path != nil {
		t.Error("expected nil for out of bounds start")
	}

	// Goal out of bounds
	path = pf.FindPath(0, 0, 10, 10)
	if path != nil {
		t.Error("expected nil for out of bounds goal")
	}
}

func TestPathFinder_FindPath_BlockedGoal(t *testing.T) {
	blocked := [][2]int{{4, 4}}
	gat := mockGAT(5, 5, blocked)
	pf := NewPathFinder(gat)

	path := pf.FindPath(0, 0, 4, 4)
	if path != nil {
		t.Error("expected nil for blocked goal")
	}
}

func TestPathFinder_IsWalkable(t *testing.T) {
	blocked := [][2]int{{2, 2}}
	gat := mockGAT(5, 5, blocked)
	pf := NewPathFinder(gat)

	if pf.IsWalkable(2, 2) {
		t.Error("expected (2,2) to be blocked")
	}

	if !pf.IsWalkable(0, 0) {
		t.Error("expected (0,0) to be walkable")
	}

	if pf.IsWalkable(-1, 0) {
		t.Error("expected out of bounds to be not walkable")
	}
}
