package scene

import (
	gomath "math"

	"github.com/Faultbox/midgard-ro/pkg/math"
)

// Plane is a plane in the form ax + by + cz + d = 0, oriented so that points
// on the inside of the frustum evaluate positive.
type Plane struct {
	A, B, C, D float32
}

// SignedDistance returns how far a point lies from the plane, positive on the
// inside. It is a true distance only while the plane is normalized, which is
// what FrustumFromViewProj guarantees and what lets the result be compared
// against a radius in world units.
func (p Plane) SignedDistance(x, y, z float32) float32 {
	return p.A*x + p.B*y + p.C*z + p.D
}

// Frustum is the six planes bounding what the camera can see: left, right,
// bottom, top, near, far.
type Frustum [6]Plane

// FrustumFromViewProj extracts the six clip planes from a combined
// view-projection matrix, by the Gribb & Hartmann method: a point is inside
// the frustum when its clip-space coordinates satisfy -w <= x,y,z <= w, and
// each of those six inequalities rearranges into a plane whose coefficients
// are a sum or difference of two matrix rows.
//
// Mat4 is column-major, so row i is (m[i], m[i+4], m[i+8], m[i+12]).
func FrustumFromViewProj(m math.Mat4) Frustum {
	row := func(i int) [4]float32 {
		return [4]float32{m[i], m[i+4], m[i+8], m[i+12]}
	}
	rx, ry, rz, rw := row(0), row(1), row(2), row(3)

	return Frustum{
		normalizePlane(planeFromSum(rw, rx)),  // left:   x >= -w
		normalizePlane(planeFromDiff(rw, rx)), // right:  x <=  w
		normalizePlane(planeFromSum(rw, ry)),  // bottom: y >= -w
		normalizePlane(planeFromDiff(rw, ry)), // top:    y <=  w
		normalizePlane(planeFromSum(rw, rz)),  // near:   z >= -w
		normalizePlane(planeFromDiff(rw, rz)), // far:    z <=  w
	}
}

// ContainsSphere reports whether any part of the sphere lies inside the
// frustum.
//
// This is the cheap conservative test: a sphere that straddles two planes near
// a corner can be reported visible while nothing of it is actually in view.
// That costs an occasional wasted draw and never a missing one, which is the
// only direction of error that is safe here.
func (f Frustum) ContainsSphere(center [3]float32, radius float32) bool {
	for _, p := range f {
		if p.SignedDistance(center[0], center[1], center[2]) < -radius {
			return false
		}
	}
	return true
}

func planeFromSum(a, b [4]float32) Plane {
	return Plane{a[0] + b[0], a[1] + b[1], a[2] + b[2], a[3] + b[3]}
}

func planeFromDiff(a, b [4]float32) Plane {
	return Plane{a[0] - b[0], a[1] - b[1], a[2] - b[2], a[3] - b[3]}
}

// normalizePlane scales a plane so its normal is unit length, which is what
// makes SignedDistance comparable to a bounding radius. A degenerate plane is
// returned unchanged; it can only come from a degenerate matrix, and leaving
// it alone keeps the test conservative rather than dividing by zero.
func normalizePlane(p Plane) Plane {
	length := float32(gomath.Sqrt(float64(p.A*p.A + p.B*p.B + p.C*p.C)))
	if length == 0 {
		return p
	}
	return Plane{p.A / length, p.B / length, p.C / length, p.D / length}
}
