package packets

import (
	"encoding/binary"
	"testing"
)

// TestEncodeRestartCharSelect checks the byte rAthena switches on: 1 is the
// hand-back to the character server, 0 is a respawn. Getting it wrong sends
// you to your save point instead of to character select.
func TestEncodeRestartCharSelect(t *testing.T) {
	pkt := EncodeRestart(RestartCharSelect)

	if len(pkt) != 3 {
		t.Fatalf("len = %d, want 3", len(pkt))
	}
	if got := binary.LittleEndian.Uint16(pkt[0:2]); got != CZ_REQ_RESTART {
		t.Errorf("id = 0x%04X, want 0x%04X", got, CZ_REQ_RESTART)
	}
	if pkt[2] != 1 {
		t.Errorf("type = %d, want 1", pkt[2])
	}
}

func TestEncodeDisconnectLength(t *testing.T) {
	pkt := EncodeDisconnect()

	// The server declares this packet as 4 bytes; a shorter one desynchronises
	// the stream rather than being ignored.
	if len(pkt) != 4 {
		t.Fatalf("len = %d, want 4", len(pkt))
	}
	if got := binary.LittleEndian.Uint16(pkt[0:2]); got != CZ_REQ_DISCONNECT {
		t.Errorf("id = 0x%04X, want 0x%04X", got, CZ_REQ_DISCONNECT)
	}
}

// TestDecodeAcksAgainstLengthTable ties the decoders to the sizes the framing
// already declares, so a decoder reading past its packet shows up here.
func TestDecodeAcksAgainstLengthTable(t *testing.T) {
	if n, ok := Length(ZC_RESTART_ACK); !ok || n != 3 {
		t.Errorf("ZC_RESTART_ACK length = %d, %v; want 3, true", n, ok)
	}
	if n, ok := Length(ZC_ACK_REQ_DISCONNECT); !ok || n != 4 {
		t.Errorf("ZC_ACK_REQ_DISCONNECT length = %d, %v; want 4, true", n, ok)
	}

	if got, ok := DecodeRestartAck([]byte{0xB3, 0x00, RestartCharSelect}); !ok || got != RestartCharSelect {
		t.Errorf("DecodeRestartAck = %d, %v", got, ok)
	}
	if _, ok := DecodeRestartAck([]byte{0xB3, 0x00}); ok {
		t.Error("a packet with no type byte must not decode")
	}

	if got, ok := DecodeDisconnectAck([]byte{0x8B, 0x01, 0, 0}); !ok || got != DisconnectGranted {
		t.Errorf("DecodeDisconnectAck = %d, %v; want granted", got, ok)
	}
	if got, _ := DecodeDisconnectAck([]byte{0x8B, 0x01, 1, 0}); got == DisconnectGranted {
		t.Error("result 1 is a refusal, not a grant")
	}
}

// TestDecodeSkillList walks the 15-byte entry rAthena declares for our packet
// version: id, inf, level, sp, range, upFlag, level2. The name field earlier
// versions carried is gone, which is why entries are 15 bytes and not 39.
func TestDecodeSkillList(t *testing.T) {
	entry := func(id uint16, level, sp, rng int, up byte) []byte {
		b := make([]byte, skillEntryLen)
		binary.LittleEndian.PutUint16(b[0:], id)
		binary.LittleEndian.PutUint32(b[2:], 1) // inf
		binary.LittleEndian.PutUint16(b[6:], uint16(level))
		binary.LittleEndian.PutUint16(b[8:], uint16(sp))
		binary.LittleEndian.PutUint16(b[10:], uint16(rng))
		b[12] = up
		binary.LittleEndian.PutUint16(b[13:], uint16(level))

		return b
	}

	body := append(entry(1, 9, 0, 0, 1), entry(5, 3, 8, 1, 0)...)

	pkt := make([]byte, 4)
	binary.LittleEndian.PutUint16(pkt[0:], ZC_SKILLINFO_LIST)
	binary.LittleEndian.PutUint16(pkt[2:], uint16(4+len(body)))
	pkt = append(pkt, body...)

	got := DecodeSkillList(pkt)
	if len(got) != 2 {
		t.Fatalf("decoded %d skills, want 2", len(got))
	}

	if got[0].ID != 1 || got[0].Level != 9 || !got[0].Raisable {
		t.Errorf("first = %+v, want id 1 level 9 raisable", got[0])
	}
	if got[1].ID != 5 || got[1].Level != 3 || got[1].SP != 8 || got[1].Range != 1 {
		t.Errorf("second = %+v, want id 5 level 3 sp 8 range 1", got[1])
	}
	if got[1].Raisable {
		t.Error("second should not be raisable")
	}
}

// TestDecodeSkillListTruncated: a declared length that does not divide into
// whole entries drops the partial tail rather than reading past it.
func TestDecodeSkillListTruncated(t *testing.T) {
	pkt := make([]byte, 4+skillEntryLen+5)
	binary.LittleEndian.PutUint16(pkt[0:], ZC_SKILLINFO_LIST)
	binary.LittleEndian.PutUint16(pkt[2:], uint16(len(pkt)))

	if got := DecodeSkillList(pkt); len(got) != 1 {
		t.Errorf("decoded %d skills, want 1 — the partial entry must be dropped", len(got))
	}
}

// TestDecodeSkillListEmpty: a character with no skills sends a header and
// nothing else, which is a list of none rather than a malformed packet.
func TestDecodeSkillListEmpty(t *testing.T) {
	pkt := []byte{0x32, 0x0B, 4, 0}

	if got := DecodeSkillList(pkt); got != nil {
		t.Errorf("decoded %v, want nil", got)
	}
}

