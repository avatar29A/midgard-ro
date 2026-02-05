// Package states implements game state management.
package states

import (
	"fmt"
	"strings"
	"time"

	"github.com/Faultbox/midgard-ro/internal/logger"
	"github.com/Faultbox/midgard-ro/internal/network"
	"github.com/Faultbox/midgard-ro/internal/network/packets"
	"go.uber.org/zap"
)

// LoadingStateConfig contains configuration for the loading state.
type LoadingStateConfig struct {
	MapName    string
	SpawnX     int
	SpawnY     int
	SpawnDir   uint8
	CharID     uint32
	TexLoader  func(string) ([]byte, error) // Function to load textures from GRF
}

// LoadingState handles map loading before entering the game.
type LoadingState struct {
	config  LoadingStateConfig
	client  *network.Client
	manager *Manager

	// Loading progress
	StatusMsg     string
	ErrorMsg      string
	Progress      float32 // 0.0 to 1.0
	LoadingPhase  string
	IsComplete    bool

	// Loaded data (passed to InGame state)
	MapLoaded bool

	// Timing
	startTime time.Time
}

// NewLoadingState creates a new loading state.
func NewLoadingState(cfg LoadingStateConfig, client *network.Client, manager *Manager) *LoadingState {
	return &LoadingState{
		config:       cfg,
		client:       client,
		manager:      manager,
		StatusMsg:    "Loading map...",
		LoadingPhase: "init",
	}
}

// Enter is called when entering this state.
func (s *LoadingState) Enter() error {
	s.startTime = time.Now()
	s.ErrorMsg = ""
	s.Progress = 0
	s.IsComplete = false

	logger.Info("entering LoadingState", zap.String("map", s.config.MapName))

	// Register map server packet handlers
	s.client.RegisterHandler(packets.ZC_ACCEPT_ENTER, s.handleMapAccept)
	s.client.RegisterHandler(packets.ZC_ACCEPT_ENTER2, s.handleMapAccept) // Modern rAthena

	// Send map enter packet
	return s.sendMapEnter()
}

// Exit is called when leaving this state.
func (s *LoadingState) Exit() error {
	return nil
}

// Update is called every frame.
func (s *LoadingState) Update(dt float64) error {
	// Check for timeout
	if time.Since(s.startTime) > 60*time.Second {
		s.ErrorMsg = "Map loading timed out"
		return nil
	}

	// Process network
	if err := s.client.Process(); err != nil {
		s.ErrorMsg = fmt.Sprintf("Network error: %v", err)
	}

	// Simulate loading progress for visual feedback
	if !s.IsComplete && s.Progress < 0.95 {
		s.Progress += float32(dt) * 0.5 // Progress over ~2 seconds
		if s.Progress > 0.95 {
			s.Progress = 0.95
		}
	}

	// Transition to ingame when complete
	if s.IsComplete {
		s.Progress = 1.0
		s.transitionToInGame()
	}

	return nil
}

// Render is called every frame to draw the state.
func (s *LoadingState) Render() error {
	// UI rendering will be handled by the UI system
	return nil
}

// HandleInput processes input events.
func (s *LoadingState) HandleInput(event interface{}) error {
	return nil
}

func (s *LoadingState) sendMapEnter() error {
	accountID, loginID1, _, sex := s.client.Session()
	charID := s.client.CharID()

	logger.Debug("sending CZ_ENTER2 (0x0436)",
		zap.Uint32("accountID", accountID),
		zap.Uint32("charID", charID),
		zap.Uint32("loginID1", loginID1),
		zap.Uint8("sex", sex))

	// Use CZ_ENTER2 (0x0436) - Korangar format (23 bytes, no auth token)
	pkt := &packets.MapEnter2{
		PacketID:   packets.CZ_ENTER2,
		AccountID:  accountID,
		CharID:     charID,
		LoginID1:   loginID1,
		ClientTick: uint32(time.Now().UnixMilli() & 0xFFFFFFFF),
		Sex:        sex,
		// Unknown bytes are zero-initialized
	}

	s.StatusMsg = fmt.Sprintf("Entering map: %s", s.getDisplayMapName())
	s.LoadingPhase = "connecting"

	if err := s.client.Send(pkt.Encode()); err != nil {
		s.ErrorMsg = fmt.Sprintf("Failed to enter map: %v", err)
		return err
	}

	return nil
}

func (s *LoadingState) handleMapAccept(data []byte) error {
	logger.Debug("handleMapAccept called", zap.Int("dataLen", len(data)))

	accept := packets.DecodeMapAccept(data)
	if accept == nil {
		s.ErrorMsg = "Failed to parse map accept"
		logger.Error("failed to parse map accept", zap.Int("dataLen", len(data)))
		return fmt.Errorf("invalid map accept packet")
	}

	// Get spawn position
	x, y, dir := accept.GetPosition()
	s.config.SpawnX = x
	s.config.SpawnY = y
	s.config.SpawnDir = dir

	logger.Info("map enter accepted",
		zap.Int("x", x),
		zap.Int("y", y),
		zap.Uint8("dir", dir),
		zap.Uint32("startTime", accept.StartTime))

	s.StatusMsg = fmt.Sprintf("Spawning at (%d, %d)", x, y)
	s.LoadingPhase = "spawning"
	s.MapLoaded = true

	// Send loading complete notification
	s.sendLoadingComplete()

	s.IsComplete = true
	return nil
}

func (s *LoadingState) sendLoadingComplete() {
	pkt := &packets.LoadingComplete{
		PacketID: packets.CZ_NOTIFY_ACTORINIT,
	}
	s.client.Send(pkt.Encode())
}

func (s *LoadingState) transitionToInGame() {
	s.manager.Change(NewInGameState(InGameStateConfig{
		MapName:   s.config.MapName,
		SpawnX:    s.config.SpawnX,
		SpawnY:    s.config.SpawnY,
		SpawnDir:  s.config.SpawnDir,
		CharID:    s.config.CharID,
		TexLoader: s.config.TexLoader,
	}, s.client, s.manager))
}

func (s *LoadingState) getDisplayMapName() string {
	// Remove .gat extension for display
	name := s.config.MapName
	if strings.HasSuffix(name, ".gat") {
		name = name[:len(name)-4]
	}
	return name
}

// GetStatusMessage returns the current status message.
func (s *LoadingState) GetStatusMessage() string {
	return s.StatusMsg
}

// GetErrorMessage returns the current error message.
func (s *LoadingState) GetErrorMessage() string {
	return s.ErrorMsg
}

// GetProgress returns the loading progress (0.0 to 1.0).
func (s *LoadingState) GetProgress() float32 {
	return s.Progress
}

// GetLoadingPhase returns the current loading phase.
func (s *LoadingState) GetLoadingPhase() string {
	return s.LoadingPhase
}

// GetMapName returns the map being loaded.
func (s *LoadingState) GetMapName() string {
	return s.getDisplayMapName()
}
