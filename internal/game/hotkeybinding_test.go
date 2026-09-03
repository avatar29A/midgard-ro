package game

import (
	"strconv"
	"testing"

	"github.com/Faultbox/midgard-ro/internal/game/ui"
)

// TestHotkeyLabelsMatchTheBindings: the quick panel says which key fires each
// cell, and the keys are bound here. A panel that says Ctrl+3 for a key that
// is really Alt+3 is worse than one that says nothing, and the two live in
// different packages — this is what holds them together.
func TestHotkeyLabelsMatchTheBindings(t *testing.T) {
	for _, binding := range hotkeyRows {
		for col := 0; col < 9; col++ {
			want := strconv.Itoa(col + 1)

			switch {
			case binding.fn:
				want = "F" + want
			case binding.ctrl:
				want = "Ctrl+" + want
			case binding.alt:
				want = "Alt+" + want
			}

			if got := ui.HotkeyKeyLabel(binding.row, col); got != want {
				t.Errorf("row %d column %d is bound to %q and named %q",
					binding.row, col, want, got)
			}
		}
	}
}
