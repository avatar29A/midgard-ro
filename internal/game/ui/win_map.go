package ui

import (
	"github.com/Faultbox/midgard-ro/internal/config"
	"github.com/Faultbox/midgard-ro/internal/engine/ui2d"
	"github.com/Faultbox/midgard-ro/internal/game/entity"
	"github.com/Faultbox/midgard-ro/internal/game/states"
	"github.com/Faultbox/midgard-ro/internal/logger"
	"github.com/Faultbox/midgard-ro/internal/network/packets"

	"go.uber.org/zap"
)

// The Map window the Map button opens: the same image the minimap draws, at
// a size worth reading, with the player marked on it.
//
// It shares the minimap's loader rather than keeping its own. There is one
// image per map and it is already cached; a second path to it would be a
// second thing to get wrong when a map has no image.
const (
	mapWindowID = "hud_win_map"

	mapWinW float32 = 480
	mapWinH float32 = 452

	mapWinPad float32 = 8

	// The strip along the bottom carrying the view switch.
	mapWinFooterH float32 = 26
	mapWinBtnW    float32 = 86
	mapWinBtnH    float32 = 18

	// mapWinMark is the dot marking the player, mapWinOther the smaller one
	// marking everyone else.
	mapWinMark  float32 = 5
	mapWinOther float32 = 3

	// mapWinHoverR is how near the pointer has to be to name a marker, as a
	// squared distance in pixels.
	mapWinHoverR float32 = 12 * 12

	// mapWorldTexture is the archive's own painted world map, "Orbis of
	// Midgard" — 1280x1024, towns already marked on it.
	mapWorldTexture = skinBasePath + "worldmap.bmp"

	mapWinTextScale float32 = 0.75
)

// drawMapWindow draws the Map window when its button has opened it.
func (b *UI2DBackend) drawMapWindow(state InGameUIState, screenW, screenH float32) {
	if !b.IsWindowOpen(WindowMap) {
		return
	}

	// Where it was left, or centered at its default size the first time.
	openX := (screenW - mapWinW) / 2
	openY := (screenH - mapWinH) / 2
	openW, openH := mapWinW, mapWinH

	if saved := b.mapPlacement(); saved.W > 0 {
		openX, openY, openW, openH = saved.X, saved.Y, saved.W, saved.H
	}

	// The body is the map image, so the frame must not paint one over it.
	// Resizable because a map is worth making bigger, which is the one window
	// here where that is true.
	opts := ui2d.DefaultWindowOptions()
	opts.Resizable = true

	// The map's name without its extension: "prontera", not "prontera.gat".
	title := hudWindowTitles[WindowMap]
	if state.MapName != "" {
		title = packets.MapBaseName(state.MapName)
	}

	if b.mapWorldView {
		title = "World Map"
	}

	if !b.ctx.BeginWindowEx(mapWindowID, openX, openY, openW, openH, title, opts) {
		if b.ctx.WindowClosed(mapWindowID) {
			b.ToggleWindow(WindowMap)
		}

		return
	}

	// The window's own size, not the caller's: once resized it keeps its own,
	// and laying the map out to the opening size would leave it in a corner
	// of a window the user has stretched.
	win := ui2d.Rect{X: openX, Y: openY, W: openW, H: openH}
	if rect, ok := b.ctx.WindowRect(mapWindowID); ok {
		win = rect
	}

	b.ctx.CaptureMouse(win)
	b.rememberMapPlacement(win)

	x, y := win.X, win.Y

	body := ui2d.Rect{
		X: x + mapWinPad,
		Y: y + ui2d.FrameTitleH + mapWinPad,
		W: win.W - 2*mapWinPad,
		H: win.H - ui2d.FrameTitleH - mapWinFooterH - 2*mapWinPad,
	}

	r := b.ctx.Renderer()
	r.DrawRect(x, y+ui2d.FrameTitleH, win.W, win.H-ui2d.FrameTitleH, ui2d.ColorWindowBody)

	if b.mapWorldView {
		b.drawWorldMap(body)
	} else {
		b.drawMapImage(state, body)
	}

	b.drawMapFooter(win)
	b.ctx.EndWindow()
}

