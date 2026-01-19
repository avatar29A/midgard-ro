// Package states implements game state management.
package states

import (
	"fmt"
	"time"

	"github.com/Faultbox/midgard-ro/internal/logger"
	"github.com/Faultbox/midgard-ro/internal/network"
	"github.com/Faultbox/midgard-ro/internal/network/packets"
	"go.uber.org/zap"
)

// CharSelectStateConfig contains configuration for character selection.
type CharSelectStateConfig struct {
	CharServerHost string
	CharServerPort int
}

// CharSelectState handles character selection.
type CharSelectState struct {
	config  CharSelectStateConfig
	client  *network.Client
	manager *Manager

	// Character data
	Characters    []*packets.CharInfo
	SelectedSlot  int
	MaxSlots      int
	AvailSlots    int

	// State
	IsLoading     bool
	ErrorMsg      string
	StatusMsg     string
	CharListReady bool

	// Map server info (after selection)
	MapServerIP   string
	MapServerPort uint16
	MapName       string
	CharID        uint32

	// Timing
	enterTime time.Time
}

// NewCharSelectState creates a new character select state.
func NewCharSelectState(cfg CharSelectStateConfig, client *network.Client, manager *Manager) *CharSelectState {
	return &CharSelectState{
		config:       cfg,
		client:       client,
		manager:      manager,
		SelectedSlot: -1,
		StatusMsg:    "Requesting character list...",
	}
}

// Enter is called when entering this state.
func (s *CharSelectState) Enter() error {
	s.enterTime = time.Now()
	s.ErrorMsg = ""
	s.IsLoading = true
	s.CharListReady = false
	s.Characters = nil

	// Register packet handlers
	s.client.RegisterHandler(packets.HC_ACCEPT_ENTER, s.handleCharListAccept)
	s.client.RegisterHandler(packets.HC_REFUSE_ENTER, s.handleCharListRefuse)
	s.client.RegisterHandler(packets.HC_NOTIFY_ZONESVR, s.handleMapServerInfo)
	s.client.RegisterHandler(packets.HC_NOTIFY_ZONESVR2, s.handleMapServerInfo) // Modern rAthena

	// Send character server enter request
	return s.sendCharEnter()
}

// Exit is called when leaving this state.
func (s *CharSelectState) Exit() error {
	return nil
}

// Update is called every frame.
func (s *CharSelectState) Update(dt float64) error {
	// Check for timeout
	if s.IsLoading && time.Since(s.enterTime) > 30*time.Second {
		s.ErrorMsg = "Timeout waiting for character list"
		s.IsLoading = false
		return nil
	}

	// Process network
	if err := s.client.Process(); err != nil {
		s.ErrorMsg = fmt.Sprintf("Network error: %v", err)
		s.IsLoading = false
	}

	return nil
}

// Render is called every frame to draw the state.
func (s *CharSelectState) Render() error {
	// UI rendering will be handled by the UI system
	return nil
}

// HandleInput processes input events.
func (s *CharSelectState) HandleInput(event interface{}) error {
	return nil
}

func (s *CharSelectState) sendCharEnter() error {
	accountID, loginID1, loginID2, sex := s.client.Session()

	logger.Debug("sending CH_ENTER",
		zap.Uint32("accountID", accountID),
		zap.Uint32("loginID1", loginID1),
		zap.Uint32("loginID2", loginID2),
		zap.Uint8("sex", sex))

	pkt := &packets.CharEnter{
		PacketID:  packets.CH_ENTER,
		AccountID: accountID,
		LoginID1:  loginID1,
		LoginID2:  loginID2,
		Sex:       sex,
	}

	if err := s.client.Send(pkt.Encode()); err != nil {
		s.ErrorMsg = fmt.Sprintf("Failed to send char enter: %v", err)
		s.IsLoading = false
		return err
	}

	return nil
}

