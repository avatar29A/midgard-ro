package ui

import (
	"strconv"

	"github.com/Faultbox/midgard-ro/internal/engine/ui2d"
	"github.com/Faultbox/midgard-ro/internal/game/items"
	"github.com/Faultbox/midgard-ro/internal/network/packets"
)

// The Inventory window the Item button opens.
//
// A grid of cells rather than a list, with tabs down the left edge, which is
// how the original shows it: usable things, worn things, and everything else.
const (
	itemsWindowID = "hud_win_items"

	itemsW float32 = 260
	itemsH float32 = 232

	itemsPad float32 = 6

	// The grid. Cells are 32 square with a little air between them.
	itemsCell    float32 = 32
	itemsCellGap float32 = 4
	itemsCols            = 6

	// The slot mark inside a cell: a flat oval, inset from the cell's sides
	// and shorter than it is wide.
	itemsSlotInset float32 = 1
	itemsSlotH     float32 = 18

	// itemsTabW is the strip of tabs down the left, and itemsTabH one tab.
	//
	// A tab has to be tall enough for its label stacked a letter to a line,
	// and "equip" is five of them: at the body scale they overran the tab and
	// ran into the one below, so the labels have a smaller scale of their own.
	itemsTabW     float32 = 18
	itemsTabH     float32 = 62
	itemsTabScale float32 = 0.45

	itemsFooterH float32 = 24

	itemsTextScale float32 = 0.65
)

