package packets

import (
	"fmt"
	"strings"
)

// NPC conversation packets.
//
// The server owns the whole conversation: it sends text, then asks for a Next,
// a Close or a choice from a menu. The client only reports what the player
// did. Ids and layouts are from the server's own headers at PACKETVER 20211103
// — `packets_struct.hpp` for the structs and `clif_packetdb.hpp` for what it
// actually parses.
//
// A note on 0x0BA8: `PACKET_CZ_CHOOSE_MENU_ZERO` is declared for
// `PACKETVER_RE_NUM >= 20211103`, which our server satisfies, and it looks
// like the modern replacement for 0x00B8. It is not: `clif_packetdb.hpp`
// registers `0x00b8` and never mentions 0x0ba8, so the older id is the one the
// server listens for.
const (
	// CZ_CONTACTNPC starts a conversation. `type(2) AID(4) kind(1)`.
	CZ_CONTACTNPC uint16 = 0x0090
	// ZC_SAY_DIALOG is what the NPC said. `type(2) len(2) npcId(4) message[]`.
	ZC_SAY_DIALOG uint16 = 0x00B4
	// ZC_WAIT_DIALOG asks for a Next button. `type(2) npcId(4)`.
	ZC_WAIT_DIALOG uint16 = 0x00B5
	// ZC_CLOSE_DIALOG asks for a Close button. `type(2) npcId(4)`.
	ZC_CLOSE_DIALOG uint16 = 0x00B6
	// ZC_MENU_LIST offers choices. `type(2) len(2) npcId(4) menu[]`.
	ZC_MENU_LIST uint16 = 0x00B7
	// CZ_CHOOSE_MENU reports the choice. `type(2) npcId(4) select(1)`.
	CZ_CHOOSE_MENU uint16 = 0x00B8
	// CZ_REQ_NEXT_SCRIPT reports a Next click. `type(2) npcId(4)`.
	CZ_REQ_NEXT_SCRIPT uint16 = 0x00B9
	// CZ_CLOSE_DIALOG reports a Close click. `type(2) npcId(4)`.
	CZ_CLOSE_DIALOG uint16 = 0x0146
)

const (
	// menuCancel is the selection that means the player backed out.
	menuCancel = 0xFF

	// maxMenuItems is what a selection byte can carry before it wraps. The
	// server refuses to build a longer menu, warning "Too many options
	// specified (current=%d, max=254)".
	maxMenuItems = 254

	// contactKind is the `type` byte of CZ_CONTACTNPC. The server ignores it
	// — `clif_parse_NpcClicked` switches on the target's own block type, not
	// on this — and the original sends zero.
	contactKind = 0

	// sayDialogHeader is the size of ZC_SAY_DIALOG and ZC_MENU_LIST before
	// their variable part: id, length and npc id.
	sayDialogHeader = 8
)

// ContactNPC asks to start talking to a unit.
func ContactNPC(npcID uint32) []byte {
	buf := make([]byte, 7)
	buf[0], buf[1] = byte(CZ_CONTACTNPC), byte(CZ_CONTACTNPC>>8)
	writeU32(buf, 2, npcID)
	buf[6] = contactKind

	return buf
}

// RequestNextScript reports that the player clicked Next.
func RequestNextScript(npcID uint32) []byte {
	return npcIDPacket(CZ_REQ_NEXT_SCRIPT, npcID)
}

// CloseDialogPacket reports that the player clicked Close.
func CloseDialogPacket(npcID uint32) []byte {
	return npcIDPacket(CZ_CLOSE_DIALOG, npcID)
}

// CancelMenu backs out of a menu without choosing anything.
func CancelMenu(npcID uint32) []byte {
	return chooseMenuPacket(npcID, menuCancel)
}

