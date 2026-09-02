// Package picking provides ray casting and object picking utilities.
package picking

import (
	gomath "math"

	"github.com/Faultbox/midgard-ro/pkg/math"
)

// Ray represents a ray in 3D space with origin and direction.
type Ray struct {
	Origin    [3]float32
	Direction [3]float32 // Normalized direction
}

// AABB represents an axis-aligned bounding box.
type AABB struct {
	Min [3]float32
	Max [3]float32
}

// ScreenToRay converts screen coordinates to a world-space ray.
// screenX, screenY are pixel coordinates, viewportW/H are viewport dimensions.
// invViewProj is the inverse of the view-projection matrix.
func ScreenToRay(screenX, screenY, viewportW, viewportH float32, invViewProj math.Mat4) Ray {
	// Convert screen coords to normalized device coords (-1 to 1)
	ndcX := (2.0*screenX/viewportW - 1.0)
	ndcY := (1.0 - 2.0*screenY/viewportH) // Flip Y

	// Unproject near and far points
	nearPoint := math.Vec4{ndcX, ndcY, -1.0, 1.0}
	farPoint := math.Vec4{ndcX, ndcY, 1.0, 1.0}

	nearWorld := invViewProj.MulVec4(nearPoint)
	farWorld := invViewProj.MulVec4(farPoint)

	// Perspective divide
	if nearWorld[3] != 0 {
		nearWorld[0] /= nearWorld[3]
		nearWorld[1] /= nearWorld[3]
		nearWorld[2] /= nearWorld[3]
	}
	if farWorld[3] != 0 {
		farWorld[0] /= farWorld[3]
		farWorld[1] /= farWorld[3]
		farWorld[2] /= farWorld[3]
	}

	origin := [3]float32{nearWorld[0], nearWorld[1], nearWorld[2]}
	dir := [3]float32{
		farWorld[0] - nearWorld[0],
		farWorld[1] - nearWorld[1],
		farWorld[2] - nearWorld[2],
	}

	// Normalize direction
	rayLen := float32(gomath.Sqrt(float64(dir[0]*dir[0] + dir[1]*dir[1] + dir[2]*dir[2])))
	if rayLen > 0 {
		dir[0] /= rayLen
		dir[1] /= rayLen
		dir[2] /= rayLen
	}

	return Ray{Origin: origin, Direction: dir}
}

// IntersectPlaneY intersects a ray with a horizontal plane at the given Y level.
// Returns the intersection point (X, Z) and whether the intersection is valid.
func (r Ray) IntersectPlaneY(planeY float32) (x, z float32, ok bool) {
	// Ray: P = Origin + t * Direction
	// Plane: Y = planeY
	// Solve: Origin.Y + t * Direction.Y = planeY
	if gomath.Abs(float64(r.Direction[1])) < 0.001 {
		return 0, 0, false // Ray parallel to plane
	}

	t := (planeY - r.Origin[1]) / r.Direction[1]
	if t < 0 {
		return 0, 0, false // Intersection behind ray origin
	}

	x = r.Origin[0] + t*r.Direction[0]
	z = r.Origin[2] + t*r.Direction[2]
	return x, z, true
}

// IntersectGround walks a ray until it goes below the ground and reports where
// it crossed.
//
// A single flat plane will not do. The ground is not flat: a ramp, a stair or
// a hillside all put the surface above or below whatever height the plane was
// chosen at, and a ray that misses by a little vertically misses by a lot
// horizontally once the camera is anything but overhead. That is what makes
// the pointer and the cell it picks drift apart as the camera is lowered — and
// the further from the chosen height you point, the wider the gap.
//
// So: step along the ray until it is under the ground, then halve the last
// step until the crossing is pinned. Stepping finds the first crossing rather
// than the nearest, which is what picking wants — a ridge between you and the
// place you clicked should be what the click lands on.
//
// The step is in world units; anything up to about half a cell is fine, since
// the bisection does the accuracy and the step only has to not step over a
// feature whole.
func (r Ray) IntersectGround(height func(x, z float32) float32, step, maxDistance float32) (x, z float32, ok bool) {
	if height == nil || step <= 0 || maxDistance <= 0 {
		return 0, 0, false
	}

	above := func(t float32) (float32, float32, bool) {
		px := r.Origin[0] + t*r.Direction[0]
		py := r.Origin[1] + t*r.Direction[1]
		pz := r.Origin[2] + t*r.Direction[2]

		return px, pz, py >= height(px, pz)
	}

	// Starting underground means the camera is inside the terrain, which is
	// not a state the map puts it in; nothing sensible can be picked from
	// there.
	if _, _, up := above(0); !up {
		return 0, 0, false
	}

	last := float32(0)

	for t := step; t <= maxDistance; t += step {
		if _, _, up := above(t); up {
			last = t

			continue
		}

		// Crossed between last and t: halve until the two are close enough
		// that the cell no longer changes.
		lo, hi := last, t
		for i := 0; i < groundBisections; i++ {
			mid := (lo + hi) / 2
			if _, _, up := above(mid); up {
				lo = mid
			} else {
				hi = mid
			}
		}

		x, z, _ = above((lo + hi) / 2)

		return x, z, true
	}

	return 0, 0, false
}

