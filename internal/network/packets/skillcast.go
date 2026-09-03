package packets

import "encoding/binary"

// Casting, and what the server says about it.
//
// Every id and every field offset below was read out of the rAthena the client
// talks to rather than from a table of packet numbers, because both move with
// the packet version and a wrong guess reads a different packet's fields
// without failing.

// CZ_USE_SKILL_TOGROUND asks to cast a skill at a cell: `<level>.W <skill>.W
// <x>.W <y>.W`, 10 bytes.
//
// 0x0366 is what the default branch of clif_shuffle.hpp registers for
// clif_parse_UseSkillToPos, which is the branch that applies at this
// packetver — the same branch that registers 0x0438 for the unit-targeted
// cast we already send.
const CZ_USE_SKILL_TOGROUND uint16 = 0x0366

// EncodeUseSkillAt asks to cast a skill at a cell rather than at a unit.
//
// The level comes before the skill, which is the order rAthena's parse offsets
// declare and the opposite of how the packet reads.
func EncodeUseSkillAt(skillID uint16, level, cellX, cellY int) []byte {
	pkt := make([]byte, 10)
	binary.LittleEndian.PutUint16(pkt, CZ_USE_SKILL_TOGROUND)
	binary.LittleEndian.PutUint16(pkt[2:], uint16(level))
	binary.LittleEndian.PutUint16(pkt[4:], skillID)
	binary.LittleEndian.PutUint16(pkt[6:], uint16(cellX))
	binary.LittleEndian.PutUint16(pkt[8:], uint16(cellY))

	return pkt
}

// The server's side of a cast.
const (
	// ZC_USESKILL_ACK says a cast has begun and how long it takes. 0x0b1a at
	// this packetver, 29 bytes, which is the shape with attackMT on the end.
	ZC_USESKILL_ACK uint16 = 0x0B1A

	// ZC_ACK_TOUSESKILL says a cast was refused, and why. 14 bytes here: the
	// widened btype and itemId that PACKETVER_RE 20180704 brought.
	ZC_ACK_TOUSESKILL uint16 = 0x0110

	// ZC_USE_SKILL is a skill that did no damage — a buff, a heal, a
	// teleport. 0x09cb at this packetver, 17 bytes, level widened to 32 bits.
	ZC_USE_SKILL uint16 = 0x09CB

	// ZC_NOTIFY_SKILL is a skill that did damage. 0x01de and 33 bytes: the
	// 0x0114 shape with the narrow damage field is guarded by PACKETVER < 3.
	ZC_NOTIFY_SKILL uint16 = 0x01DE

	// ZC_NOTIFY_GROUNDSKILL is a skill placed on a cell. 18 bytes, and one
	// shape at every packet version.
	ZC_NOTIFY_GROUNDSKILL uint16 = 0x0117
)

// SkillCast is a cast that has started.
type SkillCast struct {
	// SourceID is who is casting and TargetID what at, which is zero for a
	// cast aimed at a cell — CellX and CellY carry that instead.
	SourceID uint32
	TargetID uint32
	CellX    int
	CellY    int

	SkillID uint16

	// Element is what the cast is made of, which decides the color of the
	// circle drawn under the caster.
	Element uint32

	// CastMs is how long the cast takes. Zero is instant, which most skills
	// are, and there is nothing to draw a bar for.
	CastMs uint32

	// Disposable is rAthena's own field name; it marks a cast that cannot be
	// interrupted.
	Disposable bool
}

// DecodeSkillCast reads ZC_USESKILL_ACK.
func DecodeSkillCast(data []byte) (SkillCast, bool) {
	const size = 29
	if len(data) < size {
		return SkillCast{}, false
	}

	return SkillCast{
		SourceID:   binary.LittleEndian.Uint32(data[2:]),
		TargetID:   binary.LittleEndian.Uint32(data[6:]),
		CellX:      int(binary.LittleEndian.Uint16(data[10:])),
		CellY:      int(binary.LittleEndian.Uint16(data[12:])),
		SkillID:    binary.LittleEndian.Uint16(data[14:]),
		Element:    binary.LittleEndian.Uint32(data[16:]),
		CastMs:     binary.LittleEndian.Uint32(data[20:]),
		Disposable: data[24] != 0,
	}, true
}

// SkillFail is a cast the server refused.
type SkillFail struct {
	SkillID uint16
	ItemID  uint32

	// Cause is rAthena's useskill_fail_cause.
	Cause uint8
}

// DecodeSkillFail reads ZC_ACK_TOUSESKILL.
func DecodeSkillFail(data []byte) (SkillFail, bool) {
	const size = 14
	if len(data) < size {
		return SkillFail{}, false
	}

	return SkillFail{
		SkillID: binary.LittleEndian.Uint16(data[2:]),
		ItemID:  binary.LittleEndian.Uint32(data[8:]),
		Cause:   data[13],
	}, true
}

