package entity

import "testing"

// TestSpriteOwnIntervalDrivesAnimation is the difference between a Kafra who
// shifts her weight and one who crawls. The fixed idle rate suits a player,
// whose idle is a single frame; applied to a real animation it holds every
// frame for a quarter of a second.
func TestSpriteOwnIntervalDrivesAnimation(t *testing.T) {
	const ownInterval = 100.0

	c := NewCharacter(0, 0, 0)
	c.AnimIntervalMs = [LoadedActions]float32{ownInterval, 0}

	// Settle into idle first: a character that has never walked still holds
	// the walk cycle for WalkHoldMs, and the switch resets the frame.
	c.AdvanceAnimation(WalkHoldMs, 4, 4, 0, 0)
	if c.CurrentAction != ActionIdle {
		t.Fatalf("action = %d, want idle after the walk hold elapsed", c.CurrentAction)
	}
	c.CurrentFrame, c.FrameTime = 0, 0

	// Four idle frames at the sprite's own rate: one step per interval.
	c.AdvanceAnimation(ownInterval, 4, 4, 0, 0)
	if c.CurrentFrame != 1 {
		t.Errorf("frame = %d after one interval, want 1", c.CurrentFrame)
	}
	c.AdvanceAnimation(ownInterval*2, 4, 4, 0, 0)
	if c.CurrentFrame != 3 {
		t.Errorf("frame = %d after three intervals, want 3", c.CurrentFrame)
	}

	// The default rate would have advanced barely one frame in that time.
	slow := NewCharacter(0, 0, 0)
	slow.AdvanceAnimation(WalkHoldMs, 4, 4, 0, 0)
	slow.CurrentFrame, slow.FrameTime = 0, 0
	slow.AdvanceAnimation(ownInterval*3, 4, 4, 0, 0)
	if slow.CurrentFrame != 1 {
		t.Errorf("default-rate frame = %d, want 1; the fixed idle rate is %vms "+
			"a frame, which is what made long animations crawl",
			slow.CurrentFrame, IdleAnimIntervalMs)
	}
}

// TestDefaultIntervalWhenSpriteDoesNotSay: an ACT that carries no interval
// falls back to the fixed rate rather than to zero, which would advance the
// animation once per frame and blur it.
func TestDefaultIntervalWhenSpriteDoesNotSay(t *testing.T) {
	c := NewCharacter(0, 0, 0)

	if got := c.frameIntervalMs(ActionIdle); got != IdleAnimIntervalMs {
		t.Errorf("idle interval = %v, want the default %v", got, IdleAnimIntervalMs)
	}
	if got := c.frameIntervalMs(ActionWalk); got != WalkAnimIntervalMs {
		t.Errorf("walk interval = %v, want the default %v", got, WalkAnimIntervalMs)
	}

	c.AnimIntervalMs = [LoadedActions]float32{0, 40}
	if got := c.frameIntervalMs(ActionWalk); got != 40 {
		t.Errorf("walk interval = %v, want the sprite's 40", got)
	}
	if got := c.frameIntervalMs(ActionIdle); got != IdleAnimIntervalMs {
		t.Errorf("idle interval = %v, want the default when the sprite says zero", got)
	}
}

// TestPickupPlaysOnceThenReturns: the pick-up motion is a one-shot. It has to
// end by itself, because nothing else will end it — there is no "stopped
// picking up" from the server, only the motion running out.
func TestPickupPlaysOnceThenReturns(t *testing.T) {
	c := NewCharacter(0, 0, 0)
	c.PlayPickup()

	if c.CurrentAction != ActionPickup {
		t.Fatalf("CurrentAction = %d, want ActionPickup", c.CurrentAction)
	}

	// Three frames at the rate the pick-up actually runs at — its own, times
	// the slowdown that gives the stoop room to be seen — then off the end.
	const frames = 3

	step := AnimIntervalMs(ActionPickup) * PickupSlowdown
	for i := 0; i < frames; i++ {
		c.AdvanceAnimation(step, 1, 8, frames, 0)
	}

	if c.CurrentAction == ActionPickup {
		t.Error("still picking up after the last frame; the one-shot never ended")
	}
	if c.CurrentFrame != 0 {
		t.Errorf("CurrentFrame = %d, want 0 once the motion is over", c.CurrentFrame)
	}
}

