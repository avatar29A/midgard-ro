package states

import (
	"encoding/binary"
	"testing"

	"github.com/Faultbox/midgard-ro/internal/network/packets"
)

// autorunPacket builds a ZC_AUTORUN_SKILL the way the server does.
func autorunPacket(skill uint16, inf uint32, level uint16) []byte {
	pkt := make([]byte, 39)
	binary.LittleEndian.PutUint16(pkt, packets.ZC_AUTORUN_SKILL)
	binary.LittleEndian.PutUint16(pkt[2:], skill)
	binary.LittleEndian.PutUint32(pkt[4:], inf)
	binary.LittleEndian.PutUint16(pkt[8:], level)
	binary.LittleEndian.PutUint16(pkt[10:], 10) // sp
	binary.LittleEndian.PutUint16(pkt[12:], 1)  // range
	copy(pkt[14:], "AL_TELEPORT")

	return pkt
}

// TestDecodeAutorunSkill: the level sits after a four-byte targeting field,
// and read as two the level comes out of the top half of the targeting.
func TestDecodeAutorunSkill(t *testing.T) {
	const teleport = 26

	use, ok := packets.DecodeAutorunSkill(autorunPacket(teleport, packets.InfSelf, 3))
	if !ok {
		t.Fatal("a full-length packet did not decode")
	}

	if use.SkillID != teleport {
		t.Errorf("skill = %d, want %d", use.SkillID, teleport)
	}
	if use.Level != 3 {
		t.Errorf("level = %d, want 3", use.Level)
	}
	if use.Inf != packets.InfSelf {
		t.Errorf("targeting = %d, want %d", use.Inf, packets.InfSelf)
	}

	if _, ok := packets.DecodeAutorunSkill(autorunPacket(teleport, packets.InfSelf, 3)[:9]); ok {
		t.Error("a short packet decoded")
	}
}

// TestAWingDoesNotWaitToBeAimed: a Fly Wing's skill is cast on the caster, and
// holding it for a target would leave the player with a cursor and no
// teleport. This is the whole of why using one did nothing: the packet asking
// for the cast was never read at all.
func TestAWingDoesNotWaitToBeAimed(t *testing.T) {
	s := &InGameState{}

	if err := s.handleAutorunSkill(autorunPacket(26, packets.InfSelf, 1)); err != nil {
		t.Fatalf("handling an autorun: %v", err)
	}

	if skill, holding := s.Placing(); holding {
		t.Errorf("a self-cast skill is being held for a cell: %d", skill)
	}
}

// TestAnAimedAutorunIsHeld: an item handing over a skill that needs a target
// or a cell asks for one, the same way the skill window does.
func TestAnAimedAutorunIsHeld(t *testing.T) {
	for _, tc := range []struct {
		name  string
		inf   uint32
		atNo  bool
		level uint16
	}{
		{"a ground skill waits for a cell", packets.InfGround, false, 5},
		{"a targeted skill waits for somebody", packets.InfAttack, true, 2},
	} {
		s := &InGameState{}

		if err := s.handleAutorunSkill(autorunPacket(80, tc.inf, tc.level)); err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}

		skill, holding := s.Placing()
		if !holding {
			t.Errorf("%s: nothing is being held", tc.name)

			continue
		}
		if skill != 80 {
			t.Errorf("%s: holding skill %d, want 80", tc.name, skill)
		}
		if s.placingLevel != int(tc.level) {
			t.Errorf("%s: holding level %d, want %d", tc.name, s.placingLevel, tc.level)
		}
		if s.placingAtUnit != tc.atNo {
			t.Errorf("%s: atUnit = %v, want %v", tc.name, s.placingAtUnit, tc.atNo)
		}
	}
}

// TestAnAutorunWithoutALevel: the level is used as it comes, and nought would
// ask the server to cast level nothing.
func TestAnAutorunWithoutALevel(t *testing.T) {
	s := &InGameState{}

	if err := s.handleAutorunSkill(autorunPacket(80, packets.InfGround, 0)); err != nil {
		t.Fatalf("handling an autorun: %v", err)
	}

	if s.placingLevel < 1 {
		t.Errorf("held at level %d, want at least one", s.placingLevel)
	}
}
