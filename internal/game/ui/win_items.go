package ui

import (
	"strconv"

	"github.com/Faultbox/midgard-ro/internal/engine/ui2d"
	"github.com/Faultbox/midgard-ro/internal/game/items"
	"github.com/Faultbox/midgard-ro/internal/game/skills"
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

	// itemsScrollW is the bar down the right when there are more rows than
	// fit. Six columns of thirty-six leave twenty-two pixels between the last
	// cell and the window's edge, which is where it goes.
	itemsScrollW float32 = 14

	// The slot mark inside a cell: a flat oval, inset from the cell's sides
	// and shorter than it is wide.
	itemsSlotInset float32 = 4
	itemsSlotH     float32 = 14

	// The strip of tabs down the left, measured off tab_itm_01.bmp — the art
	// the archive ships for this very window. Twenty wide, so its borders
	// land on whole pixels the way they do there.
	itemsTabW     float32 = 20
	itemsTabScale float32 = 0.45

	// itemsBodyH is the height the grid and the tab strip share, between the
	// title bar and the footer.
	itemsBodyH = itemsH - ui2d.FrameTitleH - itemsFooterH - 2*itemsPad

	// Where the edges fall inside those twenty. The open tab reaches two
	// pixels further left than the shut ones and has no right edge at all, so
	// it runs into the grid — which is how the strip says which one is open.
	itemsTabOpenX float32 = 3
	itemsTabShutX float32 = 5
	itemsTabShutR float32 = 18

	// itemsTabSlant is the shape the tabs actually have. The boundary between
	// two of them is not level but slanted: in the art it falls from the left
	// edge to the right over five rows. Drawn square instead — which is what
	// this did before — the strip reads as three stacked boxes rather than as
	// tabs.
	itemsTabSlant float32 = 5

	// itemsTabSlantX is where a slant begins: at the shut edge, which is the
	// innermost of the two a boundary divides, since only the open tab reaches
	// further out.
	//
	// The art begins it at the outermost instead and turns the corner over a
	// single row. At the height the original draws a tab that reads as a
	// corner; at the height this one does, the same two pixels read as a spur
	// running past the tab below and into the band outside it.
	itemsTabSlantX = itemsTabShutX

	itemsFooterH float32 = 24

	itemsTextScale float32 = 0.65
)

