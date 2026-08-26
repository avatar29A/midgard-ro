package packets

import (
	"bytes"
	"reflect"
	"testing"
)

func TestNPCOutgoingPackets(t *testing.T) {
	tests := []struct {
		name string
		got  []byte
		want []byte
	}{
		{
			name: "CZ_CONTACTNPC",
			got:  ContactNPC(0x00110022),
			want: []byte{0x90, 0x00, 0x22, 0x00, 0x11, 0x00, 0x00},
		},
		{
			name: "CZ_REQ_NEXT_SCRIPT",
			got:  RequestNextScript(0x00110022),
			want: []byte{0xB9, 0x00, 0x22, 0x00, 0x11, 0x00},
		},
		{
			name: "CZ_CLOSE_DIALOG",
			got:  CloseDialogPacket(0x00110022),
			want: []byte{0x46, 0x01, 0x22, 0x00, 0x11, 0x00},
		},
		{
			name: "canceling a menu sends 255",
			got:  CancelMenu(0x00110022),
			want: []byte{0xB8, 0x00, 0x22, 0x00, 0x11, 0x00, 0xFF},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !bytes.Equal(tt.got, tt.want) {
				t.Errorf("got % X, want % X", tt.got, tt.want)
			}

			if length, known := Length(readU16(tt.want, 0)); known && length != len(tt.want) {
				t.Errorf("built %d bytes, but the length table says %d", len(tt.want), length)
			}
		})
	}
}

func TestChooseMenu(t *testing.T) {
	tests := []struct {
		name              string
		choice, itemCount int
		want              []byte
		wantErr           bool
	}{
		{
			name:   "the first item is 1, not 0",
			choice: 1, itemCount: 3,
			want: []byte{0xB8, 0x00, 0x22, 0x00, 0x11, 0x00, 0x01},
		},
		{
			name:   "the last item",
			choice: 3, itemCount: 3,
			want: []byte{0xB8, 0x00, 0x22, 0x00, 0x11, 0x00, 0x03},
		},
		// The three below are disconnections if they reach the server:
		// clif_parse_NpcSelectMenu kicks on a zero or out-of-range selection.
		{"zero is refused", 0, 3, nil, true},
		{"past the end is refused", 4, 3, nil, true},
		{"negative is refused", -1, 3, nil, true},
		{"a menu with no items cannot be chosen from", 1, 0, nil, true},
		{"more items than a selection byte can carry", 1, 255, nil, true},
		{"exactly 254 items is allowed", 254, 254, []byte{0xB8, 0x00, 0x22, 0x00, 0x11, 0x00, 0xFE}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ChooseMenu(0x00110022, tt.choice, tt.itemCount)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("ChooseMenu(%d, %d) = % X, want an error", tt.choice, tt.itemCount, got)
				}
				return
			}

			if err != nil {
				t.Fatalf("ChooseMenu(%d, %d) failed: %v", tt.choice, tt.itemCount, err)
			}
			if !bytes.Equal(got, tt.want) {
				t.Errorf("got % X, want % X", got, tt.want)
			}
		})
	}
}

// sayPacket builds a ZC_SAY_DIALOG the way the server does: header, then the
// message with the NUL the length includes.
func sayPacket(id uint16, npcID uint32, message string) []byte {
	body := append([]byte(message), 0)
	buf := make([]byte, sayDialogHeader+len(body))
	buf[0], buf[1] = byte(id), byte(id>>8)
	buf[2], buf[3] = byte(len(buf)), byte(len(buf)>>8)
	writeU32(buf, 4, npcID)
	copy(buf[sayDialogHeader:], body)

	return buf
}

