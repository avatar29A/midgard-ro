package packets

// Status parameter packets. The server pushes one whenever a value it keeps
// for us changes; there is no request that asks for them, so a client that
// does not handle these never learns its own HP.
//
// Three packets carry the same idea at different widths, and which one a
// given parameter uses depends on PACKETVER. At 20211103 — what the server in
// docker/rathena is built for — experience moved to the 64-bit form, so a
// client handling only 0x00B0 and 0x00B1 sees no experience at all.
//
// Sources: rAthena src/map/clif.cpp:3547 (clif_par_change), :3558
// (clif_longpar_change), :3621 (clif_longlongpar_change), and the dispatcher
// clif_updatestatus at :3635 that picks between them.
const (
	// ZC_PAR_CHANGE is `<varID>.W <value>.L`, 8 bytes. HP, SP, the levels,
	// weight and most other single values.
	ZC_PAR_CHANGE uint16 = 0x00B0
	// ZC_LONGPAR_CHANGE is the same shape, for values that would outgrow a
	// short. Zeny always; experience on packet versions before 20170830.
	ZC_LONGPAR_CHANGE uint16 = 0x00B1
	// ZC_LONGLONGPAR_CHANGE is `<varID>.W <value>.Q`, 12 bytes: experience at
	// our packet version, where the totals no longer fit in 32 bits.
	ZC_LONGLONGPAR_CHANGE uint16 = 0x0ACB
)

// Status parameter ids, from `enum _sp` in rAthena src/map/map.hpp:497. Only
// the ones the client acts on are named; the server sends a good many more.
const (
	SP_SPEED       uint16 = 0
	SP_BASEEXP     uint16 = 1
	SP_JOBEXP      uint16 = 2
	SP_HP          uint16 = 5
	SP_MAXHP       uint16 = 6
	SP_SP          uint16 = 7
	SP_MAXSP       uint16 = 8
	SP_STATUSPOINT uint16 = 9
	SP_BASELEVEL   uint16 = 11
	SP_SKILLPOINT  uint16 = 12
	SP_CLASS       uint16 = 19
	SP_ZENY        uint16 = 20
	SP_NEXTBASEEXP uint16 = 22
	SP_NEXTJOBEXP  uint16 = 23
	SP_WEIGHT      uint16 = 24
	SP_MAXWEIGHT   uint16 = 25
	SP_JOBLEVEL    uint16 = 55
)

// StatusChange is one parameter update, whichever of the three packets
// carried it. The width difference matters on the wire and nowhere after it,
// so the value is widened once here.
type StatusChange struct {
	VarID uint16
	Value int64
}

// DecodeStatusChange decodes a status packet, returning nil for anything that
// is not one or is too short to be one.
//
// The value is signed: the server sends these as int32/int64, and a parameter
// can legitimately be negative.
func DecodeStatusChange(data []byte) *StatusChange {
	if len(data) < 4 {
		return nil
	}

	switch readU16(data, 0) {
	case ZC_PAR_CHANGE, ZC_LONGPAR_CHANGE:
		if len(data) < 8 {
			return nil
		}

		return &StatusChange{VarID: readU16(data, 2), Value: int64(int32(readU32(data, 4)))}

	case ZC_LONGLONGPAR_CHANGE:
		if len(data) < 12 {
			return nil
		}

		return &StatusChange{VarID: readU16(data, 2), Value: readI64(data, 4)}
	}

	return nil
}

// readI64 reads a little-endian signed 64-bit value.
func readI64(data []byte, offset int) int64 {
	return int64(uint64(readU32(data, offset)) | uint64(readU32(data, offset+4))<<32)
}
