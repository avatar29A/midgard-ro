// Utility functions for map viewer.
package main

import (
	gomath "math"

	"github.com/Faultbox/midgard-ro/pkg/math"
)

// cross computes the cross product of two 3D vectors.
func cross(a, b [3]float32) [3]float32 {
	return [3]float32{
		a[1]*b[2] - a[2]*b[1],
		a[2]*b[0] - a[0]*b[2],
		a[0]*b[1] - a[1]*b[0],
	}
}

// normalize returns a unit vector in the same direction as v.
func normalize(v [3]float32) [3]float32 {
	len := sqrtf(v[0]*v[0] + v[1]*v[1] + v[2]*v[2])
	if len < 0.0001 {
		return [3]float32{0, 1, 0}
	}
	return [3]float32{v[0] / len, v[1] / len, v[2] / len}
}

// sqrtf computes the square root of a float32.
func sqrtf(x float32) float32 {
	return float32(gomath.Sqrt(float64(x)))
}

// cosf computes the cosine of a float32 angle.
func cosf(x float32) float64 {
	return gomath.Cos(float64(x))
}

// sinf computes the sine of a float32 angle.
func sinf(x float32) float64 {
	return gomath.Sin(float64(x))
}

// absf returns the absolute value of a float32.
func absf(x float32) float32 {
	if x < 0 {
		return -x
	}
	return x
}

// calculateSunDirection converts RSW longitude/latitude to a light direction vector.
func calculateSunDirection(longitude, latitude int32) [3]float32 {
	// Convert degrees to radians
	lonRad := float64(longitude) * gomath.Pi / 180.0
	latRad := float64(latitude) * gomath.Pi / 180.0

	// Spherical to Cartesian conversion
	// Longitude is around Y axis, latitude is elevation from horizon
	x := float32(gomath.Cos(latRad) * gomath.Sin(lonRad))
	y := float32(gomath.Sin(latRad))
	z := float32(gomath.Cos(latRad) * gomath.Cos(lonRad))

	return [3]float32{x, y, z}
}

// transformPoint applies a 4x4 matrix transformation to a 3D point.
func transformPoint(m math.Mat4, p [3]float32) [3]float32 {
	x := m[0]*p[0] + m[4]*p[1] + m[8]*p[2] + m[12]
	y := m[1]*p[0] + m[5]*p[1] + m[9]*p[2] + m[13]
	z := m[2]*p[0] + m[6]*p[1] + m[10]*p[2] + m[14]
	return [3]float32{x, y, z}
}
