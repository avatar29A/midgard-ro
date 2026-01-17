package character

import (
	"github.com/Faultbox/midgard-ro/internal/game/entity"
)

// Animation timing defaults (independent of movement speed)
const (
	// DefaultIdleAnimInterval is the default interval for idle animation in milliseconds
	DefaultIdleAnimInterval = 250.0
	// DefaultWalkAnimInterval is the default interval for walk animation in milliseconds
	DefaultWalkAnimInterval = 70.0
)

// Configurable animation intervals (can be modified at runtime)
var (
	// IdleAnimInterval is the current interval for idle animation in milliseconds
	IdleAnimInterval float32 = DefaultIdleAnimInterval
	// WalkAnimInterval is the current interval for walk animation in milliseconds
	WalkAnimInterval float32 = DefaultWalkAnimInterval
)

// UpdateAnimation advances player animation frame based on elapsed time.
// Animation timing is independent of movement speed.
// deltaMs is the time since last update in milliseconds.
func UpdateAnimation(player *Player, deltaMs float32) {
	if player == nil || player.Character == nil {
		return
	}

	// Update render position interpolation for smooth movement
	player.Character.UpdateRenderPosition(deltaMs)

	// Procedural players don't have animation data
	if player.ACT == nil {
		return
	}

	// Determine action based on movement state
	newAction := entity.ActionIdle
	if player.IsMoving {
		newAction = entity.ActionWalk
	}

	// Reset animation time when action changes
	if newAction != player.CurrentAction {
		player.CurrentAction = newAction
		player.CurrentFrame = 0
		player.FrameTime = 0
	}

	// Get current action
	actionIdx := player.CurrentAction*8 + player.Direction
	if actionIdx >= len(player.ACT.Actions) {
		actionIdx = 0
	}
	action := &player.ACT.Actions[actionIdx]
	if len(action.Frames) == 0 {
		return
	}

	// Get animation interval - use configurable values (ignore ACT intervals for consistency)
	var interval float32
	if player.CurrentAction == entity.ActionWalk {
		interval = WalkAnimInterval
	} else {
		interval = IdleAnimInterval
	}

	// Accumulate time and advance frames
	player.FrameTime += deltaMs
	if player.FrameTime >= interval {
		player.FrameTime -= interval
		player.CurrentFrame++
		if player.CurrentFrame >= len(action.Frames) {
			player.CurrentFrame = 0 // Loop animation
		}
	}
}

// GetActionIndex returns the action index for the current action and direction.
func GetActionIndex(player *Player) int {
	if player == nil || player.Character == nil || player.ACT == nil {
		return 0
	}
	actionIdx := player.CurrentAction*8 + player.Direction
	if actionIdx >= len(player.ACT.Actions) {
		return 0
	}
	return actionIdx
}

// GetFrameCount returns the number of frames in the current action.
func GetFrameCount(player *Player) int {
	actionIdx := GetActionIndex(player)
	if player.ACT == nil || actionIdx >= len(player.ACT.Actions) {
		return 0
	}
	return len(player.ACT.Actions[actionIdx].Frames)
}
