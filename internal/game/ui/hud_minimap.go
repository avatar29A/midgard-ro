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
	minimapDotSize = 5.0

	// minimapBtn is the size of the zoom buttons under the map.
	minimapBtn = 16.0
)

// minimapZooms are the steps the + and - buttons move through. 1 shows the
// whole map; each step after that shows a smaller part of it, centered on the
// player.
var minimapZooms = []float32{1, 2, 4}

// minimapDot is the color the player is marked in.
//
// White was the obvious choice and the wrong one: Prontera's map is mostly
// pale stone and pale road, and a white dot on it is invisible at four pixels.
// Red reads against every part of it.
var minimapDot = ui2d.Color{R: 1, G: 0.15, B: 0.15, A: 1}

// minimapPath is where the pre-rendered map images live.
//
// RO ships one image per map rather than deriving the minimap from terrain at
// runtime, so a map with no image simply has no minimap — which is why this
// warns with the path rather than falling back to something invented.
const minimapPath = skinBasePath + `map\`

// minimapProject maps a cell to a pixel offset inside a square minimap box, at
// a zoom level and centered on a cell.
//
// At zoom 1 the whole map is shown and the center is ignored. Above that the
// box shows a slice of the map that many times smaller, kept centered on the
// player, so the marker stays put and the map moves under it.
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
func minimapProject(cellX, cellY, cellsX, cellsY int, zoom float32, centerX, centerY int) (px, py float32) {
	const box = minimapSize

	if cellsX <= 0 || cellsY <= 0 {
		return 0, 0
	}
	if zoom < 1 {
		zoom = 1
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

	if zoom == 1 {
		return px, py
	}

	// Everything moves away from the center by the zoom, and the center itself
	// lands in the middle of the box.
	cpx := offsetX + float32(centerX)*scale
	cpy := box - offsetY - float32(centerY)*scale

	return box/2 + (px-cpx)*zoom, box/2 + (py-cpy)*zoom
}

// minimapViewUV is the part of the map image the box shows, as UV coordinates.
//
// At zoom 1 that is all of it. Above, it is a window that many times smaller
// centered on the player, clamped so it never runs off the edge of the image —
// walking into a corner should show the corner, not empty space beside it.
func minimapViewUV(cellX, cellY, cellsX, cellsY int, zoom float32) (u0, v0, u1, v1 float32) {
	if zoom <= 1 || cellsX <= 0 || cellsY <= 0 {
		return 0, 0, 1, 1
	}

	px, py := minimapProject(cellX, cellY, cellsX, cellsY, 1, 0, 0)

	// The player's spot in the image, as a fraction of it.
	cu := px / minimapSize
	cv := py / minimapSize

	half := 0.5 / zoom

	u0, v0 = clampUV(cu-half, half), clampUV(cv-half, half)

	return u0, v0, u0 + 2*half, v0 + 2*half
}

// clampUV keeps a window of the given half-size inside 0..1.
func clampUV(start, half float32) float32 {
	if start < 0 {
		return 0
	}
	if start+2*half > 1 {
		return 1 - 2*half
	}

	return start
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
	zoom := b.minimapZoom()

	u0, v0, u1, v1 := minimapViewUV(state.PlayerTileX, state.PlayerTileY,
		state.MapCellsX, state.MapCellsY, zoom)
	r.DrawImageUV(tex.ID, x, y, minimapSize, minimapSize, u0, v0, u1, v1, ui2d.ColorWhite)

	b.drawMinimapZoomButtons(x, y+minimapSize+2)

	if state.MapCellsX <= 0 || state.MapCellsY <= 0 {
		return
	}

	px, py := minimapProject(state.PlayerTileX, state.PlayerTileY,
		state.MapCellsX, state.MapCellsY, zoom, state.PlayerTileX, state.PlayerTileY)

	// Centered on the cell rather than starting at it, so the marker sits on
	// the player instead of below and to the right of them.
	r.DrawRect(x+px-minimapDotSize/2, y+py-minimapDotSize/2,
		minimapDotSize, minimapDotSize, minimapDot)
}

// minimapZoom is the current zoom factor.
func (b *UI2DBackend) minimapZoom() float32 {
	if b.minimapZoomIdx < 0 || b.minimapZoomIdx >= len(minimapZooms) {
		return 1
	}

	return minimapZooms[b.minimapZoomIdx]
}

// drawMinimapZoomButtons puts + and - under the map.
//
// Laid out right to left from the map's right edge, so they sit under it
// rather than beside it whatever the screen width.
func (b *UI2DBackend) drawMinimapZoomButtons(x, y float32) {
	plusX := x + minimapSize - minimapBtn
	minusX := plusX - minimapBtn - 2

	if b.ctx.ButtonAt("hud_minimap_out", minusX, y, minimapBtn, minimapBtn, "-") {
		b.setMinimapZoom(b.minimapZoomIdx - 1)
	}
	if b.ctx.ButtonAt("hud_minimap_in", plusX, y, minimapBtn, minimapBtn, "+") {
		b.setMinimapZoom(b.minimapZoomIdx + 1)
	}
}

// setMinimapZoom moves to a zoom step, ignoring anything past either end.
func (b *UI2DBackend) setMinimapZoom(idx int) {
	if idx < 0 || idx >= len(minimapZooms) {
		return
	}

	b.minimapZoomIdx = idx
	trace.Emit(trace.HUD, "minimap-zoom", zap.Float32("zoom", minimapZooms[idx]))
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
