package ui

import (
	"go.uber.org/zap"

	"github.com/Faultbox/midgard-ro/internal/engine/ui2d"
	"github.com/Faultbox/midgard-ro/internal/logger"
)

// The menu window. A second window, drawn beside the text one and shown at the
// same time — ref-01 has both on screen at once.
//
// Sizes come from roBrowser's `NpcMenu.css`, with the same +2 for its border
// that the text window needed: its 276 content box measures 278 in ref-01.
const (
	npcMenuW = float32(278)
	npcMenuH = float32(120)

	npcMenuBorder  = float32(1)
	npcMenuInsetX  = float32(8)
	npcMenuInsetY  = float32(8)
	npcMenuRowH    = float32(20)
	npcMenuRowText = float32(5)

	// Four rows fit; the rest scroll. That is not a guess — roBrowser gives
	// the list an 80px box and 20px rows, and the warp-list capture shows
	// exactly four rows above a scrollbar.
	npcMenuVisibleRows = 4

	// The buttons sit along the bottom right, cancel outermost.
	npcMenuBtnGap    = float32(6)
	npcMenuBtnBottom = float32(6)
)

var (
	// The selected row's fill, from roBrowser's `.selected` — the pale blue
	// band that runs the full width of the row in ref-01.
	npcMenuSelectedColor = ui2d.Color{R: 0.804, G: 0.878, B: 1, A: 1}
	npcMenuListColor     = ui2d.Color{R: 0.976, G: 0.976, B: 0.976, A: 1}
)

// renderNPCMenu draws the choices, and reports whether it drew anything.
func (b *UI2DBackend) renderNPCMenu(state InGameUIState, width, height float32) bool {
	if len(state.DialogMenu) == 0 {
		b.npcMenuIdx, b.npcMenuScroll = 0, 0

		return false
	}

	b.clampMenuSelection(len(state.DialogMenu))

	if !b.npcMenuPlaced {
		b.npcMenuX = float32(int((width - npcMenuW) / 2))
		b.npcMenuY = float32(int(height - npcMenuH - 24))
		b.npcMenuPlaced = true
	}

	b.ctx.DragHandle("npc_menu", ui2d.Rect{X: b.npcMenuX, Y: b.npcMenuY, W: npcMenuW, H: npcMenuH},
		&b.npcMenuX, &b.npcMenuY)

	x, y := b.npcMenuX, b.npcMenuY

	b.fillNPCRect(x, y, npcMenuW, npcMenuH, npcWinBorderColor)
	b.fillNPCRect(x+npcMenuBorder, y+npcMenuBorder,
		npcMenuW-2*npcMenuBorder, npcMenuH-2*npcMenuBorder, npcWinFillColor)

	listX := x + npcMenuInsetX
	listY := y + npcMenuInsetY
	listW := npcMenuW - 2*npcMenuInsetX
	listH := npcMenuVisibleRows * npcMenuRowH

	b.fillNPCRect(listX, listY, listW, listH, npcMenuListColor)

	b.scrollMenu(state, listX, listY, listW, listH)
	b.drawMenuRows(state, listX, listY, listW)
	if maxScroll := len(state.DialogMenu) - npcMenuVisibleRows; maxScroll > 0 {
		b.npcMenuScroll = b.scrollbar("npc_menu_scroll",
			listX+listW-scrollW, listY, listH,
			b.npcMenuScroll, maxScroll, npcMenuVisibleRows)
	}
	b.drawNPCMenuButtons(state, x, y)

	b.ctx.CaptureMouse(ui2d.Rect{X: x, Y: y, W: npcMenuW, H: npcMenuH})

	return true
}

// clampMenuSelection keeps the selection and the scroll inside a list that may
// have changed under them — a new menu is usually a different length.
func (b *UI2DBackend) clampMenuSelection(count int) {
	if b.npcMenuIdx >= count {
		b.npcMenuIdx = count - 1
	}
	if b.npcMenuIdx < 0 {
		b.npcMenuIdx = 0
	}

	maxScroll := count - npcMenuVisibleRows
	if maxScroll < 0 {
		maxScroll = 0
	}

	if b.npcMenuScroll > maxScroll {
		b.npcMenuScroll = maxScroll
	}
	if b.npcMenuScroll < 0 {
		b.npcMenuScroll = 0
	}

	// Keep the selected row on screen, which is what makes a keyboard or a
	// wrapped selection usable at all.
	if b.npcMenuIdx < b.npcMenuScroll {
		b.npcMenuScroll = b.npcMenuIdx
	}
	if b.npcMenuIdx >= b.npcMenuScroll+npcMenuVisibleRows {
		b.npcMenuScroll = b.npcMenuIdx - npcMenuVisibleRows + 1
	}
}

