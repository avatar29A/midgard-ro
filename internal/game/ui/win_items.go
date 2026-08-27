package ui

import (
	"strconv"

	"github.com/Faultbox/midgard-ro/internal/engine/ui2d"
	"github.com/Faultbox/midgard-ro/internal/game/items"
	"github.com/Faultbox/midgard-ro/internal/network/packets"
)

// The Item window the Item button opens.
//
// Built the same way as the Skill Tree — there is no single bitmap for it —
// and for the same reason it paints its own body in the image layer, so the
// item icons are not buried under it.
const (
	itemsWindowID = "hud_win_items"

	itemsW float32 = 280
	itemsH float32 = 250

	itemsPad     float32 = 6
	itemsRowH    float32 = 32
	itemsIcon    float32 = 24
	itemsIconGap float32 = 10
	itemsScrollW float32 = 14

	// itemsCountW is the column the counts are right-aligned in.
	itemsCountW float32 = 52

	itemsFooterH float32 = 26

	itemsTextScale float32 = 0.75
)

// drawItemsWindow draws the Item window when its button has opened it.
func (b *UI2DBackend) drawItemsWindow(state InGameUIState, screenW, screenH float32) {
	if !b.IsWindowOpen(WindowItem) {
		return
	}

	openX := (screenW - itemsW) / 2
	openY := (screenH - itemsH) / 2

	opts := ui2d.DefaultWindowOptions()
	opts.BitmapBody = true

	if !b.ctx.BeginWindowEx(itemsWindowID, openX, openY, itemsW, itemsH, "Item", opts) {
		if b.ctx.WindowClosed(itemsWindowID) {
			b.ToggleWindow(WindowItem)
		}

		return
	}

	x, y := openX, openY
	if rect, ok := b.ctx.WindowRect(itemsWindowID); ok {
		x, y = rect.X, rect.Y
	}

	b.ctx.CaptureMouse(ui2d.Rect{X: x, Y: y, W: itemsW, H: itemsH})

	bodyY := y + ui2d.FrameTitleH
	b.ctx.Renderer().FillImageLayer(x, bodyY, itemsW, itemsH-ui2d.FrameTitleH, ui2d.ColorWindowBody)

	b.drawItemRows(state, x, y)
	b.drawItemsFooter(state, x, y)
	b.ctx.EndWindow()
}

// drawItemRows lists what is being carried, scrolled to wherever the bar is.
func (b *UI2DBackend) drawItemRows(state InGameUIState, x, y float32) {
	r := b.ctx.Renderer()

	listX := x + itemsPad
	listY := y + ui2d.FrameTitleH + itemsPad
	listH := itemsH - ui2d.FrameTitleH - itemsFooterH - 2*itemsPad

	if len(state.Inventory) == 0 {
		r.DrawText(listX, listY, "Carrying nothing.", itemsTextScale, skillsEmptyText)

		return
	}

	visible := max(1, int(listH/itemsRowH))
	maxOffset := max(0, len(state.Inventory)-visible)

	offset := min(b.itemScroll, maxOffset)
	listW := itemsW - 2*itemsPad

	if maxOffset > 0 {
		b.itemScroll = b.scrollbar("hud_items", x+itemsW-itemsPad-itemsScrollW, listY,
			listH, offset, maxOffset, visible)
		offset = b.itemScroll
		listW -= itemsScrollW
	}

	for i := 0; i < visible && offset+i < len(state.Inventory); i++ {
		b.drawItemRow(state.Inventory[offset+i], listX, listY+float32(i)*itemsRowH, listW)
	}
}

// drawItemRow draws one item: its cell, its name, and how many are held.
func (b *UI2DBackend) drawItemRow(item packets.InventoryItem, x, y, w float32) {
	r := b.ctx.Renderer()

	// The cell is drawn empty. Item icons are named for the item's own
	// sprite, and the inventory packet carries ids only — the archive's
	// lookup from one to the other is Korean and not read yet, so there is
	// nothing to put in the cell but the cell.
	iconY := y + (itemsRowH-itemsIcon)/2
	r.FillImageLayer(x, iconY, itemsIcon, itemsIcon, skillsIconBg)
	r.DrawRectOutline(x, iconY, itemsIcon, itemsIcon, 1, ui2d.ColorPanelBorder)

	// An id the table does not know is a newer item than the table, so it
	// shows as its id rather than as a blank row.
	name := items.Name(item.ID)
	if name == "" {
		name = "Item #" + strconv.FormatUint(uint64(item.ID), 10)
	}

	if item.Equipped {
		// Marked rather than listed separately: it is still being carried,
		// and the original marks worn things in the same list.
		name += " (equipped)"
	}

	textX := x + itemsIcon + itemsIconGap
	textW := w - itemsIcon - itemsIconGap - itemsCountW

	_, capH := r.MeasureText(name, itemsTextScale)
	textY := y + (itemsRowH-capH)/2

	r.DrawText(textX, textY, fitTextEnd(r, name, itemsTextScale, textW), itemsTextScale, ui2d.ColorText)

	// The count against the right, and only when there is more than one:
	// every line reading "1" is noise.
	if item.Count <= 1 {
		return
	}

	count := strconv.Itoa(item.Count)
	countW, _ := r.MeasureText(count, itemsTextScale)

	r.DrawText(x+w-countW, textY, count, itemsTextScale, ui2d.ColorText)
}

// drawItemsFooter draws the strip along the bottom: the weight and the zeny.
func (b *UI2DBackend) drawItemsFooter(state InGameUIState, x, y float32) {
	r := b.ctx.Renderer()

	footerY := y + itemsH - itemsFooterH
	r.DrawRect(x+1, footerY, itemsW-2, 1, ui2d.ColorPanelBorder)

	weight := "Weight : " + strconv.Itoa(state.PlayerWeight) + " / " + strconv.Itoa(state.PlayerMaxWeight)
	_, capH := r.MeasureText(weight, itemsTextScale)
	textY := footerY + (itemsFooterH-capH)/2

	r.DrawText(x+itemsPad, textY, weight, itemsTextScale, ui2d.ColorText)

	zeny := strconv.FormatInt(state.PlayerZeny, 10) + " z"
	zenyW, _ := r.MeasureText(zeny, itemsTextScale)

	r.DrawText(x+itemsW-itemsPad-zenyW, textY, zeny, itemsTextScale, ui2d.ColorText)
}
