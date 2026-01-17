// Package lighting provides lighting utilities for 3D rendering.
package lighting

import "math"

// SunDirection converts RSW longitude/latitude angles to a light direction vector.
// Longitude is rotation around Y axis (0-360), latitude is elevation from horizon (0-90).
// Returns a normalized direction vector pointing towards the sun.
func SunDirection(longitude, latitude int32) [3]float32 {
	// Convert degrees to radians
	lonRad := float64(longitude) * math.Pi / 180.0
	latRad := float64(latitude) * math.Pi / 180.0

	// Spherical to Cartesian conversion
	// Longitude is around Y axis, latitude is elevation from horizon
	x := float32(math.Cos(latRad) * math.Sin(lonRad))
	y := float32(math.Sin(latRad))
	z := float32(math.Cos(latRad) * math.Cos(lonRad))

	return [3]float32{x, y, z}
}

// SunDirectionF32 is like SunDirection but accepts float32 angles.
func SunDirectionF32(longitude, latitude float32) [3]float32 {
	return SunDirection(int32(longitude), int32(latitude))
}
