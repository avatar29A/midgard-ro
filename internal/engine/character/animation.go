package character

import (
	"github.com/Faultbox/midgard-ro/internal/game/entity"
)

// UpdateAnimation advances player animation frame based on elapsed time.
// deltaMs is the time since last update in milliseconds.
func UpdateAnimation(player *Player, deltaMs float32) {
	if player == nil || player.Character == nil {
		return
	}

	// Procedural players don't have animation data
	if player.ACT == nil {
		return
	}

	// Determine action based on movement state
	newAction := entity.ActionIdle
	if player.IsMoving {
		newAction = entity.ActionWalk
	}

	// Reset frame when action changes
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

	// Get animation interval from ACT (default 150ms for smoother animation)
	interval := float32(150.0)
	if actionIdx < len(player.ACT.Intervals) && player.ACT.Intervals[actionIdx] > 0 {
		interval = player.ACT.Intervals[actionIdx]
		// ACT intervals can be very small, enforce minimum
		if interval < 50 {
			interval = 50
		}
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

// DefaultAnimInterval is the default animation interval in milliseconds.
const DefaultAnimInterval = 150.0

// MinAnimInterval is the minimum animation interval in milliseconds.
const MinAnimInterval = 50.0
