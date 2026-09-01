package states

import (
	"testing"

	"github.com/Faultbox/midgard-ro/internal/network/packets"
)

// newCharSelectForTest builds a state holding `filled` characters out of
// `slots`, without a client or a manager — RequestCreate touches neither.
func newCharSelectForTest(filled, slots int) *CharSelectState {
	s := &CharSelectState{
		SelectedSlot:  -1,
		CreateSlot:    -1,
		MaxSlots:      slots,
		CharListReady: true,
	}
	for i := 0; i < filled; i++ {
		s.Characters = append(s.Characters, &packets.CharInfo{})
	}

	return s
}

// TestRequestCreateAcceptsAnEmptySlot: the whole point of step 1 — a slot with
// no character in it can be asked to make one.
func TestRequestCreateAcceptsAnEmptySlot(t *testing.T) {
	s := newCharSelectForTest(1, 9)

	s.RequestCreate(1)

	if got := s.PendingCreateSlot(); got != 1 {
		t.Errorf("pending slot = %d, want 1", got)
	}
}

// TestRequestCreateRefusesWhatCannotBeCreated: the UI only offers creation on
// an empty slot, so every case here means the UI and the state disagree about
// what is empty. Refusing is what keeps that disagreement from becoming a
// packet.
func TestRequestCreateRefusesWhatCannotBeCreated(t *testing.T) {
	tests := []struct {
		name   string
		filled int
		slots  int
		slot   int
	}{
		{"a slot that already holds a character", 3, 9, 0},
		{"the last filled slot", 3, 9, 2},
		{"past the account's slot count", 1, 9, 9},
		{"well past it", 1, 9, 99},
		{"a negative slot", 1, 9, -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newCharSelectForTest(tt.filled, tt.slots)

			s.RequestCreate(tt.slot)

			if got := s.PendingCreateSlot(); got != -1 {
				t.Errorf("pending slot = %d, want -1 (refused)", got)
			}
		})
	}
}

// TestPendingCreateSlotStartsUnset: -1 means nothing was asked for, and the
// screen that reads this must not open on a fresh state.
func TestPendingCreateSlotStartsUnset(t *testing.T) {
	cfg := CharSelectStateConfig{}
	s := NewCharSelectState(cfg, nil, nil)

	if got := s.PendingCreateSlot(); got != -1 {
		t.Errorf("a fresh state reports slot %d, want -1", got)
	}
}

// TestRequestCreateTakesTheFirstFreeSlotBoundary: with 3 of 9 used, slot 3 is
// the first free one and slot 2 is the last taken one. Off by one here means
// creating over a character.
func TestRequestCreateTakesTheFirstFreeSlotBoundary(t *testing.T) {
	s := newCharSelectForTest(3, 9)

	s.RequestCreate(2)
	if s.PendingCreateSlot() != -1 {
		t.Error("slot 2 holds a character but was accepted")
	}

	s.RequestCreate(3)
	if s.PendingCreateSlot() != 3 {
		t.Errorf("slot 3 is free but was refused (pending = %d)", s.PendingCreateSlot())
	}
}

// TestRequestCreateWaitsForTheCharacterList is a regression test for a bug
// that only appeared *after* a character was successfully created.
//
// Returning to character select re-enters it, which clears the characters and
// asks the server for them again. In the window before they arrive the list is
// empty — and "not known yet" must not read as "every slot is free", or the
// screen reopens on slot 0 and the server refuses everything from then on.
// That is exactly what happened: one character made, then nothing would create.
func TestRequestCreateWaitsForTheCharacterList(t *testing.T) {
	s := newCharSelectForTest(2, 9)

	// Re-entering does this.
	s.Characters = nil
	s.CharListReady = false

	s.RequestCreate(0)

	if got := s.PendingCreateSlot(); got != -1 {
		t.Errorf("pending slot = %d, want -1 — slot 0 holds a character, the "+
			"list just had not arrived to say so", got)
	}
}

// TestRequestCreateResumesOnceTheListArrives: the guard above must not be a
// permanent refusal.
func TestRequestCreateResumesOnceTheListArrives(t *testing.T) {
	s := newCharSelectForTest(2, 9)
	s.Characters = nil
	s.CharListReady = false

	s.RequestCreate(2)
	if s.PendingCreateSlot() != -1 {
		t.Fatal("accepted a slot before the list arrived")
	}

	// The list comes back with two characters, so slot 2 is genuinely free.
	s.Characters = newCharSelectForTest(2, 9).Characters
	s.CharListReady = true

	s.RequestCreate(2)
	if got := s.PendingCreateSlot(); got != 2 {
		t.Errorf("pending slot = %d, want 2 once the list is known", got)
	}
}
