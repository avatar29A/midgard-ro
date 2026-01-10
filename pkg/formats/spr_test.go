package formats

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func TestParseSPR_InvalidMagic(t *testing.T) {
	data := []byte("XX\x01\x02") // Invalid magic
	_, err := ParseSPR(data)
	if err != ErrInvalidSPRMagic {
		t.Errorf("expected ErrInvalidSPRMagic, got %v", err)
	}
}

func TestParseSPR_TruncatedData(t *testing.T) {
	data := []byte("SP") // Too short
	_, err := ParseSPR(data)
	if err != ErrTruncatedSPRData {
		t.Errorf("expected ErrTruncatedSPRData, got %v", err)
	}
}

func TestParseSPR_UnsupportedVersion(t *testing.T) {
	// Version 1.0 (uses system palette, not supported)
	data := make([]byte, 1030) // Minimum for palette
	copy(data, []byte("SP\x00\x01"))
	_, err := ParseSPR(data)
	if err == nil {
		t.Error("expected error for unsupported version 1.0")
	}
}

func TestParseSPR_Version11_Synthetic(t *testing.T) {
	// Create synthetic v1.1 SPR
	spr := buildSyntheticSPR(1, 1, 1, 0, false)

	parsed, err := ParseSPR(spr)
	if err != nil {
		t.Fatalf("failed to parse synthetic v1.1 SPR: %v", err)
	}

	if parsed.Version.Major != 1 || parsed.Version.Minor != 1 {
		t.Errorf("expected version 1.1, got %s", parsed.Version)
	}

	if len(parsed.Images) != 1 {
		t.Errorf("expected 1 image, got %d", len(parsed.Images))
	}

	if parsed.Images[0].Width != 2 || parsed.Images[0].Height != 2 {
		t.Errorf("expected 2x2 image, got %dx%d", parsed.Images[0].Width, parsed.Images[0].Height)
	}
}

func TestParseSPR_Version20_Synthetic(t *testing.T) {
	// Create synthetic v2.0 SPR with indexed and true-color images
	spr := buildSyntheticSPR(2, 0, 1, 1, false)

	parsed, err := ParseSPR(spr)
	if err != nil {
		t.Fatalf("failed to parse synthetic v2.0 SPR: %v", err)
	}

	if parsed.Version.Major != 2 || parsed.Version.Minor != 0 {
		t.Errorf("expected version 2.0, got %s", parsed.Version)
	}

	if len(parsed.Images) != 2 {
		t.Errorf("expected 2 images, got %d", len(parsed.Images))
	}
}

func TestParseSPR_Version21_RLE(t *testing.T) {
	// Create synthetic v2.1 SPR with RLE compression
	spr := buildSyntheticSPR(2, 1, 1, 0, true)

	parsed, err := ParseSPR(spr)
	if err != nil {
		t.Fatalf("failed to parse synthetic v2.1 SPR: %v", err)
	}

	if parsed.Version.Major != 2 || parsed.Version.Minor != 1 {
		t.Errorf("expected version 2.1, got %s", parsed.Version)
	}

	if len(parsed.Images) != 1 {
		t.Errorf("expected 1 image, got %d", len(parsed.Images))
	}

	// Check decompression worked
	img := parsed.Images[0]
	if img.Width != 4 || img.Height != 4 {
		t.Errorf("expected 4x4 image, got %dx%d", img.Width, img.Height)
	}
}

