// Package states implements game state management.
package states

import (
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/Faultbox/midgard-ro/internal/logger"
	"github.com/Faultbox/midgard-ro/internal/network"
	"github.com/Faultbox/midgard-ro/internal/network/packets"
	"github.com/Faultbox/midgard-ro/internal/trace"
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
	Characters   []*packets.CharInfo
	SelectedSlot int
	MaxSlots     int

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

	// autoSelected keeps unattended character entry to one attempt.
	autoSelected bool

	// CreatableSlots is how many characters this account may create, from
	// HC_ACCEPT_ENTER2. Zero until that packet arrives.
	//
	// Not the same as MaxSlots: that is MAX_CHARS, the server's compile-time
	// ceiling (15 on a stock build), while this is what the account itself
	// may use (9 on ours). Creating past it is refused.
	CreatableSlots int

	// CreateSlot is the slot the player asked to create a character in, or
	// -1. Set by RequestCreate.
	CreateSlot int
}

// NewCharSelectState creates a new character select state.
func NewCharSelectState(cfg CharSelectStateConfig, client *network.Client, manager *Manager) *CharSelectState {
	return &CharSelectState{
		config:       cfg,
		client:       client,
		manager:      manager,
		SelectedSlot: -1,
		CreateSlot:   -1,
		StatusMsg:    "Requesting character list...",
	}
}

// Enter is called when entering this state.
func (s *CharSelectState) Enter() error {
	s.manager.PlayFallbackBGM()

	s.enterTime = time.Now()
	s.ErrorMsg = ""
	s.IsLoading = true
	s.CharListReady = false
	s.Characters = nil

	// Register packet handlers
	s.client.RegisterHandler(packets.HC_ACCEPT_ENTER2, s.handleSlotCounts)
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

	// Unattended character entry, the other half of --autologin. Waits for the
	// list to arrive, then takes the first character; with no characters there
	// is nothing to enter and the screen is left as it is.
	if s.manager.AutoPlay && !s.manager.StopAtCharSelect &&
		!s.autoSelected && s.CharListReady && len(s.Characters) > 0 {
		s.autoSelected = true
		_ = s.SelectCharacter(0)
	}

	// Open creation for a slot that was asked for. Done here rather than in
	// the click handler so the state change happens between frames, with the
	// screen that requested it no longer mid-draw.
	if s.CreateSlot >= 0 {
		slot := s.CreateSlot
		s.CreateSlot = -1

		s.manager.Change(NewCharCreateState(slot, s.client, s.manager, s))
	}

	return nil
}

// handleSlotCounts reads how many characters this account may create.
//
// It arrives before the character list and is the only packet that carries the
// number; without it the client would have to guess, and guessing MAX_CHARS
// offers six slots the server will refuse.
func (s *CharSelectState) handleSlotCounts(data []byte) error {
	counts, ok := packets.DecodeSlotCounts(data)
	if !ok {
		logger.Warn("could not read the account's slot counts",
			zap.Int("bytes", len(data)))

		return nil
	}

	s.CreatableSlots = int(counts.Producible)

	trace.Emit(trace.Char, "slot-counts",
		zap.Uint8("normal", counts.Normal),
		zap.Uint8("premium", counts.Premium),
		zap.Uint8("billing", counts.Billing),
		zap.Uint8("producible", counts.Producible),
		zap.Uint8("total", counts.Total))

	return nil
}

// creatableSlots is how many slots creation may use.
//
// Falls back to MaxSlots when HC_ACCEPT_ENTER2 has not been seen, which is
// the pre-20130000 case and should not happen on our server. The fallback is
// permissive rather than zero: refusing every slot because a packet was
// missing would look exactly like the click doing nothing.
func (s *CharSelectState) creatableSlots() int {
	if s.CreatableSlots > 0 {
		return s.CreatableSlots
	}

	return s.MaxSlots
}

// RequestCreate records that the player asked to create a character in a
// slot, by double-clicking it or by pressing Make.
//
// Step 1 of #109 goes no further than recording it: the creation screen is
// step 2, and having the slot land here first means the click path can be
// proved on its own rather than only through a screen that does not exist yet.
func (s *CharSelectState) RequestCreate(slot int) {
	if limit := s.creatableSlots(); slot < 0 || slot >= limit {
		logger.Warn("asked to create in a slot the account may not use",
			zap.Int("slot", slot), zap.Int("creatable", limit))

		return
	}

	if slot < len(s.Characters) {
		// Not an error: the UI only offers this on an empty slot, so reaching
		// here means the two disagree about what is empty.
		logger.Warn("asked to create in a slot that already holds a character",
			zap.Int("slot", slot))

		return
	}

	s.CreateSlot = slot

	trace.Emit(trace.Char, "create-requested", zap.Int("slot", slot))
	logger.Info("character creation requested", zap.Int("slot", slot))
}

// CreatableSlotCount is how many slots this account may use, for the screen
// that pages over them.
func (s *CharSelectState) CreatableSlotCount() int {
	return s.creatableSlots()
}

// ClearPendingCreate forgets a creation request, so returning to this screen
// does not immediately open the creation one again.
func (s *CharSelectState) ClearPendingCreate() {
	s.CreateSlot = -1
}

// PendingCreateSlot returns the slot creation was asked for, or -1.
func (s *CharSelectState) PendingCreateSlot() int {
	return s.CreateSlot
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
	s.Characters = charList.Characters
	s.CharListReady = true

	// The slot count gates every creation, and it comes off the wire rather
	// than from anything we control. A zero here would refuse every empty
	// slot and look like the click having done nothing.
	trace.Emit(trace.Char, "list",
		zap.Int("characters", len(s.Characters)),
		zap.Int("maxSlots", s.MaxSlots),
		zap.Int("creatable", s.CreatableSlots))

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

	// Remember who we're playing — the in-game state reads walk speed, job
	// and appearance out of this long after char select is gone.
	s.manager.Session.Char = s.Characters[slotIndex]

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
