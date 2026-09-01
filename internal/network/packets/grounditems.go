package packets

import "encoding/binary"

// Ground item packets: an item lying on the map, how it gets there, and how it
// is picked back up.
//
// Three of these four ids are version-shuffled, and the numbers a wiki gives
// are for an older client than ours. They are written here as the generated
// tables resolved them at PACKETVER 20211103, and the tests in
// lengths_test.go pin each length against those tables rather than against
// these comments.
const (
	// ZC_ITEM_ENTRY is an item already lying there when we arrive, sent to us
	// alone: `<AID>.L <itemId>.L <identified>.B <x>.W <y>.W <amount>.W
	// <subX>.B <subY>.B`, 19 bytes.
	//
	// 19 rather than the 17 usually quoted: the item id widened from uint16
	// to uint32 at PACKETVER_RE 20180704, which moves every field after it.
	ZC_ITEM_ENTRY uint16 = 0x009D

	// ZC_ITEM_FALL_ENTRY is an item dropping now, broadcast to everyone who
	// can see the cell: `<ITAID>.L <ITID>.L <type>.W <identified>.B <x>.W
	// <y>.W <subX>.B <subY>.B <amount>.W <showDropEffect>.B
	// <dropEffectMode>.W`.
	//
	// Not 0x009E. rAthena picks this id through the dropflooritemType enum,
	// which resolves to 0x0ADD at our packetver — 0x009E is the original and
	// would simply never arrive. Note also that this orders subX/subY before
	// the amount, where ZC_ITEM_ENTRY puts them after it.
	ZC_ITEM_FALL_ENTRY uint16 = 0x0ADD

	// ZC_ITEM_DISAPPEAR is an item leaving the ground — picked up by someone,
	// or expired: `<ITAID>.L`, 6 bytes.
	ZC_ITEM_DISAPPEAR uint16 = 0x00A1

	// ZC_ITEM_PICKUP_ACK answers our pick-up, and is the only thing that says
	// whether it worked. 70 bytes at this packetver, most of it the item
	// detail the inventory already knows how to ask for.
	ZC_ITEM_PICKUP_ACK uint16 = 0x0B41

	// ZC_ITEM_THROW_ACK answers our drop: `<index>.W <count>.W`, 6 bytes.
	//
	// A count of zero means the server refused and kept the item — rAthena
	// sends the same packet either way, "because the client does not like
	// being ignored".
	ZC_ITEM_THROW_ACK uint16 = 0x00AF

	// CZ_ITEM_PICKUP asks to pick an item up: `<ITAID>.L`, 6 bytes.
	//
	// clif_shuffle.hpp settles this at our packetver, and it disagrees with
	// the main packet table: 0x0362 is clif_parse_TakeItem here, while 0x07E4
	// — which the main table calls TakeItem — is reassigned to
	// clif_parse_ItemListWindowSelected as a variable-length packet. Sending
	// on 0x07E4 makes the server read the ground item id as a length and drop
	// the connection, which is exactly what it did.
	CZ_ITEM_PICKUP uint16 = 0x0362
)

// GroundItem is an item lying on the map.
type GroundItem struct {
	// ID is the ground object's own id, which is what a pick-up names. It is
	// not the item id and not an account id, though the entry packet calls
	// the field AID.
	ID uint32

	// ItemID is what the item is, for its name and its sprite.
	ItemID uint32

	// X and Y are the cell. SubX and SubY place it within that cell, in
	// sixteenths, so several items in one cell do not stack up on the exact
	// same point.
	X, Y       int
	SubX, SubY int

	// Amount is how many are in the pile.
	Amount int

	// Identified is false for an unidentified drop.
	Identified bool
}

