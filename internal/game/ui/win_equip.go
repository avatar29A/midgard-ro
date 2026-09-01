package ui

import (
	"strconv"

	"github.com/Faultbox/midgard-ro/internal/engine/ui2d"
	"github.com/Faultbox/midgard-ro/internal/game/items"
	"github.com/Faultbox/midgard-ro/internal/network/packets"
)

// The Equipment window, which is one bitmap with ten places painted into it.
//
// equipwin_bg.bmp carries the whole layout — two columns of labeled slots
// around a middle strip meant for a view of the character — so nothing here
// draws chrome. What it does is put icons on the right ovals, and the oval
// positions were measured off the bitmap rather than guessed: they are
// 26x13, at x 4 and 249, on rows 11, 36, 62, 88 and 114.
const (
	equipWindowID = "hud_win_equip"

	equipBgFile = "equipwin_bg.bmp"

	equipW     float32 = 280
	equipBodyH float32 = 130

	// equipSlotW and equipSlotH are the painted oval, and equipSlotLeftX and
	// equipSlotRightX where the two columns sit.
	equipSlotW      float32 = 26
	equipSlotH      float32 = 13
	equipSlotLeftX  float32 = 4
	equipSlotRightX float32 = 249

	// equipRow0Y is the first row's top and equipRowPitch the step between
	// rows: 11, 36, 62, 88, 114 is 25.75 apart, which rounds to those five
	// when laid out from the first.
	equipRow0Y    float32 = 11
	equipRowPitch float32 = 25.75

	// equipIcon is how big an item is drawn. Taller than the oval it sits on,
	// as in the original: the oval marks the place and the icon covers it.
	//
	// Not 24, which is what an item icon is: the bottom oval sits three pixels
	// from the edge of the bitmap, so the last row has to be pushed back up to
	// stay inside — and at 24 that push is enough to put it on top of the row
	// above. The limit works out at 23.5, so 22 leaves the rows a little air.
	equipIcon float32 = 22
)

// equipLayout is the ten slots, in the order the bitmap paints them.
//
// Left column first, then right, each top to bottom. The pairing of label to
// position is the bitmap's: it reads head/head/R-hand/robe/accessary down the
// left and head/body/L-hand/shoes/accessary down the right, so the two heads
// on the left are the upper and middle ones and the single head on the right
// is the lower.
var equipLayout = []struct {
	slot  uint32
	right bool
	row   int
}{
	{packets.EQP_HEAD_TOP, false, 0},
	{packets.EQP_HEAD_MID, false, 1},
	{packets.EQP_HAND_R, false, 2},
	{packets.EQP_GARMENT, false, 3},
	{packets.EQP_ACC_R, false, 4},

	{packets.EQP_HEAD_LOW, true, 0},
	{packets.EQP_ARMOR, true, 1},
	{packets.EQP_HAND_L, true, 2},
	{packets.EQP_SHOES, true, 3},
	{packets.EQP_ACC_L, true, 4},
}

// drawEquipWindow draws what the character is wearing.
func (b *UI2DBackend) drawEquipWindow(state InGameUIState, screenW, screenH float32) {
	if !b.IsWindowOpen(WindowEquip) {
		return
	}

	height := equipBodyH + ui2d.FrameTitleH

	openX := (screenW - equipW) / 2
	openY := (screenH - height) / 2

	// The frame must not paint the body: the background bitmap goes there,
	// and a solid over it would hide the slots it paints.
	opts := ui2d.DefaultWindowOptions()

	if !b.ctx.BeginWindowEx(equipWindowID, openX, openY, equipW, height, "Equipment", opts) {
		if b.ctx.WindowClosed(equipWindowID) {
			b.ToggleWindow(WindowEquip)
		}

		return
	}

	x, y := openX, openY
	if rect, ok := b.ctx.WindowRect(equipWindowID); ok {
		x, y = rect.X, rect.Y
	}

	b.ctx.CaptureMouse(ui2d.Rect{X: x, Y: y, W: equipW, H: height})

	bodyY := y + ui2d.FrameTitleH

	if tex, err := b.texCache.Load(basicInterfacePath + equipBgFile); err == nil {
		b.ctx.Renderer().DrawImage(tex.ID, x, bodyY, equipW, equipBodyH, ui2d.ColorWhite)
	} else {
		// Without the bitmap there is no window at all, only floating icons.
		// A plain body is enough to keep them readable.
		b.ctx.Renderer().DrawRect(x, bodyY, equipW, equipBodyH, ui2d.ColorWindowBody)
	}

	b.itemHover = itemHover{}

	for _, place := range equipLayout {
		b.drawEquipSlot(state, place.slot, equipSlotRect(x, bodyY, place.right, place.row))
	}

	b.drawItemHover(screenW, screenH)
	b.ctx.EndWindow()
}