func TestDecodeSayDialog(t *testing.T) {
	t.Run("a greeting", func(t *testing.T) {
		got := DecodeSayDialog(sayPacket(ZC_SAY_DIALOG, 0x000004D2, "[Kafra Employee]\nWelcome."))
		if got == nil {
			t.Fatal("DecodeSayDialog returned nil")
		}
		if got.NPCID != 0x000004D2 {
			t.Errorf("NPCID = %d, want 1234", got.NPCID)
		}
		if got.Message != "[Kafra Employee]\nWelcome." {
			t.Errorf("Message = %q, want the greeting with its line break intact", got.Message)
		}
	})

	t.Run("an empty message", func(t *testing.T) {
		got := DecodeSayDialog(sayPacket(ZC_SAY_DIALOG, 1, ""))
		if got == nil || got.Message != "" {
			t.Errorf("got %+v, want an empty message", got)
		}
	})

	// The length field bounds the text, not the buffer: several packets
	// arrive in one read, and trusting the buffer would swallow the next one.
	t.Run("text stops at the packet length", func(t *testing.T) {
		data := append(sayPacket(ZC_SAY_DIALOG, 1, "first"), sayPacket(ZC_SAY_DIALOG, 2, "second")...)

		got := DecodeSayDialog(data)
		if got == nil || got.Message != "first" {
			t.Errorf("got %+v, want only the first message", got)
		}
	})

	t.Run("too short to decode", func(t *testing.T) {
		if got := DecodeSayDialog([]byte{0xB4, 0x00, 0x08}); got != nil {
			t.Errorf("got %+v, want nil", got)
		}
	})

	t.Run("a length shorter than the header", func(t *testing.T) {
		data := sayPacket(ZC_SAY_DIALOG, 1, "hello")
		data[2], data[3] = 4, 0

		if got := DecodeSayDialog(data); got != nil {
			t.Errorf("got %+v, want nil", got)
		}
	})
}

func TestDecodeDialogNPCID(t *testing.T) {
	npcID, ok := DecodeDialogNPCID([]byte{0xB5, 0x00, 0x22, 0x00, 0x11, 0x00})
	if !ok || npcID != 0x00110022 {
		t.Errorf("got (%d, %v), want (1114146, true)", npcID, ok)
	}

	if _, ok := DecodeDialogNPCID([]byte{0xB5, 0x00, 0x22}); ok {
		t.Error("a short packet decoded")
	}
}

func TestSplitMenu(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want []string
	}{
		{"empty", "", nil},
		{"one item", "Save", []string{"Save"}},
		{"several", "Save:Use Storage:Cancel", []string{"Save", "Use Storage", "Cancel"}},
		{"a trailing colon adds no item", "Save:Cancel:", []string{"Save", "Cancel"}},
		{"a leading colon adds no item", ":Save", []string{"Save"}},
		// menu_countoptions counts only non-empty options into npc_menu, so
		// `b` here is choice 2. Keeping the empty would make it 3 and get us
		// kicked.
		{"an empty entry between two items", "a::b", []string{"a", "b"}},
		{"newlines belong to the item", "Yes.\nGo on:No", []string{"Yes.\nGo on", "No"}},
		{"nothing but colons", ":::", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SplitMenu(tt.raw)

			if len(got) == 0 && len(tt.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SplitMenu(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestDecodeMenuList(t *testing.T) {
	got := DecodeMenuList(sayPacket(ZC_MENU_LIST, 0x000004D2, "Save:Use Storage:Cancel"))
	if got == nil {
		t.Fatal("DecodeMenuList returned nil")
	}
	if got.NPCID != 0x000004D2 {
		t.Errorf("NPCID = %d, want 1234", got.NPCID)
	}

	want := []string{"Save", "Use Storage", "Cancel"}
	if !reflect.DeepEqual(got.Items, want) {
		t.Errorf("Items = %q, want %q", got.Items, want)
	}

	// The index the player picks is its position here, 1-based.
	if _, err := ChooseMenu(got.NPCID, len(got.Items), len(got.Items)); err != nil {
		t.Errorf("choosing the last item of a decoded menu failed: %v", err)
	}
}
