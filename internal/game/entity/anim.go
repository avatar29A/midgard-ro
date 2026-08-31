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

// AnimIntervalMs returns the default frame duration for an action.
func AnimIntervalMs(action int) float32 {
	if action == ActionWalk {
		return WalkAnimIntervalMs
	}
	return IdleAnimIntervalMs
}

// frameIntervalMs is how long this character holds one frame of an action.
//
// The fixed rates above are right for players, whose idle is a single frame
// and whose walk the official client also drives at a fixed rate. They are
// wrong for anything else: a monster or NPC has a real idle animation with a
// rate stored in its own ACT, and holding those frames for a quarter second
// each turns a Kafra's idle into a ten-second crawl.
func (c *Character) frameIntervalMs(action int) float32 {
	if action >= 0 && action < len(c.AnimIntervalMs) && c.AnimIntervalMs[action] > 0 {
		return c.AnimIntervalMs[action]
	}
	return AnimIntervalMs(action)
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
func (c *Character) AdvanceAnimation(deltaMs float32, idleFrames, walkFrames, pickupFrames int) {
	if c.IsMoving {
		c.sinceWalkMs = 0

		// Moving cancels the pick-up: walking away should look like walking,
		// not like still bending over.
		c.playingOnce = false
	} else {
		c.sinceWalkMs += deltaMs
	}

	if c.playingOnce && c.advancePickup(deltaMs, pickupFrames) {
		return
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

	interval := c.frameIntervalMs(c.CurrentAction)
	c.FrameTime += deltaMs
	for c.FrameTime >= interval {
		c.FrameTime -= interval
		c.CurrentFrame++
	}
	c.CurrentFrame %= frameCount
}

// PlayPickup starts the pick-up motion, which plays through once and then
// gives way to whatever the movement state says.
//
// Started on the click rather than on the server's answer. It is feedback,
// not a change to anything the server owns: the item is not removed, the
// inventory is not touched, and a refused pick-up costs one motion nobody
// will mistake for an item arriving.
func (c *Character) PlayPickup() {
	c.playingOnce = true
	c.CurrentAction = ActionPickup
	c.CurrentFrame = 0
	c.FrameTime = 0
}

// advancePickup plays the pick-up through once, reporting whether it is still
// running.
//
// A sprite with no pick-up frames ends it immediately rather than freezing
// mid-motion — not every sheet has the action, and one that lacks it should
// simply not play it.
func (c *Character) advancePickup(deltaMs float32, frames int) bool {
	if frames <= 0 {
		c.playingOnce = false

		return false
	}

	c.CurrentAction = ActionPickup

	interval := c.frameIntervalMs(ActionPickup)
	c.FrameTime += deltaMs
	for c.FrameTime >= interval {
		c.FrameTime -= interval
		c.CurrentFrame++
	}

	if c.CurrentFrame >= frames {
		c.playingOnce = false
		c.CurrentFrame = 0
		c.FrameTime = 0

		return false
	}

	return true
}
