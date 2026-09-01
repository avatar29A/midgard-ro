package packets

import "encoding/binary"

// Where on the body a piece of equipment goes.
//
// These are rAthena's own equip_pos, read out of src/common/mmo.hpp rather
// than from a wiki: the two accessory bits in particular are not where they
// look like they should be — the right accessory is 0x08 and the left 0x80,
// with armour, garment and shoes filling the gaps between them. Ordering them
// by slot rather than by value is what the server does, and inventing an order
// here would put rings on feet.
//
// A single item can name several bits at once. A two-handed weapon names both
// hands, and an accessory names both accessory slots — for those the server
// picks which one to fill, so the client sends what it was given and reads the
// answer back out of the ack.
const (
	EQP_HEAD_LOW uint32 = 0x000001
	EQP_HAND_R   uint32 = 0x000002
	EQP_GARMENT  uint32 = 0x000004
	EQP_ACC_R    uint32 = 0x000008
	EQP_ARMOR    uint32 = 0x000010
	EQP_HAND_L   uint32 = 0x000020
	EQP_SHOES    uint32 = 0x000040
	EQP_ACC_L    uint32 = 0x000080
	EQP_HEAD_TOP uint32 = 0x000100
	EQP_HEAD_MID uint32 = 0x000200

	EQP_AMMO uint32 = 0x008000
)

// The answers to asking for something to be worn or taken off.
const (
	// ZC_REQ_WEAR_EQUIP_ACK is `<index>.W <wearLocation>.L <sprite>.W
	// <result>.B`, 11 bytes.
	//
	// The id and the shape both come from rAthena's own guards at this
	// packetver: PACKETVER_RE_NUM >= 20121107 selects the 0x0999 form with a
	// 32-bit wearLocation, and packet_db agreeing at 11 bytes is the same
	// struct counted a second way.
	ZC_REQ_WEAR_EQUIP_ACK uint16 = 0x0999

	// ZC_REQ_TAKEOFF_EQUIP_ACK is `<index>.W <wearLocation>.L <flag>.B`,
	// 9 bytes, selected by PACKETVER >= 20130000.
	ZC_REQ_TAKEOFF_EQUIP_ACK uint16 = 0x099A
)

// Why the server refused to equip something, as clif_equipitemack_flag names
// them at this packetver.
//
// The values are inverted below 20121107 — a zero there means failure rather
// than success — so these are only right for the version we speak, and the
// guard that picks them is worth remembering if that ever moves.
const (
	EquipAckOK        uint8 = 0
	EquipAckFailLevel uint8 = 1
	EquipAckFail      uint8 = 2
)

// EquipAck is the server's answer to wearing something.
type EquipAck struct {
	// Index is the inventory slot, in the same numbering the item list uses.
	Index int

	// Position is where the server actually put it, which is not always what
	// was asked for: an accessory can name both slots and the server chooses
	// one. This is the value to file the item under.
	Position uint32

	// Sprite is the weapon or headgear view id, non-zero only for something
	// that shows on the character.
	Sprite uint16

	// Result is the raw flag, kept so a refusal can say which kind it was.
	Result uint8
}

// OK reports whether the item is now worn.
func (a EquipAck) OK() bool { return a.Result == EquipAckOK }

// Reason describes a refusal in the words the original uses, for the chat box.
// Empty when nothing was refused.
func (a EquipAck) Reason() string {
	switch a.Result {
	case EquipAckOK:
		return ""
	case EquipAckFailLevel:
		return "You are not high enough level to equip that."
	default:
		return "You cannot equip that."
	}
}

// DecodeEquipAck reads ZC_REQ_WEAR_EQUIP_ACK.
func DecodeEquipAck(data []byte) (EquipAck, bool) {
	if len(data) < 11 {
		return EquipAck{}, false
	}

	return EquipAck{
		Index:    int(binary.LittleEndian.Uint16(data[2:])),
		Position: binary.LittleEndian.Uint32(data[4:]),
		Sprite:   binary.LittleEndian.Uint16(data[8:]),
		Result:   data[10],
	}, true
}

// UnequipAck is the server's answer to taking something off.
type UnequipAck struct {
	Index    int
	Position uint32

	// OK is whether it came off. The flag on the wire is inverted at this
	// packetver — clif_unequipitemack does `success = !success` for anything
	// past 20110824 — so zero is success here and one is refusal. Reading it
	// the other way round makes every successful unequip look refused, and
	// every refusal look like it worked.
	OK bool
}

