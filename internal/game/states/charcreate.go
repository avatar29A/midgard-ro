package states

import (
	"go.uber.org/zap"

	"github.com/Faultbox/midgard-ro/internal/logger"
	"github.com/Faultbox/midgard-ro/internal/network"
	"github.com/Faultbox/midgard-ro/internal/trace"
)

// CharCreateState is the "Create Character" screen.
//
// Reached by double-clicking an empty slot on character select, or by pressing
// Make with one selected. It owns the choices that go into CH_MAKE_CHAR and
// nothing else: the slot is decided before we get here and the character list
// stays character select's to know.
type CharCreateState struct {
	client  *network.Client
	manager *Manager

	// slot is where the character will go. Fixed for the life of the state —
	// changing it means going back and picking another.
	slot int

	// back is the character select to return to. Kept rather than rebuilt so
	// Go back cannot lose what that screen already knows; re-entering it
	// re-requests the list, which is what we want after a character is
	// actually made.
	back *CharSelectState

	// Sex is the character's sex, which at our packet version the client
	// chooses and sends. Defaults to the account's, since that is what the
	// player is most likely to want and it is the only sensible starting
	// point when the toggle has not been touched.
	Sex uint8

	// StatusMsg and ErrorMsg are what the screen says about itself.
	StatusMsg string
	ErrorMsg  string
}

// NewCharCreateState creates the character creation screen for one slot.
func NewCharCreateState(slot int, client *network.Client, manager *Manager, back *CharSelectState) *CharCreateState {
	sex := uint8(0)
	if client != nil {
		_, _, _, sex = client.Session()
	}

	return &CharCreateState{
		client:  client,
		manager: manager,
		slot:    slot,
		back:    back,
		Sex:     sex,
	}
}

// Slot is where the character being built will go.
func (s *CharCreateState) Slot() int { return s.slot }

// GetStatusMessage returns what the screen is saying.
func (s *CharCreateState) GetStatusMessage() string { return s.StatusMsg }

// GetErrorMessage returns the last refusal, or empty.
func (s *CharCreateState) GetErrorMessage() string { return s.ErrorMsg }

// Enter is called when entering this state.
func (s *CharCreateState) Enter() error {
	trace.Emit(trace.Char, "create-open",
		zap.Int("slot", s.slot), zap.Uint8("sex", s.Sex))
	logger.Info("character creation opened",
		zap.Int("slot", s.slot), zap.Uint8("sex", s.Sex))

	return nil
}

// Exit is called when leaving this state.
func (s *CharCreateState) Exit() error {
	return nil
}

// Update is called every frame.
//
// The char server is still connected and still talking — the character list
// and the slot counts arrive on it — so its packets have to keep being read
// while this screen is up, or the connection backs up behind us.
func (s *CharCreateState) Update(_ float64) error {
	if s.client == nil {
		return nil
	}

	if err := s.client.Process(); err != nil {
		s.ErrorMsg = "Network error: " + err.Error()
	}

	return nil
}

// Render is called every frame to draw the state.
func (s *CharCreateState) Render() error { return nil }

// HandleInput processes input events.
func (s *CharCreateState) HandleInput(_ interface{}) error { return nil }

// Cancel abandons creation and returns to character select.
//
// Nothing has been sent by the time this is reachable, so there is nothing to
// undo — which is the property worth keeping as the screen grows.
func (s *CharCreateState) Cancel() {
	trace.Emit(trace.Char, "create-cancel", zap.Int("slot", s.slot))
	logger.Info("character creation canceled", zap.Int("slot", s.slot))

	if s.back == nil || s.manager == nil {
		return
	}

	s.back.ClearPendingCreate()
	s.manager.Change(s.back)
}
