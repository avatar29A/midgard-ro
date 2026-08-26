package packets

import "bytes"

// Unit appearance and movement.
//
// Every field offset in this file was produced by tools/packetlen/layout.py
// reading the server's own headers at PACKETVER 20211103, not read off a wiki
// or inferred from a capture. The three packets share a 35-byte prefix and
// then drift apart, because the walking variant inserts a 4-byte start time in
// the middle and carries a 6-byte movement where the others carry a 3-byte
// position. Offsets past that point line up with nothing, so they are written
// out one by one rather than derived from each other.
//
// To re-derive them after a PACKETVER change:
//
//	python3 tools/packetlen/layout.py <rathena-src> packet_idle_unit 20211103
//	python3 tools/packetlen/layout.py <rathena-src> packet_spawn_unit 20211103
//	python3 tools/packetlen/layout.py <rathena-src> packet_unit_walking 20211103

// nameLength is NAME_LENGTH from rAthena's mmo.hpp.
const nameLength = 24

// Base sizes as reported by layout.py. These packets are variable length, so
// the value is a minimum rather than the size on the wire.
const (
	entityIdleBase  = 108
	entitySpawnBase = 107
	entityWalkBase  = 114
)

// Shared prefix, identical in all three packets.
const (
	offObjectType = 4
	offAID        = 5
	offGID        = 9
	offSpeed      = 13
	offJob        = 23
	offHead       = 25
	offWeapon     = 27
	offShield     = 31
	offAccessory  = 35
)

// Fields shared by packet_idle_unit (0x09FF) and packet_spawn_unit (0x09FE),
// which agree with each other from the prefix up to clevel. Past that they
// diverge, because the idle packet carries an extra state byte.
const (
	offUnitHeadPalette = 41
	offUnitBodyPalette = 43
	offUnitHeadDir     = 45
	offUnitRobe        = 47
	offUnitSex         = 62
	offUnitPos         = 63
	offIdleLevel       = 69
	offIdleMaxHP       = 73
	offIdleHP          = 77
	offIdleName        = 84

	offSpawnLevel = 68
	offSpawnMaxHP = 72
	offSpawnHP    = 76
	offSpawnName  = 83
)

// packet_unit_walking (0x09FD) past the prefix. The 4-byte moveStartTime at 37
// pushes everything after it out by four relative to the idle packet.
const (
	offWalkMoveStart   = 37
	offWalkHeadPalette = 45
	offWalkBodyPalette = 47
	offWalkHeadDir     = 49
	offWalkRobe        = 51
	offWalkSex         = 66
	offWalkMoveData    = 67
	offWalkLevel       = 75
	offWalkMaxHP       = 79
	offWalkHP          = 83
	offWalkName        = 90
)

// EntityKind is the packet's objecttype, which says what the unit is and so
// which sprite to draw. Values are rAthena's clif_bl_type (src/map/clif.cpp).
type EntityKind uint8

// Unit kinds as they appear on the wire.
const (
	EntityPlayer     EntityKind = 0x0
	EntityDisguised  EntityKind = 0x1
	EntityItem       EntityKind = 0x2
	EntitySkill      EntityKind = 0x3
	EntityChat       EntityKind = 0x4
	EntityMob        EntityKind = 0x5
	EntityNPC        EntityKind = 0x6
	EntityWalkingNPC EntityKind = 0xC
	EntityABR        EntityKind = 0xD
	EntityBionic     EntityKind = 0xE
)

// IsCharacter reports whether the unit is drawn from the humanoid sprite set,
// which is what decides how it gets composited.
func (k EntityKind) IsCharacter() bool {
	return k == EntityPlayer || k == EntityDisguised
}

// Entity is one unit the server has told us about.
type Entity struct {
	Kind EntityKind

	// AID is the server's block-list id and the only field that identifies
	// every unit: rAthena fills GID with the character id, which is zero for
	// monsters and NPCs. Key units by AID, never by GID.
	AID uint32

	// GID is the character id for players and zero for everything else.
	GID uint32

	// SpeedMs is milliseconds per cell, the same units as a character's own
	// walk speed.
	SpeedMs int16

	Job          int16
	HairStyle    uint16
	HairColor    int16
	ClothesColor int16
	Weapon       uint32
	Shield       uint32
	Accessory    uint16
	Robe         uint16
	HeadDir      int16
	Sex          uint8
	Level        int16
	MaxHP        int32
	HP           int32
	Name         string

	// X, Y is where the unit is. Dir is its facing, in server directions.
	X, Y int
	Dir  int

	// Moving is set when the unit arrived already walking. ToX, ToY is then
	// where it is headed and MoveStartTick when it set off; otherwise they
	// repeat X, Y and are zero.
	Moving        bool
	ToX, ToY      int
	MoveStartTick uint32
}

