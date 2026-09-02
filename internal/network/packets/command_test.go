package packets

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"
)

// TestEncodeUserCount: /who carries nothing but its id, and the server reads
// exactly two bytes for it.
func TestEncodeUserCount(t *testing.T) {
	got := EncodeUserCount()

	want := []byte{0xC1, 0x00}
	if !bytes.Equal(got, want) {
		t.Errorf("EncodeUserCount() = % X, want % X", got, want)
	}

	size, ok := ClientPacketLength(CZ_REQ_USER_COUNT)
	if !ok {
		t.Fatal("0x00C1 is not in the client table")
	}
	if len(got) != size {
		t.Errorf("built %d bytes, server expects %d", len(got), size)
	}
}

func TestDecodeUserCount(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want int
		ok   bool
	}{
		{"one player", []byte{0xC2, 0x00, 0x01, 0x00, 0x00, 0x00}, 1, true},
		{"none", []byte{0xC2, 0x00, 0x00, 0x00, 0x00, 0x00}, 0, true},
		{"many", []byte{0xC2, 0x00, 0xE8, 0x03, 0x00, 0x00}, 1000, true},
		{"short", []byte{0xC2, 0x00, 0x01}, 0, false},
		{"empty", nil, 0, false},
		// map_getusers() returns a signed int. A negative count is not
		// something to show a player, so it reads as unreadable instead.
		{"negative", []byte{0xC2, 0x00, 0xFF, 0xFF, 0xFF, 0xFF}, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := DecodeUserCount(tt.data)
			if ok != tt.ok {
				t.Fatalf("ok = %v, want %v", ok, tt.ok)
			}
			if got != tt.want {
				t.Errorf("count = %d, want %d", got, tt.want)
			}
		})
	}
}

// TestUserCountReplyIsFramed: the reply's length has to be in the server table
// or the connection desynchronizes the moment /who is answered.
func TestUserCountReplyIsFramed(t *testing.T) {
	size, ok := Length(ZC_USER_COUNT)
	if !ok {
		t.Fatal("0x00C2 has no length: the framing layer will lose the stream")
	}
	if size != 6 {
		t.Errorf("0x00C2 length = %d, want 6 (type 2 + count 4)", size)
	}
}

// TestEncodeMapMoveLayout: 22 bytes, and the map name has to be terminated
// inside its 16-byte field or the server warps somewhere else.
func TestEncodeMapMoveLayout(t *testing.T) {
	pkt := EncodeMapMove("prontera", 150, 160)

	if pkt == nil {
		t.Fatal("EncodeMapMove returned nil for a valid request")
	}

	size, ok := ClientPacketLength(CZ_MOVETO_MAP)
	if !ok {
		t.Fatal("0x0140 is not in the client table")
	}
	if len(pkt) != size {
		t.Fatalf("built %d bytes, server expects %d", len(pkt), size)
	}

	if got := binary.LittleEndian.Uint16(pkt[0:2]); got != CZ_MOVETO_MAP {
		t.Errorf("id = 0x%04X, want 0x%04X", got, CZ_MOVETO_MAP)
	}

	name := pkt[2:18]
	if string(name[:len("prontera")]) != "prontera" {
		t.Errorf("map = %q, want it to start with prontera", name)
	}
	if name[len("prontera")] != 0 {
		t.Error("map name is not terminated; the server reads it with safestrncpy")
	}

	if got := binary.LittleEndian.Uint16(pkt[18:20]); got != 150 {
		t.Errorf("x = %d, want 150", got)
	}
	if got := binary.LittleEndian.Uint16(pkt[20:22]); got != 160 {
		t.Errorf("y = %d, want 160", got)
	}
}

// TestEncodeMapMoveRefusesUnterminatedNames: the field holds 16 bytes and one
// of them must be the terminator. A name that fills it would be truncated by
// the server and warp the player to a different map, which is worse than
// sending nothing.
func TestEncodeMapMoveRefusesUnterminatedNames(t *testing.T) {
	if EncodeMapMove("", 0, 0) != nil {
		t.Error("encoded a move with no map name")
	}
	if EncodeMapMove("0123456789abcdef", 0, 0) != nil {
		t.Error("encoded a 16-byte name, leaving no room for the terminator")
	}
}

// TestEncodeBroadcastLayout: header plus the message and no terminator, which
// is what the server's own length arithmetic expects — it copies
// packetSize - 4 + 1 bytes with safestrncpy, which writes the NUL itself.
func TestEncodeBroadcastLayout(t *testing.T) {
	tests := []struct {
		name string
		got  []byte
		id   uint16
	}{
		{"broadcast", EncodeBroadcast("hello"), CZ_BROADCAST},
		{"local broadcast", EncodeLocalBroadcast("hello"), CZ_LOCALBROADCAST},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got == nil {
				t.Fatal("returned nil for a valid message")
			}

			want := 4 + len("hello")
			if len(tt.got) != want {
				t.Fatalf("len = %d, want %d (header + message, no terminator)", len(tt.got), want)
			}
			if id := binary.LittleEndian.Uint16(tt.got[0:2]); id != tt.id {
				t.Errorf("id = 0x%04X, want 0x%04X", id, tt.id)
			}
			if declared := int(binary.LittleEndian.Uint16(tt.got[2:4])); declared != want {
				t.Errorf("declared length = %d, want %d", declared, want)
			}
			if string(tt.got[4:]) != "hello" {
				t.Errorf("message = %q, want hello", tt.got[4:])
			}

			// Variable-length: the table says -1, and the framing layer
			// trusts the declared size instead.
			if size, ok := ClientPacketLength(tt.id); !ok || size != -1 {
				t.Errorf("client table says %d/%v, want -1 (variable)", size, ok)
			}
		})
	}
}

