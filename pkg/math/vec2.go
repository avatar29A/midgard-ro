// Package math provides math types and functions for game development.
package math

import "math"

// Vec2 is a 2D vector.
type Vec2 struct {
	X, Y float32
}

// Add returns v + other.
func (v Vec2) Add(other Vec2) Vec2 {
	return Vec2{v.X + other.X, v.Y + other.Y}
}

// Sub returns v - other.
func (v Vec2) Sub(other Vec2) Vec2 {
	return Vec2{v.X - other.X, v.Y - other.Y}
}

// Scale returns v * scalar.
func (v Vec2) Scale(s float32) Vec2 {
	return Vec2{v.X * s, v.Y * s}
}

// Dot returns the dot product.
func (v Vec2) Dot(other Vec2) float32 {
	return v.X*other.X + v.Y*other.Y
}

// Length returns the magnitude.
func (v Vec2) Length() float32 {
	return float32(math.Sqrt(float64(v.X*v.X + v.Y*v.Y)))
}

// Normalize returns a unit vector.
func (v Vec2) Normalize() Vec2 {
	l := v.Length()
	if l == 0 {
		return Vec2{}
	}
	return Vec2{v.X / l, v.Y / l}
}

// Distance returns the distance to another point.
func (v Vec2) Distance(other Vec2) float32 {
	return v.Sub(other).Length()
}
