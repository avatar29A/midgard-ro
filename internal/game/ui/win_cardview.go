package ui

import (
	"github.com/Faultbox/midgard-ro/internal/engine/ui2d"
	"github.com/Faultbox/midgard-ro/internal/game/items"
)

// The window that shows a card's drawing.
//
// A card's icon says nothing about it — every card in the game shares one, a
// blank card back. The drawing is the card, and the original gives it a window
// of its own rather than shrinking it into the information panel: it is a
// portrait, and it is drawn to be looked at.
//
// Opened from the View button on a card's own information window, or by
// clicking a card sitting in another item's slot.

// cardViewWindowID is the frame's id, needed to read its position back and to
// tell a close from a minimize.
const cardViewWindowID = "hud_card_view"

const (
	// The drawing is 220 by 288 in the archive, and is drawn at its own size:
	// scaled up it is a blur and scaled down it loses the lettering along the
	// top, which is part of the picture rather than a caption over it.
	cardViewArtW float32 = 220
	cardViewArtH float32 = 288

	cardViewPad float32 = 6

	cardViewW = cardViewArtW + 2*cardViewPad
	cardViewH = cardViewArtH + 2*cardViewPad + ui2d.FrameTitleH
)

// cardArtPath is where the archive keeps the drawings, named in the same
// Korean everything else in the interface folder is.
const cardArtPath = skinBasePath + `cardbmp\`

// CardArtOf is the drawing a card is filed under, and whether it has one.
//
// Worth asking before offering to show it: a third of the cards in the
// database have no drawing in this archive, and a button that opens an empty
// window is worse than no button.
func CardArtOf(id uint32) (string, bool) {
	info, known := items.Lookup(id)
	if !known || info.CardArt == "" {
		return "", false
	}

	return info.CardArt, true
}

// ShowCardView opens the window on a card.
func (b *UI2DBackend) ShowCardView(id uint32) {
	if _, has := CardArtOf(id); !has {
		return
	}

	b.cardViewID = id

	// Clearing the closed flag its own X set, or it opens once and never
	// again. The nil check is for a backend built without a context, which
	// this file's own tests do.
	if b.ctx != nil {
		b.ctx.OpenWindow(cardViewWindowID)
	}
}

// drawCardView draws it, if a card is being looked at.
func (b *UI2DBackend) drawCardView(screenW, screenH float32) {
	if b.cardViewID == 0 {
		return
	}

	art, has := CardArtOf(b.cardViewID)
	if !has {
		b.cardViewID = 0

		return
	}

	// Below the information window it was opened from, as the original puts
	// it, so both can be read at once.
	infoX := (screenW - itemInfoW) / 2
	infoY := (screenH - itemInfoH) / 2

	openX := (screenW - cardViewW) / 2
	openY := infoY + itemInfoH + 4

	// Beside it instead when there is no room below. Overlapping the very
	// window it was opened from hides the half of it that says what the card
	// does, which is the reason for opening both.
	if openY+cardViewH > screenH {
		openX = infoX + itemInfoW + 4
		openY = min(infoY, max(screenH-cardViewH, 0))

		if openX+cardViewW > screenW {
			openX = max(infoX-cardViewW-4, 0)
		}
	}

	if !b.ctx.BeginWindowEx(cardViewWindowID, openX, openY, cardViewW, cardViewH,
		itemDisplayName(b.cardViewID), ui2d.WindowOptions{Closable: true}) {
		// Minimized is not closed: the title bar is still drawn and the
		// window is still open, so only a real close puts it away.
		if b.ctx.WindowClosed(cardViewWindowID) {
			b.cardViewID = 0
		}

		return
	}

	x, y := openX, openY
	if rect, ok := b.ctx.WindowRect(cardViewWindowID); ok {
		x, y = rect.X, rect.Y
	}

	r := b.ctx.Renderer()

	artX := x + cardViewPad
	artY := y + ui2d.FrameTitleH + cardViewPad

	if tex, err := b.texCache.Load(cardArtPath + art + ".bmp"); err == nil {
		r.DrawImage(tex.ID, artX, artY, cardViewArtW, cardViewArtH, ui2d.ColorWhite)
	} else {
		// Named in the table and missing from the archive, which happens.
		// Said rather than left blank: an empty window looks like one that
		// failed to open.
		r.DrawRect(artX, artY, cardViewArtW, cardViewArtH, itemsCellBg)
		r.DrawText(artX+cardViewPad, artY+cardViewPad,
			"No drawing for this card.", itemInfoTextScale, itemInfoLabel)
	}

	b.ctx.EndWindow()
}
