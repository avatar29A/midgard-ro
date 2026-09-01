package ui

import "testing"

func TestGaugeFillWidth(t *testing.T) {
	const track = 135

	tests := []struct {
		name             string
		current, maximum int
		want             float32
	}{
		{"empty", 0, 40, 0},
		{"half", 20, 40, 67},
		{"full", 40, 40, track},
		{"over full clamps to the track", 60, 40, track},
		{"the server has not said yet", 0, 0, 0},
		{"a maximum of zero with a value is still empty", 10, 0, 0},
		{"negative values draw nothing", -5, 40, 0},
		{"one hit point left is still a pixel", 1, 10000, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := gaugeFillWidth(tt.current, tt.maximum, track); got != tt.want {
				t.Errorf("gaugeFillWidth(%d, %d, %v) = %v, want %v",
					tt.current, tt.maximum, float32(track), got, tt.want)
			}
		})
	}
}

// TestGaugeFillWidthIsWholePixels pins the rounding: the art is
// nearest-filtered, so a fractional edge shimmers as the value changes.
func TestGaugeFillWidthIsWholePixels(t *testing.T) {
	for current := 0; current <= 40; current++ {
		got := gaugeFillWidth(current, 40, 135)
		if got != float32(int(got)) {
			t.Errorf("gaugeFillWidth(%d, 40, 135) = %v, want a whole number", current, got)
		}
	}
}

func TestPercentOf(t *testing.T) {
	tests := []struct {
		name             string
		current, maximum int
		want             int
	}{
		{"empty", 0, 40, 0},
		{"half", 20, 40, 50},
		{"full", 40, 40, 100},
		{"nearly full rounds down rather than reading 100", 39999, 40000, 99},
		{"the server has not said yet", 0, 0, 0},
		{"over full", 60, 40, 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := percentOf(tt.current, tt.maximum); got != tt.want {
				t.Errorf("percentOf(%d, %d) = %d, want %d", tt.current, tt.maximum, got, tt.want)
			}
		})
	}
}

func TestExpFillWidth(t *testing.T) {
	const track = 110

	tests := []struct {
		name          string
		current, next int64
		want          float32
	}{
		{"none earned", 0, 548, 0},
		{"half way", 274, 548, 55},
		{"level reached", 548, 548, track},
		{"max level sends no next total", 12345, 0, 0},
		{"a fraction of a percent still shows", 1, 100000, float32(track) / 100000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := expFillWidth(tt.current, tt.next, track); got != tt.want {
				t.Errorf("expFillWidth(%d, %d, %v) = %v, want %v",
					tt.current, tt.next, float32(track), got, tt.want)
			}
		})
	}
}

func TestWithThousands(t *testing.T) {
	tests := []struct {
		in   int64
		want string
	}{
		{0, "0"},
		{7, "7"},
		{999, "999"},
		{1000, "1,000"},
		{100000, "100,000"},
		{1234567, "1,234,567"},
		{-1234, "-1,234"},
	}

	for _, tt := range tests {
		if got := withThousands(tt.in); got != tt.want {
			t.Errorf("withThousands(%d) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// TestMenuWordWidthCountsTheGaps: the word is centered on this, so a width
// that forgot the gaps would sit the letters left of where the row puts its
// own labels.
func TestMenuWordWidthCountsTheGaps(t *testing.T) {
	// "equip" is 5+5+4+1+5 of letter and four gaps of two.
	if got := menuWordWidth(hudMenuButtonWords["equip"]); got != 28 {
		t.Errorf("menuWordWidth = %v, want 28", got)
	}

	if got := menuWordWidth(nil); got != 0 {
		t.Errorf("an empty word is %v wide", got)
	}
}

// TestEveryComposedWordHasItsLabels: a glyph naming a label nothing loads
// would leave a hole in the middle of a word.
func TestEveryComposedWordHasItsLabels(t *testing.T) {
	for name, word := range hudMenuButtonWords {
		if len(word) == 0 {
			t.Errorf("%q is composed but spells nothing", name)
		}

		for _, glyph := range word {
			if glyph.w <= 0 {
				t.Errorf("%q has a letter with no width: %+v", name, glyph)
			}
			if glyph.x < 0 || glyph.x+glyph.w > hudBtnW {
				t.Errorf("%q takes a letter from outside %s: %+v", name, glyph.from, glyph)
			}
		}
	}
}
