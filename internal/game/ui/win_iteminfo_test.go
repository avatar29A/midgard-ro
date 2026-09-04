package ui

import (
	"strings"
	"testing"

	"github.com/Faultbox/midgard-ro/internal/game/items"
)

// TestPrettyWordSplitsTheDatabasesNames: rAthena's names are written for a
// config file — Right_Hand, SuperNovice, BardDancer — and printed as they come
// they put an underscore in the middle of a window and run two words together.
func TestPrettyWordSplitsTheDatabasesNames(t *testing.T) {
	for _, tc := range []struct {
		raw  string
		want string
	}{
		{"Right_Hand", "Right Hand"},
		{"SuperNovice", "Super Novice"},
		{"BardDancer", "Bard Dancer"},
		{"1hSword", "1h Sword"},
		{"Knight", "Knight"},
		{"", ""},
	} {
		if got := prettyWord(tc.raw); got != tc.want {
			t.Errorf("prettyWord(%q) = %q, want %q", tc.raw, got, tc.want)
		}
	}
}

// TestItemWeightWordsCountsInWholes: the database stores tenths and the
// character panel counts in wholes, so a Red Potion is 70 in the file and 7 on
// screen.
func TestItemWeightWordsCountsInWholes(t *testing.T) {
	for _, tc := range []struct {
		tenths int
		want   string
	}{
		{70, "7"},
		{500, "50"},
		{5, "0.5"},
		{125, "12.5"},
		{0, "0"},
	} {
		if got := itemWeightWords(tc.tenths); got != tc.want {
			t.Errorf("itemWeightWords(%d) = %q, want %q", tc.tenths, got, tc.want)
		}
	}
}

// TestWrapValueBreaksALongList: a job list is eleven names long and ran off
// the right edge of the window, which is where a restriction is least use to
// anybody.
//
// Measured as one unit a character, which is enough to check where the breaks
// fall without a renderer.
func TestWrapValueBreaksALongList(t *testing.T) {
	measure := func(text string) float32 { return float32(len(text)) }

	// Short enough for the space beside its label: one line, unbroken.
	if got := wrapValue("Right Hand", measure, 20, 30); len(got) != 1 || got[0] != "Right Hand" {
		t.Errorf("a short value came out as %q", got)
	}

	lines := wrapValue("Alchemist, Assassin, Blacksmith, Crusader, Knight", measure, 12, 22)
	if len(lines) < 3 {
		t.Fatalf("a long list wrapped to %d lines: %q", len(lines), lines)
	}

	// The first line fits beside the label, the rest fit the whole width, and
	// nothing is lost on the way.
	if float32(len(lines[0])) > 12 {
		t.Errorf("the first line is %q, wider than the space beside the label", lines[0])
	}
	for _, line := range lines[1:] {
		if float32(len(line)) > 22 {
			t.Errorf("a wrapped line is %q, wider than the window", line)
		}
	}

	if joined := strings.Join(lines, " "); joined != "Alchemist, Assassin, Blacksmith, Crusader, Knight" {
		t.Errorf("wrapping changed the value to %q", joined)
	}
}

// TestWrapValueKeepsAWordThatCannotFit: a single word wider than the window is
// put on a line of its own rather than dropped or looped on forever.
func TestWrapValueKeepsAWordThatCannotFit(t *testing.T) {
	measure := func(text string) float32 { return float32(len(text)) }

	lines := wrapValue("Supercalifragilistic", measure, 4, 6)
	if len(lines) != 1 || lines[0] != "Supercalifragilistic" {
		t.Errorf("an unbreakable word came out as %q", lines)
	}
}

// TestItemInfoLinesSayWhatMatters: the window is read top to bottom, so what
// an item is comes first and what it costs last. A figure an item does not
// have is left out rather than printed as a nought.
func TestItemInfoLinesSayWhatMatters(t *testing.T) {
	sword := itemInfoLines(1101, items.Info{Name: "Sword"})

	if len(sword) == 0 || sword[0].label != "Class:" {
		t.Fatalf("the first line is %+v, want the class", sword)
	}
	if sword[0].value != "1h Sword" {
		t.Errorf("a sword's class reads %q", sword[0].value)
	}

	labels := map[string]string{}
	for _, line := range sword {
		labels[line.label] = line.value
	}

	for _, want := range []string{"Attack:", "Slots:", "Weight:", "Worn:", "Required Level:", "Jobs:"} {
		if _, said := labels[want]; !said {
			t.Errorf("a sword says nothing about %q: %+v", want, sword)
		}
	}

	// A potion has no attack, no slots and nowhere to be worn, and says so by
	// leaving them out.
	potion := itemInfoLines(501, items.Info{Name: "Red Potion"})
	for _, line := range potion {
		switch line.label {
		case "Attack:", "Defense:", "Slots:", "Worn:", "Jobs:":
			t.Errorf("a potion has a %s line: %q", line.label, line.value)
		}
	}
}

// TestItemInfoLinesForSomethingUnknown: an id past the database still gets a
// window rather than an empty one.
func TestItemInfoLinesForSomethingUnknown(t *testing.T) {
	lines := itemInfoLines(0, items.Info{Category: items.CategoryEtc})

	if len(lines) != 1 || lines[0].label != "Class:" {
		t.Errorf("an unknown item came out as %+v", lines)
	}
}
