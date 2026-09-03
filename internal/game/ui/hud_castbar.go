package ui

import (
	"github.com/Faultbox/midgard-ro/internal/engine/ui2d"
	"github.com/Faultbox/midgard-ro/internal/game/states"
)

// The cast bar.
//
// Under the caster's feet, where the original puts it and where the health
// bars already sit — one below the other rather than on top of each other,
// since a character casting is usually a character whose health is worth
// watching too.

const (
	// castBarW and castBarH are its size. Wider than the health bar and no
	// taller: it is read as a length rather than as a level.
	castBarW = float32(70)
	castBarH = float32(6)

	// castBarRise is how far above the head it sits. Above rather than below:
	// the ring the cast draws on the ground is already under the feet, and a
	// bar down there reads as part of it.
	castBarRise = float32(10)
)

var (
	castBarBorder = ui2d.Color{R: 0.06, G: 0.09, B: 0.61, A: 1}
	castBarEmpty  = ui2d.Color{R: 0.26, G: 0.26, B: 0.26, A: 1}
	castBarFill   = ui2d.Color{R: 1, G: 0.85, B: 0.35, A: 1}
)

// drawCastBar puts the cast in progress under the caster.
func (b *UI2DBackend) drawCastBar(bar states.CastBar) {
	x := bar.ScreenX - castBarW/2
	y := bar.ScreenY - castBarRise - castBarH

	r := b.ctx.Renderer()

	// Drawn in the image pass for the reason the health bars are: solid quads
	// paint over every image in the frame, and a cast bar showing through an
	// open window is not a depth anyone asked for.
	fill := r.DrawRect

	fill(x-1, y-1, castBarW+2, castBarH+2, castBarBorder)
	fill(x, y, castBarW, castBarH, castBarEmpty)

	done := bar.Progress
	if done < 0 {
		done = 0
	}
	if done > 1 {
		done = 1
	}

	if done > 0 {
		fill(x, y, castBarW*done, castBarH, castBarFill)
	}
}
