package picking

import "testing"

// TestIntersectGroundFollowsASlope: a flat plane is only right where the
// ground happens to be at the height it was drawn through. On a slope the ray
// meets the surface somewhere else entirely, and the shallower the camera the
// further off it is — which is the pointer drifting from the cell it picks.
func TestIntersectGroundFollowsASlope(t *testing.T) {
	// Ground rising one unit for every unit east.
	slope := func(x, _ float32) float32 { return x }

	// Looking down and east at 45°, from above the origin.
	ray := Ray{
		Origin:    [3]float32{0, 100, 0},
		Direction: [3]float32{0.7071, -0.7071, 0},
	}

	x, _, ok := ray.IntersectGround(slope, 1, 1000)
	if !ok {
		t.Fatal("the ray never met the ground")
	}

	// Height falls from 100 at the same rate the ground rises, so they meet
	// halfway: at x = 50.
	if x < 49 || x > 51 {
		t.Errorf("met the slope at x=%v, want about 50", x)
	}

	// The flat plane through the origin's height would have said something
	// else entirely, which is the bug.
	if flatX, _, _ := ray.IntersectPlaneY(0); flatX < 99 {
		t.Errorf("the flat plane gave x=%v; the test is not showing the difference", flatX)
	}
}

// TestIntersectGroundTakesTheFirstCrossing: a ridge between the eye and the
// place pointed at is what the pointer is on, not the ground beyond it.
func TestIntersectGroundTakesTheFirstCrossing(t *testing.T) {
	// Flat at zero but for a wall between x 20 and 30, tall enough that the
	// ray meets it rather than passing over.
	ground := func(x, _ float32) float32 {
		if x > 20 && x < 30 {
			return 85
		}

		return 0
	}

	ray := Ray{
		Origin:    [3]float32{0, 100, 0},
		Direction: [3]float32{0.7071, -0.7071, 0},
	}

	x, _, ok := ray.IntersectGround(ground, 1, 1000)
	if !ok {
		t.Fatal("the ray never met the ground")
	}

	// The ray is at 100 above the origin and falls a unit for every unit
	// east, so it drops to the wall's height just past x=15 — and the wall
	// begins at 20, so that is where it lands.
	if x < 20 || x > 31 {
		t.Errorf("met the ground at x=%v, want the ridge between 20 and 30", x)
	}
}

// TestIntersectGroundMissesTheSky: a ray that never comes down says so, and
// the caller falls back rather than picking a cell from nothing.
func TestIntersectGroundMissesTheSky(t *testing.T) {
	flat := func(_, _ float32) float32 { return 0 }

	up := Ray{Origin: [3]float32{0, 10, 0}, Direction: [3]float32{0, 1, 0}}
	if _, _, ok := up.IntersectGround(flat, 1, 100); ok {
		t.Error("a ray pointing at the sky met the ground")
	}

	// And one that starts underground, which is not a place the camera goes.
	under := Ray{Origin: [3]float32{0, -10, 0}, Direction: [3]float32{0, -1, 0}}
	if _, _, ok := under.IntersectGround(flat, 1, 100); ok {
		t.Error("a ray starting underground picked something")
	}

	if _, _, ok := up.IntersectGround(nil, 1, 100); ok {
		t.Error("a ray with no ground to meet picked something")
	}
}
