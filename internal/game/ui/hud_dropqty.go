package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Faultbox/midgard-ro/internal/engine/ui2d"
	"github.com/Faultbox/midgard-ro/internal/game/items"
)

// The "how many?" dialog, for dragging part of a stack out of the inventory.
//
// One row: the amount, and OK. What is being dropped goes in the title bar
// rather than into the body, which keeps the dialog one row tall — at this
// size a caption line would double its height to repeat what the title says.
//
// No Cancel button. Closing it is canceling, and the title bar's close and a
// click anywhere outside both do that; a third way to say no would be the
// widest thing in the row.
const (
	dropQtyWindowID = "hud_drop_qty"

	dropQtyPad     = float32(6)
	dropQtyRowH    = float32(20)
	dropQtyOKW     = float32(44)
	dropQtySpinW   = float32(11)
	dropQtyGap     = float32(5)
	dropQtyTextPad = float32(8)

	// dropQtyMaxDigits bounds what can be typed. Six digits covers any stack
	// the server will hold, and stops the field growing past the window.
	dropQtyMaxDigits = 6

	// dropQtyNear is the gap between the inventory window and this one.
	dropQtyNear = float32(6)
)

// dropPrompt is a drag-out that is waiting to be told an amount.
type dropPrompt struct {
	open   bool
	index  int
	itemID uint32

	// max is the size of the stack, which is the most that can be dropped.
	max int

	// text is what has been typed, kept as text rather than a number so a
	// half-deleted field does not snap back to a value nobody entered.
	text string

	// opened marks the frame the dialog appeared on, so the click that
	// finished the drag is not also read as a click outside it.
	opened bool

	// forShop marks the one a counter asked for. The same dialog serves
	// both, but what happens on OK is not the same thing and neither is what
	// it opens with: a stack dropped whole by accident is a worse mistake
	// than one dropped singly, and a stack half sold is a nuisance.
	forShop bool

	// sell says which side of the counter the shop one belongs to.
	sell bool
}

// beginDropPrompt asks how many to drop.
func (b *UI2DBackend) beginDropPrompt(index int, itemID uint32, count int) {
	b.dropPrompt = dropPrompt{
		open:   true,
		index:  index,
		itemID: itemID,
		max:    count,
		opened: true,

		// One, matching the original. A stack dropped whole by accident is a
		// worse mistake than one dropped singly on purpose.
		text: "1",
	}

	// The window may have been closed by its own button last time, which is
	// remembered; without this the dialog would open invisible.
	b.ctx.OpenWindow(dropQtyWindowID)
}

// beginShopPrompt asks how many to buy or sell.
//
// Selling opens with the whole stack: something is being carried out of the
// bag and all of it is what is usually meant, so the number is there to be
// cut down rather than typed out.
//
// Buying opens with one. There is no stack behind a shelf to mean "all of it"
// — the most is only the most the server will take in a line — and a list
// opening at that would put a hundred thousand zeny on the order for one
// click on a name.
func (b *UI2DBackend) beginShopPrompt(key int, itemID uint32, most int, sell bool) {
	if most <= 1 {
		b.addShopLine(key, 1)

		return
	}

	amount := "1"
	if sell {
		amount = strconv.Itoa(most)
	}

	b.dropPrompt = dropPrompt{
		open:    true,
		index:   key,
		itemID:  itemID,
		max:     most,
		opened:  true,
		forShop: true,
		sell:    sell,
		text:    amount,
	}

	if b.ctx != nil {
		b.ctx.OpenWindow(dropQtyWindowID)
	}
}

// cancelDropPrompt puts the dialog away without dropping anything.
func (b *UI2DBackend) cancelDropPrompt() {
	b.dropPrompt = dropPrompt{}
}

// dropPromptAmount is what has been typed, clamped to something droppable.
func (b *UI2DBackend) dropPromptAmount() int {
	amount, err := strconv.Atoi(b.dropPrompt.text)
	if err != nil || amount < 1 {
		return 1
	}
	if amount > b.dropPrompt.max {
		return b.dropPrompt.max
	}

	return amount
}

