package shadow

import (
	gomath "math"

	"github.com/Faultbox/midgard-ro/pkg/math"
)

// AABB represents an axis-aligned bounding box.
type AABB struct {
	Min [3]float32
	Max [3]float32
}

// Center returns the center point of the AABB.
func (b AABB) Center() math.Vec3 {
	return math.Vec3{
		X: (b.Min[0] + b.Max[0]) / 2,
		Y: (b.Min[1] + b.Max[1]) / 2,
		Z: (b.Min[2] + b.Max[2]) / 2,
	}
}

// Radius returns the distance from center to corner (half-diagonal).
func (b AABB) Radius() float32 {
	dx := (b.Max[0] - b.Min[0]) / 2
	dy := (b.Max[1] - b.Min[1]) / 2
	dz := (b.Max[2] - b.Min[2]) / 2
	return sqrt32(dx*dx + dy*dy + dz*dz)
}

// CalculateDirectionalLightMatrix computes view-projection for shadow map.
// lightDir is the normalized direction TO the light (sun direction).
// sceneBounds is the AABB of the scene to be shadowed.
func CalculateDirectionalLightMatrix(lightDir [3]float32, sceneBounds AABB) math.Mat4 {
	center := sceneBounds.Center()
	radius := sceneBounds.Radius()

	// Position light far enough to encompass entire scene
	lightDistance := radius * 2.0

	// Light position: center + lightDir * distance
	lightPos := math.Vec3{
		X: center.X + lightDir[0]*lightDistance,
		Y: center.Y + lightDir[1]*lightDistance,
		Z: center.Z + lightDir[2]*lightDistance,
	}

	// Choose an appropriate up vector (avoid parallel with light direction)
	up := math.Vec3{X: 0, Y: 1, Z: 0}
	// If light is nearly vertical, use a different up vector
	if abs32(lightDir[1]) > 0.99 {
		up = math.Vec3{X: 0, Y: 0, Z: 1}
	}

	// View matrix: look from light position towards scene center
	view := math.LookAt(lightPos, center, up)

	// Orthographic projection sized to encompass the scene
	// Add padding to avoid edge artifacts
	padding := radius * 0.1
	halfSize := radius + padding
	near := 0.1
	far := lightDistance + radius + padding

	proj := math.Ortho(-halfSize, halfSize, -halfSize, halfSize, float32(near), float32(far))

	return proj.Mul(view)
}

// CalculateTightLightMatrix computes a tighter light matrix based on visible frustum.
// This is more efficient as it only covers what's visible to the camera.
// cameraViewProj is the camera's view-projection matrix.
// lightDir is the normalized direction TO the light.
// sceneBounds provides fallback bounds if frustum is too large.
func CalculateTightLightMatrix(lightDir [3]float32, sceneBounds AABB, cameraPos math.Vec3, cameraDistance float32) math.Mat4 {
	center := sceneBounds.Center()

	// Use camera position to determine shadow focus area
	// This creates a "follow" shadow that moves with the camera
	focusCenter := math.Vec3{
		X: cameraPos.X,
		Y: center.Y, // Keep Y at scene center for terrain
		Z: cameraPos.Z,
	}

	// Shadow radius based on camera distance (closer = tighter shadows)
	shadowRadius := cameraDistance * 1.5
	if shadowRadius < 100 {
		shadowRadius = 100
	}
	if shadowRadius > sceneBounds.Radius() {
		shadowRadius = sceneBounds.Radius()
	}

	// Light distance should be enough to cover scene height
	sceneHeight := sceneBounds.Max[1] - sceneBounds.Min[1]
	lightDistance := shadowRadius + sceneHeight

	// Light position
	lightPos := math.Vec3{
		X: focusCenter.X + lightDir[0]*lightDistance,
		Y: focusCenter.Y + lightDir[1]*lightDistance,
		Z: focusCenter.Z + lightDir[2]*lightDistance,
	}

	up := math.Vec3{X: 0, Y: 1, Z: 0}
	if abs32(lightDir[1]) > 0.99 {
		up = math.Vec3{X: 0, Y: 0, Z: 1}
	}

	view := math.LookAt(lightPos, focusCenter, up)

	// Tighter orthographic projection
	padding := shadowRadius * 0.1
	halfSize := shadowRadius + padding
	near := float32(0.1)
	far := lightDistance + sceneHeight + padding

	proj := math.Ortho(-halfSize, halfSize, -halfSize, halfSize, near, far)

	return proj.Mul(view)
}

// sqrt32 returns the square root of a float32.
func sqrt32(x float32) float32 {
	return float32(gomath.Sqrt(float64(x)))
}

// abs32 returns the absolute value of a float32.
func abs32(x float32) float32 {
	if x < 0 {
		return -x
	}
	return x
}