// drawMapImage draws the map itself, at whatever zoom is set, with the units
// on it. A map with no image says so rather than showing an empty frame.
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

	// Where the player is, as a fraction of the image. Cell Y counts from the
	// map's bottom and the image from its top.
	fx := float32(state.PlayerTileX) / float32(state.MapCellsX)
	fy := 1 - float32(state.PlayerTileY)/float32(state.MapCellsY)

	// Drawn whole. There was a zoom here and it made the map worse: these
	// images are 512x512 for a map three hundred cells across, so magnifying
	// a quarter of one into the window blew 128 pixels up to four hundred and
	// showed less than before. Resizing the window is how to see more, and it
	// keeps every pixel the archive has.
	r.DrawImage(tex.ID, drawX, drawY, drawW, drawH, ui2d.ColorWhite)

	place := func(imgX, imgY float32) (float32, float32, bool) {
		return drawX + imgX*drawW, drawY + imgY*drawH, true
	}

	b.drawMapMarkers(state, place)

	if px, py, on := place(fx, fy); on {
		r.DrawRect(px-mapWinMark/2, py-mapWinMark/2, mapWinMark, mapWinMark, minimapDot)
	}
}

// drawMapMarkers draws everyone else, and names whichever one is under the
// pointer.
//
// Hover rather than click: a dot this size is hard enough to hit once, and
// asking for a click before saying what something is means clicking every dot
// to find the one you wanted.
func (b *UI2DBackend) drawMapMarkers(
	state InGameUIState, place func(float32, float32) (float32, float32, bool),
) {
	r := b.ctx.Renderer()

	mouseX, mouseY := b.ctx.Input().MouseX, b.ctx.Input().MouseY

	var (
		hovered   *states.MapMarker
		hoverX    float32
		hoverY    float32
		hoverBest = mapWinHoverR
	)

	for i := range state.MapMarkers {
		marker := &state.MapMarkers[i]

		// Off the map means the unit has no position yet — the server sends
		// some units before it says where they are, and those arrive at cell
		// zero, which is the map's corner and not where they are standing.
		if marker.CellX <= 0 || marker.CellY <= 0 ||
			marker.CellX >= state.MapCellsX || marker.CellY >= state.MapCellsY {
			continue
		}

		imgX := float32(marker.CellX) / float32(state.MapCellsX)
		imgY := 1 - float32(marker.CellY)/float32(state.MapCellsY)

		px, py, on := place(imgX, imgY)
		if !on {
			continue
		}

		r.DrawRect(px-mapWinOther/2, py-mapWinOther/2, mapWinOther, mapWinOther,
			mapMarkerColor(marker.Type))

		// The nearest within reach wins, so a crowd does not name whichever
		// happened to be drawn last.
		if d := dist2(px, py, mouseX, mouseY); d < hoverBest {
			hoverBest = d
			hovered, hoverX, hoverY = marker, px, py
		}
	}

	if hovered != nil {
		b.drawMarkerLabel(*hovered, hoverX, hoverY)
	}
}

// drawMarkerLabel names the unit under the pointer.
func (b *UI2DBackend) drawMarkerLabel(marker states.MapMarker, x, y float32) {
	r := b.ctx.Renderer()

	name := marker.Name
	if name == "" {
		// Some units arrive without one. Their kind is still worth saying.
		name = mapMarkerKind(marker.Type)
	} else {
		name += " (" + mapMarkerKind(marker.Type) + ")"
	}

	capW, capH := r.MeasureText(name, mapWinTextScale)

	box := ui2d.Rect{X: x + 6, Y: y - capH/2 - 2, W: capW + 8, H: capH + 4}

	r.DrawRect(box.X, box.Y, box.W, box.H, mapLabelBg)
	r.DrawText(box.X+4, box.Y+2, name, mapWinTextScale, ui2d.ColorTextOnDark)
}

// mapMarkerKind is what to call a unit when naming it.
func mapMarkerKind(kind entity.Type) string {
	switch kind {
	case entity.TypeNPC:
		return "NPC"
	case entity.TypeMonster:
		return "Monster"
	case entity.TypeWarp:
		return "Portal"
	default:
		return "Player"
	}
}

// dist2 is the squared distance, which is enough to compare two of them and
// avoids a square root per marker per frame.
func dist2(ax, ay, bx, by float32) float32 {
	dx, dy := ax-bx, ay-by

	return dx*dx + dy*dy
}

