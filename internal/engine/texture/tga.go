// Package texture provides image decoding and texture processing utilities.
package texture

import (
	"image"
	"image/color"
)

// IsMagentaKey checks if an RGB color matches the RO magenta transparency key.
// Uses tolerance (R >= 250, G <= 10, B >= 250) to handle BMP decoding variations.
func IsMagentaKey(r, g, b uint8) bool {
	return r >= 250 && g <= 10 && b >= 250
}

// ApplyMagentaKey modifies an RGBA image in-place, making magenta pixels transparent.
// Also sets RGB to black on transparent pixels to prevent color bleeding during filtering.
func ApplyMagentaKey(img *image.RGBA) {
	bounds := img.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			i := img.PixOffset(x, y)
			r, g, b := img.Pix[i], img.Pix[i+1], img.Pix[i+2]
			if IsMagentaKey(r, g, b) {
				// Set to transparent black
				img.Pix[i] = 0
				img.Pix[i+1] = 0
				img.Pix[i+2] = 0
				img.Pix[i+3] = 0
			}
		}
	}
}

// wholePalette returns img with a palette long enough for every index its
// pixels can hold, or img unchanged when it is not paletted.
//
// image.Paletted.At indexes the palette with the raw pixel byte and does not
// range-check it, so a file whose header declares fewer colors than its pixels
// reference panics the whole client. Morocc's models do exactly that: a
// 255-entry palette with pixels holding index 255. Which color those pixels
// were meant to be is not recoverable — the file does not say — so they are
// made transparent, which is already what this package does with the magenta
// key and is the least visible way to be wrong.
//
// The palette is copied rather than extended in place: the image belongs to
// the caller, and a decoder may share one palette across several.
func wholePalette(img image.Image) image.Image {
	p, ok := img.(*image.Paletted)
	if !ok || len(p.Palette) >= 256 {
		return img
	}

	widened := make(color.Palette, 256)
	copy(widened, p.Palette)
	for i := len(p.Palette); i < len(widened); i++ {
		widened[i] = color.RGBA{}
	}

	// Same pixels, same rectangle, only the lookup table grows.
	clone := *p
	clone.Palette = widened

	return &clone
}

// ImageToRGBA converts any image.Image to *image.RGBA.
// If applyMagentaKey is true, magenta pixels are made transparent.
func ImageToRGBA(img image.Image, applyMagentaKey bool) *image.RGBA {
	img = wholePalette(img)

	bounds := img.Bounds()
	rgba := image.NewRGBA(bounds)

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			c := img.At(x, y)
			r16, g16, b16, a16 := c.RGBA()
			// Convert from 16-bit to 8-bit
			r8, g8, b8, a8 := uint8(r16>>8), uint8(g16>>8), uint8(b16>>8), uint8(a16>>8)

			if applyMagentaKey && IsMagentaKey(r8, g8, b8) {
				// Set to transparent black
				r8, g8, b8, a8 = 0, 0, 0, 0
			}

			rgba.SetRGBA(x, y, color.RGBA{R: r8, G: g8, B: b8, A: a8})
		}
	}

	return rgba
}
