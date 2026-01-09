// Package math provides math types and functions for game development.
package math

import "math"

// Vec3 is a 3D vector.
type Vec3 struct {
	X, Y, Z float32
}

// Add returns v + other.
func (v Vec3) Add(other Vec3) Vec3 {
	return Vec3{v.X + other.X, v.Y + other.Y, v.Z + other.Z}
}

// Sub returns v - other.
func (v Vec3) Sub(other Vec3) Vec3 {
	return Vec3{v.X - other.X, v.Y - other.Y, v.Z - other.Z}
}

// Scale returns v * scalar.
func (v Vec3) Scale(s float32) Vec3 {
	return Vec3{v.X * s, v.Y * s, v.Z * s}
}

// Dot returns the dot product.
func (v Vec3) Dot(other Vec3) float32 {
	return v.X*other.X + v.Y*other.Y + v.Z*other.Z
}

// Cross returns the cross product.
func (v Vec3) Cross(other Vec3) Vec3 {
	return Vec3{
		v.Y*other.Z - v.Z*other.Y,
		v.Z*other.X - v.X*other.Z,
		v.X*other.Y - v.Y*other.X,
	}
}

// Length returns the magnitude.
func (v Vec3) Length() float32 {
	return float32(math.Sqrt(float64(v.X*v.X + v.Y*v.Y + v.Z*v.Z)))
}

// Normalize returns a unit vector.
func (v Vec3) Normalize() Vec3 {
	l := v.Length()
	if l == 0 {
		return Vec3{}
	}
	return Vec3{v.X / l, v.Y / l, v.Z / l}
}

// Distance returns the distance to another point.
func (v Vec3) Distance(other Vec3) float32 {
	return v.Sub(other).Length()
}

// XZ returns the XZ components as Vec2.
func (v Vec3) XZ() Vec2 {
	return Vec2{v.X, v.Z}
}
