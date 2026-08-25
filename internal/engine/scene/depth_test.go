package scene

import "testing"

// depthQuality is a relative figure of merit for how well coplanar surfaces at
// a given view distance can be separated: perspective depth resolution falls
// off with the square of the distance and improves in proportion to the near
// plane. Lower is better.
func depthQuality(viewDistance float32) float64 {
	near := float64(nearPlaneFor(viewDistance))
	d := float64(viewDistance)
	return d * d / near
}

// TestNearPlaneTracksViewDistance pins the reported symptom. Wall joins at
// Prontera (95,276) flickered at zoom 262 and were clean at zoom 100, with the
// near plane fixed at 1.0. Whatever we do must leave every zoom at least as
// well resolved as zoom 100 used to be.
func TestNearPlaneTracksViewDistance(t *testing.T) {
	// The old behaviour at the zoom that looked correct.
	const fixedNearPlane = 1.0
	baseline := 100.0 * 100.0 / fixedNearPlane

	for _, zoom := range []float32{100, 145, 262, 400, 600, 800} {
		got := depthQuality(zoom)
		if got > baseline {
			t.Errorf("zoom %.0f: depth quality %.0f is worse than the %.0f that "+
				"was already flickering-free at zoom 100", zoom, got, baseline)
		}
	}
}

// TestNearPlaneFixesTheReportedZoom is the specific regression: zoom 262 was
// 6.9x worse than zoom 100 and visibly flickered.
func TestNearPlaneFixesTheReportedZoom(t *testing.T) {
	const fixedNearPlane = 1.0

	oldAt262 := 262.0 * 262.0 / fixedNearPlane
	newAt262 := depthQuality(262)

	improvement := oldAt262 / newAt262
	if improvement < 6.9 {
		t.Errorf("zoom 262 improved only %.1fx; it needs at least the 6.9x it "+
			"had lost relative to the zoom that rendered cleanly", improvement)
	}
}

func TestNearPlaneIsClamped(t *testing.T) {
	if got := nearPlaneFor(0); got != minNearPlane {
		t.Errorf("nearPlaneFor(0) = %v, want the floor %v", got, minNearPlane)
	}
	if got := nearPlaneFor(1); got != minNearPlane {
		t.Errorf("nearPlaneFor(1) = %v, want the floor %v", got, minNearPlane)
	}
	// Far out, the near plane must stop growing or it starts clipping the
	// scene out from under the camera.
	if got := nearPlaneFor(100000); got != maxNearPlane {
		t.Errorf("nearPlaneFor(100000) = %v, want the ceiling %v", got, maxNearPlane)
	}
}

// TestNearPlaneStaysBelowTheSubject: the near plane must never reach the thing
// the camera is looking at, or the subject itself would be clipped away.
func TestNearPlaneStaysBelowTheSubject(t *testing.T) {
	for _, zoom := range []float32{100, 145, 262, 400, 600, 800} {
		if near := nearPlaneFor(zoom); near >= zoom {
			t.Errorf("zoom %.0f: near plane %.1f reaches the subject", zoom, near)
		}
	}
}
