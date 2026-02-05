// Package states implements game state management.
package states

import (
	"fmt"
	"time"

	"github.com/Faultbox/midgard-ro/internal/network"
)

// ConnectingStateConfig contains configuration for the connecting state.
type ConnectingStateConfig struct {
	NextState   string // State to transition to after connecting
	ServerHost  string // Server to connect to (for char/map server)
	ServerPort  int
	Timeout     time.Duration
	MapName     string // Map name (for ingame transition)
}

// ConnectingState handles connection transitions between servers.
type ConnectingState struct {
	config    ConnectingStateConfig
	client    *network.Client
	manager   *Manager

	// Connection state
	startTime time.Time
	connected bool
	ErrorMsg  string

	// Display
	StatusMsg string
}

// NewConnectingState creates a new connecting state.
func NewConnectingState(cfg ConnectingStateConfig, client *network.Client, manager *Manager) *ConnectingState {
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}
	return &ConnectingState{
		config:    cfg,
		client:    client,
		manager:   manager,
		StatusMsg: "Connecting...",
	}
}

// Enter is called when entering this state.
func (s *ConnectingState) Enter() error {
	s.startTime = time.Now()
	s.connected = false
	s.ErrorMsg = ""

	// If we need to connect to a new server
	if s.config.ServerHost != "" {
		s.StatusMsg = fmt.Sprintf("Connecting to %s:%d...", s.config.ServerHost, s.config.ServerPort)
		go s.connect()
	} else {
		// Already connected, just transitioning state
		s.connected = true
	}

	return nil
}

// Exit is called when leaving this state.
func (s *ConnectingState) Exit() error {
	return nil
}

// Update is called every frame.
func (s *ConnectingState) Update(dt float64) error {
	// Check timeout
	if time.Since(s.startTime) > s.config.Timeout {
		s.ErrorMsg = "Connection timed out"
		return nil
	}

	// If connected, transition to next state
	if s.connected {
		return s.transitionToNextState()
	}

	// Process network
	if err := s.client.Process(); err != nil {
		s.ErrorMsg = fmt.Sprintf("Network error: %v", err)
	}

	return nil
}

// Render is called every frame to draw the state.
func (s *ConnectingState) Render() error {
	// UI rendering will be handled by the UI system
	return nil
}

// HandleInput processes input events.
func (s *ConnectingState) HandleInput(event interface{}) error {
	return nil
}

func (s *ConnectingState) connect() {
	// Determine server type
	serverType := network.ServerChar
	if s.config.NextState == "ingame" {
		serverType = network.ServerMap
	}

	err := s.client.Connect(s.config.ServerHost, s.config.ServerPort, serverType)
	if err != nil {
		s.ErrorMsg = fmt.Sprintf("Connection failed: %v", err)
		return
	}

	s.connected = true
	s.StatusMsg = "Connected!"
}

func (s *ConnectingState) transitionToNextState() error {
	switch s.config.NextState {
	case "charselect":
		// Transition to character select state
		s.manager.Change(NewCharSelectState(CharSelectStateConfig{
			CharServerHost: s.config.ServerHost,
			CharServerPort: s.config.ServerPort,
		}, s.client, s.manager))
		return nil
	case "ingame":
		// Transition to loading state for map loading
		s.manager.Change(NewLoadingState(LoadingStateConfig{
			MapName: s.config.MapName,
			CharID:  s.client.CharID(),
		}, s.client, s.manager))
		return nil
	default:
		return fmt.Errorf("unknown next state: %s", s.config.NextState)
	}
}

// GetStatusMessage returns the current status message.
func (s *ConnectingState) GetStatusMessage() string {
	return s.StatusMsg
}

// GetErrorMessage returns the current error message.
func (s *ConnectingState) GetErrorMessage() string {
	return s.ErrorMsg
}
