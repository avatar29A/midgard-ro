package ui

import (
	"strconv"
	"strings"

	"github.com/Faultbox/midgard-ro/internal/config"
	"github.com/Faultbox/midgard-ro/internal/engine/cursor"
	"github.com/Faultbox/midgard-ro/internal/engine/ui2d"
	"github.com/Faultbox/midgard-ro/internal/game/items"
	"github.com/Faultbox/midgard-ro/internal/logger"
	"github.com/Faultbox/midgard-ro/internal/network/packets"

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

	// hotkeyNumScale shrinks the row numbers. At full size they crowded the
	// strip they sit in and ran into the resize corner.
	hotkeyNumScale float32 = 0.7

	// hotkeyCountScale shrinks the count in a cell's corner.
	//
	// Smaller than the inventory's, which is the same number at the same
	// scale in a cell half again as wide. At the inventory's size a
	// three-digit count filled a 24px cell corner to corner and sat over the
	// icon rather than beside it.
	hotkeyCountScale float32 = 0.42
)

// hotkeyCountEmpty is the count on a cell whose item has run out.
var hotkeyCountEmpty = ui2d.Color{R: 0.65, G: 0.3, B: 0.3, A: 1}

// drawHotkeys draws the bar and handles moving and resizing it.
func (b *UI2DBackend) drawHotkeys(state InGameUIState, screenW, screenH float32) {
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

	b.drawHotkeyCells(state)
	b.drawHotkeyRowNumbers(x, y, rows)
	b.drawHotkeyClose(x, y)

	// Cells first: a press on one is claimed before the bar can read it as a
	// move.
	b.hotkeyCellInput(state)
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

		capW, capH := r.MeasureText(label, hotkeyNumScale)
		capX := numX + (numW-capW)/2
		capY := y + float32(i)*hotkeyRowH + (hotkeyRowH-capH)/2

		// Dark: the row is a light strip, and the pale-on-dark color the rest
		// of the HUD uses reads as an outline here rather than a number.
		r.DrawText(capX, capY, label, hotkeyNumScale, ui2d.ColorText)
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
	b.loadHotkeyItems(saved.HotkeyItems)
}

// hotkeyCellKey names a cell in the saved state.
func hotkeyCellKey(row, col int) string {
	return strconv.Itoa(row) + "," + strconv.Itoa(col)
}

// loadHotkeyItems puts saved shortcuts back in their cells.
//
// A key that is not a cell on this bar is skipped rather than refused: a
// config written when the bar had more rows should lose the rows that are
// gone and keep the rest, not fail to load at all.
func (b *UI2DBackend) loadHotkeyItems(saved map[string]uint32) {
	for key, itemID := range saved {
		row, col, ok := parseHotkeyCellKey(key)
		if !ok || itemID == 0 {
			continue
		}

		b.setHotkeyItem(row, col, itemID)
	}
}

// parseHotkeyCellKey reads a "row,col" key back.
func parseHotkeyCellKey(key string) (row, col int, ok bool) {
	comma := strings.IndexByte(key, ',')
	if comma < 0 {
		return 0, 0, false
	}

	row, err := strconv.Atoi(key[:comma])
	if err != nil {
		return 0, 0, false
	}

	col, err = strconv.Atoi(key[comma+1:])
	if err != nil {
		return 0, 0, false
	}

	return row, col, true
}

// savedHotkeyItems is the filled cells, ready to write out.
func (b *UI2DBackend) savedHotkeyItems() map[string]uint32 {
	items := make(map[string]uint32)
	for row := 0; row < hotkeyMaxRows; row++ {
		for col := 0; col < hotkeySlots; col++ {
			if id := b.hotkeyItems[row][col]; id != 0 {
				items[hotkeyCellKey(row, col)] = id
			}
		}
	}

	return items
}

