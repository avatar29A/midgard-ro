package ui

import (
	"strconv"

	"github.com/Faultbox/midgard-ro/internal/engine/ui2d"
	"github.com/Faultbox/midgard-ro/internal/game/items"
	"github.com/Faultbox/midgard-ro/internal/network/packets"
)

// The Equipment window, which in the modern client is also the Status window.
//
// One window, stacked out of the archive's own pieces: a row of tabs, the ten
// slots around a view of the character, the strip that carries the equipment
// switch, and the status block underneath. They are all 280 wide, which is
// what says they were meant to sit on top of one another.
//
// Both menu buttons open it, because both of the things it shows are in it.
const (
	equipWindowID = "hud_win_equip"

	equipBgFile      = "equipwin_bg2.bmp"
	equipSpecialFile = "equipwin_special.bmp"
	equipDividerFile = "equipwin_bg3.bmp"

	equipW float32 = 280

	// The pieces, top to bottom. equipPageH is the taller of the two pages,
	// so the window does not change height when the tab is switched.
	equipTabsH    float32 = 21
	equipPageH    float32 = 157
	equipSpecialH float32 = 133
	equipDividerH float32 = 20

	// equipSlotW and equipSlotH are the painted oval, and equipSlotLeftX and
	// equipSlotRightX where the two columns sit. The same in equipwin_bg2 as
	// in the shorter equipwin_bg: 26 by 13, at x 4 and 249.
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

	equipTextScale float32 = 0.7
)

// equipHeight is the whole window, title bar included.
func equipHeight() float32 {
	return ui2d.FrameTitleH + equipTabsH + equipPageH + equipDividerH + statsH
}

// equipPages are the two tabs across the top.
//
// Special Equipment is drawn from its own art and stands empty: costume and
// shadow gear are their own slots on the server and nothing here wears them
// yet. Drawn rather than left out, because the tab is what says the gear
// exists and that this client does not dress it.
var equipPages = []struct {
	label string
	art   string
}{
	{"General", equipBgFile},
	{"Special Equipment", equipSpecialFile},
}

// equipLayout is the ten slots, in the order the bitmap paints them.
//
// Left column first, then right, each top to bottom. The pairing of label to
// position is the bitmap's: it reads head/head/R-hand/robe/acc.1 down the left
// and head/body/L-hand/shoes/acc.2 down the right, so the two heads on the
// left are the upper and middle ones and the single head on the right is the
// lower.
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

