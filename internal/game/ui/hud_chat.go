package ui

import (
	"strings"

	"github.com/Faultbox/midgard-ro/internal/engine/cursor"
	"github.com/Faultbox/midgard-ro/internal/engine/ui2d"
	"github.com/Faultbox/midgard-ro/internal/game/states"
	"github.com/Faultbox/midgard-ro/internal/network/packets"
	"github.com/Faultbox/midgard-ro/internal/trace"

	"go.uber.org/zap"
)

// The chat box sits along the bottom left, as the original does.
// The chat window, sized as the original is (roBrowser's ChatBox.css, which
// transcribes it).
const (
	chatWidth   float32 = 595
	chatHeight  float32 = 120
	chatPadding float32 = 6

	// chatInputScale shrinks the text in the input bar. Its boxes are 16px
	// tall in an image we do not control, and the body's size overflows them.
	chatInputScale float32 = 0.8

	// chatLineGap is the air between one message and the next. The line's own
	// height comes from the font, so this is only the breathing room.
	chatLineGap float32 = 4

	// chatScrollW is the gutter the scrollbar occupies.
	chatScrollW float32 = 14

	// chatTabH is the tab strip above the box; chatTabW one tab.
	chatTabH float32 = 17
	chatTabW float32 = 75

	// chatInputH is the bar along the bottom, which carries dialog_bg.bmp.
	chatInputH float32 = 25

	// The two boxes are painted into dialog_bg.bmp rather than drawn by us,
	// and these are where they sit in it: a 600x24 image whose name box spans
	// x 6-93 and whose message box spans x 110-568, both from y 5 to y 21.
	//
	// They are fractions because the bar is one stretched image. At fixed
	// offsets the fields drift out of their boxes as soon as the chat is
	// resized, and even at the default width the message text started 14px
	// left of its box, sitting on the divider between the two.
	chatBGW = 600
	chatBGH = 24

	chatNameBoxL = 6.0 / chatBGW
	chatNameBoxR = 93.0 / chatBGW
	chatMsgBoxL  = 110.0 / chatBGW
	chatMsgBoxR  = 568.0 / chatBGW
	chatBoxT     = 5.0 / chatBGH
	chatBoxB     = 21.0 / chatBGH

	// chatCtrlBtn is one control-panel button in the tab strip, and
	// chatCtrlGap the space between them.
	chatCtrlBtn float32 = 15
	chatCtrlGap float32 = 2

	// chatStepLines is how much one +/- press changes the box, in lines. The
	// original steps a ladder of 3-line stops rather than a free height.
	chatStepLines = 3

	// chatGrip is the corner you drag to resize, and how big that corner is.
	// Bigger than it looks: a 14px target on a HiDPI screen is a 7px one to
	// the hand, and the corner was being missed.
	chatGrip    float32 = 14
	chatGripHit float32 = 20

	// The window will not be dragged or resized past these.
	chatMinW float32 = 260
	chatMinH float32 = 60
	chatMaxW float32 = 1200
	chatMaxH float32 = 500
)

// chatInputBG is the bar's background, a 600x24 strip in the archive.
const chatInputBG = basicInterfacePath + "dialog_bg.bmp"

// chatResizeTexture is the hatched bar the original marks a resizable dialog
// with, 6x24 in the archive.
const (
	chatResizeTexture         = basicInterfacePath + "dialog_resize.bmp"
	chatResizeW       float32 = 6
)

