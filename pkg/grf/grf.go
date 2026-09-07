// Package grf provides reading functionality for Ragnarok Online GRF archives.
package grf

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"strings"
)

const grfMagic = "Master of Magic"

// Archive represents an opened GRF archive.
type Archive struct {
	file     *os.File
	header   Header
	fileList map[string]*Entry
	size     int64
}

// Header contains GRF file header information.
type Header struct {
	Magic         [15]byte
	EncryptionKey [15]byte
	TableOffset   uint32
	Seed          uint32
	FileCount     uint32
	Version       uint32
}

// Entry represents a file entry in the archive.
type Entry struct {
	Name             string
	CompressedSize   uint32
	AlignedSize      uint32
	UncompressedSize uint32
	Flags            uint8
	Offset           uint32
}

// Open opens a GRF archive for reading.
func Open(path string) (*Archive, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening file: %w", err)
	}

	stat, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, err
	}
	archive := &Archive{
		size:     stat.Size(),
		file:     file,
		fileList: make(map[string]*Entry),
	}

	if err := archive.readHeader(); err != nil {
		file.Close()
		return nil, fmt.Errorf("reading header: %w", err)
	}

	if err := archive.readFileTable(); err != nil {
		file.Close()
		return nil, fmt.Errorf("reading file table: %w", err)
	}

	return archive, nil
}

// Close closes the archive.
func (a *Archive) Close() error {
	if a.file != nil {
		return a.file.Close()
	}
	return nil
}

func (a *Archive) readHeader() error {
	if _, err := a.file.Seek(0, io.SeekStart); err != nil {
		return err
	}

	if err := binary.Read(a.file, binary.LittleEndian, &a.header); err != nil {
		return fmt.Errorf("reading header: %w", err)
	}

	if string(a.header.Magic[:]) != grfMagic {
		return fmt.Errorf("invalid GRF magic")
	}

	if a.header.Version != 0x200 {
		return fmt.Errorf("unsupported GRF version: 0x%x", a.header.Version)
	}

	return nil
}

func (a *Archive) readFileTable() error {
	tableOffset := int64(a.header.TableOffset) + 46
	if tableOffset < 46 || tableOffset+8 > a.size {
		return fmt.Errorf("invalid GRF table offset")
	}
	var sizes [8]byte
	if _, err := a.file.ReadAt(sizes[:], tableOffset); err != nil {
		return err
	}
	compressedSize := binary.LittleEndian.Uint32(sizes[:4])
	uncompressedSize := binary.LittleEndian.Uint32(sizes[4:])
	if compressedSize > 64<<20 || uncompressedSize > 256<<20 || int64(compressedSize) > a.size-tableOffset-8 {
		return fmt.Errorf("invalid or oversized GRF table")
	}
	compressedData := make([]byte, int(compressedSize))
	if _, err := a.file.ReadAt(compressedData, tableOffset+8); err != nil {
		return err
	}
	reader, err := zlib.NewReader(bytes.NewReader(compressedData))
	if err != nil {
		return fmt.Errorf("table zlib: %w", err)
	}
	defer reader.Close()
	tableData, err := io.ReadAll(io.LimitReader(reader, int64(uncompressedSize)+1))
	if err != nil {
		return err
	}
	if len(tableData) != int(uncompressedSize) {
		return fmt.Errorf("GRF table size mismatch")
	}
	if uint64(a.header.FileCount) < uint64(a.header.Seed)+7 {
		return fmt.Errorf("invalid GRF file count")
	}
	fileCount := a.header.FileCount - a.header.Seed - 7
	if uint64(fileCount)*18 > uint64(len(tableData)) {
		return fmt.Errorf("truncated GRF table")
	}
	offset := 0
	for i := uint32(0); i < fileCount; i++ {
		if offset >= len(tableData) {
			return fmt.Errorf("truncated GRF entry")
		}
		nameEnd := bytes.IndexByte(tableData[offset:], 0)
		if nameEnd < 0 {
			return fmt.Errorf("unterminated GRF entry name")
		}
		name := string(tableData[offset : offset+nameEnd])
		offset += nameEnd + 1
		if offset+17 > len(tableData) {
			return fmt.Errorf("truncated GRF entry metadata")
		}
		entry := &Entry{
			Name:             normalizePath(name),
			CompressedSize:   binary.LittleEndian.Uint32(tableData[offset:]),
			AlignedSize:      binary.LittleEndian.Uint32(tableData[offset+4:]),
			UncompressedSize: binary.LittleEndian.Uint32(tableData[offset+8:]),
			Flags:            tableData[offset+12],
			Offset:           binary.LittleEndian.Uint32(tableData[offset+13:]),
		}
		offset += 17
		if entry.Flags&1 != 0 {
			a.fileList[entry.Name] = entry
		}
	}
	return nil
}

// Entry returns a copy of the metadata, without exposing mutable index state.
func (a *Archive) Entry(path string) (Entry, bool) {
	e, ok := a.fileList[normalizePath(path)]
	if !ok {
		return Entry{}, false
	}
	return *e, true
}

// List returns all file paths in the archive.
func (a *Archive) List() []string {
	result := make([]string, 0, len(a.fileList))
	for path := range a.fileList {
		result = append(result, path)
	}
	return result
}

// Contains checks if a file exists.
func (a *Archive) Contains(path string) bool {
	_, ok := a.fileList[normalizePath(path)]
	return ok
}

// Read reads a file from the archive.
func (a *Archive) Read(path string) ([]byte, error) {
	return a.ReadLimit(path, 512<<20)
}

// ReadLimit bounds decompression for previews. ReadAt keeps concurrent reads
// independent; callers must still keep the archive open until reads finish.
func (a *Archive) ReadLimit(path string, maxBytes int64) ([]byte, error) {
	entry, ok := a.fileList[normalizePath(path)]
	if !ok {
		return nil, fmt.Errorf("file not found: %s", path)
	}
	if entry.Flags&0x06 != 0 {
		return nil, fmt.Errorf("encrypted files not yet supported")
	}
	if maxBytes < 0 || int64(entry.UncompressedSize) > maxBytes || int64(entry.CompressedSize) > maxBytes {
		return nil, fmt.Errorf("GRF entry exceeds read limit")
	}
	offset := int64(entry.Offset) + 46
	if entry.CompressedSize > entry.AlignedSize || offset > a.size || int64(entry.AlignedSize) > a.size-offset {
		return nil, fmt.Errorf("invalid GRF entry bounds")
	}
	data := make([]byte, int(entry.CompressedSize))
	if _, err := a.file.ReadAt(data, offset); err != nil {
		return nil, err
	}
	if entry.CompressedSize == entry.UncompressedSize {
		return data, nil
	}
	reader, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	result, err := io.ReadAll(io.LimitReader(reader, int64(entry.UncompressedSize)+1))
	if err != nil {
		return nil, err
	}
	if len(result) != int(entry.UncompressedSize) {
		return nil, fmt.Errorf("GRF entry size mismatch")
	}
	return result, nil
}

func normalizePath(path string) string {
	path = strings.ReplaceAll(path, "\\", "/")
	return asciiToLower(path)
}

// asciiToLower converts ASCII letters to lowercase while preserving high bytes.
// This is necessary because GRF filenames use EUC-KR encoding for Korean,
// and strings.ToLower corrupts non-UTF-8 byte sequences.
func asciiToLower(s string) string {
	b := []byte(s)
	for i := range b {
		if b[i] >= 'A' && b[i] <= 'Z' {
			b[i] = b[i] + 32 // Convert A-Z to a-z
		}
	}
	return string(b)
}
