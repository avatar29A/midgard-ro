package packets

import (
	"encoding/binary"
	"testing"
)

// stateChange builds one the way the server does.
func stateChange(aid uint32, body, health uint16, effect uint32, pk byte) []byte {
	pkt := make([]byte, 15)
	binary.LittleEndian.PutUint16(pkt, ZC_STATE_CHANGE)
	binary.LittleEndian.PutUint32(pkt[2:], aid)
	binary.LittleEndian.PutUint16(pkt[6:], body)
	binary.LittleEndian.PutUint16(pkt[8:], health)
	binary.LittleEndian.PutUint32(pkt[10:], effect)
	pkt[14] = pk

	return pkt
}

// TestDecodeStateChange: the fields come out where the 0x0229 struct puts
// them, which is the shape with a four-byte effectState. The old 0x0119 has a
// two-byte one, and reading this packet that way takes the top half of the
// option word for the PK flag.
func TestDecodeStateChange(t *testing.T) {
	got, ok := DecodeStateChange(stateChange(2000000, BodyFreeze, 0x0004, 0x00120000, 0))
	if !ok {
		t.Fatal("a full-length packet did not decode")
	}

	if got.AID != 2000000 {
		t.Errorf("AID = %d, want 2000000", got.AID)
	}
	if got.Body != BodyFreeze {
		t.Errorf("Body = %d, want %d", got.Body, BodyFreeze)
	}
	if !got.Frozen() {
		t.Error("a body state of freeze does not read as frozen")
	}
	if got.Health != 0x0004 {
		t.Errorf("Health = %#x, want 0x0004", got.Health)
	}
	if got.Effect != 0x00120000 {
		t.Errorf("Effect = %#x, want 0x00120000 — a four-byte read", got.Effect)
	}
	if got.PKMode {
		t.Error("the PK flag is set on a packet that has it clear")
	}
}

// TestBodyStatesAreOneAtATime: the states drawn on a unit are a value, not a
// set of bits. Treating freeze as a flag would make a stunned unit frozen as
// well, since three is one and two together.
func TestBodyStatesAreOneAtATime(t *testing.T) {
	for _, body := range []uint16{BodyNone, BodyStone, BodyStun, BodySleep, BodyStoneWait, BodyBurning} {
		got, _ := DecodeStateChange(stateChange(1, body, 0, 0, 0))
		if got.Frozen() {
			t.Errorf("body state %d reads as frozen", body)
		}
	}

	if BodyStun != BodyStone|BodyFreeze {
		t.Fatal("this test no longer says anything; stun is no longer stone and freeze together")
	}
}

// TestDecodeStateChangeRejectsShortPackets: the old thirteen-byte shape is two
// short of this one, and reading it as this one runs off the end.
func TestDecodeStateChangeRejectsShortPackets(t *testing.T) {
	if _, ok := DecodeStateChange(stateChange(1, BodyFreeze, 0, 0, 0)[:13]); ok {
		t.Error("a thirteen-byte packet decoded as the fifteen-byte one")
	}
}
