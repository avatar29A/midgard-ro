package packets

// Being thrown off the server.
//
// The map server sends this and then closes the socket. Nothing else says so:
// a client that does not read it goes on drawing a world it is no longer
// connected to, and every click after that goes nowhere — which reads as the
// game having quietly stopped working rather than as having been disconnected.

// SC_NOTIFY_BAN is `<reason>.B`, three bytes.
const SC_NOTIFY_BAN uint16 = 0x0081

// The reasons worth telling apart, from clif_authfail_fd's own list. The rest
// are payment and region rules this server does not use.
const (
	KickedServerClosed  uint8 = 1
	KickedSomewhereElse uint8 = 2
	KickedTimedOut      uint8 = 3
	KickedServerFull    uint8 = 4
	KickedLastSession   uint8 = 8
	KickedTooManyIP     uint8 = 9
	KickedByGM          uint8 = 15
)

// DecodeKicked reads why. Reports false when the packet is too short to say.
func DecodeKicked(data []byte) (uint8, bool) {
	if len(data) < 3 {
		return 0, false
	}

	return data[2], true
}

// KickedReason says why in words.
func KickedReason(reason uint8) string {
	switch reason {
	case KickedServerClosed:
		return "The server closed."
	case KickedSomewhereElse:
		return "Someone else logged in with this account."
	case KickedTimedOut:
		return "Disconnected for being idle."
	case KickedServerFull:
		return "The server is full."
	case KickedLastSession:
		return "The server is still holding the last session for this account."
	case KickedTooManyIP:
		return "Too many connections from this address."
	case KickedByGM:
		return "Disconnected by a GM."
	}

	return "Disconnected by the server."
}
