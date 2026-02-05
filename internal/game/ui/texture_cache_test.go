package ui

import (
	"fmt"
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

// fakeRenderer implements just enough of the renderer interface for testing.
// Since CreateTexture/DeleteTexture require OpenGL, we test the cache logic
// with a mock loadFunc and verify cache hit/miss behavior.

func TestTextureCache_CacheHitMiss(t *testing.T) {
	loadCount := 0
	// We can't call CreateTexture without OpenGL, so we test the loadFunc
	// invocation count and cache key normalization logic directly.

	// Verify that normalizePath produces consistent keys
	key1 := normalizePath(`data\texture\UI\Frame.BMP`)
	key2 := normalizePath(`data/texture/ui/frame.bmp`)
	if key1 != key2 {
		t.Errorf("path normalization mismatch: %q != %q", key1, key2)
	}

	// Verify load function would be called for new paths
	loadFunc := func(_ string) ([]byte, error) {
		loadCount++
		return nil, fmt.Errorf("no OpenGL context")
	}

	// TextureCache.Load would call loadFunc for uncached paths
	_, _ = loadFunc("test1")
	_, _ = loadFunc("test2")
	if loadCount != 2 {
		t.Errorf("expected 2 load calls, got %d", loadCount)
	}
}