// The control pictograms. The plus and minus are the archive's own — the same
// pair the minimap zooms with, and the icons the original draws here. The
// chevron is built from the interface's right arrow, doubled. Nothing in the
// archive under any name I could find is a padlock, so that one is drawn.
const (
	chatIconMinus = minimapPath + minimapMinusAsset
	chatIconPlus  = minimapPath + minimapPlusAsset
	chatIconArrow = basicInterfacePath + "arw_right.bmp"

	chatIconSize float32 = 12
)

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

	// The control pictograms: dimmed until the pointer reaches them, and lit
	// while the thing they toggle is on.
	chatCtrlIdle = ui2d.Color{R: 0.82, G: 0.82, B: 0.80, A: 0.85}
	chatCtrlHot  = ui2d.Color{R: 1, G: 1, B: 1, A: 1}
	chatCtrlOn   = ui2d.Color{R: 1, G: 0.9, B: 0.4, A: 1}
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
		// the same way they do in NPC dialog. Text with no codes keeps the
		// color the line would have had.
		var expanded []TextRun
		for _, run := range runs {
			expanded = append(expanded, ParseNPCTextColored(run.Text, run.Color)...)
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
		// Flush into the bottom-left corner. Nothing sits below it now that
		// the debug bar is gone, so a margin only left a gap.
		b.chatX = 0
		b.chatY = screenH - chatInputH - chatHeight - chatTabH
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

	// The corner comes first: it sits inside the window, so if the window
	// took the press there would be nothing left to resize with.
	grip := ui2d.Rect{X: b.chatX + b.chatW - chatGrip, Y: b.chatY, W: chatGrip, H: chatGrip}
	hit := ui2d.Rect{X: b.chatX + b.chatW - chatGripHit, Y: b.chatY, W: chatGripHit, H: chatGripHit}
	b.drawChatGrip(grip, hit)

	// Resizing drags two numbers that are not the window's position, so it
	// cannot hand both to DragHandle. Width follows the pointer; height grows
	// as the top edge is pulled up, which is why the top's movement is added
	// back to the height.
	beforeY := b.chatY
	b.ctx.DragHandle("hud_chat_resize", hit, &b.chatW, &b.chatY)

	if b.chatY != beforeY {
		b.chatH += beforeY - b.chatY
	}

	// The whole box drags, not a strip of it. Picking out the bare parts of
	// the tab row left a target that was easy to miss and gave no sign of
	// where it was; pressing anywhere that is not itself a control is what
	// the box is expected to do.
	//
	// DragHandleFree is what makes that safe: it takes the press only if no
	// tab, pictogram, scrollbar, text field or the corner above has already
	// claimed it, all of which are handled before this runs.
	window := ui2d.Rect{
		X: b.chatX,
		Y: b.chatY,
		W: b.chatW,
		H: chatTabH + b.chatH + chatInputH,
	}

	if window.Contains(b.ctx.Input().MouseX, b.ctx.Input().MouseY) {
		b.wantCursor(cursor.StateClick)
	}

	beforeX, beforeDragY := b.chatX, b.chatY

	b.ctx.DragHandleFree("hud_chat_drag", window, &b.chatX, &b.chatY)

	// Traced because "it does not move" has two very different causes: the
	// press never reaching the handle, and the box being dragged into a
	// clamp. The second still reports movement here, so the log tells them
	// apart.
	if b.chatX != beforeX || b.chatY != beforeDragY {
		trace.Emit(trace.HUD, "chat-drag",
			zap.Float32("x", b.chatX), zap.Float32("y", b.chatY))
	}

	b.clampChatToScreen(screenH)
}

// clampChatToScreen keeps the box a sane size and on the screen.
func (b *UI2DBackend) clampChatToScreen(screenH float32) {
	b.chatW = clampF(b.chatW, chatMinW, chatMaxW)
	b.chatH = clampF(b.chatH, chatMinH, chatMaxH)

	// Never off the bottom or the sides of the screen.
	maxTop := screenH - chatInputH - b.chatH - chatTabH
	b.chatY = clampF(b.chatY, 0, maxTop)
	b.chatX = clampF(b.chatX, 0, 4000)
}

