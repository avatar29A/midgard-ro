// Image, text, and hex preview for GRF Browser.
package main

import (
	"bytes"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"strings"

	"github.com/AllenDang/cimgui-go/backend"
	"github.com/AllenDang/cimgui-go/imgui"
	"golang.org/x/text/encoding/korean"
	"golang.org/x/text/transform"

	"github.com/Faultbox/midgard-ro/internal/engine/texture"
)

// decodeTGA decodes a TGA image file using the texture package.
func decodeTGA(data []byte) (image.Image, error) {
	return texture.DecodeTGA(data)
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
