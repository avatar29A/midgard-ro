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
	SP_STR         uint16 = 13
	SP_AGI         uint16 = 14
	SP_VIT         uint16 = 15
	SP_INT         uint16 = 16
	SP_DEX         uint16 = 17
	SP_LUK         uint16 = 18
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

// The two packets behind the status window.
//
// ZC_STATUS is the whole thing at once, sent when the window is first filled
// in; ZC_COUPLESTATUS is one stat afterwards. They do not carry the same
// pair of numbers, which is the thing to be careful about:
//
//	ZC_STATUS's "standard" fields are pc_need_status_point — what it costs to
//	raise that stat by one, not a bonus.
//
//	ZC_COUPLESTATUS carries the bonus: the base and the equipment and buffs
//	added on top of it.
//
// A window built from ZC_STATUS alone can show the six values and their cost
// but cannot show a bonus, because the packet does not contain one.
const (
	// ZC_STATUS is `<point>.W` then six `<value>.B <cost>.B` pairs, then the
	// derived numbers. 44 bytes.
	ZC_STATUS uint16 = 0x00BD

	// ZC_COUPLESTATUS is `<status id>.L <base>.L <bonus>.L`. 14 bytes.
	ZC_COUPLESTATUS uint16 = 0x0141
)

// PrimaryStats is what ZC_STATUS says about the six.
type PrimaryStats struct {
	// StatusPoints is what there is left to spend.
	StatusPoints int

	// Values are the six stats, indexed by their offset from SP_STR, and
	// Costs what raising each by one would take.
	Values [PrimaryStatCount]int
	Costs  [PrimaryStatCount]int
}

// PrimaryStatCount is how many primary stats there are: STR through LUK.
const PrimaryStatCount = 6

// PrimaryStatIndex maps a status id to its place in PrimaryStats, reporting
// false for anything that is not one of the six.
func PrimaryStatIndex(varID uint16) (int, bool) {
	if varID < SP_STR || varID > SP_LUK {
		return 0, false
	}

	return int(varID - SP_STR), true
}

// DecodeStatus reads the whole status window. Returns nil when the packet is
// too short to hold it.
func DecodeStatus(data []byte) *PrimaryStats {
	// Header, the point total, and the six pairs.
	const need = 2 + 2 + 2*PrimaryStatCount

	if len(data) < need {
		return nil
	}

	stats := &PrimaryStats{StatusPoints: int(readU16(data, 2))}
	for i := 0; i < PrimaryStatCount; i++ {
		stats.Values[i] = int(data[4+i*2])
		stats.Costs[i] = int(data[5+i*2])
	}

	return stats
}

// CoupleStatus is one stat and what is added to it.
type CoupleStatus struct {
	VarID uint16
	Base  int
	Bonus int
}

// DecodeCoupleStatus reads a single stat's base and bonus. Returns nil when
// the packet is too short.
func DecodeCoupleStatus(data []byte) *CoupleStatus {
	if len(data) < 14 {
		return nil
	}

	// The status id is four bytes wide here, unlike the two it takes in
	// ZC_PAR_CHANGE.
	return &CoupleStatus{
		VarID: uint16(readU32(data, 2)),
		Base:  int(int32(readU32(data, 6))),
		Bonus: int(int32(readU32(data, 10))),
	}
}
