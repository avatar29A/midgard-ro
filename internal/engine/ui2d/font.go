// TrueType font loading + glyph atlas for the ui2d renderer.
//
// Replaces the old hard-coded 8x8 bitmap font. We rasterize each glyph from
// a system TTF (Arial Unicode on macOS, falling back per-platform) into a
// shelf-packed alpha atlas at startup, and store per-glyph metrics so
// variable-width text renders correctly.
package ui2d

import (
	"fmt"
	"image"
	"image/draw"
	"os"
	"unsafe"

	"github.com/go-gl/gl/v4.1-core/gl"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// Glyph holds atlas + render metrics for a single rasterized character.
type Glyph struct {
	// UV bounds within the atlas (0..1).
	U0, V0, U1, V1 float32
	// Pixel dimensions of the glyph image at scale=1.
	Width, Height int
	// Bearing: offset from the cursor (baseline X, baseline Y) to the glyph's
	// top-left in the atlas. BearingY is typically negative — capitals extend
	// above the baseline.
	BearingX, BearingY float32
	// Advance: how far to move the cursor X after drawing this glyph.
	Advance float32
}

// Font is a TTF-rasterized atlas for ASCII printable glyphs.
type Font struct {
	textureID uint32
	texWidth  int
	texHeight int

	glyphs   map[rune]*Glyph
	fallback *Glyph

	ascent     float32 // baseline → top of typical glyphs (positive)
	lineHeight float32 // total line advance (ascent + descent + leading)
}

const (
	// fontSize: chosen so text rendered at scale=1.0 visually matches the
	// old 8x8 bitmap font at scale=2.0 (~16px visual). Tuned by eye.
	fontSize = 14.0
	fontDPI  = 96.0
	atlasW   = 512
	glyphPad = 1
)

// glyphRanges is the inclusive Unicode ranges we pre-rasterize at startup.
// Covers ASCII printable + Latin-1 Supplement (accented chars) + Cyrillic
// (Russian). Korean/CJK is intentionally left out — those would balloon
// startup time and atlas size; on-demand caching is the right answer there
// and is a follow-up.
var glyphRanges = [][2]rune{
	{0x0020, 0x007E}, // Basic Latin (printable ASCII)
	{0x00A0, 0x00FF}, // Latin-1 Supplement
	{0x0400, 0x04FF}, // Cyrillic
}

// systemFontPaths is a per-platform fallback list of TTFs we try in order.
var systemFontPaths = []string{
	"/System/Library/Fonts/Supplemental/Arial Unicode.ttf",
	"/Library/Fonts/Arial Unicode.ttf",
	"/System/Library/Fonts/Helvetica.ttc",
	"C:\\Windows\\Fonts\\arial.ttf",
	"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
	"/usr/share/fonts/TTF/DejaVuSans.ttf",
}

// NewFont loads a system TTF and builds the glyph atlas. Returns nil if
// no usable font is found; callers should treat that as "no text".
func NewFont() *Font {
	data, err := loadSystemFont()
	if err != nil {
		return nil
	}

	parsed, err := opentype.Parse(data)
	if err != nil {
		return nil
	}

	face, err := opentype.NewFace(parsed, &opentype.FaceOptions{
		Size:    fontSize,
		DPI:     fontDPI,
		Hinting: font.HintingFull,
	})
	if err != nil {
		return nil
	}
	defer face.Close()

	metrics := face.Metrics()
	f := &Font{
		glyphs:     make(map[rune]*Glyph, 96),
		texWidth:   atlasW,
		ascent:     float32(metrics.Ascent.Ceil()),
		lineHeight: float32(metrics.Height.Ceil()),
	}

	// Plan glyph placement before allocating the atlas — we don't know the
	// full height until we've shelf-packed every glyph. We copy each mask
	// into a private buffer here because face.Glyph reuses its internal
	// rasterizer; without the copy, subsequent Glyph calls overwrite the
	// pixels we're about to read.
	type placement struct {
		r        rune
		x, y     int
		w, h     int
		bearingX float32
		bearingY float32
		advance  float32
		mask     *image.Alpha // private copy, origin at (0,0)
	}
	var plans []placement
	curX, curY, rowH := 0, 0, 0

	for _, rg := range glyphRanges {
		for r := rg[0]; r <= rg[1]; r++ {
			dr, mask, maskp, advance, ok := face.Glyph(fixed.P(0, 0), r)
			if !ok {
				continue
			}
			gw := dr.Dx()
			gh := dr.Dy()
			adv := float32(advance.Ceil())

			if gw <= 0 || gh <= 0 {
				// Whitespace and similar — record advance only, no atlas slot.
				plans = append(plans, placement{r: r, advance: adv})
				continue
			}

			if curX+gw+glyphPad > atlasW {
				curX = 0
				curY += rowH + glyphPad
				rowH = 0
			}

			// Take an immediate copy — the rasterizer reuses its internal
			// mask buffer across calls.
			maskCopy := image.NewAlpha(image.Rect(0, 0, gw, gh))
			draw.Draw(maskCopy, maskCopy.Bounds(), mask, maskp, draw.Src)

			plans = append(plans, placement{
				r:        r,
				x:        curX,
				y:        curY,
				w:        gw,
				h:        gh,
				bearingX: float32(dr.Min.X),
				bearingY: float32(dr.Min.Y),
				advance:  adv,
				mask:     maskCopy,
			})

			curX += gw + glyphPad
			if gh > rowH {
				rowH = gh
			}
		}
	}

	atlasH := nextPow2(curY + rowH + glyphPad)
	if atlasH < 16 {
		atlasH = 16
	}
	f.texHeight = atlasH

	// Rasterize each glyph into the alpha atlas.
	atlas := image.NewAlpha(image.Rect(0, 0, atlasW, atlasH))
	for _, p := range plans {
		g := &Glyph{
			Width:    p.w,
			Height:   p.h,
			BearingX: p.bearingX,
			BearingY: p.bearingY,
			Advance:  p.advance,
		}
		if p.mask != nil && p.w > 0 && p.h > 0 {
			dst := image.Rect(p.x, p.y, p.x+p.w, p.y+p.h)
			draw.Draw(atlas, dst, p.mask, image.Point{}, draw.Src)
			g.U0 = float32(p.x) / float32(atlasW)
			g.V0 = float32(p.y) / float32(atlasH)
			g.U1 = float32(p.x+p.w) / float32(atlasW)
			g.V1 = float32(p.y+p.h) / float32(atlasH)
		}
		f.glyphs[p.r] = g
	}
	if g, ok := f.glyphs['?']; ok {
		f.fallback = g
	}

	// Convert alpha → RGBA so the existing text shader (which expects RGBA
	// + tints by per-vertex colour) just works.
	rgba := alphaToRGBA(atlas)

	gl.GenTextures(1, &f.textureID)
	gl.BindTexture(gl.TEXTURE_2D, f.textureID)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
	gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA, int32(atlasW), int32(atlasH), 0,
		gl.RGBA, gl.UNSIGNED_BYTE, unsafe.Pointer(&rgba[0]))
	gl.BindTexture(gl.TEXTURE_2D, 0)

	return f
}

