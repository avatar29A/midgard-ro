package states

import (
	"testing"

	"github.com/Faultbox/midgard-ro/internal/network/packets"
)

func TestPlayerStatsFromChar(t *testing.T) {
	tests := []struct {
		name string
		char *packets.CharInfo
		want PlayerStats
	}{
		{
			name: "no session yet",
			char: nil,
			want: PlayerStats{},
		},
		{
			name: "a fresh novice",
			char: &packets.CharInfo{
				HP: 40, MaxHP: 40,
				SP: 11, MaxSP: 11,
				BaseLevel: 1, JobLevel: 1,
				Class: 0, Zeny: 500,
			},
			want: PlayerStats{
				HP: 40, MaxHP: 40, SP: 11, MaxSP: 11,
				BaseLevel: 1, JobLevel: 1, Zeny: 500,
			},
		},
		{
			name: "past what a 16-bit HP would hold",
			char: &packets.CharInfo{
				HP: 70000, MaxHP: 120000,
				SP: 500, MaxSP: 900,
				BaseLevel: 99, JobLevel: 70, Class: 4,
			},
			want: PlayerStats{
				HP: 70000, MaxHP: 120000, SP: 500, MaxSP: 900,
				BaseLevel: 99, JobLevel: 70, Class: 4,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := PlayerStatsFromChar(tt.char); got != tt.want {
				t.Errorf("PlayerStatsFromChar() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestPlayerStatsApply(t *testing.T) {
	tests := []struct {
		name  string
		varID uint16
		value int64
		want  PlayerStats
	}{
		{"HP", packets.SP_HP, 37, PlayerStats{HP: 37}},
		{"max HP", packets.SP_MAXHP, 45, PlayerStats{MaxHP: 45}},
		{"SP", packets.SP_SP, 8, PlayerStats{SP: 8}},
		{"max SP", packets.SP_MAXSP, 12, PlayerStats{MaxSP: 12}},
		{"base level", packets.SP_BASELEVEL, 12, PlayerStats{BaseLevel: 12}},
		{"job level", packets.SP_JOBLEVEL, 5, PlayerStats{JobLevel: 5}},
		{"base exp", packets.SP_BASEEXP, 1 << 33, PlayerStats{BaseExp: 1 << 33}},
		{"next base exp", packets.SP_NEXTBASEEXP, 9000, PlayerStats{NextBaseExp: 9000}},
		{"job exp", packets.SP_JOBEXP, 40, PlayerStats{JobExp: 40}},
		{"next job exp", packets.SP_NEXTJOBEXP, 660, PlayerStats{NextJobExp: 660}},
		{"Zeny", packets.SP_ZENY, 100000, PlayerStats{Zeny: 100000}},
		// Weight arrives in tenths of a unit.
		{"weight", packets.SP_WEIGHT, 500, PlayerStats{Weight: 50}},
		{"max weight", packets.SP_MAXWEIGHT, 21500, PlayerStats{MaxWeight: 2150}},
		{"a weight that does not divide evenly rounds down", packets.SP_WEIGHT, 507, PlayerStats{Weight: 50}},
		{"class", packets.SP_CLASS, 4, PlayerStats{Class: 4}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got PlayerStats
			if !got.Apply(tt.varID, tt.value) {
				t.Fatalf("Apply(%d) reported the id as untracked", tt.varID)
			}
			if got != tt.want {
				t.Errorf("after Apply(%d, %d) = %+v, want %+v", tt.varID, tt.value, got, tt.want)
			}
		})
	}
}

// TestPlayerStatsApplyUntracked pins that an id we do not show is reported as
// such and changes nothing — the server sends every combat stat down the same
// packet, and silently absorbing them would hide a parameter we do want.
func TestPlayerStatsApplyUntracked(t *testing.T) {
	const spATK1 = 41

	stats := PlayerStats{HP: 40, MaxHP: 40}
	before := stats

	if stats.Apply(spATK1, 11) {
		t.Error("Apply() reported an untracked id as tracked")
	}
	if stats != before {
		t.Errorf("stats changed to %+v, want %+v", stats, before)
	}
}
