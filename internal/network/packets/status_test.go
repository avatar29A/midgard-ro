package packets

import (
	"encoding/binary"
	"testing"
)

func TestDecodeStatusChange(t *testing.T) {
	tests := []struct {
		name      string
		data      []byte
		wantNil   bool
		wantVarID uint16
		wantValue int64
	}{
		{
			name:      "ZC_PAR_CHANGE carries HP",
			data:      []byte{0xB0, 0x00, 0x05, 0x00, 0x28, 0x00, 0x00, 0x00},
			wantVarID: SP_HP,
			wantValue: 40,
		},
		{
			name:      "ZC_PAR_CHANGE carries a max weight that needs more than a short",
			data:      []byte{0xB0, 0x00, 0x19, 0x00, 0x60, 0xEA, 0x00, 0x00},
			wantVarID: SP_MAXWEIGHT,
			wantValue: 60000,
		},
		{
			name:      "ZC_LONGPAR_CHANGE carries Zeny",
			data:      []byte{0xB1, 0x00, 0x14, 0x00, 0xA0, 0x86, 0x01, 0x00},
			wantVarID: SP_ZENY,
			wantValue: 100000,
		},
		{
			name:      "the value is signed",
			data:      []byte{0xB0, 0x00, 0x05, 0x00, 0xFF, 0xFF, 0xFF, 0xFF},
			wantVarID: SP_HP,
			wantValue: -1,
		},
		{
			name:      "ZC_LONGLONGPAR_CHANGE carries experience past 32 bits",
			data:      []byte{0xCB, 0x0A, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00},
			wantVarID: SP_BASEEXP,
			wantValue: 1 << 32,
		},
		{
			name:      "an id we do not map still decodes — the caller decides",
			data:      []byte{0xB0, 0x00, 0x29, 0x00, 0x0B, 0x00, 0x00, 0x00},
			wantVarID: 41, // SP_ATK1
			wantValue: 11,
		},
		{
			name:    "not a status packet",
			data:    []byte{0x7F, 0x00, 0x05, 0x00, 0x28, 0x00, 0x00, 0x00},
			wantNil: true,
		},
		{
			name:    "truncated 8-byte packet",
			data:    []byte{0xB0, 0x00, 0x05, 0x00, 0x28},
			wantNil: true,
		},
		{
			name:    "truncated 12-byte packet",
			data:    []byte{0xCB, 0x0A, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00},
			wantNil: true,
		},
		{
			name:    "too short to hold an id",
			data:    []byte{0xB0},
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DecodeStatusChange(tt.data)

			if tt.wantNil {
				if got != nil {
					t.Fatalf("DecodeStatusChange() = %+v, want nil", got)
				}
				return
			}

			if got == nil {
				t.Fatal("DecodeStatusChange() = nil, want a change")
			}
			if got.VarID != tt.wantVarID {
				t.Errorf("VarID = %d, want %d", got.VarID, tt.wantVarID)
			}
			if got.Value != tt.wantValue {
				t.Errorf("Value = %d, want %d", got.Value, tt.wantValue)
			}
		})
	}
}

// TestStatusPacketLengths ties the decoder to the generated length table: a
// packet whose framing length disagrees with what the decoder reads would
// desync the connection rather than fail here.
func TestStatusPacketLengths(t *testing.T) {
	for _, tt := range []struct {
		id   uint16
		want int
	}{
		{ZC_PAR_CHANGE, 8},
		{ZC_LONGPAR_CHANGE, 8},
		{ZC_LONGLONGPAR_CHANGE, 12},
	} {
		got, known := Length(tt.id)
		if !known {
			t.Errorf("0x%04X is not in the length table", tt.id)
			continue
		}
		if got != tt.want {
			t.Errorf("length of 0x%04X = %d, want %d", tt.id, got, tt.want)
		}
	}
}

// TestDecodeStatusLayout walks the status window packet against the layout
// rAthena declares: a point total, then six value/cost pairs.
func TestDecodeStatusLayout(t *testing.T) {
	if n, ok := Length(ZC_STATUS); !ok || n != 44 {
		t.Fatalf("ZC_STATUS length = %d, %v; want 44, true", n, ok)
	}

	pkt := make([]byte, 44)
	binary.LittleEndian.PutUint16(pkt[0:], ZC_STATUS)
	binary.LittleEndian.PutUint16(pkt[2:], 7) // status points

	// value, cost for each of STR, AGI, VIT, INT, DEX, LUK.
	pairs := []byte{9, 2, 8, 3, 7, 4, 6, 5, 5, 6, 4, 7}
	copy(pkt[4:], pairs)

	got := DecodeStatus(pkt)
	if got == nil {
		t.Fatal("a full packet must decode")
	}
	if got.StatusPoints != 7 {
		t.Errorf("StatusPoints = %d, want 7", got.StatusPoints)
	}

	wantValues := [PrimaryStatCount]int{9, 8, 7, 6, 5, 4}
	wantCosts := [PrimaryStatCount]int{2, 3, 4, 5, 6, 7}
	if got.Values != wantValues {
		t.Errorf("Values = %v, want %v", got.Values, wantValues)
	}
	if got.Costs != wantCosts {
		t.Errorf("Costs = %v, want %v", got.Costs, wantCosts)
	}
}