// DecodeEntityIdle parses ZC_NOTIFY_STANDENTRY (0x09FF), a unit that is
// standing still. Returns nil on short data.
func DecodeEntityIdle(data []byte) *Entity {
	if len(data) < entityIdleBase {
		return nil
	}

	e := decodeEntityPrefix(data)
	e.HairColor = int16(readU16(data, offUnitHeadPalette))
	e.ClothesColor = int16(readU16(data, offUnitBodyPalette))
	e.HeadDir = int16(readU16(data, offUnitHeadDir))
	e.Robe = readU16(data, offUnitRobe)
	e.Sex = data[offUnitSex]
	e.Level = int16(readU16(data, offIdleLevel))
	e.MaxHP = int32(readU32(data, offIdleMaxHP))
	e.HP = int32(readU32(data, offIdleHP))
	e.Name = decodeName(data, offIdleName)

	e.X, e.Y, e.Dir = DecodePos(data[offUnitPos : offUnitPos+PosSize])
	e.ToX, e.ToY = e.X, e.Y
	return e
}

// DecodeEntitySpawn parses ZC_NOTIFY_NEWENTRY (0x09FE), a unit appearing where
// there was none. It differs from the idle packet only in lacking a state
// byte, which shifts everything after the position down by one.
func DecodeEntitySpawn(data []byte) *Entity {
	if len(data) < entitySpawnBase {
		return nil
	}

	e := decodeEntityPrefix(data)
	e.HairColor = int16(readU16(data, offUnitHeadPalette))
	e.ClothesColor = int16(readU16(data, offUnitBodyPalette))
	e.HeadDir = int16(readU16(data, offUnitHeadDir))
	e.Robe = readU16(data, offUnitRobe)
	e.Sex = data[offUnitSex]
	e.Level = int16(readU16(data, offSpawnLevel))
	e.MaxHP = int32(readU32(data, offSpawnMaxHP))
	e.HP = int32(readU32(data, offSpawnHP))
	e.Name = decodeName(data, offSpawnName)

	e.X, e.Y, e.Dir = DecodePos(data[offUnitPos : offUnitPos+PosSize])
	e.ToX, e.ToY = e.X, e.Y
	return e
}

// DecodeEntityWalk parses ZC_NOTIFY_MOVEENTRY (0x09FD), a unit that is already
// walking when we first see it or that has just started. Returns nil on short
// data.
func DecodeEntityWalk(data []byte) *Entity {
	if len(data) < entityWalkBase {
		return nil
	}

	e := decodeEntityPrefix(data)
	e.HairColor = int16(readU16(data, offWalkHeadPalette))
	e.ClothesColor = int16(readU16(data, offWalkBodyPalette))
	e.HeadDir = int16(readU16(data, offWalkHeadDir))
	e.Robe = readU16(data, offWalkRobe)
	e.Sex = data[offWalkSex]
	e.Level = int16(readU16(data, offWalkLevel))
	e.MaxHP = int32(readU32(data, offWalkMaxHP))
	e.HP = int32(readU32(data, offWalkHP))
	e.Name = decodeName(data, offWalkName)
	e.MoveStartTick = readU32(data, offWalkMoveStart)

	x0, y0, x1, y1, _, _ := DecodePos2(data[offWalkMoveData : offWalkMoveData+Pos2Size])
	e.X, e.Y = x0, y0
	e.ToX, e.ToY = x1, y1
	e.Moving = true

	// A walking packet carries no facing of its own; it is implied by the
	// direction of travel. Leaving Dir at zero would face every walker south.
	e.Dir = -1
	return e
}

// DecodeEntityVanish parses ZC_NOTIFY_VANISH (0x0080), which removes a unit.
// The reason distinguishes walking out of sight (0) from dying (1), logging
// out (2), teleporting (3) and playing dead (4).
//
// The id is the block-list id, matching Entity.AID — rAthena names the field
// GID here, but passes bl->id into it, which is not the char id that the unit
// packets call GID.
func DecodeEntityVanish(data []byte) (aid uint32, reason uint8, ok bool) {
	if len(data) < 7 {
		return 0, 0, false
	}
	return readU32(data, 2), data[6], true
}

func decodeEntityPrefix(data []byte) *Entity {
	return &Entity{
		Kind:      EntityKind(data[offObjectType]),
		AID:       readU32(data, offAID),
		GID:       readU32(data, offGID),
		SpeedMs:   int16(readU16(data, offSpeed)),
		Job:       int16(readU16(data, offJob)),
		HairStyle: readU16(data, offHead),
		Weapon:    readU32(data, offWeapon),
		Shield:    readU32(data, offShield),
		Accessory: readU16(data, offAccessory),
	}
}

// decodeName reads a fixed-width NAME_LENGTH field, which the server pads with
// NULs rather than trimming.
func decodeName(data []byte, offset int) string {
	if offset+nameLength > len(data) {
		return ""
	}
	raw := data[offset : offset+nameLength]
	if end := bytes.IndexByte(raw, 0); end >= 0 {
		raw = raw[:end]
	}
	return string(raw)
}
