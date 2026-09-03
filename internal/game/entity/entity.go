// Package entity implements game entities (players, monsters, NPCs).
package entity

import (
	"github.com/Faultbox/midgard-ro/pkg/math"
)

// Type represents the type of entity.
type Type uint8

const (
	TypePlayer Type = iota
	TypeMonster
	TypeNPC
	TypeItem
	TypeSkillEffect
	TypeWarp
	TypePortal
)

// State represents the current state of an entity.
type State uint8

const (
	StateIdle State = iota
	StateWalking
	StateSitting
	StateDead
	StateAttacking
	StateCasting
	StatePickingUp
)

// Entity represents a game entity.
type Entity struct {
	ID        uint32
	Type      Type
	Name      string
	Position  math.Vec3
	Direction uint8 // 0-7 for 8 directions
	State     State

	// ItemID is what a TypeItem entity is — an item lying on the ground is
	// the one kind identified by an item id rather than by a job, and it is
	// what names both its sprite and its label.
	ItemID uint32

	// Amount is how many are in a ground item's pile. Zero for everything
	// else.
	Amount int

	// Visual
	SpriteID     int  // Base sprite ID (job ID for players, monster ID for mobs)
	HeadSprite   int  // Head sprite for players
	Weapon       int  // Weapon sprite
	Shield       int  // Shield sprite
	HeadTop      int  // Headgear top
	HeadMid      int  // Headgear mid
	HeadBottom   int  // Headgear bottom
	Robe         int  // Garment/robe
	Female       bool // Selects the sprite folder and filename suffix
	HairStyle    int  // Hair style
	HairColor    int  // Hair color
	ClothesColor int  // Clothes color
	BodyPalette  int  // Body palette

	// Display properties
	ShowHP      bool       // Whether to show HP bar
	ShowName    bool       // Whether to show name
	NameColor   [4]float32 // Name display color (RGBA)
	GuildName   string     // Guild name (for players)
	GuildEmblem int        // Guild emblem ID
	Title       string     // Title/party name

	// Stats (for players/monsters)
	Level int
	HP    int
	MaxHP int
	SP    int
	MaxSP int
	Job   int // Job/class ID

	// Movement
	MoveSpeed     float64
	MovePath      []math.Vec2
	MoveStartTime float64 // When movement started
	MoveEndTime   float64 // When movement should end

	// Animation
	AnimAction int     // Current animation action
	AnimFrame  int     // Current frame
	AnimTime   float64 // Time in current animation
	AnimSpeed  float64 // Animation speed multiplier

	// Combat
	AttackSpeed int    // Attack speed (ASPD)
	AttackRange int    // Attack range
	TargetID    uint32 // Current target

	// SoundFrame is the animation frame this unit's last noise was made on,
	// so a walk cycle sounds once a step rather than once a frame. Minus one
	// when it is not making any.
	SoundFrame int

	// BodyState is rAthena's opt1: petrified, frozen, stunned or asleep, one
	// at a time. These are drawn on the unit rather than shown as an icon,
	// which is why it is here and not with the rest of the statuses.
	BodyState uint16

	// Flags
	IsVisible    bool
	IsTargetable bool
	IsDead       bool

	// Leaving marks a unit the server has removed but which is still fading
	// out, and FadeMs is how far through the current fade it is.
	Leaving bool
	FadeMs  float32

	// Body carries position, facing and animation for entities that are drawn
	// on the map, and is nil for those that are not. It is a separate type
	// because walking is the same problem for every unit — cell paths, step
	// timing, terrain following — and the local player already solves it.
	Body *Character
}

// NewEntity creates a new entity.
func NewEntity(id uint32, entityType Type) *Entity {
	e := &Entity{
		ID:           id,
		Type:         entityType,
		MoveSpeed:    1.0,
		AnimSpeed:    1.0,
		IsVisible:    true,
		IsTargetable: true,
		NameColor:    [4]float32{1, 1, 1, 1}, // White by default
	}

	// Set default display properties based on type
	switch entityType {
	case TypePlayer:
		e.ShowHP = false // Only show HP when damaged
		e.ShowName = true
	case TypeMonster:
		e.ShowHP = true
		e.ShowName = true
		e.NameColor = [4]float32{1, 0.8, 0.2, 1} // Yellow for monsters
	case TypeNPC:
		e.ShowHP = false
		e.ShowName = true
		e.NameColor = [4]float32{0.5, 1, 0.5, 1} // Green for NPCs
		e.IsTargetable = false
	case TypeItem:
		e.ShowHP = false
		e.ShowName = true
		e.NameColor = [4]float32{0.7, 0.7, 1, 1} // Light blue for items
		e.IsTargetable = false
	}

	return e
}

// FadeDurationMs is how long a unit takes to fade in when it appears and out
// when it leaves.
//
// The server only tells us about units within its area_size, so they cross
// that boundary abruptly as the player moves — which reads as units blinking
// in and out of nothing. The boundary is the server's to move; what the client
// can do is stop the crossing being instant.
const FadeDurationMs = 220.0

// Alpha is how opaque the entity should be drawn, ramping in as it appears and
// out as it leaves.
func (e *Entity) Alpha() float32 {
	progress := e.FadeMs / FadeDurationMs
	if progress > 1 {
		progress = 1
	}
	if progress < 0 {
		progress = 0
	}
	if e.Leaving {
		return 1 - progress
	}
	return progress
}

// FadedOut reports whether a leaving entity has finished fading and can be
// dropped.
func (e *Entity) FadedOut() bool {
	return e.Leaving && e.FadeMs >= FadeDurationMs
}