func TestDecodeStatusShort(t *testing.T) {
	if DecodeStatus(make([]byte, 10)) != nil {
		t.Error("a packet too short for the six pairs must not decode")
	}
}

// TestDecodeCoupleStatus covers the packet that carries the bonus — the only
// one that does. Its status id is four bytes wide, unlike ZC_PAR_CHANGE's two.
func TestDecodeCoupleStatus(t *testing.T) {
	if n, ok := Length(ZC_COUPLESTATUS); !ok || n != 14 {
		t.Fatalf("ZC_COUPLESTATUS length = %d, %v; want 14, true", n, ok)
	}

	pkt := make([]byte, 14)
	binary.LittleEndian.PutUint16(pkt[0:], ZC_COUPLESTATUS)
	binary.LittleEndian.PutUint32(pkt[2:], uint32(SP_DEX))
	binary.LittleEndian.PutUint32(pkt[6:], 12)
	// A debuff subtracts from the stat, so the bonus goes out as a negative
	// int32 in an unsigned field.
	bonus := int32(-3)
	binary.LittleEndian.PutUint32(pkt[10:], uint32(bonus))

	got := DecodeCoupleStatus(pkt)
	if got == nil {
		t.Fatal("a full packet must decode")
	}
	if got.VarID != SP_DEX {
		t.Errorf("VarID = %d, want %d", got.VarID, SP_DEX)
	}
	if got.Base != 12 {
		t.Errorf("Base = %d, want 12", got.Base)
	}
	// Bonuses go negative: a debuff subtracts from the stat.
	if got.Bonus != -3 {
		t.Errorf("Bonus = %d, want -3", got.Bonus)
	}
}

// TestPrimaryStatIndex pins the six to their ids. STR is 13 and LUK 18 in
// rAthena's _sp enum, and anything outside that is not a primary stat — the
// server sends couple-status packets for derived numbers on the same packet.
func TestPrimaryStatIndex(t *testing.T) {
	for want, id := range []uint16{SP_STR, SP_AGI, SP_VIT, SP_INT, SP_DEX, SP_LUK} {
		got, ok := PrimaryStatIndex(id)
		if !ok || got != want {
			t.Errorf("PrimaryStatIndex(%d) = %d, %v; want %d, true", id, got, ok, want)
		}
	}

	for _, id := range []uint16{SP_STATUSPOINT, SP_CLASS, SP_ZENY} {
		if _, ok := PrimaryStatIndex(id); ok {
			t.Errorf("PrimaryStatIndex(%d) reported a primary stat", id)
		}
	}
}

// TestDecodeStatusDerived walks a real ZC_STATUS off the wire — a level-1
// Novice on a renewal server — so the derived offsets are pinned to bytes the
// server actually sent rather than to a reading of the struct.
//
// The two that look wrong are not: renewal HIT is 175 + level + DEX and FLEE
// is 100 + level + AGI, which is why a brand new character has 177 and 102.
func TestDecodeStatusDerived(t *testing.T) {
	pkt := []byte{
		0xbd, 0x00, 0x00, 0x00,
		1, 2, 1, 2, 1, 2, 1, 2, 1, 2, 1, 2,
		0x01, 0x00, 0x00, 0x00, // atk 1, refine 0
		0x00, 0x00, 0x01, 0x00, // matk max 0, min 1
		0x01, 0x00, 0x00, 0x00, // def 1, +0
		0x01, 0x00, 0x00, 0x00, // mdef 1, +0
		0xb1, 0x00, // hit 177
		0x66, 0x00, 0x01, 0x00, // flee 102, +1
		0x01, 0x00, // critical 1
		0xb8, 0x01, 0x00, 0x00, // amotion 440
	}

	got := DecodeStatus(pkt)
	if got == nil {
		t.Fatal("a full packet must decode")
	}

	for _, c := range []struct {
		name string
		got  int
		want int
	}{
		{"Atk", got.Atk, 1},
		{"AtkBonus", got.AtkBonus, 0},
		{"MatkMax", got.MatkMax, 0},
		{"MatkMin", got.MatkMin, 1},
		{"Def", got.Def, 1},
		{"Mdef", got.Mdef, 1},
		{"Hit", got.Hit, 177},
		{"Flee", got.Flee, 102},
		{"FleeBonus", got.FleeBonus, 1},
		{"Critical", got.Critical, 1},
		// 200 - 440/10.
		{"Aspd", got.Aspd, 156},
	} {
		if c.got != c.want {
			t.Errorf("%s = %d, want %d", c.name, c.got, c.want)
		}
	}
}

// TestDecodeStatusWithoutDerived: a packet with the six stats but nothing
// after them leaves the derived numbers zero rather than reading past its end.
func TestDecodeStatusWithoutDerived(t *testing.T) {
	pkt := make([]byte, 16)
	binary.LittleEndian.PutUint16(pkt[0:], ZC_STATUS)

	got := DecodeStatus(pkt)
	if got == nil {
		t.Fatal("the six stats alone must still decode")
	}
	if got.Hit != 0 || got.Aspd != 0 {
		t.Errorf("derived read from a short packet: Hit=%d Aspd=%d", got.Hit, got.Aspd)
	}
}
