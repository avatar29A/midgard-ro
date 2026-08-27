package scene

import "testing"

// TestMarkerScaleSwellsAndSettles: the flourish has to read as a response to
// the click, not a resize. It starts and ends at the marker's resting size and
// is largest in between — a scale that only grew would leave the marker bigger
// than it started.
func TestMarkerScaleSwellsAndSettles(t *testing.T) {
	if got := MarkerScale(0); got != 1 {
		t.Errorf("scale at the start = %v, want 1", got)
	}
	if got := MarkerScale(1); got != 1 {
		t.Errorf("scale at the end = %v, want 1", got)
	}

	peak := MarkerScale(0.5)
	if peak <= 1 {
		t.Errorf("scale at the peak = %v, want larger than resting", peak)
	}

	// Rising through the first half, falling through the second.
	if !(MarkerScale(0.25) > MarkerScale(0.05)) {
		t.Error("the flourish is not growing through its first half")
	}
	if !(MarkerScale(0.75) < peak) {
		t.Error("the flourish is not settling through its second half")
	}
}

// TestMarkerScaleIsRestingOutsideThePulse: a marker that is merely following
// the cursor draws at its resting size, and progress outside 0..1 is what a
// spent or unstarted flourish looks like.
func TestMarkerScaleIsRestingOutsideThePulse(t *testing.T) {
	for _, progress := range []float32{-1, -0.01, 1.01, 5} {
		if got := MarkerScale(progress); got != 1 {
			t.Errorf("MarkerScale(%v) = %v, want the resting size 1", progress, got)
		}
	}
}

// TestMarkerScaleIsSymmetric: the swell and the settle should mirror each
// other, so the flourish does not snap back faster than it grew.
func TestMarkerScaleIsSymmetric(t *testing.T) {
	for _, p := range []float32{0.1, 0.25, 0.4} {
		a, b := MarkerScale(p), MarkerScale(1-p)
		if diff := a - b; diff > 1e-6 || diff < -1e-6 {
			t.Errorf("MarkerScale(%v)=%v and MarkerScale(%v)=%v are not mirrored", p, a, 1-p, b)
		}
	}
}
