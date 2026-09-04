package ui

import "github.com/Faultbox/midgard-ro/internal/engine/ui2d"

// The window that comes up when the character dies.
//
// A character at nought hit points is not playing any more: the server will
// refuse every move, and the only things left to do are the three this offers.
// Without it the client sat on a corpse with nothing to press.
//
// It has no close button and no minimize button, because there is nowhere to
// go if it is put away — the only way out of being dead is one of these three.
// It can still be dragged, since the body it is over is worth looking at.

// DeadAction is what the player picked, read once and cleared.
type DeadAction int

const (
	// DeadNone is no choice made.
	DeadNone DeadAction = iota

	// DeadRespawn returns to the save point, which is whichever Kafra the
	// character last registered with.
	DeadRespawn

	// DeadCharSelect hands back to the character server.
	DeadCharSelect

	// DeadQuit leaves the game.
	DeadQuit
)

// deadWindowID is the frame's id, needed to read its position back.
const deadWindowID = "hud_dead_menu"

const deadMenuW = escBtnW + 2*escPad

// deadMenuItems are its buttons, in the order the original lists them.
var deadMenuItems = []struct {
	id     string
	label  string
	action DeadAction
}{
	{"dead_respawn", "Return to last save point", DeadRespawn},
	{"dead_charselect", "Character Select", DeadCharSelect},
	{"dead_quit", "Exit to Windows", DeadQuit},
}

// deadMenuH is however tall the buttons make it.
var deadMenuH = ui2d.FrameTitleH + escPad +
	float32(len(deadMenuItems))*(escBtnH+escBtnG) - escBtnG + escPad

// TakeDeadAction returns what was picked and clears it, the same way the ESC
// menu's is: the interface has no client to send it with.
func (b *UI2DBackend) TakeDeadAction() DeadAction {
	action := b.deadAction
	b.deadAction = DeadNone

	return action
}

// drawDeadMenu draws the window while the character is dead.
//
// Nothing is remembered about whether it is open. It is shown exactly while
// the server says the character has no hit points left, so a resurrection
// takes it away by itself — there is no flag to clear and no way for it to be
// left behind on a character who is up again.
func (b *UI2DBackend) drawDeadMenu(state InGameUIState, screenW, screenH float32) {
	if !state.PlayerDead {
		return
	}

	// Only where it opens: once dragged, the window keeps its own position.
	openX := (screenW - deadMenuW) / 2
	openY := (screenH - deadMenuH) / 2

	// A zero WindowOptions is a window with no system buttons: it cannot be
	// closed or minimized, which is the point of it.
	if !b.ctx.BeginWindowEx(deadWindowID, openX, openY, deadMenuW, deadMenuH,
		"", ui2d.WindowOptions{}) {
		return
	}

	// Read back after BeginWindowEx, not before. Before, the position is last
	// frame's, and the contents trail the frame by a frame as it is dragged.
	x, y := openX, openY
	if rect, ok := b.ctx.WindowRect(deadWindowID); ok {
		x, y = rect.X, rect.Y
	}

	// The window claims the pointer, so a click meant for a button does not
	// also try to walk a corpse. Only the window: the rest of the screen is
	// still worth looking around while dead.
	b.ctx.CaptureMouse(ui2d.Rect{X: x, Y: y, W: deadMenuW, H: deadMenuH})

	btnX := x + escPad
	btnY := y + ui2d.FrameTitleH + escPad

	for _, item := range deadMenuItems {
		box := ui2d.Rect{X: btnX, Y: btnY, W: escBtnW, H: escBtnH}
		btnY += escBtnH + escBtnG

		b.drawFlatButton(box, item.label, false)

		if b.ctx.InvisibleButtonAt(item.id, box.X, box.Y, box.W, box.H) {
			b.deadAction = item.action
		}
	}

	b.ctx.EndWindow()
}
