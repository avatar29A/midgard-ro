package packets

import "testing"

// TestKnownLengths pins the packets the login → char → map → walk flow depends
// on, with the values taken from the rAthena tree the server is built from.
//
// The two marked entries are the ones that were wrong in the old hand-written
// table and desynchronized the connection: every packet after them, walk
// acknowledgements included, was parsed from the wrong offset.
func TestKnownLengths(t *testing.T) {
	tests := []struct {
		name string
		id   uint16
		want int
	}{
		// Login server.
		{"AC_ACCEPT_LOGIN2", 0x0AC4, VariableLength},
		{"AC_REFUSE_LOGIN", 0x006A, 23},
		{"AC_REFUSE_LOGIN2", 0x083E, 26},

		// Char server.
		{"HC_ACCEPT_ENTER", 0x006B, VariableLength},
		{"HC_REFUSE_ENTER", 0x006C, 3},
		// struct PACKET_HC_NOTIFY_ZONESVR at PACKETVER >= 20170315 is
		// 2 + 4 + 16 + 4 + 2 + 128. The old table said 28 — the pre-20170315
		// layout, missing the 128-byte domain field.
		{"HC_NOTIFY_ZONESVR (0x0AC5)", 0x0AC5, 156},
		{"HC_NOTIFY_ZONESVR (0x0071, legacy)", 0x0071, 28},

		// Map server.
		{"ZC_ACCEPT_ENTER2", 0x02EB, 13},
		{"ZC_NOTIFY_PLAYERMOVE", 0x0087, 12},
		{"ZC_NOTIFY_MOVEENTRY", 0x007B, 60},
		{"ZC_NPCACK_MAPMOVE", 0x0091, 22},
		{"ZC_NOTIFY_TIME", 0x007F, 6},
		// The old table had no entry, so it read bytes 2-4 as a length and
		// consumed 70 bytes for a 6-byte packet.
		{"ZC_OVERWEIGHT_PERCENT (0x0ADE)", 0x0ADE, 6},

		// Both arrive during login and are declared only in
		// packets_struct.hpp, which the generator missed on its first pass —
		// so the connection still desynchronized once per login until that
		// header was added as a source.
		{"ZC_PAR_CHANGE (0x00B0)", 0x00B0, 8},
		{"ZC_EXTEND_BODYITEM_SIZE (0x0B18)", 0x0B18, 4},

		// Declared with DEFINE_PACKET_ID rather than DEFINE_PACKET_HEADER —
		// an alias the generator did not match at first.
		{"ZC_COUPLESTATUS (0x0141)", 0x0141, 14},
		// Sized by a #define in the same header, behind a build feature flag,
		// and embeds a helper struct: 2 + 1 + 2 + 38*7.
		{"ZC_SHORTCUT_KEY_LIST (0x0B20)", 0x0B20, 271},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, known := Length(tt.id)
			if !known {
				t.Fatalf("packet 0x%04X is not in the generated table", tt.id)
			}
			if got != tt.want {
				t.Errorf("Length(0x%04X) = %d, want %d", tt.id, got, tt.want)
			}
		})
	}
}

func TestUnknownPacketReportsUnknown(t *testing.T) {
	// 0xFFFF is not a real packet; it's the sort of value that turns up when
	// reading payload bytes as an id.
	if _, known := Length(0xFFFF); known {
		t.Error("0xFFFF should not be a known packet id")
	}
	if IsKnown(0xFFFF) {
		t.Error("IsKnown(0xFFFF) should be false")
	}
}

// TestTableIsPopulated guards against the generator silently producing an
// empty or truncated table — which would make every packet "unknown" and stall
// the client rather than fail loudly.
func TestTableIsPopulated(t *testing.T) {
	if len(mapPacketLengths) < 950 {
		t.Errorf("packet table has %d entries, expected the generated table to be much larger",
			len(mapPacketLengths))
	}
}

// TestLengthsAreSane checks that no fixed length is smaller than the two-byte
// id itself, which would make the read loop spin without consuming input.
func TestLengthsAreSane(t *testing.T) {
	for id, length := range mapPacketLengths {
		if length == VariableLength {
			continue
		}
		if length < 2 {
			t.Errorf("packet 0x%04X has length %d, which cannot even hold its id", id, length)
		}
	}
}

// TestInventoryListsAreFramed is the guard for a gap that cost the inventory
// entirely: the generator reads packet_db and DEFINE_PACKET_HEADER structs,
// and these two ids are declared as enum constants that it sees neither way.
// Without a length the reader treats them as corruption and resynchronises
// past them, so the handler never runs.
func TestInventoryListsAreFramed(t *testing.T) {
	for _, id := range []uint16{ZC_INVENTORY_ITEMLIST_NORMAL, ZC_INVENTORY_ITEMLIST_EQUIP} {
		length, known := Length(id)
		if !known {
			t.Errorf("0x%04X is not framed — the reader will resynchronise past it", id)

			continue
		}
		if length != VariableLength {
			t.Errorf("0x%04X length = %d, want VariableLength", id, length)
		}
		if !IsKnown(id) {
			t.Errorf("0x%04X is not IsKnown, so resync will not stop on it", id)
		}
	}
}

// TestGroundItemsAreFramed guards the four packets a ground item needs.
//
// They were missing for a subtler reason than the inventory lists: the
// generator paired an id to its struct one header at a time, and
// ZC_ITEM_ENTRY is a struct in packets_struct.hpp whose DEFINE_PACKET_HEADER
// is in packets.hpp, under a comment reading "Other packets without struct
// defined in this file". ZC_ITEM_PICKUP_ACK resolved in neither, because its
// option array is bounded by MAX_ITEM_OPTIONS, which is defined in mmo.hpp.
//
// The sizes are pinned because both are packetver-dependent and would be
// wrong if the generator silently fell back to an older branch: ZC_ITEM_ENTRY
// is 19 rather than 17 because its item id widened to uint32 at RE 20180704,
// and ZC_ITEM_PICKUP_ACK carries five item options at this packetver.
func TestGroundItemsAreFramed(t *testing.T) {
	tests := []struct {
		name string
		id   uint16
		want int
	}{
		{"ZC_ITEM_ENTRY", 0x009D, 19},
		{"ZC_ITEM_FALL_ENTRY", 0x009E, 17},
		{"ZC_ITEM_DISAPPEAR", 0x00A1, 6},
		{"ZC_ITEM_PICKUP_ACK", 0x0B41, 70},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			length, known := Length(tt.id)
			if !known {
				t.Fatalf("0x%04X is not framed — the reader will resynchronise past it", tt.id)
			}
			if length != tt.want {
				t.Errorf("0x%04X length = %d, want %d", tt.id, length, tt.want)
			}
			if !IsKnown(tt.id) {
				t.Errorf("0x%04X is not IsKnown, so resync will not stop on it", tt.id)
			}
		})
	}
}
