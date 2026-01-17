package character

import (
	gomath "math"

	"github.com/Faultbox/midgard-ro/internal/game/entity"
)

// UpdateMovement updates player position for click-to-move navigation.
// deltaMs is the time since last update in milliseconds.
// terrain provides walkability and height queries (can be nil for no terrain checks).
func UpdateMovement(player *Player, deltaMs float32, terrain TerrainQuery) {
	if player == nil || player.Character == nil || !player.HasDestination {
		return
	}

	// Calculate direction to destination
	dx := player.DestX - player.WorldX
	dz := player.DestZ - player.WorldZ
	dist := float32(gomath.Sqrt(float64(dx*dx + dz*dz)))

	// Check if reached destination
	if dist < ArrivalThreshold {
		player.HasDestination = false
		player.IsMoving = false
		player.CurrentAction = entity.ActionIdle
		return
	}

	// Normalize direction
	dx /= dist
	dz /= dist

	// Calculate movement amount
	moveAmount := player.MoveSpeed * deltaMs / 1000.0
	if moveAmount > dist {
		moveAmount = dist
	}

	// Calculate new position
	newX := player.WorldX + dx*moveAmount
	newZ := player.WorldZ + dz*moveAmount

	// Check if new position is walkable (if terrain query provided)
	if terrain != nil && !terrain.IsWalkable(newX, newZ) {
		// Stop if hit obstacle
		player.HasDestination = false
		player.IsMoving = false
		player.CurrentAction = entity.ActionIdle
		return
	}

	// Update position
	player.WorldX = newX
	player.WorldZ = newZ
	if terrain != nil {
		player.WorldY = terrain.GetHeight(newX, newZ)
	}

	// Update facing direction
	player.Direction = entity.CalculateDirection(dx, dz)
	player.IsMoving = true
	player.CurrentAction = entity.ActionWalk
}

// SetDestination sets the player's click-to-move destination.
func SetDestination(player *Player, worldX, worldZ float32) {
	if player == nil || player.Character == nil {
		return
	}
	player.DestX = worldX
	player.DestZ = worldZ
	player.HasDestination = true
}

// ClearDestination clears the player's current destination.
func ClearDestination(player *Player) {
	if player == nil || player.Character == nil {
		return
	}
	player.HasDestination = false
	player.IsMoving = false
	player.CurrentAction = entity.ActionIdle
}

// ArrivalThreshold is the distance at which a character is considered to have arrived.
const ArrivalThreshold = 1.0

// DefaultMoveSpeed is the default movement speed in world units per second.
// Korangar uses 150 as base movement speed.
const DefaultMoveSpeed = 150.0

// DiagonalSpeedMultiplier is applied to diagonal movement (sqrt(2) ≈ 1.414).
// Korangar uses 1.4 for diagonal path segments.
const DiagonalSpeedMultiplier = 1.4
