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

	// damageScale enlarges the digits.
	//
	// The digits are eleven pixels by twelve in the archive, which is what
	// the original drew on a much smaller screen and is close to invisible on
	// this one. This is the one number to turn if they want to be bigger or
	// smaller; everything else about them is measured from the art.
	damageScale = float32(2)

	// The arc a figure travels: thrown up out of whatever was hit, then
	// falling back under its own weight.
	//
	// The original does not simply raise the number. It launches it, and the
	// figure slows, turns and drops — vanishing on the way down rather than
	// landing. So the height is launch*p - gravity*p², which peaks a little
	// under halfway through its life and finishes below where it started.
	//
	// Both are in proportion to how tall the figure is drawn, so the arc does
	// not flatten when the digits are made smaller.
	damageLaunch  = float32(180) * damageScale
	damageGravity = float32(200) * damageScale

	// damageFadeFrom is the point in a figure's life where it starts to fade.
	//
	// A little past the top of the arc, so the figure is solid on the way up
	// and through the turn, and goes as it falls — which is what makes it
	// vanish in the air rather than appear to land.
	damageFadeFrom = float32(0.5)

	// damageMissNativeHeight is the height of the word in the archive, which
	// the drawn size is derived from.
	damageMissNativeHeight = float32(16)

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

	// nativeHeight is the image's own height, before scaling, which is what
	// the word's scale is derived against.
	nativeHeight float32
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

	upload := func(path string, scale float32, into func(int, damageGlyph)) {
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
			// Smoothed rather than hard-edged, which is the opposite of what
			// the rest of the interface wants.
			//
			// The interface's art is drawn about 1:1 and nearest keeps its
			// one-pixel bevels crisp. These are the one thing magnified
			// several times over — eleven pixels of digit standing in for a
			// figure meant to be read at a glance — and at that magnification
			// nearest gives squares rather than numerals.
			//
			// The bleed linear pulls out of the transparent pixels is black,
			// and these digits already carry a dark outline, so it lands on
			// the outline rather than haloing the glyph.
			tex := b.ctx.Renderer().CreateTexture(int(img.Width), int(img.Height), img.Pixels)
			into(i, damageGlyph{
				texture:      tex,
				width:        float32(img.Width) * scale,
				height:       float32(img.Height) * scale,
				nativeHeight: float32(img.Height),
			})
		}
	}

	upload(damageDigitsPath, damageScale, func(i int, g damageGlyph) {
		if i < len(b.damageArt.digits) {
			b.damageArt.digits[i] = g
		}
	})

	// The word is drawn the same height as a digit rather than at the digits'
	// magnification.
	//
	// It is already forty-nine pixels wide and sixteen tall in the archive
	// against a digit's eleven by twelve, so enlarging it as much made a miss
	// shout where a hit murmured. Deriving the scale from the two heights
	// keeps them the same size on screen whatever the digits are set to.
	missScale := damageScale
	if digit := b.damageArt.digits[0]; digit.ok() && digit.nativeHeight > 0 {
		missScale = digit.height / damageMissNativeHeight
	}

	// The message sprite holds several; the first is "Miss", which is the
	// only one a blow needs.
	upload(damageMsgPath, missScale, func(i int, g damageGlyph) {
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

	// Up is negative on screen, so the arc is subtracted.
	arc := damageLaunch*n.Progress - damageGravity*n.Progress*n.Progress

	x := n.ScreenX - width/2
	y := n.ScreenY - height - arc

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
