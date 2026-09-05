package ui

import "github.com/Faultbox/midgard-ro/internal/engine/ui2d"

// The original's "message" window: a line of text, and a row of choices along
// the bottom right.
//
// Not the shop's own, which is why it is here rather than beside it. The same
// window asks whether to buy or to sell, whether to accept a trade from
// another player, and whether to go through with one — the text and the
// buttons are all that change, so those are what it is handed.

const (
	msgBoxW float32 = 330
	msgBoxH float32 = 140

	msgBoxPad float32 = 14

	// The row along the bottom. The buttons are right-aligned, as the
	// original has them, so the last one sits under the corner rather than
	// the first sitting under the text.
	msgBoxBtnW   float32 = 68
	msgBoxBtnH   float32 = 24
	msgBoxBtnGap float32 = 6

	msgBoxFooterH float32 = msgBoxBtnH + 2*msgBoxPad

	msgBoxTextScale float32 = 0.85
)

var (
	msgBoxText = ui2d.Color{R: 0.11, G: 0.11, B: 0.11, A: 1}
	msgBoxRule = ui2d.Color{R: 0.78, G: 0.78, B: 0.78, A: 1}

	msgBoxBtnFace   = ui2d.Color{R: 0.98, G: 0.98, B: 0.98, A: 1}
	msgBoxBtnHot    = ui2d.Color{R: 0.70, G: 0.79, B: 0.93, A: 1}
	msgBoxBtnBorder = ui2d.Color{R: 0.62, G: 0.62, B: 0.62, A: 1}
	msgBoxBtnLabel  = ui2d.Color{R: 0.16, G: 0.16, B: 0.16, A: 1}
)

// msgButton is one of the choices along the bottom.
type msgButton struct {
	// id is the widget's, which has to be steady across frames for a press to
	// be followed to its release.
	id string

	label string
}

// drawMessageBox draws one and reports which button was pressed, by id, and
// whether the window itself was closed.
//
// The window is one the player does not open — something asks, and the asking
// is what puts it there — so it goes through hostedWindow like the rest of
// those.
func (b *UI2DBackend) drawMessageBox(
	win *hostedWindow, screenW, screenH float32,
	title, text string, buttons []msgButton,
) (pressed string, closed bool) {
	openX := (screenW - msgBoxW) / 2
	openY := (screenH - msgBoxH) / 2

	draw, shut := win.begin(b.ctx, true, openX, openY, msgBoxW, msgBoxH,
		title, ui2d.WindowOptions{Closable: true})
	if !draw {
		return "", shut
	}

	x, y := win.rect(b.ctx, openX, openY)

	r := b.ctx.Renderer()

	r.DrawText(x+msgBoxPad, y+ui2d.FrameTitleH+msgBoxPad, text, msgBoxTextScale, msgBoxText)

	// The rule above the buttons, which is what separates what is being asked
	// from the answers to it.
	footerY := y + msgBoxH - msgBoxFooterH
	r.DrawRect(x+1, footerY, msgBoxW-2, 1, msgBoxRule)

	for i, button := range buttons {
		box := msgBoxButtonAt(x, footerY+msgBoxPad, len(buttons), i)

		b.drawRoundedButton(box, button.label)

		if b.ctx.InvisibleButtonAt(button.id, box.X, box.Y, box.W, box.H) {
			pressed = button.id
		}
	}

	b.ctx.EndWindow()

	return pressed, false
}

// msgBoxButtonAt is where one button of a row sits.
//
// Laid out from the right, so the last one named is the one in the corner and
// the row grows leftward — which is where the original puts cancel, and it
// stays there however many choices there are.
func msgBoxButtonAt(x, y float32, count, i int) ui2d.Rect {
	fromRight := count - 1 - i

	left := x + msgBoxW - msgBoxPad -
		float32(fromRight+1)*msgBoxBtnW - float32(fromRight)*msgBoxBtnGap

	return ui2d.Rect{X: left, Y: y, W: msgBoxBtnW, H: msgBoxBtnH}
}

// drawRoundedButton draws one of the original's buttons: a pale face in a thin
// border with its corners taken off, washed blue under the pointer.
//
// The corners are shaved rather than drawn round. At this size the difference
// is a pixel at each end, and a pixel is what the original's own art is doing
// too — its buttons are twenty-four tall and the rounding is one step.
func (b *UI2DBackend) drawRoundedButton(box ui2d.Rect, label string) {
	r := b.ctx.Renderer()

	face := msgBoxBtnFace
	if box.Contains(b.ctx.Input().MouseX, b.ctx.Input().MouseY) {
		face = msgBoxBtnHot
	}

	// The body, then the two shorter rows that take the corners off.
	r.DrawRect(box.X, box.Y+2, box.W, box.H-4, face)
	r.DrawRect(box.X+1, box.Y+1, box.W-2, 1, face)
	r.DrawRect(box.X+1, box.Y+box.H-2, box.W-2, 1, face)
	r.DrawRect(box.X+2, box.Y, box.W-4, 1, face)
	r.DrawRect(box.X+2, box.Y+box.H-1, box.W-4, 1, face)

	// The border, the same shape one pixel out.
	r.DrawRect(box.X+2, box.Y, box.W-4, 1, msgBoxBtnBorder)
	r.DrawRect(box.X+2, box.Y+box.H-1, box.W-4, 1, msgBoxBtnBorder)
	r.DrawRect(box.X, box.Y+2, 1, box.H-4, msgBoxBtnBorder)
	r.DrawRect(box.X+box.W-1, box.Y+2, 1, box.H-4, msgBoxBtnBorder)
	r.DrawRect(box.X+1, box.Y+1, 1, 1, msgBoxBtnBorder)
	r.DrawRect(box.X+box.W-2, box.Y+1, 1, 1, msgBoxBtnBorder)
	r.DrawRect(box.X+1, box.Y+box.H-2, 1, 1, msgBoxBtnBorder)
	r.DrawRect(box.X+box.W-2, box.Y+box.H-2, 1, 1, msgBoxBtnBorder)

	textW, _ := r.MeasureText(label, msgBoxTextScale)
	ascent := r.FontAscent(msgBoxTextScale)

	r.DrawText(box.X+(box.W-textW)/2, box.Y+(box.H-ascent)/2, label,
		msgBoxTextScale, msgBoxBtnLabel)
}
