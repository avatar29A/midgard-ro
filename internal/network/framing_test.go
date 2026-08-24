package network

import (
	"encoding/binary"
	"testing"

	"github.com/Faultbox/midgard-ro/internal/network/packets"
)

func TestPacketLengthFixed(t *testing.T) {
	c := &Client{}

	got, known := c.packetLength(0x0087, []byte{0x87, 0x00})
	if !known {
		t.Fatal("ZC_NOTIFY_PLAYERMOVE should be known")
	}
	if got != 12 {
		t.Errorf("length = %d, want 12", got)
	}
}

func TestPacketLengthVariable(t *testing.T) {
	c := &Client{}

	// A variable-length packet declaring 202 bytes.
	buf := []byte{0x6B, 0x00, 0xCA, 0x00}
	got, known := c.packetLength(0x006B, buf)
	if !known {
		t.Fatal("HC_ACCEPT_ENTER should be known")
	}
	if got != 202 {
		t.Errorf("length = %d, want 202", got)
	}
}

// TestPacketLengthVariableNeedsMoreData covers the case where the id has
// arrived but the two length bytes behind it have not. That must read as
// "wait", not as "unknown" — treating it as unknown would resynchronise past a
// perfectly good packet.
func TestPacketLengthVariableNeedsMoreData(t *testing.T) {
	c := &Client{}

	got, known := c.packetLength(0x006B, []byte{0x6B, 0x00})
	if !known {
		t.Error("a known id with an incomplete header should still report known")
	}
	if got != 0 {
		t.Errorf("length = %d, want 0 (meaning: need more bytes)", got)
	}
}

func TestPacketLengthUnknownID(t *testing.T) {
	c := &Client{}

	if _, known := c.packetLength(0xFFFF, []byte{0xFF, 0xFF, 0x40, 0x00}); known {
		t.Error("an unknown id must not report a length — guessing one from the " +
			"payload bytes is what desynchronised the stream")
	}
}

// TestResyncFindsOddOffset is the regression guard for the recovery bug: the
// old code skipped two bytes at a time, which preserves parity and so could
// never recover from an odd-byte misalignment.
func TestResyncFindsOddOffset(t *testing.T) {
	// One junk byte, then a real ZC_NOTIFY_PLAYERMOVE.
	buf := []byte{0xAA}
	buf = binary.LittleEndian.AppendUint16(buf, 0x0087)
	buf = append(buf, make([]byte, 10)...)

	if got := resyncOffset(buf); got != 1 {
		t.Errorf("resyncOffset = %d, want 1 (an odd offset must be reachable)", got)
	}
}

func TestResyncSkipsGarbageToKnownPacket(t *testing.T) {
	// Payload-looking bytes that are not valid ids, then a real packet.
	buf := []byte{0x00, 0x00, 0x00, 0x00, 0x00}
	buf = binary.LittleEndian.AppendUint16(buf, 0x007F) // ZC_NOTIFY_TIME
	buf = append(buf, make([]byte, 4)...)

	got := resyncOffset(buf)
	if got != 5 {
		t.Fatalf("resyncOffset = %d, want 5", got)
	}
	id := binary.LittleEndian.Uint16(buf[got : got+2])
	if !packets.IsKnown(id) {
		t.Errorf("resync landed on 0x%04X, which is not a known packet", id)
	}
}

// TestResyncGivesUpCleanly checks that a buffer with no recognisable packet
// reports the whole buffer as discardable rather than looping.
func TestResyncGivesUpCleanly(t *testing.T) {
	buf := []byte{0xAA, 0xBB, 0xCC, 0xDD}
	if got := resyncOffset(buf); got != len(buf) {
		t.Errorf("resyncOffset = %d, want %d (discard everything)", got, len(buf))
	}
}
