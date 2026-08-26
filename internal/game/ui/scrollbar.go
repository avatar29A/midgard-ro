package ui

import (
	"go.uber.org/zap"

	"github.com/Faultbox/midgard-ro/internal/engine/ui2d"
	"github.com/Faultbox/midgard-ro/internal/logger"
)

// The original's vertical scrollbar, from the root of the interface folder.
// `scroll0*` is the vertical set — `scroll1*` is its horizontal twin.
//
// Everything is 13 wide: two 13x13 arrow buttons, a 13x13 track stretched
// between them, and a thumb three-sliced from 13x4 caps and a 13x4 middle.
const (
	scrollW      = float32(13)
	scrollArrowH = float32(13)
	scrollCapH   = float32(4)
)

var scrollPieces = []string{
	"scroll0up", "scroll0down", "scroll0mid",
	"scroll0bar_up", "scroll0bar_mid", "scroll0bar_down",
}

// scrollbarSkin is the six pieces, loaded once.
type scrollbarSkin struct {
	up, down, track        *TextureInfo
	barUp, barMid, barDown *TextureInfo
}

// loadScrollbar loads the scrollbar art. A miss returns nil and the caller
// draws no bar — the wheel still works, so the list is not stranded.
func (b *UI2DBackend) loadScrollbar() *scrollbarSkin {
	if b.scrollSkin != nil || b.scrollTried {
		return b.scrollSkin
	}

	b.scrollTried = true

	loaded := make([]*TextureInfo, 0, len(scrollPieces))

	for _, name := range scrollPieces {
		tex, err := b.texCache.Load(skinBasePath + name + ".bmp")
		if err != nil {
			logger.Warn("scrollbar art unavailable, falling back to the wheel alone",
				zap.String("path", skinBasePath+name+".bmp"), zap.Error(err))

			return nil
		}

		loaded = append(loaded, tex)
	}

	b.scrollSkin = &scrollbarSkin{
		up: loaded[0], down: loaded[1], track: loaded[2],
		barUp: loaded[3], barMid: loaded[4], barDown: loaded[5],
	}

	return b.scrollSkin
}

// scrollbar draws a vertical scrollbar and returns the offset after whatever
// the player did to it: the arrows step by one, the thumb drags, and clicking
// the track above or below the thumb pages.
//
// x, y is its top left; height covers the arrows and the track between them.
func (b *UI2DBackend) scrollbar(id string, x, y, height float32, offset, maxOffset, visible int) int {
	if maxOffset <= 0 {
		return 0
	}

	skin := b.loadScrollbar()
	if skin == nil {
		return clampScroll(offset, maxOffset)
	}

	r := b.ctx.Renderer()

	trackY := y + scrollArrowH
	trackH := height - 2*scrollArrowH

	if trackH < scrollCapH*2 {
		return clampScroll(offset, maxOffset)
	}

	r.DrawImage(skin.track.ID, x, trackY, scrollW, trackH, ui2d.ColorWhite)

	if b.ctx.ImageButtonAt(id+"_up", x, y, scrollW, scrollArrowH,
		skin.up.ID, skin.up.ID, skin.up.ID) {
		offset--
	}

	if b.ctx.ImageButtonAt(id+"_down", x, y+height-scrollArrowH, scrollW, scrollArrowH,
		skin.down.ID, skin.down.ID, skin.down.ID) {
		offset++
	}

	// The thumb's length says how much of the whole is on screen, which is
	// the only cue for how long a conversation is.
	total := visible + maxOffset

	thumbH := trackH * float32(visible) / float32(total)
	if thumbH < 2*scrollCapH+1 {
		thumbH = 2*scrollCapH + 1
	}

	span := trackH - thumbH
	thumbY := trackY + span*float32(clampScroll(offset, maxOffset))/float32(maxOffset)

	// Dragging the thumb: DragHandle moves a position by the mouse delta, so
	// the thumb's own y is what it moves, and the offset is read back off it.
	dragY := thumbY
	dragX := x

	b.ctx.DragHandle(id+"_thumb", ui2d.Rect{X: x, Y: thumbY, W: scrollW, H: thumbH}, &dragX, &dragY)

	if dragY != thumbY && span > 0 {
		offset = int((dragY-trackY)/span*float32(maxOffset) + 0.5)
		thumbY = trackY + span*float32(clampScroll(offset, maxOffset))/float32(maxOffset)
	}

	r.DrawImage(skin.barUp.ID, x, thumbY, scrollW, scrollCapH, ui2d.ColorWhite)
	r.DrawImage(skin.barMid.ID, x, thumbY+scrollCapH, scrollW, thumbH-2*scrollCapH, ui2d.ColorWhite)
	r.DrawImage(skin.barDown.ID, x, thumbY+thumbH-scrollCapH, scrollW, scrollCapH, ui2d.ColorWhite)

	return clampScroll(offset, maxOffset)
}

// clampScroll keeps an offset inside a list. Scroll offsets never start
// anywhere but zero, so there is no low bound to pass.
func clampScroll(v, high int) int {
	if v < 0 {
		return 0
	}
	if v > high {
		return high
	}

	return v
}
