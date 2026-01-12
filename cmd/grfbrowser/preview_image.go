// Image, text, and hex preview for GRF Browser.
package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"strings"

	"github.com/AllenDang/cimgui-go/backend"
	"github.com/AllenDang/cimgui-go/imgui"
	"golang.org/x/text/encoding/korean"
	"golang.org/x/text/transform"
)

// decodeTGA decodes a TGA image file.
// Supports uncompressed true-color (type 2) and RLE compressed (type 10) TGA files,
// which are the formats commonly used in Ragnarok Online.
func decodeTGA(data []byte) (image.Image, error) {
	if len(data) < 18 {
		return nil, fmt.Errorf("TGA data too short")
	}

	// TGA header
	idLength := int(data[0])
	colorMapType := data[1]
	imageType := data[2]
	// colorMapSpec: bytes 3-7 (skip for now)
	// imageSpec: bytes 8-17
	xOrigin := int(data[8]) | int(data[9])<<8
	yOrigin := int(data[10]) | int(data[11])<<8
	width := int(data[12]) | int(data[13])<<8
	height := int(data[14]) | int(data[15])<<8
	bpp := int(data[16])
	descriptor := data[17]

	_ = xOrigin
	_ = yOrigin

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

				for i := 0; i < count && pixelIdx < pixelCount; i++ {
					x := pixelIdx % width
					y := pixelIdx / width
					destY := y
					if !topToBottom {
						destY = height - 1 - y
					}
					img.SetRGBA(x, destY, color.RGBA{R: r, G: g, B: b, A: a})
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
	}

	return img, nil
}

// loadImagePreview loads an image file (BMP, TGA, JPG, PNG) for preview.
func (app *App) loadImagePreview(path string) {
	data, err := app.archive.Read(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading image: %v\n", err)
		return
	}

	// Decode image - use image.Decode which auto-detects format
	// BMP is registered via golang.org/x/image/bmp import
	// JPEG and PNG are in standard library
	var img image.Image
	ext := strings.ToLower(filepath.Ext(path))

	if ext == ".tga" {
		// TGA needs special handling (not in standard library)
		img, err = decodeTGA(data)
	} else {
		img, _, err = image.Decode(bytes.NewReader(data))
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error decoding image %s: %v\n", ext, err)
		return
	}

	// Convert to RGBA
	bounds := img.Bounds()
	rgba := image.NewRGBA(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			rgba.Set(x, y, img.At(x, y))
		}
	}

	// Create texture
	app.previewImage = backend.NewTextureFromRgba(rgba)
	app.previewImgSize = [2]int{bounds.Dx(), bounds.Dy()}
}

// loadTextPreview loads a text file for preview.
func (app *App) loadTextPreview(path string) {
	data, err := app.archive.Read(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading text file: %v\n", err)
		return
	}

	// Try to convert from EUC-KR to UTF-8 if it looks like Korean
	text := string(data)
	if hasHighBytes(data) {
		decoder := korean.EUCKR.NewDecoder()
		if decoded, _, err := transform.String(decoder, text); err == nil {
			text = decoded
		}
	}

	// Limit preview size to avoid performance issues
	const maxPreviewSize = 64 * 1024 // 64KB
	if len(text) > maxPreviewSize {
		text = text[:maxPreviewSize] + "\n\n... (truncated)"
	}

	app.previewText = text
}

// loadHexPreview loads raw bytes for hex preview.
func (app *App) loadHexPreview(path string) {
	data, err := app.archive.Read(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file: %v\n", err)
		return
	}

	app.previewHexSize = int64(len(data))

	// Limit hex preview to first 4KB
	const maxHexSize = 4 * 1024
	if len(data) > maxHexSize {
		app.previewHex = data[:maxHexSize]
	} else {
		app.previewHex = data
	}
}

// hasHighBytes checks if data contains non-ASCII bytes (potential EUC-KR).
func hasHighBytes(data []byte) bool {
	for _, b := range data {
		if b > 127 {
			return true
		}
	}
	return false
}

