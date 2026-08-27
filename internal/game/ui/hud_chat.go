package ui

import (
	"strings"

	"github.com/Faultbox/midgard-ro/internal/engine/ui2d"
	"github.com/Faultbox/midgard-ro/internal/game/states"
	"github.com/Faultbox/midgard-ro/internal/network/packets"
)

// The chat box sits along the bottom left, as the original does.
// The chat window, sized as the original is (roBrowser's ChatBox.css, which
// transcribes it).
const (
	chatWidth   float32 = 595
	chatHeight  float32 = 120
	chatMargin  float32 = 10
	chatPadding float32 = 6

	// chatLineH is the height of one wrapped line.
	chatLineH float32 = 14

	// chatScrollW is the gutter the scrollbar occupies.
	chatScrollW float32 = 14

	// chatTabH is the tab strip above the box; chatTabW one tab.
	chatTabH float32 = 17
	chatTabW float32 = 75

	// chatInputH is the bar along the bottom, which carries dialog_bg.bmp.
	chatInputH float32 = 25

	// chatNameW is the speaker field at the left of that bar.
	chatNameW float32 = 90

	// chatStatusBarH is the strip along the bottom of the screen the chat has
	// to sit above. Kept here rather than shared, so the two do not have to
	// know about each other beyond this number.
	chatStatusBarH float32 = 25
)

// chatInputBG is the bar's background, a 600x24 strip in the archive.
const chatInputBG = basicInterfacePath + "dialog_bg.bmp"

// chatTabs are the tabs across the top. The original lets you rename and add
// them; these two are what it opens with.
var chatTabs = []string{"Public", "Battle"}

// Chat colors. RO tints a line by where it came from, which is how you tell
// your own words from someone else's at a glance.
var (
	chatColorOther     = ui2d.Color{R: 1, G: 1, B: 1, A: 1}
	chatColorSelf      = ui2d.Color{R: 0.6, G: 0.9, B: 1, A: 1}
	chatColorBroadcast = ui2d.Color{R: 1, G: 0.9, B: 0.4, A: 1}
	chatColorSpeaker   = ui2d.Color{R: 0.7, G: 1, B: 0.7, A: 1}
	chatBackground     = ui2d.Color{R: 0, G: 0, B: 0, A: 0.5}
	chatBorder         = ui2d.Color{R: 1, G: 1, B: 1, A: 0.85}
	chatTabActive      = ui2d.Color{R: 0, G: 0, B: 0, A: 0.5}
	chatTabIdle        = ui2d.Color{R: 0, G: 0, B: 0, A: 0.75}
	chatShadow         = ui2d.Color{R: 0, G: 0, B: 0, A: 0.9}
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
	// The whole thing: tabs, then the scrollback, then the input bar.
	top := screenH - chatStatusBarH - chatMargin - chatInputH - chatHeight - chatTabH
	bodyY := top + chatTabH

	b.drawChatTabs(x, top)

	r := b.ctx.Renderer()
	r.DrawRect(x, bodyY, chatWidth, chatHeight, chatBackground)

	// A white edge down each side, as the original has. The top is the tab
	// strip's and the bottom is the input bar's, so neither is drawn here.
	r.DrawRect(x, bodyY, 1, chatHeight, chatBorder)
	r.DrawRect(x+chatWidth-1, bodyY, 1, chatHeight, chatBorder)

	b.drawChatLines(state, x, bodyY)
	b.drawChatInput(state, x, bodyY+chatHeight)
}

