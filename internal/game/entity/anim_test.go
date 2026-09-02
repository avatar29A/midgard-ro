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
	c.AdvanceAnimation(WalkHoldMs, 4, 4, 0, 0, 0)
	if c.CurrentAction != ActionIdle {
		t.Fatalf("action = %d, want idle after the walk hold elapsed", c.CurrentAction)
	}
	c.CurrentFrame, c.FrameTime = 0, 0

	// Four idle frames at the sprite's own rate: one step per interval.
	c.AdvanceAnimation(ownInterval, 4, 4, 0, 0, 0)
	if c.CurrentFrame != 1 {
		t.Errorf("frame = %d after one interval, want 1", c.CurrentFrame)
	}
	c.AdvanceAnimation(ownInterval*2, 4, 4, 0, 0, 0)
	if c.CurrentFrame != 3 {
		t.Errorf("frame = %d after three intervals, want 3", c.CurrentFrame)
	}

	// The default rate would have advanced barely one frame in that time.
	slow := NewCharacter(0, 0, 0)
	slow.AdvanceAnimation(WalkHoldMs, 4, 4, 0, 0, 0)
	slow.CurrentFrame, slow.FrameTime = 0, 0
	slow.AdvanceAnimation(ownInterval*3, 4, 4, 0, 0, 0)
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
		c.AdvanceAnimation(step, 1, 8, frames, 0, 0)
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

	c.AdvanceAnimation(16, 1, 8, 6, 0, 0)

	if c.CurrentAction != ActionWalk {
		t.Errorf("CurrentAction = %d, want ActionWalk while moving", c.CurrentAction)
	}
}

