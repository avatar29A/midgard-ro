// Package texture provides image decoding and texture processing utilities.
package texture

import (
	"fmt"
	"image"
	"image/color"
)

// DecodeTGA decodes a TGA image file.
// Supports uncompressed true-color (type 2) and RLE compressed (type 10) TGA files,
// which are the formats commonly used in Ragnarok Online.
func DecodeTGA(data []byte) (image.Image, error) {
	if len(data) < 18 {
		return nil, fmt.Errorf("TGA data too short")
	}

	// TGA header
	idLength := int(data[0])
	colorMapType := data[1]
	imageType := data[2]
	// colorMapSpec: bytes 3-7 (skip for now)
	// imageSpec: bytes 8-17
	width := int(data[12]) | int(data[13])<<8
	height := int(data[14]) | int(data[15])<<8
	bpp := int(data[16])
	descriptor := data[17]

	// Check supported formats
	if colorMapType != 0 {
		return nil, fmt.Errorf("color-mapped TGA not supported")
	}
	if imageType != 2 && imageType != 10 {
		return nil, fmt.Errorf("unsupported TGA type %d (only uncompressed/RLE true-color supported)", imageType)
	}
	if bpp != 24 && bpp != 32 {
		return nil, fmt.Errorf("unsupported TGA bit depth %d (only 24/32 supported)", bpp)
	}

	// Skip ID field
	offset := 18 + idLength
	if offset > len(data) {
		return nil, fmt.Errorf("TGA data truncated")
	}
	pixelData := data[offset:]

	// Create image
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	bytesPerPixel := bpp / 8

	// Check if image is flipped (bit 5 of descriptor = top-to-bottom)
	topToBottom := (descriptor & 0x20) != 0

	if imageType == 2 {
		// Uncompressed
		expectedSize := width * height * bytesPerPixel
		if len(pixelData) < expectedSize {
			return nil, fmt.Errorf("TGA pixel data truncated")
		}

		for y := 0; y < height; y++ {
			destY := y
			if !topToBottom {
				destY = height - 1 - y
			}
			for x := 0; x < width; x++ {
				i := (y*width + x) * bytesPerPixel
				b := pixelData[i]
				g := pixelData[i+1]
				r := pixelData[i+2]
				a := uint8(255)
				if bytesPerPixel == 4 {
					a = pixelData[i+3]
				}
				img.SetRGBA(x, destY, color.RGBA{R: r, G: g, B: b, A: a})
			}
		}
	} else {
		// RLE compressed (type 10)
		if err := decodeTGARLE(img, pixelData, width, height, bytesPerPixel, topToBottom); err != nil {
			return nil, err
		}
	}

	return img, nil
}

// decodeTGARLE decodes RLE-compressed TGA pixel data into an image.
func decodeTGARLE(img *image.RGBA, pixelData []byte, width, height, bytesPerPixel int, topToBottom bool) error {
	pixelCount := width * height
	pixelIdx := 0
	dataIdx := 0

	for pixelIdx < pixelCount && dataIdx < len(pixelData) {
		packet := pixelData[dataIdx]
		dataIdx++
		count := int(packet&0x7F) + 1

		if packet&0x80 != 0 {
			// RLE packet - repeat single pixel
			if dataIdx+bytesPerPixel > len(pixelData) {
				break
			}
			b := pixelData[dataIdx]
			g := pixelData[dataIdx+1]
			r := pixelData[dataIdx+2]
			a := uint8(255)
			if bytesPerPixel == 4 {
				a = pixelData[dataIdx+3]
			}
			dataIdx += bytesPerPixel

			c := color.RGBA{R: r, G: g, B: b, A: a}
			for i := 0; i < count && pixelIdx < pixelCount; i++ {
				x := pixelIdx % width
				y := pixelIdx / width
				destY := y
				if !topToBottom {
					destY = height - 1 - y
				}
				img.SetRGBA(x, destY, c)
				pixelIdx++
			}
		} else {
			// Raw packet - read count pixels
			for i := 0; i < count && pixelIdx < pixelCount; i++ {
				if dataIdx+bytesPerPixel > len(pixelData) {
					break
				}
				b := pixelData[dataIdx]
				g := pixelData[dataIdx+1]
				r := pixelData[dataIdx+2]
				a := uint8(255)
				if bytesPerPixel == 4 {
					a = pixelData[dataIdx+3]
				}
				dataIdx += bytesPerPixel

				x := pixelIdx % width
				y := pixelIdx / width
				destY := y
				if !topToBottom {
					destY = height - 1 - y
				}
				img.SetRGBA(x, destY, color.RGBA{R: r, G: g, B: b, A: a})
				pixelIdx++
			}
		}
	}

	return nil
}

// TGA image type constants.
const (
	TGATypeUncompressed = 2  // Uncompressed true-color
	TGATypeRLE          = 10 // RLE compressed true-color
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

// ImageToRGBA converts any image.Image to *image.RGBA.
// If applyMagentaKey is true, magenta pixels are made transparent.
func ImageToRGBA(img image.Image, applyMagentaKey bool) *image.RGBA {
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
