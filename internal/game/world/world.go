// Package world handles map loading and management.
package world

import (
	"fmt"

	"github.com/Faultbox/midgard-ro/pkg/formats"
	"github.com/Faultbox/midgard-ro/pkg/math"
)

// Map represents a loaded game map.
type Map struct {
	Name   string
	Width  int
	Height int

	// Collision/walkability data
	GAT *formats.GAT

	// Ground mesh data
	GND *formats.GND

	// Resource/object data
	RSW *formats.RSW
}

// NewMap creates a new map.
func NewMap(name string) *Map {
	return &Map{
		Name: name,
	}
}

// Load loads map data from assets.
// TODO: This will use the asset manager to load from GRF.
func (m *Map) Load() error {
	// TODO: Load GAT file (data/name.gat)
	// TODO: Load GND file (data/name.gnd)
	// TODO: Load RSW file (data/name.rsw)
	return fmt.Errorf("map loading not implemented")
}

// IsWalkable checks if a position is walkable.
func (m *Map) IsWalkable(x, y int) bool {
	if m.GAT == nil {
		return false
	}
	return m.GAT.IsWalkable(x, y)
}

// WorldToCell converts world coordinates to cell coordinates.
func (m *Map) WorldToCell(pos math.Vec3) (int, int) {
	// RO uses a specific coordinate system
	// TODO: Implement proper conversion
	return int(pos.X), int(pos.Z)
}

// CellToWorld converts cell coordinates to world coordinates.
func (m *Map) CellToWorld(x, y int) math.Vec3 {
	// TODO: Implement proper conversion
	return math.Vec3{X: float32(x), Y: 0, Z: float32(y)}
}

// Manager manages the current map and map transitions.
type Manager struct {
	current *Map
	loading bool
}

// NewManager creates a new world manager.
func NewManager() *Manager {
	return &Manager{}
}

// Current returns the current map.
func (m *Manager) Current() *Map {
	return m.current
}

// LoadMap loads a map by name.
func (m *Manager) LoadMap(name string) error {
	m.loading = true
	defer func() { m.loading = false }()

	newMap := NewMap(name)
	if err := newMap.Load(); err != nil {
		return fmt.Errorf("loading map %s: %w", name, err)
	}

	m.current = newMap
	return nil
}

// IsLoading returns whether a map is currently loading.
func (m *Manager) IsLoading() bool {
	return m.loading
}