func (s *CharSelectState) handleCharListAccept(data []byte) error {
	s.IsLoading = false

	charList := packets.DecodeCharSelectAccept(data)
	if charList == nil {
		s.ErrorMsg = "Failed to parse character list"
		return fmt.Errorf("invalid character list packet")
	}

	s.MaxSlots = int(charList.MaxSlots)
	s.AvailSlots = int(charList.AvailSlots)
	s.Characters = charList.Characters
	s.CharListReady = true

	if len(s.Characters) > 0 {
		s.StatusMsg = fmt.Sprintf("Found %d character(s)", len(s.Characters))
	} else {
		s.StatusMsg = "No characters found. Create a new character."
	}

	return nil
}

func (s *CharSelectState) handleCharListRefuse(data []byte) error {
	s.IsLoading = false

	errorCode := byte(0)
	if len(data) >= 3 {
		errorCode = data[2]
	}

	switch errorCode {
	case 0:
		s.ErrorMsg = "Session expired or invalid"
	case 1:
		s.ErrorMsg = "Character selection denied"
	default:
		s.ErrorMsg = fmt.Sprintf("Character server refused (code %d)", errorCode)
	}
	return nil
}

func (s *CharSelectState) handleMapServerInfo(data []byte) error {
	logger.Debug("handleMapServerInfo called", zap.Int("dataLen", len(data)))

	info := packets.DecodeMapServerInfo(data)
	if info == nil {
		s.ErrorMsg = "Failed to parse map server info"
		logger.Error("failed to parse map server info", zap.Int("dataLen", len(data)))
		return fmt.Errorf("invalid map server info packet")
	}

	logger.Info("map server info received",
		zap.String("map", info.GetMapName()),
		zap.String("ip", info.GetIP()),
		zap.Uint16("port", info.Port))

	s.MapServerIP = info.GetIP()
	s.MapServerPort = info.Port
	s.MapName = info.GetMapName()
	s.CharID = info.CharID

	// Store character ID in client
	s.client.SetCharID(info.CharID)

	s.StatusMsg = fmt.Sprintf("Connecting to map: %s", s.MapName)

	// Disconnect from char server before connecting to map server
	s.client.Disconnect()

	// Transition to connecting state for map server
	s.manager.Change(NewConnectingState(ConnectingStateConfig{
		NextState:  "ingame",
		ServerHost: s.MapServerIP,
		ServerPort: int(s.MapServerPort),
		MapName:    s.MapName,
	}, s.client, s.manager))

	return nil
}

// SelectCharacter selects a character by slot index and requests map server info.
func (s *CharSelectState) SelectCharacter(slotIndex int) error {
	if slotIndex < 0 || slotIndex >= len(s.Characters) {
		return fmt.Errorf("invalid slot index: %d", slotIndex)
	}

	s.SelectedSlot = slotIndex
	s.IsLoading = true
	s.StatusMsg = "Selecting character..."

	pkt := &packets.CharSelect{
		PacketID: packets.CH_SELECT_CHAR,
		Slot:     s.Characters[slotIndex].Slot,
	}

	if err := s.client.Send(pkt.Encode()); err != nil {
		s.ErrorMsg = fmt.Sprintf("Failed to select character: %v", err)
		s.IsLoading = false
		return err
	}

	return nil
}

// GetCharacters returns the list of characters.
func (s *CharSelectState) GetCharacters() []*packets.CharInfo {
	return s.Characters
}

// GetSelectedCharacter returns the currently selected character, if any.
func (s *CharSelectState) GetSelectedCharacter() *packets.CharInfo {
	if s.SelectedSlot >= 0 && s.SelectedSlot < len(s.Characters) {
		return s.Characters[s.SelectedSlot]
	}
	return nil
}

// GetStatusMessage returns the current status message.
func (s *CharSelectState) GetStatusMessage() string {
	return s.StatusMsg
}

// GetErrorMessage returns the current error message.
func (s *CharSelectState) GetErrorMessage() string {
	return s.ErrorMsg
}

// IsCharListReady returns whether the character list is ready.
func (s *CharSelectState) IsCharListReady() bool {
	return s.CharListReady
}

// IsLoadingState returns whether the state is currently loading.
func (s *CharSelectState) IsLoadingState() bool {
	return s.IsLoading
}
