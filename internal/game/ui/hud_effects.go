package ui

import (
	"github.com/Faultbox/midgard-ro/internal/engine/ui2d"
	"github.com/Faultbox/midgard-ro/internal/game/states"
)

// Drawing the world's STR effects.
//
// The quads arrive already projected, so nothing here knows about the world.
// What it does know is that these are drawn in the interface layer rather than
// in the scene: RO's effects are two-dimensional, authored against the
// original client's window, and drawing them as world geometry would have them
// tilt with the camera and sink into the ground.

// drawWorldEffects draws whatever effects are playing over the world.
//
// Before the windows, and after the world labels: an effect belongs to the
// map, and a level-up flash over an open inventory would be drawn on top of
// the panel it is nowhere near.
func (b *UI2DBackend) drawWorldEffects(quads []states.EffectQuad) {
	if len(quads) == 0 {
		return
	}

	r := b.ctx.Renderer()

	for _, quad := range quads {
		tex, err := b.texCache.Load(states.EffectTexturePath() + quad.Texture)
		if err != nil {
			continue
		}

		r.DrawImageQuad(tex.ID, quad.Corners, quad.UV, ui2d.Color{
			R: quad.Color[0],
			G: quad.Color[1],
			B: quad.Color[2],
			A: quad.Color[3],
		}, quad.Additive)
	}
}
