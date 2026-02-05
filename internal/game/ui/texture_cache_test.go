package ui

import (
	"testing"
)

func TestNormalizePath(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{`data\texture\ui\frame.bmp`, "data/texture/ui/frame.bmp"},
		{`Data\Texture\UI\Frame.BMP`, "data/texture/ui/frame.bmp"},
		{"data/texture/ui/frame.bmp", "data/texture/ui/frame.bmp"},
		{"", ""},
	}

	for _, tt := range tests {
		got := normalizePath(tt.input)
		if got != tt.want {
			t.Errorf("normalizePath(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestNormalizePath_ConsistentKeys(t *testing.T) {
	// Backslash vs forward slash and mixed case should produce the same key
	key1 := normalizePath(`data\texture\UI\Frame.BMP`)
	key2 := normalizePath(`data/texture/ui/frame.bmp`)
	if key1 != key2 {
		t.Errorf("path normalization mismatch: %q != %q", key1, key2)
	}

	// Korean paths should also normalize consistently
	key3 := normalizePath(`data\texture\유저인터페이스\basic_interface\win_msgbox.bmp`)
	key4 := normalizePath(`data/texture/유저인터페이스/basic_interface/win_msgbox.bmp`)
	if key3 != key4 {
		t.Errorf("Korean path normalization mismatch: %q != %q", key3, key4)
	}
}
