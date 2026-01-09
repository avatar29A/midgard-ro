package grf

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// testGRFPath returns path to test GRF file if it exists
func testGRFPath() string {
	// Check for data directory relative to project root
	paths := []string{
		"../../data/rdata.grf", // Smaller file for faster tests
		"../../data/data.grf",
	}
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func TestOpen(t *testing.T) {
	path := testGRFPath()
	if path == "" {
		t.Skip("No GRF file available for testing")
	}

	archive, err := Open(path)
	if err != nil {
		t.Fatalf("failed to open GRF: %v", err)
	}
	defer archive.Close()

	t.Logf("Opened: %s", path)
	t.Logf("Version: 0x%x", archive.header.Version)
	t.Logf("File count: %d", len(archive.fileList))
}

func TestList(t *testing.T) {
	path := testGRFPath()
	if path == "" {
		t.Skip("No GRF file available for testing")
	}

	archive, err := Open(path)
	if err != nil {
		t.Fatalf("failed to open GRF: %v", err)
	}
	defer archive.Close()

	files := archive.List()
	t.Logf("Total files: %d", len(files))

	// Show first 10 files
	count := 10
	if len(files) < count {
		count = len(files)
	}
	t.Log("Sample files:")
	for i := 0; i < count; i++ {
		t.Logf("  %s", files[i])
	}

	// Count by extension
	extCount := make(map[string]int)
	for _, f := range files {
		ext := strings.ToLower(filepath.Ext(f))
		extCount[ext]++
	}
	t.Log("Files by extension:")
	for ext, count := range extCount {
		if count > 100 {
			t.Logf("  %s: %d", ext, count)
		}
	}
}

func TestContains(t *testing.T) {
	path := testGRFPath()
	if path == "" {
		t.Skip("No GRF file available for testing")
	}

	archive, err := Open(path)
	if err != nil {
		t.Fatalf("failed to open GRF: %v", err)
	}
	defer archive.Close()

	files := archive.List()
	if len(files) == 0 {
		t.Fatal("no files in archive")
	}

	// Test with first file
	firstFile := files[0]
	if !archive.Contains(firstFile) {
		t.Errorf("Contains returned false for existing file: %s", firstFile)
	}

	// Test with non-existent file
	if archive.Contains("nonexistent/file/path.txt") {
		t.Error("Contains returned true for non-existent file")
	}

	// Test case insensitivity
	upperFile := strings.ToUpper(firstFile)
	if !archive.Contains(upperFile) {
		t.Logf("Case sensitivity issue: %s vs %s", firstFile, upperFile)
	}
}

func TestRead(t *testing.T) {
	path := testGRFPath()
	if path == "" {
		t.Skip("No GRF file available for testing")
	}

	archive, err := Open(path)
	if err != nil {
		t.Fatalf("failed to open GRF: %v", err)
	}
	defer archive.Close()

	files := archive.List()

	// Find a small file to read (prefer .txt or .xml)
	var testFile string
	for _, f := range files {
		ext := strings.ToLower(filepath.Ext(f))
		if ext == ".txt" || ext == ".xml" || ext == ".lua" {
			entry := archive.fileList[f]
			if entry.UncompressedSize < 10000 { // < 10KB
				testFile = f
				break
			}
		}
	}

	if testFile == "" {
		// Just use first file
		testFile = files[0]
	}

	entry := archive.fileList[testFile]
	t.Logf("Reading: %s (compressed: %d, uncompressed: %d, flags: 0x%x)",
		testFile, entry.CompressedSize, entry.UncompressedSize, entry.Flags)

	data, err := archive.Read(testFile)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	t.Logf("Read %d bytes", len(data))
	if len(data) != int(entry.UncompressedSize) {
		t.Errorf("size mismatch: got %d, expected %d", len(data), entry.UncompressedSize)
	}

	// Show first 100 bytes if text-ish
	if len(data) > 0 && data[0] < 128 {
		preview := string(data)
		if len(preview) > 200 {
			preview = preview[:200] + "..."
		}
		t.Logf("Content preview:\n%s", preview)
	}
}

func TestReadSprite(t *testing.T) {
	path := testGRFPath()
	if path == "" {
		t.Skip("No GRF file available for testing")
	}

	archive, err := Open(path)
	if err != nil {
		t.Fatalf("failed to open GRF: %v", err)
	}
	defer archive.Close()

	// Find a sprite file
	var sprFile string
	for _, f := range archive.List() {
		if strings.HasSuffix(f, ".spr") {
			sprFile = f
			break
		}
	}

	if sprFile == "" {
		t.Skip("No .spr files found")
	}

	entry := archive.fileList[sprFile]
	t.Logf("Reading sprite: %s (size: %d)", sprFile, entry.UncompressedSize)

	data, err := archive.Read(sprFile)
	if err != nil {
		t.Fatalf("failed to read sprite: %v", err)
	}

	t.Logf("Read %d bytes", len(data))

	// Check SPR magic (first 2 bytes should be "SP")
	if len(data) >= 2 {
		magic := string(data[:2])
		t.Logf("SPR magic: %q", magic)
		if magic != "SP" {
			t.Error("invalid SPR magic")
		}
	}
}
