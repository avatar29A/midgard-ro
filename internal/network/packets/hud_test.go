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
