package ui

import (
	"github.com/Faultbox/midgard-ro/internal/engine/ui2d"
	"github.com/Faultbox/midgard-ro/internal/game/entity"
	"github.com/Faultbox/midgard-ro/internal/game/states"
)

// The bars under a unit's feet, drawn rather than skinned.
//
// The gauge art in the archive (gzered, gzeblue) belongs to the Basic Info
// panel: 1px fills sized for a 280px trough the panel supplies. These float
// over the world with nothing behind them, and the original fills them
// directly — roBrowser does the same (Renderer/Entity/EntityLife.js), and its
// geometry is what these numbers are.
const (
	entityBarW = float32(60)

	// entityBarH is the whole thing: 5 for an HP bar alone, 9 once an SP bar
	// is under it.
	entityBarH   = float32(5)
	entityBarHSP = float32(9)

	// entityBarFillH is how tall one bar's fill is, inside the 1px border.
	entityBarFillH = float32(3)

	// entityBarLowHP is the fraction below which the HP bar turns to warn.
	entityBarLowHP = float32(0.25)

	// entityBarDrop is how far below the unit's feet the bars sit.
	entityBarDrop = float32(4)
)

// Bar colors, from the original by way of roBrowser. The HP color says what
// you are looking at — a monster, a pet or a person — which is why there is
// more than one.
var (
	entityBarBorder  = ui2d.Color{R: 0x10 / 255.0, G: 0x18 / 255.0, B: 0x9c / 255.0, A: 1}
	entityBarEmpty   = ui2d.Color{R: 0x42 / 255.0, G: 0x42 / 255.0, B: 0x42 / 255.0, A: 1}
	entityBarLow     = ui2d.Color{R: 1, G: 1, B: 0, A: 1}
	entityBarPlayer  = ui2d.Color{R: 0x10 / 255.0, G: 0xef / 255.0, B: 0x21 / 255.0, A: 1}
	entityBarPlayerL = ui2d.Color{R: 1, G: 0, B: 0, A: 1}
	entityBarMob     = ui2d.Color{R: 1, G: 0, B: 0xe7 / 255.0, A: 1}
	entityBarSP      = ui2d.Color{R: 0x18 / 255.0, G: 0x63 / 255.0, B: 0xde / 255.0, A: 1}
)

// barFraction is how full a bar is, clamped to 0..1.
//
// A maximum of zero reads as empty rather than dividing: a unit whose MaxHP we
// have not been told is not a unit at full health.
func barFraction(value, max int) float32 {
	if max <= 0 || value <= 0 {
		return 0
	}
	if value >= max {
		return 1
	}

	return float32(value) / float32(max)
}

// hpColor is the color an HP bar fills with, which says what kind of thing it
// belongs to and whether it is in trouble.
// The original also has a color for pets (#FFE7E7). We do not model them —
// nothing on the wire tells them apart from any other unit at this packet
// version — so there is no branch for one here.
func hpColor(t entity.Type, fraction float32) ui2d.Color {
	if fraction < entityBarLowHP {
		if t == entity.TypeMonster {
			return entityBarLow
		}

		return entityBarPlayerL
	}

	if t == entity.TypeMonster {
		return entityBarMob
	}

	return entityBarPlayer
}

// drawEntityBars puts one unit's bars under its feet.
func (b *UI2DBackend) drawEntityBars(bar states.EntityBar) {
	if bar.Alpha <= 0 || bar.MaxHP <= 0 {
		return
	}

	height := entityBarH
	if bar.HasSP {
		height = entityBarHSP
	}

	x := bar.ScreenX - entityBarW/2
	y := bar.ScreenY + entityBarDrop

	r := b.ctx.Renderer()

	// Filled in the image pass, not with DrawRect.
	//
	// These belong to the world, and every window belongs over them. Solid
	// quads paint over every image in the frame by design, so bars drawn as
	// solids sat on top of the map window's picture — a character's health
	// showing through a map is not a depth anyone asked for. In the image
	// pass they are ordered by call, and the windows are drawn after.
	fill := r.DrawRect

	// Border first, then the empty channel inset into it, then the fills —
	// the same order the original builds them in, so a bar at zero still
	// reads as a bar rather than disappearing.
	fill(x, y, entityBarW, height, entityBarBorder.WithAlpha(bar.Alpha))
	fill(x+1, y+1, entityBarW-2, height-2, entityBarEmpty.WithAlpha(bar.Alpha))

	hp := barFraction(bar.HP, bar.MaxHP)
	if hp > 0 {
		fill(x+1, y+1, (entityBarW-2)*hp, entityBarFillH,
			hpColor(bar.Type, hp).WithAlpha(bar.Alpha))
	}

	if !bar.HasSP {
		return
	}

	// The separator between the two, then the SP fill under it.
	fill(x, y+4, entityBarW, 1, entityBarBorder.WithAlpha(bar.Alpha))

	if sp := barFraction(bar.SP, bar.MaxSP); sp > 0 {
		fill(x+1, y+5, (entityBarW-2)*sp, entityBarFillH,
			entityBarSP.WithAlpha(bar.Alpha))
	}
}

// The name over a ground item the pointer is on.
const (
	// itemLabelScale matches the bars' world-scale text rather than the
	// panels', so a label on the map does not read as part of the interface.
	itemLabelScale = float32(1)

	// itemLabelRise is how far above the item's base the line sits. An item
	// sprite is about twenty pixels tall, so this clears it without floating.
	itemLabelRise = float32(26)

	// itemLabelPadX and itemLabelPadY inset the text from the plate behind it.
	itemLabelPadX = float32(4)
	itemLabelPadY = float32(2)
)

var (
	itemLabelColor = ui2d.Color{R: 1, G: 1, B: 1, A: 1}

	// itemLabelPlate is the plate the text sits on. A shadow alone was not
	// enough: the label lands on whatever the ground happens to be — pale
	// flagstones in Prontera, snow, sand — and white text on a one-pixel
	// shadow disappears against the light ones. Dark and mostly opaque, so
	// the text carries wherever it falls, but still see-through enough to
	// read as part of the world rather than a panel.
	itemLabelPlate = ui2d.Color{R: 0, G: 0, B: 0, A: 0.6}
)

// drawItemLabel names the ground item under the pointer.
func (b *UI2DBackend) drawItemLabel(label *states.HoverLabel) {
	if label == nil || label.Text == "" {
		return
	}

	r := b.ctx.Renderer()

	width, height := r.MeasureText(label.Text, itemLabelScale)
	x := label.ScreenX - width/2
	y := label.ScreenY - itemLabelRise

	r.DrawRect(x-itemLabelPadX, y-itemLabelPadY,
		width+2*itemLabelPadX, height+2*itemLabelPadY, itemLabelPlate)
	r.DrawText(x, y, label.Text, itemLabelScale, itemLabelColor)
}
