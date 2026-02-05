package ui

import (
	"bytes"
	"fmt"
	"image"
	"strings"

	"github.com/Faultbox/midgard-ro/internal/engine/texture"
	"github.com/Faultbox/midgard-ro/internal/engine/ui2d"
)

// TextureInfo holds GPU texture metadata.
type TextureInfo struct {
	ID     uint32
	Width  int
	Height int
}

// TextureCache loads images from GRF archives and caches them as GPU textures.
type TextureCache struct {
	renderer *ui2d.Renderer
	loadFunc func(string) ([]byte, error)
	cache    map[string]*TextureInfo
}

// NewTextureCache creates a new texture cache.
func NewTextureCache(renderer *ui2d.Renderer, loadFunc func(string) ([]byte, error)) *TextureCache {
	return &TextureCache{
		renderer: renderer,
		loadFunc: loadFunc,
		cache:    make(map[string]*TextureInfo),
	}
}

// normalizePath converts backslashes to forward slashes and lowercases for consistent cache keys.
func normalizePath(path string) string {
	return strings.ToLower(strings.ReplaceAll(path, "\\", "/"))
}

// Load loads (or returns cached) a texture from the given GRF path.
// Applies magenta key transparency for BMP/TGA images.
func (tc *TextureCache) Load(grfPath string) (*TextureInfo, error) {
	key := normalizePath(grfPath)

	if info, ok := tc.cache[key]; ok {
		return info, nil
	}

	data, err := tc.loadFunc(grfPath)
	if err != nil {
		return nil, fmt.Errorf("loading texture %s: %w", grfPath, err)
	}

	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decoding texture %s: %w", grfPath, err)
	}

	// Convert to RGBA with magenta key transparency
	rgba := texture.ImageToRGBA(img, true)
	bounds := rgba.Bounds()

	texID := tc.renderer.CreateTexture(bounds.Dx(), bounds.Dy(), rgba.Pix)

	info := &TextureInfo{
		ID:     texID,
		Width:  bounds.Dx(),
		Height: bounds.Dy(),
	}
	tc.cache[key] = info
	return info, nil
}

// Get returns a cached texture or nil if not loaded.
func (tc *TextureCache) Get(path string) *TextureInfo {
	return tc.cache[normalizePath(path)]
}

// Close releases all cached GPU textures.
func (tc *TextureCache) Close() {
	for _, info := range tc.cache {
		tc.renderer.DeleteTexture(info.ID)
	}
	tc.cache = nil
}
