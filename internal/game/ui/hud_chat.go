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

	// chatNameW is the whisper-target field at the left of that bar. Empty
	// means the line goes to public chat; a name makes it private.
	chatNameW float32 = 90

	// chatCtrlBtn is one control-panel button in the tab strip, and
	// chatCtrlGap the space between them.
	chatCtrlBtn float32 = 15
	chatCtrlGap float32 = 2

	// chatStepLines is how much one +/- press changes the box, in lines. The
	// original steps a ladder of 3-line stops rather than a free height.
	chatStepLines = 3

	// chatStatusBarH is the strip along the bottom of the screen the chat has
	// to sit above. Kept here rather than shared, so the two do not have to
	// know about each other beyond this number.
	chatStatusBarH float32 = 25

	// chatGrip is the corner you drag to resize, and how big that corner is.
	chatGrip float32 = 14

	// The window will not be dragged or resized past these.
	chatMinW float32 = 260
	chatMinH float32 = 60
	chatMaxW float32 = 1200
	chatMaxH float32 = 500
)

// chatInputBG is the bar's background, a 600x24 strip in the archive.
const chatInputBG = basicInterfacePath + "dialog_bg.bmp"

// chatTabs are the tabs across the top. The original lets you rename and add
// them; these two are what it opens with.
var chatTabs = []string{"Public", "Battle"}

// Chat colors. RO tints a line by where it came from, which is how you tell
// your own words from someone else's at a glance.
var (
	// What someone said, whether that is us or anyone else.
	chatColorMessage = ui2d.Color{R: 1, G: 1, B: 1, A: 1}
	// The server talking: welcome lines, announcements, notices.
	chatColorSystem = ui2d.Color{R: 1, G: 0.92, B: 0.25, A: 1}
	// A private message.
	chatColorWhisper = ui2d.Color{R: 0.78, G: 0.55, B: 1, A: 1}
	// Battle lines. Nothing routes here yet — the combat packets are Track F
	// — but the color belongs with the others rather than in that work.
	chatColorDamage  = ui2d.Color{R: 1, G: 0.3, B: 0.3, A: 1}
	chatColorSpeaker = ui2d.Color{R: 0.7, G: 1, B: 0.7, A: 1}
	chatBackground   = ui2d.Color{R: 0, G: 0, B: 0, A: 0.5}
	chatBorder       = ui2d.Color{R: 1, G: 1, B: 1, A: 0.85}

	// chatFocusBorder frames whichever input field has the keyboard.
	chatFocusBorder = ui2d.Color{R: 1, G: 0.85, B: 0.35, A: 0.95}
	chatTabActive   = ui2d.Color{R: 0, G: 0, B: 0, A: 0.5}
	chatTabIdle     = ui2d.Color{R: 0, G: 0, B: 0, A: 0.75}
	chatShadow      = ui2d.Color{R: 0, G: 0, B: 0, A: 0.9}
	chatGripHot     = ui2d.Color{R: 1, G: 0.9, B: 0.4, A: 1}
)