// TestEncodeBroadcastRefusesEmpty: the server would accept it and announce
// nothing.
func TestEncodeBroadcastRefusesEmpty(t *testing.T) {
	if EncodeBroadcast("") != nil {
		t.Error("encoded an empty broadcast")
	}
	if EncodeLocalBroadcast("") != nil {
		t.Error("encoded an empty local broadcast")
	}
}

// TestEncodeMakeCharLayout: 36 bytes, and every field where the server reads
// it. At our packet version the request carries a job and a sex and no stats —
// the server writes 1 to each of those itself.
func TestEncodeMakeCharLayout(t *testing.T) {
	pkt := EncodeMakeChar(MakeCharRequest{
		Name:      "QaFreshName",
		Slot:      3,
		HairColor: 5,
		HairStyle: 12,
		Job:       4218,
		Sex:       1,
	})

	if pkt == nil {
		t.Fatal("EncodeMakeChar returned nil for a valid request")
	}

	// Not cross-checked against ClientPacketLength: that table is generated
	// from the map server's clif_packetdb.hpp and holds no char-server
	// packets at all — CH_ENTER and PING are absent from it too. The size
	// here is the struct's own, from common/packets.hpp:122-132.
	if len(pkt) != 36 {
		t.Fatalf("len = %d, want 36 (type 2 + name 24 + slot 1 + hair 4 + job 4 + sex 1)",
			len(pkt))
	}

	if got := binary.LittleEndian.Uint16(pkt[0:2]); got != CH_MAKE_CHAR {
		t.Errorf("id = 0x%04X, want 0x%04X", got, CH_MAKE_CHAR)
	}

	name := pkt[2:26]
	if string(name[:len("QaFreshName")]) != "QaFreshName" {
		t.Errorf("name = %q", name)
	}
	if name[len("QaFreshName")] != 0 {
		t.Error("name is not terminated inside its field")
	}

	if pkt[26] != 3 {
		t.Errorf("slot = %d, want 3", pkt[26])
	}
	if got := binary.LittleEndian.Uint16(pkt[27:29]); got != 5 {
		t.Errorf("hair color = %d, want 5", got)
	}
	if got := binary.LittleEndian.Uint16(pkt[29:31]); got != 12 {
		t.Errorf("hair style = %d, want 12", got)
	}
	if got := binary.LittleEndian.Uint32(pkt[31:35]); got != 4218 {
		t.Errorf("job = %d, want 4218 (Summoner)", got)
	}
	if pkt[35] != 1 {
		t.Errorf("sex = %d, want 1 (male)", pkt[35])
	}
}

// TestEncodeMakeCharRefusesUnsendableNames: the field is 24 bytes and must
// hold a terminator. Everything else about a name is the server's to judge.
func TestEncodeMakeCharRefusesUnsendableNames(t *testing.T) {
	if EncodeMakeChar(MakeCharRequest{Name: ""}) != nil {
		t.Error("encoded an empty name")
	}
	if EncodeMakeChar(MakeCharRequest{Name: strings.Repeat("a", 24)}) != nil {
		t.Error("encoded a 24-byte name, leaving no room for the terminator")
	}
	if EncodeMakeChar(MakeCharRequest{Name: strings.Repeat("a", 23)}) == nil {
		t.Error("refused a 23-byte name, which fits")
	}
}

// TestMakeCharRefusal: a taken name is the only answer to "is this name free",
// because nothing asks that question — so its code must be read exactly.
func TestMakeCharRefusal(t *testing.T) {
	code, ok := DecodeMakeCharRefuse([]byte{0x6E, 0x00, MakeCharNameTaken})
	if !ok {
		t.Fatal("could not read a refusal")
	}
	if code != MakeCharNameTaken {
		t.Errorf("code = 0x%02X, want 0x00", code)
	}
	if msg := MakeCharFailure(code); !strings.Contains(msg, "already taken") {
		t.Errorf("message = %q, want it to name the cause", msg)
	}

	if _, ok := DecodeMakeCharRefuse([]byte{0x6E, 0x00}); ok {
		t.Error("read a refusal from a packet too short to hold one")
	}

	// Everything the server refuses for another reason still has to say
	// something rather than nothing.
	for _, code := range []uint8{MakeCharDenied, MakeCharUnderaged, MakeCharSlotNotAllowed} {
		if MakeCharFailure(code) == "" {
			t.Errorf("code 0x%02X has no message", code)
		}
	}
}

// TestMakeCharAcceptIsFramed: 0x0B6F is the id at our packet version, and the
// framing layer has to know its length or the stream desynchronizes the first
// time a character is made.
func TestMakeCharAcceptIsFramed(t *testing.T) {
	if _, ok := Length(HC_ACCEPT_MAKECHAR); !ok {
		t.Errorf("0x%04X has no length: the framing layer will lose the stream",
			HC_ACCEPT_MAKECHAR)
	}
	if _, ok := Length(HC_REFUSE_MAKECHAR); !ok {
		t.Error("0x006E has no length")
	}
}
