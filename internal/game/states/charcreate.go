package states

import (
	"fmt"

	"go.uber.org/zap"

	"github.com/Faultbox/midgard-ro/internal/engine/charsprite"
	"github.com/Faultbox/midgard-ro/internal/logger"
	"github.com/Faultbox/midgard-ro/internal/network"
	"github.com/Faultbox/midgard-ro/internal/network/packets"
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

	// HairColor is which palette the hair is drawn through, 0..8. Sent to the
	// server, which stores it.
	HairColor int

	// Facing is which way the preview is turned, 0..7. Not sent anywhere —
	// the server has no field for it — it only decides which frame is drawn.
	Facing int

	// Name is what the player has typed. Validated here only for what is
	// locally knowable; whether it is free is the server's to answer.
	Name string

	// StatusMsg and ErrorMsg are what the screen says about itself.
	StatusMsg string
	ErrorMsg  string

	// autoCreated keeps unattended creation to one attempt.
	autoCreated bool

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
		Job:       charsprite.JobNovice,
		HairStyle: 1,
	}
}

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
	if job != charsprite.JobNovice && job != charsprite.JobSummoner {
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

// SetHairStyle picks a hair style. Style numbers start at 1: there is no
// file for style 0.
func (s *CharCreateState) SetHairStyle(style int) {
	if style < 1 || s.HairStyle == style {
		return
	}

	s.HairStyle = style
	trace.Emit(trace.Char, "set-hair", zap.Int("style", style))
}

// SetHairColor picks a hair palette. Nine exist per style and sex, 0 to 8.
func (s *CharCreateState) SetHairColor(color int) {
	if color < 0 || color >= HairColorCount || s.HairColor == color {
		return
	}

	s.HairColor = color
	trace.Emit(trace.Char, "set-hair-color", zap.Int("color", color))
}

// HairColorCount is how many hair palettes exist per style and sex.
const HairColorCount = 9

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
	if s.client != nil {
		s.client.RegisterHandler(packets.HC_ACCEPT_MAKECHAR, s.handleCreated)
		s.client.RegisterHandler(packets.HC_REFUSE_MAKECHAR, s.handleRefused)
	}

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

	// Unattended creation. One attempt: a refusal must stay on screen to be
	// read rather than being retried forever.
	if s.manager != nil && s.manager.MakeCharName != "" && !s.autoCreated {
		s.autoCreated = true
		s.SetName(s.manager.MakeCharName)
		s.Create()
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

// Name rules our server applies, from char_athena.conf. Checked here so a
// name it would certainly refuse costs no packet, and so the reason can be
// specific rather than the server's single "denied".
//
//	char_name_min_length: 4
//	char_name_option: 1 with char_name_letters — letters, digits and space
//	NAME_LENGTH 24, so 23 usable characters
const (
	nameMinLength = 4
	nameMaxLength = 23
)

// SetName records what has been typed.
func (s *CharCreateState) SetName(name string) {
	s.Name = name
}

// ValidateName reports why a name cannot be sent, or "" when it can.
//
// It deliberately does not try to answer whether the name is free: nothing
// asks the server that, and the only way to find out is to create and be
// refused.
func ValidateName(name string) string {
	if len(name) < nameMinLength {
		return fmt.Sprintf("A name needs at least %d characters.", nameMinLength)
	}

	if len(name) > nameMaxLength {
		return fmt.Sprintf("A name can be at most %d characters.", nameMaxLength)
	}

	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == ' ':
		default:
			return "A name can use letters, digits and spaces only."
		}
	}

	return ""
}

// Create sends the request, or explains why it did not.
func (s *CharCreateState) Create() {
	if reason := ValidateName(s.Name); reason != "" {
		s.ErrorMsg = reason
		trace.Emit(trace.Char, "create-rejected",
			zap.String("name", s.Name), zap.String("reason", reason))

		return
	}

	if s.client == nil {
		s.ErrorMsg = "Not connected."

		return
	}

	pkt := packets.EncodeMakeChar(packets.MakeCharRequest{
		Name:      s.Name,
		Slot:      uint8(s.slot),
		HairColor: uint16(s.HairColor),
		HairStyle: uint16(s.HairStyle),
		Job:       uint32(s.Job),
		Sex:       s.Sex,
	})
	if pkt == nil {
		s.ErrorMsg = "That name cannot be sent."

		return
	}

	trace.Emit(trace.Char, "create-send",
		zap.String("name", s.Name), zap.Int("slot", s.slot),
		zap.Int("job", s.Job), zap.Uint8("sex", s.Sex),
		zap.Int("hairStyle", s.HairStyle), zap.Int("hairColor", s.HairColor))

	if err := s.client.Send(pkt); err != nil {
		logger.Warn("could not send the creation request", zap.Error(err))
		s.ErrorMsg = "Could not reach the server."

		return
	}

	s.ErrorMsg = ""
	s.StatusMsg = "Creating..."
}

// handleCreated enters the game as the character the server has just made.
//
// The accept carries the whole character record, so nothing has to be asked
// for: it is added to the list character select already holds and then
// selected, which is what a player wants after naming someone — they made it
// to play it, not to look at a list.
//
// The list is patched rather than re-requested because it cannot be
// re-requested: CH_ENTER is how a session connects, not how it refreshes, and
// the char server does not answer a second one.
func (s *CharCreateState) handleCreated(data []byte) error {
	trace.Emit(trace.Char, "create-ok", zap.Int("slot", s.slot))
	logger.Info("character created", zap.Int("slot", s.slot), zap.String("name", s.Name))

	if s.back == nil || s.manager == nil {
		return nil
	}

	s.back.ClearPendingCreate()

	// type, then one CHARACTER_INFO.
	char := packets.DecodeCharInfo(data[2:])
	if char == nil {
		// The character exists — the server said so — but we cannot read it
		// back. Returning to the list is the honest fallback, and it will be
		// missing this one until the next login.
		logger.Warn("could not read the character the server just made",
			zap.Int("bytes", len(data)))
		s.manager.Change(s.back)

		return nil
	}

	index := s.back.AddCharacter(char)

	trace.Emit(trace.Char, "create-enter",
		zap.Int("slot", s.slot), zap.String("name", char.GetName()))

	if err := s.back.SelectCharacter(index); err != nil {
		logger.Warn("could not enter the game as the new character",
			zap.Error(err))
		s.manager.Change(s.back)
	}

	return nil
}

// handleRefused reports why the server would not create the character and
// leaves the screen up so it can be corrected.
func (s *CharCreateState) handleRefused(data []byte) error {
	code, ok := packets.DecodeMakeCharRefuse(data)
	if !ok {
		logger.Warn("could not read a creation refusal", zap.Int("bytes", len(data)))
		s.ErrorMsg = "The server refused, without saying why."

		return nil
	}

	s.StatusMsg = ""
	s.ErrorMsg = packets.MakeCharFailure(code)

	trace.Emit(trace.Char, "create-refused",
		zap.Uint8("code", code), zap.String("reason", s.ErrorMsg))
	logger.Info("character creation refused",
		zap.Uint8("code", code), zap.String("name", s.Name))

	return nil
}

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
