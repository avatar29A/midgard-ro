package formats

import "fmt"

// DirectionNames provides names for the 8 RO sprite directions.
var DirectionNames = []string{
	"S", "SW", "W", "NW", "N", "NE", "E", "SE",
}

// MonsterActionNames provides names for monster sprite action types.
// Monsters typically have 5-8 action types with 8 directions each.
var MonsterActionNames = []string{
	"Idle",     // 0
	"Walk",     // 1
	"Attack",   // 2
	"Damage",   // 3
	"Die",      // 4
	"Attack 2", // 5 (some monsters)
	"Attack 3", // 6 (some monsters)
	"Special",  // 7 (some monsters)
}

// PlayerActionNames provides names for player sprite action types.
// Players typically have 13+ action types with 8 directions each.
var PlayerActionNames = []string{
	"Idle",        // 0
	"Walk",        // 1
	"Sit",         // 2
	"Pick Up",     // 3
	"Standby",     // 4
	"Attack 1",    // 5
	"Damage",      // 6
	"Die",         // 7
	"Dead",        // 8
	"Attack 2",    // 9
	"Attack 3",    // 10
	"Skill Cast",  // 11
	"Skill Ready", // 12
	"Freeze",      // 13
}

// GetActionName returns a standard RO action name for the given index.
// For sprites with 8 directions per action, returns "ActionType Dir" format.
// Uses different naming based on total action count to detect monster vs player sprites.
func GetActionName(index, totalActions int) string {
	// Check if this looks like an 8-direction sprite (multiple of 8 actions)
	if totalActions >= 8 && totalActions%8 == 0 {
		actionType := index / 8
		direction := index % 8
		dirName := DirectionNames[direction]

		// Determine sprite type based on action count:
		// - Monster sprites: typically 40-64 actions (5-8 action types)
		// - Player sprites: typically 80+ actions (10+ action types)
		var typeName string
		actionTypeCount := totalActions / 8

		if actionTypeCount <= 8 {
			// Likely a monster sprite
			if actionType < len(MonsterActionNames) {
				typeName = MonsterActionNames[actionType]
			} else {
				typeName = fmt.Sprintf("Action%d", actionType)
			}
		} else {
			// Likely a player sprite
			if actionType < len(PlayerActionNames) {
				typeName = PlayerActionNames[actionType]
			} else {
				typeName = fmt.Sprintf("Action%d", actionType)
			}
		}

		return fmt.Sprintf("%s %s", typeName, dirName)
	}

	// Non-8-direction sprite (items, effects, etc.) - just show index
	return fmt.Sprintf("Action %d", index)
}

// GetDirectionName returns the name for a direction index (0-7).
func GetDirectionName(direction int) string {
	if direction >= 0 && direction < len(DirectionNames) {
		return DirectionNames[direction]
	}
	return fmt.Sprintf("Dir%d", direction)
}

// GetActionTypeName returns the action type name for player or monster sprites.
// isPlayer should be true for player sprites, false for monster sprites.
func GetActionTypeName(actionType int, isPlayer bool) string {
	if isPlayer {
		if actionType < len(PlayerActionNames) {
			return PlayerActionNames[actionType]
		}
	} else {
		if actionType < len(MonsterActionNames) {
			return MonsterActionNames[actionType]
		}
	}
	return fmt.Sprintf("Action%d", actionType)
}

// Action type constants for common actions.
const (
	ActionIdle    = 0
	ActionWalk    = 1
	ActionSit     = 2 // Player only
	ActionPickUp  = 3 // Player only
	ActionStandby = 4 // Player only
	ActionAttack  = 2 // Monster (same slot as Sit for players)
	ActionDamage  = 3 // Monster (same slot as PickUp for players)
	ActionDie     = 4 // Monster (same slot as Standby for players)
)

// Direction constants for 8-way sprites.
const (
	DirS  = 0 // South (facing camera)
	DirSW = 1 // Southwest
	DirW  = 2 // West
	DirNE = 5 // Northeast
	DirE  = 6 // East
	DirSE = 7 // Southeast
	DirNW = 3 // Northwest
	DirN  = 4 // North (facing away)
)
