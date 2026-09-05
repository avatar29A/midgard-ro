package ui2d

import "testing"

// press, hold and lift drive the input the way a frame does, so a claim can be
// followed across frames without a renderer.
func (c *Context) press() {
	c.input.MouseX, c.input.MouseY = 10, 10
	c.input.MouseLeftDown = true
	c.input.MouseLeftPressed = true
	c.input.MouseLeftReleased = false
}

func (c *Context) hold() {
	c.input.MouseX, c.input.MouseY = 20, 20
	c.input.MouseLeftDown = true
	c.input.MouseLeftPressed = false
	c.input.MouseLeftReleased = false
}

// lift is the button coming up without the release ever being delivered,
// which is what a live window resize does on macOS.
func (c *Context) lift() {
	c.input.MouseLeftDown = false
	c.input.MouseLeftPressed = false
	c.input.MouseLeftReleased = false
}

// TestADragClaimDiesWithTheButton: every widget clears its own claim on
// release, and can only do that while it is being drawn. A claim made by
// something that then stopped being drawn was held for good, and everything
// asking whether the pointer was free got "no" for the rest of the session —
// which is a chat box that cannot be moved again.
func TestADragClaimDiesWithTheButton(t *testing.T) {
	c := &Context{input: &InputState{}}

	handle := Rect{X: 0, Y: 0, W: 100, H: 100}
	x, y := float32(0), float32(0)

	c.press()
	c.DragHandle("thing", handle, &x, &y)

	if c.activeWidget != "thing" {
		t.Fatalf("the press was not claimed: %q", c.activeWidget)
	}

	// The release never arrives — the run loop stopped while a window edge
	// was dragged — so the widget never clears it.
	c.lift()
	c.dropStaleClaim()

	if c.activeWidget != "" {
		t.Errorf("the claim outlived the button: %q", c.activeWidget)
	}
}

// TestTheReleaseFrameKeepsTheClaim: the frame the button comes up on is the
// frame a widget resolves its own click on. Cleared before that, a button is
// never pressed at all.
func TestTheReleaseFrameKeepsTheClaim(t *testing.T) {
	c := &Context{input: &InputState{}}

	handle := Rect{X: 0, Y: 0, W: 100, H: 100}
	x, y := float32(0), float32(0)

	c.press()
	c.DragHandle("thing", handle, &x, &y)

	c.input.MouseLeftDown = false
	c.input.MouseLeftPressed = false
	c.input.MouseLeftReleased = true

	c.dropStaleClaim()

	if c.activeWidget != "thing" {
		t.Errorf("the claim was dropped on the release frame: %q", c.activeWidget)
	}
}

// TestTypingSurvivesTheClick: focus is where the letters go and outlives every
// press. Held as the pointer's claim — which is how the two were kept, and the
// whole of why the chat box stopped moving — it either dies with the button
// and nothing can be typed, or it lives on and nothing can be dragged.
func TestTypingSurvivesTheClick(t *testing.T) {
	c := &Context{input: &InputState{}}

	// What a text field does when it takes focus, and the press it took.
	c.focusWidget = "chat_input"
	c.activeWidget = "chat_input"

	c.lift()
	c.dropStaleClaim()

	if c.focusWidget != "chat_input" {
		t.Errorf("focus was dropped when the button came up: %q", c.focusWidget)
	}
	if c.activeWidget != "" {
		t.Errorf("the pointer is still claimed by %q", c.activeWidget)
	}
}

// TestABoxIsDraggableWhileSomethingIsBeingTypedInto: the chat box carries the
// field that is typed into, so with focus and the pointer's claim as one thing
// the first click in the box left it unmovable for the rest of the session.
func TestABoxIsDraggableWhileSomethingIsBeingTypedInto(t *testing.T) {
	c := &Context{input: &InputState{}}

	c.focusWidget = "hud_chat_input"

	handle := Rect{X: 0, Y: 0, W: 100, H: 100}
	x, y := float32(0), float32(0)

	c.press()
	c.DragHandleFree("hud_chat_drag", handle, &x, &y)

	if c.activeWidget != "hud_chat_drag" {
		t.Errorf("the box refused to drag while a field had focus: %q", c.activeWidget)
	}
}

// TestADragIsStillHeldWhileTheButtonIsDown: the safety net must not interrupt
// a drag in progress.
func TestADragIsStillHeldWhileTheButtonIsDown(t *testing.T) {
	c := &Context{input: &InputState{}}

	handle := Rect{X: 0, Y: 0, W: 100, H: 100}
	x, y := float32(0), float32(0)

	c.press()
	c.DragHandle("thing", handle, &x, &y)

	c.hold()
	c.dropStaleClaim()

	if c.activeWidget != "thing" {
		t.Errorf("a drag in progress lost its claim: %q", c.activeWidget)
	}
}

// TestDragHandleFreeWaitsForAFreePointer: it refuses while anything else has
// the pointer, which is what made a stale claim so total — nothing using it
// could ever start a drag again.
func TestDragHandleFreeWaitsForAFreePointer(t *testing.T) {
	c := &Context{input: &InputState{}}

	c.activeWidget = "something_else"

	handle := Rect{X: 0, Y: 0, W: 100, H: 100}
	x, y := float32(0), float32(0)

	c.press()
	c.DragHandleFree("thing", handle, &x, &y)

	if c.activeWidget == "thing" {
		t.Error("a free drag took a pointer somebody else was holding")
	}

	// Once the stale claim is gone it takes the next press.
	c.lift()
	c.dropStaleClaim()

	c.press()
	c.DragHandleFree("thing", handle, &x, &y)

	if c.activeWidget != "thing" {
		t.Errorf("the drag did not take a free pointer: %q", c.activeWidget)
	}
}

// TestAFieldKeepsFocusAfterTheButtonComesUp: the whole point of keeping focus
// apart from the claim. Clicked, the field is typed into; a frame later the
// button is up, the claim is gone, and the field is still typed into.
//
// This is the case that broke: two fields were moved onto the focus and one
// was left asking the claim, and the one left behind lost what was being typed
// into it the moment the button came up.
func TestAFieldKeepsFocusAfterTheButtonComesUp(t *testing.T) {
	c := &Context{input: &InputState{}}

	c.press()

	if !c.takeFocus("chat_input", true) {
		t.Fatal("the field did not take the press")
	}
	if c.activeWidget != "chat_input" {
		t.Errorf("the press was not claimed: %q", c.activeWidget)
	}

	// The next frame, with the button up.
	c.lift()
	c.dropStaleClaim()

	if !c.Focused("chat_input") {
		t.Error("the field stopped being typed into when the button came up")
	}
	if c.activeWidget != "" {
		t.Errorf("the pointer is still claimed by %q", c.activeWidget)
	}

	// And it is still focused with nothing happening at all.
	if !c.takeFocus("chat_input", false) {
		t.Error("focus was lost on a frame with no press in it")
	}
}

// TestAPressElsewhereEndsTheTyping: focus ends when a press lands somewhere
// that is not it, which Begin settles once rather than every field noticing in
// turn.
func TestAPressElsewhereEndsTheTyping(t *testing.T) {
	c := &Context{input: &InputState{}}

	c.press()
	c.takeFocus("chat_input", true)

	c.lift()
	c.dropStaleClaim()

	// A press somewhere else: Begin drops the focus, and the field is drawn
	// afterwards without the pointer over it.
	c.press()
	c.focusWidget = ""

	if c.takeFocus("chat_input", false) {
		t.Error("the field kept focus through a press somewhere else")
	}
}
