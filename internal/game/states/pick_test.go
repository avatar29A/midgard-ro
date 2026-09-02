package states

import (
	"testing"

	"github.com/Faultbox/midgard-ro/internal/engine/picking"
	"github.com/Faultbox/midgard-ro/internal/game/entity"
)

// TestGroundPickStepCannotStrideOverACell: the ray is sampled at intervals and
// the crossing pinned by bisection afterwards, so the step only has to be fine
// enough not to step over a feature whole. A cell is the smallest thing the
// ground can change over, so a step wider than half of one could pass clean
// through a stair tread and land the pointer on whatever is behind it.
func TestGroundPickStepCannotStrideOverACell(t *testing.T) {
	if groundPickStep > entity.CellSize/2 {
		t.Errorf("the ground is sampled every %v units, which is more than half a %v-unit cell",
			groundPickStep, entity.CellSize)
	}
	if groundPickStep <= 0 {
		t.Errorf("a step of %v samples nothing", groundPickStep)
	}
}

// TestGroundPickReachCrossesATown: the reach has to carry from the camera to
// whatever is being pointed at. A low camera looking towards the horizon runs
// a long way before it meets the ground, and a ray that gives up short falls
// back to the flat plane — which is the bug this replaced.
func TestGroundPickReachCrossesATown(t *testing.T) {
	// Geffen is 200 cells across, and the diagonal of one is the furthest a
	// ray can have to travel over it.
	const townCells = 200

	if want := float32(townCells) * entity.CellSize * 1.42; groundPickReach < want {
		t.Errorf("the ray reaches %v units, short of the %v across a town's diagonal",
			groundPickReach, want)
	}
}

// TestGroundPickBeatsThePlaneOnARamp is the regression this pair of constants
// exists for: the click used to be cast against one flat plane drawn through
// the player's feet, and on Geffen's steps that put the marker most of a
// hundred pixels from the pointer. Cast against the ground itself the two
// agree.
//
// The numbers are a stand-in for that camera: looking down at about 40°, from
// a little over a hundred units up, at a ramp climbing away from the eye.
func TestGroundPickBeatsThePlaneOnARamp(t *testing.T) {
	// Flat at the player's feet until x = 40, then climbing a unit for every
	// two east — a staircase's slope.
	ground := func(x, _ float32) float32 {
		if x <= 40 {
			return 0
		}

		return (x - 40) / 2
	}

	ray := picking.Ray{
		Origin:    [3]float32{0, 125, 0},
		Direction: [3]float32{0.766, -0.643, 0},
	}

	onGround, _, ok := ray.IntersectGround(ground, groundPickStep, groundPickReach)
	if !ok {
		t.Fatal("the ray never met the ramp")
	}

	onPlane, _, ok := ray.IntersectPlaneY(0)
	if !ok {
		t.Fatal("the ray never met the plane")
	}

	// The ground is above the plane here, so the ray meets it sooner.
	if onGround >= onPlane {
		t.Errorf("the ground was met at x=%v, no sooner than the plane's x=%v", onGround, onPlane)
	}

	// The crossing is on the ramp's own surface, which is the whole point.
	if got, want := ray.Origin[1]+onGround/ray.Direction[0]*ray.Direction[1], ground(onGround, 0); got < want-0.5 || got > want+0.5 {
		t.Errorf("the ray is at y=%v where the ramp is at %v", got, want)
	}

	// And the two answers are cells apart, not a rounding away.
	if gap := onPlane - onGround; gap < entity.CellSize {
		t.Errorf("plane and ground differ by %v units, less than a cell; the test is not showing the bug", gap)
	}
}