// groundBisections is how many times the crossing is halved. Ten takes a step
// of a few world units down to thousandths of one, far finer than the cell the
// answer is rounded to.
const groundBisections = 10

// IntersectAABB tests ray intersection with an axis-aligned bounding box.
// Returns the distance to intersection (t) and whether intersection occurred.
// If the ray starts inside the box, returns the exit distance.
func (r Ray) IntersectAABB(box AABB) (t float32, hit bool) {
	tmin := float32(-gomath.MaxFloat32)
	tmax := float32(gomath.MaxFloat32)

	// X slab
	if r.Direction[0] != 0 {
		t1 := (box.Min[0] - r.Origin[0]) / r.Direction[0]
		t2 := (box.Max[0] - r.Origin[0]) / r.Direction[0]
		if t1 > t2 {
			t1, t2 = t2, t1
		}
		if t1 > tmin {
			tmin = t1
		}
		if t2 < tmax {
			tmax = t2
		}
	} else if r.Origin[0] < box.Min[0] || r.Origin[0] > box.Max[0] {
		return 0, false
	}

	// Y slab
	if r.Direction[1] != 0 {
		t1 := (box.Min[1] - r.Origin[1]) / r.Direction[1]
		t2 := (box.Max[1] - r.Origin[1]) / r.Direction[1]
		if t1 > t2 {
			t1, t2 = t2, t1
		}
		if t1 > tmin {
			tmin = t1
		}
		if t2 < tmax {
			tmax = t2
		}
	} else if r.Origin[1] < box.Min[1] || r.Origin[1] > box.Max[1] {
		return 0, false
	}

	// Z slab
	if r.Direction[2] != 0 {
		t1 := (box.Min[2] - r.Origin[2]) / r.Direction[2]
		t2 := (box.Max[2] - r.Origin[2]) / r.Direction[2]
		if t1 > t2 {
			t1, t2 = t2, t1
		}
		if t1 > tmin {
			tmin = t1
		}
		if t2 < tmax {
			tmax = t2
		}
	} else if r.Origin[2] < box.Min[2] || r.Origin[2] > box.Max[2] {
		return 0, false
	}

	// Check if intersection is valid
	if tmax < tmin || tmax < 0 {
		return 0, false
	}

	// Return entry point, or exit point if starting inside
	if tmin < 0 {
		return tmax, true
	}
	return tmin, true
}

// NewAABB creates an AABB from min and max corners, handling negative scales.
func NewAABB(minX, minY, minZ, maxX, maxY, maxZ float32) AABB {
	box := AABB{
		Min: [3]float32{minX, minY, minZ},
		Max: [3]float32{maxX, maxY, maxZ},
	}
	// Ensure min < max for each axis
	if box.Min[0] > box.Max[0] {
		box.Min[0], box.Max[0] = box.Max[0], box.Min[0]
	}
	if box.Min[1] > box.Max[1] {
		box.Min[1], box.Max[1] = box.Max[1], box.Min[1]
	}
	if box.Min[2] > box.Max[2] {
		box.Min[2], box.Max[2] = box.Max[2], box.Min[2]
	}
	return box
}

// TransformAABB transforms a local AABB by position and scale to world space.
func TransformAABB(localBbox [6]float32, position, scale [3]float32) AABB {
	return NewAABB(
		localBbox[0]*scale[0]+position[0],
		localBbox[1]*scale[1]+position[1],
		localBbox[2]*scale[2]+position[2],
		localBbox[3]*scale[0]+position[0],
		localBbox[4]*scale[1]+position[1],
		localBbox[5]*scale[2]+position[2],
	)
}
