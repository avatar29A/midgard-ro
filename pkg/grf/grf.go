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

	archive := &Archive{
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
	if _, err := a.file.Seek(tableOffset, io.SeekStart); err != nil {
		return err
	}

	var compressedSize, uncompressedSize uint32
	binary.Read(a.file, binary.LittleEndian, &compressedSize)
	binary.Read(a.file, binary.LittleEndian, &uncompressedSize)

	compressedData := make([]byte, compressedSize)
	io.ReadFull(a.file, compressedData)

	reader, _ := zlib.NewReader(bytes.NewReader(compressedData))
	defer reader.Close()

	tableData := make([]byte, uncompressedSize)
	io.ReadFull(reader, tableData)

	fileCount := a.header.FileCount - a.header.Seed - 7
	offset := 0

	for i := uint32(0); i < fileCount; i++ {
		nameEnd := bytes.IndexByte(tableData[offset:], 0)
		if nameEnd < 0 {
			break
		}
		name := string(tableData[offset : offset+nameEnd])
		offset += nameEnd + 1

		if offset+17 > len(tableData) {
			break
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

		if entry.Flags&0x01 != 0 {
			a.fileList[entry.Name] = entry
		}
	}

	return nil
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
	entry, ok := a.fileList[normalizePath(path)]
	if !ok {
		return nil, fmt.Errorf("file not found: %s", path)
	}

	dataOffset := int64(entry.Offset) + 46
	a.file.Seek(dataOffset, io.SeekStart)

	compressedData := make([]byte, entry.AlignedSize)
	io.ReadFull(a.file, compressedData)

	if entry.Flags&0x02 != 0 {
		return nil, fmt.Errorf("encrypted files not yet supported")
	}

	if entry.CompressedSize == entry.UncompressedSize {
		return compressedData[:entry.UncompressedSize], nil
	}

	reader, err := zlib.NewReader(bytes.NewReader(compressedData[:entry.CompressedSize]))
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	result := make([]byte, entry.UncompressedSize)
	io.ReadFull(reader, result)
	return result, nil
}

func normalizePath(path string) string {
	path = strings.ReplaceAll(path, "\\", "/")
	return strings.ToLower(path)
}