// renderImagePreview renders an image (BMP, TGA, JPG, PNG) with zoom controls.
func (app *App) renderImagePreview() {
	if app.previewImage == nil {
		imgui.TextDisabled("Failed to load image")
		return
	}

	imgui.Text(fmt.Sprintf("Size: %d x %d", app.previewImgSize[0], app.previewImgSize[1]))

	// Zoom controls
	imgui.Text("Zoom:")
	imgui.SameLine()
	if imgui.Button("-##imgzoom") && app.previewZoom > 0.25 {
		app.previewZoom -= 0.25
	}
	imgui.SameLine()
	imgui.Text(fmt.Sprintf("%.0f%%", app.previewZoom*100))
	imgui.SameLine()
	if imgui.Button("+##imgzoom") && app.previewZoom < 4.0 {
		app.previewZoom += 0.25
	}
	imgui.SameLine()
	if imgui.Button("Reset##imgzoom") {
		app.previewZoom = 1.0
	}

	imgui.Separator()

	// Display image centered
	w := float32(app.previewImgSize[0]) * app.previewZoom
	h := float32(app.previewImgSize[1]) * app.previewZoom

	avail := imgui.ContentRegionAvail()
	startX := imgui.CursorPosX()
	startY := imgui.CursorPosY()
	if w < avail.X {
		imgui.SetCursorPosX(startX + (avail.X-w)/2)
	}
	if h < avail.Y {
		imgui.SetCursorPosY(startY + (avail.Y-h)/2)
	}

	imgui.ImageWithBgV(
		app.previewImage.ID,
		imgui.NewVec2(w, h),
		imgui.NewVec2(0, 0),
		imgui.NewVec2(1, 1),
		imgui.NewVec4(0.2, 0.2, 0.2, 1.0),
		imgui.NewVec4(1, 1, 1, 1),
	)
}

// renderTextPreview renders a text file with scrolling.
func (app *App) renderTextPreview() {
	if app.previewText == "" {
		imgui.TextDisabled("Empty file or failed to load")
		return
	}

	imgui.Text(fmt.Sprintf("Size: %d bytes", len(app.previewText)))
	imgui.Separator()

	// Scrollable text area
	flags := imgui.WindowFlagsHorizontalScrollbar
	if imgui.BeginChildStrV("TextPreview", imgui.NewVec2(0, 0), imgui.ChildFlagsBorders, flags) {
		imgui.TextUnformatted(app.previewText)
	}
	imgui.EndChild()
}

// renderHexPreview renders a hex dump of binary data.
func (app *App) renderHexPreview() {
	if app.previewHex == nil {
		imgui.TextDisabled("Failed to load file")
		return
	}

	imgui.Text(fmt.Sprintf("File size: %d bytes", app.previewHexSize))
	if int64(len(app.previewHex)) < app.previewHexSize {
		imgui.SameLine()
		imgui.TextDisabled(fmt.Sprintf("(showing first %d bytes)", len(app.previewHex)))
	}
	imgui.Separator()

	// Scrollable hex view
	flags := imgui.WindowFlagsHorizontalScrollbar
	if imgui.BeginChildStrV("HexPreview", imgui.NewVec2(0, 0), imgui.ChildFlagsBorders, flags) {
		// Render hex dump in classic format: offset | hex bytes | ascii
		const bytesPerLine = 16
		for offset := 0; offset < len(app.previewHex); offset += bytesPerLine {
			// Offset
			line := fmt.Sprintf("%08X  ", offset)

			// Hex bytes
			for i := 0; i < bytesPerLine; i++ {
				if offset+i < len(app.previewHex) {
					line += fmt.Sprintf("%02X ", app.previewHex[offset+i])
				} else {
					line += "   "
				}
				if i == 7 {
					line += " "
				}
			}

			// ASCII representation
			line += " |"
			for i := 0; i < bytesPerLine && offset+i < len(app.previewHex); i++ {
				b := app.previewHex[offset+i]
				if b >= 32 && b < 127 {
					line += string(b)
				} else {
					line += "."
				}
			}
			line += "|"

			imgui.Text(line)
		}
	}
	imgui.EndChild()
}