// ChooseMenu reports a menu selection, which is 1-based.
//
// An index the server would reject is refused here instead of sent.
// `clif_parse_NpcSelectMenu` calls `clif_GM_kick` for a selection of zero or
// one past the end, so getting this wrong is a disconnection rather than a
// wrong branch — worth catching on our side of the wire.
func ChooseMenu(npcID uint32, choice, itemCount int) ([]byte, error) {
	if itemCount < 1 || itemCount > maxMenuItems {
		return nil, fmt.Errorf("menu of %d items is not selectable (1..%d)", itemCount, maxMenuItems)
	}

	if choice < 1 || choice > itemCount {
		return nil, fmt.Errorf("menu choice %d is outside 1..%d", choice, itemCount)
	}

	return chooseMenuPacket(npcID, byte(choice)), nil
}

// npcIDPacket builds one of the several 6-byte `id + npc id` packets.
func npcIDPacket(packetID uint16, npcID uint32) []byte {
	buf := make([]byte, 6)
	buf[0], buf[1] = byte(packetID), byte(packetID>>8)
	writeU32(buf, 2, npcID)

	return buf
}

func chooseMenuPacket(npcID uint32, choice byte) []byte {
	buf := make([]byte, 7)
	buf[0], buf[1] = byte(CZ_CHOOSE_MENU), byte(CZ_CHOOSE_MENU>>8)
	writeU32(buf, 2, npcID)
	buf[6] = choice

	return buf
}

// NPCSay is a decoded ZC_SAY_DIALOG.
type NPCSay struct {
	NPCID   uint32
	Message string
}

// DecodeSayDialog decodes what the NPC said, or nil if the packet is short.
//
// The message runs to the packet's own length and ends in a NUL the server
// counts (`clif_scriptmes` sends `strlen(mes) + 1`), so the terminator is
// trimmed rather than kept as part of the text.
func DecodeSayDialog(data []byte) *NPCSay {
	npcID, body, ok := decodeVariableDialog(data)
	if !ok {
		return nil
	}

	return &NPCSay{NPCID: npcID, Message: body}
}

// NPCMenu is a decoded ZC_MENU_LIST.
type NPCMenu struct {
	NPCID uint32
	Items []string
}

// DecodeMenuList decodes a menu, or nil if the packet is short.
func DecodeMenuList(data []byte) *NPCMenu {
	npcID, body, ok := decodeVariableDialog(data)
	if !ok {
		return nil
	}

	return &NPCMenu{NPCID: npcID, Items: SplitMenu(body)}
}

// DecodeDialogNPCID decodes the packets that carry nothing but an npc id —
// ZC_WAIT_DIALOG and ZC_CLOSE_DIALOG.
func DecodeDialogNPCID(data []byte) (uint32, bool) {
	if len(data) < 6 {
		return 0, false
	}

	return readU32(data, 2), true
}

// decodeVariableDialog reads the header shared by ZC_SAY_DIALOG and
// ZC_MENU_LIST and returns the text after it.
func decodeVariableDialog(data []byte) (npcID uint32, body string, ok bool) {
	if len(data) < sayDialogHeader {
		return 0, "", false
	}

	// The packet's own length bounds the text. Trusting len(data) instead
	// would swallow whatever the next packet is when several arrive in one
	// read.
	end := int(readU16(data, 2))
	if end > len(data) {
		end = len(data)
	}

	if end < sayDialogHeader {
		return 0, "", false
	}

	return readU32(data, 4), strings.TrimRight(string(data[sayDialogHeader:end]), "\x00"), true
}

// SplitMenu splits the server's menu string into its selectable items.
//
// The menu arrives as one colon-separated string and the client sends back the
// item's 1-based position. Empty entries are dropped, and that is what makes
// the position right: `menu_countoptions` in the server's script engine counts
// only non-empty options into `sd->npc_menu`, which is the bound it then
// checks the selection against. A menu of `a::b` offers two choices, and `b`
// is choice 2 — not 3.
func SplitMenu(raw string) []string {
	items := make([]string, 0, strings.Count(raw, ":")+1)

	for _, item := range strings.Split(raw, ":") {
		if item != "" {
			items = append(items, item)
		}
	}

	return items
}
