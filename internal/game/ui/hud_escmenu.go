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

	// EscSound opens the sound dialog, which the menu handles itself rather
	// than handing back: nothing outside the interface is involved.
	EscSound
)

// The window and its buttons, sized and colored from the archive's own.
//
// esc_01a.bmp through esc_08a.bmp are the original's buttons, 221x20, with
// their labels baked into the bitmap — in Korean, in this archive. So the
// look is taken from them rather than the images themselves: the border,
// face, hover wash and text colors below are sampled out of esc_01a and its
// hover twin esc_01b.
// escWindowID is the frame's id, needed to read its position back.
const escWindowID = "hud_esc_menu"

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
	// escBackdrop dims the game behind the menu, enough to say the menu has
	// the screen without hiding where you were standing.
	escBackdrop = ui2d.Color{R: 0, G: 0, B: 0, A: 0.45}

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
	{"esc_game", "Game Configuration", EscNone, true},
	{"esc_sound", "Sound Configuration", EscSound, false},
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

	// The backdrop, dimming the game behind the menu.
	//
	// Filled in the image pass rather than with DrawRect: solid quads paint
	// over every image in the frame, so a backdrop drawn as one would land on
	// top of the very window it is meant to sit behind — title bar and all.
	b.ctx.Renderer().DrawRect(0, 0, screenW, screenH, escBackdrop)

	// Only where it opens: once dragged, the window keeps its own position.
	openX := (screenW - escMenuW) / 2
	openY := (screenH - escMenuH) / 2

	if !b.ctx.BeginWindow(escWindowID, openX, openY, escMenuW, escMenuH, "Game setting window") {
		return
	}

	// Read back after BeginWindow, not before. Before, the position is last
	// frame's — the frame moves with the pointer and its contents trail a
	// frame behind, which is what made the window look like loose parts.
	x, y := openX, openY
	if rect, ok := b.ctx.WindowRect(escWindowID); ok {
		x, y = rect.X, rect.Y
	}

	// The whole screen claims the pointer while the menu is up: a click meant
	// for a button must not also walk the character, and neither should the
	// click that dismisses the menu.
	b.ctx.CaptureMouse(ui2d.Rect{X: 0, Y: 0, W: screenW, H: screenH})

	b.closeOnClickOutside(ui2d.Rect{X: x, Y: y, W: escMenuW, H: escMenuH})

	btnX := x + escPad
	btnY := y + ui2d.FrameTitleH + escPad

	for _, item := range escMenuItems {
		box := ui2d.Rect{X: btnX, Y: btnY, W: escBtnW, H: escBtnH}
		btnY += escBtnH + escBtnG

		b.drawEscButton(box, item.label, item.disabled)

		if item.disabled {
			continue
		}

		if !b.ctx.InvisibleButtonAt(item.id, box.X, box.Y, box.W, box.H) {
			continue
		}

		// Sound opens a dialog of its own and leaves the menu up. Return to
		// game just closes. The other two are handed to the caller, which has
		// the connection to send them on.
		if item.action == EscSound {
			b.soundOpen = true

			// Clear the closed flag its own X set, or it opens once and
			// never again.
			b.ctx.OpenWindow(soundWindowID)

			continue
		}

		b.escAction = item.action
		b.escOpen = false
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

// closeOnClickOutside shuts the menu when the pointer is pressed away from it.
//
// The sound dialog is opened from this menu and is its own window elsewhere on
// screen, so a press inside that one is not outside this one — closing the
// menu out from under the dialog it opened would be its own small bug.
func (b *UI2DBackend) closeOnClickOutside(menu ui2d.Rect) {
	if !b.ctx.Input().MouseLeftPressed {
		return
	}

	mouseX, mouseY := b.ctx.Input().MouseX, b.ctx.Input().MouseY
	if menu.Contains(mouseX, mouseY) {
		return
	}

	if b.soundOpen {
		if rect, ok := b.ctx.WindowRect(soundWindowID); ok && rect.Contains(mouseX, mouseY) {
			return
		}
	}

	b.escOpen = false
	b.soundOpen = false
}
