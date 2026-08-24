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
)

// AnimIntervalMs returns the frame duration for an action.
func AnimIntervalMs(action int) float32 {
	if action == ActionWalk {
		return WalkAnimIntervalMs
	}
	return IdleAnimIntervalMs
}

// AdvanceAnimation moves the sprite animation forward by deltaMs, looping
// within frameCount frames. It also picks the action to play from the
// movement state, resetting to frame 0 whenever that changes so a walk always
// starts on its first step.
//
// frameCount comes from whatever holds the loaded sprite sheet; a count of
// zero (no sprites yet) just parks the animation on frame 0.
func (c *Character) AdvanceAnimation(deltaMs float32, frameCount int) {
	action := ActionIdle
	if c.IsMoving {
		action = ActionWalk
	}

	if action != c.CurrentAction {
		c.CurrentAction = action
		c.CurrentFrame = 0
		c.FrameTime = 0
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