// commitDropPrompt turns what was typed into a drop, or into a line on an
// order — the same dialog asks for both.
func (b *UI2DBackend) commitDropPrompt() {
	amount := b.dropPromptAmount()

	if b.dropPrompt.forShop {
		b.addShopLine(b.dropPrompt.index, amount)
		b.dropPrompt = dropPrompt{}

		return
	}

	b.dropAction = DropAction{Index: b.dropPrompt.index, Amount: amount}
	b.dropPrompt = dropPrompt{}
}

// digitsOnly keeps a typed value to a number of at most six digits.
//
// Filtering as it is typed rather than validating on OK: a field that will not
// accept a letter says so more clearly than a message afterwards, and there is
// nothing sensible to do with "12a" anyway.
func digitsOnly(value string) string {
	var kept strings.Builder
	for _, r := range value {
		if r >= '0' && r <= '9' && kept.Len() < dropQtyMaxDigits {
			kept.WriteRune(r)
		}
	}

	return kept.String()
}

// dropQtyLayout is where the dialog sits and how wide it is.
//
// The width follows the widest number the field will take, measured rather
// than guessed, so the field fits six digits at whatever scale the text is
// drawn.
func (b *UI2DBackend) dropQtyLayout(screenW, screenH float32) (x, y, w, h, fieldW float32) {
	digitsW, _ := b.ctx.Renderer().MeasureText(strings.Repeat("0", dropQtyMaxDigits), 1)
	fieldW = digitsW + dropQtyTextPad

	w = dropQtyPad + fieldW + dropQtySpinW + dropQtyGap + dropQtyOKW + dropQtyPad
	h = ui2d.FrameTitleH + dropQtyPad + dropQtyRowH + dropQtyPad

	// Beside the window it came from, so the two are readable together and
	// the dialog never lands on the item you were looking at. Falls back to
	// the middle of the screen if that window is not up.
	//
	// At a counter that is the list rather than the bag. Both sides put the
	// thing being asked about into the list, and on the selling side the bag
	// is what it was just carried out of — anchoring to that would land the
	// dialog on the list every time.
	anchor := itemsWindowID
	if b.dropPrompt.forShop {
		anchor = shopCartWindowID
	}

	x, y = (screenW-w)/2, (screenH-h)/2
	if rect, ok := b.ctx.WindowRect(anchor); ok {
		x, y = rect.X+rect.W+dropQtyNear, rect.Y

		// Off the right edge: put it on the other side instead.
		if x+w > screenW {
			x = rect.X - w - dropQtyNear
		}
	}

	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}

	return x, y, w, h, fieldW
}

