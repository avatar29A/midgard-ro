package packets

import "encoding/binary"

// Leaving the game: back to character select, or out altogether.
//
// The two are different requests with different answers, and the server can
// refuse either — you cannot log out while cloaking or hiding, and rAthena's
// prevent_logout keeps you in for a few seconds after combat. A client that
// tears down its socket on the button press rather than on the answer will
// sometimes drop a session the server was going to keep.
const (
	// CZ_REQ_RESTART asks to respawn or to go back to character select,
	// depending on its one byte. 3 bytes: id, then the type.
	CZ_REQ_RESTART uint16 = 0x00B2

	// CZ_REQ_DISCONNECT asks to leave the game. 4 bytes: id, then a field the
	// server does not read.
	CZ_REQ_DISCONNECT uint16 = 0x018A

	// ZC_RESTART_ACK answers CZ_REQ_RESTART, echoing the type it granted.
	ZC_RESTART_ACK uint16 = 0x00B3

	// ZC_ACK_REQ_DISCONNECT answers CZ_REQ_DISCONNECT: 0 granted, 1 refused.
	ZC_ACK_REQ_DISCONNECT uint16 = 0x018B
)

// ZC_RESURRECTION is the server standing a dead unit back up: `<gid>.L
// <type>.W`, eight bytes.
//
// Which is the only thing that says a character is alive again. Hit points do
// not: rAthena leaves a corpse on one rather than nought, so a client reading
// death off the numbers never sees a death at all.
const ZC_RESURRECTION uint16 = 0x0148

// DecodeResurrection reads whose. Reports false when the packet is too short
// to say.
func DecodeResurrection(data []byte) (uint32, bool) {
	if len(data) < 6 {
		return 0, false
	}

	return binary.LittleEndian.Uint32(data[2:]), true
}

// Restart types, from rAthena's clif_parse_Restart.
const (
	// RestartRespawn returns you to the save point after dying.
	RestartRespawn uint8 = 0

	// RestartCharSelect hands you back to the character server.
	RestartCharSelect uint8 = 1
)

// DisconnectGranted is the result ZC_ACK_REQ_DISCONNECT carries when the
// server is letting you go. Anything else is a refusal.
const DisconnectGranted uint16 = 0

// EncodeRestart builds a CZ_REQ_RESTART for the given type.
func EncodeRestart(restartType uint8) []byte {
	pkt := make([]byte, 3)
	binary.LittleEndian.PutUint16(pkt, CZ_REQ_RESTART)
	pkt[2] = restartType

	return pkt
}

// EncodeDisconnect builds a CZ_REQ_DISCONNECT.
//
// The two bytes after the id are what the packet's declared length wants; the
// server reads its own field out of them and ignores the value.
func EncodeDisconnect() []byte {
	pkt := make([]byte, 4)
	binary.LittleEndian.PutUint16(pkt, CZ_REQ_DISCONNECT)

	return pkt
}

// DecodeRestartAck reads which restart the server granted. Reports false when
// the packet is too short to say.
func DecodeRestartAck(data []byte) (uint8, bool) {
	if len(data) < 3 {
		return 0, false
	}

	return data[2], true
}

// DecodeDisconnectAck reads whether we may leave. Reports false when the
// packet is too short to say.
func DecodeDisconnectAck(data []byte) (uint16, bool) {
	if len(data) < 4 {
		return 0, false
	}

	return binary.LittleEndian.Uint16(data[2:4]), true
}

// The skill list.
//
// The entry layout moved with the packet version and so did the id: before
// PACKETVER 20190807 each entry carried the skill's name and the packet was
// 0x010F; from that version the name is gone and it is 0x0B32. Ours is the
// later one, which is why the client needs a name table of its own — the
// server no longer sends one.
const (
	// ZC_SKILLINFO_LIST is `<len>.W` then a run of 15-byte entries.
	ZC_SKILLINFO_LIST uint16 = 0x0B32

	// skillEntryLen is one entry: id, inf, level, sp, range, upFlag, level2.
	skillEntryLen = 15
)

// Skill is one entry of the list.
type Skill struct {
	ID    uint16
	Level int

	// Inf is what the skill targets. Zero means it targets nothing, which is
	// how the server says passive — the window shows those as "Passive"
	// rather than as an SP cost they do not have.
	Inf int

	// SP is what casting it costs, and Range how far it reaches.
	SP    int
	Range int

	// Raisable is the server saying this skill can be leveled with a skill
	// point right now.
	Raisable bool
}

