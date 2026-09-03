package states

import (
	"encoding/binary"
	"testing"

	"github.com/Faultbox/midgard-ro/internal/network/packets"
)

// stateChangeFor builds the packet the server sends when a unit's state
// changes.
func stateChangeFor(aid uint32, body uint16) []byte {
	pkt := make([]byte, 15)
	binary.LittleEndian.PutUint16(pkt, packets.ZC_STATE_CHANGE)
	binary.LittleEndian.PutUint32(pkt[2:], aid)
	binary.LittleEndian.PutUint16(pkt[6:], body)

	return pkt
}

// TestAFrozenUnitIsSealed: the ice stands over whoever the server says is
// frozen, and over nobody else.
func TestAFrozenUnitIsSealed(t *testing.T) {
	s, mob := withMob()

	if len(s.frozenUnits()) != 0 {
		t.Fatal("a unit in no particular state is sealed in ice")
	}

	mob.BodyState = packets.BodyFreeze

	frozen := s.frozenUnits()
	if len(frozen) != 1 || frozen[0] != mob.ID {
		t.Errorf("frozen units are %v, want just %d", frozen, mob.ID)
	}
}

// TestTheOtherBodyStatesAreNotIce: stunned is not frozen, and asleep is not
// either. Read as a set of bits rather than as one value at a time, stun —
// which is three — would be stone and freeze together.
func TestTheOtherBodyStatesAreNotIce(t *testing.T) {
	s, mob := withMob()

	for _, state := range []uint16{
		packets.BodyStone, packets.BodyStun, packets.BodySleep,
		packets.BodyStoneWait, packets.BodyBurning,
	} {
		mob.BodyState = state

		if got := s.frozenUnits(); len(got) != 0 {
			t.Errorf("body state %d is drawn as ice", state)
		}
	}
}

// TestTheIceGoesWithTheUnit: a dead monster is not held in ice, and a block
// standing where one used to be is worse than none.
func TestTheIceGoesWithTheUnit(t *testing.T) {
	s, mob := withMob()
	mob.BodyState = packets.BodyFreeze

	mob.IsDead = true
	if got := s.frozenUnits(); len(got) != 0 {
		t.Errorf("%d units still sealed after dying", len(got))
	}

	mob.IsDead = false
	s.entityManager.Remove(mob.ID)

	if got := s.frozenUnits(); len(got) != 0 {
		t.Errorf("%d units still sealed after leaving", len(got))
	}
}

// TestAStateChangeForAUnitNotInView: the packet arrives for anybody, and one
// for somebody the client has never heard of is not a crash.
func TestAStateChangeForAUnitNotInView(t *testing.T) {
	s, _ := withMob()

	// An id nothing has.
	if err := s.handleStateChange(stateChangeFor(99, packets.BodyFreeze)); err != nil {
		t.Errorf("a state change for a stranger returned %v", err)
	}
	if len(s.frozenUnits()) != 0 {
		t.Error("a stranger was sealed in ice")
	}
}

// TestAStateChangeSetsTheUnit: the state arrives on its own packet as well as
// with the unit, because it changes while the unit is standing there.
func TestAStateChangeSetsTheUnit(t *testing.T) {
	s, mob := withMob()

	if err := s.handleStateChange(stateChangeFor(mob.ID, packets.BodyFreeze)); err != nil {
		t.Fatalf("handling: %v", err)
	}

	if mob.BodyState != packets.BodyFreeze {
		t.Errorf("the unit is in state %d, want frozen", mob.BodyState)
	}

	if err := s.handleStateChange(stateChangeFor(mob.ID, packets.BodyNone)); err != nil {
		t.Fatalf("handling the thaw: %v", err)
	}

	if mob.BodyState != packets.BodyNone {
		t.Errorf("the unit is still in state %d after thawing", mob.BodyState)
	}
	if len(s.frozenUnits()) != 0 {
		t.Error("the ice outlived the freeze")
	}
}

// TestTheIceIsHeldAtFullHeight: this is a state that lasts, not a thing that
// happens. Played through, the spikes would grow, fade and leave the target
// standing in the open while the server still has them frozen.
func TestTheIceIsHeldAtFullHeight(t *testing.T) {
	seal := frostDiver2Parts()
	held := &activeBurst{parts: seal.parts, ageMs: frostDiverReachMs() + burstFrames(spikeRiseFrames)}

	quads := held.quadsAt(flatView)
	if len(quads) != len(seal.parts) {
		t.Fatalf("%d spikes drawn of %d, want them all standing", len(quads), len(seal.parts))
	}

	// The same spikes at the moment they break through, to compare against:
	// held, they should be many times taller than that.
	fresh := &activeBurst{parts: seal.parts, ageMs: frostDiverReachMs() + burstFrames(3)}
	young := fresh.quadsAt(flatView)

	if len(young) != len(quads) {
		t.Fatalf("%d spikes just after breaking through against %d held", len(young), len(quads))
	}

	for i, q := range quads {
		if q.Color[3] < 0.5 {
			t.Errorf("spike %d stands at %v strength, which is on its way out", i, q.Color[3])
		}

		_, grown := quadSpan(q.Corners)
		_, sliver := quadSpan(young[i].Corners)

		if grown < sliver*4 {
			t.Errorf("spike %d stands %v tall against %v when it broke through, which is not grown",
				i, grown, sliver)
		}
	}
}
