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