// drawEquipWindow draws what the character is wearing, and what it is worth.
func (b *UI2DBackend) drawEquipWindow(state InGameUIState, screenW, screenH float32) {
	if !b.IsWindowOpen(WindowEquip) {
		return
	}

	height := equipHeight()

	openX := (screenW - equipW) / 2
	openY := (screenH - height) / 2

	// The frame must not paint the body: the background bitmaps go there, and
	// a solid over them would hide the slots they paint.
	opts := ui2d.DefaultWindowOptions()

	if !b.ctx.BeginWindowEx(equipWindowID, openX, openY, equipW, height, "Equip", opts) {
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

	b.itemHover = itemHover{}

	body := y + ui2d.FrameTitleH

	b.drawEquipTabs(x, body)

	pageY := body + equipTabsH
	b.drawEquipPage(state, x, pageY)

	dividerY := pageY + equipPageH
	b.drawEquipDivider(state, x, dividerY)

	b.drawEquipStats(state, x, dividerY+equipDividerH)

	b.drawItemHover(screenW, screenH)
	b.ctx.EndWindow()
}

// drawEquipTabs draws the row across the top and switches pages.
func (b *UI2DBackend) drawEquipTabs(x, y float32) {
	r := b.ctx.Renderer()

	at := x

	for i, page := range equipPages {
		width, _ := r.MeasureText(page.label, equipTextScale)
		w := width + 2*equipTabPad

		box := ui2d.Rect{X: at, Y: y, W: w, H: equipTabsH}

		b.drawEquipTabFrame(box, i == b.equipPage)

		_, capH := r.MeasureText(page.label, equipTextScale)
		r.DrawText(at+equipTabPad, y+(equipTabsH-capH)/2, page.label, equipTextScale, ui2d.ColorText)

		if b.ctx.InvisibleButtonAt("hud_equip_tab_"+page.label, box.X, box.Y, box.W, box.H) {
			b.equipPage = i
		}

		at += w
	}

	// The rest of the row is the shut tab's own art, so the strip runs the
	// width of the window rather than stopping where the labels do.
	if at < x+equipW {
		b.drawEquipTabFrame(ui2d.Rect{X: at, Y: y, W: x + equipW - at, H: equipTabsH}, false)
	}
}

// equipTabPad is the air either side of a tab's label.
const equipTabPad float32 = 12

// drawEquipTabFrame draws one tab from its three slices.
//
// tab_a_* is the open one and tab_* a shut one, each four pixels of cap and a
// single column of middle to stretch between them.
func (b *UI2DBackend) drawEquipTabFrame(box ui2d.Rect, open bool) {
	left, mid, right := "tab_l.bmp", "tab_m.bmp", "tab_r.bmp"
	if open {
		left, mid, right = "tab_a_l.bmp", "tab_a_m.bmp", "tab_a_r.bmp"
	}

	const cap = float32(4)

	r := b.ctx.Renderer()

	if tex, err := b.texCache.Load(basicInterfacePath + left); err == nil {
		r.DrawImage(tex.ID, box.X, box.Y, cap, box.H, ui2d.ColorWhite)
	}

	if tex, err := b.texCache.Load(basicInterfacePath + mid); err == nil {
		r.DrawImage(tex.ID, box.X+cap, box.Y, box.W-2*cap, box.H, ui2d.ColorWhite)
	}

	if tex, err := b.texCache.Load(basicInterfacePath + right); err == nil {
		r.DrawImage(tex.ID, box.X+box.W-cap, box.Y, cap, box.H, ui2d.ColorWhite)
	}
}

// drawEquipPage draws whichever tab is open.
func (b *UI2DBackend) drawEquipPage(state InGameUIState, x, y float32) {
	r := b.ctx.Renderer()

	page := equipPages[b.equipPage]

	// The body first, so a page shorter than the window's own page area does
	// not leave the map showing through below it.
	r.DrawRect(x, y, equipW, equipPageH, ui2d.ColorWindowBody)

	if tex, err := b.texCache.Load(basicInterfacePath + page.art); err == nil {
		h := equipPageH
		if b.equipPage != 0 {
			h = equipSpecialH
		}

		r.DrawImage(tex.ID, x, y, equipW, h, ui2d.ColorWhite)
	}

	if b.equipPage != 0 {
		return
	}

	b.drawEquipPortrait(state, x, y)

	for _, place := range equipLayout {
		b.drawEquipSlot(state, place.slot, equipSlotRect(x, y, place.right, place.row))
	}
}

// drawEquipPortrait draws the character between the two columns of slots.
//
// Fitted by height alone, and centered on the window rather than on the strip
// the art paints: the original lets the character overlap the labels either
// side.
//
// What is fitted is the character's own art, cut out of the frame it was baked
// into. That frame is padded to the widest and tallest the sheet holds — a
// swing with a weapon in it, a hat that reaches above the head — so fitting
// the whole frame fits mostly empty space and leaves the character a fraction
// of the room it has.
func (b *UI2DBackend) drawEquipPortrait(state InGameUIState, x, y float32) {
	if state.Portrait == 0 || state.PortraitH <= 0 {
		return
	}

	// Down to the last row of slots, where the art paints the shadow the
	// character stands on.
	const stand = equipRow0Y + 4*equipRowPitch

	scale := stand / state.PortraitH
	w, h := state.PortraitW*scale, state.PortraitH*scale

	b.ctx.Renderer().DrawImageUV(state.Portrait,
		x+(equipW-w)/2, y+stand-h, w, h,
		state.PortraitU0, state.PortraitV0, state.PortraitU1, state.PortraitV1,
		ui2d.ColorWhite)
}

// drawEquipDivider draws the strip between the slots and the status block,
// which carries the equipment switch.
func (b *UI2DBackend) drawEquipDivider(state InGameUIState, x, y float32) {
	r := b.ctx.Renderer()

	if tex, err := b.texCache.Load(basicInterfacePath + equipDividerFile); err == nil {
		r.DrawImage(tex.ID, x, y, equipW, equipDividerH, ui2d.ColorWhite)
	} else {
		r.DrawRect(x, y, equipW, equipDividerH, ui2d.ColorWindowBody)
	}

	_, capH := r.MeasureText("Status", equipTextScale)
	textY := y + (equipDividerH-capH)/2

	r.DrawText(x+equipTabPad, textY, "Status", equipTextScale, ui2d.ColorText)

	b.drawShowEquipment(state, x, y, textY)
}

// drawShowEquipment draws the checkbox that lets other players look at what
// this character is wearing.
//
// The state shown is the server's, not one remembered here: it is part of the
// character, and a box that answered to the click rather than to the server
// would go out of step the first time the server refused.
func (b *UI2DBackend) drawShowEquipment(state InGameUIState, x, y, textY float32) {
	r := b.ctx.Renderer()

	const label = "Show Equipment"

	width, _ := r.MeasureText(label, equipTextScale)

	box := ui2d.Rect{
		X: x + equipW - equipTabPad - equipCheckW,
		Y: y + (equipDividerH-equipCheckW)/2,
		W: equipCheckW,
		H: equipCheckW,
	}

	art := equipCheckOff
	if state.ShowEquipment {
		art = equipCheckOn
	}

	if tex, err := b.texCache.Load(art); err == nil {
		r.DrawImage(tex.ID, box.X, box.Y, box.W, box.H, ui2d.ColorWhite)
	}

	r.DrawText(box.X-equipCheckGap-width, textY, label, equipTextScale, ui2d.ColorText)

	if b.ctx.InvisibleButtonAt("hud_equip_show", box.X, box.Y, box.W, box.H) {
		want := !state.ShowEquipment
		b.showEquip = &want
	}
}

// TakeShowEquipAction returns a click on the equipment switch and clears it.
func (b *UI2DBackend) TakeShowEquipAction() (bool, bool) {
	if b.showEquip == nil {
		return false, false
	}

	want := *b.showEquip
	b.showEquip = nil

	return want, true
}

// The checkbox art and its spacing.
const (
	equipCheckOff = basicInterfacePath + `rodexsystem\renewal\checkbox_off.bmp`
	equipCheckOn  = basicInterfacePath + `rodexsystem\renewal\checkbox_on.bmp`

	equipCheckW   float32 = 13
	equipCheckGap float32 = 5
)

// drawEquipStats draws the status block at the foot of the window.
func (b *UI2DBackend) drawEquipStats(state InGameUIState, x, y float32) {
	r := b.ctx.Renderer()

	if tex, err := b.texCache.Load(statsTexture); err == nil {
		r.DrawImage(tex.ID, x, y, statsW, statsH, ui2d.ColorWhite)
	} else {
		r.DrawRect(x, y, statsW, statsH, ui2d.ColorWindowBody)
	}

	b.drawStatValues(state, x, y)
}

// equipSlotRect is where one slot's icon goes, in screen coordinates.
//
// The icon is wider and taller than the oval, centered on it, and the bottom
// row is nudged up so the last icon stays inside the window: the oval there
// sits 3 pixels from the bitmap's edge and a 24-pixel icon centered on it
// would hang over.
func equipSlotRect(x, pageY float32, right bool, row int) ui2d.Rect {
	left := x + equipSlotLeftX
	if right {
		left = x + equipSlotRightX
	}

	top := pageY + equipRow0Y + float32(row)*equipRowPitch

	icon := ui2d.Rect{
		X: left + (equipSlotW-equipIcon)/2,
		Y: top + (equipSlotH-equipIcon)/2,
		W: equipIcon,
		H: equipIcon,
	}

	if over := icon.Y + icon.H - (pageY + equipPageH); over > 0 {
		icon.Y -= over
	}

	if under := pageY - icon.Y; under > 0 {
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

	// The slots are on the first page only, below the title bar and the tabs.
	if b.equipPage != 0 {
		return 0, false
	}

	pageY := rect.Y + ui2d.FrameTitleH + equipTabsH

	for _, place := range equipLayout {
		if equipSlotRect(rect.X, pageY, place.right, place.row).Contains(px, py) {
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