// itemIconPath is where the archive keeps item icons, named in its own
// Korean: the resource table maps an id to one of these.
//
// Under the interface folder, beside the skill icons — not data\texture\item,
// which holds fifteen files and none of them these.
const itemIconPath = skinBasePath + `item\`

// itemsDragIconH is the size of the icon that follows the pointer during a
// drag, and itemsDragTint fades it so what is underneath still reads.
const itemsDragIconH = float32(24)

var itemsDragTint = ui2d.Color{R: 1, G: 1, B: 1, A: 0.75}

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
	b.ctx.Renderer().DrawRect(x, bodyY, itemsW, itemsH-ui2d.FrameTitleH, ui2d.ColorWindowBody)

	b.itemHover = itemHover{}

	b.drawItemTabs(x, bodyY)
	b.drawItemGrid(state, x, bodyY)
	b.drawItemsFooter(state, x, y)
	b.drawItemHover(screenW, screenH)
	b.ctx.EndWindow()
}

// itemHover is the cell the pointer is over, waiting to be named once the
// grid has finished drawing.
//
// Deferred rather than drawn from the cell: a name is wider than the cell it
// belongs to and would go under the cells drawn after it.
type itemHover struct {
	text string

	// x is the middle of the cell and y its bottom, which is where the name
	// hangs from — under the item, as the world labels hang under a monster.
	x, y float32
}

// drawItemHover names the item under the pointer, in the same plate the world
// uses for a monster or an item on the ground.
func (b *UI2DBackend) drawItemHover(screenW, screenH float32) {
	if b.itemHover.text == "" {
		return
	}

	r := b.ctx.Renderer()
	width, height := r.MeasureText(b.itemHover.text, itemLabelScale)

	// Kept on screen: the inventory can be dragged against either edge, and a
	// name centered on a cell in the far column would otherwise run off it.
	x := min(max(b.itemHover.x, width/2+itemLabelPadX), screenW-width/2-itemLabelPadX)

	// Above the cell instead when there is no room below, so the name is
	// never clipped by the bottom of the screen.
	y := b.itemHover.y + itemLabelDrop
	if y+height+itemLabelPadY > screenH {
		y = b.itemHover.y - itemsCell - itemLabelDrop - height
	}

	b.drawNamePlate(b.itemHover.text, x, y)
}

// drawItemTabs draws the three tabs down the left edge.
//
// The strip is drawn rather than blitted because the art the archive ships for
// it, tab_itm_01 to 03, has its labels baked in and in Korean. What is taken
// from that art is its shape, which is all flat colors and one-pixel edges:
// the open tab's face is the window body, the shut ones are a shade of grey,
// and every boundary between them is a slanted line rather than a straight
// one.
//
// Each tab is one quad with slanted top and bottom edges, not a stack of
// one-pixel rows. A slant built out of rows is a staircase whose steps are as
// coarse as the rows are wide — three pixels here, which is what it looked
// like — where a quad is rasterized at the display's own resolution.
func (b *UI2DBackend) drawItemTabs(x, bodyY float32) {
	top := bodyY + itemsPad

	for i, tab := range itemTabs {
		b.drawItemTab(x, top, i, tab.label)
	}

	// The edges last, over the faces they divide, so a face never covers half
	// the line above it.
	for k := 1; k <= len(itemTabs); k++ {
		b.drawItemTabEdge(x, top, k)
	}

	for i, tab := range itemTabs {
		box := b.itemTabBox(x, top, i)

		if b.ctx.InvisibleButtonAt("hud_item_tab_"+tab.label, box.X, box.Y, box.W, box.H) {
			b.itemTab = i
			b.itemScroll = 0
		}
	}
}

// itemsTabHeight is how tall one tab is.
//
// Shared out of the height the strip has rather than fixed, so the run of
// them ends where the grid does. Fixed at sixty-two they came to twelve
// pixels more than there was room for, and the last tab and the slant closing
// it hung over the line above the footer.
//
// The slant is taken off the top because it belongs to the strip too: the
// closing one starts where the last tab ends and needs its own rows below.
func itemsTabHeight() float32 {
	return (itemsBodyH - itemsTabSlant) / float32(len(itemTabs))
}

// itemTabBox is the band one tab is pressed in. Square, unlike the tab drawn
// in it: a slanted hit area would leave slivers between the tabs that answer
// to nothing.
func (b *UI2DBackend) itemTabBox(x, top float32, i int) ui2d.Rect {
	h := itemsTabHeight()

	return ui2d.Rect{X: x, Y: top + float32(i)*h, W: itemsTabW, H: h}
}

// itemTabLeft is where a tab's left edge falls. The open one reaches two
// pixels further out than the shut ones, which is half of how the strip says
// which is open; the other half is that it has no right edge at all.
func (b *UI2DBackend) itemTabLeft(i int) float32 {
	if i == b.itemTab {
		return itemsTabOpenX
	}

	return itemsTabShutX
}

// itemTabRight is where it ends. The open tab runs into the grid.
func (b *UI2DBackend) itemTabRight(i int) float32 {
	if i == b.itemTab {
		return itemsTabW
	}

	return itemsTabShutR
}

// itemTabEdgeY is where boundary k sits at a given x.
//
// Boundary zero is the top of the strip and is level; every other one slants,
// starting at the leftmost edge either of its two tabs has and reaching the
// far side of the strip after itemsTabSlant rows.
func (b *UI2DBackend) itemTabEdgeY(top float32, k int, x float32) float32 {
	y := top + float32(k)*itemsTabHeight()
	if k == 0 {
		return y
	}

	switch {
	case x <= itemsTabSlantX:
		return y
	case x >= itemsTabShutR:
		return y + itemsTabSlant
	default:
		return y + itemsTabSlant*(x-itemsTabSlantX)/(itemsTabShutR-itemsTabSlantX)
	}
}

// drawItemTab fills one tab, hatches the band outside it and draws its
// upright edges.
func (b *UI2DBackend) drawItemTab(x, top float32, i int, label string) {
	r := b.ctx.Renderer()

	left, right := b.itemTabLeft(i), b.itemTabRight(i)

	face := itemsTabIdle
	if i == b.itemTab {
		face = ui2d.ColorWindowBody
	}

	// The face, slanted top and bottom.
	r.DrawQuad(
		[2]float32{x + left, b.itemTabEdgeY(top, i, left)},
		[2]float32{x + right, b.itemTabEdgeY(top, i, right)},
		[2]float32{x + right, b.itemTabEdgeY(top, i+1, right)},
		[2]float32{x + left, b.itemTabEdgeY(top, i+1, left)},
		face)

	// The band outside it, and beyond a shut tab the window itself. Both are
	// square: the slant starts at the tab's own edge, so nothing outside it
	// is cut by one.
	bandTop := b.itemTabEdgeY(top, i, 0)
	bandH := b.itemTabEdgeY(top, i+1, 0) - bandTop

	b.drawTabHatch(x, bandTop, left, bandH)

	if i != b.itemTab {
		r.DrawRect(x+itemsTabShutR+1, bandTop, itemsTabW-itemsTabShutR-1, bandH, ui2d.ColorWindowBody)
	}

	// The upright edges, each running between the two slants.
	b.drawItemTabUpright(x, top, i, left)

	if i != b.itemTab {
		b.drawItemTabUpright(x, top, i, itemsTabShutR)
	}

	b.drawTabLabel(b.itemTabBox(x, top, i), label, i == b.itemTab)
}

// drawItemTabUpright draws one of a tab's vertical edges, between the slants
// above and below it.
func (b *UI2DBackend) drawItemTabUpright(x, top float32, i int, at float32) {
	edgeTop := b.itemTabEdgeY(top, i, at)
	edgeBottom := b.itemTabEdgeY(top, i+1, at)

	b.ctx.Renderer().DrawRect(x+at, edgeTop, 1, edgeBottom-edgeTop, itemsTabBorder)
}

// drawItemTabEdge draws the slanted line at one boundary, and the step that
// joins the upright edges either side of it.
func (b *UI2DBackend) drawItemTabEdge(x, top float32, k int) {
	r := b.ctx.Renderer()

	left := itemsTabSlantX

	startY := b.itemTabEdgeY(top, k, left)
	endY := b.itemTabEdgeY(top, k, itemsTabShutR)

	// The two tabs a boundary divides do not have their upright edges in the
	// same place — the open one reaches two pixels further out — so the line
	// has to step across to meet the next one. Without it the edge simply
	// stops and starts again further in, and the corner reads as open.
	b.drawItemTabStep(x, startY, k)

	r.DrawQuad(
		[2]float32{x + left, startY},
		[2]float32{x + itemsTabShutR + 1, endY},
		[2]float32{x + itemsTabShutR + 1, endY + 1},
		[2]float32{x + left, startY + 1},
		itemsTabBorder)
}

// drawItemTabStep joins the upright edge above a boundary to the one below it.
func (b *UI2DBackend) drawItemTabStep(x, y float32, k int) {
	above := b.itemTabLeft(k - 1)

	below := above
	if k < len(itemTabs) {
		below = b.itemTabLeft(k)
	}

	if above == below {
		return
	}

	from, to := min(above, below), max(above, below)

	b.ctx.Renderer().DrawRect(x+from, y, to-from+1, 1, itemsTabBorder)
}

// drawTabHatch draws the band down the outside of the strip.
//
// Rows of the body color and of the shut grey in turn, which is how the art
// fills the sliver beside the tabs.
func (b *UI2DBackend) drawTabHatch(x, y, w, h float32) {
	if w <= 0 || h <= 0 {
		return
	}

	r := b.ctx.Renderer()

	for row := float32(0); row < h; row++ {
		shade := ui2d.ColorWindowBody
		if int(row)%2 == 0 {
			shade = itemsTabIdle
		}

		r.DrawRect(x, y+row, w, 1, shade)
	}
}

// drawTabLabel writes a label down a narrow tab, turned on its side.
//
// Turned rather than stacked a letter to a line, which is what this did
// before. The archive's own tab strips carry their words a quarter turn round
// — tab_cmd_03 reads WIN, EXE and ON/OFF that way up the side of the window —
// and a column of upright letters reads as letters rather than as a word.
//
// The measured width and height swap round once the line is turned, so
// centering it in the tab is the ordinary arithmetic with the two exchanged.
func (b *UI2DBackend) drawTabLabel(box ui2d.Rect, label string, open bool) {
	r := b.ctx.Renderer()

	width, height := r.MeasureText(label, itemsTabScale)

	// Darker on the open tab than on the shut ones, as the art has it.
	color := itemsTabTextShut
	if open {
		color = itemsTabTextOpen
	}

	r.DrawTextVertical(
		box.X+(box.W-height)/2,
		box.Y+(box.H+width)/2,
		label, itemsTabScale, color)
}

// drawItemGrid draws the cells and whatever is in them, scrolled to wherever
// the bar is.
//
// A bag holds far more than the two dozen cells that fit, and until this the
// window drew the first twenty-four and nothing else: an item past them was
// carried, weighed, and could not be reached.
func (b *UI2DBackend) drawItemGrid(state InGameUIState, x, bodyY float32) {
	gridX := x + itemsTabW + itemsPad
	gridY := bodyY + itemsPad
	gridH := itemsH - ui2d.FrameTitleH - itemsFooterH - 2*itemsPad
	gridW := itemsCols*(itemsCell+itemsCellGap) - itemsCellGap

	rows := max(int(gridH/(itemsCell+itemsCellGap)), 1)

	shown := b.itemsOnTab(state)

	// Scrolled by the row rather than by the item: the cells are a grid, and
	// a bag that moved one item at a time would shuffle everything sideways
	// under the pointer.
	filled := itemRowsFor(len(shown))
	maxOffset := max(0, filled-rows)

	offset := min(b.itemScroll, maxOffset)

	if maxOffset > 0 {
		b.itemScroll = b.scrollbar("hud_items",
			x+itemsW-itemsPad-itemsScrollW, gridY, gridH, offset, maxOffset, rows)
		offset = b.itemScroll
	} else {
		// Nothing to scroll: the bar goes, and so does any offset left over
		// from when there was more in the bag.
		b.itemScroll = 0
		offset = 0
	}

	for i := 0; i < rows*itemsCols; i++ {
		col := float32(i % itemsCols)
		row := float32(i / itemsCols)

		cell := ui2d.Rect{
			X: gridX + col*(itemsCell+itemsCellGap),
			Y: gridY + row*(itemsCell+itemsCellGap),
			W: itemsCell,
			H: itemsCell,
		}

		b.drawItemCell(cell, shown, offset*itemsCols+i)
	}

	b.scrollItems(gridX, gridY, gridW, gridH, filled, rows)
}

// itemRowsFor is how many rows a bag fills, counting a part-filled last row:
// twenty-five items in sixes are five rows, not four.
func itemRowsFor(count int) int {
	return (count + itemsCols - 1) / itemsCols
}

// scrollItems moves the grid under the wheel.
//
// Over the grid rather than over the window, the same way the skill list does
// it: the wheel belongs to what is under it, and a bag that scrolled while the
// pointer was on the tabs would move under a tab being aimed at.
func (b *UI2DBackend) scrollItems(gridX, gridY, gridW, gridH float32, filled, rows int) {
	in := b.ctx.Input()
	if in == nil || in.ScrollY == 0 {
		return
	}

	if !(ui2d.Rect{X: gridX, Y: gridY, W: gridW, H: gridH}).Contains(in.MouseX, in.MouseY) {
		return
	}

	maxOffset := max(0, filled-rows)
	b.itemScroll = min(max(b.itemScroll-int(in.ScrollY), 0), maxOffset)
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

	// The name, once the grid is done: what is in a cell is an icon and a
	// count, and neither says what the thing is called.
	//
	// Not while dragging — the icon is already under the pointer then, and a
	// name that followed it would be naming the cell it happens to be over
	// rather than what is being carried.
	if in := b.ctx.Input(); !b.itemDrag.active && cell.Contains(in.MouseX, in.MouseY) {
		b.itemHover = itemHover{
			text: items.Name(item.ID),
			x:    cell.X + cell.W/2,
			y:    cell.Y + cell.H,
		}
	}

	// Double click uses it, or wears it, depending on which tab it is on.
	// Single clicks do nothing yet: selecting is what a single click means in
	// the original, and there is nothing to select for.
	if b.ctx.DoubleClickedIn("hud_item_cell_"+strconv.Itoa(index), cell) {
		b.itemAction = ItemAction{Index: item.Index, Equip: itemTabs[b.itemTab].category == items.CategoryEquip}
	}

	// A right click opens the menu on it, which is where asking what an item
	// is lives. The double click that uses or wears one is still there; this
	// is the rest of what a cell can be asked.
	if in := b.ctx.Input(); in.MouseRightPressed && cell.Contains(in.MouseX, in.MouseY) {
		b.openItemMenu(item, itemTabs[b.itemTab].category == items.CategoryEquip,
			in.MouseX, in.MouseY)
	}

	// Pressing on a cell begins a drag. Whether it becomes a drop is decided
	// on release, in drawItemsWindow, by where the pointer ended up — a press
	// that goes nowhere is also how a double click starts.
	if in := b.ctx.Input(); in.MouseLeftPressed && !b.itemDrag.active &&
		cell.Contains(in.MouseX, in.MouseY) {
		b.itemDrag = itemDrag{
			active: true,
			index:  item.Index,
			itemID: item.ID,
			count:  item.Count,
		}
	}
}

// drawDraggedItem draws the icon under the pointer while a drag is in
// progress, so there is something to aim with.
//
// Drawn last in the frame rather than from the window it started in: a
// shortcut dragged off the quick panel has no window behind it, and one
// dragged out of the inventory should ride over that window rather than under
// it.
func (b *UI2DBackend) drawDraggedItem() {
	path, dragging := b.draggedIconPath()
	if !dragging {
		return
	}

	tex, err := b.texCache.Load(path)
	if err != nil {
		return
	}

	in := b.ctx.Input()
	b.ctx.Renderer().DrawImage(tex.ID,
		in.MouseX-itemsDragIconH/2, in.MouseY-itemsDragIconH/2,
		itemsDragIconH, itemsDragIconH, itemsDragTint)
}

// draggedIconPath is the art for whatever is under the pointer, if anything
// is being dragged.
//
// One place for all three kinds of drag — an item out of the bag or off the
// body, a cell moved along the bar, a skill out of its window — because the
// icon that follows the pointer is the same affordance in each case and the
// only difference is which table the art comes from.
func (b *UI2DBackend) draggedIconPath() (string, bool) {
	switch {
	case b.itemDrag.active:
		return itemIconFor(b.itemDrag.itemID)
	case b.skillDrag.active:
		return skillIconFor(b.skillDrag.skill)
	case b.hotkeyDrag.active:
		if b.hotkeyDrag.cell.skill {
			return skillIconFor(uint16(b.hotkeyDrag.cell.id))
		}

		return itemIconFor(b.hotkeyDrag.cell.id)
	}

	return "", false
}

// itemIconFor and skillIconFor are where each kind of art lives, empty when
// the archive has none — which is most of what has been added recently.
func itemIconFor(itemID uint32) (string, bool) {
	info, ok := items.Lookup(itemID)
	if !ok || info.Resource == "" {
		return "", false
	}

	return itemIconPath + info.Resource + ".bmp", true
}

func skillIconFor(skillID uint16) (string, bool) {
	sprite := skills.Sprite(skillID)
	if sprite == "" {
		return "", false
	}

	return skillIconPath + sprite + ".bmp", true
}

// finishItemDrag decides what a released drag meant.
//
// A quick-panel cell takes it as a shortcut, which is not a drop: nothing
// leaves the bag and nothing is asked about an amount, because the cell holds
// the item's identity rather than any of the item. Released anywhere else
// outside the window it is a drop; released inside the window it is nothing.
//
// There is no rearranging within the grid to confuse the last case with,
// because the server decides which slot an item sits in.
func (b *UI2DBackend) finishItemDrag() {
	if !b.itemDrag.active {
		return
	}

	in := b.ctx.Input()
	if in.MouseLeftDown {
		return
	}

	dragged := b.itemDrag
	b.itemDrag = itemDrag{}

	// A slot of the equipment window: wear it there. The slot is passed on
	// rather than left to the server, so dropping a ring on the left hand
	// means the left hand and not whichever one the server would have picked.
	if slot, ok := b.equipSlotAt(in.MouseX, in.MouseY); ok {
		if !dragged.fromEquip {
			b.itemAction = ItemAction{Index: dragged.index, Equip: true, Mask: slot}
		}

		return
	}

	// Coming off the body: anywhere but back onto the body takes it off. The
	// original is no stricter than this — there is nowhere else a worn item
	// can go, and it is never dropped on the ground by dragging it out.
	if dragged.fromEquip {
		if window, ok := b.equipWindowRect(); !ok || !window.Contains(in.MouseX, in.MouseY) {
			b.itemAction = ItemAction{Index: dragged.index, Unequip: true}
		}

		return
	}

	if row, col, ok := b.hotkeyCellAt(in.MouseX, in.MouseY); ok {
		b.AssignHotkey(row, col, hotkeyCell{id: dragged.itemID})

		return
	}

	if window, ok := b.itemsWindowRect(); ok && window.Contains(in.MouseX, in.MouseY) {
		return
	}

	// A stack asks how many; a single item just goes. Asking about a stack of
	// one would be a dialog with one answer.
	if dragged.count > 1 {
		b.beginDropPrompt(dragged.index, dragged.itemID, dragged.count)

		return
	}

	b.dropAction = DropAction{Index: dragged.index, Amount: 1}
}

// itemsWindowRect is the inventory window, when it is open.
func (b *UI2DBackend) itemsWindowRect() (ui2d.Rect, bool) {
	if !b.IsWindowOpen(WindowItem) {
		return ui2d.Rect{}, false
	}

	return b.ctx.WindowRect(itemsWindowID)
}

// itemDrag is an inventory drag in progress.
type itemDrag struct {
	active bool
	index  int
	itemID uint32
	count  int

	// fromEquip marks a drag that started on the body rather than in the bag,
	// and slot which place it was worn in. Both directions use the same drag
	// so that one release can be resolved in one place; without knowing where
	// it started, a release over the inventory cannot tell "put this back" from
	// "you dropped it where it already was".
	fromEquip bool
	slot      uint32
}

// DropAction is an item dragged out of the window, waiting to be sent.
type DropAction struct {
	// Index is the inventory slot, as the server names it.
	Index int

	// Amount is how many to drop.
	Amount int
}

// TakeDropAction returns a completed drag-out and clears it.
func (b *UI2DBackend) TakeDropAction() (DropAction, bool) {
	action := b.dropAction
	if action.Amount == 0 {
		return DropAction{}, false
	}

	b.dropAction = DropAction{}

	return action, true
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

// ItemAction is something asked of one item, waiting to be sent.
type ItemAction struct {
	// Index is the inventory slot, which is how the server names an item.
	Index int

	// Equip is set when the double click was on the equipment tab, where
	// using an item means wearing it.
	Equip bool

	// Unequip is set for taking something off, which is what a double click
	// in the equipment window means.
	Unequip bool

	// Mask narrows where on the body it should go, for an item dropped on a
	// particular slot. Zero leaves the choice to the server, which is what a
	// double click means: it names no slot.
	Mask uint32
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
	// itemsCellBg is the mark under a slot, and itemsTabIdle a tab that is
	// not open.
	//
	// The slot is a shadow for an item to sit on, not a plate behind it.
	// Given any weight the grid reads before its contents do — the eye lands
	// on rows of ovals and finds the items afterwards, which is backwards.
	// The slot mark: a blue-grey, not the neutral grey the rest of the panel
	// uses. It has to read as blue against a white body or the grid looks
	// like smudges rather than slots.
	itemsCellBg = ui2d.Color{R: 0.88, G: 0.91, B: 0.97, A: 1}
	// The strip's own greys, taken from tab_itm_01.bmp: f2f2f2 for a shut
	// tab's face, c0c0c0 for every edge, and 404040 and 767676 for the label
	// on the open tab and on a shut one.
	itemsTabIdle     = ui2d.Color{R: 0.949, G: 0.949, B: 0.949, A: 1}
	itemsTabBorder   = ui2d.Color{R: 0.753, G: 0.753, B: 0.753, A: 1}
	itemsTabTextOpen = ui2d.Color{R: 0.251, G: 0.251, B: 0.251, A: 1}
	itemsTabTextShut = ui2d.Color{R: 0.463, G: 0.463, B: 0.463, A: 1}
	itemsCountText   = ui2d.Color{R: 0.1, G: 0.1, B: 0.15, A: 1}
)
