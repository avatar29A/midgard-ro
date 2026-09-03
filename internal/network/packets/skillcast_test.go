package packets

import (
	"encoding/binary"
	"testing"
)

// TestEncodeUseSkillAtPutsTheLevelFirst: rAthena's parse offsets read the
// level at 2 and the skill at 4, which is the opposite way round from how the
// packet reads. Swapping them casts skill 5 at level 1 as skill 1 at level 5.
func TestEncodeUseSkillAtPutsTheLevelFirst(t *testing.T) {
	pkt := EncodeUseSkillAt(28, 3, 150, 220)

	if len(pkt) != 10 {
		t.Fatalf("packet is %d bytes, want 10", len(pkt))
	}
	if got := binary.LittleEndian.Uint16(pkt); got != CZ_USE_SKILL_TOGROUND {
		t.Errorf("header is %#x, want %#x", got, CZ_USE_SKILL_TOGROUND)
	}
	if got := binary.LittleEndian.Uint16(pkt[2:]); got != 3 {
		t.Errorf("level is at the wrong offset: %d", got)
	}
	if got := binary.LittleEndian.Uint16(pkt[4:]); got != 28 {
		t.Errorf("skill is at the wrong offset: %d", got)
	}
	if x, y := binary.LittleEndian.Uint16(pkt[6:]), binary.LittleEndian.Uint16(pkt[8:]); x != 150 || y != 220 {
		t.Errorf("cell is %d,%d, want 150,220", x, y)
	}
}

// castAck builds a ZC_USESKILL_ACK the way clif_skillcasting does.
func castAck(src, dst uint32, x, y int, skill uint16, element, castMs uint32) []byte {
	pkt := make([]byte, 29)
	binary.LittleEndian.PutUint16(pkt, ZC_USESKILL_ACK)
	binary.LittleEndian.PutUint32(pkt[2:], src)
	binary.LittleEndian.PutUint32(pkt[6:], dst)
	binary.LittleEndian.PutUint16(pkt[10:], uint16(x))
	binary.LittleEndian.PutUint16(pkt[12:], uint16(y))
	binary.LittleEndian.PutUint16(pkt[14:], skill)
	binary.LittleEndian.PutUint32(pkt[16:], element)
	binary.LittleEndian.PutUint32(pkt[20:], castMs)

	return pkt
}

// TestDecodeSkillCast: the fields come out where the 0x0b1a struct puts them.
// The element sits between the skill and the cast time, and reading past it
// takes the top half of the cast time for the whole of it.
func TestDecodeSkillCast(t *testing.T) {
	cast, ok := DecodeSkillCast(castAck(2000000, 110030001, 150, 220, 14, 3, 1500))
	if !ok {
		t.Fatal("a full-length cast did not decode")
	}

	if cast.SourceID != 2000000 || cast.TargetID != 110030001 {
		t.Errorf("source/target = %d/%d", cast.SourceID, cast.TargetID)
	}
	if cast.CellX != 150 || cast.CellY != 220 {
		t.Errorf("cell = %d,%d, want 150,220", cast.CellX, cast.CellY)
	}
	if cast.SkillID != 14 {
		t.Errorf("skill = %d, want 14", cast.SkillID)
	}
	if cast.Element != 3 {
		t.Errorf("element = %d, want 3", cast.Element)
	}
	if cast.CastMs != 1500 {
		t.Errorf("cast time = %d, want 1500", cast.CastMs)
	}
}

// TestDecodeSkillCastRejectsShort: a truncated packet must be refused rather
// than read past its end.
func TestDecodeSkillCastRejectsShort(t *testing.T) {
	if _, ok := DecodeSkillCast(castAck(1, 2, 3, 4, 5, 0, 0)[:28]); ok {
		t.Error("a short cast decoded")
	}
}

// TestDecodeSkillFail: the cause is the last byte, past the widened btype and
// item id that PACKETVER_RE 20180704 brought. Reading the older offsets takes
// the flag for the cause and says "your skill level is too low" for
// everything.
func TestDecodeSkillFail(t *testing.T) {
	pkt := make([]byte, 14)
	binary.LittleEndian.PutUint16(pkt, ZC_ACK_TOUSESKILL)
	binary.LittleEndian.PutUint16(pkt[2:], 28)
	binary.LittleEndian.PutUint32(pkt[8:], 501)
	pkt[13] = FailSP

	fail, ok := DecodeSkillFail(pkt)
	if !ok {
		t.Fatal("a full-length refusal did not decode")
	}
	if fail.SkillID != 28 {
		t.Errorf("skill = %d, want 28", fail.SkillID)
	}
	if fail.ItemID != 501 {
		t.Errorf("item = %d, want 501", fail.ItemID)
	}
	if fail.Cause != FailSP {
		t.Errorf("cause = %d, want %d", fail.Cause, FailSP)
	}
	if fail.Reason() == "" {
		t.Error("running out of SP has nothing to say")
	}
}

