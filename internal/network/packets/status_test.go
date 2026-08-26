package packets

import "testing"

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
