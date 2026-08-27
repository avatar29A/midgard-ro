package ui

import (
	"strconv"

	"github.com/Faultbox/midgard-ro/internal/config"
	"github.com/Faultbox/midgard-ro/internal/engine/cursor"
	"github.com/Faultbox/midgard-ro/internal/engine/ui2d"
	"github.com/Faultbox/midgard-ro/internal/logger"

	"go.uber.org/zap"
)

// The hotkey bar, built from shortitem_bg.bmp — one row, 280x34, repeated
// down for as many rows as are open.
//
// shortcut_bg.bmp, which this used first, is a single 29px strip without the
// arrows under each cell. shortitem_bg.bmp is the one the original repeats,
// and it is what roBrowser tiles down its own bar.
const (
	hotkeyRowTexture         = basicInterfacePath + "shortitem_bg.bmp"
	hotkeyCloseOff           = basicInterfacePath + "sys_close_off.bmp"
	hotkeyCloseOn            = basicInterfacePath + "sys_close_on.bmp"
	hotkeyResizeSkin         = skinBasePath + "btn_resize.bmp"
	hotkeyBarW       float32 = 280
	hotkeyRowH       float32 = 34

	// Where the cells sit in a row: the first icon area starts at x=5, y=5,
	// they are 24 square and repeat every 29. Nine of them, and the strip
	// right of the last carries the row number.
	hotkeyCellX     float32 = 5
	hotkeyCellY     float32 = 5
	hotkeyCellSize  float32 = 24
	hotkeyCellPitch float32 = 29
	hotkeySlots             = 9

	// hotkeyMaxRows is what the original opens to.
	hotkeyMaxRows = 4

	hotkeySysBtn float32 = 11
	hotkeyResize float32 = 13
)

// drawHotkeys draws the bar and handles moving and resizing it.
func (b *UI2DBackend) drawHotkeys(screenW, screenH float32) {
	tex, err := b.texCache.Load(hotkeyRowTexture)
	if err != nil {
		return
	}

	if !b.hotkeyPlaced {
		b.placeHotkeys()
	}

	rows := b.hotkeyRows
	x, y := b.hotkeyX, b.hotkeyY
	h := float32(rows) * hotkeyRowH

	// Claimed before anything else, so a click on a slot does not fall
	// through and walk the character to whatever is behind the bar.
	bar := ui2d.Rect{X: x, Y: y, W: hotkeyBarW, H: h}
	b.ctx.CaptureMouse(bar)

	r := b.ctx.Renderer()
	for i := 0; i < rows; i++ {
		r.DrawImage(tex.ID, x, y+float32(i)*hotkeyRowH, hotkeyBarW, hotkeyRowH, ui2d.ColorWhite)
	}

	b.drawHotkeyRowNumbers(x, y, rows)
	b.drawHotkeyClose(x, y)
	b.hotkeyResizeAndDrag(bar, screenW, screenH)
}

// drawHotkeyRowNumbers puts the row's number in the strip right of its last
// cell, which is what that strip is for.
func (b *UI2DBackend) drawHotkeyRowNumbers(x, y float32, rows int) {
	r := b.ctx.Renderer()

	// The numbers sit right of the ninth cell, in what is left of the row.
	numX := x + hotkeyCellX + hotkeySlots*hotkeyCellPitch
	numW := hotkeyBarW - (numX - x)

	for i := 0; i < rows; i++ {
		label := strconv.Itoa(i + 1)

		capW, capH := r.MeasureText(label, 1)
		capX := numX + (numW-capW)/2
		capY := y + float32(i)*hotkeyRowH + (hotkeyRowH-capH)/2

		// Dark: the row is a light strip, and the pale-on-dark color the rest
		// of the HUD uses reads as an outline here rather than a number.
		r.DrawText(capX, capY, label, 1, ui2d.ColorText)
	}
}

// drawHotkeyClose draws the button at the top right.
//
// It is inert. Nothing reopens the bar yet — there is no menu entry for it —
// so wiring it would let you lose the bar with no way back.
func (b *UI2DBackend) drawHotkeyClose(x, y float32) {
	box := ui2d.Rect{
		X: x + hotkeyBarW - hotkeySysBtn - 2,
		Y: y + 2,
		W: hotkeySysBtn,
		H: hotkeySysBtn,
	}

	asset := hotkeyCloseOff
	if box.Contains(b.ctx.Input().MouseX, b.ctx.Input().MouseY) {
		asset = hotkeyCloseOn
	}

	if tex, err := b.texCache.Load(asset); err == nil {
		b.ctx.Renderer().DrawImage(tex.ID, box.X, box.Y, box.W, box.H, ui2d.ColorWhite)
	}
}

