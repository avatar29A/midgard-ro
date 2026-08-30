package packets

import (
	"bytes"
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
