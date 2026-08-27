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

	// The value boxes, measured out of the bitmap: six of them at x 36 to
	// 103, starting at y=6 and repeating every 16.
	statsBoxX     float32 = 36
	statsBoxW     float32 = 68
	statsBoxY     float32 = 6
	statsBoxH     float32 = 13
	statsBoxPitch float32 = 16

	// statsPointsX is where the status point total goes, at the right end of
	// its own row on the other side of the window.
	statsPointsX float32 = 236
	statsPointsW float32 = 38

	// statsPointsRow is which row that is, counting from the top.
	statsPointsRow = 4

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
	opts.BitmapBody = true

	if !b.ctx.BeginWindowEx(statsWindowID, openX, openY, statsW, frameH, title, opts) {
		// Closed from its own X, which is the same as the button closing it.
		b.ToggleWindow(WindowInfo)

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
		box := ui2d.Rect{
			X: x + statsBoxX,
			Y: y + statsBoxY + float32(i)*statsBoxPitch,
			W: statsBoxW,
			H: statsBoxH,
		}

		// The stat itself against the left of its box.
		value := strconv.Itoa(state.PrimaryStats[i])
		_, capH := r.MeasureText(value, statsTextScale)
		textY := box.Y + (box.H-capH)/2

		r.DrawText(box.X+4, textY, value, statsTextScale, ui2d.ColorText)

		// What equipment and buffs add, against the right, and only when
		// there is something to say: every stat reading "+0" is noise.
		bonus := state.PrimaryBonus[i]
		if bonus == 0 {
			continue
		}

		label := "+" + strconv.Itoa(bonus)
		color := statsBonusUp
		if bonus < 0 {
			// Already carries its own minus sign.
			label = strconv.Itoa(bonus)
			color = statsBonusDown
		}

		capW, _ := r.MeasureText(label, statsTextScale)
		r.DrawText(box.X+box.W-capW-4, textY, label, statsTextScale, color)
	}

	points := strconv.Itoa(state.StatusPoints)
	capW, capH := r.MeasureText(points, statsTextScale)

	r.DrawText(
		x+statsPointsX+statsPointsW-capW,
		y+statsBoxY+statsPointsRow*statsBoxPitch+(statsBoxH-capH)/2,
		points, statsTextScale, ui2d.ColorText,
	)
}

// statsWindowID is the frame's id, needed to read its position back.
const statsWindowID = "hud_win_stats"

var (
	// A bonus is green when it helps and red when it does not, which is the
	// only thing that distinguishes the two at a glance.
	statsBonusUp   = ui2d.Color{R: 0.11, G: 0.45, B: 0.15, A: 1}
	statsBonusDown = ui2d.Color{R: 0.65, G: 0.13, B: 0.13, A: 1}
)
