package ui

import "testing"

// TestMessageButtonsAreLaidOutFromTheRight: the last choice named is the one
// in the corner, which is where the original puts cancel — and it stays there
// however many choices there are.
func TestMessageButtonsAreLaidOutFromTheRight(t *testing.T) {
	const x, y = 100, 200

	for _, count := range []int{1, 2, 3, 4} {
		last := msgBoxButtonAt(x, y, count, count-1)

		if want := x + msgBoxW - msgBoxPad - msgBoxBtnW; last.X != want {
			t.Errorf("with %d buttons the last sits at %v, want %v", count, last.X, want)
		}
	}
}

// TestMessageButtonsDoNotOverlap: two that share pixels take each other's
// clicks, and the one drawn second swallows the press.
func TestMessageButtonsDoNotOverlap(t *testing.T) {
	const count = 3

	var before float32

	for i := 0; i < count; i++ {
		box := msgBoxButtonAt(0, 0, count, i)

		if i > 0 && box.X < before {
			t.Errorf("button %d starts at %v, inside the one before it ending at %v",
				i, box.X, before)
		}

		before = box.X + box.W
	}
}

// TestMessageButtonsStayInsideTheWindow: a row long enough to run off the left
// edge is a choice that cannot be pressed.
func TestMessageButtonsStayInsideTheWindow(t *testing.T) {
	const count = 3

	first := msgBoxButtonAt(0, 0, count, 0)
	last := msgBoxButtonAt(0, 0, count, count-1)

	if first.X < msgBoxPad {
		t.Errorf("the first of %d buttons starts at %v, off the window's edge", count, first.X)
	}
	if right := last.X + last.W; right > msgBoxW-msgBoxPad {
		t.Errorf("the last button ends at %v, past the window's %v", right, msgBoxW-msgBoxPad)
	}
}
