package skills

import "testing"

// TestEverySkillNamedInATableIsKnown: the tables are generated from the
// client's data and the names from the server's, and they have to be talking
// about the same skills. An id in one and not the other is a window slot with
// nothing in it, or a prerequisite that cannot be explained.
func TestEverySkillNamedInATableIsKnown(t *testing.T) {
	unknown := 0

	check := func(what string, skill uint16) {
		if Name(skill) == "" {
			t.Errorf("%s names skill %d, which has no name", what, skill)
			unknown++
		}
	}

	for skill, list := range needs {
		check("a prerequisite list", skill)
		for _, need := range list {
			check("a prerequisite", need.Skill)
		}
	}

	for job, slots := range tree {
		for _, slot := range slots {
			if Name(slot.Skill) == "" {
				t.Errorf("job %d has skill %d in slot %d, which has no name",
					job, slot.Skill, slot.Slot)
				unknown++
			}
		}
	}

	for skill := range effects {
		check("an effect", skill)
	}
}

// TestNoJobPutsTwoSkillsInOneSlot: the slot is where the skill draws, and two
// in one place means one of them is invisible.
func TestNoJobPutsTwoSkillsInOneSlot(t *testing.T) {
	for job, slots := range tree {
		seen := map[int]uint16{}
		for _, slot := range slots {
			if first, clash := seen[slot.Slot]; clash {
				t.Errorf("job %d puts skills %d and %d in slot %d",
					job, first, slot.Skill, slot.Slot)
			}
			seen[slot.Slot] = slot.Skill
		}
	}
}

// TestPrerequisitesDoNotLoop: a skill that needs itself, directly or through a
// chain, can never be learned — and a reader that walks the chain to explain
// why the button is grey would not stop.
func TestPrerequisitesDoNotLoop(t *testing.T) {
	var walk func(skill uint16, seen map[uint16]bool) bool

	walk = func(skill uint16, seen map[uint16]bool) bool {
		if seen[skill] {
			return true
		}

		seen[skill] = true
		defer delete(seen, skill)

		for _, need := range needs[skill] {
			if walk(need.Skill, seen) {
				return true
			}
		}

		return false
	}

	for skill := range needs {
		if walk(skill, map[uint16]bool{}) {
			t.Errorf("skill %d (%s) needs itself", skill, Name(skill))
		}
	}
}

// TestTheFirstClassesHaveTheirTrees: a spot check that the grid is the window's
// and not something else. A Novice's three skills are the left column of three
// rows, so they land on slots 0, 7 and 14 of a seven-wide grid.
func TestTheFirstClassesHaveTheirTrees(t *testing.T) {
	novice := Tree(0)
	if len(novice) != 3 {
		t.Fatalf("the Novice has %d skills in its window, want 3", len(novice))
	}

	for i, want := range []int{0, TreeColumns, TreeColumns * 2} {
		if novice[i].Slot != want {
			t.Errorf("the Novice's skill %d is in slot %d, want %d", i, novice[i].Slot, want)
		}
	}

	// Basic Skill is the first thing anyone learns.
	if novice[0].Skill != 1 {
		t.Errorf("the Novice's first slot holds skill %d, want Basic Skill", novice[0].Skill)
	}

	if got := len(Tree(1)); got == 0 {
		t.Error("the Swordman has no skill tree")
	}
}

// TestEffectsNameSomething: an effect entry with nothing in it is an entry
// that would have been better absent — the caller cannot tell it from a skill
// that simply has no effect.
func TestEffectsNameSomething(t *testing.T) {
	for skill, effect := range effects {
		if len(effect.OnCaster) == 0 && len(effect.OnTarget) == 0 &&
			effect.CasterSound == "" && effect.TargetSound == "" && effect.CastMotion < 0 {
			t.Errorf("skill %d (%s) has an effect entry that says nothing", skill, Name(skill))
		}
	}
}