// TestPickupYieldsToWalking: walking away cancels the motion rather than
// finishing it, so a character who is moving is drawn moving.
func TestPickupYieldsToWalking(t *testing.T) {
	c := NewCharacter(0, 0, 0)
	c.PlayPickup()
	c.IsMoving = true

	c.AdvanceAnimation(16, 1, 8, 6, 0)

	if c.CurrentAction != ActionWalk {
		t.Errorf("CurrentAction = %d, want ActionWalk while moving", c.CurrentAction)
	}
}

// TestPickupWithNoFramesDoesNotStick: a sheet without the action must not
// freeze the character mid-motion.
func TestPickupWithNoFramesDoesNotStick(t *testing.T) {
	c := NewCharacter(0, 0, 0)
	c.PlayPickup()

	c.AdvanceAnimation(16, 1, 8, 0, 0)

	if c.CurrentAction == ActionPickup {
		t.Error("stuck in a pick-up the sprite has no frames for")
	}
}

// TestStandbyOnlyWhileReady: the armed stance is worn while there is
// something to stand ready against and dropped the moment there is not.
// Holding it the rest of the time reads as a character stuck mid-fight, which
// is exactly how it was reported.
func TestStandbyOnlyWhileReady(t *testing.T) {
	c := NewCharacter(0, 0, 0)

	// Past the walk hold, which bridges the gap between acknowledged paths
	// and keeps a character that has just stopped on the walk for a moment.
	c.AdvanceAnimation(WalkHoldMs, 1, 8, 0, 6)

	c.AdvanceAnimation(16, 1, 8, 0, 6)
	if c.CurrentAction != ActionIdle {
		t.Errorf("CurrentAction = %d, want the plain idle when not fighting", c.CurrentAction)
	}

	c.Ready = true
	c.AdvanceAnimation(16, 1, 8, 0, 6)
	if c.CurrentAction != ActionStandby {
		t.Errorf("CurrentAction = %d, want the armed stance while fighting", c.CurrentAction)
	}

	c.Ready = false
	c.AdvanceAnimation(16, 1, 8, 0, 6)
	if c.CurrentAction != ActionIdle {
		t.Errorf("CurrentAction = %d, want the plain idle once the fight is over", c.CurrentAction)
	}
}

// TestWalkingOutranksTheStance: a character running to a target is drawn
// running, not standing ready.
func TestWalkingOutranksTheStance(t *testing.T) {
	c := NewCharacter(0, 0, 0)
	c.Ready = true
	c.IsMoving = true

	c.AdvanceAnimation(16, 1, 8, 0, 6)

	if c.CurrentAction != ActionWalk {
		t.Errorf("CurrentAction = %d, want the walk while moving", c.CurrentAction)
	}
}

// TestOneShotSpeedScalesTheRate: a character that attacks quickly swings
// quickly. The scale multiplies the frame interval, so half runs it twice as
// fast.
func TestOneShotSpeedScalesTheRate(t *testing.T) {
	const frames = 4

	fast := NewCharacter(0, 0, 0)
	fast.PlayOnceAt(ActionAttack, 0.5)

	slow := NewCharacter(0, 0, 0)
	slow.PlayOnceAt(ActionAttack, 1)

	// One frame's worth at the halved rate advances the fast one and not the
	// other.
	step := AnimIntervalMs(ActionAttack) * 0.5
	fast.AdvanceAnimation(step, 1, 8, frames, 0)
	slow.AdvanceAnimation(step, 1, 8, frames, 0)

	if fast.CurrentFrame == slow.CurrentFrame {
		t.Errorf("both on frame %d; the scale did not change the rate", fast.CurrentFrame)
	}
	if fast.CurrentFrame <= slow.CurrentFrame {
		t.Errorf("fast on frame %d, slow on %d — the faster swing should be ahead",
			fast.CurrentFrame, slow.CurrentFrame)
	}
}

// TestOneShotSpeedRejectsNonsense: a speed of zero would stall the animation
// on its first frame forever.
func TestOneShotSpeedRejectsNonsense(t *testing.T) {
	c := NewCharacter(0, 0, 0)
	c.PlayOnceAt(ActionAttack, 0)

	if c.OnceSpeed != 1 {
		t.Errorf("OnceSpeed = %v, want 1 when given nonsense", c.OnceSpeed)
	}
}
