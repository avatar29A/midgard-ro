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

	// PickupAnimIntervalMs is how long each frame of the pick-up is held.
	//
	// Nearer the walk's rate than the idle's: bending down and straightening
	// up is a quick motion, and at the idle rate the whole thing took over a
	// second, which reads as the character being reluctant rather than busy.
	PickupAnimIntervalMs = 60.0

	// ActionAnimIntervalMs is how long each frame of a blow — thrown, taken,
	// or fatal — is held.
	//
	// A swing has to finish inside the attack it belongs to or the next one
	// starts before the last has landed, and a flinch has to be over quickly
	// enough to take the following hit.
	ActionAnimIntervalMs = 50.0

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
	switch action {
	case ActionWalk:
		return WalkAnimIntervalMs
	case ActionPickup:
		return PickupAnimIntervalMs
	case ActionAttack, ActionHurt, ActionDie:
		return ActionAnimIntervalMs
	default:
		return IdleAnimIntervalMs
	}
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
func (c *Character) AdvanceAnimation(deltaMs float32, idleFrames, walkFrames, onceFrames int) {
	// Death outranks everything and does not end: a corpse is not idle, and
	// nothing it might otherwise be doing matters any more.
	if c.Dead {
		c.holdDeath(deltaMs, onceFrames)

		return
	}

	if c.IsMoving {
		c.sinceWalkMs = 0

		// Moving cancels a one-shot: walking away from a blow should look
		// like walking, not like still swinging.
		c.playingOnce = false
	} else {
		c.sinceWalkMs += deltaMs
	}

	if c.playingOnce && c.advanceOnce(deltaMs, onceFrames) {
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

// PlayOnce starts an action that plays through once and then gives way to
// whatever the movement state says.
//
// Started when the thing it depicts is asked for or reported, not when its
// consequences are confirmed: it is feedback, and a motion nobody will
// mistake for an outcome costs nothing when the server disagrees.
func (c *Character) PlayOnce(action int) {
	c.playingOnce = true
	c.OnceAction = action
	c.CurrentAction = action
	c.CurrentFrame = 0
	c.FrameTime = 0
}

// PlayPickup starts the pick-up motion.
func (c *Character) PlayPickup() { c.PlayOnce(ActionPickup) }

// PlayAttack starts a swing.
func (c *Character) PlayAttack() { c.PlayOnce(ActionAttack) }

// PlayHurt starts a flinch.
func (c *Character) PlayHurt() { c.PlayOnce(ActionHurt) }

// Die stops the character on its death animation, where it stays.
func (c *Character) Die() {
	if c.Dead {
		return
	}

	c.Dead = true
	c.playingOnce = false
	c.OnceAction = ActionDie
	c.CurrentAction = ActionDie
	c.CurrentFrame = 0
	c.FrameTime = 0
	c.ClearDestination()
}

// Revive puts a dead character back on its feet.
func (c *Character) Revive() {
	c.Dead = false
	c.playingOnce = false
	c.OnceAction = ActionIdle
	c.CurrentAction = ActionIdle
	c.CurrentFrame = 0
	c.FrameTime = 0
}

// OnceAction is the action a one-shot is playing, which the caller needs in
// order to look up how many frames it has.
func (c *Character) PlayingAction() int {
	if c.Dead {
		return ActionDie
	}
	if c.playingOnce {
		return c.OnceAction
	}

	return -1
}

// holdDeath runs the death animation to its last frame and stops there.
func (c *Character) holdDeath(deltaMs float32, frames int) {
	c.CurrentAction = ActionDie
	if frames <= 0 {
		c.CurrentFrame = 0

		return
	}

	if c.CurrentFrame >= frames-1 {
		c.CurrentFrame = frames - 1

		return
	}

	interval := c.frameIntervalMs(ActionDie)
	c.FrameTime += deltaMs
	for c.FrameTime >= interval && c.CurrentFrame < frames-1 {
		c.FrameTime -= interval
		c.CurrentFrame++
	}
}

// advanceOnce plays a one-shot through once, reporting whether it is still
// running.
//
// A sprite with no frames for the action ends it immediately rather than
// freezing mid-motion — not every sheet has every action, and one that lacks
// it should simply not play it.
func (c *Character) advanceOnce(deltaMs float32, frames int) bool {
	if frames <= 0 {
		c.playingOnce = false

		return false
	}

	c.CurrentAction = c.OnceAction

	interval := c.frameIntervalMs(c.OnceAction)
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
