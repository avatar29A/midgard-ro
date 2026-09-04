package states

import (
	"encoding/binary"
	"testing"

	"github.com/Faultbox/midgard-ro/internal/network/packets"
)

// TestDeadIsNotReadFromTheHitPoints: the first thing tried, and wrong.
// rAthena leaves a corpse on one hit point rather than on nought, so a
// character who has just died looks like one with a sliver left — and one who
// logs in dead looks perfectly well.
func TestDeadIsNotReadFromTheHitPoints(t *testing.T) {
	s := &InGameState{stats: PlayerStats{HP: 1, MaxHP: 3184}}

	if s.Dead() {
		t.Error("a character on one hit point is dead")
	}

	s.playerDead = true

	if !s.Dead() {
		t.Error("a character the server said had died is not dead, on the same numbers")
	}
}

// resurrection builds a ZC_RESURRECTION the way the server does.
func resurrection(aid uint32) []byte {
	pkt := make([]byte, 8)
	binary.LittleEndian.PutUint16(pkt, packets.ZC_RESURRECTION)
	binary.LittleEndian.PutUint32(pkt[2:], aid)

	return pkt
}

// TestResurrectionStandsThePlayerUp: whoever put the hit points back, the
// window offering a way out of being dead has to go with the corpse.
func TestResurrectionStandsThePlayerUp(t *testing.T) {
	s := &InGameState{playerDead: true}

	if err := s.handleResurrection(resurrection(s.selfAID())); err != nil {
		t.Fatalf("handling a resurrection: %v", err)
	}

	if s.Dead() {
		t.Error("still dead after being resurrected")
	}
}

// TestSomebodyElseBeingRaisedChangesNothing: a priest raising the character
// standing beside you is not you getting up.
func TestSomebodyElseBeingRaisedChangesNothing(t *testing.T) {
	s := &InGameState{playerDead: true}

	// An id that is not ours, whatever ours turns out to be.
	other := s.selfAID() + 1

	if err := s.handleResurrection(resurrection(other)); err != nil {
		t.Fatalf("handling a resurrection: %v", err)
	}

	if !s.Dead() {
		t.Error("somebody else being raised stood us up too")
	}
}

// TestResurrectionRejectsShortPackets: a truncated packet is ignored rather
// than read past its end.
func TestResurrectionRejectsShortPackets(t *testing.T) {
	s := &InGameState{playerDead: true}

	if err := s.handleResurrection(resurrection(s.selfAID())[:5]); err != nil {
		t.Fatalf("handling a short resurrection: %v", err)
	}

	if !s.Dead() {
		t.Error("a short packet stood us up")
	}
}