// TestDecodeInventoryNormal walks the 34-byte entry our packet version
// declares: index, ITID, type, count, WearState, four uint32 card slots,
// HireExpireDate and a flag byte.
//
// The card slots are where this is easy to get wrong — they were four uint16
// until PACKETVER_RE 20180704 and are four uint32 after, eight bytes a row.
func TestDecodeInventoryNormal(t *testing.T) {
	entry := func(index int, id uint32, count int) []byte {
		b := make([]byte, NormalItemLen)
		binary.LittleEndian.PutUint16(b[0:], uint16(index))
		binary.LittleEndian.PutUint32(b[2:], id)
		b[6] = 3 // type
		binary.LittleEndian.PutUint16(b[7:], uint16(count))

		return b
	}

	body := append(entry(2, 501, 5), entry(3, 512, 10)...)

	pkt := make([]byte, itemListHeaderLen)
	binary.LittleEndian.PutUint16(pkt[0:], ZC_INVENTORY_ITEMLIST_NORMAL)
	binary.LittleEndian.PutUint16(pkt[2:], uint16(itemListHeaderLen+len(body)))
	pkt = append(pkt, body...)

	got := DecodeInventoryNormal(pkt)
	if len(got) != 2 {
		t.Fatalf("decoded %d items, want 2", len(got))
	}
	if got[0].ID != 501 || got[0].Count != 5 || got[0].Index != 2 {
		t.Errorf("first = %+v, want index 2 id 501 count 5", got[0])
	}
	if got[1].ID != 512 || got[1].Count != 10 {
		t.Errorf("second = %+v, want id 512 count 10", got[1])
	}
}

// TestItemListRemainderCatchesWrongEntrySize is the guard that matters most
// here: the entry sizes are read off the server's structs with our version's
// guards resolved by hand, and a wrong one would otherwise fill the window
// with nonsense rather than say anything.
func TestItemListRemainderCatchesWrongEntrySize(t *testing.T) {
	pkt := make([]byte, itemListHeaderLen+NormalItemLen)
	binary.LittleEndian.PutUint16(pkt[0:], ZC_INVENTORY_ITEMLIST_NORMAL)
	binary.LittleEndian.PutUint16(pkt[2:], uint16(len(pkt)))

	if left := ItemListRemainder(pkt, NormalItemLen); left != 0 {
		t.Errorf("remainder = %d for the size it was built with, want 0", left)
	}

	// A size that does not divide must report the leftovers, and decoding
	// with it must return nothing rather than a misread list.
	if left := ItemListRemainder(pkt, NormalItemLen-1); left == 0 {
		t.Error("a size that does not divide must report a remainder")
	}
	if got := decodeItemList(pkt, NormalItemLen-1, func([]byte) InventoryItem { return InventoryItem{} }); got != nil {
		t.Errorf("decoded %v with the wrong entry size, want nothing", got)
	}
}

// TestDecodeUseItemAck reads the answer to using an item.
//
// The amount is what is left rather than what was spent, and the account id
// matters because rAthena broadcasts the success case to everyone nearby — an
// ack decoded without checking it would let a stranger's potion decrement our
// own inventory.
func TestDecodeUseItemAck(t *testing.T) {
	pkt := make([]byte, 15)
	binary.LittleEndian.PutUint16(pkt, ZC_USE_ITEM_ACK)
	binary.LittleEndian.PutUint16(pkt[2:], 2)       // index
	binary.LittleEndian.PutUint32(pkt[4:], 501)     // red potion
	binary.LittleEndian.PutUint32(pkt[8:], 2000000) // account
	binary.LittleEndian.PutUint16(pkt[12:], 4)      // four left
	pkt[14] = 1                                     // used

	ack, ok := DecodeUseItemAck(pkt)
	if !ok {
		t.Fatal("a 15-byte ack should decode")
	}
	if ack.Index != 2 {
		t.Errorf("Index = %d, want 2", ack.Index)
	}
	if ack.ItemID != 501 {
		t.Errorf("ItemID = %d, want 501", ack.ItemID)
	}
	if ack.AccountID != 2000000 {
		t.Errorf("AccountID = %d, want 2000000", ack.AccountID)
	}
	if ack.Amount != 4 {
		t.Errorf("Amount = %d, want 4 (what is left, not what was spent)", ack.Amount)
	}
	if !ack.OK {
		t.Error("OK = false, want true")
	}
}

// TestDecodeUseItemAckRefused: result 0 means the server kept the item.
func TestDecodeUseItemAckRefused(t *testing.T) {
	pkt := make([]byte, 15)
	binary.LittleEndian.PutUint16(pkt, ZC_USE_ITEM_ACK)
	binary.LittleEndian.PutUint16(pkt[2:], 2)
	pkt[14] = 0

	ack, ok := DecodeUseItemAck(pkt)
	if !ok {
		t.Fatal("a 15-byte ack should decode")
	}
	if ack.OK {
		t.Error("OK = true for result 0, which would spend an item the server kept")
	}
}

// TestDecodeUseItemAckShort: a truncated ack must report false rather than
// read past the end of the buffer.
func TestDecodeUseItemAckShort(t *testing.T) {
	if _, ok := DecodeUseItemAck(make([]byte, 14)); ok {
		t.Error("a 14-byte ack decoded, but the packet is 15")
	}
}
