package ui

import (
	"fmt"
	gomath "math"

	"github.com/Faultbox/midgard-ro/pkg/formats"

	"go.uber.org/zap"

	"github.com/Faultbox/midgard-ro/internal/engine/texture"
	"github.com/Faultbox/midgard-ro/internal/engine/ui2d"
	"github.com/Faultbox/midgard-ro/internal/logger"
)

// The original's loading screen, transcribed from roBrowser's Background.js
// (196-222): one of the archive's loading pictures filling the window, and a
// 240×15 bar three quarters of the way down with a cyan border, a grey
// trough, a blue fill inset two pixels, and the percentage in yellow on top.
// Nothing else — no map name, no tips.
const (
	loadingBarW = float32(240)
	loadingBarH = float32(15)
	loadingBarY = float32(0.75) // of the window height
)

var (
	loadingBarBorder = ui2d.RGB(0, 255, 255)
	loadingBarTrough = ui2d.RGB(140, 140, 140)
	loadingBarFill   = ui2d.RGB(66, 99, 165)
	loadingBarText   = ui2d.RGB(255, 255, 0)
	loadingError     = ui2d.RGB(255, 80, 80)
)

// loadingImagePath is where the archive keeps the loading screens:
// loading00.jpg … loading10.jpg under the interface folder. The original
// draws 01 to 10.
func loadingImagePath(index int) string {
	return uiTexBasePath + fmt.Sprintf("loading%02d.jpg", index)
}

// loadingImage returns the loading picture with the given 1-based index,
// loading it the first time it is asked for, or nil when the archive does
// not have it. A missing file is reported once, with its path: a loading
// screen that silently fell back would be the black login screen all over
// again.
//
// The picture is a photo-like JPEG stretched over the window, so it gets
// linear filtering rather than the texture cache's nearest, which is right
// for pixel art and wrong for this.
func (b *UI2DBackend) loadingImage(index int) *TextureInfo {
	if index <= 0 || b.assetLoader == nil {
		return nil
	}
	if tex, ok := b.loadingTex[index]; ok {
		return tex
	}
	if b.loadingTex == nil {
		b.loadingTex = make(map[int]*TextureInfo)
	}

	path := loadingImagePath(index)
	tex, err := b.decodeLinear(path)
	if err != nil {
		logger.Warn("loading screen picture unavailable, using the title backdrop",
			zap.String("path", path), zap.Error(err))
	}
	b.loadingTex[index] = tex // nil is remembered too, so the warning is one line
	return tex
}

// decodeLinear loads an image from the archives as a linearly filtered
// texture, without the magenta key the interface art needs.
func (b *UI2DBackend) decodeLinear(path string) (*TextureInfo, error) {
	data, err := b.assetLoader(path)
	if err != nil {
		return nil, err
	}
	img, err := formats.DecodeImage(data)
	if err != nil {
		return nil, fmt.Errorf("decoding %s: %w", path, err)
	}
	rgba := texture.ImageToRGBA(img, false)
	bounds := rgba.Bounds()
	return &TextureInfo{
		ID:     b.ctx.Renderer().CreateTexture(bounds.Dx(), bounds.Dy(), rgba.Pix),
		Width:  bounds.Dx(),
		Height: bounds.Dy(),
	}, nil
}

// RenderLoadingUI renders the map loading screen the way the original does.
func (b *UI2DBackend) RenderLoadingUI(state LoadingUIState, width, height float32) {
	r := b.ctx.Renderer()

	// The picture fills the window. Without one, the title backdrop stands
	// in; without that, black — never the previous screen.
	if tex := b.loadingImage(state.ImageIndex); tex != nil {
		r.DrawImage(tex.ID, 0, 0, width, height, ui2d.ColorWhite)
	} else {
		b.loadLoginTextures()
		if b.loginBgTex != nil {
			r.DrawImage(b.loginBgTex.ID, 0, 0, width, height, ui2d.ColorWhite)
		} else {
			r.DrawRect(0, 0, width, height, ui2d.RGB(0, 0, 0))
		}
	}

	pct := state.Progress
	if pct < 0 {
		pct = 0
	}
	if pct > 1 {
		pct = 1
	}

	x := float32(gomath.Floor(float64((width - loadingBarW) / 2)))
	y := float32(gomath.Floor(float64(height * loadingBarY)))

	r.DrawRect(x, y, loadingBarW, loadingBarH, loadingBarBorder)
	r.DrawRect(x+1, y+1, loadingBarW-2, loadingBarH-2, loadingBarTrough)
	if fill := float32(gomath.Floor(float64(pct * (loadingBarW - 4)))); fill > 0 {
		r.DrawRect(x+2, y+2, fill, loadingBarH-4, loadingBarFill)
	}

	label := fmt.Sprintf("%d%%", int(pct*100))
	scale := float32(0.75)
	textW, _ := r.MeasureText(label, scale)
	ascent := r.FontAscent(scale)
	r.DrawText(x+(loadingBarW-textW)/2, y+(loadingBarH-ascent)/2-1, label, scale, loadingBarText)

	// A load that failed says so where the player is looking.
	if state.ErrorMessage != "" {
		msgW, _ := r.MeasureText(state.ErrorMessage, 1)
		r.DrawText((width-msgW)/2, y-28, state.ErrorMessage, 1, loadingError)
	}
}
