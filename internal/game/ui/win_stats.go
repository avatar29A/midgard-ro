package ui

import (
	"strconv"

	"github.com/Faultbox/midgard-ro/internal/engine/ui2d"
	"github.com/Faultbox/midgard-ro/internal/network/packets"
)

// The Status window the Info button opens.
//
// statwin_bg.bmp carries the whole layout — the six names down the left, the
// derived headings on the right — with the numbers left out. So this draws
// the bitmap and fills in the boxes, rather than laying anything out itself.
const (
	statsTexture = basicInterfacePath + "statwin_bg.bmp"

	statsW float32 = 280
	statsH float32 = 103

	// Each stat's row is three cells, not one: the bitmap divides it at x=76
	// and again at x=88. The value goes in the first, the button that spends
	// a status point in the second, and what that costs in the third.
	statsBoxY     float32 = 6
	statsBoxH     float32 = 13
	statsBoxPitch float32 = 16

	statsValueX  float32 = 37
	statsArrowX  float32 = 77
	statsArrowW  float32 = 11
	statsCostEnd float32 = 100

	// statsArrow is the raise button's mark.
	statsArrow = basicInterfacePath + "arw_right.bmp"

	// The right column is two sub-columns, each ending at an underline the
	// bitmap draws: x 118 to 192 and 200 to 273. The bottom two rows run the
	// full width instead.
	statsRightL    float32 = 118
	statsRightLEnd float32 = 192
	statsRightR    float32 = 200
	statsRightREnd float32 = 273

	statsTextScale float32 = 0.75
)

// drawStatsWindow draws the Status window when the Info button has opened it.
func (b *UI2DBackend) drawStatsWindow(state InGameUIState, screenW, screenH float32) {
	if !b.IsWindowOpen(WindowInfo) {
		return
	}

	tex, err := b.texCache.Load(statsTexture)
	if err != nil {
		return
	}

	title := hudWindowTitles[WindowInfo]
	frameH := statsH + ui2d.FrameTitleH

	openX := (screenW - statsW) / 2
	openY := (screenH - frameH) / 2

	// The body is statwin_bg.bmp, so the frame must not paint one: rectangles
	// and images go in separate batches, and the fill would cover the bitmap
	// however the calls are ordered.
	opts := ui2d.DefaultWindowOptions()

	if !b.ctx.BeginWindowEx(statsWindowID, openX, openY, statsW, frameH, title, opts) {
		// Minimized is not closed: the frame has already drawn its title bar
		// and the window is still open, just collapsed. Only a real close
		// takes the menu button back out.
		if b.ctx.WindowClosed(statsWindowID) {
			b.ToggleWindow(WindowInfo)
		}

		return
	}

	// After BeginWindow, so the contents move with the frame rather than
	// trailing it by a frame while it is dragged.
	x, y := openX, openY
	if rect, ok := b.ctx.WindowRect(statsWindowID); ok {
		x, y = rect.X, rect.Y
	}

	b.ctx.CaptureMouse(ui2d.Rect{X: x, Y: y, W: statsW, H: frameH})

	body := y + ui2d.FrameTitleH
	r := b.ctx.Renderer()
	r.DrawImage(tex.ID, x, body, statsW, statsH, ui2d.ColorWhite)

	b.drawStatValues(state, x, body)
	b.ctx.EndWindow()
}

// drawStatValues fills in the six boxes and the status point total.
func (b *UI2DBackend) drawStatValues(state InGameUIState, x, y float32) {
	r := b.ctx.Renderer()

	for i := 0; i < packets.PrimaryStatCount; i++ {
		rowY := y + statsBoxY + float32(i)*statsBoxPitch

		// The stat and what equipment adds to it, as the original writes it:
		// "60+6" in the box, with the bonus colored so the two read apart.
		value := strconv.Itoa(state.PrimaryStats[i])
		_, capH := r.MeasureText(value, statsTextScale)
		textY := rowY + (statsBoxH-capH)/2

		r.DrawText(x+statsValueX+4, textY, value, statsTextScale, ui2d.ColorText)

		if bonus := state.PrimaryBonus[i]; bonus != 0 {
			valueW, _ := r.MeasureText(value, statsTextScale)

			label := "+" + strconv.Itoa(bonus)
			color := statsBonusUp
			if bonus < 0 {
				// Already carries its own minus sign.
				label = strconv.Itoa(bonus)
				color = statsBonusDown
			}

			r.DrawText(x+statsValueX+4+valueW, textY, label, statsTextScale, color)
		}

		// The raise button, in the cell between the two, and only when there
		// is a point to spend: the original leaves the cell empty otherwise,
		// and a button that cannot do anything is worse than no button.
		if state.StatusPoints > 0 {
			b.drawStatRaise(x+statsArrowX, rowY)
		}

		// What raising it by one would cost, in the third cell.
		cost := strconv.Itoa(state.PrimaryCost[i])
		costW, _ := r.MeasureText(cost, statsTextScale)

		r.DrawText(x+statsCostEnd-costW, textY, cost, statsTextScale, ui2d.ColorText)
	}

	b.drawDerivedStats(state, x, y)
}

