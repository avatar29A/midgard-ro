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
)

// Entity represents a game entity.
type Entity struct {
	ID        uint32
	Type      Type
	Name      string
	Position  math.Vec3
	Direction uint8 // 0-7 for 8 directions

	// Visual
	SpriteID int
	// TODO: Add sprite/animation reference

	// Stats (for players/monsters)
	Level int
	HP    int
	MaxHP int
	SP    int
	MaxSP int

	// Movement
	MoveSpeed float64
	MovePath  []math.Vec2
}

// NewEntity creates a new entity.
func NewEntity(id uint32, entityType Type) *Entity {
	return &Entity{
		ID:        id,
		Type:      entityType,
		MoveSpeed: 1.0,
	}
}

// Update updates the entity.
func (e *Entity) Update(dt float64) {
	// TODO: Process movement
	// TODO: Update animation
}

// Manager manages all entities in the game.
type Manager struct {
	entities map[uint32]*Entity
	player   *Entity // Reference to local player
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
	m.Add(e)
}

// Player returns the local player.
func (m *Manager) Player() *Entity {
	return m.player
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
