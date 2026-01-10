// Package formats provides parsers for Ragnarok Online file formats.
package formats

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
)

// SPR format errors.
var (
	ErrInvalidSPRMagic       = errors.New("invalid SPR magic: expected 'SP'")
	ErrUnsupportedSPRVersion = errors.New("unsupported SPR version")
	ErrTruncatedSPRData      = errors.New("truncated SPR data")
	ErrInvalidImageSize      = errors.New("invalid image dimensions")
)

// SPRVersion represents the SPR file version.
type SPRVersion struct {
	Major uint8
	Minor uint8
}

// String returns the version as "Major.Minor".
func (v SPRVersion) String() string {
	return fmt.Sprintf("%d.%d", v.Major, v.Minor)
}

// SPRImage represents a single sprite image in RGBA format.
type SPRImage struct {
	Width  uint16
	Height uint16
	Pixels []byte // RGBA format, 4 bytes per pixel
}

// SPRColor represents an RGBA color.
type SPRColor struct {
	R, G, B, A uint8
}

// SPRPalette represents a 256-color palette.
type SPRPalette struct {
	Colors [256]SPRColor
}

// SPR represents a parsed sprite file.
type SPR struct {
	Version SPRVersion
	Images  []SPRImage  // All images converted to RGBA
	Palette *SPRPalette // Original palette (nil for pure TGA sprites)
}

// ParseSPR parses an SPR file from raw bytes.
func ParseSPR(data []byte) (*SPR, error) {
	if len(data) < 4 {
		return nil, ErrTruncatedSPRData
	}

	// Check magic "SP"
	if data[0] != 'S' || data[1] != 'P' {
		return nil, ErrInvalidSPRMagic
	}

	// Version is stored as Minor, Major (reversed)
	version := SPRVersion{
		Major: data[3],
		Minor: data[2],
	}

	// Check supported versions
	if version.Major < 1 || version.Major > 2 {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedSPRVersion, version)
	}
	if version.Major == 1 && version.Minor < 1 {
		return nil, fmt.Errorf("%w: %s (system palette not supported)", ErrUnsupportedSPRVersion, version)
	}

	r := bytes.NewReader(data[4:])

	// Read indexed image count
	var indexedCount uint16
	if err := binary.Read(r, binary.LittleEndian, &indexedCount); err != nil {
		return nil, fmt.Errorf("%w: reading indexed count", ErrTruncatedSPRData)
	}

	// Read true-color image count (v2.0+)
	var trueColorCount uint16
	if version.Major >= 2 {
		if err := binary.Read(r, binary.LittleEndian, &trueColorCount); err != nil {
			return nil, fmt.Errorf("%w: reading true-color count", ErrTruncatedSPRData)
		}
	}

	spr := &SPR{
		Version: version,
		Images:  make([]SPRImage, 0, int(indexedCount)+int(trueColorCount)),
	}

	// Parse palette (last 1024 bytes for v1.1+)
	if len(data) < 1024 {
		return nil, ErrTruncatedSPRData
	}
	spr.Palette = parsePalette(data[len(data)-1024:])

	// Calculate where image data ends (before palette)
	imageDataEnd := int64(len(data) - 1024 - 4) // -4 for header already consumed

	// Parse indexed images
	useRLE := version.Major == 2 && version.Minor >= 1
	for i := uint16(0); i < indexedCount; i++ {
		img, err := parseIndexedImage(r, spr.Palette, useRLE)
		if err != nil {
			return nil, fmt.Errorf("parsing indexed image %d: %w", i, err)
		}
		spr.Images = append(spr.Images, img)
	}

	// Parse true-color images
	for i := uint16(0); i < trueColorCount; i++ {
		// Check if we've gone past image data
		pos, _ := r.Seek(0, io.SeekCurrent)
		if pos >= imageDataEnd {
			break
		}

		img, err := parseTrueColorImage(r)
		if err != nil {
			return nil, fmt.Errorf("parsing true-color image %d: %w", i, err)
		}
		spr.Images = append(spr.Images, img)
	}

	return spr, nil
}

// ParseSPRFile parses an SPR file from disk.
func ParseSPRFile(path string) (*SPR, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading SPR file: %w", err)
	}
	return ParseSPR(data)
}

// parsePalette parses 256 RGBA colors from 1024 bytes.
func parsePalette(data []byte) *SPRPalette {
	p := &SPRPalette{}
	for i := 0; i < 256; i++ {
		offset := i * 4
		p.Colors[i] = SPRColor{
			R: data[offset],
			G: data[offset+1],
			B: data[offset+2],
			A: data[offset+3],
		}
	}
	return p
}