// DecodeUnequipAck reads ZC_REQ_TAKEOFF_EQUIP_ACK.
func DecodeUnequipAck(data []byte) (UnequipAck, bool) {
	if len(data) < 9 {
		return UnequipAck{}, false
	}

	return UnequipAck{
		Index:    int(binary.LittleEndian.Uint16(data[2:])),
		Position: binary.LittleEndian.Uint32(data[4:]),
		OK:       data[8] == 0,
	}, true
}

// EquipSlots are the places the equipment window has room for, which is every
// bit an ordinary character can fill.
//
// Costume and shadow gear are left out deliberately: the original keeps them
// on their own tabs of the window, and folding them into these ten would show
// a costume hat sitting in the hat slot.
var EquipSlots = []uint32{
	EQP_HEAD_TOP, EQP_HEAD_MID, EQP_HEAD_LOW,
	EQP_ARMOR, EQP_GARMENT, EQP_SHOES,
	EQP_HAND_R, EQP_HAND_L,
	EQP_ACC_R, EQP_ACC_L,
}

// CZ_CONFIG carries one of the switches the interface offers:
// `<type>.L <flag>.L`, 10 bytes.
//
// Only in the main packet table, not the shuffle, so this id holds at every
// packetver the shuffle covers.
const CZ_CONFIG uint16 = 0x02D8

// ConfigShowEquipment is the equipment window's own checkbox, which decides
// whether other players may look at what you are wearing.
//
// The first of rAthena's e_config_type, and the only one offered here — the
// rest are about pets, homunculi and costumes.
const ConfigShowEquipment uint32 = 0

// EncodeConfig sets one of those switches.
func EncodeConfig(setting uint32, on bool) []byte {
	pkt := make([]byte, 10)
	binary.LittleEndian.PutUint16(pkt, CZ_CONFIG)
	binary.LittleEndian.PutUint32(pkt[2:], setting)

	if on {
		binary.LittleEndian.PutUint32(pkt[6:], 1)
	}

	return pkt
}

// ZC_CONFIG_NOTIFY is the server stating the equipment switch: `<flag>.B`,
// 3 bytes. Sent once as the map is entered, and again whenever it changes.
const ZC_CONFIG_NOTIFY uint16 = 0x02DA

// DecodeConfigNotify reads it.
func DecodeConfigNotify(data []byte) (bool, bool) {
	if len(data) < 3 {
		return false, false
	}

	return data[2] != 0, true
}

// ZC_SPRITE_CHANGE says one thing about how a unit looks has changed:
// `<AID>.L <type>.B <val>.L <val2>.L`, 15 bytes.
//
// The length is not what packet_db declares. rAthena's own guard —
// PACKETVER_RE_NUM >= 20180704 — widens val and val2 from uint16 to uint32,
// which is the 15-byte shape; the table still carries the 11 of the older
// one. Same disagreement as ZC_ITEM_FALL_ENTRY, and settled the same way:
// what the client has to match is the clif_send, which writes sizeof(struct).
const ZC_SPRITE_CHANGE uint16 = 0x01D7

// What a sprite change is about, from rAthena's LOOK_ enumeration. Only the
// ones that change what is drawn are named.
const (
	LookBase       uint8 = 0
	LookHair       uint8 = 1
	LookWeapon     uint8 = 2
	LookHeadBottom uint8 = 3
	LookHeadTop    uint8 = 4
	LookHeadMid    uint8 = 5
	LookShield     uint8 = 8
	LookRobe       uint8 = 12
)

// SpriteChange is one of those, decoded.
type SpriteChange struct {
	AID  uint32
	Look uint8

	// Value is the new look, and Value2 the second half of it — a weapon's
	// shield, for the one type that carries two.
	Value  uint32
	Value2 uint32
}

// DecodeSpriteChange reads ZC_SPRITE_CHANGE.
func DecodeSpriteChange(data []byte) (SpriteChange, bool) {
	if len(data) < 15 {
		return SpriteChange{}, false
	}

	return SpriteChange{
		AID:    binary.LittleEndian.Uint32(data[2:]),
		Look:   data[6],
		Value:  binary.LittleEndian.Uint32(data[7:]),
		Value2: binary.LittleEndian.Uint32(data[11:]),
	}, true
}
