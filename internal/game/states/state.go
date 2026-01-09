// Package states implements game state management.
package states

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

// Manager manages game state transitions.
type Manager struct {
	current State
	next    State
}

// NewManager creates a new state manager.
func NewManager() *Manager {
	return &Manager{}
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