// equipSlotRect is where one slot's icon goes, in screen coordinates.
//
// The icon is wider and taller than the oval, centered on it, and the bottom
// row is nudged up so the last icon stays inside the window: the oval there
// sits 3 pixels from the bitmap's edge and a 24-pixel icon centered on it
// would hang over.
func equipSlotRect(x, bodyY float32, right bool, row int) ui2d.Rect {
	left := x + equipSlotLeftX
	if right {
		left = x + equipSlotRightX
	}

	top := bodyY + equipRow0Y + float32(row)*equipRowPitch

	icon := ui2d.Rect{
		X: left + (equipSlotW-equipIcon)/2,
		Y: top + (equipSlotH-equipIcon)/2,
		W: equipIcon,
		H: equipIcon,
	}

	if over := icon.Y + icon.H - (bodyY + equipBodyH); over > 0 {
		icon.Y -= over
	}

	if under := bodyY - icon.Y; under > 0 {
		icon.Y += under
	}

	return icon
}

// drawEquipSlot draws whatever is worn in one slot, and acts on it.
func (b *UI2DBackend) drawEquipSlot(state InGameUIState, slot uint32, cell ui2d.Rect) {
	item, worn := state.Equipment[slot]
	if !worn {
		return
	}

	if info, ok := items.Lookup(item.ID); ok && info.Resource != "" {
		if tex, err := b.texCache.Load(itemIconPath + info.Resource + ".bmp"); err == nil {
			b.ctx.Renderer().DrawImage(tex.ID, cell.X, cell.Y, cell.W, cell.H, ui2d.ColorWhite)
		}
	}

	in := b.ctx.Input()
	if !b.itemDrag.active && cell.Contains(in.MouseX, in.MouseY) {
		b.itemHover = itemHover{
			text: items.Name(item.ID),
			x:    cell.X + cell.W/2,
			y:    cell.Y + cell.H,
		}
	}

	// A double click takes it off, which is what it means in the original.
	if b.ctx.DoubleClickedIn("hud_equip_slot_"+strconv.FormatUint(uint64(slot), 16), cell) {
		b.itemAction = ItemAction{Index: item.Index, Unequip: true}
	}

	// A press starts a drag, so it can be pulled back into the bag. Which
	// slot it came from is carried along: an item worn in one of two
	// accessory slots has to be told apart from the other one.
	if in.MouseLeftPressed && !b.itemDrag.active && cell.Contains(in.MouseX, in.MouseY) {
		b.itemDrag = itemDrag{
			active:    true,
			index:     item.Index,
			itemID:    item.ID,
			count:     1,
			fromEquip: true,
			slot:      slot,
		}
	}
}

// equipSlotAt finds the equipment slot under a point, for a drop.
//
// Computed from the window's stored rect rather than remembered from the last
// frame, so it does not depend on the equipment window having been drawn
// before whichever window the drag started in.
func (b *UI2DBackend) equipSlotAt(px, py float32) (uint32, bool) {
	if !b.IsWindowOpen(WindowEquip) {
		return 0, false
	}

	rect, ok := b.ctx.WindowRect(equipWindowID)
	if !ok {
		return 0, false
	}

	bodyY := rect.Y + ui2d.FrameTitleH

	for _, place := range equipLayout {
		if equipSlotRect(rect.X, bodyY, place.right, place.row).Contains(px, py) {
			return place.slot, true
		}
	}

	return 0, false
}

// equipWindowRect is the whole window, for deciding whether a drag ended
// inside it.
func (b *UI2DBackend) equipWindowRect() (ui2d.Rect, bool) {
	if !b.IsWindowOpen(WindowEquip) {
		return ui2d.Rect{}, false
	}

	return b.ctx.WindowRect(equipWindowID)
}
