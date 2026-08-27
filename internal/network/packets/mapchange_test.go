package packets

import "testing"

// mapChangeBytes is ZC_NPCACK_MAPMOVE as rAthena sends it: 0x0091, a 16-byte
// NUL-padded name with the ".gat" extension, then x and y as little-endian
// shorts.
func mapChangeBytes(name string, x, y uint16) []byte {
	b := make([]byte, mapChangeSize)
	b[0], b[1] = 0x91, 0x00
	copy(b[2:18], name)
	b[18], b[19] = byte(x), byte(x>>8)
	b[20], b[21] = byte(y), byte(y>>8)
	return b
}

func TestDecodeMapChange(t *testing.T) {
	tests := []struct {
		name     string
		wire     string
		x, y     uint16
		wantMap  string
		wantBase string
	}{
		{"town", "prontera.gat", 156, 191, "prontera.gat", "prontera"},
		{"field", "prt_fild08.gat", 170, 375, "prt_fild08.gat", "prt_fild08"},
		{"full width, no NUL", "abcdefghijk.gat!", 1, 2, "abcdefghijk.gat!", "abcdefghijk.gat!"},
		{"case", "PRT_IN.GAT", 168, 128, "PRT_IN.GAT", "prt_in"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mc := DecodeMapChange(mapChangeBytes(tt.wire, tt.x, tt.y))
			if mc == nil {
				t.Fatal("decoded nil")
			}
			if mc.MapName != tt.wantMap || mc.X != int(tt.x) || mc.Y != int(tt.y) {
				t.Fatalf("got %+v, want map %q at %d,%d", mc, tt.wantMap, tt.x, tt.y)
			}
			if got := mc.BaseName(); got != tt.wantBase {
				t.Fatalf("BaseName = %q, want %q", got, tt.wantBase)
			}
		})
	}
}

func TestDecodeMapChangeRejectsShortData(t *testing.T) {
	if DecodeMapChange(mapChangeBytes("prontera.gat", 1, 1)[:21]) != nil {
		t.Fatal("21 bytes decoded; the packet is 22")
	}
}

func TestMapBaseNameComparesTheServersSpellingWithOurs(t *testing.T) {
	if MapBaseName("prontera.gat") != MapBaseName("prontera") {
		t.Fatal("the two spellings of a map name must compare equal")
	}
}

func TestDecodeServerMove(t *testing.T) {
	b := make([]byte, serverMoveSize)
	b[0], b[1] = 0xC7, 0x0A
	copy(b[2:18], "alberta.gat")
	b[18], b[19] = 192, 0 // x
	b[20], b[21] = 147, 0 // y
	copy(b[22:26], []byte{127, 0, 0, 1})
	b[26], b[27] = 0x01, 0x14 // 5121 little-endian
	copy(b[28:], "map2.example.org")

	sm := DecodeServerMove(b)
	if sm == nil {
		t.Fatal("decoded nil")
	}
	if sm.MapName != "alberta.gat" || sm.X != 192 || sm.Y != 147 {
		t.Fatalf("got %+v", sm)
	}
	if got := sm.Address(); got != "127.0.0.1:5121" {
		t.Fatalf("Address = %q", got)
	}
	if sm.Domain != "map2.example.org" {
		t.Fatalf("Domain = %q", sm.Domain)
	}

	if DecodeServerMove(b[:serverMoveSize-1]) != nil {
		t.Fatal("155 bytes decoded; the packet is 156")
	}
}
