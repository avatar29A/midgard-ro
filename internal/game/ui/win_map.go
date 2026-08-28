package ui

import (
	"github.com/Faultbox/midgard-ro/internal/engine/ui2d"
	"github.com/Faultbox/midgard-ro/internal/network/packets"
)

// The Map window the Map button opens: the same image the minimap draws, at
// a size worth reading, with the player marked on it.
//
// It shares the minimap's loader rather than keeping its own. There is one
// image per map and it is already cached; a second path to it would be a
// second thing to get wrong when a map has no image.
const (
	mapWindowID = "hud_win_map"

	mapWinW float32 = 320
	mapWinH float32 = 340

	mapWinPad float32 = 8

	// mapWinMark is the dot marking the player, and mapWinMarkMin keeps it
	// visible on a map scaled well down.
	mapWinMark float32 = 5

	mapWinTextScale float32 = 0.75
)

// drawMapWindow draws the Map window when its button has opened it.
func (b *UI2DBackend) drawMapWindow(state InGameUIState, screenW, screenH float32) {
	if !b.IsWindowOpen(WindowMap) {
		return
	}

	openX := (screenW - mapWinW) / 2
	openY := (screenH - mapWinH) / 2

	// The body is the map image, so the frame must not paint one over it.
	opts := ui2d.DefaultWindowOptions()
	opts.BitmapBody = true

	// The map's name without its extension: "prontera", not "prontera.gat".
	title := hudWindowTitles[WindowMap]
	if state.MapName != "" {
		title = packets.MapBaseName(state.MapName)
	}

	if !b.ctx.BeginWindowEx(mapWindowID, openX, openY, mapWinW, mapWinH, title, opts) {
		if b.ctx.WindowClosed(mapWindowID) {
			b.ToggleWindow(WindowMap)
		}

		return
	}

	x, y := openX, openY
	if rect, ok := b.ctx.WindowRect(mapWindowID); ok {
		x, y = rect.X, rect.Y
	}

	b.ctx.CaptureMouse(ui2d.Rect{X: x, Y: y, W: mapWinW, H: mapWinH})

	body := ui2d.Rect{
		X: x + mapWinPad,
		Y: y + ui2d.FrameTitleH + mapWinPad,
		W: mapWinW - 2*mapWinPad,
		H: mapWinH - ui2d.FrameTitleH - 2*mapWinPad,
	}

	r := b.ctx.Renderer()
	r.FillImageLayer(x, y+ui2d.FrameTitleH, mapWinW, mapWinH-ui2d.FrameTitleH, ui2d.ColorWindowBody)

	b.drawMapImage(state, body)
	b.ctx.EndWindow()
}

// drawMapImage draws the map itself, fitted to the window, with the player on
// it. A map with no image says so rather than showing an empty frame.
func (b *UI2DBackend) drawMapImage(state InGameUIState, body ui2d.Rect) {
	r := b.ctx.Renderer()

	tex := b.minimapTexture(state.MapName)
	if tex == nil || state.MapCellsX <= 0 || state.MapCellsY <= 0 {
		r.DrawText(body.X, body.Y, "No map image for this area.", mapWinTextScale, skillsEmptyText)

		return
	}

	// Fitted rather than stretched: the images are square but the window need
	// not be, and a stretched map misplaces every mark on it.
	scale := min(body.W/float32(tex.Width), body.H/float32(tex.Height))
	drawW := float32(tex.Width) * scale
	drawH := float32(tex.Height) * scale

	drawX := body.X + (body.W-drawW)/2
	drawY := body.Y + (body.H-drawH)/2

	r.DrawImage(tex.ID, drawX, drawY, drawW, drawH, ui2d.ColorWhite)

	// The player, placed by the same fraction of the map its cell is at. The
	// image covers the whole map, so a cell maps onto it directly.
	fx := float32(state.PlayerTileX) / float32(state.MapCellsX)
	fy := float32(state.PlayerTileY) / float32(state.MapCellsY)

	// Cell Y counts from the bottom of the map and the image from its top.
	markX := drawX + fx*drawW - mapWinMark/2
	markY := drawY + (1-fy)*drawH - mapWinMark/2

	r.DrawRect(markX, markY, mapWinMark, mapWinMark, minimapDot)
}
