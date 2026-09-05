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
	msgBoxBtnW float32 = 68
	// Taller than the art, which is twenty and too short for a word at this
	// size — the letters touched both edges. The face is a gradient, so it
	// stretches without a seam; only the two border rows soften, by a third
	// of a pixel.
	msgBoxBtnH   float32 = 26
	msgBoxBtnGap float32 = 6

	msgBoxFooterH float32 = msgBoxBtnH + 2*msgBoxPad

	msgBoxTextScale float32 = 0.85
)

var (
	msgBoxText = ui2d.Color{R: 0.11, G: 0.11, B: 0.11, A: 1}
	msgBoxRule = ui2d.Color{R: 0.78, G: 0.78, B: 0.78, A: 1}

	msgBoxBtnFace   = ui2d.Color{R: 0.98, G: 0.98, B: 0.98, A: 1}
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

		b.drawFlatButton(box, button.label, false)

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

// The archive's own button, and its two other states.
//
// There is one button bitmap in the whole interface folder and it has 닫기
// baked into the middle of it — the archive ships no blank one, and the only
// other labeled buttons are the numbers and the menu strip.
//
// So the ends are used as caps and the middle is stretched from a column that
// carries no lettering, which makes a button of the real art at any width with
// our own word on it. Measured off btn_ok.bmp: it is 42 by 20 and the 닫기
// runs from x=10 to x=32, so ten pixels at each end are clean and anything
// from four to nine is a plain slice of the face.
const (
	roButtonTex     = basicInterfacePath + `btn_ok.bmp`
	roButtonTexHot  = basicInterfacePath + `btn_ok_a.bmp`
	roButtonTexDown = basicInterfacePath + `btn_ok_b.bmp`

	roButtonArtW = float32(42)
	roButtonArtH = float32(20)

	// roButtonCap is how much of each end is taken as-is, and roButtonBlank
	// the column the middle is stretched from.
	roButtonCap   = float32(10)
	roButtonBlank = float32(6)
)

// drawFlatButton draws the archive's button with a label of our own.
//
// Used wherever a button needs a word. The archive's own bitmaps carry their
// text baked in and all of them say 닫기 — fine in a Korean client, wrong in
// this one, and not something a label drawn on top can cover. This is that
// button with the lettering sliced out of it.
func (b *UI2DBackend) drawFlatButton(box ui2d.Rect, label string, disabled bool) {
	r := b.ctx.Renderer()

	art := roButtonTex

	switch {
	case disabled:
		// The plain face, dimmed: the archive has no disabled state, and a
		// hover state on something that cannot be pressed is worse than a
		// flat one.
	case b.ctx.Input().MouseLeftDown && box.Contains(b.ctx.Input().MouseX, b.ctx.Input().MouseY):
		art = roButtonTexDown
	case box.Contains(b.ctx.Input().MouseX, b.ctx.Input().MouseY):
		art = roButtonTexHot
	}

	tint := ui2d.ColorWhite
	text := msgBoxBtnLabel

	if disabled {
		tint = ui2d.Color{R: 0.75, G: 0.75, B: 0.75, A: 1}
		text = escLabelOff
	}

	if !b.drawRoButton(art, box, tint) {
		// No art: the shape drawn rather than nothing at all, so a button
		// with a missing texture is still a button.
		r.DrawRect(box.X, box.Y, box.W, box.H, msgBoxBtnFace)
		r.DrawRectOutline(box.X, box.Y, box.W, box.H, 1, msgBoxBtnBorder)
	}

	textW, _ := r.MeasureText(label, msgBoxTextScale)
	ascent := r.FontAscent(msgBoxTextScale)

	r.DrawText(box.X+(box.W-textW)/2, box.Y+(box.H-ascent)/2, label,
		msgBoxTextScale, text)
}

// drawRoButton draws the art in three pieces, and reports whether it could.
func (b *UI2DBackend) drawRoButton(path string, box ui2d.Rect, tint ui2d.Color) bool {
	tex, err := b.texCache.Load(path)
	if err != nil {
		return false
	}

	r := b.ctx.Renderer()

	cap := min(roButtonCap, box.W/2)

	// The left end, as it is.
	r.DrawImageUV(tex.ID, box.X, box.Y, cap, box.H,
		0, 0, cap/roButtonArtW, 1, tint)

	// The middle, stretched from one plain column of the face.
	if middle := box.W - 2*cap; middle > 0 {
		r.DrawImageUV(tex.ID, box.X+cap, box.Y, middle, box.H,
			roButtonBlank/roButtonArtW, 0, (roButtonBlank+1)/roButtonArtW, 1, tint)
	}

	// And the right end.
	r.DrawImageUV(tex.ID, box.X+box.W-cap, box.Y, cap, box.H,
		(roButtonArtW-cap)/roButtonArtW, 0, 1, 1, tint)

	return true
}
