package packets

import (
	"encoding/binary"
	"testing"
)

func kickedPacket(reason uint8) []byte {
	pkt := make([]byte, 3)
	binary.LittleEndian.PutUint16(pkt, SC_NOTIFY_BAN)
	pkt[2] = reason

	return pkt
}

// TestDecodeKicked: one byte, and a packet too short to hold it says so rather
// than reporting a reason of nought — which is a real reason.
func TestDecodeKicked(t *testing.T) {
	if reason, ok := DecodeKicked(kickedPacket(KickedSomewhereElse)); !ok || reason != KickedSomewhereElse {
		t.Errorf("DecodeKicked = %d, %v", reason, ok)
	}

	if _, ok := DecodeKicked(kickedPacket(1)[:2]); ok {
		t.Error("a short packet decoded")
	}
}

// TestKickedReasonsAreToldApart: "disconnected" is not an explanation. Being
// idle, being logged in twice and the server closing are three different
// things to do something about.
func TestKickedReasonsAreToldApart(t *testing.T) {
	said := map[string]uint8{}

	for _, reason := range []uint8{
		KickedServerClosed, KickedSomewhereElse, KickedTimedOut,
		KickedServerFull, KickedLastSession, KickedTooManyIP, KickedByGM,
	} {
		words := KickedReason(reason)
		if words == "" {
			t.Errorf("reason %d says nothing", reason)
		}
		if before, twice := said[words]; twice {
			t.Errorf("reasons %d and %d both say %q", before, reason, words)
		}
		said[words] = reason
	}

	// One nobody has words for still says something.
	if KickedReason(200) == "" {
		t.Error("an unknown reason says nothing at all")
	}
}
