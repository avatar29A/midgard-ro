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

	// The six primary stats, indexed by their offset from SP_STR.
	//
	// Three numbers each, because the server sends them from two different
	// packets and they mean different things: what the stat is, what the
	// equipment and buffs add to it, and what raising it by one would cost.
	// ZC_STATUS carries the value and the cost; only ZC_COUPLESTATUS carries
	// the bonus.
	Primary      [packets.PrimaryStatCount]int
	PrimaryBonus [packets.PrimaryStatCount]int
	PrimaryCost  [packets.PrimaryStatCount]int

	// StatusPoints is what there is left to spend on them.
	StatusPoints int

	// The derived numbers down the right of the status window.
	Atk, AtkBonus    int
	MatkMin, MatkMax int
	Def, DefBonus    int
	Mdef, MdefBonus  int
	Flee, FleeBonus  int
	Hit              int
	Critical         int
	Aspd             int
}

// ApplyStatus takes the whole status window, as ZC_STATUS sends it.
func (s *PlayerStats) ApplyStatus(status *packets.PrimaryStats) {
	if status == nil {
		return
	}

	s.StatusPoints = status.StatusPoints
	s.Primary = status.Values
	s.PrimaryCost = status.Costs

	s.Atk, s.AtkBonus = status.Atk, status.AtkBonus
	s.MatkMin, s.MatkMax = status.MatkMin, status.MatkMax
	s.Def, s.DefBonus = status.Def, status.DefBonus
	s.Mdef, s.MdefBonus = status.Mdef, status.MdefBonus
	s.Flee, s.FleeBonus = status.Flee, status.FleeBonus
	s.Hit, s.Critical, s.Aspd = status.Hit, status.Critical, status.Aspd
}

// ApplyCoupleStatus takes one stat and its bonus, as ZC_COUPLESTATUS sends
// them. Reports false for a status id that is not one of the six.
func (s *PlayerStats) ApplyCoupleStatus(couple *packets.CoupleStatus) bool {
	if couple == nil {
		return false
	}

	index, ok := packets.PrimaryStatIndex(couple.VarID)
	if !ok {
		return false
	}

	s.Primary[index] = couple.Base
	s.PrimaryBonus[index] = couple.Bonus

	return true
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
	case packets.SP_STATUSPOINT:
		s.StatusPoints = int(value)

	// The derived numbers, which arrive one at a time as gear and buffs
	// change. ZC_STATUS carries them once; without these the right of the
	// window would show whatever was true at login for the rest of the
	// session.
	case packets.SP_ATK1:
		s.Atk = int(value)
	case packets.SP_ATK2:
		s.AtkBonus = int(value)
	case packets.SP_MATK1:
		s.MatkMax = int(value)
	case packets.SP_MATK2:
		s.MatkMin = int(value)
	case packets.SP_DEF1:
		s.Def = int(value)
	case packets.SP_DEF2:
		s.DefBonus = int(value)
	case packets.SP_MDEF1:
		s.Mdef = int(value)
	case packets.SP_MDEF2:
		s.MdefBonus = int(value)
	case packets.SP_HIT:
		s.Hit = int(value)
	case packets.SP_FLEE1:
		s.Flee = int(value)
	case packets.SP_FLEE2:
		s.FleeBonus = int(value)
	case packets.SP_CRITICAL:
		s.Critical = int(value)
	case packets.SP_ASPD:
		s.Aspd = packets.AspdFromAmotion(int(value))
	default:
		// The six primary stats arrive this way as well as in their own
		// packets, and this is the only one of the three that reports the
		// value on its own.
		index, ok := packets.PrimaryStatIndex(varID)
		if !ok {
			return false
		}

		s.Primary[index] = int(value)
	}

	return true
}