// TestPickupWithNoFramesDoesNotStick: a sheet without the action must not
// freeze the character mid-motion.
func TestPickupWithNoFramesDoesNotStick(t *testing.T) {
	c := NewCharacter(0, 0, 0)
	c.PlayPickup()

	c.AdvanceAnimation(16, 1, 8, 0, 0, 0)

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
	c.AdvanceAnimation(WalkHoldMs, 1, 8, 0, 6, 0)

	c.AdvanceAnimation(16, 1, 8, 0, 6, 0)
	if c.CurrentAction != ActionIdle {
		t.Errorf("CurrentAction = %d, want the plain idle when not fighting", c.CurrentAction)
	}

	c.Ready = true
	c.AdvanceAnimation(16, 1, 8, 0, 6, 0)
	if c.CurrentAction != ActionStandby {
		t.Errorf("CurrentAction = %d, want the armed stance while fighting", c.CurrentAction)
	}

	c.Ready = false
	c.AdvanceAnimation(16, 1, 8, 0, 6, 0)
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

	c.AdvanceAnimation(16, 1, 8, 0, 6, 0)

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
	fast.AdvanceAnimation(step, 1, 8, frames, 0, 0)
	slow.AdvanceAnimation(step, 1, 8, frames, 0, 0)

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

// TestSittingOutranksTheWalkHold: sitting is a state rather than a motion, and
// the walk is held briefly past the end of a path. Without sitting winning,
// a character that sat down at the end of a step would stay on its feet for
// the length of that hold.
func TestSittingOutranksTheWalkHold(t *testing.T) {
	c := &Character{}
	c.IsMoving = true
	c.AdvanceAnimation(0, 4, 4, 0, 0, 1)

	c.IsMoving = false
	c.Sitting = true
	c.AdvanceAnimation(WalkHoldMs/2, 4, 4, 0, 0, 1)

	if c.CurrentAction != ActionSit {
		t.Errorf("action is %d, want the seated pose", c.CurrentAction)
	}
}

// TestStandingUpLeavesTheSeatedPose: clearing the flag is all it takes, since
// the pose is chosen from the flag every frame rather than latched.
func TestStandingUpLeavesTheSeatedPose(t *testing.T) {
	// Past the walk hold first: a character that has never moved still counts
	// as having just stopped, and would otherwise stand up into the walk.
	c := &Character{Sitting: true}
	c.AdvanceAnimation(WalkHoldMs*2, 4, 4, 0, 0, 1)

	c.Sitting = false
	c.AdvanceAnimation(0, 4, 4, 0, 0, 1)

	if c.CurrentAction != ActionIdle {
		t.Errorf("action is %d, want the idle", c.CurrentAction)
	}
}

// TestSittingDoesNotBlockAOneShot: a seated character still flinches when it
// is hit, because a one-shot is checked before the standing pose is chosen.
func TestSittingDoesNotBlockAOneShot(t *testing.T) {
	c := &Character{Sitting: true}
	c.PlayOnce(ActionHurt)
	c.AdvanceAnimation(0, 4, 4, 3, 0, 1)

	if c.CurrentAction != ActionHurt {
		t.Errorf("action is %d, want the flinch", c.CurrentAction)
	}
}

// TestPlayOnceForFillsTheWholeDuration: a swing has to be over before the next
// one starts, so it is given a length rather than a rate — however many frames
// the sprite turns out to have.
func TestPlayOnceForFillsTheWholeDuration(t *testing.T) {
	const (
		frames   = 9
		duration = 320.0
		step     = duration / frames
	)

	c := NewCharacter(0, 0, 0)
	c.PlayOnceFor(ActionAttack, duration)

	// Halfway through the last frame, the swing is still going and is on it.
	c.AdvanceAnimation(duration-step/2, 1, 1, frames, 0, 0)
	if !c.playingOnce {
		t.Fatalf("the swing ended early, on frame %d", c.CurrentFrame)
	}
	if c.CurrentFrame != frames-1 {
		t.Errorf("frame %d with one to go, want %d", c.CurrentFrame, frames-1)
	}

	// And it is over by the time the duration is up.
	c.AdvanceAnimation(step, 1, 1, frames, 0, 0)
	if c.playingOnce {
		t.Errorf("the swing is still going past its %vms", duration)
	}
}

// TestPlayOnceForTracksAttackSpeed: twice the attack speed is half the swing,
// which is the whole reason a swing is given a length.
func TestPlayOnceForTracksAttackSpeed(t *testing.T) {
	const frames = 9

	elapsed := func(duration float32) float32 {
		c := NewCharacter(0, 0, 0)
		c.PlayOnceFor(ActionAttack, duration)

		total := float32(0)
		for c.playingOnce && total < 5000 {
			c.AdvanceAnimation(1, 1, 1, frames, 0, 0)
			total++
		}

		return total
	}

	slow, fast := elapsed(432), elapsed(216)

	if slow <= fast {
		t.Fatalf("the slow swing took %v against the fast one's %v", slow, fast)
	}
	if diff := slow - fast*2; diff > frames || diff < -frames {
		t.Errorf("halving the motion gave %v against %v, want about half", fast, slow)
	}
}

// TestASwingIsNotCancelledByMovement: a character reaches its target and
// swings in the same breath, and the client is still finishing the last step
// when the server's answer arrives. Canceling on movement threw away most of
// the swings made while closing on something.
func TestASwingIsNotCancelledByMovement(t *testing.T) {
	c := NewCharacter(0, 0, 0)
	c.PlayAttackFor(320)
	c.IsMoving = true

	c.AdvanceAnimation(16, 1, 8, 9, 0, 0)

	if !c.playingOnce {
		t.Error("the swing was canceled by the character still moving")
	}
	if c.CurrentAction != ActionAttack {
		t.Errorf("action is %d, want the attack", c.CurrentAction)
	}
}

// TestAFlinchIsStillCancelledByMovement: the reason that rule exists.
// Walking away from a blow should look like walking, not like still flinching.
func TestAFlinchIsStillCancelledByMovement(t *testing.T) {
	c := NewCharacter(0, 0, 0)
	c.PlayHurt()
	c.IsMoving = true

	c.AdvanceAnimation(16, 1, 8, 3, 0, 0)

	if c.playingOnce {
		t.Error("the flinch outlasted the character walking away from it")
	}
	if c.CurrentAction != ActionWalk {
		t.Errorf("action is %d, want the walk", c.CurrentAction)
	}
}