// DecodeSkillList reads the whole list. Returns nil when the packet is too
// short to hold its own header, and stops at whatever whole entries fit —
// a truncated tail is dropped rather than read past.
func DecodeSkillList(data []byte) []Skill {
	if len(data) < 4 {
		return nil
	}

	// The declared length wins over the buffer's: the framing hands us
	// exactly one packet, but a server that declares less than it sent should
	// not have the remainder read as skills.
	length := int(readU16(data, 2))
	if length > len(data) {
		length = len(data)
	}

	count := (length - 4) / skillEntryLen
	if count <= 0 {
		return nil
	}

	list := make([]Skill, 0, count)
	for i := 0; i < count; i++ {
		at := 4 + i*skillEntryLen

		list = append(list, Skill{
			ID:    readU16(data, at),
			Inf:   int(readU32(data, at+2)),
			Level: int(readU16(data, at+6)),
			SP:    int(readU16(data, at+8)),
			Range: int(readU16(data, at+10)),
			// The byte after the range; level2 follows it.
			Raisable: data[at+12] != 0,
		})
	}

	return list
}

// One skill changing, rather than the whole list again.
//
// The list arrives once, on entering the map, and everything after it is one
// of these — a point spent, a skill granted, a skill taken away by a job
// change. None of them were read, so the window showed whatever was true when
// the map loaded and nothing since: a skill raised stayed at its old level
// until the character logged in again.
const (
	// ZC_ADD_SKILL is a skill appearing in the tree. Its body is one list
	// entry, the same fifteen bytes.
	ZC_ADD_SKILL uint16 = 0x0B31

	// ZC_SKILLINFO_UPDATE is a skill raised with a point:
	// `<id>.W <level>.W <sp>.W <range>.W <upgradable>.B`, eleven bytes.
	//
	// Shorter than an entry because it carries no targeting: the server is
	// answering a point spent on a skill the client already knows about, and
	// what a skill targets does not change with its level.
	ZC_SKILLINFO_UPDATE uint16 = 0x010E

	// ZC_SKILLINFO_UPDATE2 is a skill's whole entry changing, which is what
	// arrives when one becomes available. Its body is a list entry too.
	ZC_SKILLINFO_UPDATE2 uint16 = 0x0B33

	// ZC_SKILLINFO_DELETE is a skill going away: `<id>.W`, four bytes.
	ZC_SKILLINFO_DELETE uint16 = 0x0441
)

// DecodeSkillEntry reads a packet whose body is a single list entry —
// ZC_ADD_SKILL and ZC_SKILLINFO_UPDATE2, which differ only in what made the
// server send them. Reports false when the packet is too short.
func DecodeSkillEntry(data []byte) (Skill, bool) {
	if len(data) < 2+skillEntryLen {
		return Skill{}, false
	}

	return Skill{
		ID:       readU16(data, 2),
		Inf:      int(readU32(data, 4)),
		Level:    int(readU16(data, 8)),
		SP:       int(readU16(data, 10)),
		Range:    int(readU16(data, 12)),
		Raisable: data[14] != 0,
	}, true
}

// DecodeSkillUpdate reads a skill raised with a point.
//
// The targeting comes back as zero because the packet does not carry it. The
// caller keeps whatever it already had rather than writing that in: zero means
// passive, and a Fire Bolt raised to level two is not a passive skill.
func DecodeSkillUpdate(data []byte) (Skill, bool) {
	const size = 11

	if len(data) < size {
		return Skill{}, false
	}

	return Skill{
		ID:       readU16(data, 2),
		Level:    int(readU16(data, 4)),
		SP:       int(readU16(data, 6)),
		Range:    int(readU16(data, 8)),
		Raisable: data[10] != 0,
	}, true
}

// DecodeSkillDelete reads which skill is going away.
func DecodeSkillDelete(data []byte) (uint16, bool) {
	if len(data) < 4 {
		return 0, false
	}

	return readU16(data, 2), true
}

