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

	// Job is what the character starts as — Novice, or Summoner for a Doram.
	// Both are creatable on our server (allowed_job_flag 3).
	Job int

	// HairStyle is the head sprite number. Style 0 is not a file; every sex
	// has a style 1, which is where the screen starts.
	HairStyle int

	// Facing is which way the preview is turned, 0..7. Not sent anywhere —
	// the server has no field for it — it only decides which frame is drawn.
	Facing int

	// StatusMsg and ErrorMsg are what the screen says about itself.
	StatusMsg string
	ErrorMsg  string

	// keepAlive stops the char server dropping us while a name is being
	// thought about. Picking a name and a hair style takes longer than the
	// server's 60-second patience.
	keepAlive charKeepAlive
}

// NewCharCreateState creates the character creation screen for one slot.
func NewCharCreateState(slot int, client *network.Client, manager *Manager, back *CharSelectState) *CharCreateState {
	sex := uint8(0)
	if client != nil {
		_, _, _, sex = client.Session()
	}

	return &CharCreateState{
		client:    client,
		manager:   manager,
		slot:      slot,
		back:      back,
		Sex:       sex,
		Job:       JobNovice,
		HairStyle: 1,
	}
}

// The two jobs our server allows at creation. Anything else is refused with
// "Invalid job" (char/char.cpp:1489).
const (
	// JobNovice is a Human.
	JobNovice = 0
	// JobSummoner is a Doram.
	JobSummoner = 4218
)

// SetSex chooses the character's sex. At our packet version this is the
// client's to decide and is sent in CH_MAKE_CHAR; the server accepts only
// male or female.
func (s *CharCreateState) SetSex(sex uint8) {
	if s.Sex == sex {
		return
	}

	s.Sex = sex
	trace.Emit(trace.Char, "set-sex", zap.Uint8("sex", sex))
}

// SetJob chooses Human or Doram.
func (s *CharCreateState) SetJob(job int) {
	if job != JobNovice && job != JobSummoner {
		logger.Warn("asked for a job the server does not allow at creation",
			zap.Int("job", job))

		return
	}

	if s.Job == job {
		return
	}

	s.Job = job
	trace.Emit(trace.Char, "set-job", zap.Int("job", job))
}

// Turn rotates the preview by one step, wrapping in both directions.
func (s *CharCreateState) Turn(delta int) {
	const directions = 8

	s.Facing = ((s.Facing+delta)%directions + directions) % directions
	trace.Emit(trace.Char, "turn", zap.Int("facing", s.Facing))
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

	s.keepAlive.tick(s.client)

	if err := s.client.Process(); err != nil {
		// Logged as well as shown: a message that only ever reaches the
		// screen is invisible to an unattended run, and this one took a
		// while to explain because nothing recorded it.
		logger.Warn("character server connection failed while creating",
			zap.Int("slot", s.slot), zap.Error(err))

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
