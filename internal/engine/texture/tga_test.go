package texture

import (
	"image"
	"image/color"
	"testing"
)

// TestImageToRGBAShortPalette: a paletted image whose pixels reference an
// index past the end of its palette must not panic.
//
// image.Paletted.At does not range-check, so this crashed the client outright
// the first time a Geffen model was loaded. The file declares 255 colors and
// uses index 255.
func TestImageToRGBAShortPalette(t *testing.T) {
	short := make(color.Palette, 255)
	for i := range short {
		short[i] = color.RGBA{R: uint8(i), G: 0, B: 0, A: 255}
	}

	img := &image.Paletted{
		Pix:     []uint8{0, 254, 255, 255},
		Stride:  2,
		Rect:    image.Rect(0, 0, 2, 2),
		Palette: short,
	}

	var out *image.RGBA
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("panicked on an out-of-range palette index: %v", r)
			}
		}()
		out = ImageToRGBA(img, false)
	}()

	// In-range pixels keep their color...
	if got := out.RGBAAt(1, 0); got != (color.RGBA{R: 254, A: 255}) {
		t.Errorf("in-range pixel = %v, want the palette entry", got)
	}
	// ...and the undefined ones go transparent rather than to a wrong color.
	if got := out.RGBAAt(0, 1); got != (color.RGBA{}) {
		t.Errorf("out-of-range pixel = %v, want transparent", got)
	}

	// The caller's image must come back untouched.
	if len(img.Palette) != 255 {
		t.Errorf("caller's palette was modified, now %d entries", len(img.Palette))
	}
}
