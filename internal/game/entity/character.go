// Package entity provides game entities like characters, mobs, and NPCs.
package entity

import (
	gomath "math"
)

// Direction constants for 8-way movement (RO standard order).
const (
	DirS  = 0 // South (facing camera)
	DirSW = 1 // Southwest
	DirW  = 2 // West
	DirNW = 3 // Northwest
	DirN  = 4 // North (facing away)
	DirNE = 5 // Northeast
	DirE  = 6 // East
	DirSE = 7 // Southeast
)

// Action constants for character animations.
const (
	ActionIdle = 0
	ActionWalk = 1
)

// Character represents a game character with position, movement, and animation state.
type Character struct {
	// Position in world coordinates
	WorldX float32
	WorldY float32 // Altitude (follows terrain)
	WorldZ float32

	// Movement state
	IsMoving  bool
	Direction int     // 0-7: S, SW, W, NW, N, NE, E, SE
	MoveSpeed float32 // Units per second

	// Click-to-move destination
	DestX          float32 // Target X position
	DestZ          float32 // Target Z position
	HasDestination bool    // Whether moving to a destination

	// Animation state
	CurrentAction int     // 0=Idle, 1=Walk
	CurrentFrame  int     // Current frame within action
	FrameTime     float32 // Accumulated time for frame timing (ms)
	LastVisualDir int     // Previous visual direction for hysteresis (-1 = none)
}

// NewCharacter creates a new character at the given position.
func NewCharacter(x, y, z float32) *Character {
	return &Character{
		WorldX:        x,
		WorldY:        y,
		WorldZ:        z,
		Direction:     DirS,
		MoveSpeed:     150.0, // Default movement speed
		LastVisualDir: -1,    // No previous direction
	}
}

// SetPosition sets the character's world position.
func (c *Character) SetPosition(x, y, z float32) {
	c.WorldX = x
	c.WorldY = y
	c.WorldZ = z
}

// Position returns the character's world position.
func (c *Character) Position() (x, y, z float32) {
	return c.WorldX, c.WorldY, c.WorldZ
}

// SetDestination sets a click-to-move destination.
func (c *Character) SetDestination(x, z float32) {
	c.DestX = x
	c.DestZ = z
	c.HasDestination = true
}

// ClearDestination clears the current destination.
func (c *Character) ClearDestination() {
	c.HasDestination = false
	c.IsMoving = false
	c.CurrentAction = ActionIdle
}

// Update updates the character's position and animation state.
// deltaMs is the time since last update in milliseconds.
// Returns true if the character's state changed (for rendering updates).
func (c *Character) Update(deltaMs float32) bool {
	changed := false

	// Update movement towards destination
	if c.HasDestination {
		dx := c.DestX - c.WorldX
		dz := c.DestZ - c.WorldZ
		dist := sqrtf32(dx*dx + dz*dz)

		arrivalThreshold := float32(5.0) // Arrival distance
		if dist < arrivalThreshold {
			// Arrived at destination
			c.HasDestination = false
			c.IsMoving = false
			c.CurrentAction = ActionIdle
			changed = true
		} else {
			// Move towards destination
			moveAmount := c.MoveSpeed * deltaMs / 1000.0
			if moveAmount > dist {
				moveAmount = dist
			}
			c.WorldX += (dx / dist) * moveAmount
			c.WorldZ += (dz / dist) * moveAmount
			c.IsMoving = true
			c.CurrentAction = ActionWalk

			// Update direction based on movement
			c.Direction = CalculateDirection(dx, dz)
			changed = true
		}
	}

	return changed
}

// UpdateWithVelocity updates character position based on velocity input.
// vx, vz are velocity components (normalized -1 to 1).
// deltaMs is the time since last update in milliseconds.
func (c *Character) UpdateWithVelocity(vx, vz float32, deltaMs float32) {
	// Calculate speed based on velocity magnitude
	speed := sqrtf32(vx*vx + vz*vz)
	if speed < 0.01 {
		// No movement
		if c.IsMoving {
			c.IsMoving = false
			c.CurrentAction = ActionIdle
		}
		return
	}

	// Normalize and apply movement
	moveAmount := c.MoveSpeed * deltaMs / 1000.0
	c.WorldX += vx * moveAmount
	c.WorldZ += vz * moveAmount
	c.IsMoving = true
	c.CurrentAction = ActionWalk

	// Update direction based on movement direction
	c.Direction = CalculateDirection(vx, vz)
}

// CalculateDirection converts a movement delta to an RO direction index.
func CalculateDirection(dx, dz float32) int {
	// Calculate angle in radians (atan2 gives -PI to PI)
	angle := gomath.Atan2(float64(dx), float64(dz))

	// Convert to 0-2*PI range
	if angle < 0 {
		angle += 2 * gomath.Pi
	}

	// Divide circle into 8 sectors (each 45 degrees = PI/4)
	// Add PI/8 offset to center each sector
	sector := int((angle + gomath.Pi/8) / (gomath.Pi / 4))
	if sector >= 8 {
		sector = 0
	}

	// Map sectors to RO direction order
	// angle=0 is +Z direction (South in RO terms = facing camera)
	// Clockwise: S(0), SE(7), E(6), NE(5), N(4), NW(3), W(2), SW(1)
	directionMap := []int{DirS, DirSE, DirE, DirNE, DirN, DirNW, DirW, DirSW}
	return directionMap[sector]
}

// GetVisualDirection returns the direction to display, applying hysteresis
// to prevent visual flickering when moving near direction boundaries.
func (c *Character) GetVisualDirection() int {
	if !c.IsMoving {
		return c.Direction
	}

	// Apply hysteresis: keep previous direction unless significantly different
	if c.LastVisualDir >= 0 {
		// Calculate angular difference (handling wrap-around)
		diff := c.Direction - c.LastVisualDir
		if diff < 0 {
			diff = -diff
		}
		if diff > 4 {
			diff = 8 - diff // Shorter path around the circle
		}
		// Only change if more than 1 direction step (>45 degrees)
		if diff <= 1 {
			return c.LastVisualDir
		}
	}

	c.LastVisualDir = c.Direction
	return c.Direction
}

// sqrtf32 computes the square root of a float32.
func sqrtf32(x float32) float32 {
	return float32(gomath.Sqrt(float64(x)))
}
