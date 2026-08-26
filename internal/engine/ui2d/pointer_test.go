package ui2d

import (
	"testing"
	"time"
)

// press puts the pointer at (x, y) with the left button freshly down.
func press(c *Context, x, y float32) {
	c.input.MouseX, c.input.MouseY = x, y
	c.input.MouseLeftPressed = true
}

func TestDoubleClickedIn(t *testing.T) {
	rect := Rect{X: 10, Y: 10, W: 100, H: 20}

	newContext := func() *Context {
		return &Context{input: &InputState{}}
	}

	t.Run("one press is not a double click", func(t *testing.T) {
		c := newContext()
		press(c, 20, 15)

		if c.DoubleClickedIn("panel", rect) {
			t.Error("a single press reported a double click")
		}
	})

	t.Run("two presses in the same place are", func(t *testing.T) {
		c := newContext()
		press(c, 20, 15)
		c.DoubleClickedIn("panel", rect)
		press(c, 20, 15)

		if !c.DoubleClickedIn("panel", rect) {
			t.Error("two presses did not report a double click")
		}
	})

	t.Run("a third press starts a new pair", func(t *testing.T) {
		c := newContext()
		press(c, 20, 15)
		c.DoubleClickedIn("panel", rect)
		press(c, 20, 15)
		c.DoubleClickedIn("panel", rect)
		press(c, 20, 15)

		if c.DoubleClickedIn("panel", rect) {
			t.Error("a third press fired again instead of starting a new pair")
		}
	})

	t.Run("a mouse that drifts a pixel or two still counts", func(t *testing.T) {
		c := newContext()
		press(c, 20, 15)
		c.DoubleClickedIn("panel", rect)
		press(c, 22, 17)

		if !c.DoubleClickedIn("panel", rect) {
			t.Error("a small drift between presses broke the double click")
		}
	})

	t.Run("two presses far apart are two clicks", func(t *testing.T) {
		c := newContext()
		press(c, 20, 15)
		c.DoubleClickedIn("panel", rect)
		press(c, 100, 15)

		if c.DoubleClickedIn("panel", rect) {
			t.Error("presses far apart reported a double click")
		}
	})

	t.Run("too slow is two clicks", func(t *testing.T) {
		c := newContext()
		press(c, 20, 15)
		c.DoubleClickedIn("panel", rect)
		c.lastClickAt = time.Now().Add(-2 * doubleClickWindow)
		press(c, 20, 15)

		if c.DoubleClickedIn("panel", rect) {
			t.Error("presses far apart in time reported a double click")
		}
	})

	t.Run("a different control does not inherit the first press", func(t *testing.T) {
		c := newContext()
		press(c, 20, 15)
		c.DoubleClickedIn("panel", rect)
		press(c, 20, 15)

		if c.DoubleClickedIn("other", rect) {
			t.Error("a press on one control completed a double click on another")
		}
	})

	t.Run("outside the rect is not a click at all", func(t *testing.T) {
		c := newContext()
		press(c, 500, 500)

		if c.DoubleClickedIn("panel", rect) {
			t.Error("a press outside the rect reported a double click")
		}
	})

	t.Run("the button has to be pressed this frame", func(t *testing.T) {
		c := newContext()
		c.input.MouseX, c.input.MouseY = 20, 15
		c.input.MouseLeftDown = true

		if c.DoubleClickedIn("panel", rect) {
			t.Error("holding the button reported a double click")
		}
	})
}

func TestCaptureMouse(t *testing.T) {
	c := &Context{input: &InputState{MouseX: 50, MouseY: 50}}

	if c.MouseCaptured() {
		t.Error("the pointer was captured before anything claimed it")
	}

	c.CaptureMouse(Rect{X: 200, Y: 200, W: 10, H: 10})

	if c.MouseCaptured() {
		t.Error("a rect the pointer is nowhere near captured it")
	}

	c.CaptureMouse(Rect{X: 40, Y: 40, W: 40, H: 40})

	if !c.MouseCaptured() {
		t.Error("the rect under the pointer did not capture it")
	}
}
