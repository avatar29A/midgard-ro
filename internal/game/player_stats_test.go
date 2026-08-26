package game

import (
	"testing"

	"github.com/Faultbox/midgard-ro/internal/network/packets"
)

func TestStatsFromChar(t *testing.T) {
	tests := []struct {
		name string
		char *packets.CharInfo
		want playerStats
	}{
		{
			name: "no session yet",
			char: nil,
			want: playerStats{},
		},
		{
			name: "a fresh novice",
			char: &packets.CharInfo{
				HP: 40, MaxHP: 40,
				SP: 11, MaxSP: 11,
				BaseLevel: 1, JobLevel: 1,
			},
			want: playerStats{HP: 40, MaxHP: 40, SP: 11, MaxSP: 11, Level: 1, JobLevel: 1},
		},
		{
			name: "hurt, and past the range a 16-bit HP would hold",
			char: &packets.CharInfo{
				HP: 70000, MaxHP: 120000,
				SP: 500, MaxSP: 900,
				BaseLevel: 99, JobLevel: 70,
			},
			want: playerStats{HP: 70000, MaxHP: 120000, SP: 500, MaxSP: 900, Level: 99, JobLevel: 70},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := statsFromChar(tt.char)
			if got != tt.want {
				t.Errorf("statsFromChar() = %+v, want %+v", got, tt.want)
			}
		})
	}
}
