package packets

// Length returns the wire length of a server-to-client packet and whether the
// id is known at all.
//
// A length of VariableLength means the size is carried in a uint16 at offset 2.
// The caller must distinguish "unknown id" from "zero length" — RO frames
// packets by length alone, with no delimiters or checksums, so guessing at an
// unknown packet's size silently corrupts every packet after it on the
// connection rather than just the one.
func Length(packetID uint16) (length int, known bool) {
	length, known = mapPacketLengths[packetID]

	return length, known
}

// IsKnown reports whether the packet id appears in the generated table. Used
// when resynchronising a corrupted stream to tell a real packet boundary from
// two payload bytes that happen to look like one.
func IsKnown(packetID uint16) bool {
	_, ok := mapPacketLengths[packetID]

	return ok
}
