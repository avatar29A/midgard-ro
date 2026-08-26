package ui

import (
	"github.com/Faultbox/midgard-ro/internal/engine/ui2d"
)

// The NPC dialog window. Measured off ref-01, a lossless capture of the
// original at native size, and agreeing with roBrowser's `NpcBox.css` once its
// 1px border is added to the content box it declares.
//
// It is deliberately not a `BeginWindow`: this window has no title bar. It is
// a plain panel — a thin grey rule around a near-white field — and giving it
// the standard chrome would put a gradient caption bar on something that has
// never had one.
const (
	npcWinW = float32(278)
	npcWinH = float32(178)

	// The border is one pixel, and the text field is inset from it.
	npcWinBorder = float32(1)
	npcTextInset = float32(9)

	// Where the window sits. The original does not fix it — the three
	// captures disagree — so it is centered horizontally and set near the
	// bottom, out of the way of the panel in the top-left.
	npcWinBottomGap = float32(120)

	// The message is set at the same reduced size as the rest of our RO
	// windows.
	npcTextScale = loginTextScale

	// Line spacing, and the lift that turns a cap position into the line box
	// DrawText actually places.
	npcLineHeight = float32(13)
	npcTextLift   = charSelTextLift
)

var (
	npcWinBorderColor = ui2d.Color{R: 0.773, G: 0.773, B: 0.773, A: 1}
	npcWinFillColor   = ui2d.Color{R: 0.969, G: 0.969, B: 0.969, A: 1}
)

// renderNPCDialog draws what the NPC said, and reports whether anything was
// drawn.
func (b *UI2DBackend) renderNPCDialog(state InGameUIState, width, height float32) bool {
	if state.DialogMessage == "" {
		return false
	}

	x := float32(int((width - npcWinW) / 2))
	y := float32(int(height - npcWinBottomGap - npcWinH))

	r := b.ctx.Renderer()

	r.DrawRect(x, y, npcWinW, npcWinH, npcWinBorderColor)
	r.DrawRect(x+npcWinBorder, y+npcWinBorder,
		npcWinW-2*npcWinBorder, npcWinH-2*npcWinBorder, npcWinFillColor)

	textX := x + npcTextInset
	textY := y + npcTextInset
	textW := npcWinW - 2*npcTextInset
	textBottom := y + npcWinH - npcTextInset

	measure := func(s string) float32 {
		w, _ := r.MeasureText(s, npcTextScale)
		return w
	}

	for _, line := range WrapNPCText(ParseNPCText(state.DialogMessage), textW, measure) {
		// Text past the bottom is dropped rather than drawn over the border.
		// The original scrolls instead; that waits for Next, which is what
		// makes a message long enough to need it.
		if textY+npcLineHeight > textBottom {
			break
		}

		runX := textX
		for _, run := range line {
			r.DrawText(runX, textY-npcTextLift, run.Text, npcTextScale, run.Color)
			runX += measure(run.Text)
		}

		textY += npcLineHeight
	}

	// The window swallows clicks, so talking to an NPC does not also walk you
	// into the scenery behind the window.
	b.ctx.CaptureMouse(ui2d.Rect{X: x, Y: y, W: npcWinW, H: npcWinH})

	return true
}
