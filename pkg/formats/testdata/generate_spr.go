//go:build ignore

// This program generates a test SPR file for unit tests.
// Run with: go run generate_spr.go
package main

import (
	"bytes"
	"encoding/binary"
	"os"
)

func main() {
	// Generate a v2.1 SPR file with 2 indexed images (RLE) and 1 true-color image
	var buf bytes.Buffer

	// Header
	buf.WriteString("SP")
	buf.WriteByte(1)                                       // minor = 1
	buf.WriteByte(2)                                       // major = 2 (so version 2.1)
	binary.Write(&buf, binary.LittleEndian, uint16(2))     // 2 indexed images
	binary.Write(&buf, binary.LittleEndian, uint16(1))     // 1 true-color image

	// Indexed image 1: 4x4 with RLE compression
	// Pattern: checkerboard of transparent (0) and color 1
	binary.Write(&buf, binary.LittleEndian, uint16(4))     // width
	binary.Write(&buf, binary.LittleEndian, uint16(4))     // height
	// RLE data for: 0,1,0,1, 1,0,1,0, 0,1,0,1, 1,0,1,0 (checkerboard)
	// = 0x00 0x01, 0x01, 0x00 0x01, 0x01, 0x01, 0x00 0x01, 0x01, 0x00 0x01, ...
	rle1 := []byte{
		0x00, 0x01, // 1 zero
		0x01,       // literal 1
		0x00, 0x01, // 1 zero
		0x01,       // literal 1
		0x01,       // literal 1
		0x00, 0x01, // 1 zero
		0x01,       // literal 1
		0x00, 0x01, // 1 zero
		0x00, 0x01, // 1 zero
		0x01,       // literal 1
		0x00, 0x01, // 1 zero
		0x01,       // literal 1
		0x01,       // literal 1
		0x00, 0x01, // 1 zero
		0x01,       // literal 1
		0x00, 0x01, // 1 zero
	}
	binary.Write(&buf, binary.LittleEndian, uint16(len(rle1)))
	buf.Write(rle1)

	// Indexed image 2: 2x2 simple image
	binary.Write(&buf, binary.LittleEndian, uint16(2))     // width
	binary.Write(&buf, binary.LittleEndian, uint16(2))     // height
	rle2 := []byte{0x01, 0x02, 0x03, 0x04}                 // 4 palette indices (no zeros = no RLE)
	binary.Write(&buf, binary.LittleEndian, uint16(len(rle2)))
	buf.Write(rle2)

	// True-color image: 2x2 RGBA
	binary.Write(&buf, binary.LittleEndian, uint16(2))     // width
	binary.Write(&buf, binary.LittleEndian, uint16(2))     // height
	// ABGR pixels: Red, Green, Blue, White
	buf.Write([]byte{
		255, 0, 0, 255,     // ABGR -> Red (A=255, B=0, G=0, R=255)
		255, 0, 255, 0,     // ABGR -> Green (A=255, B=0, G=255, R=0)
		255, 255, 0, 0,     // ABGR -> Blue (A=255, B=255, G=0, R=0)
		255, 255, 255, 255, // ABGR -> White (A=255, B=255, G=255, R=255)
	})

	// Palette (256 colors * 4 bytes = 1024 bytes)
	palette := make([]byte, 1024)
	// Color 0: magenta (transparent marker, but ignored)
	palette[0], palette[1], palette[2], palette[3] = 255, 0, 255, 255
	// Color 1: red
	palette[4], palette[5], palette[6], palette[7] = 255, 0, 0, 255
	// Color 2: green
	palette[8], palette[9], palette[10], palette[11] = 0, 255, 0, 255
	// Color 3: blue
	palette[12], palette[13], palette[14], palette[15] = 0, 0, 255, 255
	// Color 4: yellow
	palette[16], palette[17], palette[18], palette[19] = 255, 255, 0, 255
	buf.Write(palette)

	// Write to file
	if err := os.WriteFile("test.spr", buf.Bytes(), 0644); err != nil {
		panic(err)
	}

	println("Generated test.spr:", buf.Len(), "bytes")
	println("  - 2 indexed images (4x4 and 2x2)")
	println("  - 1 true-color image (2x2)")
}
