package packets

import "testing"

// TestOutgoingPacketsAreParseable: every packet id we send must be one the
// server actually parses at our PACKETVER.
//
// An id that is not in this table is not "probably fine" — it is a packet the
// server will not recognize, and because RO has no framing markers it does not
// fail cleanly: the server reads our bytes as whatever packet it thinks came
// next and the connection desynchronizes. #86 nearly shipped `CZ_CHOOSE_MENU`
// as 0x0BA8 for exactly this reason; the struct exists at our packetver but
// the packet table never registers it, which is the distinction this test
// encodes.
func TestOutgoingPacketsAreParseable(t *testing.T) {
	// Every id this package sends. Add to this list when adding an encoder.
	outgoing := []struct {
		name string
		id   uint16
	}{
		{"CZ_NOTIFY_ACTORINIT", CZ_NOTIFY_ACTORINIT},
		{"CZ_REQUEST_CHAT", CZ_REQUEST_CHAT},
		{"CZ_WHISPER", CZ_WHISPER},
		{"CZ_CONTACTNPC", CZ_CONTACTNPC},
		{"CZ_CHOOSE_MENU", CZ_CHOOSE_MENU},
		{"CZ_REQ_NEXT_SCRIPT", CZ_REQ_NEXT_SCRIPT},
		{"CZ_CLOSE_DIALOG", CZ_CLOSE_DIALOG},
		{"CZ_REQ_RESTART", CZ_REQ_RESTART},
		{"CZ_REQ_DISCONNECT", CZ_REQ_DISCONNECT},
	}

	for _, p := range outgoing {
		t.Run(p.name, func(t *testing.T) {
			if _, ok := ClientPacketLength(p.id); !ok {
				t.Errorf("we send %s (0x%04X) but the server does not parse it "+
					"at this PACKETVER — the connection will desynchronize",
					p.name, p.id)
			}
		})
	}
}

// TestEncodersMatchTheExpectedLength: a fixed-length packet we build must be
// exactly as long as the server expects, and a variable-length one must say so.
//
// Building one byte short or long is the same desynchronization as a wrong id,
// and it is not visible in the encoder itself — only against the server's own
// table.
func TestEncodersMatchTheExpectedLength(t *testing.T) {
	tests := []struct {
		name string
		id   uint16
		pkt  []byte
	}{
		{"chat", CZ_REQUEST_CHAT, EncodeChat("MidgardTest", "hello")},
		{"whisper", CZ_WHISPER, EncodeWhisper("Someone", "hello")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want, ok := ClientPacketLength(tt.id)
			if !ok {
				t.Fatalf("0x%04X is not in the client table", tt.id)
			}
			if tt.pkt == nil {
				t.Fatal("encoder returned nil")
			}

			if want == VariableLength {
				// The length travels at offset 2 and must match what we built.
				got := int(readU16(tt.pkt, 2))
				if got != len(tt.pkt) {
					t.Errorf("declared length %d, built %d bytes", got, len(tt.pkt))
				}

				return
			}

			if len(tt.pkt) != want {
				t.Errorf("built %d bytes, server expects %d", len(tt.pkt), want)
			}
		})
	}
}

// TestClientTableKnowsTheVersionedIDs: the ids this packetver actually changed
// are present with the sizes the server reads.
//
// These are the ones a wiki gets wrong, and they are the reason the table is
// generated rather than typed.
func TestClientTableKnowsTheVersionedIDs(t *testing.T) {
	tests := []struct {
		name string
		id   uint16
		want int
	}{
		// /mm, /b, /lb — what step 4 sends.
		{"CZ_MOVETO_MAP", 0x0140, 22},
		{"CZ_BROADCAST", 0x0099, VariableLength},
		{"CZ_LOCALBROADCAST", 0x019C, VariableLength},
		// /who, and the reply is already in the server table at 6 bytes.
		{"CZ_REQ_USER_COUNT", 0x00C1, 2},
		// /resetstate and /resetskill both ride CZ_RESET here: the dedicated
		// CZ_RESET_SKILL 0x0BB1 is guarded on MAIN/ZERO packetvers and we are RE.
		{"CZ_RESET", 0x0197, 4},
		// /item is the trap: 0x013F/26 in the base block, 0x09CE/102 from
		// PACKETVER 20131223. Both stay registered, so both must be right.
		{"CZ_ITEM_CREATE old", 0x013F, 26},
		{"CZ_ITEM_CREATE ours", 0x09CE, 102},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ClientPacketLength(tt.id)
			if !ok {
				t.Fatalf("0x%04X missing from the client table", tt.id)
			}
			if got != tt.want {
				t.Errorf("0x%04X = %d, want %d", tt.id, got, tt.want)
			}
		})
	}
}

// TestCZResetSkillIsNotRegistered: 0x0BB1 must be absent at our PACKETVER.
//
// The struct exists, which is what makes it tempting; the packet table does
// not register it unless PACKETVER_MAIN >= 20220216 or PACKETVER_ZERO >=
// 20220203, and we are PACKETVER_RE. A generator that ignored the guard would
// put it in and /resetskill would be sent into a void.
func TestCZResetSkillIsNotRegistered(t *testing.T) {
	if size, ok := ClientPacketLength(0x0BB1); ok {
		t.Errorf("0x0BB1 is in the table at %d bytes, but its guard is "+
			"MAIN>=20220216 || ZERO>=20220203 and we are RE 20211103", size)
	}
}
