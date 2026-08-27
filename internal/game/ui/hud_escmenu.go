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

// The window and its buttons, sized and colored from the archive's own.
//
// esc_01a.bmp through esc_08a.bmp are the original's buttons, 221x20, with
// their labels baked into the bitmap — in Korean, in this archive. So the
// look is taken from them rather than the images themselves: the border,
// face, hover wash and text colors below are sampled out of esc_01a and its
// hover twin esc_01b.
const (
	escBtnW float32 = 221
	escBtnH float32 = 20
	escBtnG float32 = 4
	escPad  float32 = 8

	// escLabelScale keeps the label inside a 20px button.
	escLabelScale float32 = 0.8

	escMenuW = escBtnW + 2*escPad
)

var (
	escBorder   = ui2d.Color{R: 0.694, G: 0.694, B: 0.694, A: 1}
	escFace     = ui2d.Color{R: 0.953, G: 0.953, B: 0.953, A: 1}
	escFaceHot  = ui2d.Color{R: 0.855, G: 0.894, B: 0.969, A: 1}
	escFaceOff  = ui2d.Color{R: 0.898, G: 0.898, B: 0.898, A: 1}
	escLabel    = ui2d.Color{R: 0.192, G: 0.192, B: 0.192, A: 1}
	escLabelOff = ui2d.Color{R: 0.6, G: 0.6, B: 0.6, A: 1}
)

// escMenuItems are the menu's buttons, in the order the original lists them.
//
// The three configuration entries are drawn because the menu has them and
// their absence would be the more confusing gap, but they are disabled:
// there is nothing behind them yet.
var escMenuItems = []struct {
	id       string
	label    string
	action   EscAction
	disabled bool
}{
	{"esc_charselect", "Character Select", EscCharSelect, false},
	{"esc_video", "Video Configuration", EscNone, true},
	{"esc_sound", "Sound Configuration", EscNone, true},
	{"esc_shortcut", "Shortcut Configuration", EscNone, true},
	{"esc_quit", "Exit to Windows", EscQuit, false},
	{"esc_cancel", "Return to game", EscNone, false},
}

// escMenuH is however tall the buttons make it.
var escMenuH = ui2d.FrameTitleH + escPad +
	float32(len(escMenuItems))*(escBtnH+escBtnG) - escBtnG + escPad

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

	if !b.ctx.BeginWindow("hud_esc_menu", x, y, escMenuW, escMenuH, "Game setting window") {
		return
	}

	btnX := x + escPad
	btnY := y + ui2d.FrameTitleH + escPad

	for _, item := range escMenuItems {
		box := ui2d.Rect{X: btnX, Y: btnY, W: escBtnW, H: escBtnH}
		btnY += escBtnH + escBtnG

		b.drawEscButton(box, item.label, item.disabled)

		if item.disabled {
			continue
		}

		if b.ctx.InvisibleButtonAt(item.id, box.X, box.Y, box.W, box.H) {
			// Return to game just closes; the other two are handed to the
			// caller, which has the connection to send them on.
			b.escAction = item.action
			b.escOpen = false
		}
	}

	b.ctx.EndWindow()
}

// drawEscButton draws one entry in the original's style: a pale face inside a
// thin grey border, washed blue under the pointer, with its label centered in
// near-black.
func (b *UI2DBackend) drawEscButton(box ui2d.Rect, label string, disabled bool) {
	r := b.ctx.Renderer()

	face, text := escFace, escLabel
	switch {
	case disabled:
		face, text = escFaceOff, escLabelOff
	case box.Contains(b.ctx.Input().MouseX, b.ctx.Input().MouseY):
		face = escFaceHot
	}

	r.DrawRect(box.X, box.Y, box.W, box.H, face)
	r.DrawRect(box.X, box.Y, box.W, 1, escBorder)
	r.DrawRect(box.X, box.Y+box.H-1, box.W, 1, escBorder)
	r.DrawRect(box.X, box.Y, 1, box.H, escBorder)
	r.DrawRect(box.X+box.W-1, box.Y, 1, box.H, escBorder)

	capW, capH := r.MeasureText(label, escLabelScale)
	r.DrawText(box.X+(box.W-capW)/2, box.Y+(box.H-capH)/2, label, escLabelScale, text)
}
