package skills

import "testing"

// TestEffectsAreNamedForTheSkillsWeMeet: a spot check that the port landed on
// the right skills, against effects whose names are not guessable from the
// skill's — Increase Agi plays EF_INCAGILITY, not EF_AGIUP, which is what the
// file implementing it is called.
func TestEffectsAreNamedForTheSkillsWeMeet(t *testing.T) {
	for _, tc := range []struct {
		skill uint16
		where string
		want  string
	}{
		{5, "BeginCast", "EF_BASH"},       // SM_BASH
		{7, "OnCaster", "EF_MAGNUMBREAK"}, // SM_MAGNUM
		{28, "OnTarget", "EF_HEAL"},       // AL_HEAL
		{29, "OnTarget", "EF_INCAGILITY"}, // AL_INCAGI
		{74, "OnCaster", "EF_MAGNIFICAT"}, // PR_MAGNIFICAT
	} {
		effects, ok := EffectsOf(tc.skill)
		if !ok {
			t.Errorf("skill %d has no effects", tc.skill)
			continue
		}

		var got []string
		switch tc.where {
		case "BeginCast":
			got = effects.BeginCast
		case "OnCaster":
			got = effects.OnCaster
		case "OnTarget":
			got = effects.OnTarget
		}

		found := false
		for _, name := range got {
			if name == tc.want {
				found = true
			}
		}

		if !found {
			t.Errorf("skill %d %s = %v, want %s", tc.skill, tc.where, got, tc.want)
		}
	}
}

// TestCastingSkillsBeginWithACircle: nearly every skill with a cast time draws
// the circle under the caster first, and that is the one effect this client
// will need before any other.
func TestCastingSkillsBeginWithACircle(t *testing.T) {
	circles := 0
	for _, effects := range skillEffects {
		for _, name := range effects.BeginCast {
			if name == "EF_BEGINSPELL" {
				circles++
			}
		}
	}

	if circles < 10 {
		t.Errorf("only %d skills begin with the casting circle, which is too few "+
			"for the table to have been read properly", circles)
	}
}

// TestEveryEffectNameLooksLikeOne: the names are the client's own EFID
// spelling, and anything else means the translation from nostalro's enum went
// wrong somewhere.
func TestEveryEffectNameLooksLikeOne(t *testing.T) {
	for skill, effects := range skillEffects {
		for _, list := range [][]string{
			effects.BeginCast, effects.OnCaster, effects.OnTarget, effects.OnGround,
		} {
			for _, name := range list {
				if len(name) < 4 || name[:3] != "EF_" {
					t.Errorf("skill %d names %q, which is not an effect", skill, name)
				}
			}
		}
	}
}

// TestNoSkillIsMappedToNothing: an entry with all four moments empty says a
// skill was identified and then lost, which is worse than no entry.
func TestNoSkillIsMappedToNothing(t *testing.T) {
	for skill, effects := range skillEffects {
		if len(effects.BeginCast) == 0 && len(effects.OnCaster) == 0 &&
			len(effects.OnTarget) == 0 && len(effects.OnGround) == 0 {
			t.Errorf("skill %d has an entry that names nothing", skill)
		}
	}
}
