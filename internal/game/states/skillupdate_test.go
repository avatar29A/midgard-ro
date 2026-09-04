package states

import (
	"encoding/binary"
	"testing"

	"github.com/Faultbox/midgard-ro/internal/network/packets"
)

// skillEntryPacket builds a ZC_ADD_SKILL or ZC_SKILLINFO_UPDATE2, whose
// bodies are the same fifteen bytes a list entry is.
func skillEntryPacket(id uint16, inf uint32, level, sp, rng uint16, raisable bool) []byte {
	pkt := make([]byte, 17)
	binary.LittleEndian.PutUint16(pkt, packets.ZC_ADD_SKILL)
	binary.LittleEndian.PutUint16(pkt[2:], id)
	binary.LittleEndian.PutUint32(pkt[4:], inf)
	binary.LittleEndian.PutUint16(pkt[8:], level)
	binary.LittleEndian.PutUint16(pkt[10:], sp)
	binary.LittleEndian.PutUint16(pkt[12:], rng)
	if raisable {
		pkt[14] = 1
	}
	binary.LittleEndian.PutUint16(pkt[15:], level)

	return pkt
}

// skillRaisedPacket builds a ZC_SKILLINFO_UPDATE, which carries no targeting.
func skillRaisedPacket(id, level, sp, rng uint16, raisable bool) []byte {
	pkt := make([]byte, 11)
	binary.LittleEndian.PutUint16(pkt, packets.ZC_SKILLINFO_UPDATE)
	binary.LittleEndian.PutUint16(pkt[2:], id)
	binary.LittleEndian.PutUint16(pkt[4:], level)
	binary.LittleEndian.PutUint16(pkt[6:], sp)
	binary.LittleEndian.PutUint16(pkt[8:], rng)
	if raisable {
		pkt[10] = 1
	}

	return pkt
}

// skillDeletePacket builds a ZC_SKILLINFO_DELETE.
func skillDeletePacket(id uint16) []byte {
	pkt := make([]byte, 4)
	binary.LittleEndian.PutUint16(pkt, packets.ZC_SKILLINFO_DELETE)
	binary.LittleEndian.PutUint16(pkt[2:], id)

	return pkt
}

// TestANewSkillJoinsTheList: a skill granted after the map loaded used to
// appear nowhere until the character logged in again and the list came round
// afresh.
func TestANewSkillJoinsTheList(t *testing.T) {
	s := &InGameState{skills: []packets.Skill{{ID: 19, Level: 5, Inf: 1}}}

	if err := s.handleSkillAdded(skillEntryPacket(142, 4, 1, 0, 0, false)); err != nil {
		t.Fatalf("handling an added skill: %v", err)
	}

	if len(s.skills) != 2 {
		t.Fatalf("the list holds %d skills, want the old one and the new", len(s.skills))
	}
	if s.skills[1].ID != 142 || s.skills[1].Level != 1 || s.skills[1].Inf != 4 {
		t.Errorf("the new skill came out as %+v", s.skills[1])
	}
}

// TestARaisedSkillKeepsItsTargeting: ZC_SKILLINFO_UPDATE carries no targeting,
// and zero is how the server says passive. Written in from the packet, every
// skill raised with a point would turn into a passive one — no SP cost shown,
// no cast, nothing to aim.
func TestARaisedSkillKeepsItsTargeting(t *testing.T) {
	const fireBolt = 19

	s := &InGameState{skills: []packets.Skill{
		{ID: 14, Level: 3, Inf: 1, SP: 12},
		{ID: fireBolt, Level: 5, Inf: 1, SP: 30, Range: 9},
	}}

	if err := s.handleSkillRaised(skillRaisedPacket(fireBolt, 6, 34, 9, true)); err != nil {
		t.Fatalf("handling a raised skill: %v", err)
	}

	if len(s.skills) != 2 {
		t.Fatalf("raising a skill left %d in the list, want 2", len(s.skills))
	}

	raised := s.skills[1]
	if raised.ID != fireBolt {
		t.Fatalf("the raised skill moved: the list is %+v", s.skills)
	}
	if raised.Level != 6 || raised.SP != 34 || !raised.Raisable {
		t.Errorf("the raised skill came out as %+v", raised)
	}
	if raised.Inf != 1 {
		t.Errorf("targeting is %d after raising, want the 1 it already had", raised.Inf)
	}
}

// TestARaisedSkillStaysWhereItWas: the window draws the list in the order the
// server sent it, so a skill that jumped to the bottom on being raised would
// be a row moving out from under the pointer that raised it.
func TestARaisedSkillStaysWhereItWas(t *testing.T) {
	s := &InGameState{skills: []packets.Skill{
		{ID: 10, Level: 1}, {ID: 19, Level: 5, Inf: 1}, {ID: 21, Level: 2},
	}}

	if err := s.handleSkillRaised(skillRaisedPacket(19, 6, 34, 9, false)); err != nil {
		t.Fatalf("handling a raised skill: %v", err)
	}

	want := []uint16{10, 19, 21}
	for i, id := range want {
		if s.skills[i].ID != id {
			t.Fatalf("the list reordered to %+v", s.skills)
		}
	}
}

// TestASkillTakenAwayLeavesTheList: a job change removes what the old job had.
func TestASkillTakenAwayLeavesTheList(t *testing.T) {
	s := &InGameState{skills: []packets.Skill{
		{ID: 10}, {ID: 19}, {ID: 21},
	}}

	if err := s.handleSkillRemoved(skillDeletePacket(19)); err != nil {
		t.Fatalf("handling a removed skill: %v", err)
	}

	if len(s.skills) != 2 {
		t.Fatalf("the list holds %d skills, want 2", len(s.skills))
	}
	for _, skill := range s.skills {
		if skill.ID == 19 {
			t.Error("the removed skill is still there")
		}
	}

	// One that was never there changes nothing rather than emptying the list.
	if err := s.handleSkillRemoved(skillDeletePacket(999)); err != nil {
		t.Fatalf("handling a removal of nothing: %v", err)
	}
	if len(s.skills) != 2 {
		t.Errorf("removing a skill nobody had left %d", len(s.skills))
	}
}

// TestShortSkillPacketsAreIgnored: a truncated packet leaves the list as it
// was rather than being read past its end.
func TestShortSkillPacketsAreIgnored(t *testing.T) {
	known := []packets.Skill{{ID: 19, Level: 5, Inf: 1}}

	for _, tc := range []struct {
		name   string
		handle func(*InGameState, []byte) error
		data   []byte
	}{
		{"entry", (*InGameState).handleSkillAdded, skillEntryPacket(142, 4, 1, 0, 0, false)[:10]},
		{"raised", (*InGameState).handleSkillRaised, skillRaisedPacket(19, 6, 34, 9, false)[:7]},
		{"removed", (*InGameState).handleSkillRemoved, skillDeletePacket(19)[:3]},
	} {
		s := &InGameState{skills: append([]packets.Skill(nil), known...)}

		if err := tc.handle(s, tc.data); err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}

		if len(s.skills) != 1 || s.skills[0] != known[0] {
			t.Errorf("%s: a short packet changed the list to %+v", tc.name, s.skills)
		}
	}
}
