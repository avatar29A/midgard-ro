package packets

import (
	"encoding/binary"
	"testing"
)

// wearAck builds a ZC_REQ_WEAR_EQUIP_ACK the way the server does.
func wearAck(index int, position uint32, sprite uint16, result uint8) []byte {
	pkt := make([]byte, 11)
	binary.LittleEndian.PutUint16(pkt, ZC_REQ_WEAR_EQUIP_ACK)
	binary.LittleEndian.PutUint16(pkt[2:], uint16(index))
	binary.LittleEndian.PutUint32(pkt[4:], position)
	binary.LittleEndian.PutUint16(pkt[8:], sprite)
	pkt[10] = result

	return pkt
}

// takeoffAck builds a ZC_REQ_TAKEOFF_EQUIP_ACK.
func takeoffAck(index int, position uint32, flag uint8) []byte {
	pkt := make([]byte, 9)
	binary.LittleEndian.PutUint16(pkt, ZC_REQ_TAKEOFF_EQUIP_ACK)
	binary.LittleEndian.PutUint16(pkt[2:], uint16(index))
	binary.LittleEndian.PutUint32(pkt[4:], position)
	pkt[8] = flag

	return pkt
}

// TestDecodeEquipAck: the fields come out where the 0x0999 struct puts them,
// which is the shape with a 32-bit wearLocation and a sprite before the flag.
func TestDecodeEquipAck(t *testing.T) {
	ack, ok := DecodeEquipAck(wearAck(7, EQP_HAND_R, 1, EquipAckOK))
	if !ok {
		t.Fatal("a full-length ack did not decode")
	}

	if ack.Index != 7 {
		t.Errorf("Index = %d, want 7", ack.Index)
	}
	if ack.Position != EQP_HAND_R {
		t.Errorf("Position = %#x, want %#x", ack.Position, EQP_HAND_R)
	}
	if ack.Sprite != 1 {
		t.Errorf("Sprite = %d, want 1", ack.Sprite)
	}
	if !ack.OK() {
		t.Error("a zero result should mean the item is worn")
	}
	if ack.Reason() != "" {
		t.Errorf("a success carries a refusal reason: %q", ack.Reason())
	}
}

// TestEquipAckRefusals: zero is success at this packetver and the two refusals
// are told apart, because "too low a level" is worth saying and "no" is not
// the same message.
func TestEquipAckRefusals(t *testing.T) {
	for _, tc := range []struct {
		name   string
		result uint8
		ok     bool
	}{
		{"ok", EquipAckOK, true},
		{"level", EquipAckFailLevel, false},
		{"refused", EquipAckFail, false},
	} {
		ack, _ := DecodeEquipAck(wearAck(1, EQP_ARMOR, 0, tc.result))

		if ack.OK() != tc.ok {
			t.Errorf("%s: OK() = %v, want %v", tc.name, ack.OK(), tc.ok)
		}
		if (ack.Reason() == "") != tc.ok {
			t.Errorf("%s: Reason() = %q with OK() = %v", tc.name, ack.Reason(), ack.OK())
		}
	}

	if a, b := mustEquipReason(t, EquipAckFailLevel), mustEquipReason(t, EquipAckFail); a == b {
		t.Errorf("both refusals say the same thing: %q", a)
	}
}

func mustEquipReason(t *testing.T, result uint8) string {
	t.Helper()

	ack, ok := DecodeEquipAck(wearAck(1, EQP_ARMOR, 0, result))
	if !ok {
		t.Fatal("ack did not decode")
	}

	return ack.Reason()
}

// TestDecodeUnequipAckFlagIsInverted: clif_unequipitemack negates success for
// anything past 20110824, so zero means it came off. Reading it the natural
// way round makes every successful unequip look refused.
func TestDecodeUnequipAckFlagIsInverted(t *testing.T) {
	ack, ok := DecodeUnequipAck(takeoffAck(4, EQP_SHOES, 0))
	if !ok {
		t.Fatal("a full-length ack did not decode")
	}
	if !ack.OK {
		t.Error("flag 0 should mean the item came off")
	}
	if ack.Index != 4 || ack.Position != EQP_SHOES {
		t.Errorf("index/position = %d/%#x, want 4/%#x", ack.Index, ack.Position, EQP_SHOES)
	}

	refused, _ := DecodeUnequipAck(takeoffAck(4, EQP_SHOES, 1))
	if refused.OK {
		t.Error("flag 1 should mean the server refused")
	}
}

// TestEquipAcksRejectShortPackets: a truncated packet reads past its end
// rather than returning something wrong.
func TestEquipAcksRejectShortPackets(t *testing.T) {
	if _, ok := DecodeEquipAck(wearAck(1, EQP_ARMOR, 0, 0)[:10]); ok {
		t.Error("a short wear ack decoded")
	}
	if _, ok := DecodeUnequipAck(takeoffAck(1, EQP_ARMOR, 0)[:8]); ok {
		t.Error("a short takeoff ack decoded")
	}
}

// TestEquipSlotsAreDistinct: the window's ten places are ten different bits.
// The accessory pair is the one worth checking — the right one is 0x08 and the
// left 0x80, which is not the order they look like they should be in.
func TestEquipSlotsAreDistinct(t *testing.T) {
	seen := map[uint32]bool{}
	for _, slot := range EquipSlots {
		if slot == 0 {
			t.Error("a slot has no bit")
		}
		if seen[slot] {
			t.Errorf("slot %#x is listed twice", slot)
		}
		seen[slot] = true
	}

	if len(EquipSlots) != 10 {
		t.Errorf("the window has %d slots, want 10", len(EquipSlots))
	}
	if EQP_ACC_R != 0x08 || EQP_ACC_L != 0x80 {
		t.Errorf("accessory bits are %#x/%#x, want 0x08/0x80 as mmo.hpp has them",
			EQP_ACC_R, EQP_ACC_L)
	}
}
