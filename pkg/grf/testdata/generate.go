//go:build ignore

// This program generates a small test GRF file for unit tests.
// Run with: go run generate.go
package main

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"os"
)

const grfMagic = "Master of Magic"

func main() {
	// Test files to include
	files := []struct {
		name    string
		content []byte
	}{
		{"data/test.txt", []byte("Hello, GRF!")},
		{"data/sprite/test.spr", append([]byte("SP"), make([]byte, 10)...)}, // Fake SPR with magic
		{"data/texture/test.bmp", []byte("BM fake bitmap data")},
		{"data/subfolder/nested/file.txt", []byte("Nested file content")},
	}

	f, err := os.Create("test.grf")
	if err != nil {
		panic(err)
	}
	defer f.Close()

	// Write header placeholder (will update later)
	header := make([]byte, 46)
	copy(header[0:15], grfMagic)
	// EncryptionKey: 15 bytes zeros
	// TableOffset: will fill later
	// Seed: 0
	// FileCount: will fill later
	binary.LittleEndian.PutUint32(header[42:], 0x200) // Version
	f.Write(header)

	// Write file data and build entries
	type entry struct {
		name             string
		compressedSize   uint32
		alignedSize      uint32
		uncompressedSize uint32
		flags            uint8
		offset           uint32
	}
	var entries []entry

	currentOffset := uint32(0) // Offset relative to header end

	for _, file := range files {
		// Compress the content
		var compressed bytes.Buffer
		w := zlib.NewWriter(&compressed)
		w.Write(file.content)
		w.Close()

		compressedData := compressed.Bytes()

		// Align to 8 bytes
		alignedSize := uint32(len(compressedData))
		if alignedSize%8 != 0 {
			alignedSize += 8 - (alignedSize % 8)
		}

		entries = append(entries, entry{
			name:             file.name,
			compressedSize:   uint32(len(compressedData)),
			alignedSize:      alignedSize,
			uncompressedSize: uint32(len(file.content)),
			flags:            0x01, // FILE flag
			offset:           currentOffset,
		})

		// Write compressed data with padding
		f.Write(compressedData)
		padding := make([]byte, alignedSize-uint32(len(compressedData)))
		f.Write(padding)

		currentOffset += alignedSize
	}

	// Build file table
	var tableData bytes.Buffer
	for _, e := range entries {
		// Filename (null-terminated, use backslashes like original GRF)
		name := bytes.ReplaceAll([]byte(e.name), []byte("/"), []byte("\\"))
		tableData.Write(name)
		tableData.WriteByte(0)

		// Entry data: compressedSize(4) + alignedSize(4) + uncompressedSize(4) + flags(1) + offset(4)
		binary.Write(&tableData, binary.LittleEndian, e.compressedSize)
		binary.Write(&tableData, binary.LittleEndian, e.alignedSize)
		binary.Write(&tableData, binary.LittleEndian, e.uncompressedSize)
		tableData.WriteByte(e.flags)
		binary.Write(&tableData, binary.LittleEndian, e.offset)
	}

	// Compress file table
	var compressedTable bytes.Buffer
	tw := zlib.NewWriter(&compressedTable)
	tw.Write(tableData.Bytes())
	tw.Close()

	// Record table offset (relative to start of file, minus 46 for header)
	tableOffset := currentOffset

	// Write table header: compressedSize + uncompressedSize
	binary.Write(f, binary.LittleEndian, uint32(compressedTable.Len()))
	binary.Write(f, binary.LittleEndian, uint32(tableData.Len()))
	f.Write(compressedTable.Bytes())

	// Update header with table offset and file count
	// FileCount in GRF is: actualCount + seed + 7 (seed is 0)
	fileCount := uint32(len(entries)) + 7

	f.Seek(30, 0)
	binary.Write(f, binary.LittleEndian, tableOffset) // TableOffset
	binary.Write(f, binary.LittleEndian, uint32(0))   // Seed
	binary.Write(f, binary.LittleEndian, fileCount)   // FileCount

	println("Generated test.grf with", len(entries), "files")
}
