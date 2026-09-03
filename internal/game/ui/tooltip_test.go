package ui

import (
	"testing"

	"github.com/Faultbox/midgard-ro/internal/network/packets"
)

// TestHotkeyKeyLabels: the rows are bound in the game loop and named here,
// and a panel that says Ctrl+3 for a key that is really Alt+3 is worse than
// one that says nothing.
func TestHotkeyKeyLabels(t *testing.T) {
	for _, tc := range []struct {
		row, col int
		want     string
	}{
		{0, 0, "F1"},
		{0, 8, "F9"},
		{1, 0, "1"},
		{2, 2, "Ctrl+3"},
		{3, 8, "Alt+9"},
	} {
		if got := HotkeyKeyLabel(tc.row, tc.col); got != tc.want {
			t.Errorf("row %d col %d is called %q, want %q", tc.row, tc.col, got, tc.want)
		}
	}

	// A row nothing is bound to says nothing rather than inventing a key.
	if got := HotkeyKeyLabel(9, 0); got != "" {
		t.Errorf("an unbound row is called %q", got)
	}
	if got := HotkeyKeyLabel(0, hotkeySlots); got != "" {
		t.Errorf("a column past the end is called %q", got)
	}
}

// TestACellCastsAtItsOwnLevel: the same skill can sit on the bar several
// times over, and each key goes off at the level it was put there with.
func TestACellCastsAtItsOwnLevel(t *testing.T) {
	learned := packets.Skill{ID: 19, Level: 10, SP: 30, Inf: 1}

	for _, tc := range []struct {
		name string
		cell hotkeyCell
		want int
	}{
		{"as it was put there", hotkeyCell{level: 5}, 5},
		{"asked for nothing in particular", hotkeyCell{}, 10},
		{"at the level it is known", hotkeyCell{level: 10}, 10},

		// Put on the bar before it was unlearned, or at a level a reset took
		// away. Held down rather than refused: the server would turn it away.
		{"past what is known", hotkeyCell{level: 99}, 10},
		{"a level that makes no sense", hotkeyCell{level: -3}, 10},
	} {
		if got := hotkeyCastLevel(tc.cell, learned); got != tc.want {
			t.Errorf("%s: goes off at %d, want %d", tc.name, got, tc.want)
		}
	}
}