// drawDerivedStats fills the right of the window: the numbers worked out from
// the six, each right-aligned on the underline the bitmap draws for it.
func (b *UI2DBackend) drawDerivedStats(state InGameUIState, x, y float32) {
	// Two sub-columns of four, then Status Point across the full width. Guild
	// is left blank: we do not track one yet.
	rows := []struct {
		row   int
		right float32
		text  string
	}{
		{0, statsRightLEnd, pairText(state.Atk, state.AtkBonus)},
		{1, statsRightLEnd, rangeText(state.MatkMin, state.MatkMax)},
		{2, statsRightLEnd, strconv.Itoa(state.Hit)},
		{3, statsRightLEnd, strconv.Itoa(state.Critical)},
		{0, statsRightREnd, pairText(state.Def, state.DefBonus)},
		{1, statsRightREnd, pairText(state.Mdef, state.MdefBonus)},
		{2, statsRightREnd, pairText(state.Flee, state.FleeBonus)},
		{3, statsRightREnd, strconv.Itoa(state.Aspd)},
		{4, statsRightREnd, strconv.Itoa(state.StatusPoints)},
	}

	r := b.ctx.Renderer()
	for _, row := range rows {
		capW, capH := r.MeasureText(row.text, statsTextScale)
		textY := y + statsBoxY + float32(row.row)*statsBoxPitch + (statsBoxH-capH)/2

		r.DrawText(x+row.right-capW, textY, row.text, statsTextScale, ui2d.ColorText)
	}
}

// pairText is how the window writes a number and what equipment adds to it —
// "245 + 21" — and just the number when nothing is added.
func pairText(base, bonus int) string {
	if bonus == 0 {
		return strconv.Itoa(base)
	}

	if bonus < 0 {
		return strconv.Itoa(base) + " - " + strconv.Itoa(-bonus)
	}

	return strconv.Itoa(base) + " + " + strconv.Itoa(bonus)
}

// rangeText is how the window writes Matk, which is a span rather than a sum.
//
// Ordered rather than taken as given: a character with no attack magic can
// arrive with the two the wrong way round — a fresh Novice comes through as
// max 0, min 1 — and "1 ~ 0" reads as a fault rather than as no magic.
func rangeText(a, b int) string {
	low, high := a, b
	if low > high {
		low, high = high, low
	}

	return strconv.Itoa(low) + " ~ " + strconv.Itoa(high)
}

// statsWindowID is the frame's id, needed to read its position back.
const statsWindowID = "hud_win_stats"

var (
	// A bonus is green when it helps and red when it does not, which is the
	// only thing that distinguishes the two at a glance.
	statsBonusUp   = ui2d.Color{R: 0.11, G: 0.45, B: 0.15, A: 1}
	statsBonusDown = ui2d.Color{R: 0.65, G: 0.13, B: 0.13, A: 1}

	// statsArrowHot lights the raise button under the pointer.
	statsArrowHot = ui2d.Color{R: 1, G: 1, B: 0.75, A: 1}
)

// drawStatRaise draws the button that spends a status point on one stat.
//
// It is drawn and not yet wired: raising a stat is CZ_STATUS_CHANGE, which
// this window does not send. Showing it is still right — the cell is there in
// the bitmap and the original fills it whenever there are points — and it is
// hidden when there are none, so it never offers something it cannot do.
func (b *UI2DBackend) drawStatRaise(x, y float32) {
	tex, err := b.texCache.Load(statsArrow)
	if err != nil {
		return
	}

	arrowY := y + (statsBoxH-statsArrowW)/2
	box := ui2d.Rect{X: x, Y: arrowY, W: statsArrowW, H: statsArrowW}

	tint := ui2d.ColorWhite
	if box.Contains(b.ctx.Input().MouseX, b.ctx.Input().MouseY) {
		tint = statsArrowHot
	}

	b.ctx.Renderer().DrawImage(tex.ID, box.X, box.Y, box.W, box.H, tint)
}
