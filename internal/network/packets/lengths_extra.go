package packets

// Packet lengths the generator cannot see.
//
// tools/packetlen/gen.py reads two things: rAthena's packet_db, and the
// structs that carry a DEFINE_PACKET_HEADER. Some packets have neither. The
// inventory lists are declared as plain enum constants —
//
//	inventorylistnormalType = 0xb09,
//	inventorylistequipType  = 0xb39,
//
// — with their layout in a struct that names no id, so nothing ties the two
// together for a parser to follow.
//
// The cost of the gap is not a missing feature: RO frames by length alone, so
// an id the table does not know makes the reader resynchronise and throw away
// whatever follows. That is what it did here — "unknown packet id,
// resynchronising 0x0B09" — and the inventory never reached its handler.
//
// Entries here are hand-kept, which is why each says where its id came from.
// Prefer teaching the generator when a whole family is missing; this is for
// the strays.
var extraPacketLengths = map[uint16]int{
	// src/map/packets_struct.hpp, inventorylistnormalType for
	// PACKETVER_RE >= 20180912. struct packet_itemlist_normal carries a
	// PacketLength, so the size is on the wire.
	ZC_INVENTORY_ITEMLIST_NORMAL: VariableLength,

	// The same, inventorylistequipType for PACKETVER_RE >= 20200723.
	ZC_INVENTORY_ITEMLIST_EQUIP: VariableLength,
}