// Close releases font resources.
func (f *Font) Close() {
	if f.textureID != 0 {
		gl.DeleteTextures(1, &f.textureID)
		f.textureID = 0
	}
}

// TextureID returns the OpenGL texture ID for the glyph atlas.
func (f *Font) TextureID() uint32 { return f.textureID }

// Ascent returns the baseline-to-top distance in pixels at scale=1.
func (f *Font) Ascent() float32 { return f.ascent }

// LineHeight returns the line advance in pixels at scale=1.
func (f *Font) LineHeight() float32 { return f.lineHeight }

// Glyph returns the metrics for the given rune, or the fallback for
// missing glyphs.
func (f *Font) Glyph(r rune) *Glyph {
	if g, ok := f.glyphs[r]; ok {
		return g
	}
	return f.fallback
}

// MeasureText returns the bounding (width, height) of the rendered text
// at the given scale factor.
func (f *Font) MeasureText(text string, scale float32) (float32, float32) {
	if f == nil {
		return 0, 0
	}
	var maxLineW, lineW float32
	lines := 1
	for _, ch := range text {
		if ch == '\n' {
			if lineW > maxLineW {
				maxLineW = lineW
			}
			lineW = 0
			lines++
			continue
		}
		if g := f.Glyph(ch); g != nil {
			lineW += g.Advance * scale
		}
	}
	if lineW > maxLineW {
		maxLineW = lineW
	}
	return maxLineW, float32(lines) * f.lineHeight * scale
}

// loadSystemFont returns the bytes of the first system TTF that exists.
func loadSystemFont() ([]byte, error) {
	for _, p := range systemFontPaths {
		if data, err := os.ReadFile(p); err == nil {
			return data, nil
		}
	}
	return nil, fmt.Errorf("no system TTF found in %v", systemFontPaths)
}

// alphaToRGBA expands an alpha-only image to RGBA where every pixel is
// white with the alpha mask applied. Lets the text shader tint via its
// per-vertex colour without needing a separate alpha-only shader.
func alphaToRGBA(a *image.Alpha) []byte {
	w := a.Rect.Dx()
	h := a.Rect.Dy()
	out := make([]byte, w*h*4)
	for y := 0; y < h; y++ {
		srcRow := y * a.Stride
		dstRow := y * w * 4
		for x := 0; x < w; x++ {
			alpha := a.Pix[srcRow+x]
			off := dstRow + x*4
			out[off+0] = 255
			out[off+1] = 255
			out[off+2] = 255
			out[off+3] = alpha
		}
	}
	return out
}

// nextPow2 rounds n up to the next power of two (minimum 1).
func nextPow2(n int) int {
	if n < 1 {
		return 1
	}
	p := 1
	for p < n {
		p <<= 1
	}
	return p
}
