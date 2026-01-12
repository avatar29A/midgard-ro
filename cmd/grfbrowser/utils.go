// Utility functions for GRF Browser.
package main

import (
	"image"
	"image/color"
	"path/filepath"
	"strings"

	"golang.org/x/text/encoding/korean"
	"golang.org/x/text/transform"

	"github.com/Faultbox/midgard-ro/pkg/formats"
)

// euckrToUTF8 converts EUC-KR encoded string to UTF-8.
// Note: GRF files use EUC-KR encoding for Korean filenames.
func euckrToUTF8(s string) string {
	// Check if string contains non-ASCII bytes that might be EUC-KR
	hasHighBytes := false
	for i := 0; i < len(s); i++ {
		if s[i] > 127 {
			hasHighBytes = true
			break
		}
	}

	// Only decode if there are high bytes (potential EUC-KR)
	if !hasHighBytes {
		return s
	}

	decoder := korean.EUCKR.NewDecoder()
	result, _, err := transform.String(decoder, s)
	if err != nil {
		return s // Return original if conversion fails
	}
	return result
}

// sprImageToRGBA converts a SPR image to an RGBA image for rendering.
func sprImageToRGBA(img *formats.SPRImage) *image.RGBA {
	rgba := image.NewRGBA(image.Rect(0, 0, int(img.Width), int(img.Height)))

	// Copy pixel data
	for y := 0; y < int(img.Height); y++ {
		for x := 0; x < int(img.Width); x++ {
			i := (y*int(img.Width) + x) * 4
			rgba.SetRGBA(x, y, color.RGBA{
				R: img.Pixels[i],
				G: img.Pixels[i+1],
				B: img.Pixels[i+2],
				A: img.Pixels[i+3],
			})
		}
	}

	return rgba
}

// getFileIcon returns a short icon string for a file based on extension.
func getFileIcon(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))

	switch ext {
	case ".spr":
		return "[SPR]"
	case ".act":
		return "[ACT]"
	case ".bmp", ".tga", ".jpg", ".png", ".imf":
		return "[IMG]"
	case ".rsm":
		return "[3D]"
	case ".rsw":
		return "[MAP]"
	case ".gat":
		return "[GAT]"
	case ".gnd":
		return "[GND]"
	case ".wav", ".mp3":
		return "[SND]"
	case ".txt", ".xml", ".lua":
		return "[TXT]"
	default:
		return "[?]"
	}
}

// getFileTypeName returns a human-readable file type name.
func getFileTypeName(ext string) string {
	switch ext {
	case ".spr":
		return "Sprite Image"
	case ".act":
		return "Animation Data"
	case ".bmp", ".tga", ".jpg", ".png":
		return "Texture Image"
	case ".imf":
		return "Image Format (IMF)"
	case ".rsm":
		return "3D Model"
	case ".rsw":
		return "Map Resource"
	case ".gat":
		return "Ground Altitude"
	case ".gnd":
		return "Ground Mesh"
	case ".wav", ".mp3":
		return "Audio File"
	case ".txt":
		return "Text File"
	case ".xml":
		return "XML File"
	case ".lua":
		return "Lua Script"
	default:
		return "Unknown"
	}
}
