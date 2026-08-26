package game

import (
	"github.com/Faultbox/midgard-ro/internal/network/packets"
)

// playerStats is the subset of a character's numbers the HUD shows: the two
// gauges and the two levels. It exists so the several widths CharInfo uses
// (HP is 32-bit, SP 16, the levels 16 and 32) are converted in one place
// rather than at every read site.
type playerStats struct {
	HP, MaxHP int
	SP, MaxSP int
	Level     int
	JobLevel  int
}

// statsFromChar reads the stats character select reported. They are the
// player's values until the server pushes a change, and afterwards they are
// what those pushes are checked against: if the two disagree, the CharInfo
// struct is being read at the wrong offsets rather than the server being odd.
//
// A nil character — no session yet — gives zeroes, which the gauges draw as
// empty rather than dividing by zero.
func statsFromChar(char *packets.CharInfo) playerStats {
	if char == nil {
		return playerStats{}
	}

	return playerStats{
		HP:       int(char.HP),
		MaxHP:    int(char.MaxHP),
		SP:       int(char.SP),
		MaxSP:    int(char.MaxSP),
		Level:    int(char.BaseLevel),
		JobLevel: int(char.JobLevel),
	}
}