// The inventory.
//
// It arrives as two lists, because the client shows stackables and equipment
// differently: ZC_INVENTORY_ITEMLIST_NORMAL for things that stack and
// ZC_INVENTORY_ITEMLIST_EQUIP for things that are worn. Both share a 5-byte
// header — type, length, and which inventory it is — and then a run of
// fixed-size entries.
//
// Entry sizes are where this packet punishes guesswork: they grew with the
// packet version, and at ours the card slots are four uint32 rather than four
// uint16, which alone is eight bytes a row.
const (
	// ZC_INVENTORY_ITEMLIST_NORMAL is the stackables. 34-byte entries.
	// ZC_USE_ITEM_ACK answers using an item:
	// `<index>.W <itemId>.L <AID>.L <amount>.W <result>.B`, 15 bytes.
	ZC_USE_ITEM_ACK uint16 = 0x01C8

	ZC_INVENTORY_ITEMLIST_NORMAL uint16 = 0x0B09

	// ZC_INVENTORY_ITEMLIST_EQUIP is the equipment. 68-byte entries.
	ZC_INVENTORY_ITEMLIST_EQUIP uint16 = 0x0B39

	itemListHeaderLen = 5
	normalItemLen     = 34
	equipItemLen      = 68
)

// InventoryItem is one line of the inventory.
type InventoryItem struct {
	// Index is the slot the server keeps it in, which is how everything else
	// refers to it.
	Index int

	// ID is the item, and Count how many. Equipment always counts one.
	ID    uint32
	Count int

	// Equipped is set for a worn item, which the list marks with a place on
	// the body rather than a count.
	Equipped bool

	// WearState is where it is worn right now, zero for something merely
	// carried. One of the EQP_ bits, and the only thing that says which slot
	// of the equipment window an item belongs in — EquipPositions says where
	// it *may* go, which for an accessory or a two-hander is more than one
	// place.
	WearState uint32

	// EquipPositions is where on the body this can go, as the server told us.
	// Equipping sends it straight back: the server uses the value rather than
	// working it out, so a client that invents one equips the wrong slot or
	// nothing at all.
	EquipPositions uint32
}

// ItemListRemainder reports how many bytes are left over when a list's body
// is cut into entries of the given size.
//
// Zero means the size fits. Anything else means the layout this was built for
// is not the layout that arrived, and the caller should say so rather than
// display whatever fell out.
func ItemListRemainder(data []byte, entryLen int) int {
	if len(data) < itemListHeaderLen {
		return 0
	}

	length := min(int(readU16(data, 2)), len(data))

	body := length - itemListHeaderLen
	if body <= 0 {
		return 0
	}

	return body % entryLen
}

// NormalItemLen and EquipItemLen are the entry sizes this decodes, exported so
// a caller can report a mismatch against them.
const (
	NormalItemLen = normalItemLen
	EquipItemLen  = equipItemLen
)

// DecodeInventoryNormal reads the stackable half of the inventory.
func DecodeInventoryNormal(data []byte) []InventoryItem {
	return decodeItemList(data, normalItemLen, func(entry []byte) InventoryItem {
		return InventoryItem{
			Index: int(readU16(entry, 0)),
			ID:    readU32(entry, 2),
			Count: int(int16(readU16(entry, 7))),
		}
	})
}

// DecodeInventoryEquip reads the worn half. These have no count — one of each
// — and carry where on the body they sit instead.
func DecodeInventoryEquip(data []byte) []InventoryItem {
	return decodeItemList(data, equipItemLen, func(entry []byte) InventoryItem {
		return InventoryItem{
			Index: int(readU16(entry, 0)),
			ID:    readU32(entry, 2),
			Count: 1,
			// location is where it may go; WearState where it is now, zero
			// for something carried rather than worn.
			EquipPositions: readU32(entry, 7),
			WearState:      readU32(entry, 11),
			Equipped:       readU32(entry, 11) != 0,
		}
	})
}

// decodeItemList walks the entries a list packet carries.
//
// The declared length wins over the buffer's, and a tail too short for a
// whole entry is dropped rather than read past — a list whose entry size does
// not divide its body is a version mismatch, not something to improvise on.
func decodeItemList(data []byte, entryLen int, read func([]byte) InventoryItem) []InventoryItem {
	if len(data) < itemListHeaderLen {
		return nil
	}

	length := int(readU16(data, 2))
	if length > len(data) {
		length = len(data)
	}

	body := length - itemListHeaderLen
	if body <= 0 {
		return nil
	}

	// A body that does not divide by the entry size means the entry size is
	// wrong for this server's packet version — the layout grew and shrank
	// across versions, and at ours the card slots alone are eight bytes wider
	// than they were. Reporting it is the point: a silent misparse shows up
	// as an inventory of nonsense, which is far harder to trace back here.
	if body%entryLen != 0 {
		return nil
	}

	count := body / entryLen

	items := make([]InventoryItem, 0, count)
	for i := 0; i < count; i++ {
		at := itemListHeaderLen + i*entryLen
		items = append(items, read(data[at:at+entryLen]))
	}

	return items
}