// Skill failure causes, from rAthena's useskill_fail_cause. Only the ones an
// ordinary character meets are named; the rest are reported by number, which
// is better than a wrong message.
const (
	FailLevel      uint8 = 0
	FailSP         uint8 = 1
	FailHP         uint8 = 2
	FailStuff      uint8 = 3
	FailInterval   uint8 = 4
	FailMoney      uint8 = 5
	FailWeapon     uint8 = 6
	FailWeight     uint8 = 9
	FailGeneric    uint8 = 10
	FailTarget     uint8 = 11
	FailNeedSkill  uint8 = 16
	FailNeedHelper uint8 = 17
	FailDirection  uint8 = 18
	FailDuplicate  uint8 = 22
	FailCondition  uint8 = 23
	FailPlace      uint8 = 26
	FailNeedWall   uint8 = 28
)

// failReasons is what to say for each. Phrased as the original does: what is
// wrong, not what the server called it.
var failReasons = map[uint8]string{
	FailLevel:      "Your skill level is too low.",
	FailSP:         "Not enough SP.",
	FailHP:         "Not enough HP.",
	FailStuff:      "You are missing an item for that.",
	FailInterval:   "That skill is not ready yet.",
	FailMoney:      "Not enough zeny.",
	FailWeapon:     "That skill needs a different weapon.",
	FailWeight:     "You are carrying too much.",
	FailGeneric:    "That skill failed.",
	FailTarget:     "That is not a target for this skill.",
	FailNeedSkill:  "Another skill has to be active first.",
	FailNeedHelper: "That skill needs somebody else.",
	FailDirection:  "You are facing the wrong way.",
	FailDuplicate:  "That skill is already in effect.",
	FailCondition:  "The conditions for that skill are not met.",
	FailPlace:      "That skill cannot be placed there.",
	FailNeedWall:   "That skill has to be cast against a wall.",
}

// Reason is what to tell the player, or "" for a cause with no message of its
// own.
func (f SkillFail) Reason() string {
	return failReasons[f.Cause]
}

// SkillUse is a skill going off: on a unit, or on a cell, and doing damage or
// not. One shape for all three, because what the client does with them is the
// same — draw the caster's motion and the skill's effect where it lands.
type SkillUse struct {
	SourceID uint32
	TargetID uint32
	CellX    int
	CellY    int

	SkillID uint16

	// Level is the skill's level, which only the damaging and the placed
	// shapes carry.
	Level int

	// Amount is the field rAthena's ZC_USE_SKILL struct calls `level` and its
	// own documentation calls `heal`: how much a non-damaging skill did. For
	// Heal it is the hit points restored, which is why it comes back as 281
	// for a level 10 cast and not as 10.
	Amount int

	// Damage is what it did, and Hits how many blows that is spread over.
	// Both are zero for a skill that does none.
	Damage int
	Hits   int

	// Ground marks a skill placed on a cell rather than aimed at a unit.
	Ground bool

	// OK is the server's own result byte on a non-damaging skill: zero means
	// it did not take.
	OK bool
}

// DecodeSkillUse reads ZC_USE_SKILL, a skill that did no damage.
func DecodeSkillUse(data []byte) (SkillUse, bool) {
	const size = 17
	if len(data) < size {
		return SkillUse{}, false
	}

	return SkillUse{
		SkillID:  binary.LittleEndian.Uint16(data[2:]),
		Amount:   int(int32(binary.LittleEndian.Uint32(data[4:]))),
		TargetID: binary.LittleEndian.Uint32(data[8:]),
		SourceID: binary.LittleEndian.Uint32(data[12:]),
		OK:       data[16] != 0,
	}, true
}

// DecodeSkillDamage reads ZC_NOTIFY_SKILL, a skill that did damage.
func DecodeSkillDamage(data []byte) (SkillUse, bool) {
	const size = 33
	if len(data) < size {
		return SkillUse{}, false
	}

	return SkillUse{
		SkillID:  binary.LittleEndian.Uint16(data[2:]),
		SourceID: binary.LittleEndian.Uint32(data[4:]),
		TargetID: binary.LittleEndian.Uint32(data[8:]),
		Damage:   int(int32(binary.LittleEndian.Uint32(data[24:]))),
		Level:    int(int16(binary.LittleEndian.Uint16(data[28:]))),
		Hits:     int(int16(binary.LittleEndian.Uint16(data[30:]))),
		OK:       true,
	}, true
}

// DecodeGroundSkill reads ZC_NOTIFY_GROUNDSKILL, a skill placed on a cell.
func DecodeGroundSkill(data []byte) (SkillUse, bool) {
	const size = 18
	if len(data) < size {
		return SkillUse{}, false
	}

	return SkillUse{
		SkillID:  binary.LittleEndian.Uint16(data[2:]),
		SourceID: binary.LittleEndian.Uint32(data[4:]),
		Level:    int(int16(binary.LittleEndian.Uint16(data[8:]))),
		CellX:    int(int16(binary.LittleEndian.Uint16(data[10:]))),
		CellY:    int(int16(binary.LittleEndian.Uint16(data[12:]))),
		Ground:   true,
		OK:       true,
	}, true
}