// mapMarkerColor is how each kind of unit is marked, so the map can be read
// at a glance rather than by hovering everything on it.
func mapMarkerColor(kind entity.Type) ui2d.Color {
	switch kind {
	case entity.TypeNPC:
		return mapMarkerNPC
	case entity.TypeMonster:
		return mapMarkerMob
	case entity.TypeWarp:
		return mapMarkerWarp
	default:
		return mapMarkerPlayer
	}
}

// drawWorldMap draws the archive's painted world map.
//
// No "you are here" on it. Which map sits where on this picture is in the
// client's worldviewdata table, which is Lua bytecode this does not read yet,
// and a marker placed by guesswork would be worse than none.
func (b *UI2DBackend) drawWorldMap(body ui2d.Rect) {
	r := b.ctx.Renderer()

	tex, err := b.texCache.Load(mapWorldTexture)
	if err != nil {
		r.DrawText(body.X, body.Y, "No world map in this archive.", mapWinTextScale, skillsEmptyText)

		return
	}

	scale := min(body.W/float32(tex.Width), body.H/float32(tex.Height))
	drawW := float32(tex.Width) * scale
	drawH := float32(tex.Height) * scale

	r.DrawImage(tex.ID,
		body.X+(body.W-drawW)/2, body.Y+(body.H-drawH)/2,
		drawW, drawH, ui2d.ColorWhite)
}

// drawMapFooter draws the strip with the switch between the two views.
//
// They answer different questions — where am I on this map, and where is this
// map in the world — so the window carries both rather than one replacing the
// other, which is what the original does too.
func (b *UI2DBackend) drawMapFooter(win ui2d.Rect) {
	r := b.ctx.Renderer()

	footerY := win.Y + win.H - mapWinFooterH
	r.DrawRect(win.X+1, footerY, win.W-2, 1, ui2d.ColorPanelBorder)

	label := "World Map"
	if b.mapWorldView {
		label = "This Map"
	}

	box := ui2d.Rect{
		X: win.X + win.W - mapWinPad - mapWinBtnW,
		Y: footerY + (mapWinFooterH-mapWinBtnH)/2,
		W: mapWinBtnW,
		H: mapWinBtnH,
	}

	b.drawFlatButton(box, label, false)

	if b.ctx.InvisibleButtonAt("hud_map_view", box.X, box.Y, box.W, box.H) {
		b.mapWorldView = !b.mapWorldView
	}
}

// mapPlacement is where the Map window was last left, read once.
func (b *UI2DBackend) mapPlacement() ui2d.Rect {
	if !b.mapPlaced {
		saved := config.LoadUIState()
		b.mapSaved = ui2d.Rect{X: saved.MapX, Y: saved.MapY, W: saved.MapW, H: saved.MapH}
		b.mapPlaced = true
	}

	return b.mapSaved
}

// rememberMapPlacement records where the window is once it has settled.
//
// Written on release rather than every frame it moves: one file per gesture
// instead of one per frame, the same way the chat's placement is kept.
func (b *UI2DBackend) rememberMapPlacement(win ui2d.Rect) {
	if win == b.mapSaved {
		return
	}

	b.mapSaved = win

	if !b.ctx.Input().MouseLeftReleased {
		return
	}

	err := config.UpdateUIState(func(state *config.UIState) {
		state.MapX, state.MapY = win.X, win.Y
		state.MapW, state.MapH = win.W, win.H
	})
	if err != nil {
		logger.Warn("could not save map window placement", zap.Error(err))
	}
}

var (
	// Who is who on the map. Only units the server has told us about appear —
	// those near enough to be in view — so this is who is around rather than
	// everyone on the map.
	mapMarkerNPC    = ui2d.Color{R: 1, G: 0.85, B: 0.2, A: 1}
	mapMarkerMob    = ui2d.Color{R: 0.9, G: 0.35, B: 0.2, A: 1}
	mapMarkerPlayer = ui2d.Color{R: 1, G: 1, B: 1, A: 1}
	mapMarkerWarp   = ui2d.Color{R: 0.45, G: 0.8, B: 1, A: 1}

	// mapLabelBg is the plate a hovered unit's name sits on.
	mapLabelBg = ui2d.Color{R: 0, G: 0, B: 0, A: 0.75}
)
