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

// CH_PING keeps a character-server session alive.
//
//	0187 <account id>.L
//
// The server drops any session that has sent nothing for stall_time seconds,
// 60 by default (common/socket.cpp). Character select and character creation
// both sit idle for longer than that while a person reads the screen, so
// something has to be said on the connection or it closes underneath them.
//
// Source: common/packets.hpp:317-321, char/char_clif.cpp:1395,1604.
const CH_PING uint16 = 0x0187

// EncodePing builds the keep-alive.
func EncodePing(accountID uint32) []byte {
	pkt := make([]byte, 6)
	writeU16(pkt, 0, CH_PING)
	writeU32(pkt, 2, accountID)

	return pkt
}

// Character creation, as our packet version does it.
//
// CH_MAKE_CHAR has three shapes behind PACKETVER guards and we get the newest:
// at 20151001 and later it carries a job and a sex and no starting stats. The
// six stat bytes exist only in the pre-20120307 form; here the server writes 1
// to each itself and hands out status points to spend in game.
//
// Source: common/packets.hpp:122-132, char/char_clif.cpp:1265-1310.
// CH_MAKE_CHAR (0x0A39, 36 bytes) and HC_ACCEPT_MAKECHAR (0x0B6F) are
// declared in packets.go with the rest of the char-server ids.
const (
	// HC_REFUSE_MAKECHAR says why a creation was refused.
	HC_REFUSE_MAKECHAR uint16 = 0x006E
)

// Why a creation was refused, as the wire reports it
// (char/char_clif.cpp:1330-1352).
const (
	// MakeCharNameTaken is the one worth wording carefully: it is the only
	// answer to "is this name free", because nothing asks that question.
	MakeCharNameTaken uint8 = 0x00
	// MakeCharUnderaged is a birthday check we never trip.
	MakeCharUnderaged uint8 = 0x01
	// MakeCharSlotNotAllowed is a slot the account may not open.
	MakeCharSlotNotAllowed uint8 = 0x03
	// MakeCharDenied covers everything else the server refused: a name it
	// dislikes, a job it does not allow, a slot already in use, the account
	// limit.
	MakeCharDenied uint8 = 0xFF
)

// MakeCharRequest is the look a new character is created with.
type MakeCharRequest struct {
	Name      string
	Slot      uint8
	HairColor uint16
	HairStyle uint16
	Job       uint32
	// Sex is 0 female, 1 male. The server accepts nothing else.
	Sex uint8
}

// EncodeMakeChar builds the creation request.
//
// Returns nil when the name cannot go on the wire — empty, or too long for
// the 24-byte field to hold it with a terminator. Every other rule the server
// applies is the server's to apply; this only refuses what it could not
// encode truthfully.
func EncodeMakeChar(req MakeCharRequest) []byte {
	if req.Name == "" || len(req.Name) >= nameLength {
		return nil
	}

	// type, name[24], slot, hair color, hair style, job, sex.
	const length = 2 + nameLength + 1 + 2 + 2 + 4 + 1

	pkt := make([]byte, length)
	writeU16(pkt, 0, CH_MAKE_CHAR)
	copy(pkt[2:2+nameLength], req.Name)
	pkt[2+nameLength] = req.Slot
	writeU16(pkt, 27, req.HairColor)
	writeU16(pkt, 29, req.HairStyle)
	writeU32(pkt, 31, req.Job)
	pkt[35] = req.Sex

	return pkt
}

// DecodeMakeCharRefuse reads why a creation was refused. Reports false when
// the packet is too short to say.
func DecodeMakeCharRefuse(data []byte) (uint8, bool) {
	if len(data) < 3 {
		return 0, false
	}

	return data[2], true
}

// MakeCharFailure describes a refusal in words a player can act on.
func MakeCharFailure(code uint8) string {
	switch code {
	case MakeCharNameTaken:
		return "That name is already taken."
	case MakeCharUnderaged:
		return "This account is not old enough to create a character."
	case MakeCharSlotNotAllowed:
		return "This account cannot use that slot."
	default:
		return "The server would not create that character."
	}
}

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