// DecodeGroundItemEntry reads ZC_ITEM_ENTRY, an item already on the ground.
func DecodeGroundItemEntry(data []byte) (GroundItem, bool) {
	if len(data) < 19 {
		return GroundItem{}, false
	}

	return GroundItem{
		ID:         binary.LittleEndian.Uint32(data[2:]),
		ItemID:     binary.LittleEndian.Uint32(data[6:]),
		Identified: data[10] != 0,
		X:          int(int16(binary.LittleEndian.Uint16(data[11:]))),
		Y:          int(int16(binary.LittleEndian.Uint16(data[13:]))),
		Amount:     int(int16(binary.LittleEndian.Uint16(data[15:]))),
		SubX:       int(data[17]),
		SubY:       int(data[18]),
	}, true
}

// DecodeGroundItemFall reads ZC_ITEM_FALL_ENTRY, an item dropping now.
//
// The trailing drop-effect fields are not read: they choose a pillar of light
// for rare drops, which we do not draw. They are still part of the packet's
// length, and getting that wrong would desynchronise the connection rather
// than lose an effect.
func DecodeGroundItemFall(data []byte) (GroundItem, bool) {
	if len(data) < 21 {
		return GroundItem{}, false
	}

	return GroundItem{
		ID:         binary.LittleEndian.Uint32(data[2:]),
		ItemID:     binary.LittleEndian.Uint32(data[6:]),
		Identified: data[12] != 0,
		X:          int(int16(binary.LittleEndian.Uint16(data[13:]))),
		Y:          int(int16(binary.LittleEndian.Uint16(data[15:]))),
		SubX:       int(data[17]),
		SubY:       int(data[18]),
		Amount:     int(int16(binary.LittleEndian.Uint16(data[19:]))),
	}, true
}

// DecodeGroundItemGone reads ZC_ITEM_DISAPPEAR and returns the ground id.
func DecodeGroundItemGone(data []byte) (uint32, bool) {
	if len(data) < 6 {
		return 0, false
	}

	return binary.LittleEndian.Uint32(data[2:]), true
}

// PickupAck is the server's answer to picking something up.
type PickupAck struct {
	// Index is the inventory slot it landed in, in the same form the
	// inventory list uses.
	Index int

	// ItemID and Amount describe what arrived.
	ItemID uint32
	Amount int

	// Result is zero on success. Anything else is a refusal — too heavy, no
	// room, out of range — and the item stays where it is.
	Result uint8
}

// OK reports whether the pick-up succeeded.
func (a PickupAck) OK() bool { return a.Result == 0 }

// DecodeItemPickupAck reads ZC_ITEM_PICKUP_ACK.
//
// The result is at offset 33, on the far side of a sixteen-byte card block
// and the equip location — not next to the identify flag where it looks like
// it should be. Everything past it is expiry, bind type, random options and
// grade, which the inventory list carries anyway.
func DecodeItemPickupAck(data []byte) (PickupAck, bool) {
	if len(data) < 34 {
		return PickupAck{}, false
	}

	return PickupAck{
		Index:  int(binary.LittleEndian.Uint16(data[2:])),
		Amount: int(binary.LittleEndian.Uint16(data[4:])),
		ItemID: binary.LittleEndian.Uint32(data[6:]),
		Result: data[33],
	}, true
}

// EncodePickUpItem asks to pick up the item with this ground id.
func EncodePickUpItem(groundID uint32) []byte {
	pkt := make([]byte, 6)
	binary.LittleEndian.PutUint16(pkt, CZ_ITEM_PICKUP)
	binary.LittleEndian.PutUint32(pkt[2:], groundID)

	return pkt
}

// DropAck is the server's answer to dropping something.
type DropAck struct {
	// Index is the inventory slot, as the inventory list numbers it.
	Index int

	// Count is how many left the bag. Zero means the drop was refused and
	// nothing changed.
	Count int
}

// DecodeDropAck reads ZC_ITEM_THROW_ACK.
func DecodeDropAck(data []byte) (DropAck, bool) {
	if len(data) < 6 {
		return DropAck{}, false
	}

	return DropAck{
		Index: int(binary.LittleEndian.Uint16(data[2:])),
		Count: int(binary.LittleEndian.Uint16(data[4:])),
	}, true
}
