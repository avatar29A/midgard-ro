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
	// Logical actions, mirroring charsprite's. These say what to play, not
	// where a sprite keeps it: a monster's attack is set 2 of its ACT and a
	// player's is set 5, and charsprite.ActionIndex is what knows that.
	ActionIdle   = 0
	ActionWalk   = 1
	ActionPickup = 2
	ActionAttack = 3
	ActionHurt   = 4
	ActionDie    = 5

	// LoadedActions bounds the per-action tables here, matching charsprite's
	// count of logical actions so an index is valid in both.
	LoadedActions = 6
)

// Character represents a game character with position, movement, and animation state.
type Character struct {
	// Position in world coordinates
	WorldX float32
	WorldY float32 // Altitude (follows terrain)
	WorldZ float32

	// Render position (smoothly interpolated for visual display)
	RenderX float32
	RenderY float32
	RenderZ float32

	// Movement state
	IsMoving  bool
	Direction int     // 0-7: S, SW, W, NW, N, NE, E, SE
	MoveSpeed float32 // Units per second (free movement — see WalkSpeedMs for walks)

	// WalkSpeedMs is rAthena's `speed` stat: milliseconds per straight cell.
	// It drives server-path walking; MoveSpeed only covers free movement.
	WalkSpeedMs float32

	// TerrainHeight, when set, supplies the ground altitude at a world XZ so
	// the character follows terrain as it walks. Wired up by the game state.
	TerrainHeight func(worldX, worldZ float32) float32

	// offsetX/offsetZ hold the gap between where the character is drawn and
	// where the server says they are, bled off over time rather than snapped.
	offsetX float32
	offsetZ float32

	// sinceWalkMs counts time since the character last had a path, used to
	// hold the walk animation across the gap between consecutive steps.
	sinceWalkMs float32

	// playingOnce is set while a one-shot action — the pick-up motion — is
	// running. It outranks the movement state until it finishes or the
	// character starts moving.
	playingOnce bool

	// OnceAction is which action the one-shot is playing.
	OnceAction int

	// Dead holds the character on its death animation until it is revived.
	Dead bool

	// Server-authoritative cell path and progress along the current step.
	path        [][2]int
	pathIdx     int
	segElapsed  float32
	segDuration float32
	segFromX    float32
	segFromZ    float32
	segToX      float32
	segToZ      float32

	// Click-to-move destination
	DestX          float32 // Target X position
	DestZ          float32 // Target Z position
	HasDestination bool    // Whether moving to a destination

	// AnimIntervalMs overrides the default frame duration per action, taken
	// from the sprite's own ACT. Zero for an action means use the default.
	AnimIntervalMs [LoadedActions]float32

	// Animation state
	CurrentAction    int     // 0=Idle, 1=Walk
	CurrentFrame     int     // Current frame within action
	FrameTime        float32 // Accumulated time for frame timing (ms)
	LastCameraSector int     // Previous camera sector for sprite hysteresis (-1 = none)
}

// NewCharacter creates a new character at the given position.
func NewCharacter(x, y, z float32) *Character {
	return &Character{
		WorldX:           x,
		WorldY:           y,
		WorldZ:           z,
		RenderX:          x, // Initialize render position to match world position
		RenderY:          y,
		RenderZ:          z,
		Direction:        DirS,
		MoveSpeed:        DefaultFreeMoveSpeed,
		WalkSpeedMs:      DefaultWalkSpeedMs,
		LastCameraSector: -1, // No previous camera sector
	}
}

// DefaultFreeMoveSpeed is the velocity, in world units per second, that
// matches DefaultWalkSpeedMs. Free movement (keyboard nudging, tools) uses a
// velocity; server walks use the ms-per-cell duration instead.
const DefaultFreeMoveSpeed = CellSize * 1000.0 / DefaultWalkSpeedMs

// ArrivalThreshold is how close, in world units, counts as having reached a
// free-movement destination.
const ArrivalThreshold = 1.0

// SetPosition sets the character's world position.
// Also updates render position to prevent interpolation lag on teleport.
func (c *Character) SetPosition(x, y, z float32) {
	c.StopWalking()
	c.offsetX, c.offsetZ = 0, 0 // a teleport has nothing to catch up to
	c.WorldX = x
	c.WorldY = y
	c.WorldZ = z
	// Sync render position to prevent interpolation lag on teleport
	c.RenderX = x
	c.RenderY = y
	c.RenderZ = z
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
	c.StopWalking()
	c.HasDestination = false
	c.IsMoving = false
	c.CurrentAction = ActionIdle
}

