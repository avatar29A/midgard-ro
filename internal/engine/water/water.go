// Package water provides water geometry and animation utilities.
package water

import "github.com/Faultbox/midgard-ro/pkg/formats"

// Mesh is water geometry ready for upload: one quad per cell that has water.
type Mesh struct {
	Vertices []float32 // x, y, z per vertex, six per cell
	Cells    int
	Level    float32
}

// BuildCells builds water for the cells that have it, by the original
// client's rule (roBrowser Ground.js:471-483): a cell gets water when it has
// ground — a top surface — and any corner of that ground lies below the
// water surface, allowing for the wave height. A cell with no ground gets
// none, which is what keeps the void between an indoor map's rooms black
// instead of flooding it.
//
// RO altitudes are positive downwards and the water level is given the same
// way, so "below the surface" is a corner altitude greater than the level.
func BuildCells(gnd *formats.GND, level, waveHeight float32) *Mesh {
	m := &Mesh{Level: level}
	if gnd == nil {
		return m
	}

	y := -level
	threshold := level - waveHeight
	width, height := int(gnd.Width), int(gnd.Height)
	for ty := 0; ty < height; ty++ {
		for tx := 0; tx < width; tx++ {
			tile := gnd.GetTile(tx, ty)
			if tile == nil || tile.TopSurface < 0 {
				continue
			}
			under := false
			for _, a := range tile.Altitude {
				if a > threshold {
					under = true
					break
				}
			}
			if !under {
				continue
			}

			x0 := float32(tx) * gnd.Zoom
			z0 := float32(ty) * gnd.Zoom
			x1 := x0 + gnd.Zoom
			z1 := z0 + gnd.Zoom
			m.Vertices = append(m.Vertices,
				x0, y, z0,
				x1, y, z0,
				x1, y, z1,
				x0, y, z0,
				x1, y, z1,
				x0, y, z1,
			)
			m.Cells++
		}
	}
	return m
}

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