// drawChatGrip marks the corner that can be pulled, and lights it while the
// pointer is over it so it is clear that it can be.
func (b *UI2DBackend) drawChatGrip(grip, hit ui2d.Rect) {
	hot := hit.Contains(b.ctx.Input().MouseX, b.ctx.Input().MouseY)

	color := chatBorder
	if hot {
		color = chatGripHot

		// RO has no dedicated resize pointer, so the hand it shows for
		// anything you press is the closest thing to "you can grab this" —
		// and the pointer changing at all is what says the corner is live.
		b.wantCursor(cursor.StateClick)
	}

	r := b.ctx.Renderer()

	// The hatched bar the original marks a resizable dialog with. It is a
	// vertical strip, so it is drawn along the corner's edge; if it is
	// missing, the stepped corner lines stand in.
	if tex, err := b.texCache.Load(chatResizeTexture); err == nil {
		tint := ui2d.ColorWhite
		if hot {
			tint = chatGripHot
		}

		r.DrawImage(tex.ID, grip.X+grip.W-chatResizeW, grip.Y, chatResizeW, chatTabH, tint)

		return
	}

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
			if b.chatTab != i {
				b.chatTab = i
				// The other tab's scroll position means nothing here.
				b.chatPinned = true
				b.chatScroll = 0
			}
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
		id    string
		w     float32
		on    bool
		icon  func(box ui2d.Rect, tint ui2d.Color)
		click func()
	}

	controls := []control{
		{"chat_ctrl_lock", chatCtrlBtn, b.chatLocked, b.drawPadlock, func() {
			b.chatLocked = !b.chatLocked
		}},
		{"chat_ctrl_minus", chatCtrlBtn, false, b.icon(chatIconMinus), func() {
			b.stepChatHeight(-chatStepLines)
		}},
		{"chat_ctrl_plus", chatCtrlBtn, false, b.icon(chatIconPlus), func() {
			b.stepChatHeight(chatStepLines)
		}},
		{"chat_ctrl_last", chatCtrlBtn + 6, false, b.drawChevrons, func() {
			b.chatPinned = true
		}},
	}

	btnY := y + (chatTabH-chatCtrlBtn)/2

	for _, c := range controls {
		btnX := right - c.w
		right -= c.w + chatCtrlGap

		if btnX < x {
			break
		}

		box := ui2d.Rect{X: btnX, Y: btnY, W: c.w, H: chatCtrlBtn}

		// Pictograms, not buttons: the original puts the marks straight on
		// the strip with nothing drawn around them. The archive's icons carry
		// their own color, so hovering lights them to full brightness rather
		// than tinting them something else.
		tint := chatCtrlIdle
		switch {
		case c.on:
			tint = chatCtrlOn
		case box.Contains(b.ctx.Input().MouseX, b.ctx.Input().MouseY):
			tint = chatCtrlHot
		}

		c.icon(box, tint)

		if b.ctx.InvisibleButtonAt("hud_"+c.id, box.X, box.Y, box.W, box.H) {
			c.click()
		}
	}
}

// icon draws one of the archive's bitmaps centered in the box.
func (b *UI2DBackend) icon(path string) func(ui2d.Rect, ui2d.Color) {
	return func(box ui2d.Rect, tint ui2d.Color) {
		tex, err := b.texCache.Load(path)
		if err != nil {
			return
		}

		x := box.X + (box.W-chatIconSize)/2
		y := box.Y + (box.H-chatIconSize)/2
		b.ctx.Renderer().DrawImage(tex.ID, x, y, chatIconSize, chatIconSize, tint)
	}
}

// drawChevrons is the jump-to-newest mark: the interface's right arrow twice,
// overlapped, which is the doubled chevron the original uses.
func (b *UI2DBackend) drawChevrons(box ui2d.Rect, tint ui2d.Color) {
	tex, err := b.texCache.Load(chatIconArrow)
	if err != nil {
		return
	}

	const size float32 = 11

	y := box.Y + (box.H-size)/2
	x := box.X + (box.W-size-size/2)/2

	r := b.ctx.Renderer()
	r.DrawImage(tex.ID, x, y, size, size, tint)
	r.DrawImage(tex.ID, x+size/2, y, size, size, tint)
}

// drawPadlock draws the lock from primitives: a body with a shackle over it.
// The archive has no padlock I could find under any plausible name, and a
// letter L in its place was the thing that could not be read.
func (b *UI2DBackend) drawPadlock(box ui2d.Rect, tint ui2d.Color) {
	r := b.ctx.Renderer()

	const (
		bodyW float32 = 10
		bodyH float32 = 7
		arch  float32 = 4
	)

	bodyX := box.X + (box.W-bodyW)/2
	bodyY := box.Y + (box.H-bodyH-arch)/2 + arch

	// The shackle: two uprights and a bar across them, inset so it reads as
	// an arch rather than a second box.
	shackleX := bodyX + 2
	shackleW := bodyW - 4
	r.DrawRect(shackleX, bodyY-arch, shackleW, 2, tint)
	r.DrawRect(shackleX, bodyY-arch, 2, arch, tint)
	r.DrawRect(shackleX+shackleW-2, bodyY-arch, 2, arch, tint)

	r.DrawRect(bodyX, bodyY, bodyW, bodyH, tint)
}

