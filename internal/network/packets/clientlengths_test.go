package packets

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"strings"
	"testing"
)

// discoverOutgoingIDs finds every CZ_* packet id this package declares by
// reading the package's own source.
//
// This list used to be written out by hand, under a note asking whoever added
// an encoder to extend it. Three encoders were added without it being
// extended, and two of them carried ids that mean something else entirely at
// this packetver — CZ_USE_ITEM was clif_parse_WalkToXY and CZ_ITEM_THROW was
// clif_parse_SolveCharName. The check existed; it just did not cover them.
// Reading the constants means a new encoder is covered the moment it is
// declared, which is the only version of this test that stays true.
func discoverOutgoingIDs(t *testing.T) map[string]uint16 {
	t.Helper()

	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", nil, 0)
	if err != nil {
		t.Fatalf("parsing this package: %v", err)
	}

	ids := make(map[string]uint16)
	for _, pkg := range pkgs {
		for _, file := range pkg.Files {
			for _, decl := range file.Decls {
				gen, ok := decl.(*ast.GenDecl)
				if !ok || gen.Tok != token.CONST {
					continue
				}

				for _, spec := range gen.Specs {
					value, ok := spec.(*ast.ValueSpec)
					if !ok || len(value.Names) != 1 || len(value.Values) != 1 {
						continue
					}

					name := value.Names[0].Name
					if !strings.HasPrefix(name, "CZ_") {
						continue
					}

					lit, ok := value.Values[0].(*ast.BasicLit)
					if !ok || lit.Kind != token.INT {
						continue
					}

					id, err := strconv.ParseUint(lit.Value, 0, 16)
					if err != nil {
						continue
					}

					ids[name] = uint16(id)
				}
			}
		}
	}

	if len(ids) == 0 {
		t.Fatal("no CZ_ constants found — the discovery is broken, not the packets")
	}

	return ids
}

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
	for name, id := range discoverOutgoingIDs(t) {
		t.Run(name, func(t *testing.T) {
			if _, ok := ClientPacketLength(id); !ok {
				t.Errorf("we send %s (0x%04X) but the server does not parse it "+
					"at this PACKETVER — the connection will desynchronize",
					name, id)
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
		{"use item", CZ_USE_ITEM, EncodeUseItem(5)},
		{"equip item", CZ_REQ_WEAR_EQUIP, EncodeEquipItem(5, 0x0100)},
		{"drop item", CZ_ITEM_THROW, EncodeDropItem(5, 1)},
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

// TestEncodersReachTheRightHandler pins each encoder to the server function
// that will parse it.
//
// Length is not identity. Six bytes covers picking an item up, dropping one,
// asking a character's name and half a dozen other things, so an encoder can
// carry an id that means something else entirely and still pass every check
// that only measures. Dropping was briefly sent on 0x0362, which is six bytes
// and is clif_parse_TakeItem — the right size, the wrong verb.
//
// Handlers come from clif_shuffle.hpp where it overrides the main table, which
// is the last word at this packetver and disagrees with it on 27 ids.
func TestEncodersReachTheRightHandler(t *testing.T) {
	tests := []struct {
		name    string
		id      uint16
		handler string
	}{
		{"CZ_USE_ITEM", CZ_USE_ITEM, "clif_parse_UseItem"},
		{"CZ_REQ_WEAR_EQUIP", CZ_REQ_WEAR_EQUIP, "clif_parse_EquipItem"},
		{"CZ_ITEM_THROW", CZ_ITEM_THROW, "clif_parse_DropItem"},
		{"CZ_ITEM_PICKUP", CZ_ITEM_PICKUP, "clif_parse_TakeItem"},
		{"CZ_REQUEST_CHAT", CZ_REQUEST_CHAT, "clif_parse_GlobalMessage"},
		{"CZ_WHISPER", CZ_WHISPER, "clif_parse_WisMessage"},
		{"CZ_CONTACTNPC", CZ_CONTACTNPC, "clif_parse_NpcClicked"},
		{"CZ_CHOOSE_MENU", CZ_CHOOSE_MENU, "clif_parse_NpcSelectMenu"},
		{"CZ_REQ_DISCONNECT", CZ_REQ_DISCONNECT, "clif_parse_QuitGame"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ClientPacketHandler(tt.id)
			if !ok {
				t.Fatalf("%s (0x%04X) is not parsed at this PACKETVER", tt.name, tt.id)
			}
			if got != tt.handler {
				t.Errorf("%s (0x%04X) reaches %s, want %s — the server will do "+
					"the wrong thing with it", tt.name, tt.id, got, tt.handler)
			}
		})
	}
}

// TestShuffledIDsFollowTheShuffleTable guards the file the generator used to
// ignore entirely.
//
// clif.cpp includes clif_shuffle.hpp after clif_packetdb.hpp and
// packetdb_addpacket overwrites, so these are the ids the server ends up with.
// Reading only the main table put TakeItem on 0x07E4, which this packetver
// hands to a variable-length packet: the server read the payload as a length
// and closed the connection.
func TestShuffledIDsFollowTheShuffleTable(t *testing.T) {
	tests := []struct {
		id      uint16
		length  int
		handler string
	}{
		{0x0362, 6, "clif_parse_TakeItem"},
		{0x0363, 6, "clif_parse_DropItem"},
		{0x07E4, VariableLength, "clif_parse_ItemListWindowSelected"},
		// The map-server login packet, which the main table calls
		// FriendsListAdd at 26 bytes.
		{0x0436, 23, "clif_parse_WantToConnection"},
	}

	for _, tt := range tests {
		t.Run(tt.handler, func(t *testing.T) {
			length, ok := ClientPacketLength(tt.id)
			if !ok {
				t.Fatalf("0x%04X missing from the client table", tt.id)
			}
			if length != tt.length {
				t.Errorf("0x%04X length = %d, want %d", tt.id, length, tt.length)
			}
			if handler, _ := ClientPacketHandler(tt.id); handler != tt.handler {
				t.Errorf("0x%04X reaches %s, want %s", tt.id, handler, tt.handler)
			}
		})
	}
}