// scrollMenu moves the list under the wheel.
func (b *UI2DBackend) scrollMenu(state InGameUIState, listX, listY, listW, listH float32) {
	in := b.ctx.Input()
	if in == nil || in.ScrollY == 0 {
		return
	}

	if !(ui2d.Rect{X: listX, Y: listY, W: listW, H: listH}).Contains(in.MouseX, in.MouseY) {
		return
	}

	b.npcMenuScroll -= int(in.ScrollY)

	maxScroll := len(state.DialogMenu) - npcMenuVisibleRows
	if maxScroll < 0 {
		maxScroll = 0
	}

	if b.npcMenuScroll > maxScroll {
		b.npcMenuScroll = maxScroll
	}
	if b.npcMenuScroll < 0 {
		b.npcMenuScroll = 0
	}
}

// drawMenuRows draws the visible slice of the list. A click selects a row and
// a double click takes it, which is how the original behaves and what the
// window's OK button does the slow way.
func (b *UI2DBackend) drawMenuRows(state InGameUIState, listX, listY, listW float32) {
	r := b.ctx.Renderer()

	for row := 0; row < npcMenuVisibleRows; row++ {
		index := b.npcMenuScroll + row
		if index >= len(state.DialogMenu) {
			break
		}

		rowY := listY + float32(row)*npcMenuRowH
		rect := ui2d.Rect{X: listX, Y: rowY, W: listW, H: npcMenuRowH}

		if index == b.npcMenuIdx {
			b.fillNPCRect(listX, rowY, listW, npcMenuRowH, npcMenuSelectedColor)
		}

		if b.ctx.InvisibleButtonAt(menuRowID(index), listX, rowY, listW, npcMenuRowH) {
			b.npcMenuIdx = index
		}

		// Taking a choice with a double click is the shortcut the original
		// offers; OK is the same answer, pressed twice as slowly.
		if b.ctx.DoubleClickedIn(menuRowID(index), rect) && state.OnDialogChoose != nil {
			b.npcMenuIdx = index
			state.OnDialogChoose(index + 1)
		}

		r.DrawText(listX+npcMenuRowText, rowY+npcMenuRowText-npcTextLift,
			state.DialogMenu[index], npcTextScale, npcTextColor)
	}
}

func menuRowID(index int) string {
	return "npc_menu_row_" + string(rune('a'+index%26)) + string(rune('0'+index/26))
}

// drawNPCMenuButtons draws OK and cancel along the bottom right.
func (b *UI2DBackend) drawNPCMenuButtons(state InGameUIState, x, y float32) {
	ok, cancel := b.loadMenuButton("btn_ok.bmp"), b.loadMenuButton("btn_cancel.bmp")
	if ok == nil || cancel == nil {
		return
	}

	btnY := y + npcMenuH - npcMenuBtnBottom - loginBtnH
	cancelX := x + npcMenuW - npcMenuBtnGap - loginBtnW
	okX := cancelX - npcMenuBtnGap - loginBtnW

	if b.skinButton("npc_menu_ok", okX, btnY, ok, ok, ok, "OK") && state.OnDialogChoose != nil {
		state.OnDialogChoose(b.npcMenuIdx + 1)
	}

	if b.skinButton("npc_menu_cancel", cancelX, btnY, cancel, cancel, cancel, "cancel") &&
		state.OnDialogCancel != nil {
		state.OnDialogCancel()
	}
}

// loadMenuButton loads one of the shared OK/cancel bitmaps. They ship no hover
// or pressed art, so the same texture serves all three states and the widget
// shades it instead.
func (b *UI2DBackend) loadMenuButton(name string) *TextureInfo {
	if tex, ok := b.npcMenuBtns[name]; ok {
		return tex
	}

	if b.npcMenuBtns == nil {
		b.npcMenuBtns = map[string]*TextureInfo{}
	}

	tex, err := b.texCache.Load(loginTexBasePath + name)
	if err != nil {
		logger.Warn("NPC menu button art unavailable, the menu cannot be answered",
			zap.String("path", loginTexBasePath+name), zap.Error(err))

		b.npcMenuBtns[name] = nil

		return nil
	}

	b.npcMenuBtns[name] = tex

	return tex
}
