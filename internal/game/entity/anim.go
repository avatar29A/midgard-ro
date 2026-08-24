package entity

// Sprite animation timing.
//
// RO's ACT files carry a per-action interval, but the official client drives
// character animation on fixed rates instead — the stored intervals vary
// enough between jobs to make characters visibly out of step with each other.
// These are the rates the sprite work was tuned against.
const (
	// IdleAnimIntervalMs is how long each idle frame is held.
	IdleAnimIntervalMs = 250.0

	// WalkAnimIntervalMs is how long each walk frame is held.
	WalkAnimIntervalMs = 70.0

	// WalkHoldMs is how long the walk cycle keeps playing after a path ends.
	//
	// Walking is server-acknowledged one path at a time, so a character
	// crossing several cells stands still for a frame or two between steps
	// while the next acknowledgement is in flight. Switching to idle in that
	// gap restarts the walk cycle from frame 0 on every step, which reads as a
	// stutter even when the character never actually stopped. Holding the walk
	// briefly bridges the gap; a genuine stop is longer than this and still
	// settles into idle.
	WalkHoldMs = 120.0
)

// AnimIntervalMs returns the frame duration for an action.
func AnimIntervalMs(action int) float32 {
	if action == ActionWalk {
		return WalkAnimIntervalMs
	}
	return IdleAnimIntervalMs
}

// AdvanceAnimation moves the sprite animation forward by deltaMs, choosing the
// action from the movement state and looping within that action's frames.
// Changing action resets to frame 0, so a walk always starts on its first step.
//
// Both frame counts are passed in because this decides the action, not the
// caller: the walk is held briefly past the end of a path (see WalkHoldMs), so
// asking the caller to pick a count from IsMoving would give the wrong one
// exactly during the gap this is here to smooth over. Counts come from
// whatever holds the loaded sprite sheet; zero (no sprites yet) parks the
// animation on frame 0.
func (c *Character) AdvanceAnimation(deltaMs float32, idleFrames, walkFrames int) {
	if c.IsMoving {
		c.sinceWalkMs = 0
	} else {
		c.sinceWalkMs += deltaMs
	}

	action := ActionIdle
	if c.IsMoving || c.sinceWalkMs < WalkHoldMs {
		action = ActionWalk
	}

	if action != c.CurrentAction {
		c.CurrentAction = action
		c.CurrentFrame = 0
		c.FrameTime = 0
	}

	frameCount := idleFrames
	if c.CurrentAction == ActionWalk {
		frameCount = walkFrames
	}
	if frameCount <= 0 {
		c.CurrentFrame = 0
		return
	}

	interval := AnimIntervalMs(c.CurrentAction)
	c.FrameTime += deltaMs
	for c.FrameTime >= interval {
		c.FrameTime -= interval
		c.CurrentFrame++
	}
	c.CurrentFrame %= frameCount
}