// Acting on an item: using one, wearing one, and putting one on the ground.
//
// All three name the item by its inventory index rather than by its id, which
// is why the index the lists carry has to be kept.
const (
	// CZ_USE_ITEM is 13 bytes at our packet version, with the index at 5 and
	// the account id at 9 — not the 8-byte shape the older guard declares.
	// CZ_USE_ITEM is `<index>.W` at offset 7 in a 20-byte packet.
	//
	// Not 0x00A7: that id is registered last as clif_parse_WalkToXY at this
	// packetver, so the old 13-byte form was asking the character to walk to
	// wherever the index happened to look like, and leaving 4 trailing bytes
	// for the server to read as the start of the next packet.
	CZ_USE_ITEM uint16 = 0x009F

	// CZ_REQ_WEAR_EQUIP is `<index>.W <position>.L`, 8 bytes, from
	// PACKETVER 20120925.
	CZ_REQ_WEAR_EQUIP uint16 = 0x0998

	// CZ_ITEM_THROW drops some of a stack on the ground:
	// `<index>.W <amount>.W`.
	//
	// Not 0x00A2, which the packet table registers as clif_parse_SolveCharName
	// at 14 bytes, and not 0x0362 either: clif_shuffle.hpp reassigns that one
	// to clif_parse_TakeItem at this packetver, so dropping on it would be
	// read as picking up. Both are six bytes, which is why the length check
	// alone did not notice.
	CZ_ITEM_THROW uint16 = 0x0363
)

// EncodeUseItem asks to use the item in an inventory slot.
//
// The index is the one the inventory list gave us, sent unchanged: the server
// subtracts 2 itself, the same way it does for equipping. Everything past the
// index is padding — clif_parse_UseItem reads that one field and nothing else,
// but the packet still has to be the full 20 bytes the server will consume.
func EncodeUseItem(index int) []byte {
	pkt := make([]byte, 20)
	binary.LittleEndian.PutUint16(pkt, CZ_USE_ITEM)
	binary.LittleEndian.PutUint16(pkt[7:], uint16(index))

	return pkt
}

// EncodeEquipItem asks to wear the item in an inventory slot.
//
// The position is the item's own, as the server reported it in the equip
// list: rAthena passes it straight to pc_equipitem rather than deriving one.
func EncodeEquipItem(index int, position uint32) []byte {
	pkt := make([]byte, 8)
	binary.LittleEndian.PutUint16(pkt, CZ_REQ_WEAR_EQUIP)
	binary.LittleEndian.PutUint16(pkt[2:], uint16(index))
	binary.LittleEndian.PutUint32(pkt[4:], position)

	return pkt
}

// EncodeDropItem asks to put some of a stack on the ground.
func EncodeDropItem(index, amount int) []byte {
	pkt := make([]byte, 6)
	binary.LittleEndian.PutUint16(pkt, CZ_ITEM_THROW)
	binary.LittleEndian.PutUint16(pkt[2:], uint16(index))
	binary.LittleEndian.PutUint16(pkt[4:], uint16(amount))

	return pkt
}

// UseItemAck is the server's answer to using an item.
type UseItemAck struct {
	// Index is the inventory slot, in the same form the inventory list gave it
	// and the same form we sent: the server adds the 2 back on its way out.
	Index int

	// ItemID is what was used.
	ItemID uint32

	// AccountID is who used it. rAthena sends the success case to everyone
	// nearby rather than only to us, so this has to be checked before the ack
	// is allowed to change our own inventory.
	AccountID uint32

	// Amount is how many are left, not how many were spent.
	Amount int

	// OK is false when the server refused — too heavy, cannot be used here,
	// or a slot that no longer holds what we thought.
	OK bool
}

// DecodeUseItemAck reads ZC_USE_ITEM_ACK, reporting false if it is short.
func DecodeUseItemAck(data []byte) (UseItemAck, bool) {
	if len(data) < 15 {
		return UseItemAck{}, false
	}

	return UseItemAck{
		Index:     int(binary.LittleEndian.Uint16(data[2:])),
		ItemID:    binary.LittleEndian.Uint32(data[4:]),
		AccountID: binary.LittleEndian.Uint32(data[8:]),
		Amount:    int(binary.LittleEndian.Uint16(data[12:])),
		OK:        data[14] != 0,
	}, true
}
