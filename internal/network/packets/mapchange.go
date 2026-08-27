package packets

import (
	"fmt"
	"strings"
)

// MapChange is ZC_NPCACK_MAPMOVE (0x0091): the server has moved us — to
// another map, or to another cell of this one — and expects the client to load
// the map and answer with CZ_NOTIFY_ACTORINIT once it has.
//
// rAthena also sends one on every login, for the map being entered
// (pc_authok, "fixes the login-without-aura glitch"), so a report naming the
// current map is normal and not a request to load it again.
type MapChange struct {
	// MapName is the name as the server sends it, extension included:
	// "prontera.gat".
	MapName string
	X, Y    int
}

const (
	mapChangeSize  = 22
	serverMoveSize = 156

	// mapNameLenExt is MAP_NAME_LENGTH_EXT: an 11-character name, a NUL and
	// the ".gat" the server insists on.
	mapNameLenExt = 16
)

// DecodeMapChange parses ZC_NPCACK_MAPMOVE. Returns nil on short data.
func DecodeMapChange(data []byte) *MapChange {
	if len(data) < mapChangeSize {
		return nil
	}

	return &MapChange{
		MapName: cString(data[2 : 2+mapNameLenExt]),
		X:       int(readU16(data, 18)),
		Y:       int(readU16(data, 20)),
	}
}

// BaseName is the map name without its extension — what the map's files are
// called in the archive.
func (m *MapChange) BaseName() string {
	return MapBaseName(m.MapName)
}

// MapBaseName strips the ".gat" the server appends to map names, so the two
// spellings compare equal.
func MapBaseName(name string) string {
	return strings.TrimSuffix(strings.ToLower(name), ".gat")
}

// ServerMove is ZC_NPCACK_SERVERMOVE (0x0AC7): the destination map lives on
// another map server, and the client is expected to reconnect there and enter
// the map. Before PACKETVER 20170315 the same packet was 0x0092, without the
// domain; at ours it is this one.
type ServerMove struct {
	MapName string
	X, Y    int
	IP      [4]byte // Network order, as sent
	Port    uint16
	Domain  string
}

// DecodeServerMove parses ZC_NPCACK_SERVERMOVE. Returns nil on short data.
func DecodeServerMove(data []byte) *ServerMove {
	if len(data) < serverMoveSize {
		return nil
	}

	s := &ServerMove{
		MapName: cString(data[2 : 2+mapNameLenExt]),
		X:       int(readU16(data, 18)),
		Y:       int(readU16(data, 20)),
		// rAthena writes the port in host order — "[!] LE byte order here
		// [!]" in clif_changemapserver — unlike the address beside it.
		Port:   readU16(data, 26),
		Domain: cString(data[28:serverMoveSize]),
	}
	copy(s.IP[:], data[22:26])
	return s
}

// Address is the map server as host:port.
func (s *ServerMove) Address() string {
	return fmt.Sprintf("%d.%d.%d.%d:%d", s.IP[0], s.IP[1], s.IP[2], s.IP[3], s.Port)
}

// cString returns the bytes up to the first NUL as a string.
func cString(b []byte) string {
	for i, c := range b {
		if c == 0 {
			return string(b[:i])
		}
	}
	return string(b)
}
