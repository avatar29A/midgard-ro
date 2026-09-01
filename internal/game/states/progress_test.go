package states

import (
	"encoding/binary"
	"testing"

	"github.com/Faultbox/midgard-ro/internal/network/packets"
)

// effectPacket builds a ZC_NOTIFY_EFFECT.
func effectPacket(aid, effect uint32) []byte {
	b := make([]byte, 10)
	binary.LittleEndian.PutUint16(b, packets.ZC_NOTIFY_EFFECT)
	binary.LittleEndian.PutUint32(b[2:], aid)
	binary.LittleEndian.PutUint32(b[6:], effect)

	return b
}

// TestLevelUpRaisesTheRightButton: the two levels are told apart by the
// effect code, since the server announces both through one packet.
func TestLevelUpRaisesTheRightButton(t *testing.T) {
	var s InGameState

	_ = s.handleLevelUpEffect(effectPacket(0, packets.EffectBaseLevelUp))
	base, job := s.LevelUpPending()
	if !base || job {
		t.Errorf("base level gave base=%v job=%v, want true and false", base, job)
	}

	_ = s.handleLevelUpEffect(effectPacket(0, packets.EffectJobLevelUp))
	base, job = s.LevelUpPending()
	if !base || !job {
		t.Errorf("both levels gave base=%v job=%v, want both", base, job)
	}
}

// TestLevelUpIgnoresOtherEffects: the same packet carries a refine and a
// pharmacy brew, and neither is a level.
func TestLevelUpIgnoresOtherEffects(t *testing.T) {
	var s InGameState

	for _, effect := range []uint32{packets.EffectRefineOK, packets.EffectRefineFail, packets.EffectPharmacyOK} {
		_ = s.handleLevelUpEffect(effectPacket(0, effect))
	}

	if base, job := s.LevelUpPending(); base || job {
		t.Error("something that is not a level raised a level-up button")
	}
}

// TestAcknowledgeLevelUpClearsOnlyItsOwn: pressing one button must not take
// the other away, since both can be waiting at once.
func TestAcknowledgeLevelUpClearsOnlyItsOwn(t *testing.T) {
	s := &InGameState{pendingLevelUp: true, pendingJobLevelUp: true}

	s.AcknowledgeLevelUp(true)
	if base, job := s.LevelUpPending(); base || !job {
		t.Errorf("after the base button: base=%v job=%v, want false and true", base, job)
	}

	s.AcknowledgeLevelUp(false)
	if _, job := s.LevelUpPending(); job {
		t.Error("the job button survived being pressed")
	}
}

// TestLevelUpIgnoresOtherPeople: somebody levelling beside us is their
// business, and must not put a button on our screen.
func TestLevelUpIgnoresOtherPeople(t *testing.T) {
	s := &InGameState{}

	_ = s.handleLevelUpEffect(effectPacket(999999, packets.EffectBaseLevelUp))

	if base, _ := s.LevelUpPending(); base {
		t.Error("someone else's level raised our button")
	}
}
