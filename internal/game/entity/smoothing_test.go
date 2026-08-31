package entity

import (
	"math"
	"testing"
)

// TestCorrectionDoesNotTeleport is the regression guard for the visible jump:
// a path arriving while the character is drawn a cell or two behind must not
// move the drawn position discontinuously.
func TestCorrectionDoesNotTeleport(t *testing.T) {
	c := newWalkerAt(10, 10)

	// Pretend the last walk left us drawn two cells west of where the server
	// now says we are.
	c.RenderX -= 2 * CellSize
	before := c.RenderX

	c.FollowPath([][2]int{{10, 10}, {11, 10}})

	if jump := math.Abs(float64(c.RenderX - before)); jump > 0.001 {
		t.Errorf("drawn position jumped %.2f world units on receiving a path; "+
			"the correction should be carried, not snapped", jump)
	}
}

// TestCorrectionIsBoundedBySpeed checks the other half: the character catches
// up by walking slightly faster, never by sprinting a whole cell in a frame.
func TestCorrectionIsBoundedBySpeed(t *testing.T) {
	c := newWalkerAt(10, 10)
	c.RenderX -= 2 * CellSize // two cells behind
	c.FollowPath([][2]int{{10, 10}, {11, 10}, {12, 10}, {13, 10}})

	const frame = 16.0
	maxStep := float64(0)
	prevX := c.RenderX

	for i := 0; i < 40 && c.IsWalkingPath(); i++ {
		c.Update(frame)
		if step := math.Abs(float64(c.RenderX - prevX)); step > maxStep {
			maxStep = step
		}
		prevX = c.RenderX
	}

	// Walking covers CellSize per WalkSpeedMs. Catching up adds at most
	// CatchUpFraction on top, plus a little slack for the discrete frame.
	perFrame := float64(CellSize) * frame / DefaultWalkSpeedMs
	limit := perFrame * (1 + CatchUpFraction) * 1.3

	if maxStep > limit {
		t.Errorf("largest single-frame move was %.3f units, above the %.3f cap; "+
			"the character sprinted to catch up instead of walking it off",
			maxStep, limit)
	}
}

// TestCorrectionConverges checks the catch-up actually finishes rather than
// leaving the character permanently trailing.
func TestCorrectionConverges(t *testing.T) {
	c := newWalkerAt(10, 10)
	c.RenderX -= 2 * CellSize
	c.FollowPath([][2]int{{10, 10}, {11, 10}, {12, 10}, {13, 10}, {14, 10}})

	for i := 0; i < 200 && c.IsWalkingPath(); i++ {
		c.Update(16)
	}
	// Let any residue bleed off while idle.
	for i := 0; i < 60 && c.VisualOffset() > 0; i++ {
		c.UpdateRenderPosition(16)
	}

	if got := c.VisualOffset(); got > 0.01 {
		t.Errorf("visual offset settled at %.3f units, want ~0", got)
	}
	if diff := math.Abs(float64(c.RenderX - c.WorldX)); diff > 0.01 {
		t.Errorf("drawn position ended %.3f units from the authoritative one", diff)
	}
}

// TestLargeDesyncSnaps: past a few cells the gap is too big to walk off, and
// sliding for seconds would be worse than the jump.
func TestLargeDesyncSnaps(t *testing.T) {
	c := newWalkerAt(10, 10)
	c.RenderX -= 20 * CellSize

	c.FollowPath([][2]int{{10, 10}, {11, 10}})

	if got := c.VisualOffset(); got != 0 {
		t.Errorf("visual offset = %.3f after a %d-cell desync, want 0 (snapped)",
			got, 20)
	}
}

// TestWalkAnimationSurvivesGapBetweenSteps is the regression guard for the
// stutter: walking cell by cell leaves a frame or two of stillness between
// server acknowledgements, and restarting the walk cycle on each one reads as
// a hitch even though the character never stopped.
func TestWalkAnimationSurvivesGapBetweenSteps(t *testing.T) {
	const idleFrames, walkFrames = 1, 8

	c := newWalkerAt(10, 10)
	c.FollowPath([][2]int{{10, 10}, {11, 10}})

	// Walk the step out.
	for i := 0; i < 20 && c.IsWalkingPath(); i++ {
		c.Update(16)
		c.AdvanceAnimation(16, idleFrames, walkFrames, 0)
	}
	if c.CurrentAction != ActionWalk {
		t.Fatalf("expected to be walking, action = %d", c.CurrentAction)
	}
	frameBeforeGap := c.CurrentFrame

	// Two frames of nothing while the next acknowledgement is in flight.
	c.AdvanceAnimation(16, idleFrames, walkFrames, 0)
	c.AdvanceAnimation(16, idleFrames, walkFrames, 0)

	if c.CurrentAction != ActionWalk {
		t.Error("walk cycle dropped to idle during the gap between steps")
	}

	// Next step arrives; the cycle should carry on, not restart.
	c.FollowPath([][2]int{{11, 10}, {12, 10}})
	c.AdvanceAnimation(16, idleFrames, walkFrames, 0)

	if c.CurrentFrame == 0 && frameBeforeGap != 0 {
		t.Error("walk animation restarted at frame 0 on the next step")
	}
}

// TestWalkAnimationSettlesToIdle: the hold is a bridge, not a latch. A real
// stop still ends up idle.
func TestWalkAnimationSettlesToIdle(t *testing.T) {
	const idleFrames, walkFrames = 1, 8

	c := newWalkerAt(10, 10)
	c.FollowPath([][2]int{{10, 10}, {11, 10}})
	for i := 0; i < 20 && c.IsWalkingPath(); i++ {
		c.Update(16)
		c.AdvanceAnimation(16, idleFrames, walkFrames, 0)
	}

	// Stand still for comfortably longer than the hold.
	for elapsed := float32(0); elapsed < WalkHoldMs*3; elapsed += 16 {
		c.AdvanceAnimation(16, idleFrames, walkFrames, 0)
	}

	if c.CurrentAction != ActionIdle {
		t.Errorf("action = %d after standing still, want idle — the walk hold "+
			"should bridge a gap, not latch on", c.CurrentAction)
	}
}
