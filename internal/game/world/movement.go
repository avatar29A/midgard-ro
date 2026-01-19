// Package world provides game world functionality.
package world

import (
	"github.com/Faultbox/midgard-ro/internal/game/entity"
)

// MovementController handles player movement with pathfinding.
type MovementController struct {
	pathFinder *PathFinder
	character  *entity.Character
	tileSize   float32

	// Current path
	path      [][2]int
	pathIndex int

	// Movement state
	IsFollowingPath bool
}

// NewMovementController creates a new movement controller.
func NewMovementController(pathFinder *PathFinder, character *entity.Character, tileSize float32) *MovementController {
	return &MovementController{
		pathFinder: pathFinder,
		character:  character,
		tileSize:   tileSize,
	}
}

// SetCharacter sets the character to control.
func (mc *MovementController) SetCharacter(character *entity.Character) {
	mc.character = character
	mc.ClearPath()
}

// MoveTo attempts to move to a destination tile.
// Returns the path if one exists, nil otherwise.
func (mc *MovementController) MoveTo(destTileX, destTileY int) [][2]int {
	if mc.character == nil || mc.pathFinder == nil {
		return nil
	}

	// Get current tile position
	currentTileX := int(mc.character.WorldX / mc.tileSize)
	currentTileY := int(mc.character.WorldZ / mc.tileSize)

	// Find path
	path := mc.pathFinder.FindPath(currentTileX, currentTileY, destTileX, destTileY)
	if path == nil || len(path) == 0 {
		return nil
	}

	// Set path (skip first node as it's current position)
	if len(path) > 1 {
		mc.path = path[1:]
	} else {
		mc.path = path
	}
	mc.pathIndex = 0
	mc.IsFollowingPath = true

	// Set first waypoint
	mc.setNextWaypoint()

	return path
}

// MoveToWorld attempts to move to a world position.
func (mc *MovementController) MoveToWorld(worldX, worldZ float32) [][2]int {
	tileX := int(worldX / mc.tileSize)
	tileY := int(worldZ / mc.tileSize)
	return mc.MoveTo(tileX, tileY)
}

// Update updates the movement controller.
// deltaMs is the time since last update in milliseconds.
func (mc *MovementController) Update(deltaMs float32) {
	if mc.character == nil {
		return
	}

	// Check if we've reached current waypoint
	if mc.IsFollowingPath && !mc.character.HasDestination && mc.pathIndex < len(mc.path) {
		// Move to next waypoint
		mc.setNextWaypoint()
	}

	// Check if path is complete
	if mc.IsFollowingPath && mc.pathIndex >= len(mc.path) && !mc.character.HasDestination {
		mc.IsFollowingPath = false
	}
}

// ClearPath stops the current path following.
func (mc *MovementController) ClearPath() {
	mc.path = nil
	mc.pathIndex = 0
	mc.IsFollowingPath = false
	if mc.character != nil {
		mc.character.ClearDestination()
	}
}

// GetPath returns the current path.
func (mc *MovementController) GetPath() [][2]int {
	return mc.path
}

// GetPathIndex returns the current index in the path.
func (mc *MovementController) GetPathIndex() int {
	return mc.pathIndex
}

func (mc *MovementController) setNextWaypoint() {
	if mc.pathIndex >= len(mc.path) {
		return
	}

	waypoint := mc.path[mc.pathIndex]
	worldX := (float32(waypoint[0]) + 0.5) * mc.tileSize // Center of tile
	worldZ := (float32(waypoint[1]) + 0.5) * mc.tileSize

	mc.character.SetDestination(worldX, worldZ)
	mc.pathIndex++
}

// CanWalkTo checks if a tile is walkable.
func (mc *MovementController) CanWalkTo(tileX, tileY int) bool {
	if mc.pathFinder == nil {
		return false
	}
	return mc.pathFinder.IsWalkable(tileX, tileY)
}

// WorldToTile converts world coordinates to tile coordinates.
func (mc *MovementController) WorldToTile(worldX, worldZ float32) (int, int) {
	return int(worldX / mc.tileSize), int(worldZ / mc.tileSize)
}

// TileToWorld converts tile coordinates to world coordinates (center of tile).
func (mc *MovementController) TileToWorld(tileX, tileY int) (float32, float32) {
	return (float32(tileX) + 0.5) * mc.tileSize, (float32(tileY) + 0.5) * mc.tileSize
}
