package ui

import (
	"fmt"
	"image"
	"strings"

	"github.com/Faultbox/midgard-ro/pkg/formats"

	"github.com/Faultbox/midgard-ro/internal/engine/texture"
	"github.com/Faultbox/midgard-ro/internal/engine/ui2d"
	"github.com/Faultbox/midgard-ro/internal/game/states"
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
//
// UI art loads with nearest filtering: these are pixel-art BMPs that get
// magnified when the UI is stretched across a HiDPI framebuffer, and linear
// filtering blurs their one-pixel borders and bevels into mush.
func (tc *TextureCache) Load(grfPath string) (*TextureInfo, error) {
	return tc.load(grfPath, false)
}

// LoadSmooth is the same with linear filtering, for the art that is drawn out
// in the world rather than on the interface.
//
// An effect is magnified by however far away it is, and nothing about it is
// aligned to a pixel — a shard of ice a few dozen pixels across is drawn at
// whatever size the camera puts it. Nearest turns that into a staircase; the
// bevels and one-pixel borders that nearest is here to protect are an
// interface thing and there are none out there.
func (tc *TextureCache) LoadSmooth(grfPath string) (*TextureInfo, error) {
	return tc.load(grfPath, true)
}

func (tc *TextureCache) load(grfPath string, smooth bool) (*TextureInfo, error) {
	key := normalizePath(grfPath)
	if smooth {
		key += "|smooth"
	}

	if info, ok := tc.cache[key]; ok {
		return info, nil
	}

	sprPath, frame, isFrame := states.SpriteFrameOf(grfPath)
	if isFrame {
		grfPath = sprPath
	}

	data, err := tc.loadFunc(grfPath)
	if err != nil {
		return nil, fmt.Errorf("loading texture %s: %w", grfPath, err)
	}

	var img image.Image

	if isFrame {
		img, err = spriteFrame(data, frame)
	} else {
		img, err = formats.DecodeImage(data)
	}

	if err != nil {
		return nil, fmt.Errorf("decoding texture %s: %w", grfPath, err)
	}

	// Convert to RGBA with magenta key transparency
	rgba := texture.ImageToRGBA(img, true)
	bounds := rgba.Bounds()

	upload := tc.renderer.CreateTextureNearest
	if smooth {
		upload = tc.renderer.CreateTexture
	}

	texID := upload(bounds.Dx(), bounds.Dy(), rgba.Pix)

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

// spriteFrame is one frame of a sprite, as an image.
//
// The archive keeps some effects as sprites rather than as textures — the
// jagged ice a frozen target is sealed in is seven frames of one — and a
// frame of those is asked for the same way a file is, so everything that
// draws through this cache can take either.
func spriteFrame(data []byte, frame int) (image.Image, error) {
	spr, err := formats.ParseSPR(data)
	if err != nil {
		return nil, err
	}

	if frame < 0 || frame >= len(spr.Images) {
		return nil, fmt.Errorf("frame %d of %d", frame, len(spr.Images))
	}

	f := spr.Images[frame]
	img := image.NewRGBA(image.Rect(0, 0, int(f.Width), int(f.Height)))
	copy(img.Pix, f.Pixels)

	return img, nil
}
