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

// The three `/` commands the original client sends as packets of their own.
//
// All three are GM commands, and the server turns each one straight back into
// the atcommand it stands for: 0x0140 becomes @mapmove (clif_parse_MapMove),
// 0x0099 becomes @kami (clif_parse_Broadcast), 0x019C becomes @lkami
// (clif_parse_LocalBroadcast). Sending the atcommand text instead would work
// for a GM and be *broadcast to the map* for anyone else, because a refused
// command falls through to ordinary chat. As packets they are refused in
// silence, which is what the original does and the reason these exist.
const (
	// CZ_MOVETO_MAP is /mm — warp to a map and cell.
	//
	//	0140 <map>.16B <x>.W <y>.W
	//
	// 22 bytes fixed. The name field is MAP_NAME_LENGTH_EXT (16) and the
	// server reads it with safestrncpy, so it must hold a terminator.
	CZ_MOVETO_MAP uint16 = 0x0140

	// CZ_BROADCAST is /b — announce to the whole server.
	//
	//	0099 <len>.W <message>.?B
	CZ_BROADCAST uint16 = 0x0099

	// CZ_LOCALBROADCAST is /lb — announce on this map only.
	//
	//	019c <len>.W <message>.?B
	CZ_LOCALBROADCAST uint16 = 0x019C
)

// mapNameLen is MAP_NAME_LENGTH_EXT from the server (mmo.hpp), the width of
// the map field in CZ_MOVETO_MAP.
const mapNameLen = 16

// EncodeMapMove builds the /mm request.
//
// Returns nil when the name is missing or too long to be terminated inside
// the field, since the server would read a truncated name and warp somewhere
// else — a silent wrong answer being worse than no packet at all.
func EncodeMapMove(mapName string, x, y uint16) []byte {
	if mapName == "" || len(mapName) >= mapNameLen {
		return nil
	}

	const length = 4 + mapNameLen + 2

	pkt := make([]byte, length)
	writeU16(pkt, 0, CZ_MOVETO_MAP)
	copy(pkt[2:2+mapNameLen], mapName)
	writeU16(pkt, 2+mapNameLen, x)
	writeU16(pkt, 4+mapNameLen, y)

	return pkt
}

// EncodeBroadcast builds /b, and EncodeLocalBroadcast builds /lb. They differ
// only in the id.
func EncodeBroadcast(message string) []byte {
	return encodeBroadcast(CZ_BROADCAST, message)
}

// EncodeLocalBroadcast builds the /lb request.
func EncodeLocalBroadcast(message string) []byte {
	return encodeBroadcast(CZ_LOCALBROADCAST, message)
}

// encodeBroadcast builds either broadcast packet.
//
// The declared size is the header plus the message and no terminator: the
// server copies `packetSize - sizeof(header) + 1` bytes with safestrncpy,
// which writes its own NUL, so an exact fit is what its arithmetic expects.
//
// Returns nil for an empty message. The server would accept it and announce
// nothing, which is a packet spent to no effect.
func encodeBroadcast(id uint16, message string) []byte {
	if message == "" {
		return nil
	}

	length := 4 + len(message)

	pkt := make([]byte, length)
	writeU16(pkt, 0, id)
	writeU16(pkt, 2, uint16(length))
	copy(pkt[4:], message)

	return pkt
}
