package states

import "testing"

// TestPlacingHoldsTheSkillUntilACellIsChosen: a ground skill is asked for
// twice, and between the two the client is holding one. Nothing should go out
// on the first ask.
func TestPlacingHoldsTheSkillUntilACellIsChosen(t *testing.T) {
	s := &InGameState{}

	if _, holding := s.Placing(); holding {
		t.Fatal("something was held before anything was chosen")
	}

	s.BeginPlacing(80, 5)

	skill, holding := s.Placing()
	if !holding {
		t.Fatal("the skill was not held")
	}
	if skill != 80 {
		t.Errorf("holding skill %d, want 80", skill)
	}
}

// TestCancelPlacingPutsItDown: escape means "not that after all", and a skill
// left held would take the next click meant for something else.
func TestCancelPlacingPutsItDown(t *testing.T) {
	s := &InGameState{}
	s.BeginPlacing(80, 5)
	s.CancelPlacing()

	if _, holding := s.Placing(); holding {
		t.Error("the skill is still held after being canceled")
	}
}

// TestAClickWhileHoldingIsTakenByTheSkill: the click means "here". If it fell
// through, the character would walk to the cell instead of casting on it.
func TestAClickWhileHoldingIsTakenByTheSkill(t *testing.T) {
	s := &InGameState{}

	if s.placeHeldSkill(0, 0, 0, 0) {
		t.Error("a click was taken with nothing held")
	}

	s.BeginPlacing(80, 5)

	if !s.placeHeldSkill(0, 0, 0, 0) {
		t.Error("a click was not taken while a skill was held")
	}
	if _, holding := s.Placing(); holding {
		t.Error("the skill is still held after being placed")
	}
}

// TestCastBarRunsDownAndEnds: the server sends the length once and says
// nothing more, so the bar is run off that number alone.
func TestCastBarRunsDownAndEnds(t *testing.T) {
	s := &InGameState{}

	if _, _, casting := s.CastProgress(); casting {
		t.Fatal("a bar was running before any cast")
	}

	s.beginCast(14, 1000)

	done, skill, casting := s.CastProgress()
	if !casting {
		t.Fatal("the bar did not start")
	}
	if skill != 14 {
		t.Errorf("the bar is for skill %d, want 14", skill)
	}
	if done != 0 {
		t.Errorf("the bar starts at %v, want 0", done)
	}

	s.advanceCast(500)
	if done, _, _ := s.CastProgress(); done < 0.49 || done > 0.51 {
		t.Errorf("halfway through, the bar reads %v", done)
	}

	s.advanceCast(600)
	if _, _, casting := s.CastProgress(); casting {
		t.Error("the bar is still running past its length")
	}
}

// TestAnInstantCastDrawsNoBar: most skills have no cast time at all, and a bar
// that appears and vanishes inside one frame is worse than none.
func TestAnInstantCastDrawsNoBar(t *testing.T) {
	s := &InGameState{}
	s.beginCast(28, 0)

	if _, _, casting := s.CastProgress(); casting {
		t.Error("an instant cast started a bar")
	}
}

// TestSupportSkillsWaitForSomebody: a skill that can go on anybody is held
// until somebody is chosen. Casting it at the caster instead is what put half
// of every support skill out of reach.
func TestSupportSkillsWaitForSomebody(t *testing.T) {
	s := &InGameState{}
	s.BeginTargeting(28, 10)

	skill, holding := s.Placing()
	if !holding || skill != 28 {
		t.Fatalf("the skill was not held: %d, %v", skill, holding)
	}
	if !s.placingAtUnit {
		t.Error("a support skill is waiting for a cell rather than for somebody")
	}
}

// TestPlacingAndTargetingAreTheSameHold: both are one skill waiting to be
// aimed, and a cancel or a click has to end either.
func TestPlacingAndTargetingAreTheSameHold(t *testing.T) {
	s := &InGameState{}

	s.BeginPlacing(80, 5)
	if s.placingAtUnit {
		t.Error("a ground skill is waiting for somebody rather than for a cell")
	}

	s.CancelPlacing()
	if _, holding := s.Placing(); holding {
		t.Error("cancel left a ground skill held")
	}

	s.BeginTargeting(28, 10)
	s.CancelPlacing()
	if _, holding := s.Placing(); holding {
		t.Error("cancel left a support skill held")
	}
	if s.placingAtUnit {
		t.Error("cancel left the hold pointed at a unit")
	}
}

// TestAClickOnNothingDropsATargetedSkill: the original puts the skill down
// rather than casting it on the caster. Casting on the caster would spend SP
// nobody asked to spend, on a click that meant "never mind".
func TestAClickOnNothingDropsATargetedSkill(t *testing.T) {
	s := &InGameState{}
	s.BeginTargeting(28, 10)

	// No entity manager, so nothing is under the pointer.
	if !s.placeHeldSkill(0, 0, 0, 0) {
		t.Fatal("the click was not taken by the held skill")
	}
	if _, holding := s.Placing(); holding {
		t.Error("the skill is still held after a click on nothing")
	}
}

// TestOnlyHealingSkillsShowAFigure: rAthena writes the amount healed and the
// skill level into the same field, so a level 10 Increase Agi sends a 10 that
// means nothing. Shown as a figure it reads as ten hit points restored, which
// is what it did.
func TestOnlyHealingSkillsShowAFigure(t *testing.T) {
	if !healSkills[28] {
		t.Error("Heal does not show a figure")
	}

	for _, buff := range []uint16{
		29, // AL_INCAGI
		34, // AL_BLESSING
		66, // PR_IMPOSITIO
	} {
		if healSkills[buff] {
			t.Errorf("skill %d shows a figure, but its amount is a skill level", buff)
		}
	}
}

// TestSkillLabelsAgeOut: a name over somebody has to go away, or a fight
// leaves the map covered in them.
func TestSkillLabelsAgeOut(t *testing.T) {
	s := &InGameState{}
	s.skillLabels = []floatingSkillName{{text: "Increase AGI"}}

	s.advanceSkillLabels(skillLabelLifeMs / 2)
	if len(s.skillLabels) != 1 {
		t.Fatal("the label went early")
	}

	s.advanceSkillLabels(skillLabelLifeMs)
	if len(s.skillLabels) != 0 {
		t.Errorf("%d labels outlived their time", len(s.skillLabels))
	}
}
