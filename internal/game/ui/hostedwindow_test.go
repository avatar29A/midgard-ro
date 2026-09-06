package ui

import (
	"testing"

	"github.com/Faultbox/midgard-ro/internal/engine/ui2d"
)

// windowOpts is the ordinary chrome, spelled out so the tests read as one
// thing varying rather than four.
func windowOpts() ui2d.WindowOptions {
	return ui2d.WindowOptions{Closable: true}
}

// TestAHostedWindowClearsOnTheWayIn: the frame remembers being closed for
// longer than the window lives, so a window nobody presses a button to open —
// a shop, a death, an item being asked about — opened once and never again.
// This has caught the client three times, which is why it is one thing now.
func TestAHostedWindowClearsOnTheWayIn(t *testing.T) {
	w := newHostedWindow("hud_test")

	// Not shown: nothing to clear, and nothing to draw.
	if draw, closed := w.begin(nil, false, 0, 0, 10, 10, "", windowOpts()); draw || closed {
		t.Errorf("a window nobody opened reported draw=%v closed=%v", draw, closed)
	}
	if w.shown {
		t.Error("a window nobody opened counts as shown")
	}

	// Shown: it counts as shown from here, which is what makes the clearing
	// happen once rather than every frame.
	w.begin(nil, true, 0, 0, 10, 10, "", windowOpts())
	if !w.shown {
		t.Error("an open window does not count as shown")
	}

	// And away again, so the next opening clears afresh.
	w.begin(nil, false, 0, 0, 10, 10, "", windowOpts())
	if w.shown {
		t.Error("a window that closed still counts as shown")
	}
}

// TestHideMarksAWindowAway: two panels that are each other's alternative — a
// shop's shelf and the question before it — never both draw, so the one not
// showing has to be told, or it counts as never having left and clears
// nothing when it returns.
func TestHideMarksAWindowAway(t *testing.T) {
	w := newHostedWindow("hud_test")

	w.begin(nil, true, 0, 0, 10, 10, "", windowOpts())
	if !w.shown {
		t.Fatal("the window did not count as shown")
	}

	w.hide()

	if w.shown {
		t.Error("hide left the window counting as shown")
	}
}

// TestAHostedWindowSurvivesNoContext: the backends the window tests build have
// none, and a helper that dereferenced one would take them all down.
func TestAHostedWindowSurvivesNoContext(t *testing.T) {
	w := newHostedWindow("hud_test")

	draw, closed := w.begin(nil, true, 0, 0, 10, 10, "", windowOpts())
	if draw || closed {
		t.Errorf("a window with no context reported draw=%v closed=%v", draw, closed)
	}

	if x, y := w.rect(nil, 3, 4); x != 3 || y != 4 {
		t.Errorf("rect with no context gave %v,%v rather than what it was handed", x, y)
	}
}