func TestDecompressRLE(t *testing.T) {
	tests := []struct {
		name       string
		compressed []byte
		targetSize int
		expected   []byte
	}{
		{
			name:       "literal bytes",
			compressed: []byte{1, 2, 3, 4},
			targetSize: 4,
			expected:   []byte{1, 2, 3, 4},
		},
		{
			name:       "run of zeros",
			compressed: []byte{0x00, 0x04}, // 4 zeros
			targetSize: 4,
			expected:   []byte{0, 0, 0, 0},
		},
		{
			name:       "single zero",
			compressed: []byte{0x00, 0x00}, // single zero
			targetSize: 1,
			expected:   []byte{0},
		},
		{
			name:       "mixed",
			compressed: []byte{1, 0x00, 0x02, 2}, // 1, [2 zeros], 2
			targetSize: 4,
			expected:   []byte{1, 0, 0, 2},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := decompressRLE(tt.compressed, tt.targetSize)
			if !bytes.Equal(result, tt.expected) {
				t.Errorf("got %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestParseSPR_GeneratedFile(t *testing.T) {
	// Load generated test SPR file
	testFile := filepath.Join("testdata", "test.spr")
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Skip("testdata/test.spr not found, run: go run testdata/generate_spr.go")
	}

	spr, err := ParseSPRFile(testFile)
	if err != nil {
		t.Fatalf("failed to parse test SPR file: %v", err)
	}

	// Verify version
	if spr.Version.Major != 2 || spr.Version.Minor != 1 {
		t.Errorf("expected version 2.1, got %s", spr.Version)
	}

	// Verify image count: 2 indexed + 1 true-color = 3 images
	if len(spr.Images) != 3 {
		t.Errorf("expected 3 images, got %d", len(spr.Images))
	}

	// Verify first indexed image (4x4 checkerboard)
	if len(spr.Images) > 0 {
		img := spr.Images[0]
		if img.Width != 4 || img.Height != 4 {
			t.Errorf("first image: expected 4x4, got %dx%d", img.Width, img.Height)
		}
		expectedPixels := 4 * 4 * 4 // 4x4 pixels * 4 bytes RGBA
		if len(img.Pixels) != expectedPixels {
			t.Errorf("first image pixel data: expected %d bytes, got %d", expectedPixels, len(img.Pixels))
		}
	}

	// Verify second indexed image (2x2)
	if len(spr.Images) > 1 {
		img := spr.Images[1]
		if img.Width != 2 || img.Height != 2 {
			t.Errorf("second image: expected 2x2, got %dx%d", img.Width, img.Height)
		}
	}

	// Verify third image (true-color 2x2)
	if len(spr.Images) > 2 {
		img := spr.Images[2]
		if img.Width != 2 || img.Height != 2 {
			t.Errorf("third image: expected 2x2, got %dx%d", img.Width, img.Height)
		}
		// Check first pixel is red (RGBA: 255, 0, 0, 255)
		if img.Pixels[0] != 255 || img.Pixels[1] != 0 || img.Pixels[2] != 0 || img.Pixels[3] != 255 {
			t.Errorf("first pixel should be red, got RGBA(%d,%d,%d,%d)",
				img.Pixels[0], img.Pixels[1], img.Pixels[2], img.Pixels[3])
		}
	}

	// Verify palette exists
	if spr.Palette == nil {
		t.Error("expected palette to be set")
	}
}

func TestParseSPR_InvalidImage(t *testing.T) {
	// Test handling of invalid (-1,-1) image dimensions
	spr := buildSPRWithInvalidImage()

	parsed, err := ParseSPR(spr)
	if err != nil {
		t.Fatalf("failed to parse SPR with invalid image: %v", err)
	}

	// Invalid images should be replaced with 1x1 transparent
	if len(parsed.Images) != 1 {
		t.Errorf("expected 1 image, got %d", len(parsed.Images))
	}

	img := parsed.Images[0]
	if img.Width != 1 || img.Height != 1 {
		t.Errorf("expected 1x1 placeholder, got %dx%d", img.Width, img.Height)
	}
}

// buildSyntheticSPR creates a synthetic SPR file for testing.
func buildSyntheticSPR(major, minor uint8, indexedCount, trueColorCount int, useRLE bool) []byte {
	var buf bytes.Buffer

	// Header
	buf.WriteString("SP")
	buf.WriteByte(minor)
	buf.WriteByte(major)

	// Image counts
	binary.Write(&buf, binary.LittleEndian, uint16(indexedCount))
	if major >= 2 {
		binary.Write(&buf, binary.LittleEndian, uint16(trueColorCount))
	}

	// Indexed images
	for i := 0; i < indexedCount; i++ {
		if useRLE {
			// 4x4 RLE image: 4 transparent, color1, 6 transparent, color2, 5 transparent
			binary.Write(&buf, binary.LittleEndian, uint16(4)) // width
			binary.Write(&buf, binary.LittleEndian, uint16(4)) // height
			rle := []byte{0x00, 0x04, 0x01, 0x00, 0x06, 0x02, 0x00, 0x05}
			binary.Write(&buf, binary.LittleEndian, uint16(len(rle))) // compressed size
			buf.Write(rle)
		} else {
			// 2x2 uncompressed image
			binary.Write(&buf, binary.LittleEndian, uint16(2)) // width
			binary.Write(&buf, binary.LittleEndian, uint16(2)) // height
			buf.Write([]byte{0, 1, 2, 3})                      // 4 pixel indices
		}
	}

	// True-color images (ABGR format)
	for i := 0; i < trueColorCount; i++ {
		binary.Write(&buf, binary.LittleEndian, uint16(2)) // width
		binary.Write(&buf, binary.LittleEndian, uint16(2)) // height
		// 4 pixels in ABGR format
		buf.Write([]byte{
			255, 0, 0, 255, // ABGR: A=255, B=0, G=0, R=255 -> RGBA: R=255, G=0, B=0, A=255
			255, 0, 255, 0, // ABGR: A=255, B=0, G=255, R=0 -> RGBA: R=0, G=255, B=0, A=255
			255, 255, 0, 0, // ABGR: A=255, B=255, G=0, R=0 -> RGBA: R=0, G=0, B=255, A=255
			128, 128, 128, 128, // ABGR: A=128, B=128, G=128, R=128 -> RGBA: R=128, G=128, B=128, A=128
		})
	}

	// Palette (1024 bytes)
	palette := make([]byte, 1024)
	// Color 0: transparent (ignored)
	palette[0], palette[1], palette[2], palette[3] = 0, 0, 0, 0
	// Color 1: red
	palette[4], palette[5], palette[6], palette[7] = 255, 0, 0, 255
	// Color 2: green
	palette[8], palette[9], palette[10], palette[11] = 0, 255, 0, 255
	// Color 3: blue
	palette[12], palette[13], palette[14], palette[15] = 0, 0, 255, 255
	buf.Write(palette)

	return buf.Bytes()
}

// buildSPRWithInvalidImage creates an SPR with (-1,-1) dimensions.
func buildSPRWithInvalidImage() []byte {
	var buf bytes.Buffer

	// Header (v2.0)
	buf.WriteString("SP")
	buf.WriteByte(0) // minor
	buf.WriteByte(2) // major

	// Image counts
	binary.Write(&buf, binary.LittleEndian, uint16(1)) // 1 indexed
	binary.Write(&buf, binary.LittleEndian, uint16(0)) // 0 true-color

	// Invalid image (-1, -1) = (0xFFFF, 0xFFFF)
	binary.Write(&buf, binary.LittleEndian, uint16(0xFFFF))
	binary.Write(&buf, binary.LittleEndian, uint16(0xFFFF))

	// Palette
	buf.Write(make([]byte, 1024))

	return buf.Bytes()
}
