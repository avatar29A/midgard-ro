package entity

import (
	"math"
	"testing"
)

func TestCellWorldRoundTrip(t *testing.T) {
	tests := []struct {
		name         string
		cellX, cellY int
	}{
		{"origin", 0, 0},
		{"prontera spawn", 153, 244},
		{"far corner", 311, 391},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wx, wz := CellToWorld(tt.cellX, tt.cellY)
			gotX, gotY := WorldToCell(wx, wz)
			if gotX != tt.cellX || gotY != tt.cellY {
				t.Errorf("round trip = (%d,%d), want (%d,%d)", gotX, gotY, tt.cellX, tt.cellY)
			}
			// The world position must be the cell's center, not its corner.
			if wantX := (float32(tt.cellX) + 0.5) * CellSize; wx != wantX {
				t.Errorf("worldX = %v, want %v", wx, wantX)
			}
		})
	}
}

func TestDirectionFromCellDelta(t *testing.T) {
	// Server coordinates: +x is east, +y is north.
	tests := []struct {
		name   string
		dx, dy int
		want   int
	}{
		{"north", 0, 1, DirN},
		{"south", 0, -1, DirS},
		{"east", 1, 0, DirE},
		{"west", -1, 0, DirW},
		{"northeast", 1, 1, DirNE},
		{"northwest", -1, 1, DirNW},
		{"southeast", 1, -1, DirSE},
		{"southwest", -1, -1, DirSW},
		{"stationary", 0, 0, -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DirectionFromCellDelta(tt.dx, tt.dy); got != tt.want {
				t.Errorf("DirectionFromCellDelta(%d,%d) = %d, want %d", tt.dx, tt.dy, got, tt.want)
			}
		})
	}
}

// TestWalkSpeedIsMillisecondsPerCell is the regression guard for the bug this
// package was built to fix: rAthena's `speed` stat is a *duration per cell*,
// not a velocity. A default character must take exactly 150 ms to cross one
// cell — treating 150 as world-units-per-second made it ~4.5x too fast.
func TestWalkSpeedIsMillisecondsPerCell(t *testing.T) {
	c := newWalkerAt(10, 10)
	c.FollowPath([][2]int{{10, 10}, {11, 10}})

	// One tick short of the cell duration: still walking, not yet arrived.
	c.Update(DefaultWalkSpeedMs - 1)
	if !c.IsWalkingPath() {
		t.Fatal("arrived before one cell duration elapsed")
	}

	c.Update(1)
	if c.IsWalkingPath() {
		t.Error("still walking after one cell duration elapsed")
	}
	if gotX, gotY := c.CurrentCell(); gotX != 11 || gotY != 10 {
		t.Errorf("ended at cell (%d,%d), want (11,10)", gotX, gotY)
	}
}

func TestWalkDiagonalCostsMore(t *testing.T) {
	straight := newWalkerAt(10, 10)
	straight.FollowPath([][2]int{{10, 10}, {11, 10}})

	diagonal := newWalkerAt(10, 10)
	diagonal.FollowPath([][2]int{{10, 10}, {11, 11}})

	// After exactly one straight-cell duration the straight walker is done
	// and the diagonal one is still going, because diagonals cost 1.4x.
	straight.Update(DefaultWalkSpeedMs)
	diagonal.Update(DefaultWalkSpeedMs)

	if straight.IsWalkingPath() {
		t.Error("straight step should have completed")
	}
	if !diagonal.IsWalkingPath() {
		t.Error("diagonal step should still be in progress")
	}

	diagonal.Update(DefaultWalkSpeedMs*DiagonalCostFactor - DefaultWalkSpeedMs)
	if diagonal.IsWalkingPath() {
		t.Error("diagonal step should have completed at 1.4x the duration")
	}
}

func TestWalkMultiCellPathTiming(t *testing.T) {
	c := newWalkerAt(0, 0)
	// 4 cells = 3 straight steps.
	c.FollowPath([][2]int{{0, 0}, {1, 0}, {2, 0}, {3, 0}})

	// Advance in small slices, the way real frames arrive.
	const frame = 16.0
	elapsed := 0.0
	for c.IsWalkingPath() && elapsed < 10_000 {
		c.Update(frame)
		elapsed += frame
	}

	want := 3 * DefaultWalkSpeedMs
	if math.Abs(elapsed-want) > frame {
		t.Errorf("3-cell walk took %.0f ms, want ~%.0f ms", elapsed, want)
	}
	if gotX, gotY := c.CurrentCell(); gotX != 3 || gotY != 0 {
		t.Errorf("ended at cell (%d,%d), want (3,0)", gotX, gotY)
	}
}

// TestWalkSurvivesLongFrame checks that a stall (say a texture upload) is
// absorbed by consuming several cells at once rather than desyncing.
func TestWalkSurvivesLongFrame(t *testing.T) {
	c := newWalkerAt(0, 0)
	c.FollowPath([][2]int{{0, 0}, {1, 0}, {2, 0}, {3, 0}})

	c.Update(10 * DefaultWalkSpeedMs)

	if c.IsWalkingPath() {
		t.Error("path should be finished after a frame longer than the whole walk")
	}
	if gotX, gotY := c.CurrentCell(); gotX != 3 || gotY != 0 {
		t.Errorf("ended at cell (%d,%d), want (3,0)", gotX, gotY)
	}
}

func TestWalkSetsFacingFromPath(t *testing.T) {
	c := newWalkerAt(10, 10)
	c.FollowPath([][2]int{{10, 10}, {10, 11}}) // north
	if c.Direction != DirN {
		t.Errorf("Direction = %d, want DirN(%d)", c.Direction, DirN)
	}

	c.FollowPath([][2]int{{10, 10}, {9, 9}}) // southwest
	if c.Direction != DirSW {
		t.Errorf("Direction = %d, want DirSW(%d)", c.Direction, DirSW)
	}
}

func TestFollowPathTooShortStopsWalking(t *testing.T) {
	c := newWalkerAt(5, 5)
	c.FollowPath([][2]int{{5, 5}})

	if c.IsWalkingPath() {
		t.Error("a single-cell path is not a walk")
	}
	if c.IsMoving {
		t.Error("IsMoving should be false for a degenerate path")
	}
}

func TestCellLineFallback(t *testing.T) {
	// Diagonal-then-straight, matching how RO steps.
	path := CellLine(0, 0, 3, 1)
	want := [][2]int{{0, 0}, {1, 1}, {2, 1}, {3, 1}}

	if len(path) != len(want) {
		t.Fatalf("CellLine length = %d, want %d (%v)", len(path), len(want), path)
	}
	for i := range want {
		if path[i] != want[i] {
			t.Errorf("cell %d = %v, want %v", i, path[i], want[i])
		}
	}
}

func TestCellLineSameCell(t *testing.T) {
	if path := CellLine(7, 7, 7, 7); len(path) != 1 {
		t.Errorf("CellLine to the same cell = %v, want a single cell", path)
	}
}

// newWalkerAt returns a character standing on the given cell with default
// walk speed and flat terrain.
func newWalkerAt(cellX, cellY int) *Character {
	x, z := CellToWorld(cellX, cellY)
	return NewCharacter(x, 0, z)
}
