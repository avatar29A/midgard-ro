package ui

import "github.com/Faultbox/midgard-ro/internal/engine/ui2d"

// The menu Escape opens.
//
// Escape used to call os.Exit straight from the key handler, which skipped
// telling the server anything at all: the session was left for rAthena to
// time out. Both ways out of the game now go through it.

// EscAction is what the player picked, read once and cleared.
type EscAction int

const (
	// EscNone is no choice made.
	EscNone EscAction = iota

	// EscCharSelect is a return to character select.
	EscCharSelect

	// EscQuit is leaving the game.
	EscQuit
)

const (
	escMenuW float32 = 190
	escMenuH float32 = 132

	escBtnW float32 = 160
	escBtnH float32 = 26
	escBtnG float32 = 8
)

// escMenuItems are the menu's buttons, in the order the original lists them.
var escMenuItems = []struct {
	id     string
	label  string
	action EscAction
}{
	{"esc_charselect", "Character Select", EscCharSelect},
	{"esc_quit", "Exit to Windows", EscQuit},
	{"esc_cancel", "Return to Game", EscNone},
}

// ToggleEscMenu opens the menu, or closes it if it is already open.
func (b *UI2DBackend) ToggleEscMenu() {
	b.escOpen = !b.escOpen
}

// EscMenuOpen reports whether the menu is showing.
func (b *UI2DBackend) EscMenuOpen() bool {
	return b.escOpen
}

// TakeEscAction returns what was picked and clears it, the same way a typed
// chat line is handed back: the interface has no client to leave with.
func (b *UI2DBackend) TakeEscAction() EscAction {
	action := b.escAction
	b.escAction = EscNone

	return action
}

// drawEscMenu draws the menu in the middle of the screen.
func (b *UI2DBackend) drawEscMenu(screenW, screenH float32) {
	if !b.escOpen {
		return
	}

	x := (screenW - escMenuW) / 2
	y := (screenH - escMenuH) / 2

	// The whole menu claims the pointer: a click meant for a button must not
	// also walk the character to whatever is behind it.
	b.ctx.CaptureMouse(ui2d.Rect{X: x, Y: y, W: escMenuW, H: escMenuH})

	if !b.ctx.BeginWindow("hud_esc_menu", x, y, escMenuW, escMenuH, "Menu") {
		return
	}

	btnX := x + (escMenuW-escBtnW)/2
	btnY := y + ui2d.FrameTitleH + escBtnG

	for _, item := range escMenuItems {
		if b.ctx.ButtonAt(item.id, btnX, btnY, escBtnW, escBtnH, item.label) {
			// Cancel just closes; the other two are handed to the caller,
			// which has the connection to send them on.
			b.escAction = item.action
			b.escOpen = false
		}

		btnY += escBtnH + escBtnG
	}

	b.ctx.EndWindow()
}
