package packets

import (
	"encoding/binary"
	"testing"
)

// buyListPacket builds a ZC_PC_PURCHASE_ITEMLIST the way the server does at
// this packetver: nineteen bytes an entry, with the sprite between the type
// and the location.
func buyListPacket(items ...ShopItem) []byte {
	pkt := make([]byte, shopListHeaderLen+len(items)*shopBuyEntryLen)
	binary.LittleEndian.PutUint16(pkt, ZC_PC_PURCHASE_ITEMLIST)
	binary.LittleEndian.PutUint16(pkt[2:], uint16(len(pkt)))

	for i, item := range items {
		at := shopListHeaderLen + i*shopBuyEntryLen
		binary.LittleEndian.PutUint32(pkt[at:], item.ID)
		binary.LittleEndian.PutUint32(pkt[at+4:], item.Price)
		binary.LittleEndian.PutUint32(pkt[at+8:], item.Discount)
		pkt[at+12] = item.Kind
		binary.LittleEndian.PutUint16(pkt[at+13:], 7) // the sprite, unread
		binary.LittleEndian.PutUint32(pkt[at+15:], item.Location)
	}

	return pkt
}

// sellListPacket builds a ZC_PC_PURCHASE_SELL_ITEMLIST: ten bytes an entry.
func sellListPacket(items ...SellItem) []byte {
	pkt := make([]byte, shopListHeaderLen+len(items)*shopSellEntryLen)
	binary.LittleEndian.PutUint16(pkt, ZC_PC_SELL_ITEMLIST)
	binary.LittleEndian.PutUint16(pkt[2:], uint16(len(pkt)))

	for i, item := range items {
		at := shopListHeaderLen + i*shopSellEntryLen
		binary.LittleEndian.PutUint16(pkt[at:], uint16(item.Index))
		binary.LittleEndian.PutUint32(pkt[at+2:], item.Price)
		binary.LittleEndian.PutUint32(pkt[at+6:], item.Overcharge)
	}

	return pkt
}

// TestDecodeShopItems: the sprite sits between the type and the location and
// is of no use here, so the location is two bytes further along than a reader
// that forgot it would look.
func TestDecodeShopItems(t *testing.T) {
	want := []ShopItem{
		{ID: 734, Price: 2250, Discount: 2250, Kind: 3},
		{ID: 1101, Price: 100, Discount: 90, Kind: 4, Location: EQP_HAND_R},
	}

	got := DecodeShopItems(buyListPacket(want...))

	if len(got) != len(want) {
		t.Fatalf("decoded %d items, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("item %d came out as %+v, want %+v", i, got[i], want[i])
		}
	}
}

// TestDecodeSellItems: the shop names slots rather than items, since two of
// the same thing in a bag are not worth the same money.
func TestDecodeSellItems(t *testing.T) {
	want := []SellItem{
		{Index: 3, Price: 5, Overcharge: 6},
		{Index: 61, Price: 1125, Overcharge: 1237},
	}

	got := DecodeSellItems(sellListPacket(want...))

	if len(got) != len(want) {
		t.Fatalf("decoded %d items, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("item %d came out as %+v, want %+v", i, got[i], want[i])
		}
	}
}

// TestShopListsDropATruncatedTail: a list whose entry size does not divide its
// body is a version mismatch, not something to improvise on.
func TestShopListsDropATruncatedTail(t *testing.T) {
	whole := buyListPacket(ShopItem{ID: 734}, ShopItem{ID: 735})

	// Half an entry short, and the declared length says so.
	short := whole[:len(whole)-10]
	binary.LittleEndian.PutUint16(short[2:], uint16(len(short)))

	if got := DecodeShopItems(short); len(got) != 1 {
		t.Errorf("a truncated tail decoded as %d items, want the one whole entry", len(got))
	}

	if got := DecodeShopItems(whole[:3]); len(got) != 0 {
		t.Errorf("a packet too short for its own header decoded as %+v", got)
	}
}

// TestEncodeBuyNamesItems: every copy on a shelf is the same, so an order for
// one names the item.
func TestEncodeBuyNamesItems(t *testing.T) {
	pkt := EncodeBuy([]ShopOrder{{ID: 501, Amount: 3}, {ID: 601, Amount: 1}})

	if id := binary.LittleEndian.Uint16(pkt); id != CZ_PC_PURCHASE_ITEMLIST {
		t.Errorf("packet id is %#x, want %#x", id, CZ_PC_PURCHASE_ITEMLIST)
	}
	if length := binary.LittleEndian.Uint16(pkt[2:]); int(length) != len(pkt) {
		t.Errorf("declared length is %d for %d bytes", length, len(pkt))
	}
	if len(pkt) != shopListHeaderLen+2*6 {
		t.Fatalf("two lines came to %d bytes", len(pkt))
	}

	// Amount first, then a four-byte id: item ids outgrew sixteen bits.
	if amount := binary.LittleEndian.Uint16(pkt[4:]); amount != 3 {
		t.Errorf("the first amount is %d, want 3", amount)
	}
	if id := binary.LittleEndian.Uint32(pkt[6:]); id != 501 {
		t.Errorf("the first item is %d, want 501", id)
	}
}

// TestEncodeSellNamesSlots: two of the same thing in a bag are not the same
// thing, so an order to sell names the slot.
func TestEncodeSellNamesSlots(t *testing.T) {
	pkt := EncodeSell([]ShopOrder{{Index: 7, Amount: 2}})

	if id := binary.LittleEndian.Uint16(pkt); id != CZ_PC_SELL_ITEMLIST {
		t.Errorf("packet id is %#x, want %#x", id, CZ_PC_SELL_ITEMLIST)
	}
	if len(pkt) != shopListHeaderLen+4 {
		t.Fatalf("one line came to %d bytes", len(pkt))
	}
	if index := binary.LittleEndian.Uint16(pkt[4:]); index != 7 {
		t.Errorf("the slot is %d, want 7", index)
	}
	if amount := binary.LittleEndian.Uint16(pkt[6:]); amount != 2 {
		t.Errorf("the amount is %d, want 2", amount)
	}
}

// TestAnEmptyOrderIsNotSent: the server answers a list of nothing with a
// refusal, which reads as a fault the player did not cause.
func TestAnEmptyOrderIsNotSent(t *testing.T) {
	if pkt := EncodeBuy(nil); pkt != nil {
		t.Errorf("an empty order encoded as %d bytes", len(pkt))
	}
	if pkt := EncodeSell(nil); pkt != nil {
		t.Errorf("an empty order encoded as %d bytes", len(pkt))
	}
}

// TestShopRefusalsAreToldApart: "no" is not a message. Every refusal the
// server can give has words of its own, and a success has none.
func TestShopRefusalsAreToldApart(t *testing.T) {
	if got := ShopRefusal(ShopOK); got != "" {
		t.Errorf("a sale that went through says %q", got)
	}

	said := map[string]uint8{}
	for _, result := range []uint8{
		ShopNoZeny, ShopOverweight, ShopOutOfStock,
		ShopTooManyKind, ShopTooMuch, ShopTooFar,
	} {
		words := ShopRefusal(result)
		if words == "" {
			t.Errorf("refusal %d says nothing", result)
		}
		if before, twice := said[words]; twice {
			t.Errorf("refusals %d and %d both say %q", before, result, words)
		}
		said[words] = result
	}

	// One nobody has words for still says something rather than nothing.
	if got := ShopRefusal(200); got == "" {
		t.Error("an unknown refusal says nothing at all")
	}
}
