package ui

import (
	"strconv"

	"go.uber.org/zap"

	"github.com/Faultbox/midgard-ro/internal/engine/ui2d"
	"github.com/Faultbox/midgard-ro/internal/game/states"
	"github.com/Faultbox/midgard-ro/internal/logger"
	"github.com/Faultbox/midgard-ro/pkg/formats"
)

// The figures that float up from a blow.
//
// Drawn from the original's own art rather than from text: the digits are a
// sprite, and RO's damage numbers are as much a part of how a fight looks as
// the swing is. damageskininfo.lub names four sets of them — this is the one
// the default skin uses.
const (
	damageDigitsPath = `data\sprite\이팩트\newnumber.spr`
	damageMsgPath    = `data\sprite\이팩트\msg.spr`

	// damageScale enlarges the figures.
	//
	// The digits are eleven pixels by twelve in the archive, which is what
	// the original drew on a much smaller screen and is close to invisible on
	// this one. This is the one number to turn if they want to be bigger or
	// smaller; everything else about them is measured from the art.
	damageScale = float32(3)

	// damageRise is how far a figure climbs over its life, in pixels, in
	// proportion to how tall it is drawn.
	damageRise = float32(34) * damageScale

	// damageFadeFrom is the point in a figure's life where it starts to fade.
	// It holds full strength for most of it and then goes quickly, which
	// reads as a number being read rather than one dissolving throughout.
	damageFadeFrom = float32(0.6)

	// damageDigitOverlap tightens the spacing, since each digit's own image
	// carries a little air on both sides. Scaled with the digits so the
	// figures do not loosen as they grow.
	damageDigitOverlap = float32(2) * damageScale
)

// damageArt is the digit and message sprites, uploaded once.
type damageArt struct {
	loaded bool

	digits [10]damageGlyph
	miss   damageGlyph
}

// damageGlyph is one uploaded image.
type damageGlyph struct {
	texture       uint32
	width, height float32
}

// ok reports whether a glyph has art behind it.
func (g damageGlyph) ok() bool { return g.texture != 0 && g.width > 0 }

// loadDamageArt uploads the digits and the miss message.
//
// Once, on the first blow. Doing it at startup would read two sprites every
// session for a client that might never see a fight.
func (b *UI2DBackend) loadDamageArt() {
	if b.damageArt.loaded {
		return
	}
	b.damageArt.loaded = true

	if b.assetLoader == nil {
		return
	}

	upload := func(path string, into func(int, damageGlyph)) {
		raw, err := b.assetLoader(path)
		if err != nil {
			logger.Warn("no damage art", zap.String("path", path), zap.Error(err))

			return
		}

		spr, err := formats.ParseSPR(raw)
		if err != nil {
			logger.Warn("damage art would not parse", zap.String("path", path), zap.Error(err))

			return
		}

		for i, img := range spr.Images {
			tex := b.ctx.Renderer().CreateTextureNearest(int(img.Width), int(img.Height), img.Pixels)
			into(i, damageGlyph{
				texture: tex,
				width:   float32(img.Width) * damageScale,
				height:  float32(img.Height) * damageScale,
			})
		}
	}

	upload(damageDigitsPath, func(i int, g damageGlyph) {
		if i < len(b.damageArt.digits) {
			b.damageArt.digits[i] = g
		}
	})

	// The message sprite holds several; the first is "Miss", which is the
	// only one a blow needs.
	upload(damageMsgPath, func(i int, g damageGlyph) {
		if i == 0 {
			b.damageArt.miss = g
		}
	})
}

// drawDamageNumbers draws every figure floating over the world.
func (b *UI2DBackend) drawDamageNumbers(numbers []states.DamageNumber) {
	if len(numbers) == 0 {
		return
	}

	b.loadDamageArt()

	for _, n := range numbers {
		b.drawDamageNumber(n)
	}
}

// drawDamageNumber draws one figure, risen and faded by how far through its
// life it is.
func (b *UI2DBackend) drawDamageNumber(n states.DamageNumber) {
	glyphs := b.damageGlyphs(n)
	if len(glyphs) == 0 {
		return
	}

	width, height := float32(0), float32(0)
	for _, g := range glyphs {
		width += g.width - damageDigitOverlap
		if g.height > height {
			height = g.height
		}
	}

	alpha := float32(1)
	if n.Progress > damageFadeFrom {
		alpha = 1 - (n.Progress-damageFadeFrom)/(1-damageFadeFrom)
	}
	if alpha <= 0 {
		return
	}

	tint := ui2d.Color{R: 1, G: 1, B: 1, A: alpha}

	x := n.ScreenX - width/2
	y := n.ScreenY - height - n.Progress*damageRise

	r := b.ctx.Renderer()
	for _, g := range glyphs {
		r.DrawImage(g.texture, x, y, g.width, g.height, tint)
		x += g.width - damageDigitOverlap
	}
}

// damageGlyphs is the art for one figure: the word for a miss, the digits
// otherwise.
//
// A critical is drawn with the same digits. The original has a set of its own
// for them, and giving it one is a matter of loading a second sprite rather
// than of anything here changing.
func (b *UI2DBackend) damageGlyphs(n states.DamageNumber) []damageGlyph {
	if n.Miss {
		if !b.damageArt.miss.ok() {
			return nil
		}

		return []damageGlyph{b.damageArt.miss}
	}

	amount := n.Amount
	if amount < 0 {
		amount = -amount
	}

	text := strconv.Itoa(amount)
	glyphs := make([]damageGlyph, 0, len(text))
	for _, c := range text {
		digit := int(c - '0')
		if digit < 0 || digit >= len(b.damageArt.digits) {
			continue
		}

		g := b.damageArt.digits[digit]
		if !g.ok() {
			return nil
		}

		glyphs = append(glyphs, g)
	}

	return glyphs
}
