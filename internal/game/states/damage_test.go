package states

import (
	"testing"

	"github.com/Faultbox/midgard-ro/internal/game/entity"
	"github.com/Faultbox/midgard-ro/internal/network/packets"
)

// TestDamageNumbersAgeAndExpire: a figure lasts long enough to read and then
// goes, or a long fight would leave the map papered in old numbers.
func TestDamageNumbersAgeAndExpire(t *testing.T) {
	s := &InGameState{damageNumbers: []floatingDamage{{amount: 12}}}

	s.updateDamageNumbers(damageLifeMs / 2)
	if len(s.damageNumbers) != 1 {
		t.Fatal("dropped a figure that is still within its life")
	}

	s.updateDamageNumbers(damageLifeMs)
	if len(s.damageNumbers) != 0 {
		t.Error("kept a figure past the end of its life")
	}
}

// TestDamageNumbersAreCapped: a fight against several monsters lands a blow
// every few frames, and there is no point holding figures nobody can read.
// The oldest goes, not the newest — the blow that just landed is the one
// worth seeing.
func TestDamageNumbersAreCapped(t *testing.T) {
	m := entity.NewManager()
	target := mobAt(42, 100, 100)
	m.Add(target)

	s := &InGameState{entityManager: m}

	for i := 0; i < damageMaxOnScreen+10; i++ {
		s.addDamageNumber(packets.Damage{TargetID: 42, Amount: i, Type: packets.DamageNormal})
	}

	if len(s.damageNumbers) > damageMaxOnScreen {
		t.Errorf("holding %d figures, want at most %d", len(s.damageNumbers), damageMaxOnScreen)
	}

	// The last blow must have survived the capping.
	last := s.damageNumbers[len(s.damageNumbers)-1]
	if last.amount != damageMaxOnScreen+9 {
		t.Errorf("newest figure is %d, want the blow that just landed", last.amount)
	}
}

// TestDamageNumberNeedsSomethingToFloatOver: a blow between two units we
// cannot see has nowhere to put a figure, and must not be a crash.
func TestDamageNumberNeedsSomethingToFloatOver(t *testing.T) {
	s := &InGameState{entityManager: entity.NewManager()}

	s.addDamageNumber(packets.Damage{TargetID: 999, Amount: 5})

	if len(s.damageNumbers) != 0 {
		t.Error("started a figure over a unit that is not on the map")
	}
}

// TestDamageNumberRecordsAMiss: a miss is drawn as the word, not as a nought,
// so the difference has to survive into the figure.
func TestDamageNumberRecordsAMiss(t *testing.T) {
	m := entity.NewManager()
	m.Add(mobAt(42, 100, 100))

	s := &InGameState{entityManager: m}
	s.addDamageNumber(packets.Damage{TargetID: 42, Amount: 0, Type: packets.DamageNormal})

	if len(s.damageNumbers) != 1 {
		t.Fatal("no figure for a miss")
	}
	if !s.damageNumbers[0].miss {
		t.Error("a blow for nothing did not record as a miss")
	}
}
