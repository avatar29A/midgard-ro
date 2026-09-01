package packets

// HC_ACCEPT_ENTER2 carries the character-select window's slot counts.
//
//	082d <len>.W <normal>.B <premium>.B <billing>.B <producible>.B <total>.B <extension>.20B
//
// From PACKETVER 20130000 the char server sends this *before* the character
// list (chclif_mmo_char_send), and it is the only packet that carries the
// account's own slot limit. HC_ACCEPT_ENTER 0x006B, which carries the
// characters, does not: its three bytes are total, premium_start and
// premium_end, so reading a creatable count out of it gives the compile-time
// MAX_CHARS instead of what this account may actually use.
//
// Source: common/packets.hpp:508-518, char/char_clif.cpp:435-451,477-481.
const HC_ACCEPT_ENTER2 uint16 = 0x082D

// SlotCounts is what HC_ACCEPT_ENTER2 says about an account's slots.
type SlotCounts struct {
	// Normal is MIN_CHARS — the slots every account has.
	Normal uint8
	// Premium and Billing are the VIP and billing extras this account has.
	Premium uint8
	Billing uint8
	// Producible is how many characters this account may create. This is the
	// number a creation screen must respect; going past it is refused with
	// "not eligible to open the Character Slot".
	Producible uint8
	// Total is MAX_CHARS, the compile-time ceiling — 15 on a stock server
	// regardless of what the account may use.
	Total uint8
}

// DecodeSlotCounts reads HC_ACCEPT_ENTER2. Reports false on short data.
func DecodeSlotCounts(data []byte) (SlotCounts, bool) {
	// type, length, and the five counts.
	const need = 9

	if len(data) < need {
		return SlotCounts{}, false
	}

	return SlotCounts{
		Normal:     data[4],
		Premium:    data[5],
		Billing:    data[6],
		Producible: data[7],
		Total:      data[8],
	}, true
}
