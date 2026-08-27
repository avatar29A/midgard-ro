package ui

import (
	"testing"

	"github.com/Faultbox/midgard-ro/internal/game/entity"
)

// TestBarFractionClamps: a bar's width comes straight from this, so anything
// outside 0..1 draws past the border or backwards.
func TestBarFractionClamps(t *testing.T) {
	tests := []struct {
		name       string
		value, max int
		want       float32
	}{
		{"full", 40, 40, 1},
		{"half", 20, 40, 0.5},
		{"empty", 0, 40, 0},
		{"negative health", -5, 40, 0},
		{"over maximum", 60, 40, 1},
		{"unknown maximum", 40, 0, 0},
		{"negative maximum", 40, -1, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := barFraction(tt.value, tt.max); got != tt.want {
				t.Errorf("barFraction(%d, %d) = %v, want %v", tt.value, tt.max, got, tt.want)
			}
		})
	}
}

// TestUnknownMaxReadsAsEmpty is the case worth stating on its own: a unit
// whose maximum we have not been told is not a unit at full health, and
// dividing by it would panic besides.
func TestUnknownMaxReadsAsEmpty(t *testing.T) {
	if got := barFraction(100, 0); got != 0 {
		t.Errorf("a unit with no known maximum read as %v full", got)
	}
}

// TestHPColorSaysWhatItIs: the color is how you tell a monster's bar from a
// person's at a glance, and how you notice one is in trouble.
func TestHPColorSaysWhatItIs(t *testing.T) {
	healthyPlayer := hpColor(entity.TypePlayer, 1)
	healthyMob := hpColor(entity.TypeMonster, 1)

	if healthyPlayer == healthyMob {
		t.Error("a player and a monster draw the same color at full health")
	}

	hurtPlayer := hpColor(entity.TypePlayer, 0.1)
	if hurtPlayer == healthyPlayer {
		t.Error("a player below the low-HP threshold looks the same as a healthy one")
	}

	hurtMob := hpColor(entity.TypeMonster, 0.1)
	if hurtMob == healthyMob {
		t.Error("a monster below the low-HP threshold looks the same as a healthy one")
	}
}

// TestLowHPThresholdIsExclusive pins where the color changes, so a bar does
// not flicker between two colors sitting exactly on the boundary.
func TestLowHPThresholdIsExclusive(t *testing.T) {
	atThreshold := hpColor(entity.TypePlayer, entityBarLowHP)
	healthy := hpColor(entity.TypePlayer, 1)

	if atThreshold != healthy {
		t.Errorf("exactly at the threshold the bar already reads as low; "+
			"the comparison should be strictly below %v", entityBarLowHP)
	}

	if hpColor(entity.TypePlayer, entityBarLowHP-0.01) == healthy {
		t.Error("just below the threshold the bar still reads as healthy")
	}
}

// TestBarHeightGrowsWithSP: an entity with no SP shows one bar, and the
// original sizes the whole thing accordingly rather than leaving a gap.
func TestBarHeightGrowsWithSP(t *testing.T) {
	if entityBarHSP <= entityBarH {
		t.Errorf("the bar with SP (%v) is not taller than without (%v)", entityBarHSP, entityBarH)
	}
	if entityBarHSP-entityBarH != 4 {
		t.Errorf("SP adds %v to the height, want 4 as the original does",
			entityBarHSP-entityBarH)
	}
}
