package packets

import "encoding/binary"

// Buying from and selling to an NPC.
//
// Four packets each way. Talking to a shopkeeper who only sells opens the buy
// list straight away; one who does both asks first, which is its own packet
// and its own answer.
//
// The entry layouts moved with the packet version and so did one of the ids.
// Ours is the later buy list, 0x0B77, whose entries carry the sprite and the
// place on the body an item goes as well as its price — the older 0x00C6 has
// only the price and the type.
const (
	// ZC_SELECT_DEALTYPE asks whether we mean to buy or to sell:
	// `<NPC id>.L`, six bytes.
	ZC_SELECT_DEALTYPE uint16 = 0x00C4

	// CZ_ACK_SELECT_DEALTYPE answers it: `<NPC id>.L <type>.B`, seven bytes.
	CZ_ACK_SELECT_DEALTYPE uint16 = 0x00C5

	// ZC_PC_PURCHASE_ITEMLIST is what the shop sells: `<len>.W` then a run of
	// nineteen-byte entries.
	ZC_PC_PURCHASE_ITEMLIST uint16 = 0x0B77

	// CZ_PC_PURCHASE_ITEMLIST is what we are buying: `<len>.W` then a run of
	// `<amount>.W <item id>.L`.
	CZ_PC_PURCHASE_ITEMLIST uint16 = 0x00C8

	// ZC_PC_SELL_ITEMLIST is what the shop will take, by inventory slot:
	// `<len>.W` then a run of `<index>.W <price>.L <overcharge>.L`.
	ZC_PC_SELL_ITEMLIST uint16 = 0x00C7

	// CZ_PC_SELL_ITEMLIST is what we are selling: `<len>.W` then a run of
	// `<index>.W <amount>.W`.
	CZ_PC_SELL_ITEMLIST uint16 = 0x00C9

	// ZC_PC_PURCHASE_RESULT and ZC_PC_SELL_RESULT answer each: `<result>.B`,
	// three bytes. Nought is the sale going through.
	ZC_PC_PURCHASE_RESULT uint16 = 0x00CA
	ZC_PC_SELL_RESULT     uint16 = 0x00CB
)

// Which side of the counter, for CZ_ACK_SELECT_DEALTYPE.
const (
	DealBuy  uint8 = 0
	DealSell uint8 = 1
)

// Entry sizes, header included where the header is fixed.
const (
	shopListHeaderLen = 4

	shopBuyEntryLen  = 19
	shopSellEntryLen = 10
)

// ShopItem is one line of what a shop sells.
type ShopItem struct {
	ID uint32

	// Price is what it costs and Discount what it costs to this character,
	// which a Merchant's Discount skill lowers. The original shows the second.
	Price    uint32
	Discount uint32

	// Kind is rAthena's item type, and Location where on the body it goes —
	// zero for anything not worn.
	Kind     uint8
	Location uint32
}

// SellItem is one line of what a shop will take, named by inventory slot
// rather than by item: two of the same thing can be worth different money,
// since one of them may be refined.
type SellItem struct {
	Index int

	// Price is what it fetches and Overcharge what it fetches with a
	// Merchant's Overcharge skill. The original shows the second.
	Price      uint32
	Overcharge uint32
}

// DecodeDealType reads which NPC is asking whether we mean to buy or sell.
func DecodeDealType(data []byte) (uint32, bool) {
	if len(data) < 6 {
		return 0, false
	}

	return readU32(data, 2), true
}

// EncodeDealType answers it.
func EncodeDealType(npcID uint32, deal uint8) []byte {
	pkt := make([]byte, 7)
	binary.LittleEndian.PutUint16(pkt, CZ_ACK_SELECT_DEALTYPE)
	binary.LittleEndian.PutUint32(pkt[2:], npcID)
	pkt[6] = deal

	return pkt
}

// DecodeShopItems reads what a shop sells.
//
// The declared length wins over the buffer's, and a tail too short for a whole
// entry is dropped rather than read past: a list whose entry size does not
// divide its body is a version mismatch, not something to improvise on.
func DecodeShopItems(data []byte) []ShopItem {
	entries := shopListEntries(data, shopBuyEntryLen)

	items := make([]ShopItem, 0, len(entries))
	for _, at := range entries {
		items = append(items, ShopItem{
			ID:       readU32(data, at),
			Price:    readU32(data, at+4),
			Discount: readU32(data, at+8),
			Kind:     data[at+12],
			// The sprite sits between the type and the location and is of no
			// use here: the icon is looked up by item id like every other.
			Location: readU32(data, at+15),
		})
	}

	return items
}

