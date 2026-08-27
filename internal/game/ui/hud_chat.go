package ui

import (
	"github.com/Faultbox/midgard-ro/internal/engine/ui2d"
	"github.com/Faultbox/midgard-ro/internal/game/states"
	"github.com/Faultbox/midgard-ro/internal/network/packets"
)

// The chat box sits along the bottom left, as the original does.
const (
	chatWidth   float32 = 400
	chatHeight  float32 = 120
	chatMargin  float32 = 10
	chatPadding float32 = 6

	// chatLineH is the height of one wrapped line.
	chatLineH float32 = 14

	// chatScrollW is the gutter the scrollbar occupies.
	chatScrollW float32 = 14
)

// Chat colors. RO tints a line by where it came from, which is how you tell
// your own words from someone else's at a glance.
var (
	chatColorOther     = ui2d.Color{R: 1, G: 1, B: 1, A: 1}
	chatColorSelf      = ui2d.Color{R: 0.6, G: 0.9, B: 1, A: 1}
	chatColorBroadcast = ui2d.Color{R: 1, G: 0.9, B: 0.4, A: 1}
	chatColorSpeaker   = ui2d.Color{R: 0.7, G: 1, B: 0.7, A: 1}
	chatBackground     = ui2d.Color{R: 0, G: 0, B: 0, A: 0.45}
)

// chatKindColor is the color a line is drawn in.
func chatKindColor(kind packets.ChatKind) ui2d.Color {
	switch kind {
	case packets.ChatSelf:
		return chatColorSelf
	case packets.ChatBroadcast:
		return chatColorBroadcast
	default:
		return chatColorOther
	}
}

// chatRuns turns one line into the colored runs that draw it.
//
// The speaker is drawn in its own color, which is the reason the decoder
// splits it out of the message rather than leaving the line whole. A line with
// no speaker — a server message — is one run.
func chatRuns(line states.ChatLine) []TextRun {
	body := TextRun{Text: line.Text, Color: chatKindColor(line.Kind)}

	if line.Speaker == "" {
		return []TextRun{body}
	}

	return []TextRun{
		{Text: line.Speaker + " :", Color: chatColorSpeaker},
		body,
	}
}

// wrapChat lays the backlog out into drawable lines, oldest first.
//
// Shares the NPC dialog's wrapper: chat carries the same `^RRGGBB` color
// codes, and a second implementation would drift from it.
func (b *UI2DBackend) wrapChat(lines []states.ChatLine, maxWidth float32) [][]TextRun {
	measure := func(s string) float32 {
		w, _ := b.ctx.Renderer().MeasureText(s, 1.0)

		return w
	}

	var wrapped [][]TextRun
	for _, line := range lines {
		runs := chatRuns(line)

		// Color codes inside the text itself win over the line's own color,
		// the same way they do in NPC dialog.
		var expanded []TextRun
		for _, run := range runs {
			parsed := ParseNPCText(run.Text)
			if len(parsed) == 1 && parsed[0].Color == (ui2d.Color{}) {
				expanded = append(expanded, run)

				continue
			}
			for _, p := range parsed {
				if p.Color == (ui2d.Color{}) {
					p.Color = run.Color
				}
				expanded = append(expanded, p)
			}
		}

		wrapped = append(wrapped, WrapNPCText(expanded, maxWidth, measure)...)
	}

	return wrapped
}

// drawChat puts the scrollback in the bottom-left corner.
//
// Drawn even when empty, unlike the minimap: the box is part of the interface
// rather than something the map supplies, and an empty one still says where
// chat will appear.
func (b *UI2DBackend) drawChat(state InGameUIState, screenH float32) {
	x := chatMargin
	y := screenH - chatHeight - chatMargin

	r := b.ctx.Renderer()
	r.DrawRect(x, y, chatWidth, chatHeight, chatBackground)

	textW := chatWidth - 2*chatPadding - chatScrollW
	wrapped := b.wrapChat(state.ChatLines, textW)

	// Through a variable: dividing two constants gives a constant, and Go will
	// not truncate one to int implicitly.
	usableH := chatHeight - 2*chatPadding
	visible := int(usableH / chatLineH)
	if visible < 1 {
		visible = 1
	}

	maxOffset := len(wrapped) - visible
	if maxOffset < 0 {
		maxOffset = 0
	}

	// Pinned to the bottom: chat follows what was just said, and the offset is
	// only consulted once there is more than fits.
	offset := b.chatScroll
	if offset > maxOffset {
		offset = maxOffset
	}
	if b.chatPinned {
		offset = maxOffset
	}

	if maxOffset > 0 {
		newOffset := b.scrollbar("hud_chat", x+chatWidth-chatScrollW, y,
			chatHeight, offset, maxOffset, visible)
		if newOffset != offset {
			b.chatScroll = newOffset
			// Following again only once the reader returns to the bottom.
			b.chatPinned = newOffset >= maxOffset
			offset = newOffset
		}
	}

	for i := 0; i < visible && offset+i < len(wrapped); i++ {
		lineY := y + chatPadding + float32(i)*chatLineH
		runX := x + chatPadding

		for _, run := range wrapped[offset+i] {
			r.DrawText(runX, lineY, run.Text, 1.0, run.Color)
			w, _ := r.MeasureText(run.Text, 1.0)
			runX += w
		}
	}
}
