package packets

import "testing"

// TestKnownLengths pins the packets the login → char → map → walk flow depends
// on, with the values taken from the rAthena tree the server is built from.
//
// The two marked entries are the ones that were wrong in the old hand-written
// table and desynchronised the connection: every packet after them, walk
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
		// so the connection still desynchronised once per login until that
		// header was added as a source.
		{"ZC_PAR_CHANGE (0x00B0)", 0x00B0, 8},
		{"ZC_EXTEND_BODYITEM_SIZE (0x0B18)", 0x0B18, 4},
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
	if len(mapPacketLengths) < 900 {
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
