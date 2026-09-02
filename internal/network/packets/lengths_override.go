package packets

// Lengths where rAthena's own packet table disagrees with what the server
// actually puts on the wire.
//
// The generator prefers packet_db, because that is the server's own statement
// of how long a packet is. For server-to-client packets that statement is not
// always maintained: nothing in the server reads it back, so a stale entry
// costs rAthena nothing and goes unnoticed. What the client has to match is
// the clif_send call, which writes sizeof(struct).
//
// These override the generated table rather than filling gaps in it. Each one
// needs the evidence for why, because "the table is wrong" is the conclusion
// of last resort and is usually a misread struct instead.
var packetLengthOverrides = map[uint16]int{
	// ZC_ITEM_FALL_ENTRY. packet_db says 22; clif_dropflooritem sends
	// sizeof(packet_dropflooritem), which is 24 at PACKETVER_RE 20211103.
	//
	// The 22 is the pre-20180704 shape, before the item id widened from
	// uint16 to uint32 — the same widening that makes ZC_ITEM_ENTRY 19 rather
	// than 17. The packet_db line was never updated to match.
	//
	// Confirmed on the wire rather than argued from the source: with 22 the
	// reader logged one "resynchronising 0x0000, skipped: 2" per dropped
	// item, which is exactly the two bytes it was leaving behind.
	ZC_ITEM_FALL_ENTRY: 24,

	// ZC_SPRITE_CHANGE. packet_db says 11; clif_sprite_change sends
	// sizeof(PACKET_ZC_SPRITE_CHANGE), which is 15 at PACKETVER_RE 20211103.
	//
	// The 11 is the pre-20180704 shape, before val and val2 widened from
	// uint16 to uint32 — the guard is rAthena's own, in packets_struct.hpp,
	// and it is the same widening that made ZC_ITEM_FALL_ENTRY 24 rather than
	// 22. The table line was never updated to match.
	ZC_SPRITE_CHANGE: 15,
}
