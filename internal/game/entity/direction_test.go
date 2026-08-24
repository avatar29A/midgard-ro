package entity

import "testing"

// TestCalculateDirectionMatchesWorldAxes pins the world convention: +X is
// east, +Z is north. The table used to start at DirS, which pointed every
// facing 180 degrees the wrong way and forced callers to negate their deltas.
func TestCalculateDirectionMatchesWorldAxes(t *testing.T) {
	tests := []struct {
		name   string
		dx, dz float32
		want   int
	}{
		{"north is +Z", 0, 1, DirN},
		{"south is -Z", 0, -1, DirS},
		{"east is +X", 1, 0, DirE},
		{"west is -X", -1, 0, DirW},
		{"northeast", 1, 1, DirNE},
		{"northwest", -1, 1, DirNW},
		{"southeast", 1, -1, DirSE},
		{"southwest", -1, -1, DirSW},
		// Off-axis vectors must land in the nearest 45-degree sector.
		{"mostly north", 0.2, 1, DirN},
		{"mostly east", 1, 0.2, DirE},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CalculateDirection(tt.dx, tt.dz); got != tt.want {
				t.Errorf("CalculateDirection(%v,%v) = %d, want %d", tt.dx, tt.dz, got, tt.want)
			}
		})
	}
}

// TestCalculateDirectionAgreesWithCellDelta guards against the two direction
// tables in this package drifting apart — walking a path and moving freely
// must produce the same facing for the same heading.
func TestCalculateDirectionAgreesWithCellDelta(t *testing.T) {
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			if dx == 0 && dy == 0 {
				continue
			}
			fromCells := DirectionFromCellDelta(dx, dy)
			fromVector := CalculateDirection(float32(dx), float32(dy))
			if fromCells != fromVector {
				t.Errorf("delta (%d,%d): cell table says %d, vector math says %d",
					dx, dy, fromCells, fromVector)
			}
		}
	}
}

// TestDirectionFromServer covers the conversion between rAthena's direction
// enum and RO's sprite indices. The two run opposite ways around the compass,
// so assigning one to the other directly only happens to be right for east and
// west — which is exactly how the bug survived.
//
// Server values are from rAthena src/map/path.hpp `enum directions`.
func TestDirectionFromServer(t *testing.T) {
	tests := []struct {
		name      string
		serverDir uint8
		want      int
	}{
		{"DIR_NORTH", 0, DirN},
		{"DIR_NORTHWEST", 1, DirNW},
		{"DIR_WEST", 2, DirW},
		{"DIR_SOUTHWEST", 3, DirSW},
		{"DIR_SOUTH", 4, DirS},
		{"DIR_SOUTHEAST", 5, DirSE},
		{"DIR_EAST", 6, DirE},
		{"DIR_NORTHEAST", 7, DirNE},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DirectionFromServer(tt.serverDir); got != tt.want {
				t.Errorf("DirectionFromServer(%d) = %d, want %d", tt.serverDir, got, tt.want)
			}
		})
	}
}

func TestDirectionFromServerOutOfRange(t *testing.T) {
	if got := DirectionFromServer(200); got != DirS {
		t.Errorf("out-of-range server direction = %d, want DirS(%d)", got, DirS)
	}
}

// TestServerAndSpriteEnumsDisagree documents *why* the conversion is needed:
// only west and east happen to share an index between the two schemes.
func TestServerAndSpriteEnumsDisagree(t *testing.T) {
	same := 0
	for d := uint8(0); d < 8; d++ {
		if DirectionFromServer(d) == int(d) {
			same++
		}
	}
	if same != 2 {
		t.Errorf("%d directions map to themselves, want exactly 2 (west and east)", same)
	}
}

func TestCellDeltaForDirectionRoundTrip(t *testing.T) {
	for _, dir := range []int{DirS, DirSW, DirW, DirNW, DirN, DirNE, DirE, DirSE} {
		dx, dy := CellDeltaForDirection(dir)
		if dx == 0 && dy == 0 {
			t.Errorf("direction %d produced no step", dir)
			continue
		}
		if got := DirectionFromCellDelta(dx, dy); got != dir {
			t.Errorf("direction %d -> delta (%d,%d) -> direction %d", dir, dx, dy, got)
		}
	}
}
