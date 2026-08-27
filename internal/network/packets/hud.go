package packets

import "encoding/binary"

// Leaving the game: back to character select, or out altogether.
//
// The two are different requests with different answers, and the server can
// refuse either — you cannot log out while cloaking or hiding, and rAthena's
// prevent_logout keeps you in for a few seconds after combat. A client that
// tears down its socket on the button press rather than on the answer will
// sometimes drop a session the server was going to keep.
const (
	// CZ_REQ_RESTART asks to respawn or to go back to character select,
	// depending on its one byte. 3 bytes: id, then the type.
	CZ_REQ_RESTART uint16 = 0x00B2

	// CZ_REQ_DISCONNECT asks to leave the game. 4 bytes: id, then a field the
	// server does not read.
	CZ_REQ_DISCONNECT uint16 = 0x018A

	// ZC_RESTART_ACK answers CZ_REQ_RESTART, echoing the type it granted.
	ZC_RESTART_ACK uint16 = 0x00B3

	// ZC_ACK_REQ_DISCONNECT answers CZ_REQ_DISCONNECT: 0 granted, 1 refused.
	ZC_ACK_REQ_DISCONNECT uint16 = 0x018B
)

// Restart types, from rAthena's clif_parse_Restart.
const (
	// RestartRespawn returns you to the save point after dying.
	RestartRespawn uint8 = 0

	// RestartCharSelect hands you back to the character server.
	RestartCharSelect uint8 = 1
)

// DisconnectGranted is the result ZC_ACK_REQ_DISCONNECT carries when the
// server is letting you go. Anything else is a refusal.
const DisconnectGranted uint16 = 0

// EncodeRestart builds a CZ_REQ_RESTART for the given type.
func EncodeRestart(restartType uint8) []byte {
	pkt := make([]byte, 3)
	binary.LittleEndian.PutUint16(pkt, CZ_REQ_RESTART)
	pkt[2] = restartType

	return pkt
}

// EncodeDisconnect builds a CZ_REQ_DISCONNECT.
//
// The two bytes after the id are what the packet's declared length wants; the
// server reads its own field out of them and ignores the value.
func EncodeDisconnect() []byte {
	pkt := make([]byte, 4)
	binary.LittleEndian.PutUint16(pkt, CZ_REQ_DISCONNECT)

	return pkt
}

// DecodeRestartAck reads which restart the server granted. Reports false when
// the packet is too short to say.
func DecodeRestartAck(data []byte) (uint8, bool) {
	if len(data) < 3 {
		return 0, false
	}

	return data[2], true
}

// DecodeDisconnectAck reads whether we may leave. Reports false when the
// packet is too short to say.
func DecodeDisconnectAck(data []byte) (uint16, bool) {
	if len(data) < 4 {
		return 0, false
	}

	return binary.LittleEndian.Uint16(data[2:4]), true
}

// The skill list.
//
// The entry layout moved with the packet version and so did the id: before
// PACKETVER 20190807 each entry carried the skill's name and the packet was
// 0x010F; from that version the name is gone and it is 0x0B32. Ours is the
// later one, which is why the client needs a name table of its own — the
// server no longer sends one.
const (
	// ZC_SKILLINFO_LIST is `<len>.W` then a run of 15-byte entries.
	ZC_SKILLINFO_LIST uint16 = 0x0B32

	// skillEntryLen is one entry: id, inf, level, sp, range, upFlag, level2.
	skillEntryLen = 15
)

// Skill is one entry of the list.
type Skill struct {
	ID    uint16
	Level int

	// Inf is what the skill targets. Zero means it targets nothing, which is
	// how the server says passive — the window shows those as "Passive"
	// rather than as an SP cost they do not have.
	Inf int

	// SP is what casting it costs, and Range how far it reaches.
	SP    int
	Range int

	// Raisable is the server saying this skill can be leveled with a skill
	// point right now.
	Raisable bool
}

// DecodeSkillList reads the whole list. Returns nil when the packet is too
// short to hold its own header, and stops at whatever whole entries fit —
// a truncated tail is dropped rather than read past.
func DecodeSkillList(data []byte) []Skill {
	if len(data) < 4 {
		return nil
	}

	// The declared length wins over the buffer's: the framing hands us
	// exactly one packet, but a server that declares less than it sent should
	// not have the remainder read as skills.
	length := int(readU16(data, 2))
	if length > len(data) {
		length = len(data)
	}

	count := (length - 4) / skillEntryLen
	if count <= 0 {
		return nil
	}

	list := make([]Skill, 0, count)
	for i := 0; i < count; i++ {
		at := 4 + i*skillEntryLen

		list = append(list, Skill{
			ID:    readU16(data, at),
			Inf:   int(readU32(data, at+2)),
			Level: int(readU16(data, at+6)),
			SP:    int(readU16(data, at+8)),
			Range: int(readU16(data, at+10)),
			// The byte after the range; level2 follows it.
			Raisable: data[at+12] != 0,
		})
	}

	return list
}
