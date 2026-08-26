package states

import (
	"github.com/Faultbox/midgard-ro/internal/network/packets"
)

// PlayerStats is the player's own numbers — what the Basic Info panel shows.
//
// Character select gives the starting values and the server pushes every
// change after that, one parameter per packet. Nothing here is derived: a
// value we were never sent stays zero rather than being guessed at, which is
// why the gauges have to cope with a zero maximum.
type PlayerStats struct {
	HP, MaxHP int
	SP, MaxSP int

	BaseLevel, JobLevel int

	// Experience is 64-bit: past a certain level the totals do not fit in 32
	// and the server sends them in their own packet because of it.
	BaseExp, NextBaseExp int64
	JobExp, NextJobExp   int64

	Zeny int64

	// Weight is in units, not the tenths the server sends — see Apply.
	Weight, MaxWeight int

	// Class is the job id, which changes when the player advances.
	Class int
}

// PlayerStatsFromChar takes the starting values from the character list. A nil
// character — no session — gives zeroes.
func PlayerStatsFromChar(char *packets.CharInfo) PlayerStats {
	if char == nil {
		return PlayerStats{}
	}

	return PlayerStats{
		HP:        int(char.HP),
		MaxHP:     int(char.MaxHP),
		SP:        int(char.SP),
		MaxSP:     int(char.MaxSP),
		BaseLevel: int(char.BaseLevel),
		JobLevel:  int(char.JobLevel),
		BaseExp:   int64(char.BaseExp),
		JobExp:    int64(char.JobExp),
		Zeny:      int64(char.Zeny),
		Class:     int(char.Class),
	}
}

// Apply records one parameter update, reporting whether it was one we track.
//
// A false result is not an error: the server sends a great many parameters —
// every combat stat, every stat point cost — and this panel wants a dozen of
// them. The caller logs the unknown ids once each so a parameter we should be
// showing does not go missing in silence.
func (s *PlayerStats) Apply(varID uint16, value int64) bool {
	switch varID {
	case packets.SP_HP:
		s.HP = int(value)
	case packets.SP_MAXHP:
		s.MaxHP = int(value)
	case packets.SP_SP:
		s.SP = int(value)
	case packets.SP_MAXSP:
		s.MaxSP = int(value)
	case packets.SP_BASELEVEL:
		s.BaseLevel = int(value)
	case packets.SP_JOBLEVEL:
		s.JobLevel = int(value)
	case packets.SP_BASEEXP:
		s.BaseExp = value
	case packets.SP_NEXTBASEEXP:
		s.NextBaseExp = value
	case packets.SP_JOBEXP:
		s.JobExp = value
	case packets.SP_NEXTJOBEXP:
		s.NextJobExp = value
	case packets.SP_ZENY:
		s.Zeny = value
	// Weight travels in tenths of a unit — a max weight of 2030 arrives as
	// 20300. Converted here rather than at the point it is displayed, so
	// `Weight` means weight everywhere it is read.
	case packets.SP_WEIGHT:
		s.Weight = int(value) / 10
	case packets.SP_MAXWEIGHT:
		s.MaxWeight = int(value) / 10
	case packets.SP_CLASS:
		s.Class = int(value)
	default:
		return false
	}

	return true
}