// TestEveryNamedFailureHasSomethingToSay: a cause named here and left without
// a message would refuse the cast in silence, which is the thing this is for.
func TestEveryNamedFailureHasSomethingToSay(t *testing.T) {
	for _, cause := range []uint8{
		FailLevel, FailSP, FailHP, FailStuff, FailInterval, FailMoney,
		FailWeapon, FailWeight, FailGeneric, FailTarget, FailNeedSkill,
		FailNeedHelper, FailDirection, FailDuplicate, FailCondition,
		FailPlace, FailNeedWall,
	} {
		if (SkillFail{Cause: cause}).Reason() == "" {
			t.Errorf("cause %d has no message", cause)
		}
	}

	// And one nobody named says nothing rather than something wrong.
	if got := (SkillFail{Cause: 200}).Reason(); got != "" {
		t.Errorf("an unnamed cause said %q", got)
	}
}

// TestDecodeSkillUse: a skill that did no damage. The level is 32 bits at this
// packetver, so the target starts at 8 rather than at 6.
func TestDecodeSkillUse(t *testing.T) {
	pkt := make([]byte, 17)
	binary.LittleEndian.PutUint16(pkt, ZC_USE_SKILL)
	binary.LittleEndian.PutUint16(pkt[2:], 28)
	binary.LittleEndian.PutUint32(pkt[4:], 5)
	binary.LittleEndian.PutUint32(pkt[8:], 110030001)
	binary.LittleEndian.PutUint32(pkt[12:], 2000000)
	pkt[16] = 1

	use, ok := DecodeSkillUse(pkt)
	if !ok {
		t.Fatal("a full-length use did not decode")
	}
	// Five is what rAthena writes into the field its struct calls `level` and
	// its own comment calls `heal`. For Heal that is hit points, not a level.
	if use.SkillID != 28 || use.Amount != 5 {
		t.Errorf("skill/amount = %d/%d, want 28/5", use.SkillID, use.Amount)
	}
	if use.Level != 0 {
		t.Errorf("a non-damaging skill reported level %d, which it does not carry", use.Level)
	}
	if use.SourceID != 2000000 || use.TargetID != 110030001 {
		t.Errorf("source/target = %d/%d", use.SourceID, use.TargetID)
	}
	if !use.OK {
		t.Error("a result of one should mean it took")
	}
	if use.Ground {
		t.Error("a skill aimed at a unit is not a ground skill")
	}
}

// TestDecodeSkillDamage: the damaging shape, whose damage is 32 bits — the
// narrow one is guarded by PACKETVER < 3, so a client this new never sees it.
func TestDecodeSkillDamage(t *testing.T) {
	pkt := make([]byte, 33)
	binary.LittleEndian.PutUint16(pkt, ZC_NOTIFY_SKILL)
	binary.LittleEndian.PutUint16(pkt[2:], 5)
	binary.LittleEndian.PutUint32(pkt[4:], 2000000)
	binary.LittleEndian.PutUint32(pkt[8:], 110030001)
	binary.LittleEndian.PutUint32(pkt[24:], 70000)
	binary.LittleEndian.PutUint16(pkt[28:], 10)
	binary.LittleEndian.PutUint16(pkt[30:], 3)

	use, ok := DecodeSkillDamage(pkt)
	if !ok {
		t.Fatal("a full-length damaging skill did not decode")
	}
	if use.Damage != 70000 {
		t.Errorf("damage = %d, want 70000 — which does not fit in the narrow field", use.Damage)
	}
	if use.Level != 10 || use.Hits != 3 {
		t.Errorf("level/hits = %d/%d, want 10/3", use.Level, use.Hits)
	}
}

// TestDecodeGroundSkill: a skill placed on a cell names no target, and the
// cell is what says where it went.
func TestDecodeGroundSkill(t *testing.T) {
	pkt := make([]byte, 18)
	binary.LittleEndian.PutUint16(pkt, ZC_NOTIFY_GROUNDSKILL)
	binary.LittleEndian.PutUint16(pkt[2:], 80)
	binary.LittleEndian.PutUint32(pkt[4:], 2000000)
	binary.LittleEndian.PutUint16(pkt[8:], 5)
	binary.LittleEndian.PutUint16(pkt[10:], 150)
	binary.LittleEndian.PutUint16(pkt[12:], 220)

	use, ok := DecodeGroundSkill(pkt)
	if !ok {
		t.Fatal("a full-length ground skill did not decode")
	}
	if !use.Ground {
		t.Error("a placed skill is not marked as one")
	}
	if use.CellX != 150 || use.CellY != 220 {
		t.Errorf("cell = %d,%d, want 150,220", use.CellX, use.CellY)
	}
	if use.TargetID != 0 {
		t.Errorf("a placed skill named a target: %d", use.TargetID)
	}
}
