// Package states implements game state management.
package states

import (
	"github.com/Faultbox/midgard-ro/internal/engine/charsprite"
	"github.com/Faultbox/midgard-ro/internal/game/entity"
	"github.com/Faultbox/midgard-ro/internal/network/packets"
)

// State represents a game state (login, character select, in-game, etc.)
type State interface {
	// Enter is called when entering this state.
	Enter() error

	// Exit is called when leaving this state.
	Exit() error

	// Update is called every frame.
	Update(dt float64) error

	// Render is called every frame to draw the state.
	Render() error

	// HandleInput processes input events.
	HandleInput(event interface{}) error
}

// TexLoaderFunc is a function that loads asset data from GRF.
type TexLoaderFunc func(path string) ([]byte, error)

// Session holds the data for the character we're playing, which outlives the
// individual states that discover it. Character select fills it in; the
// in-game state reads walk speed, job and appearance back out of it.
type Session struct {
	Char *packets.CharInfo
}

// WalkSpeedMs returns the character's `speed` stat (milliseconds per cell),
// falling back to rAthena's default when the value is missing or outside the
// range a real character can have.
func (s *Session) WalkSpeedMs() float32 {
	if s == nil || s.Char == nil {
		return entity.DefaultWalkSpeedMs
	}
	speed := float32(s.Char.WalkSpeed)
	if speed < 20 || speed > 2000 {
		return entity.DefaultWalkSpeedMs
	}
	return speed
}

// SpriteSpec returns which character sprites to load for this session,
// defaulting to a male Novice when we have no character data yet.
//
// Sex comes from CharInfo.Sex, where rAthena sends 0 for female and 1 for
// male; anything else is treated as male.
func (s *Session) SpriteSpec() charsprite.Spec {
	if s == nil || s.Char == nil {
		return charsprite.Spec{Job: charsprite.FallbackJob, HairStyle: 1}
	}
	return charsprite.Spec{
		Job:       int(s.Char.Class),
		Female:    s.Char.Sex == 0,
		HairStyle: int(s.Char.HairStyle),
	}
}

// Manager manages game state transitions.
type Manager struct {
	current   State
	next      State
	TexLoader TexLoaderFunc

	// Session data for the character being played.
	Session Session

	// AutoPlay walks the login and character select screens without input, so
	// anything past them can be checked unattended. Set from --autologin.
	AutoPlay bool
}

// NewManager creates a new state manager.
func NewManager() *Manager {
	return &Manager{}
}

// SetTexLoader sets the texture loader function.
func (m *Manager) SetTexLoader(loader TexLoaderFunc) {
	m.TexLoader = loader
}

// Current returns the current state.
func (m *Manager) Current() State {
	return m.current
}

// Change schedules a state change.
func (m *Manager) Change(next State) {
	m.next = next
}

// Update processes state changes and updates current state.
func (m *Manager) Update(dt float64) error {
	// Handle state transition
	if m.next != nil {
		if m.current != nil {
			if err := m.current.Exit(); err != nil {
				return err
			}
		}
		m.current = m.next
		m.next = nil
		if err := m.current.Enter(); err != nil {
			return err
		}
	}

	// Update current state
	if m.current != nil {
		return m.current.Update(dt)
	}
	return nil
}

// Render renders the current state.
func (m *Manager) Render() error {
	if m.current != nil {
		return m.current.Render()
	}
	return nil
}