// saveHotkeyPlacement records where the bar was left and how far it was open.
func (b *UI2DBackend) saveHotkeyPlacement() {
	b.hotkeyDirty = false

	err := config.UpdateUIState(func(state *config.UIState) {
		state.HotkeyX, state.HotkeyY = b.hotkeyX, b.hotkeyY
		state.HotkeyRows = b.hotkeyRows
		state.HotkeyItems = b.savedHotkeyItems()
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

// The quick panel's contents.
//
// A cell holds an item id, not an inventory slot. Slots move — using a potion
// can renumber what sits above it — so a shortcut pointing at slot four would
// quietly become a shortcut to whatever landed there. The original keys on the
// item too, which is why its shortcuts survive a rearranged bag.
//
// The count is not stored either. It is counted out of the inventory when the
// cell is drawn, so it follows what is actually carried without anything
// having to remember to update it.

// hotkeyDrag is a cell being dragged, either to another cell or off the bar.
type hotkeyDrag struct {
	active   bool
	row, col int
	itemID   uint32
}

// hotkeyPress is a shortcut pressed this frame, resolved when the bar is
// drawn and the inventory is to hand.
type hotkeyPress struct {
	row, col int
	set      bool
}

// hotkeyCellRect is where one cell sits on screen.
func (b *UI2DBackend) hotkeyCellRect(row, col int) ui2d.Rect {
	return ui2d.Rect{
		X: b.hotkeyX + hotkeyCellX + float32(col)*hotkeyCellPitch,
		Y: b.hotkeyY + float32(row)*hotkeyRowH + hotkeyCellY,
		W: hotkeyCellSize,
		H: hotkeyCellSize,
	}
}

// hotkeyCellAt finds the cell under a point, in the rows that are open.
func (b *UI2DBackend) hotkeyCellAt(px, py float32) (row, col int, ok bool) {
	for r := 0; r < b.hotkeyRows; r++ {
		for c := 0; c < hotkeySlots; c++ {
			if b.hotkeyCellRect(r, c).Contains(px, py) {
				return r, c, true
			}
		}
	}

	return 0, 0, false
}

// setHotkeyItem writes a cell, reporting whether it is a cell at all.
//
// Rows that are not open still take assignments: the bar can be pulled shut
// over a row without emptying it, and pulling it back open should find the
// row as it was left.
func (b *UI2DBackend) setHotkeyItem(row, col int, itemID uint32) bool {
	if row < 0 || row >= hotkeyMaxRows || col < 0 || col >= hotkeySlots {
		return false
	}

	b.hotkeyItems[row][col] = itemID

	return true
}

// AssignHotkey puts an item in a cell, replacing whatever was there, and
// saves.
//
// Saved on the change rather than on the next mouse release: an assignment
// arrives from the inventory window, which is drawn after the bar, so by the
// time the bar next looks the release that made it is a frame gone.
func (b *UI2DBackend) AssignHotkey(row, col int, itemID uint32) bool {
	if !b.setHotkeyItem(row, col, itemID) {
		return false
	}

	b.saveHotkeyPlacement()

	return true
}

// PressHotkey asks for the item in a cell to be used. Resolved against the
// inventory when the bar is next drawn, which is where the inventory is.
func (b *UI2DBackend) PressHotkey(row, col int) {
	if row < 0 || row >= hotkeyMaxRows || col < 0 || col >= hotkeySlots {
		return
	}

	b.hotkeyPress = hotkeyPress{row: row, col: col, set: true}
}

// TextEntryFocused reports whether typing is going into a field rather than
// to the game, so a shortcut key does not fire while a message is being
// written.
func (b *UI2DBackend) TextEntryFocused() bool {
	for _, id := range []string{"hud_chat_input", "hud_chat_name", dropQtyWindowID + "_amount"} {
		if b.ctx.Focused(id) {
			return true
		}
	}

	return false
}

// inventoryCount is how many of an item are carried, across every slot
// holding it.
func inventoryCount(inventory []packets.InventoryItem, itemID uint32) int {
	total := 0
	for _, item := range inventory {
		if item.ID == itemID {
			total += item.Count
		}
	}

	return total
}

// useHotkey turns a cell into the same action an inventory double click
// produces, so both go out through one path.
//
// A cell whose item is all gone does nothing. The shortcut is not cleared:
// running out of potions is not a reason to lose the key you drink them with.
func (b *UI2DBackend) useHotkey(state InGameUIState, row, col int) {
	itemID := b.hotkeyItems[row][col]
	if itemID == 0 {
		return
	}

	for _, item := range state.Inventory {
		if item.ID != itemID {
			continue
		}

		b.itemAction = ItemAction{
			Index: item.Index,
			Equip: items.CategoryOf(itemID) == items.CategoryEquip,
		}

		return
	}
}

// drawHotkeyCells draws what is in the cells: the icon, and how many are
// carried.
func (b *UI2DBackend) drawHotkeyCells(state InGameUIState) {
	r := b.ctx.Renderer()

	for row := 0; row < b.hotkeyRows; row++ {
		for col := 0; col < hotkeySlots; col++ {
			itemID := b.hotkeyItems[row][col]
			if itemID == 0 {
				continue
			}

			cell := b.hotkeyCellRect(row, col)

			info, known := items.Lookup(itemID)
			if known && info.Resource != "" {
				if tex, err := b.texCache.Load(itemIconPath + info.Resource + ".bmp"); err == nil {
					r.DrawImage(tex.ID, cell.X, cell.Y, cell.W, cell.H, ui2d.ColorWhite)
				}
			}

			// How many are left. Dimmed at zero rather than hidden: an empty
			// shortcut you can still see is the difference between "none
			// left" and "nothing assigned".
			count := inventoryCount(state.Inventory, itemID)
			label := strconv.Itoa(count)
			color := itemsCountText
			if count == 0 {
				color = hotkeyCountEmpty
			}

			capW, capH := r.MeasureText(label, hotkeyCountScale)
			r.DrawText(cell.X+cell.W-capW, cell.Y+cell.H-capH, label, hotkeyCountScale, color)
		}
	}
}

// hotkeyCellInput handles using, moving and clearing cells.
//
// Runs before the bar's own move-and-resize so a press on a cell is claimed
// first: dragging a potion out of a slot should not also drag the bar across
// the screen.
func (b *UI2DBackend) hotkeyCellInput(state InGameUIState) {
	if b.hotkeyPress.set {
		b.useHotkey(state, b.hotkeyPress.row, b.hotkeyPress.col)
		b.hotkeyPress = hotkeyPress{}
	}

	for row := 0; row < b.hotkeyRows; row++ {
		for col := 0; col < hotkeySlots; col++ {
			if b.hotkeyItems[row][col] == 0 {
				continue
			}

			cell := b.hotkeyCellRect(row, col)
			id := "hud_hotkey_cell_" + strconv.Itoa(row) + "_" + strconv.Itoa(col)

			if b.ctx.DoubleClickedIn(id, cell) {
				b.useHotkey(state, row, col)
			}

			// Held claims the press, which is what keeps the bar still while
			// a cell is dragged off it.
			if b.ctx.Held(id+"_drag", cell) && !b.hotkeyDrag.active {
				b.hotkeyDrag = hotkeyDrag{
					active: true,
					row:    row,
					col:    col,
					itemID: b.hotkeyItems[row][col],
				}
			}
		}
	}

	b.finishHotkeyDrag()
}

// finishHotkeyDrag decides what a released cell drag meant: dropped on
// another cell it moves there, replacing whatever was in it, and dropped
// anywhere else it comes off the bar.
//
// Coming off the bar does not drop the item on the ground. The cell holds a
// shortcut, not the item — the item never left the bag, and throwing it away
// because a shortcut was rearranged would be a nasty surprise.
func (b *UI2DBackend) finishHotkeyDrag() {
	if !b.hotkeyDrag.active || b.ctx.Input().MouseLeftDown {
		return
	}

	in := b.ctx.Input()
	from := b.hotkeyDrag

	moved := false
	if row, col, ok := b.hotkeyCellAt(in.MouseX, in.MouseY); ok {
		if row != from.row || col != from.col {
			b.hotkeyItems[row][col] = from.itemID
			b.hotkeyItems[from.row][from.col] = 0
			moved = true
		}
	} else {
		b.hotkeyItems[from.row][from.col] = 0
		moved = true
	}

	b.hotkeyDrag = hotkeyDrag{}

	if moved {
		b.saveHotkeyPlacement()
	}
}
