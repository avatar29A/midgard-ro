package ui

import "github.com/Faultbox/midgard-ro/internal/engine/ui2d"

// A window the player does not open.
//
// A shop is opened by a shopkeeper, a death window by dying, an item's
// information by asking about one. What they have in common is a trap that has
// caught this client three times over.
//
// The frame remembers being closed, and remembers it longer than the window
// lives: closed from its own X once, BeginWindow returns false for good after.
// For a window the player opens from a button that is harmless — the button
// clears the flag on the way in. For one that something else opens there is no
// button, so nothing cleared it: the window opened once and never again, and
// its false return was read as the player closing it, before a single frame of
// it had been drawn. The ESC menu went dark, the item window vanished behind
// the inventory, and a shop shut itself the moment a shopkeeper offered one.
//
// So the two halves are kept together here rather than written out again at
// every such window: clear the flags on the way in, and tell a false return
// from a real close apart.

// hostedWindow is one of them.
type hostedWindow struct {
	id string

	// shown is whether it was being drawn last frame, which is what makes the
	// clearing happen on the way in and only there. Cleared every frame, the
	// player closing the window would be undone before they saw it.
	shown bool
}

// newHostedWindow names one.
func newHostedWindow(id string) hostedWindow {
	return hostedWindow{id: id}
}

// begin starts the window when open says it should be there.
//
// It reports whether to draw — the caller must call EndWindow if so — and
// whether the player has just closed it, which is the caller's cue to put away
// whatever the window was showing.
//
// Minimized is not closed: the title bar is still drawn and the window is
// still the caller's to draw next frame, so only a real close is reported.
func (w *hostedWindow) begin(
	ctx *ui2d.Context, open bool,
	x, y, width, height float32, title string, opts ui2d.WindowOptions,
) (draw, closed bool) {
	if !open {
		w.shown = false

		return false, false
	}

	if !w.shown && ctx != nil {
		// The flags the window's own X and minimize set, which outlive it.
		ctx.OpenWindow(w.id)
	}

	w.shown = true

	if ctx == nil {
		return false, false
	}

	if ctx.BeginWindowEx(w.id, x, y, width, height, title, opts) {
		return true, false
	}

	return false, ctx.WindowClosed(w.id)
}

// hide says the window is not being drawn this frame, so the next time it is,
// its remembered flags are cleared on the way in.
//
// For a window with an alternative — a shop's shelf and the question that
// comes before it are one panel or the other — the one not showing has to be
// told, or it counts as never having left and clears nothing when it returns.
func (w *hostedWindow) hide() {
	w.shown = false
}

// rect is where the window actually is, which is read back after begin: read
// before, the position is last frame's and the contents trail the frame while
// it is dragged.
func (w *hostedWindow) rect(ctx *ui2d.Context, x, y float32) (float32, float32) {
	if ctx == nil {
		return x, y
	}

	if at, ok := ctx.WindowRect(w.id); ok {
		return at.X, at.Y
	}

	return x, y
}
