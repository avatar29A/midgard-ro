package model

import (
	gomath "math"

	"github.com/Faultbox/midgard-ro/pkg/math"
)

// Cross computes the cross product of two 3D vectors.
func Cross(a, b [3]float32) [3]float32 {
	return [3]float32{
		a[1]*b[2] - a[2]*b[1],
		a[2]*b[0] - a[0]*b[2],
		a[0]*b[1] - a[1]*b[0],
	}
}

// Normalize returns a unit vector in the same direction as v.
func Normalize(v [3]float32) [3]float32 {
	length := sqrtf(v[0]*v[0] + v[1]*v[1] + v[2]*v[2])
	if length < 0.0001 {
		return [3]float32{0, 1, 0}
	}
	return [3]float32{v[0] / length, v[1] / length, v[2] / length}
}

// TransformPoint applies a 4x4 matrix transformation to a 3D point.
func TransformPoint(m math.Mat4, p [3]float32) [3]float32 {
	x := m[0]*p[0] + m[4]*p[1] + m[8]*p[2] + m[12]
	y := m[1]*p[0] + m[5]*p[1] + m[9]*p[2] + m[13]
	z := m[2]*p[0] + m[6]*p[1] + m[10]*p[2] + m[14]
	return [3]float32{x, y, z}
}

func sqrtf(x float32) float32 {
	return float32(gomath.Sqrt(float64(x)))
}
