package ui

import (
	"fmt"
	"strconv"

	"github.com/Faultbox/midgard-ro/internal/engine/ui2d"
	"github.com/Faultbox/midgard-ro/internal/game/items"
)

// The "how many?" dialog, for dragging part of a stack out of the inventory.
//
// A single item goes straight out with no dialog, which is what makes the
// dialog bearable: it only appears when there is a decision to make.
const (
	dropQtyWindowID = "hud_drop_qty"

	dropQtyW = float32(210)
	dropQtyH = float32(110)

	dropQtyPad    = float32(10)
	dropQtyRowH   = float32(22)
	dropQtyBtnW   = float32(84)
	dropQtyFieldH = float32(22)
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
}

// beginDropPrompt asks how many to drop.
func (b *UI2DBackend) beginDropPrompt(index int, itemID uint32, count int) {
	b.dropPrompt = dropPrompt{
		open:   true,
		index:  index,
		itemID: itemID,
		max:    count,

		// One, matching the original. A stack dropped whole by accident is a
		// worse mistake than one dropped singly on purpose.
		text: "1",
	}

	// The window may have been closed by its own button last time, which is
	// remembered; without this the dialog would open invisible.
	b.ctx.OpenWindow(dropQtyWindowID)
}

// cancelDropPrompt puts the dialog away without dropping anything.
func (b *UI2DBackend) cancelDropPrompt() {
	b.dropPrompt = dropPrompt{}
}

// commitDropPrompt turns what was typed into a drop.
//
// An unreadable or out-of-range amount is clamped rather than refused. The
// field is small and the stack size is on screen beside it, so there is
// nothing to explain that clamping does not say more quickly.
func (b *UI2DBackend) commitDropPrompt() {
	amount, err := strconv.Atoi(b.dropPrompt.text)
	if err != nil || amount < 1 {
		amount = 1
	}
	if amount > b.dropPrompt.max {
		amount = b.dropPrompt.max
	}

	b.dropAction = DropAction{Index: b.dropPrompt.index, Amount: amount}
	b.dropPrompt = dropPrompt{}
}

// drawDropQuantity draws the dialog and acts on it.
func (b *UI2DBackend) drawDropQuantity(screenW, screenH float32) {
	if !b.dropPrompt.open {
		return
	}

	openX := (screenW - dropQtyW) / 2
	openY := (screenH - dropQtyH) / 2

	if !b.ctx.BeginWindow(dropQtyWindowID, openX, openY, dropQtyW, dropQtyH, "Drop item") {
		if b.ctx.WindowClosed(dropQtyWindowID) {
			b.cancelDropPrompt()
		}

		return
	}

	// Read back after BeginWindow, or the contents trail the frame while it
	// is being dragged.
	x, y := openX, openY
	if rect, ok := b.ctx.WindowRect(dropQtyWindowID); ok {
		x, y = rect.X, rect.Y
	}

	b.ctx.CaptureMouse(ui2d.Rect{X: x, Y: y, W: dropQtyW, H: dropQtyH})

	r := b.ctx.Renderer()
	r.DrawRect(x, y+ui2d.FrameTitleH, dropQtyW, dropQtyH-ui2d.FrameTitleH, ui2d.ColorWindowBody)

	top := y + ui2d.FrameTitleH + dropQtyPad

	// What is being dropped, and how many there are to drop from.
	name := items.Name(b.dropPrompt.itemID)
	caption := fmt.Sprintf("%s (%d)", name, b.dropPrompt.max)
	_, capH := r.MeasureText(caption, 1)
	r.DrawText(x+dropQtyPad, top, caption, 1, ui2d.ColorText)

	fieldY := top + capH + 6
	value, _, submitted := b.ctx.TextInputAt(dropQtyWindowID+"_amount",
		x+dropQtyPad, fieldY, dropQtyW-2*dropQtyPad, dropQtyFieldH, b.dropPrompt.text)
	b.dropPrompt.text = value

	btnY := fieldY + dropQtyFieldH + 8
	accepted := b.ctx.ButtonAt(dropQtyWindowID+"_ok",
		x+dropQtyPad, btnY, dropQtyBtnW, dropQtyRowH, "OK")
	cancelled := b.ctx.ButtonAt(dropQtyWindowID+"_cancel",
		x+dropQtyW-dropQtyPad-dropQtyBtnW, btnY, dropQtyBtnW, dropQtyRowH, "Cancel")

	b.ctx.EndWindow()

	switch {
	case cancelled:
		b.cancelDropPrompt()
	case accepted || submitted:
		b.commitDropPrompt()
	}
}
