// Package ui2d — TTF font loading + glyph atlas (this file).
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

// Font is a TTF-rasterized atlas for the pre-loaded glyph ranges plus
// any runes encountered at runtime (e.g. Korean/CJK in chat). The atlas
// is allocated once at startup and grown into via shelf-pack as new
// glyphs come in; misses upload just the new region via TexSubImage2D.
type Font struct {
	textureID uint32
	texWidth  int
	texHeight int

	glyphs   map[rune]*Glyph
	fallback *Glyph

	ascent     float32 // baseline → top of typical glyphs (positive)
	lineHeight float32 // total line advance (ascent + descent + leading)

	// Kept alive after NewFont so we can rasterize glyphs on demand.
	face  font.Face
	atlas *image.Alpha

	// Shelf-pack cursor — where the next runtime glyph will land.
	curX, curY, rowH int
}

const (
	// fontSize: chosen so text rendered at scale=1.0 visually matches the
	// old 8x8 bitmap font at scale=2.0 (~16px visual). Tuned by eye.
	fontSize = 14.0
	fontDPI  = 96.0
	atlasW   = 1024
	atlasH   = 1024
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

	metrics := face.Metrics()
	f := &Font{
		glyphs:     make(map[rune]*Glyph, 512),
		texWidth:   atlasW,
		texHeight:  atlasH,
		ascent:     float32(metrics.Ascent.Ceil()),
		lineHeight: float32(metrics.Height.Ceil()),
		face:       face,
		atlas:      image.NewAlpha(image.Rect(0, 0, atlasW, atlasH)),
	}

	// Allocate the GL texture up front; we'll fill it via TexSubImage2D as
	// glyphs are added. Initial state is fully transparent which is fine
	// (empty atlas → nothing draws until a glyph is added).
	gl.GenTextures(1, &f.textureID)
	gl.BindTexture(gl.TEXTURE_2D, f.textureID)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
	gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA, int32(atlasW), int32(atlasH), 0,
		gl.RGBA, gl.UNSIGNED_BYTE, nil)
	gl.BindTexture(gl.TEXTURE_2D, 0)

	// Pre-rasterize the common ranges so most text avoids per-glyph upload
	// in the hot path.
	for _, rg := range glyphRanges {
		for r := rg[0]; r <= rg[1]; r++ {
			f.rasterize(r)
		}
	}
	if g, ok := f.glyphs['?']; ok {
		f.fallback = g
	}

	return f
}

// rasterize generates the glyph for r, places it in the atlas via
// shelf-pack, uploads the new region, and caches the metrics. Returns
// the cached glyph (which may be nil if the rune has no representation
// in the font, in which case the caller should fall back).
func (f *Font) rasterize(r rune) *Glyph {
	dr, mask, maskp, advance, ok := f.face.Glyph(fixed.P(0, 0), r)
	if !ok {
		f.glyphs[r] = nil
		return nil
	}
	gw := dr.Dx()
	gh := dr.Dy()
	adv := float32(advance.Ceil())

	g := &Glyph{
		Width:    gw,
		Height:   gh,
		BearingX: float32(dr.Min.X),
		BearingY: float32(dr.Min.Y),
		Advance:  adv,
	}

	if gw <= 0 || gh <= 0 {
		// Whitespace etc — advance only, no atlas slot.
		f.glyphs[r] = g
		return g
	}

	// Shelf-pack: wrap to next row when current row is full.
	if f.curX+gw+glyphPad > atlasW {
		f.curX = 0
		f.curY += f.rowH + glyphPad
		f.rowH = 0
	}
	if f.curY+gh+glyphPad > atlasH {
		// Atlas is full — return whatever fallback we have.
		f.glyphs[r] = f.fallback
		return f.fallback
	}

	x, y := f.curX, f.curY

	// Copy the just-rasterized mask into the atlas immediately, then
	// upload that sub-region to the GPU. face.Glyph reuses its internal
	// mask buffer, so we can't defer this.
	dst := image.Rect(x, y, x+gw, y+gh)
	draw.Draw(f.atlas, dst, mask, maskp, draw.Src)

	rgba := alphaSubregionToRGBA(f.atlas, x, y, gw, gh)
	gl.BindTexture(gl.TEXTURE_2D, f.textureID)
	gl.TexSubImage2D(gl.TEXTURE_2D, 0, int32(x), int32(y), int32(gw), int32(gh),
		gl.RGBA, gl.UNSIGNED_BYTE, unsafe.Pointer(&rgba[0]))
	gl.BindTexture(gl.TEXTURE_2D, 0)

	g.U0 = float32(x) / float32(atlasW)
	g.V0 = float32(y) / float32(atlasH)
	g.U1 = float32(x+gw) / float32(atlasW)
	g.V1 = float32(y+gh) / float32(atlasH)

	f.curX += gw + glyphPad
	if gh > f.rowH {
		f.rowH = gh
	}

	f.glyphs[r] = g
	return g
}

// alphaSubregionToRGBA copies a (w×h) rectangle of the alpha atlas
// starting at (x,y) into a fresh RGBA byte slice (R=G=B=255, A=alpha).
// Used to feed a single newly-added glyph to TexSubImage2D.
func alphaSubregionToRGBA(a *image.Alpha, x, y, w, h int) []byte {
	out := make([]byte, w*h*4)
	for j := 0; j < h; j++ {
		srcRow := (y + j) * a.Stride
		dstRow := j * w * 4
		for i := 0; i < w; i++ {
			alpha := a.Pix[srcRow+x+i]
			off := dstRow + i*4
			out[off+0] = 255
			out[off+1] = 255
			out[off+2] = 255
			out[off+3] = alpha
		}
	}
	return out
}

// Close releases font resources.
func (f *Font) Close() {
	if f.face != nil {
		_ = f.face.Close()
		f.face = nil
	}
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

// Glyph returns the metrics for the given rune, lazily rasterizing it
// into the atlas if it isn't already cached. Returns the fallback glyph
// (`?`) when the font has no representation for r.
func (f *Font) Glyph(r rune) *Glyph {
	if g, ok := f.glyphs[r]; ok {
		if g == nil {
			return f.fallback
		}
		return g
	}
	if f.face == nil {
		return f.fallback
	}
	if g := f.rasterize(r); g != nil {
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
