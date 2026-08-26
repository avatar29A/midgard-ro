package scene

import (
	gomath "math"
	"testing"

	"github.com/Faultbox/midgard-ro/pkg/math"
)

// testViewProj returns the view-projection of a camera at (0, 0, 100) looking
// back at the origin down -Z, with a 45-degree vertical field of view. Chosen
// to resemble the in-game camera, which sits a fixed distance from the player
// and looks inward.
func testViewProj() math.Mat4 {
	proj := math.Perspective(45*gomath.Pi/180, 16.0/9.0, 1, 1000)
	view := math.LookAt(
		math.Vec3{X: 0, Y: 0, Z: 100},
		math.Vec3{X: 0, Y: 0, Z: 0},
		math.Vec3{X: 0, Y: 1, Z: 0},
	)
	return proj.Mul(view)
}

// TestFrustumPlanesAreNormalized guards the property the whole test depends
// on: SignedDistance is only comparable to a bounding radius while the plane
// normals are unit length. An unnormalized plane still gets the sign right, so
// this would otherwise fail silently as models culled at the wrong distance.
func TestFrustumPlanesAreNormalized(t *testing.T) {
	f := FrustumFromViewProj(testViewProj())

	for i, p := range f {
		length := gomath.Sqrt(float64(p.A*p.A + p.B*p.B + p.C*p.C))
		if gomath.Abs(length-1) > 1e-5 {
			t.Errorf("plane %d normal length = %v, want 1", i, length)
		}
	}
}

func TestFrustumKeepsWhatIsInView(t *testing.T) {
	f := FrustumFromViewProj(testViewProj())

	tests := []struct {
		name   string
		center [3]float32
		radius float32
	}{
		{"at the focal point", [3]float32{0, 0, 0}, 10},
		{"just inside the near plane", [3]float32{0, 0, 98}, 1},
		{"off to one side but still framed", [3]float32{10, 0, 0}, 5},
		{"a point-sized object dead center", [3]float32{0, 0, 0}, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !f.ContainsSphere(tt.center, tt.radius) {
				t.Error("culled something in view; the map would have holes in it")
			}
		})
	}
}

func TestFrustumRejectsWhatIsNotInView(t *testing.T) {
	f := FrustumFromViewProj(testViewProj())

	tests := []struct {
		name   string
		center [3]float32
		radius float32
	}{
		{"behind the camera", [3]float32{0, 0, 300}, 10},
		{"far past the far plane", [3]float32{0, 0, -2000}, 10},
		{"way off to the left", [3]float32{-5000, 0, 0}, 10},
		{"way off to the right", [3]float32{5000, 0, 0}, 10},
		{"high above the top", [3]float32{0, 5000, 0}, 10},
		{"far below the bottom", [3]float32{0, -5000, 0}, 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if f.ContainsSphere(tt.center, tt.radius) {
				t.Error("kept something out of view; culling is not saving anything")
			}
		})
	}
}

// TestFrustumKeepsStraddlingSphere is the case a naive center-only test gets
// wrong: the center is outside the frustum but part of the object is not.
// Getting this wrong pops large models out of view at the screen edge.
func TestFrustumKeepsStraddlingSphere(t *testing.T) {
	f := FrustumFromViewProj(testViewProj())

	// At the focal plane the frustum reaches about 74 units to each side, so
	// this center sits roughly 62 units outside the left plane.
	center := [3]float32{-150, 0, 0}
	if f.ContainsSphere(center, 1) {
		t.Fatal("test setup is wrong: the center should be outside the frustum")
	}

	// The same center, on an object big enough to reach back into view.
	if !f.ContainsSphere(center, 100) {
		t.Error("culled a model whose center is outside but whose body is visible")
	}
}

// TestFrustumRadiusIsInWorldUnits pins the scale of the radius comparison. A
// sphere just short of reaching the frustum must be culled and one just past
// it kept, which only holds if the plane distances are in world units.
func TestFrustumRadiusIsInWorldUnits(t *testing.T) {
	f := FrustumFromViewProj(testViewProj())

	center := [3]float32{0, 0, 300} // 200 behind the camera, 201 behind the near plane

	if f.ContainsSphere(center, 150) {
		t.Error("a sphere that does not reach the near plane was kept")
	}
	if !f.ContainsSphere(center, 250) {
		t.Error("a sphere that reaches well past the near plane was culled")
	}
}