// chatKindColor is the color a line is drawn in.
func chatKindColor(kind packets.ChatKind) ui2d.Color {
	switch kind {
	case packets.ChatSystem, packets.ChatBroadcast:
		return chatColorSystem
	case packets.ChatWhisper:
		return chatColorWhisper
	case packets.ChatDamage:
		return chatColorDamage
	default:
		// Anything a person said, ours or theirs.
		return chatColorMessage
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
	if !b.chatPlaced {
		b.chatX = chatMargin
		b.chatY = screenH - chatStatusBarH - chatMargin - chatInputH - chatHeight - chatTabH
		b.chatW, b.chatH = chatWidth, chatHeight
		b.chatPlaced = true
	}

	x, top := b.chatX, b.chatY
	w, h := b.chatW, b.chatH
	bodyY := top + chatTabH

	// The whole window claims the pointer. Without this a click in the box
	// falls through to the world: the character walks to wherever you clicked
	// and the field you aimed at never takes focus.
	b.ctx.CaptureMouse(ui2d.Rect{X: x, Y: top, W: w, H: chatTabH + h + chatInputH})

	b.drawChatTabs(x, top, w)
	b.drawChatControls(x, top, w)

	r := b.ctx.Renderer()
	r.DrawRect(x, bodyY, w, h, chatBackground)

	// A white edge down each side, as the original has. The top is the tab
	// strip's and the bottom is the input bar's, so neither is drawn here.
	r.DrawRect(x, bodyY, 1, h, chatBorder)
	r.DrawRect(x+w-1, bodyY, 1, h, chatBorder)

	b.drawChatLines(state, x, bodyY, w, h)
	b.drawChatInput(x, bodyY+h, w)

	b.chatDragAndResize(screenH)
}

// chatDragAndResize moves the window by its tab strip and resizes it from the
// top-right corner.
//
// The grip is at the top right rather than the bottom right because the window
// grows upward: the input bar stays where it is and the scrollback gets
// taller, which is what you want when the bar is what you type into.
func (b *UI2DBackend) chatDragAndResize(screenH float32) {
	// Locked means locked in place: no drag handle, no grip, so neither a
	// stray drag on the strip nor one on the corner can shift the box. The
	// clamp below still runs, because +/- resizes a locked box and would
	// otherwise walk its top edge off the screen.
	if b.chatLocked {
		b.clampChatToScreen(screenH)

		return
	}

	// Dragged by the strip above the box, past the tabs, which are buttons.
	tabsEnd := float32(len(chatTabs)) * (chatTabW + 1)
	handle := ui2d.Rect{X: b.chatX + tabsEnd, Y: b.chatY, W: b.chatW - tabsEnd - chatGrip, H: chatTabH}
	b.ctx.DragHandle("hud_chat_drag", handle, &b.chatX, &b.chatY)

	grip := ui2d.Rect{X: b.chatX + b.chatW - chatGrip, Y: b.chatY, W: chatGrip, H: chatGrip}
	b.drawChatGrip(grip)

	// Resizing drags two numbers that are not the window's position, so it
	// cannot hand both to DragHandle. Width follows the pointer; height grows
	// as the top edge is pulled up, which is why the top's movement is added
	// back to the height.
	beforeY := b.chatY
	b.ctx.DragHandle("hud_chat_resize", grip, &b.chatW, &b.chatY)

	if b.chatY != beforeY {
		b.chatH += beforeY - b.chatY
	}

	b.clampChatToScreen(screenH)
}

// clampChatToScreen keeps the box a sane size and on the screen.
func (b *UI2DBackend) clampChatToScreen(screenH float32) {
	b.chatW = clampF(b.chatW, chatMinW, chatMaxW)
	b.chatH = clampF(b.chatH, chatMinH, chatMaxH)

	// Never off the bottom or the sides of the screen.
	maxTop := screenH - chatStatusBarH - chatInputH - b.chatH - chatTabH
	b.chatY = clampF(b.chatY, 0, maxTop)
	b.chatX = clampF(b.chatX, 0, 4000)
}

// drawChatGrip marks the corner that can be pulled, and lights it while the
// pointer is over it so it is clear that it can be.
func (b *UI2DBackend) drawChatGrip(grip ui2d.Rect) {
	color := chatBorder
	if grip.Contains(b.ctx.Input().MouseX, b.ctx.Input().MouseY) {
		color = chatGripHot
	}

	r := b.ctx.Renderer()

	// Stepped lines in the corner, the marking RO puts on a resizable window.
	for i := float32(0); i < 3; i++ {
		offset := i*4 + 2
		r.DrawRect(grip.X+offset, grip.Y+2, chatGrip-offset-2, 1, color)
		r.DrawRect(grip.X+chatGrip-2, grip.Y+2, 1, offset, color)
	}
}

// clampF keeps a value between two bounds.
func clampF(v, lo, hi float32) float32 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}

	return v
}

// drawChatTabs puts the tab strip above the box, the active one lighter.
func (b *UI2DBackend) drawChatTabs(x, y, w float32) {
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

		capW, capH := r.MeasureText(name, 1)
		capX, capY := tabX+(chatTabW-capW)/2, y+(chatTabH-capH)/2
		r.DrawText(capX+1, capY+1, name, 1, chatShadow)
		r.DrawText(capX, capY, name, 1, ui2d.ColorTextOnDark)
	}

	// The strip continues to the window's edge so the box has a lid.
	tabsEnd := x + float32(len(chatTabs))*(chatTabW+1)
	r.DrawRect(tabsEnd, y+chatTabH-1, x+w-tabsEnd, 1, chatBorder)
}

// drawChatControls draws the panel at the right of the tab strip: the two
// height steps, the lock, and the jump to the newest line.
//
// The official client's captions are these four, and the two that are not
// self-evident do what their tooltips describe: the lock pins the box so a
// stray drag cannot move or resize it, and the arrows return to the bottom of
// the scrollback after you have read back through it.
func (b *UI2DBackend) drawChatControls(x, y, w float32) {
	// Right-aligned, and left of the resize grip so the two never overlap.
	right := x + w - chatGrip - chatCtrlGap

	type control struct {
		id      string
		caption string
		on      bool
		click   func()
	}

	controls := []control{
		{"chat_ctrl_last", ">>", false, func() {
			b.chatPinned = true
		}},
		{"chat_ctrl_lock", "L", b.chatLocked, func() {
			b.chatLocked = !b.chatLocked
		}},
		{"chat_ctrl_minus", "-", false, func() {
			b.stepChatHeight(-chatStepLines)
		}},
		{"chat_ctrl_plus", "+", false, func() {
			b.stepChatHeight(chatStepLines)
		}},
	}

	r := b.ctx.Renderer()
	btnY := y + (chatTabH-chatCtrlBtn)/2

	for _, c := range controls {
		// Sized to its caption: ">>" does not fit the square the other three
		// sit in, and centring it in one just spills over the borders.
		capW, capH := r.MeasureText(c.caption, 1)
		btnW := max32(chatCtrlBtn, capW+6)
		btnX := right - btnW
		right -= btnW + chatCtrlGap

		if btnX < x {
			break
		}

		bg := chatTabIdle
		if c.on {
			bg = chatTabActive
		}
		r.DrawRect(btnX, btnY, btnW, chatCtrlBtn, bg)
		r.DrawRect(btnX, btnY, btnW, 1, chatBorder)
		r.DrawRect(btnX, btnY+chatCtrlBtn-1, btnW, 1, chatBorder)
		r.DrawRect(btnX, btnY, 1, chatCtrlBtn, chatBorder)
		r.DrawRect(btnX+btnW-1, btnY, 1, chatCtrlBtn, chatBorder)

		capX, capY := btnX+(btnW-capW)/2, btnY+(chatCtrlBtn-capH)/2
		r.DrawText(capX, capY, c.caption, 1, ui2d.ColorTextOnDark)

		if b.ctx.InvisibleButtonAt("hud_"+c.id, btnX, btnY, btnW, chatCtrlBtn) {
			c.click()
		}
	}
}

