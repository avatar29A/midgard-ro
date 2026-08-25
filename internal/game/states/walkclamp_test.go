package states

import "testing"

// TestClampWalkRequest covers the fix for clicks past the server's reach doing
// nothing at all. rAthena answers a walk it cannot path with no packet, so an
// over-long request is indistinguishable from the click being ignored.
func TestClampWalkRequest(t *testing.T) {
	tests := []struct {
		name          string
		fromX, fromY  int
		toX, toY      int
		wantX, wantY  int
		wantUnchanged bool
	}{
		{
			name:  "within reach is left alone",
			fromX: 100, fromY: 100, toX: 105, toY: 103,
			wantX: 105, wantY: 103, wantUnchanged: true,
		},
		{
			name:  "exactly at the limit is left alone",
			fromX: 100, fromY: 100,
			toX: 100 + MaxWalkRequestCells, toY: 100,
			wantX: 100 + MaxWalkRequestCells, wantY: 100, wantUnchanged: true,
		},
		{
			name:  "straight east is pulled back to the limit",
			fromX: 100, fromY: 100, toX: 200, toY: 100,
			wantX: 100 + MaxWalkRequestCells, wantY: 100,
		},
		{
			name:  "straight south is pulled back to the limit",
			fromX: 100, fromY: 100, toX: 100, toY: 40,
			wantX: 100, wantY: 100 - MaxWalkRequestCells,
		},
		{
			name:  "diagonal keeps its direction",
			fromX: 100, fromY: 100, toX: 200, toY: 200,
			wantX: 100 + MaxWalkRequestCells, wantY: 100 + MaxWalkRequestCells,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotX, gotY := clampWalkRequest(tt.fromX, tt.fromY, tt.toX, tt.toY)
			if gotX != tt.wantX || gotY != tt.wantY {
				t.Errorf("clampWalkRequest = (%d,%d), want (%d,%d)", gotX, gotY, tt.wantX, tt.wantY)
			}
			if tt.wantUnchanged && (gotX != tt.toX || gotY != tt.toY) {
				t.Error("a reachable destination should not be clamped")
			}
		})
	}
}

// TestClampWalkRequestAlwaysWithinReach is the property that matters: whatever
// is asked for, what goes on the wire must be something the server will path.
func TestClampWalkRequestAlwaysWithinReach(t *testing.T) {
	const origin = 150

	for dx := -80; dx <= 80; dx += 7 {
		for dy := -80; dy <= 80; dy += 7 {
			gotX, gotY := clampWalkRequest(origin, origin, origin+dx, origin+dy)

			reach := abs(gotX - origin)
			if d := abs(gotY - origin); d > reach {
				reach = d
			}
			if reach > MaxWalkRequestCells {
				t.Fatalf("request to (%+d,%+d) clamped to %d cells, over the %d limit",
					dx, dy, reach, MaxWalkRequestCells)
			}
		}
	}
}

// TestClampWalkRequestPreservesDirection checks the clamped point still lies
// towards the click: walking off at the wrong angle would be worse than not
// moving.
func TestClampWalkRequestPreservesDirection(t *testing.T) {
	const origin = 150

	for dx := -60; dx <= 60; dx += 5 {
		for dy := -60; dy <= 60; dy += 5 {
			if dx == 0 && dy == 0 {
				continue
			}
			gotX, gotY := clampWalkRequest(origin, origin, origin+dx, origin+dy)
			cx, cy := gotX-origin, gotY-origin

			if dx > 0 && cx < 0 || dx < 0 && cx > 0 {
				t.Errorf("(%+d,%+d) clamped to (%+d,%+d): X direction flipped", dx, dy, cx, cy)
			}
			if dy > 0 && cy < 0 || dy < 0 && cy > 0 {
				t.Errorf("(%+d,%+d) clamped to (%+d,%+d): Y direction flipped", dx, dy, cx, cy)
			}
		}
	}
}

// TestClampStaysUnderMeasuredServerLimit guards the constant itself. Measured
// against rAthena: 17 cells acknowledged, 18 dropped.
func TestClampStaysUnderMeasuredServerLimit(t *testing.T) {
	const measuredServerLimit = 17
	if MaxWalkRequestCells >= measuredServerLimit {
		t.Errorf("MaxWalkRequestCells is %d, which is not under the measured "+
			"server limit of %d — and paths around obstacles are longer than "+
			"the straight-line distance, so headroom is needed",
			MaxWalkRequestCells, measuredServerLimit)
	}
}
