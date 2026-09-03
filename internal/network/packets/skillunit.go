package packets

import "encoding/binary"

// Ground skills leave something behind.
//
// Thunder Storm, Fire Wall, Storm Gust and the rest do not damage from the
// caster: they put a unit on a cell, and that unit does the damage. Its blows
// arrive as ordinary skill damage whose source is the unit's own block id,
// which belongs to nobody the client has otherwise heard of — so without this
// a player's own Thunder Storm reads as somebody else's fight.

// ZC_SKILL_ENTRY is `<length>.W <id>.L <creatorID>.L <x>.W <y>.W <unitID>.L
// <range>.B <visible>.B <level>.B`, twenty-three bytes.
const ZC_SKILL_ENTRY uint16 = 0x09CA

// ZC_SKILL_DISAPPEAR is `<id>.L`, six bytes: the unit is gone.
const ZC_SKILL_DISAPPEAR uint16 = 0x0120

// SkillUnit is one of them standing on a cell.
type SkillUnit struct {
	// ID is the unit's own block id, which is what its damage says it came
	// from, and CreatorID the account that put it there.
	ID        uint32
	CreatorID uint32

	X, Y int

	// Kind is the unit's own type — which shape the original draws for it —
	// and Range how many cells either way it covers.
	Kind  uint32
	Range int

	Visible bool
	Level   int
}

// DecodeSkillUnit reads one. Returns false on short data.
func DecodeSkillUnit(data []byte) (SkillUnit, bool) {
	const size = 23

	if len(data) < size {
		return SkillUnit{}, false
	}

	return SkillUnit{
		ID:        binary.LittleEndian.Uint32(data[4:]),
		CreatorID: binary.LittleEndian.Uint32(data[8:]),
		X:         int(binary.LittleEndian.Uint16(data[12:])),
		Y:         int(binary.LittleEndian.Uint16(data[14:])),
		Kind:      binary.LittleEndian.Uint32(data[16:]),
		Range:     int(data[20]),
		Visible:   data[21] != 0,
		Level:     int(data[22]),
	}, true
}

// DecodeSkillUnitGone reads which unit has gone.
func DecodeSkillUnitGone(data []byte) (uint32, bool) {
	const size = 6

	if len(data) < size {
		return 0, false
	}

	return binary.LittleEndian.Uint32(data[2:]), true
}
