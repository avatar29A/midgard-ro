package ui

import (
	"strconv"

	"github.com/Faultbox/midgard-ro/internal/engine/ui2d"
)

// The hotkey bar along the bottom right, drawn from shortcut_bg.bmp.
//
// The strip is 280x29 and carries its own slots: ten cells, the first nine of
// which are the shortcuts and the tenth the row number. That split is not
// guesswork — roBrowser lays out nine containers and an index label over this
// same background, and the cells measured out of the bitmap agree with it.
const (
	hotkeyTexture = basicInterfacePath + "shortcut_bg.bmp"

	hotkeyBarW float32 = 280
	hotkeyBarH float32 = 29

	// Where the cells sit in the strip: the first interior starts at x=7 and
	// they repeat every 27, each 26 wide and 22 tall from y=4.
	hotkeyCellX     float32 = 7
	hotkeyCellY     float32 = 4
	hotkeyCellW     float32 = 26
	hotkeyCellH     float32 = 22
	hotkeyCellPitch float32 = 27

	// hotkeySlots is how many of those cells hold a shortcut. The last cell
	// is the row number instead.
	hotkeySlots = 9
)

// drawHotkeys puts the bar in the bottom-right corner, where the original
// keeps it — opposite the chat, which has the bottom left.
//
// The slots are empty. Nothing can be put in them until the skill and item
// windows exist to drag from, so this draws the bar and reserves the cells;
// what goes in them is a later step's problem.
func (b *UI2DBackend) drawHotkeys(screenW, screenH float32) {
	x := screenW - hotkeyBarW
	y := screenH - hotkeyBarH

	r := b.ctx.Renderer()

	tex, err := b.texCache.Load(hotkeyTexture)
	if err != nil {
		return
	}

	// Claimed before it is drawn, so a click on a slot does not fall through
	// and walk the character to whatever is behind the bar.
	b.ctx.CaptureMouse(ui2d.Rect{X: x, Y: y, W: hotkeyBarW, H: hotkeyBarH})

	r.DrawImage(tex.ID, x, y, hotkeyBarW, hotkeyBarH, ui2d.ColorWhite)

	// The row number in the last cell. One row for now: the original stacks
	// up to four and cycles them with the resize handle, which needs slots
	// worth stacking first.
	label := strconv.Itoa(1)
	cell := hotkeyCell(x, y, hotkeySlots)

	capW, capH := r.MeasureText(label, 1)
	capX := cell.X + (cell.W-capW)/2
	capY := cell.Y + (cell.H-capH)/2

	// Dark: the cell it sits in is a light grey box, and the pale-on-dark
	// color the rest of the HUD uses reads as an outline here rather than a
	// number.
	r.DrawText(capX, capY, label, 1, ui2d.ColorText)
}

// hotkeyCell is where one cell of the bar sits on screen, given the bar's
// top-left corner.
func hotkeyCell(barX, barY float32, index int) ui2d.Rect {
	return ui2d.Rect{
		X: barX + hotkeyCellX + float32(index)*hotkeyCellPitch,
		Y: barY + hotkeyCellY,
		W: hotkeyCellW,
		H: hotkeyCellH,
	}
}