// drawChatTabs puts the tab strip above the box, the active one lighter.
func (b *UI2DBackend) drawChatTabs(x, y float32) {
	r := b.ctx.Renderer()

	for i, name := range chatTabs {
		tabX := x + float32(i)*(chatTabW+1)

		bg := chatTabIdle
		if i == b.chatTab {
			bg = chatTabActive
		}
		r.DrawRect(tabX, y, chatTabW, chatTabH, bg)

		// The inactive tabs are underlined, the active one is not — that is
		// what joins it to the box below.
		if i != b.chatTab {
			r.DrawRect(tabX, y+chatTabH-1, chatTabW, 1, chatBorder)
		}

		if b.ctx.InvisibleButtonAt("hud_chat_tab_"+name, tabX, y, chatTabW, chatTabH) {
			b.chatTab = i
		}

		w, h := r.MeasureText(name, 1)
		capX, capY := tabX+(chatTabW-w)/2, y+(chatTabH-h)/2
		r.DrawText(capX+1, capY+1, name, 1, chatShadow)
		r.DrawText(capX, capY, name, 1, ui2d.ColorTextOnDark)
	}

	// The strip continues to the window's edge so the box has a lid.
	tabsEnd := x + float32(len(chatTabs))*(chatTabW+1)
	r.DrawRect(tabsEnd, y+chatTabH-1, x+chatWidth-tabsEnd, 1, chatBorder)
}

// drawChatLines draws the scrollback, newest at the bottom.
func (b *UI2DBackend) drawChatLines(state InGameUIState, x, y float32) {
	r := b.ctx.Renderer()

	textW := chatWidth - 2*chatPadding - chatScrollW
	wrapped := b.wrapChat(state.ChatLines, textW)

	usableH := chatHeight - 2*chatPadding
	visible := int(usableH / chatLineH)
	if visible < 1 {
		visible = 1
	}

	maxOffset := len(wrapped) - visible
	if maxOffset < 0 {
		maxOffset = 0
	}

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
			b.chatPinned = newOffset >= maxOffset
			offset = newOffset
		}
	}

	for i := 0; i < visible && offset+i < len(wrapped); i++ {
		lineY := y + chatPadding + float32(i)*chatLineH
		runX := x + chatPadding

		for _, run := range wrapped[offset+i] {
			// Shadowed, as the original is. The panel is half transparent and
			// Prontera's pavement is bright; without this the text competes
			// with whatever is behind it and loses.
			r.DrawText(runX+1, lineY+1, run.Text, 1.0, chatShadow)
			r.DrawText(runX, lineY, run.Text, 1.0, run.Color)

			w, _ := r.MeasureText(run.Text, 1.0)
			runX += w
		}
	}
}

// drawChatInput puts the bar along the bottom: who is speaking, then what they
// are about to say.
//
// Enter sends and clears. An empty line is not sent — the server would refuse
// it, and the encoder declines to build one.
func (b *UI2DBackend) drawChatInput(state InGameUIState, x, y float32) {
	r := b.ctx.Renderer()

	if tex, err := b.texCache.Load(chatInputBG); err == nil {
		r.DrawImage(tex.ID, x, y, chatWidth, chatInputH, ui2d.ColorWhite)
	} else {
		r.DrawRect(x, y, chatWidth, chatInputH, chatBackground)
	}

	// The speaker's own name, which the server requires on every line, shown
	// so it is clear what is being prefixed rather than hidden in the packet.
	name := state.PlayerName
	if name == "" {
		name = "..."
	}
	r.DrawText(x+8, y+6, name, 1, ui2d.ColorTextOnDark)

	msgX := x + chatNameW
	msgW := chatWidth - chatNameW - 8

	value, _, submitted := b.ctx.TextInputBareAt("hud_chat_input",
		msgX, y+3, msgW, chatInputH-6, 1, b.chatInput)
	b.chatInput = value

	if submitted {
		b.chatPending = strings.TrimSpace(b.chatInput)
		b.chatInput = ""
		// Back to following the newest line: you just added one.
		b.chatPinned = true
	}
}

// TakeChatMessage returns a line the player has entered, and clears it.
//
// The backend collects it and the game layer sends it: the UI has no client
// to send with, and threading one in would put the network inside the widget.
func (b *UI2DBackend) TakeChatMessage() string {
	msg := b.chatPending
	b.chatPending = ""

	return msg
}
