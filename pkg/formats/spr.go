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
	Version      SPRVersion
	Images       []SPRImage  // All images converted to RGBA
	Palette      *SPRPalette // Original palette (nil for pure TGA sprites)
	IndexedCount int         // Number of indexed (palette) images; RGBA images start after this
}

// PaletteSize is the byte length of a 256-color RO palette, whether it is
// the block at the end of an SPR or a standalone .pal file. The two share a
// layout, which is what makes recoloring a sprite a palette swap.
const PaletteSize = 1024

// ErrBadPalette reports a palette file that is not the expected size.
var ErrBadPalette = errors.New("palette must be 1024 bytes")

// ParsePAL reads a standalone .pal file.
//
// RO recolors a sprite by handing it a different palette rather than by
// storing the sprite again: every hair style ships one .pal per color, and
// they are byte-for-byte the same layout as the block an SPR carries at its
// end.
func ParsePAL(data []byte) (*SPRPalette, error) {
	if len(data) < PaletteSize {
		return nil, fmt.Errorf("%w: got %d", ErrBadPalette, len(data))
	}

	// Trailing bytes are ignored: some files carry a little padding after the
	// table, and the table is what matters.
	return parsePalette(data[:PaletteSize]), nil
}

// ParseSPR parses an SPR file from raw bytes.
func ParseSPR(data []byte) (*SPR, error) {
	return ParseSPRWithPalette(data, nil)
}

// ParseSPRWithPalette parses an SPR, drawing its indexed images through
// override instead of the palette the file carries.
//
// A nil override means use the file's own, which is what ParseSPR does.
//
// Index 0 stays transparent whatever the palette says, so a substituted one
// cannot accidentally make a sprite opaque or invisible — the transparency is
// a property of the index, not of the color it points at.
func ParseSPRWithPalette(data []byte, override *SPRPalette) (*SPR, error) {
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
		Version:      version,
		Images:       make([]SPRImage, 0, int(indexedCount)+int(trueColorCount)),
		IndexedCount: int(indexedCount),
	}

	// Parse palette (last 1024 bytes for v1.1+)
	if len(data) < 1024 {
		return nil, ErrTruncatedSPRData
	}
	spr.Palette = parsePalette(data[len(data)-1024:])

	// Draw through the caller's palette when there is one. The file's own is
	// still recorded, because it is what the sprite looks like unrecolored.
	drawPalette := spr.Palette
	if override != nil {
		drawPalette = override
	}

	// Calculate where image data ends (before palette)
	imageDataEnd := int64(len(data) - 1024 - 4) // -4 for header already consumed

	// Parse indexed images
	useRLE := version.Major == 2 && version.Minor >= 1
	for i := uint16(0); i < indexedCount; i++ {
		img, err := parseIndexedImage(r, drawPalette, useRLE)
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

	// These are stored bottom-up — the last row in the file is the top of the
	// picture — as well as a channel the other way round. Read straight
	// through, a true-color sprite comes out upside down.
	//
	// Which is why it went unnoticed for so long: every sprite a character, a
	// monster or a piece of gear is made of is palette-indexed and stored the
	// right way up. Only the effects are true color, so a fire wall burning
	// upside down was the first sign of it.
	pixels := make([]byte, pixelCount*4)
	for y := 0; y < int(height); y++ {
		for x := 0; x < int(width); x++ {
			src := (y*int(width) + x) * 4
			dst := ((int(height)-y-1)*int(width) + x) * 4

			pixels[dst] = abgr[src+3]   // R
			pixels[dst+1] = abgr[src+2] // G
			pixels[dst+2] = abgr[src+1] // B
			pixels[dst+3] = abgr[src]   // A
		}
	}

	return SPRImage{
		Width:  width,
		Height: height,
		Pixels: pixels,
	}, nil
}