// hotkeyResizeAndDrag opens and closes rows from the corner, and moves the
// bar from anywhere else on it.
func (b *UI2DBackend) hotkeyResizeAndDrag(bar ui2d.Rect, screenW, screenH float32) {
	handle := ui2d.Rect{
		X: bar.X + bar.W - hotkeyResize - 1,
		Y: bar.Y + bar.H - hotkeyResize - 1,
		W: hotkeyResize,
		H: hotkeyResize,
	}

	if tex, err := b.texCache.Load(hotkeyResizeSkin); err == nil {
		b.ctx.Renderer().DrawImage(tex.ID, handle.X, handle.Y, handle.W, handle.H, ui2d.ColorWhite)
	}

	// The hand over the corner, and the plain arrow over the rest: the corner
	// is the only part of the bar that does something other than move it.
	if handle.Contains(b.ctx.Input().MouseX, b.ctx.Input().MouseY) {
		b.wantCursor(cursor.StateClick)
	}

	// Held rather than dragged by a delta: the row count follows where the
	// pointer has got to, so pulling down past each row's edge opens it and
	// pulling back up closes it again, one at a time either way.
	if b.ctx.Held("hud_hotkey_resize", handle) {
		want := int((b.ctx.Input().MouseY - bar.Y) / hotkeyRowH)
		if want < 1 {
			want = 1
		}
		if want > hotkeyMaxRows {
			want = hotkeyMaxRows
		}

		if want != b.hotkeyRows {
			b.hotkeyRows = want
			b.hotkeyDirty = true
		}
	}

	// Moved from anywhere the corner is not, and only if nothing else has
	// claimed the press.
	beforeX, beforeY := b.hotkeyX, b.hotkeyY
	b.ctx.DragHandleFree("hud_hotkey_drag", bar, &b.hotkeyX, &b.hotkeyY)

	if b.hotkeyX != beforeX || b.hotkeyY != beforeY {
		b.hotkeyDirty = true
	}

	b.clampHotkeysToScreen(screenW, screenH)

	if b.hotkeyDirty && b.ctx.Input().MouseLeftReleased {
		b.saveHotkeyPlacement()
	}
}

// placeHotkeys puts the bar where it was last left, or at the top beside the
// Basic Info panel the first time.
func (b *UI2DBackend) placeHotkeys() {
	b.hotkeyX = hudPanelW + 12
	b.hotkeyY = 4
	b.hotkeyRows = 1
	b.hotkeyPlaced = true

	saved := config.LoadUIState()
	if saved.HotkeyRows <= 0 {
		return
	}

	b.hotkeyX, b.hotkeyY = saved.HotkeyX, saved.HotkeyY
	b.hotkeyRows = min(saved.HotkeyRows, hotkeyMaxRows)
}

// saveHotkeyPlacement records where the bar was left and how far it was open.
func (b *UI2DBackend) saveHotkeyPlacement() {
	b.hotkeyDirty = false

	err := config.UpdateUIState(func(state *config.UIState) {
		state.HotkeyX, state.HotkeyY = b.hotkeyX, b.hotkeyY
		state.HotkeyRows = b.hotkeyRows
	})
	if err != nil {
		logger.Warn("could not save hotkey placement", zap.Error(err))
	}
}

// clampHotkeysToScreen keeps the bar reachable, whatever it was dragged
// towards and whatever size the window is now.
func (b *UI2DBackend) clampHotkeysToScreen(screenW, screenH float32) {
	h := float32(b.hotkeyRows) * hotkeyRowH

	b.hotkeyX = clampF(b.hotkeyX, 0, maxF(0, screenW-hotkeyBarW))
	b.hotkeyY = clampF(b.hotkeyY, 0, maxF(0, screenH-h))
}

// maxF is the float32 max the standard library only offers for float64.
func maxF(a, b float32) float32 {
	if a > b {
		return a
	}

	return b
}