// parseIndexedImage parses an indexed-color image and converts to RGBA.
func parseIndexedImage(r *bytes.Reader, palette *SPRPalette, useRLE bool) (SPRImage, error) {
	var width, height uint16
	if err := binary.Read(r, binary.LittleEndian, &width); err != nil {
		return SPRImage{}, fmt.Errorf("%w: reading width", ErrTruncatedSPRData)
	}
	if err := binary.Read(r, binary.LittleEndian, &height); err != nil {
		return SPRImage{}, fmt.Errorf("%w: reading height", ErrTruncatedSPRData)
	}

	// Handle invalid/blank images
	if width == 0 || height == 0 || width == 0xFFFF || height == 0xFFFF {
		return SPRImage{
			Width:  1,
			Height: 1,
			Pixels: []byte{0, 0, 0, 0}, // 1x1 transparent
		}, nil
	}

	pixelCount := int(width) * int(height)
	var indices []byte

	if useRLE {
		// Read compressed size
		var compressedSize uint16
		if err := binary.Read(r, binary.LittleEndian, &compressedSize); err != nil {
			return SPRImage{}, fmt.Errorf("%w: reading compressed size", ErrTruncatedSPRData)
		}

		// Read compressed data
		compressed := make([]byte, compressedSize)
		if _, err := io.ReadFull(r, compressed); err != nil {
			return SPRImage{}, fmt.Errorf("%w: reading compressed data", ErrTruncatedSPRData)
		}

		// Decompress RLE
		indices = decompressRLE(compressed, pixelCount)
	} else {
		// Read raw indices
		indices = make([]byte, pixelCount)
		if _, err := io.ReadFull(r, indices); err != nil {
			return SPRImage{}, fmt.Errorf("%w: reading pixel indices", ErrTruncatedSPRData)
		}
	}

	// Convert to RGBA
	pixels := make([]byte, pixelCount*4)
	for i, idx := range indices {
		offset := i * 4
		if idx == 0 {
			// Index 0 is always transparent
			pixels[offset] = 0
			pixels[offset+1] = 0
			pixels[offset+2] = 0
			pixels[offset+3] = 0
		} else {
			c := palette.Colors[idx]
			pixels[offset] = c.R
			pixels[offset+1] = c.G
			pixels[offset+2] = c.B
			pixels[offset+3] = 255 // Indexed images are fully opaque (except index 0)
		}
	}

	return SPRImage{
		Width:  width,
		Height: height,
		Pixels: pixels,
	}, nil
}

// decompressRLE decompresses RLE-encoded pixel data.
// Format: 0x00 0xNN = NN zeros, 0x00 0x00 = single zero, other = literal
func decompressRLE(compressed []byte, targetSize int) []byte {
	result := make([]byte, 0, targetSize)

	for i := 0; i < len(compressed) && len(result) < targetSize; {
		b := compressed[i]
		i++

		if b == 0 {
			if i >= len(compressed) {
				break
			}
			count := compressed[i]
			i++

			if count == 0 {
				// 0x00 0x00 = single zero
				result = append(result, 0)
			} else {
				// 0x00 0xNN = NN zeros
				for j := uint8(0); j < count && len(result) < targetSize; j++ {
					result = append(result, 0)
				}
			}
		} else {
			result = append(result, b)
		}
	}

	// Pad if needed
	for len(result) < targetSize {
		result = append(result, 0)
	}

	return result
}

// parseTrueColorImage parses an ABGR true-color image and converts to RGBA.
func parseTrueColorImage(r *bytes.Reader) (SPRImage, error) {
	var width, height uint16
	if err := binary.Read(r, binary.LittleEndian, &width); err != nil {
		return SPRImage{}, fmt.Errorf("%w: reading width", ErrTruncatedSPRData)
	}
	if err := binary.Read(r, binary.LittleEndian, &height); err != nil {
		return SPRImage{}, fmt.Errorf("%w: reading height", ErrTruncatedSPRData)
	}

	// Handle invalid/blank images
	if width == 0 || height == 0 || width == 0xFFFF || height == 0xFFFF {
		return SPRImage{
			Width:  1,
			Height: 1,
			Pixels: []byte{0, 0, 0, 0}, // 1x1 transparent
		}, nil
	}

	pixelCount := int(width) * int(height)
	abgr := make([]byte, pixelCount*4)
	if _, err := io.ReadFull(r, abgr); err != nil {
		return SPRImage{}, fmt.Errorf("%w: reading ABGR data", ErrTruncatedSPRData)
	}

	// Convert ABGR to RGBA
	pixels := make([]byte, pixelCount*4)
	for i := 0; i < pixelCount; i++ {
		srcOffset := i * 4
		dstOffset := i * 4
		// ABGR -> RGBA
		pixels[dstOffset] = abgr[srcOffset+3]   // R <- A position? No, ABGR means A,B,G,R
		pixels[dstOffset+1] = abgr[srcOffset+2] // G
		pixels[dstOffset+2] = abgr[srcOffset+1] // B
		pixels[dstOffset+3] = abgr[srcOffset]   // A
	}

	return SPRImage{
		Width:  width,
		Height: height,
		Pixels: pixels,
	}, nil
}