// itemIconPath is where the archive keeps item icons, named in its own
// Korean: the resource table maps an id to one of these.
//
// Under the interface folder, beside the skill icons — not data\texture\item,
// which holds fifteen files and none of them these.
const itemIconPath = skinBasePath + `item\`

// itemTabs are the three the original has, in its order.
var itemTabs = []struct {
	label    string
	category items.Category
}{
	{"item", items.CategoryUsable},
	{"equip", items.CategoryEquip},
	{"etc", items.CategoryEtc},
}

// drawItemsWindow draws the Inventory window when its button has opened it.
func (b *UI2DBackend) drawItemsWindow(state InGameUIState, screenW, screenH float32) {
	if !b.IsWindowOpen(WindowItem) {
		return
	}

	openX := (screenW - itemsW) / 2
	openY := (screenH - itemsH) / 2

	// The body is filled in the image layer below, so the icons are not
	// buried under a solid one.
	opts := ui2d.DefaultWindowOptions()
	opts.BitmapBody = true

	if !b.ctx.BeginWindowEx(itemsWindowID, openX, openY, itemsW, itemsH, "Inventory", opts) {
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

	b.drawItemTabs(x, bodyY)
	b.drawItemGrid(state, x, bodyY)
	b.drawItemsFooter(state, x, y)
	b.ctx.EndWindow()
}

// drawItemTabs draws the three tabs down the left edge.
func (b *UI2DBackend) drawItemTabs(x, bodyY float32) {
	r := b.ctx.Renderer()

	for i, tab := range itemTabs {
		box := ui2d.Rect{X: x + 1, Y: bodyY + itemsPad + float32(i)*itemsTabH, W: itemsTabW, H: itemsTabH}

		face := itemsTabIdle
		if i == b.itemTab {
			face = ui2d.ColorWindowBody
		}

		r.FillImageLayer(box.X, box.Y, box.W, box.H, face)

		// Edged rather than boxed, and in a grey of its own: the panel border
		// is a dark blue-grey meant for a window's outline, and a full box of
		// it around every tab read as three black rectangles.
		//
		// The open tab keeps no right edge, so it runs into the grid beside
		// it — which is how a tab says it is the open one.
		r.DrawRect(box.X, box.Y, box.W, 1, itemsTabBorder)
		r.DrawRect(box.X, box.Y+box.H-1, box.W, 1, itemsTabBorder)
		r.DrawRect(box.X, box.Y, 1, box.H, itemsTabBorder)

		if i != b.itemTab {
			r.DrawRect(box.X+box.W-1, box.Y, 1, box.H, itemsTabBorder)
		}

		// Stacked down the tab, a letter to a line: the strip is 18px wide,
		// the labels do not fit across it, and cutting them to one letter
		// gave two tabs both reading "e".
		b.drawStackedLabel(box, tab.label)

		if b.ctx.InvisibleButtonAt("hud_item_tab_"+tab.label, box.X, box.Y, box.W, box.H) {
			b.itemTab = i
			b.itemScroll = 0
		}
	}
}

// drawStackedLabel writes a label down a narrow tab, one letter to a line.
func (b *UI2DBackend) drawStackedLabel(box ui2d.Rect, label string) {
	r := b.ctx.Renderer()

	letters := []rune(label)

	_, lineH := r.MeasureText(label, itemsTabScale)
	top := box.Y + (box.H-lineH*float32(len(letters)))/2

	for i, letter := range letters {
		text := string(letter)

		capW, _ := r.MeasureText(text, itemsTabScale)
		r.DrawText(box.X+(box.W-capW)/2, top+float32(i)*lineH, text, itemsTabScale, ui2d.ColorText)
	}
}

// drawItemGrid draws the cells and whatever is in them.
func (b *UI2DBackend) drawItemGrid(state InGameUIState, x, bodyY float32) {
	gridX := x + itemsTabW + itemsPad
	gridY := bodyY + itemsPad
	gridH := itemsH - ui2d.FrameTitleH - itemsFooterH - 2*itemsPad
	rows := int(gridH / (itemsCell + itemsCellGap))

	shown := b.itemsOnTab(state)

	for i := 0; i < rows*itemsCols; i++ {
		col := float32(i % itemsCols)
		row := float32(i / itemsCols)

		cell := ui2d.Rect{
			X: gridX + col*(itemsCell+itemsCellGap),
			Y: gridY + row*(itemsCell+itemsCellGap),
			W: itemsCell,
			H: itemsCell,
		}

		b.drawItemCell(cell, shown, i)
	}
}

// drawItemCell draws one cell of the grid, empty or occupied.
func (b *UI2DBackend) drawItemCell(cell ui2d.Rect, shown []packets.InventoryItem, index int) {
	r := b.ctx.Renderer()

	// The slot mark: a flat oval, as the original draws it, not a square.
	// It shows through wherever there is nothing to put in the cell, and an
	// icon sits over it where there is.
	b.ctx.FillEllipseImageLayer(
		cell.X+itemsSlotInset, cell.Y+(cell.H-itemsSlotH)/2,
		cell.W-2*itemsSlotInset, itemsSlotH, itemsCellBg,
	)

	if index >= len(shown) {
		return
	}

	item := shown[index]

	if info, ok := items.Lookup(item.ID); ok && info.Resource != "" {
		if tex, err := b.texCache.Load(itemIconPath + info.Resource + ".bmp"); err == nil {
			r.DrawImage(tex.ID, cell.X+2, cell.Y+2, cell.W-4, cell.H-4, ui2d.ColorWhite)
		}
	}

	// The count in the corner, and only past one: a grid of "1" is noise.
	if item.Count > 1 {
		count := strconv.Itoa(item.Count)
		capW, capH := r.MeasureText(count, itemsTextScale)

		r.DrawText(cell.X+cell.W-capW-1, cell.Y+cell.H-capH, count, itemsTextScale, itemsCountText)
	}

	if item.Equipped {
		// A worn item stays in the list; the original marks it rather than
		// moving it, so the corner carries the mark.
		r.DrawRect(cell.X+1, cell.Y+1, 4, 4, statsBonusUp)
	}

	// Double click uses it, or wears it, depending on which tab it is on.
	// Single clicks do nothing yet: selecting is what a single click means in
	// the original, and there is nothing to select for.
	if b.ctx.DoubleClickedIn("hud_item_cell_"+strconv.Itoa(index), cell) {
		b.itemAction = ItemAction{Index: item.Index, Equip: itemTabs[b.itemTab].category == items.CategoryEquip}
	}
}

// itemsOnTab is the inventory filtered to the open tab.
func (b *UI2DBackend) itemsOnTab(state InGameUIState) []packets.InventoryItem {
	if b.itemTab < 0 || b.itemTab >= len(itemTabs) {
		return state.Inventory
	}

	want := itemTabs[b.itemTab].category

	shown := make([]packets.InventoryItem, 0, len(state.Inventory))
	for _, item := range state.Inventory {
		if items.CategoryOf(item.ID) == want {
			shown = append(shown, item)
		}
	}

	return shown
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

// ItemAction is a double click on an item, waiting to be sent.
type ItemAction struct {
	// Index is the inventory slot, which is how the server names an item.
	Index int

	// Equip is set when the double click was on the equipment tab, where
	// using an item means wearing it.
	Equip bool
}

// TakeItemAction returns a double click on an item and clears it. The
// interface has no connection, so acting on it is the caller's job.
func (b *UI2DBackend) TakeItemAction() (ItemAction, bool) {
	action := b.itemAction
	if action.Index == 0 {
		return ItemAction{}, false
	}

	b.itemAction = ItemAction{}

	return action, true
}

var (
	// itemsCellBg is an empty slot, and itemsTabIdle a tab that is not open.
	// The slot mark: a blue-grey, not the neutral grey the rest of the panel
	// uses. It has to read as blue against a white body or the grid looks
	// like smudges rather than slots.
	itemsCellBg    = ui2d.Color{R: 0.78, G: 0.81, B: 0.91, A: 1}
	itemsTabIdle   = ui2d.Color{R: 0.82, G: 0.83, B: 0.86, A: 1}
	itemsTabBorder = ui2d.Color{R: 0.62, G: 0.63, B: 0.68, A: 1}
	itemsCountText = ui2d.Color{R: 0.1, G: 0.1, B: 0.15, A: 1}
)
