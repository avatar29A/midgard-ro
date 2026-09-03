package ui

import (
	"testing"

	"github.com/Faultbox/midgard-ro/internal/network/packets"
)

// fireBolt is a skill known at ten, which is the case the level picker exists
// for.
var fireBolt = packets.Skill{ID: 19, Level: 10, SP: 30, Inf: packets.InfAttack, Range: 9}

// TestASkillGoesOffAtWhatWasPicked: a skill can be cast at any level up to
// the one it is known at, and a lower one costs less. Without a pick it goes
// off at what the character has, which is what every cast meant before.
func TestASkillGoesOffAtWhatWasPicked(t *testing.T) {
	b := &UI2DBackend{}

	if got := b.skillCastLevel(fireBolt); got != 10 {
		t.Errorf("with nothing picked it goes off at %d, want the learned 10", got)
	}

	b.setSkillLevel(fireBolt.ID, 5)
	if got := b.skillCastLevel(fireBolt); got != 5 {
		t.Errorf("picked at five it goes off at %d", got)
	}
}

// TestAPickIsHeldInsideWhatIsLearned: a skill picked at five and then reset
// to three would ask for a level the server refuses.
func TestAPickIsHeldInsideWhatIsLearned(t *testing.T) {
	b := &UI2DBackend{}
	b.setSkillLevel(fireBolt.ID, 9)

	lowered := fireBolt
	lowered.Level = 3

	if got := b.skillCastLevel(lowered); got != 3 {
		t.Errorf("a pick past what is known goes off at %d, want 3", got)
	}
}

// TestTheUseButtonNeedsARowThatCanBeCast: nothing chosen, a skill the server
// no longer lists, and a passive one all leave it with nothing to do.
func TestTheUseButtonNeedsARowThatCanBeCast(t *testing.T) {
	state := InGameUIState{Skills: []packets.Skill{
		fireBolt,
		{ID: 2, Level: 10, Inf: 0}, // SM_SWORD, passive
	}}

	b := &UI2DBackend{}
	if _, ok := b.chosenSkill(state); ok {
		t.Error("the use button offered to cast with no row chosen")
	}

	b.skillChosen = 999
	if _, ok := b.chosenSkill(state); ok {
		t.Error("it offered to cast a skill the server does not list")
	}

	b.skillChosen = 2
	if _, ok := b.chosenSkill(state); ok {
		t.Error("it offered to cast a passive skill")
	}

	b.skillChosen = fireBolt.ID
	got, ok := b.chosenSkill(state)
	if !ok || got.ID != fireBolt.ID {
		t.Errorf("it did not offer the chosen skill: %+v, %v", got, ok)
	}
}

// TestCastingFromTheWindowCarriesTheLevel: the row's pick is what goes out,
// and a passive skill sends nothing at all.
func TestCastingFromTheWindowCarriesTheLevel(t *testing.T) {
	b := &UI2DBackend{}
	b.setSkillLevel(fireBolt.ID, 4)

	b.castSkillFromWindow(fireBolt)

	cast, ok := b.TakeSkillCast()
	if !ok {
		t.Fatal("the window cast nothing")
	}
	if cast.Skill != fireBolt.ID || cast.Level != 4 {
		t.Errorf("it cast %+v, want skill %d at level 4", cast, fireBolt.ID)
	}

	b.castSkillFromWindow(packets.Skill{ID: 2, Level: 10, Inf: 0})
	if _, ok := b.TakeSkillCast(); ok {
		t.Error("a passive skill was cast")
	}
}

// TestASkillGoesToTheBarAtThePickedLevel: which is how the same skill sits on
// the quick panel twice, cheap on one key and full on another.
func TestASkillGoesToTheBarAtThePickedLevel(t *testing.T) {
	b := &UI2DBackend{}
	b.hotkeyRows = 1
	b.setSkillLevel(fireBolt.ID, 3)

	b.AssignHotkey(0, 0, hotkeyCell{id: uint32(fireBolt.ID), skill: true, level: b.skillCastLevel(fireBolt)})
	b.AssignHotkey(0, 1, hotkeyCell{id: uint32(fireBolt.ID), skill: true, level: 10})

	if got := b.hotkeyItems[0][0].level; got != 3 {
		t.Errorf("the first key holds level %d, want 3", got)
	}
	if got := b.hotkeyItems[0][1].level; got != 10 {
		t.Errorf("the second key holds level %d, want 10", got)
	}
}

// TestASavedLevelComesBack: and a cell that goes off at whatever is learned
// writes nothing extra, so a bar nobody has picked a level on is unchanged.
func TestASavedLevelComesBack(t *testing.T) {
	b := &UI2DBackend{}
	b.setHotkeyItem(0, 0, hotkeyCell{id: 19, skill: true, level: 3})
	b.setHotkeyItem(0, 1, hotkeyCell{id: 19, skill: true})

	_, skills, levels := b.savedHotkeyItems()

	if len(skills) != 2 {
		t.Fatalf("%d skills saved, want 2", len(skills))
	}
	if levels["0,0"] != 3 {
		t.Errorf("the picked level saved as %d", levels["0,0"])
	}
	if _, written := levels["0,1"]; written {
		t.Error("a cell that goes off at whatever is learned wrote a level")
	}

	back := &UI2DBackend{}
	back.loadHotkeyItems(nil, skills, levels)

	if got := back.hotkeyItems[0][0].level; got != 3 {
		t.Errorf("the level came back as %d", got)
	}
	if got := back.hotkeyItems[0][1].level; got != 0 {
		t.Errorf("a cell with no level came back as %d", got)
	}
}
