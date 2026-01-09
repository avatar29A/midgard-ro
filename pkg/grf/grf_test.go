package grf

import (
	"testing"
)

// testGRFPath returns path to the test fixture GRF file
func testGRFPath() string {
	return "testdata/test.grf"
}

func TestOpen(t *testing.T) {
	archive, err := Open(testGRFPath())
	if err != nil {
		t.Fatalf("failed to open GRF: %v", err)
	}
	defer archive.Close()

	if archive.header.Version != 0x200 {
		t.Errorf("expected version 0x200, got 0x%x", archive.header.Version)
	}
	if len(archive.fileList) != 4 {
		t.Errorf("expected 4 files, got %d", len(archive.fileList))
	}
}

func TestList(t *testing.T) {
	archive, err := Open(testGRFPath())
	if err != nil {
		t.Fatalf("failed to open GRF: %v", err)
	}
	defer archive.Close()

	files := archive.List()
	if len(files) != 4 {
		t.Errorf("expected 4 files, got %d", len(files))
	}

	// Check expected files exist
	expected := []string{
		"data/test.txt",
		"data/sprite/test.spr",
		"data/texture/test.bmp",
		"data/subfolder/nested/file.txt",
	}
	for _, exp := range expected {
		found := false
		for _, f := range files {
			if f == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected file not found: %s", exp)
		}
	}
}

func TestContains(t *testing.T) {
	archive, err := Open(testGRFPath())
	if err != nil {
		t.Fatalf("failed to open GRF: %v", err)
	}
	defer archive.Close()

	// Test existing file
	if !archive.Contains("data/test.txt") {
		t.Error("Contains returned false for existing file")
	}

	// Test with backslashes (should be normalized)
	if !archive.Contains("data\\test.txt") {
		t.Error("Contains failed with backslash path")
	}

	// Test case insensitivity
	if !archive.Contains("DATA/TEST.TXT") {
		t.Error("Contains is not case insensitive")
	}

	// Test non-existent file
	if archive.Contains("nonexistent/file.txt") {
		t.Error("Contains returned true for non-existent file")
	}
}

func TestRead(t *testing.T) {
	archive, err := Open(testGRFPath())
	if err != nil {
		t.Fatalf("failed to open GRF: %v", err)
	}
	defer archive.Close()

	// Read text file
	data, err := archive.Read("data/test.txt")
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	expected := "Hello, GRF!"
	if string(data) != expected {
		t.Errorf("expected %q, got %q", expected, string(data))
	}
}

func TestReadSprite(t *testing.T) {
	archive, err := Open(testGRFPath())
	if err != nil {
		t.Fatalf("failed to open GRF: %v", err)
	}
	defer archive.Close()

	data, err := archive.Read("data/sprite/test.spr")
	if err != nil {
		t.Fatalf("failed to read sprite: %v", err)
	}

	// Check SPR magic
	if len(data) < 2 || string(data[:2]) != "SP" {
		t.Error("invalid SPR magic")
	}
}

func TestReadNestedFile(t *testing.T) {
	archive, err := Open(testGRFPath())
	if err != nil {
		t.Fatalf("failed to open GRF: %v", err)
	}
	defer archive.Close()

	data, err := archive.Read("data/subfolder/nested/file.txt")
	if err != nil {
		t.Fatalf("failed to read nested file: %v", err)
	}

	expected := "Nested file content"
	if string(data) != expected {
		t.Errorf("expected %q, got %q", expected, string(data))
	}
}

func TestReadNonExistent(t *testing.T) {
	archive, err := Open(testGRFPath())
	if err != nil {
		t.Fatalf("failed to open GRF: %v", err)
	}
	defer archive.Close()

	_, err = archive.Read("nonexistent/file.txt")
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestOpenInvalidFile(t *testing.T) {
	_, err := Open("nonexistent.grf")
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}