// stepChatHeight grows or shrinks the box by whole lines, keeping the input
// bar where it is: the bar is what the eye tracks, and having it walk up the
// screen every time the scrollback grew would be the wrong thing to move.
func (b *UI2DBackend) stepChatHeight(lines float32) {
	want := clampF(b.chatH+lines*chatLineH, chatMinH, chatMaxH)
	b.chatY -= want - b.chatH
	b.chatH = want
	b.chatPinned = true
}

// drawChatLines draws the scrollback, newest at the bottom.
func (b *UI2DBackend) drawChatLines(state InGameUIState, x, y, w, h float32) {
	r := b.ctx.Renderer()

	textW := w - 2*chatPadding - chatScrollW
	wrapped := b.wrapChat(state.ChatLines, textW)

	usableH := h - 2*chatPadding
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
		newOffset := b.scrollbar("hud_chat", x+w-chatScrollW, y,
			h, offset, maxOffset, visible)
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
func (b *UI2DBackend) drawChatInput(x, y, w float32) {
	r := b.ctx.Renderer()

	if tex, err := b.texCache.Load(chatInputBG); err == nil {
		r.DrawImage(tex.ID, x, y, w, chatInputH, ui2d.ColorWhite)
	} else {
		r.DrawRect(x, y, w, chatInputH, chatBackground)
	}

	// Two fields, as the original has: a name and a message. Leaving the name
	// blank talks to everyone; filling it in makes the line a whisper. They
	// take focus independently, by click or by Tab.
	nameW := chatNameW
	msgX := x + nameW + chatCtrlGap
	msgW := w - nameW - chatCtrlGap - chatPadding

	name, _, nameSubmit := b.ctx.TextInputBareAt("hud_chat_name",
		x+chatPadding, y+3, nameW-chatPadding, chatInputH-6, 1, b.chatName)
	b.chatName = name

	msg, _, msgSubmit := b.ctx.TextInputBareAt("hud_chat_input",
		msgX, y+3, msgW, chatInputH-6, 1, b.chatInput)
	b.chatInput = msg

	// A hairline between them, so it reads as two boxes rather than one long
	// one with a gap in the text.
	r.DrawRect(x+nameW, y+4, 1, chatInputH-8, chatBorder)

	// The fields are drawn bare, on a background strip that already looks like
	// a box, so an outline is the only thing that says where the typing goes.
	b.outlineIfFocused("hud_chat_name", x+chatPadding-2, y+2, nameW-chatPadding+2, chatInputH-4)
	b.outlineIfFocused("hud_chat_input", msgX-2, y+2, msgW+4, chatInputH-4)

	// Enter sends from either field: with the caret in the name box there is
	// nothing else Enter could sensibly mean.
	if !nameSubmit && !msgSubmit {
		return
	}

	if text := strings.TrimSpace(b.chatInput); text != "" {
		b.chatPending = text
		b.chatPendingTo = strings.TrimSpace(b.chatName)
		b.chatInput = ""
		// Back to following the newest line: you just added one.
		b.chatPinned = true
	}
}

// TakeChatMessage returns a line the player has entered, and clears it. The
// target is the whisper recipient, empty for public chat.
//
// The interface has no client to send with, so the line is handed back to the
// caller rather than sent from the widget.
func (b *UI2DBackend) TakeChatMessage() (target, message string) {
	target, message = b.chatPendingTo, b.chatPending
	b.chatPendingTo, b.chatPending = "", ""

	return target, message
}

// outlineIfFocused frames a field that has the keyboard, so it is obvious
// which of the two the next keystroke lands in.
func (b *UI2DBackend) outlineIfFocused(id string, x, y, w, h float32) {
	if !b.ctx.Focused(id) {
		return
	}

	r := b.ctx.Renderer()
	r.DrawRect(x, y, w, 1, chatFocusBorder)
	r.DrawRect(x, y+h-1, w, 1, chatFocusBorder)
	r.DrawRect(x, y, 1, h, chatFocusBorder)
	r.DrawRect(x+w-1, y, 1, h, chatFocusBorder)
}

// max32 is the float32 max the standard library only offers for float64.
func max32(a, b float32) float32 {
	if a > b {
		return a
	}

	return b
}