// BeginLeaving starts the fade out, from however opaque the entity is now, so
// a unit that leaves while still fading in does not jump to fully visible
// first.
func (e *Entity) BeginLeaving() {
	if e.Leaving {
		return
	}
	e.FadeMs = (1 - e.Alpha()) * FadeDurationMs
	e.Leaving = true
}

// CancelLeaving returns an entity the server has mentioned again to fading in,
// picking up from its current opacity.
func (e *Entity) CancelLeaving() {
	if !e.Leaving {
		return
	}
	e.FadeMs = e.Alpha() * FadeDurationMs
	e.Leaving = false
}

// SetPosition sets the entity position.
func (e *Entity) SetPosition(x, y, z float32) {
	e.Position.X = x
	e.Position.Y = y
	e.Position.Z = z
}

// GetPosition returns the entity position.
func (e *Entity) GetPosition() (x, y, z float32) {
	return e.Position.X, e.Position.Y, e.Position.Z
}

// HPPercent returns HP as a percentage (0.0 to 1.0).
func (e *Entity) HPPercent() float32 {
	if e.MaxHP <= 0 {
		return 1.0
	}
	return float32(e.HP) / float32(e.MaxHP)
}

// SPPercent returns SP as a percentage (0.0 to 1.0).
func (e *Entity) SPPercent() float32 {
	if e.MaxSP <= 0 {
		return 1.0
	}
	return float32(e.SP) / float32(e.MaxSP)
}

// IsAlive returns whether the entity is alive.
func (e *Entity) IsAlive() bool {
	return !e.IsDead && e.HP > 0
}

// TakeDamage applies damage to the entity.
func (e *Entity) TakeDamage(damage int) {
	e.HP -= damage
	if e.HP <= 0 {
		e.HP = 0
		e.IsDead = true
		e.State = StateDead
	}
	// Show HP bar when damaged
	if e.Type == TypePlayer {
		e.ShowHP = true
	}
}

// Heal restores HP to the entity.
func (e *Entity) Heal(amount int) {
	e.HP += amount
	if e.HP > e.MaxHP {
		e.HP = e.MaxHP
	}
	// Hide HP bar when fully healed (for players)
	if e.Type == TypePlayer && e.HP >= e.MaxHP {
		e.ShowHP = false
	}
}

// Update updates the entity state and animation.
func (e *Entity) Update(dt float64) {
	// Update animation time
	e.AnimTime += dt * e.AnimSpeed

	// Update state based on conditions
	if e.IsDead && e.State != StateDead {
		e.State = StateDead
		e.AnimAction = 0 // Death animation
		e.AnimFrame = 0
		e.AnimTime = 0
	}
}

// Manager manages all entities in the game.
type Manager struct {
	entities map[uint32]*Entity
	player   *Entity // Reference to local player
	playerID uint32  // Player entity ID
}

// NewManager creates a new entity manager.
func NewManager() *Manager {
	return &Manager{
		entities: make(map[uint32]*Entity),
	}
}

// Add adds an entity.
func (m *Manager) Add(e *Entity) {
	m.entities[e.ID] = e
}

// Remove removes an entity.
func (m *Manager) Remove(id uint32) {
	delete(m.entities, id)
}

// Get returns an entity by ID.
func (m *Manager) Get(id uint32) *Entity {
	return m.entities[id]
}

// SetPlayer sets the local player entity.
func (m *Manager) SetPlayer(e *Entity) {
	m.player = e
	m.playerID = e.ID
	m.Add(e)
}

// Player returns the local player.
func (m *Manager) Player() *Entity {
	return m.player
}

// PlayerID returns the local player's entity ID.
func (m *Manager) PlayerID() uint32 {
	return m.playerID
}

// SetVitals updates what a unit's bar shows.
//
// The spawn packets fill HP and MaxHP once, when a unit first appears. Nothing
// refreshes them after that until the combat packets land (Track F), which is
// what this exists for: damage and healing handlers call this and the bars
// follow, with no change to anything that draws them.
//
// Reports whether the unit was found, so a caller can tell an update that
// landed from one for a unit that has already despawned.
func (m *Manager) SetVitals(id uint32, hp, maxHP int) bool {
	e := m.entities[id]
	if e == nil {
		return false
	}

	e.HP = hp
	e.MaxHP = maxHP

	return true
}

// Update updates all entities.
func (m *Manager) Update(dt float64) {
	for _, e := range m.entities {
		e.Update(dt)
	}
}

// All returns all entities.
func (m *Manager) All() []*Entity {
	result := make([]*Entity, 0, len(m.entities))
	for _, e := range m.entities {
		result = append(result, e)
	}
	return result
}

// AllVisible returns all visible entities.
func (m *Manager) AllVisible() []*Entity {
	result := make([]*Entity, 0, len(m.entities))
	for _, e := range m.entities {
		if e.IsVisible {
			result = append(result, e)
		}
	}
	return result
}

// GetByType returns all entities of a specific type.
func (m *Manager) GetByType(entityType Type) []*Entity {
	result := make([]*Entity, 0)
	for _, e := range m.entities {
		if e.Type == entityType {
			result = append(result, e)
		}
	}
	return result
}

// Count returns the total number of entities.
func (m *Manager) Count() int {
	return len(m.entities)
}

// CountByType returns the number of entities of a specific type.
func (m *Manager) CountByType(entityType Type) int {
	count := 0
	for _, e := range m.entities {
		if e.Type == entityType {
			count++
		}
	}
	return count
}

// Clear removes all entities except the player.
func (m *Manager) Clear() {
	for id := range m.entities {
		if id != m.playerID {
			delete(m.entities, id)
		}
	}
}

// ClearAll removes all entities including the player.
func (m *Manager) ClearAll() {
	m.entities = make(map[uint32]*Entity)
	m.player = nil
	m.playerID = 0
}
