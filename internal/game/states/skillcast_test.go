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

	if s.placeHeldSkill() {
		t.Error("a click was taken with nothing held")
	}

	s.BeginPlacing(80, 5)

	if !s.placeHeldSkill() {
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
