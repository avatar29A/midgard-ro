package ui

import (
	"fmt"
	"strings"

	"go.uber.org/zap"

	"github.com/Faultbox/midgard-ro/internal/engine/ui2d"
	"github.com/Faultbox/midgard-ro/internal/logger"
	"github.com/Faultbox/midgard-ro/internal/trace"
)

// The minimap sits in the top-right corner at a fixed size, as the original
// does.
const (
	minimapSize   = 128.0
	minimapMargin = 10.0

	// minimapDotSize is the marker drawn where the player stands.
	minimapDotSize = 4.0
)

// minimapPath is where the pre-rendered map images live.
//
// RO ships one image per map rather than deriving the minimap from terrain at
// runtime, so a map with no image simply has no minimap — which is why this
// warns with the path rather than falling back to something invented.
const minimapPath = skinBasePath + `map\`

// minimapProject maps a cell to a pixel offset inside a square minimap box.
//
// The image covers a **square** of max(width, height) cells, with the shorter
// axis letterboxed inside it — Prontera's 312x392 cells arrive as a 512x512
// image. Ignoring that squashes the map along its short axis and walks the
// player marker away from where they are.
//
// Y is flipped: cell Y counts north, screen Y counts down.
//
// Sized to the minimap box. The full map window (a later step) draws the same
// projection much larger, and takes the size as an argument then — with one
// caller it would only be unused variety.
func minimapProject(cellX, cellY, cellsX, cellsY int) (px, py float32) {
	const box = minimapSize

	if cellsX <= 0 || cellsY <= 0 {
		return 0, 0
	}

	span := cellsX
	if cellsY > span {
		span = cellsY
	}

	scale := box / float32(span)
	offsetX := float32(span-cellsX) / 2 * scale
	offsetY := float32(span-cellsY) / 2 * scale

	px = offsetX + float32(cellX)*scale
	py = box - offsetY - float32(cellY)*scale

	return px, py
}

// minimapImagePath is the archive path for a map's minimap image. The map name
// arrives as `prontera.gat`; the image is named for the map alone.
func minimapImagePath(mapName string) string {
	name := strings.TrimSuffix(strings.TrimSpace(mapName), ".gat")
	if name == "" {
		return ""
	}

	return fmt.Sprintf(`%s%s.bmp`, minimapPath, name)
}

// drawMinimap puts the map image in the top-right corner with the player
// marked on it.
//
// A map with no image draws nothing at all rather than an empty frame: the
// original has no minimap for those either, and a black square reads as a
// broken one.
func (b *UI2DBackend) drawMinimap(state InGameUIState, screenW float32) {
	tex := b.minimapTexture(state.MapName)
	if tex == nil {
		return
	}

	x := screenW - minimapSize - minimapMargin
	y := float32(minimapMargin)

	r := b.ctx.Renderer()
	r.DrawImage(tex.ID, x, y, minimapSize, minimapSize, ui2d.ColorWhite)

	if state.MapCellsX <= 0 || state.MapCellsY <= 0 {
		return
	}

	px, py := minimapProject(state.PlayerTileX, state.PlayerTileY,
		state.MapCellsX, state.MapCellsY)

	// Centered on the cell rather than starting at it, so the marker sits on
	// the player instead of below and to the right of them.
	r.DrawRect(x+px-minimapDotSize/2, y+py-minimapDotSize/2,
		minimapDotSize, minimapDotSize, ui2d.ColorWhite)
}

// minimapTexture loads and caches the image for a map, remembering misses so a
// map without one is not retried every frame.
func (b *UI2DBackend) minimapTexture(mapName string) *TextureInfo {
	path := minimapImagePath(mapName)
	if path == "" {
		return nil
	}

	if b.minimapTried == path {
		return b.minimapTex
	}
	b.minimapTried = path

	tex, err := b.texCache.Load(path)
	if err != nil {
		// Warned, not swallowed: a missing asset that fails quietly is how the
		// login screen once rendered on black.
		logger.Warn("no minimap image for this map",
			zap.String("map", mapName), zap.String("path", path), zap.Error(err))

		b.minimapTex = nil

		return nil
	}

	b.minimapTex = tex
	trace.Emit(trace.HUD, "minimap",
		zap.String("map", mapName),
		zap.String("path", path),
		zap.Int("width", tex.Width), zap.Int("height", tex.Height))

	return tex
}
