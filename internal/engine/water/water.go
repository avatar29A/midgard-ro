// Package water provides water plane geometry and animation utilities.
package water

// Vertex represents a water surface vertex (position only).
type Vertex struct {
	Position [3]float32
}

// Plane holds water plane geometry ready for GPU upload.
type Plane struct {
	Vertices []float32 // Flat array: x,y,z for each vertex (4 vertices)
	Level    float32   // Water Y level in world coordinates
}

// BuildPlane creates water plane vertices covering the specified bounds.
// The waterLevel is negated for RO's Y-up coordinate system.
func BuildPlane(minX, maxX, minZ, maxZ, waterLevel float32) *Plane {
	// Water level in RSW is typically positive for below ground level
	// Convert to Y-up coordinate system
	waterY := -waterLevel

	// Simple quad vertices (position only)
	// Order: BL, BR, TR, TL for TRIANGLE_FAN rendering
	vertices := []float32{
		minX, waterY, minZ,
		maxX, waterY, minZ,
		maxX, waterY, maxZ,
		minX, waterY, maxZ,
	}

	return &Plane{
		Vertices: vertices,
		Level:    waterLevel,
	}
}

// BuildPlaneWithPadding creates water plane vertices with padding around the bounds.
func BuildPlaneWithPadding(minX, maxX, minZ, maxZ, waterLevel, padding float32) *Plane {
	return BuildPlane(
		minX-padding,
		maxX+padding,
		minZ-padding,
		maxZ+padding,
		waterLevel,
	)
}

// CalculateAnimFrame returns the current animation frame index for water texture animation.
// time is the elapsed time, speed is the animation speed multiplier, numFrames is total frames.
func CalculateAnimFrame(time, speed float32, numFrames int) int {
	if numFrames <= 0 {
		return 0
	}
	// At speed 10, cycle through frames in ~3 seconds
	frameTime := time * speed * 0.5
	return int(frameTime) % numFrames
}

// DefaultAnimSpeed is the default water animation speed if not specified in RSW.
const DefaultAnimSpeed = 30.0

// DefaultPadding is the default padding to extend water plane beyond map bounds.
const DefaultPadding = 50.0
