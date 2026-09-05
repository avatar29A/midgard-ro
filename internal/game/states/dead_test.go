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

// TestACorpseDoesNotWalk is the fault this was written for.
//
// Everything a click can mean is refused by the server for a dead character,
// but the walk is not refused visibly: the client starts one on its own, and
// the body slid across the map in its death pose, past the window offering to
// put it back at the save point.
func TestACorpseDoesNotWalk(t *testing.T) {
	s := &InGameState{playerDead: true, destCellX: 4, destCellY: 4, hasDest: true}

	if err := s.RequestMove(90, 90); err != nil {
		t.Fatalf("asking a corpse to walk: %v", err)
	}

	if s.destCellX != 4 || s.destCellY != 4 {
		t.Errorf("the corpse is walking to %d,%d", s.destCellX, s.destCellY)
	}
}

// TestACorpseDoesNothingElseEither: a hotkey is not a click, and reaches the
// commands without passing the world's own gate.
func TestACorpseDoesNothingElseEither(t *testing.T) {
	s := &InGameState{playerDead: true}

	if err := s.UseItem(3); err != nil {
		t.Errorf("using an item while dead: %v", err)
	}
	if err := s.UseSkill(28, 1); err != nil {
		t.Errorf("casting while dead: %v", err)
	}
	if err := s.UseSkillAt(80, 1, 5, 5); err != nil {
		t.Errorf("placing a skill while dead: %v", err)
	}

	// Nothing was sent, since there is no client to send it with: what is
	// being checked is that none of them tried, which a nil client would have
	// turned into a panic on the way out.
	if lines := s.chat.Lines(); len(lines) != 0 {
		t.Errorf("the chat says %+v, want nothing", lines)
	}
}
