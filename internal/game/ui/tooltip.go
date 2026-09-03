package ui

import (
	"github.com/Faultbox/midgard-ro/internal/engine/ui2d"
)

// What the pointer is over.
//
// A quick panel cell shows an icon and nothing else: which skill it is, what
// level it goes off at and which key fires it are all things the player is
// expected to remember. The original tells them instead, in a small panel
// beside the pointer, and so does this.
//
// Collected while the frame is drawn and painted at the end of it. Draw order
// is the only thing that decides what is on top, so a panel drawn beside a
// window it overlaps has to come after every window — which means it cannot
// be drawn where it is decided.

const (
	// tooltipPad is the air inside the panel and tooltipGap how far it sits
	// from the pointer, clear of the cursor's own art.
	tooltipPad float32 = 5
	tooltipGap float32 = 14

	tooltipLineGap float32 = 2
	tooltipScale   float32 = 0.7

	// tooltipTitleScale is the first line, which names the thing.
	tooltipTitleScale float32 = 0.8
)

var (
	tooltipPlate  = ui2d.Color{R: 0.05, G: 0.06, B: 0.12, A: 0.94}
	tooltipEdge   = ui2d.Color{R: 0.45, G: 0.52, B: 0.72, A: 1}
	tooltipTitle  = ui2d.Color{R: 1, G: 0.94, B: 0.68, A: 1}
	tooltipDetail = ui2d.Color{R: 0.82, G: 0.86, B: 0.95, A: 1}
)

// tooltip is what to show, and where.
type tooltip struct {
	// title names the thing and lines say the rest of it, one to a line.
	title string
	lines []string

	// at is the pointer it belongs to, which it is placed beside rather than
	// on: under the cursor it would be covered by it.
	atX, atY float32

	shown bool
}

// setTooltip says what the pointer is over this frame.
//
// The last one asked for wins. Windows are drawn back to front, so the last
// to claim the pointer is the one on top of the pile — which is the one the
// player is pointing at.
func (b *UI2DBackend) setTooltip(title string, lines []string, atX, atY float32) {
	b.tooltip = tooltip{title: title, lines: lines, atX: atX, atY: atY, shown: true}
}

// drawTooltip paints whatever the frame collected, and forgets it.
//
// Forgotten either way: a panel is shown for as long as the pointer is over
// something, and nothing means nothing rather than the last thing it was over.
func (b *UI2DBackend) drawTooltip(screenW, screenH float32) {
	pending := b.tooltip
	b.tooltip = tooltip{}

	if !pending.shown || pending.title == "" {
		return
	}

	r := b.ctx.Renderer()

	titleW, titleH := r.MeasureText(pending.title, tooltipTitleScale)

	w, h := titleW, titleH
	for _, line := range pending.lines {
		lineW, lineH := r.MeasureText(line, tooltipScale)
		w = max(w, lineW)
		h += lineH + tooltipLineGap
	}

	w += 2 * tooltipPad
	h += 2 * tooltipPad

	// Beside the pointer, and inside the screen. Against the right edge it
	// flips to the other side rather than hanging off, which is what the
	// original does and what keeps it readable in a corner.
	x := pending.atX + tooltipGap
	if x+w > screenW {
		x = pending.atX - tooltipGap - w
	}

	y := pending.atY + tooltipGap
	if y+h > screenH {
		y = screenH - h
	}

	x, y = max(x, 0), max(y, 0)

	r.DrawRect(x, y, w, h, tooltipPlate)
	r.DrawRectOutline(x, y, w, h, 1, tooltipEdge)

	textY := y + tooltipPad
	r.DrawText(x+tooltipPad, textY, pending.title, tooltipTitleScale, tooltipTitle)
	textY += titleH

	for _, line := range pending.lines {
		textY += tooltipLineGap
		_, lineH := r.MeasureText(line, tooltipScale)
		r.DrawText(x+tooltipPad, textY, line, tooltipScale, tooltipDetail)
		textY += lineH
	}
}