// stepChatHeight grows or shrinks the box by whole lines, keeping the input
// bar where it is: the bar is what the eye tracks, and having it walk up the
// screen every time the scrollback grew would be the wrong thing to move.
func (b *UI2DBackend) stepChatHeight(lines float32) {
	want := clampF(b.chatH+lines*b.chatLineH(), chatMinH, chatMaxH)
	b.chatY -= want - b.chatH
	b.chatH = want
	b.chatPinned = true
}

// drawChatLines draws the scrollback, newest at the bottom.
func (b *UI2DBackend) drawChatLines(state InGameUIState, x, y, w, h float32) {
	r := b.ctx.Renderer()

	textW := w - 2*chatPadding - chatScrollW
	wrapped := b.wrapChat(b.chatTabLines(state.ChatLines), textW)

	usableH := h - 2*chatPadding
	visible := int(usableH / b.chatLineH())
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
		lineY := y + chatPadding + float32(i)*b.chatLineH()
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
	nameBox := chatInputBox(x, y, w, chatNameBoxL, chatNameBoxR)
	msgBox := chatInputBox(x, y, w, chatMsgBoxL, chatMsgBoxR)

	name, _, nameSubmit := b.ctx.TextInputBareAt("hud_chat_name",
		nameBox.X, nameBox.Y, nameBox.W, nameBox.H, chatInputScale, b.chatName)
	b.chatName = name

	msg, _, msgSubmit := b.ctx.TextInputBareAt("hud_chat_input",
		msgBox.X, msgBox.Y, msgBox.W, msgBox.H, chatInputScale, b.chatInput)
	b.chatInput = msg

	// The fields are drawn bare, into boxes the background already paints, so
	// an outline is the only thing that says where the typing goes.
	b.outlineIfFocused("hud_chat_name", nameBox)
	b.outlineIfFocused("hud_chat_input", msgBox)

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

// chatInputBox maps one of the boxes painted into dialog_bg.bmp onto the bar
// as it is actually drawn, so the field lands inside its box whatever width
// the chat has been dragged to.
func chatInputBox(x, y, w, left, right float32) ui2d.Rect {
	return ui2d.Rect{
		X: x + w*left,
		Y: y + chatInputH*chatBoxT,
		W: w * (right - left),
		H: chatInputH * (chatBoxB - chatBoxT),
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
func (b *UI2DBackend) outlineIfFocused(id string, box ui2d.Rect) {
	if !b.ctx.Focused(id) {
		return
	}

	r := b.ctx.Renderer()
	r.DrawRect(box.X, box.Y, box.W, 1, chatFocusBorder)
	r.DrawRect(box.X, box.Y+box.H-1, box.W, 1, chatFocusBorder)
	r.DrawRect(box.X, box.Y, 1, box.H, chatFocusBorder)
	r.DrawRect(box.X+box.W-1, box.Y, 1, box.H, chatFocusBorder)
}

// chatLineH is how far apart two messages sit: the font's own line advance
// plus a gap. Measured rather than fixed, because a constant that looks right
// on one font at one pixel density crowds the lines on another — at 14px the
// descenders of one message ran into the next.
func (b *UI2DBackend) chatLineH() float32 {
	return b.ctx.Renderer().FontLineHeight(1) + chatLineGap
}

// chatTabLines is the scrollback the selected tab shows.
//
// The tabs were decoration until now: switching them changed which one was
// lit and nothing else. Public carries what people said and what the server
// announced; Battle carries the damage and the messages about fighting, which
// is what you want out of the way while talking.
func (b *UI2DBackend) chatTabLines(lines []states.ChatLine) []states.ChatLine {
	if b.chatTab < 0 || b.chatTab >= len(chatTabs) {
		return lines
	}

	battle := chatTabs[b.chatTab] == "Battle"

	filtered := make([]states.ChatLine, 0, len(lines))
	for _, line := range lines {
		if (line.Kind == packets.ChatDamage) == battle {
			filtered = append(filtered, line)
		}
	}

	return filtered
}