// Update updates the character's position and animation state.
// deltaMs is the time since last update in milliseconds.
// Returns true if the character's state changed (for rendering updates).
func (c *Character) Update(deltaMs float32) bool {
	changed := false

	// A server path supersedes free movement: it carries its own timing and
	// drives the render position directly.
	if c.IsWalkingPath() {
		return c.UpdateWalk(deltaMs)
	}

	// Update movement towards destination
	if c.HasDestination {
		dx := c.DestX - c.WorldX
		dz := c.DestZ - c.WorldZ
		dist := sqrtf32(dx*dx + dz*dz)

		if dist < ArrivalThreshold {
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

// sectorToDirection maps a 45-degree sector, measured clockwise from +Z, to
// an RO direction index.
//
// The world runs +X east and +Z north — that is how the terrain mesh is built
// (internal/engine/terrain/mesh.go lays GND row y at Z = y*zoom, so low Z is
// south) and how rAthena's own dirx/diry tables read (+y north, +x east).
// So sector 0 (+Z) is north, and every 45 degrees clockwise from there steps
// through NE, E, SE, S, SW, W, NW.
var sectorToDirection = [8]int{DirN, DirNE, DirE, DirSE, DirS, DirSW, DirW, DirNW}

// CalculateDirection converts a movement delta to an RO direction index.
//
// This used to start the table at DirS on the belief that +Z was south, which
// pointed every facing 180 degrees the wrong way. Callers compensated by
// negating their deltas; those negations are gone now that the table is right.
func CalculateDirection(dx, dz float32) int {
	// Calculate angle in radians (atan2 gives -PI to PI), measured clockwise
	// from +Z toward +X.
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

	return sectorToDirection[sector]
}

// sqrtf32 computes the square root of a float32.
func sqrtf32(x float32) float32 {
	return float32(gomath.Sqrt(float64(x)))
}

// RenderLerpSpeed controls how fast render position catches up to world position.
// This is units per second - higher = snappier, lower = smoother.
// Korangar uses linear interpolation based on movement timestamps.
// We use a fixed catch-up speed for smooth keyboard movement.
const RenderLerpSpeed = 500.0

// UpdateRenderPosition smoothly interpolates render position toward world position.
// Uses linear interpolation with fixed speed (Korangar-style).
// deltaMs is the time since last update in milliseconds.
func (c *Character) UpdateRenderPosition(deltaMs float32) {
	// Path walking interpolates the render position itself, on server
	// timing — a second catch-up pass here would fight it.
	if c.IsWalkingPath() {
		return
	}

	// Still owed a correction from the last path: keep bleeding it off so the
	// character settles onto the authoritative cell rather than stopping short.
	if c.offsetX != 0 || c.offsetZ != 0 {
		c.applyVisualOffset(c.WorldX, c.WorldZ, deltaMs)
		return
	}

	// Calculate distance to target
	dx := c.WorldX - c.RenderX
	dy := c.WorldY - c.RenderY
	dz := c.WorldZ - c.RenderZ
	dist := sqrtf32(dx*dx + dy*dy + dz*dz)

	if dist < 0.01 {
		// Close enough, snap to target
		c.RenderX = c.WorldX
		c.RenderY = c.WorldY
		c.RenderZ = c.WorldZ
		return
	}

	// Linear interpolation with fixed speed (Korangar-style)
	maxMove := RenderLerpSpeed * deltaMs / 1000.0
	if maxMove >= dist {
		// Can reach target this frame
		c.RenderX = c.WorldX
		c.RenderY = c.WorldY
		c.RenderZ = c.WorldZ
	} else {
		// Move towards target at fixed speed
		t := maxMove / dist
		c.RenderX += dx * t
		c.RenderY += dy * t
		c.RenderZ += dz * t
	}
}

// RenderPosition returns the smoothly interpolated render position.
func (c *Character) RenderPosition() (x, y, z float32) {
	return c.RenderX, c.RenderY, c.RenderZ
}