// drawDropQuantity draws the dialog and acts on it.
func (b *UI2DBackend) drawDropQuantity(screenW, screenH float32) {
	if !b.dropPrompt.open {
		return
	}

	openX, openY, w, h, fieldW := b.dropQtyLayout(screenW, screenH)

	// The title carries what is being dropped and how many there are, which
	// is the whole caption this dialog would otherwise need a row for.
	//
	// Buying, the count is left off: what bounds it is the most the server
	// will take in one line, not a number of things that exist anywhere, and
	// naming it beside a shopkeeper's item reads as the stock on the shelf.
	title := items.Name(b.dropPrompt.itemID)
	if !b.dropPrompt.forShop || b.dropPrompt.sell {
		title = fmt.Sprintf("%s (%d)", title, b.dropPrompt.max)
	}

	// Closable but not minimizable: a one-row dialog that can be rolled up to
	// its own title bar is a joke the interface does not need to tell.
	opts := ui2d.WindowOptions{Closable: true}

	if !b.ctx.BeginWindowEx(dropQtyWindowID, openX, openY, w, h, title, opts) {
		if b.ctx.WindowClosed(dropQtyWindowID) {
			b.cancelDropPrompt()
		}

		return
	}

	// Read back after BeginWindowEx, or the contents trail the frame while it
	// is being dragged.
	x, y := openX, openY
	if rect, ok := b.ctx.WindowRect(dropQtyWindowID); ok {
		x, y = rect.X, rect.Y
	}

	window := ui2d.Rect{X: x, Y: y, W: w, H: h}
	b.ctx.CaptureMouse(window)

	r := b.ctx.Renderer()
	r.DrawRect(x, y+ui2d.FrameTitleH, w, h-ui2d.FrameTitleH, ui2d.ColorWindowBody)

	rowY := y + ui2d.FrameTitleH + dropQtyPad
	fieldX := x + dropQtyPad

	value, _, submitted := b.ctx.TextInputAt(dropQtyWindowID+"_amount",
		fieldX, rowY, fieldW, dropQtyRowH, b.dropPrompt.text)
	b.dropPrompt.text = digitsOnly(value)

	b.drawDropSpinner(fieldX+fieldW, rowY)

	// The same button the ESC menu is built from, which is where this
	// interface's plain button lives.
	okBox := ui2d.Rect{
		X: x + w - dropQtyPad - dropQtyOKW,
		Y: rowY,
		W: dropQtyOKW,
		H: dropQtyRowH,
	}
	b.drawFlatButton(okBox, "OK", false)
	accepted := b.ctx.InvisibleButtonAt(dropQtyWindowID+"_ok",
		okBox.X, okBox.Y, okBox.W, okBox.H)

	b.ctx.EndWindow()

	switch {
	case accepted || submitted:
		b.commitDropPrompt()
	case !b.dropPrompt.opened:
		b.closeDropPromptOnClickOutside(window)
	}

	b.dropPrompt.opened = false
}

// drawDropSpinner puts the up and down buttons against the field's right edge.
func (b *UI2DBackend) drawDropSpinner(x, y float32) {
	half := dropQtyRowH / 2

	if b.ctx.InvisibleButtonAt(dropQtyWindowID+"_up", x, y, dropQtySpinW, half) {
		b.stepDropAmount(1)
	}
	if b.ctx.InvisibleButtonAt(dropQtyWindowID+"_down", x, y+half, dropQtySpinW, half) {
		b.stepDropAmount(-1)
	}

	r := b.ctx.Renderer()
	r.DrawRect(x, y, dropQtySpinW, dropQtyRowH, itemsCellBg)

	drawSpinArrow(r, x, y, dropQtySpinW, half, true)
	drawSpinArrow(r, x, y+half, dropQtySpinW, half, false)
}

// drawSpinArrow draws one arrowhead as stepped rows.
//
// Rows rather than a real triangle, the same way the slider's caps are drawn:
// the renderer draws rectangles, and at three pixels the steps read as the
// arrow they stand for.
//
// i counts outward from the point, so the narrow row goes at the top for an
// up arrow and at the bottom for a down one.
func drawSpinArrow(r *ui2d.Renderer, x, y, w, h float32, up bool) {
	const rows = 3

	top := y + (h-rows)/2
	for i := 0; i < rows; i++ {
		width := float32(i+1) * 2

		row := top + float32(i)
		if !up {
			row = top + float32(rows-1-i)
		}

		r.DrawRect(x+(w-width)/2, row, width, 1, ui2d.ColorText)
	}
}

// stepDropAmount nudges the amount by one, staying between one and the stack.
func (b *UI2DBackend) stepDropAmount(delta int) {
	amount := b.dropPromptAmount() + delta
	if amount < 1 {
		amount = 1
	}
	if amount > b.dropPrompt.max {
		amount = b.dropPrompt.max
	}

	b.dropPrompt.text = strconv.Itoa(amount)
}

// closeDropPromptOnClickOutside cancels the drop when the pointer goes
// elsewhere, which is the other half of having no Cancel button.
func (b *UI2DBackend) closeDropPromptOnClickOutside(window ui2d.Rect) {
	in := b.ctx.Input()
	if !in.MouseLeftPressed || window.Contains(in.MouseX, in.MouseY) {
		return
	}

	b.cancelDropPrompt()
}
