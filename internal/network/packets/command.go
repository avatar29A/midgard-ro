package packets

// Packets behind the client's own `/` commands.
//
// These are the ones the original client sends for itself rather than passing
// through as chat. Ids and lengths are checked against clientlengths.go, which
// is generated from the server we build — several of these ids move with the
// packet version.

// CZ_REQ_USER_COUNT asks how many players are online, for /w and /who.
//
//	00c1  (no payload)
//
// Source: clif_packetdb.hpp:67, registered unguarded.
const CZ_REQ_USER_COUNT uint16 = 0x00C1

// ZC_USER_COUNT is the answer.
//
//	00c2 <count>.L
//
// Source: packets.hpp:35-39, PACKET_ZC_USER_COUNT.
const ZC_USER_COUNT uint16 = 0x00C2

// EncodeUserCount builds the /who request. It carries nothing but its id.
func EncodeUserCount() []byte {
	return []byte{byte(CZ_REQ_USER_COUNT), byte(CZ_REQ_USER_COUNT >> 8)}
}

// DecodeUserCount reads the player count. Reports false on short data.
//
// The count is signed on the wire — map_getusers() returns an int — but it
// cannot sensibly be negative, so a negative value is treated as unreadable
// rather than shown to the player as a negative number of people.
func DecodeUserCount(data []byte) (int, bool) {
	if len(data) < 6 {
		return 0, false
	}

	count := int32(readU32(data, 2))
	if count < 0 {
		return 0, false
	}

	return int(count), true
}