// DecodeSellItems reads what a shop will take.
func DecodeSellItems(data []byte) []SellItem {
	entries := shopListEntries(data, shopSellEntryLen)

	items := make([]SellItem, 0, len(entries))
	for _, at := range entries {
		items = append(items, SellItem{
			Index:      int(readU16(data, at)),
			Price:      readU32(data, at+2),
			Overcharge: readU32(data, at+6),
		})
	}

	return items
}

// shopListEntries is where each whole entry of a variable-length list starts.
func shopListEntries(data []byte, entryLen int) []int {
	if len(data) < shopListHeaderLen {
		return nil
	}

	length := int(readU16(data, 2))
	if length > len(data) {
		length = len(data)
	}

	count := (length - shopListHeaderLen) / entryLen
	if count <= 0 {
		return nil
	}

	at := make([]int, 0, count)
	for i := 0; i < count; i++ {
		at = append(at, shopListHeaderLen+i*entryLen)
	}

	return at
}

// ShopOrder is one line of an order, either way across the counter.
//
// Buying names the item, since every copy on the shelf is the same; selling
// names the slot, since two of the same thing in a bag are not.
type ShopOrder struct {
	ID     uint32
	Index  int
	Amount int
}

// EncodeBuy asks to buy. Nothing is sent for an empty order: the server
// answers a list of nothing with a refusal, which reads as a fault.
func EncodeBuy(order []ShopOrder) []byte {
	if len(order) == 0 {
		return nil
	}

	const entry = 6

	pkt := make([]byte, shopListHeaderLen+len(order)*entry)
	binary.LittleEndian.PutUint16(pkt, CZ_PC_PURCHASE_ITEMLIST)
	binary.LittleEndian.PutUint16(pkt[2:], uint16(len(pkt)))

	for i, line := range order {
		at := shopListHeaderLen + i*entry
		binary.LittleEndian.PutUint16(pkt[at:], uint16(line.Amount))
		binary.LittleEndian.PutUint32(pkt[at+2:], line.ID)
	}

	return pkt
}

// EncodeSell asks to sell.
func EncodeSell(order []ShopOrder) []byte {
	if len(order) == 0 {
		return nil
	}

	const entry = 4

	pkt := make([]byte, shopListHeaderLen+len(order)*entry)
	binary.LittleEndian.PutUint16(pkt, CZ_PC_SELL_ITEMLIST)
	binary.LittleEndian.PutUint16(pkt[2:], uint16(len(pkt)))

	for i, line := range order {
		at := shopListHeaderLen + i*entry
		binary.LittleEndian.PutUint16(pkt[at:], uint16(line.Index))
		binary.LittleEndian.PutUint16(pkt[at+2:], uint16(line.Amount))
	}

	return pkt
}

// Results of a sale. Nought is it going through; the rest are why not.
const (
	ShopOK          uint8 = 0
	ShopNoZeny      uint8 = 1
	ShopOverweight  uint8 = 2
	ShopOutOfStock  uint8 = 4
	ShopTooManyKind uint8 = 5
	ShopTooMuch     uint8 = 6
	ShopTooFar      uint8 = 7
)

// DecodeShopResult reads what became of a sale.
func DecodeShopResult(data []byte) (uint8, bool) {
	if len(data) < 3 {
		return 0, false
	}

	return data[2], true
}

// ShopRefusal says why a sale did not go through, or nothing when it did.
func ShopRefusal(result uint8) string {
	switch result {
	case ShopOK:
		return ""
	case ShopNoZeny:
		return "Not enough zeny."
	case ShopOverweight:
		return "Too heavy to carry."
	case ShopOutOfStock:
		return "The shop is out of that."
	case ShopTooManyKind:
		return "No room for that many kinds of thing."
	case ShopTooMuch:
		return "That is more than the shop will trade at once."
	case ShopTooFar:
		return "Too far from the shop."
	}

	return "The shop refused."
}
