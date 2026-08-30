package ui

import (
	"strings"

	"github.com/Faultbox/midgard-ro/internal/config"
	"github.com/Faultbox/midgard-ro/internal/engine/ui2d"
	"github.com/Faultbox/midgard-ro/internal/game/states"
	"github.com/Faultbox/midgard-ro/internal/logger"
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

	// The bar is dialog_bg.bmp, 600x24, and it is drawn in three slices
	// rather than stretched whole.
	//
	// Stretching it was wrong: the name field and the separator are a fixed
	// size in the original, and the message field is what grows. Uniform
	// scaling made a wide chat's name field balloon and carried the separator
	// off with it, which is not where the original puts either.
	//
	// So the left cap up to x=110 and the right cap from x=568 are drawn at
	// their own size, and only the middle stretches. Within the caps: the
	// name box spans x 6-93, the ribbed separator 95-107, and the message box
	// starts at 110. Vertically all of them run y 5 to 21 of 24.
	chatBGW float32 = 600
	chatBGH float32 = 24

	chatCapL float32 = 110
	chatCapR float32 = chatBGW - 568

	chatNameL float32 = 6
	chatNameR float32 = 93
	chatSepL  float32 = 95
	chatSepR  float32 = 107

	chatBoxT float32 = 5
	chatBoxB float32 = 21

	// chatOrb is one of the two round buttons in the right cap.
	chatOrb float32 = 11

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
// chatInputOrb is the round button the original puts at the right of the
// input bar, twice.
const chatInputOrb = basicInterfacePath + "sys_base_off.bmp"

// chatInputList is what sits between the two fields.
//
// It is not a separator, which is why drawing one never looked right: it is a
// button, 8x18, a blue face with four ribs down it and a bevel around it.
// roBrowser calls it .list and it opens the names you have whispered before.
// We keep no such list yet, so it is drawn and does nothing.
const (
	chatInputList         = basicInterfacePath + "dialog_btn0.bmp"
	chatListW     float32 = 8
	chatListH     float32 = 18
)

// chatInputBG is the bar's background. The archive carries it in two colors at
// identical geometry, dialog_bg.bmp in grey and dialog_bg2.bmp in blue; the
// chat uses the grey one, which is what the original draws under this bar.
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
	// What someone else said. Everything with no color of its own falls
	// back to this.
	chatColorMessage = ui2d.Color{R: 1, G: 1, B: 1, A: 1}
	// Our own words, and the server's answer to an @ command. Both are
	// PUBLIC|SELF to the original, which paints that green — the single most
	// visible correction in #94's step 5, since we painted it yellow.
	chatColorSelf = ui2d.Color{R: 0, G: 1, B: 0, A: 1}
	// A private message.
	chatColorWhisper = ui2d.Color{R: 1, G: 1, B: 0, A: 1}
	// A notice the client wrote itself — what /where prints, what /bgm says
	// it did. The original's INFO color, a yellow lightened away from the
	// whisper's so the two do not read as the same line.
	chatColorNotice = ui2d.Color{R: 1, G: 1, B: 0.388, A: 1}
	// A command that could not run.
	chatColorError = ui2d.Color{R: 1, G: 0, B: 0, A: 1}
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

	// The blue inside the archive's icons, sampled from map_plus0.bmp, and
	// the white they are outlined in. The outline is why they stay legible
	// over pavement as well as over the dark strip, so the drawn lock is
	// built the same way — plain white vanished on Prontera.
	chatIconBlue    = ui2d.Color{R: 0.098, G: 0.443, B: 0.898, A: 1}
	chatIconOutline = ui2d.Color{R: 1, G: 1, B: 1, A: 1}
	chatCtrlHot     = ui2d.Color{R: 1, G: 1, B: 1, A: 1}
	chatCtrlOn      = ui2d.Color{R: 1, G: 0.9, B: 0.4, A: 1}

	// chatLockOn is the lock's own on-color, dark enough to read against the
	// white it is outlined in.
	chatLockOn = ui2d.Color{R: 0.35, G: 0.35, B: 0.38, A: 1}
)

// chatLineColor is the color a line is drawn in.
//
// A color the server chose wins over the kind. Only ZC_NPC_CHAT and
// ZC_BROADCAST carry one, and for those the server has already decided —
// @cash picks its own green, a blue broadcast is blue — so overriding it
// with the kind's color would discard the thing the packet exists to say.
func chatLineColor(line states.ChatLine) ui2d.Color {
	if line.HasColor {
		return rgbColor(line.Color)
	}

	return chatKindColor(line.Kind)
}

// rgbColor turns a packed 0xRRGGBB into a drawable color.
func rgbColor(rgb uint32) ui2d.Color {
	return ui2d.Color{
		R: float32((rgb>>16)&0xFF) / 255,
		G: float32((rgb>>8)&0xFF) / 255,
		B: float32(rgb&0xFF) / 255,
		A: 1,
	}
}

// chatKindColor is the color a line with no color of its own is drawn in.
//
// The palette is the original's, as roBrowser transcribes it and rAthena's own
// color table confirms: our words and the server's replies green, other
// people white, whispers yellow, errors red, notices light yellow.
func chatKindColor(kind states.ChatKind) ui2d.Color {
	switch kind {
	case states.ChatSelf, states.ChatSystem:
		// Both are PUBLIC|SELF to the original. ChatSystem is every reply to
		// an @ command, which is what makes this the visible half of the fix.
		return chatColorSelf
	case states.ChatWhisper:
		return chatColorWhisper
	case states.ChatNotice:
		return chatColorNotice
	case states.ChatError:
		return chatColorError
	case states.ChatDamage:
		return chatColorDamage
	case states.ChatBroadcast:
		// Only reached when the decoder found no marker to read a color
		// from, which it always does — kept so the switch is total.
		return chatColorNotice
	default:
		// Someone else speaking.
		return chatColorMessage
	}
}

// chatRuns turns one line into the colored runs that draw it.
//
// The speaker is drawn in its own color, which is the reason the decoder
// splits it out of the message rather than leaving the line whole. A line with
// no speaker — a server message — is one run.
func chatRuns(line states.ChatLine) []TextRun {
	body := TextRun{Text: line.Text, Color: chatLineColor(line)}

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
		b.placeChat(screenH)
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
		b.chatDirty = true
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

	beforeX, beforeDragY := b.chatX, b.chatY

	b.ctx.DragHandleFree("hud_chat_drag", window, &b.chatX, &b.chatY)

	// Traced because "it does not move" has two very different causes: the
	// press never reaching the handle, and the box being dragged into a
	// clamp. The second still reports movement here, so the log tells them
	// apart.
	if b.chatX != beforeX || b.chatY != beforeDragY {
		b.chatDirty = true

		trace.Emit(trace.HUD, "chat-drag",
			zap.Float32("x", b.chatX), zap.Float32("y", b.chatY))
	}

	// Written when the drag ends rather than every frame it moves: one file
	// per gesture instead of one per frame.
	if b.chatDirty && b.ctx.Input().MouseLeftReleased {
		b.saveChatPlacement()
	}

	b.clampChatToScreen(screenH)
}

// placeChat puts the box where it was last left, or in the bottom-left corner
// the first time. Nothing sits below it, so it goes flush into the corner —
// a margin would only leave a gap.
func (b *UI2DBackend) placeChat(screenH float32) {
	b.chatX = 0
	b.chatY = screenH - chatInputH - chatHeight - chatTabH
	b.chatW, b.chatH = chatWidth, chatHeight
	b.chatPlaced = true

	// A remembered size of zero is a state file written before the box was
	// remembered at all, so the defaults above stand.
	saved := config.LoadUIState()
	if saved.ChatW <= 0 || saved.ChatH <= 0 {
		return
	}

	b.chatX, b.chatY = saved.ChatX, saved.ChatY
	b.chatW, b.chatH = saved.ChatW, saved.ChatH
	b.chatLocked = saved.ChatLocked

	// The screen may be a different size than it was last time, so what was
	// on it then need not be now.
	b.clampChatToScreen(screenH)
}

// saveChatPlacement records where the box was left.
func (b *UI2DBackend) saveChatPlacement() {
	b.chatDirty = false

	err := config.UpdateUIState(func(state *config.UIState) {
		state.ChatX, state.ChatY = b.chatX, b.chatY
		state.ChatW, state.ChatH = b.chatW, b.chatH
		state.ChatLocked = b.chatLocked
	})
	if err != nil {
		logger.Warn("could not save chat placement", zap.Error(err))
	}
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
			b.saveChatPlacement()
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

// drawPadlock draws the lock from primitives, in the archive's own icon
// colors: a blue shape inside a white outline. The archive has no padlock I
// could find under any plausible name, and a plain white one was unreadable
// against pale ground.
func (b *UI2DBackend) drawPadlock(box ui2d.Rect, tint ui2d.Color) {
	// Grey while it is on. Yellow was the obvious "active" color and the
	// wrong one here: against the white it is outlined in, over pale ground,
	// it had almost nothing to contrast with, so engaging the lock made it
	// disappear.
	fill := chatIconBlue
	if b.chatLocked {
		fill = chatLockOn
	}

	// Hovering brightens the outline, which is the only feedback a pictogram
	// with no chrome around it can give.
	outline := chatIconOutline
	if tint == chatCtrlIdle {
		outline = ui2d.Color{R: 0.85, G: 0.85, B: 0.83, A: 0.9}
	}

	const (
		bodyW float32 = 9
		bodyH float32 = 7
		arch  float32 = 4
	)

	bodyX := box.X + (box.W-bodyW)/2
	bodyY := box.Y + (box.H-bodyH-arch)/2 + arch

	// Outline first, as a shape one pixel larger all round, then the fill on
	// top of it.
	shackleX := bodyX + 1.5
	shackleW := bodyW - 3

	drawLock := func(inset float32, c ui2d.Color) {
		r := b.ctx.Renderer()

		// The shackle: two uprights and a bar across them.
		r.DrawRect(shackleX-inset, bodyY-arch-inset, shackleW+2*inset, 2, c)
		r.DrawRect(shackleX-inset, bodyY-arch-inset, 2, arch+inset, c)
		r.DrawRect(shackleX+shackleW+inset-2, bodyY-arch-inset, 2, arch+inset, c)

		r.DrawRect(bodyX-inset, bodyY-inset, bodyW+2*inset, bodyH+2*inset, c)
	}

	drawLock(1, outline)
	drawLock(0, fill)
}

// stepChatHeight grows or shrinks the box by whole lines, keeping the input
// bar where it is: the bar is what the eye tracks, and having it walk up the
// screen every time the scrollback grew would be the wrong thing to move.
func (b *UI2DBackend) stepChatHeight(lines float32) {
	want := clampF(b.chatH+lines*b.chatLineH(), chatMinH, chatMaxH)
	b.chatY -= want - b.chatH
	b.chatH = want
	b.chatPinned = true

	b.saveChatPlacement()
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
	b.drawChatInputBG(x, y, w)

	// Two fields, as the original has: a name and a message. Leaving the name
	// blank talks to everyone; filling it in makes the line a whisper. They
	// take focus independently, by click or by Tab.
	//
	// The name field keeps its size whatever the chat's width; the message
	// field is the one that grows, ending short of the right cap the round
	// buttons sit in.
	top := y + chatInputH*(chatBoxT/chatBGH)
	boxH := chatInputH * ((chatBoxB - chatBoxT) / chatBGH)

	nameBox := ui2d.Rect{X: x + chatNameL, Y: top, W: chatNameR - chatNameL, H: boxH}
	msgBox := ui2d.Rect{X: x + chatCapL, Y: top, W: w - chatCapL - chatCapR, H: boxH}

	name, _, nameSubmit := b.ctx.TextInputBareAt("hud_chat_name",
		nameBox.X, nameBox.Y, nameBox.W, nameBox.H, chatInputScale, b.chatName)
	b.chatName = name

	msg, _, msgSubmit := b.ctx.TextInputBareAt("hud_chat_input",
		msgBox.X, msgBox.Y, msgBox.W, msgBox.H, chatInputScale, b.chatInput)
	b.chatInput = msg

	// The button between the fields, centered in the gap the background
	// leaves for it.
	if tex, err := b.texCache.Load(chatInputList); err == nil {
		listX := x + chatSepL + (chatSepR-chatSepL-chatListW)/2
		listY := y + (chatInputH-chatListH)/2
		r.DrawImage(tex.ID, listX, listY, chatListW, chatListH, ui2d.ColorWhite)
	}

	b.drawChatOrbs(x+w-chatCapR, y)

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

// drawChatInputBG lays the bar down in three slices, so the caps keep their
// size and only the middle stretches.
func (b *UI2DBackend) drawChatInputBG(x, y, w float32) {
	r := b.ctx.Renderer()

	tex, err := b.texCache.Load(chatInputBG)
	if err != nil {
		r.DrawRect(x, y, w, chatInputH, chatBackground)

		return
	}

	capL, capR := chatCapL, chatCapR

	// A box narrower than its own caps has nothing left to stretch; the caps
	// share what there is rather than overlapping into each other.
	if capL+capR > w {
		capL, capR = w/2, w/2
	}

	r.DrawImageUV(tex.ID, x, y, capL, chatInputH,
		0, 0, chatCapL/chatBGW, 1, ui2d.ColorWhite)

	if mid := w - capL - capR; mid > 0 {
		r.DrawImageUV(tex.ID, x+capL, y, mid, chatInputH,
			chatCapL/chatBGW, 0, (chatBGW-chatCapR)/chatBGW, 1, ui2d.ColorWhite)
	}

	r.DrawImageUV(tex.ID, x+w-capR, y, capR, chatInputH,
		(chatBGW-chatCapR)/chatBGW, 0, 1, 1, ui2d.ColorWhite)
}

// drawChatOrbs draws the pair of round buttons in the bar's right cap. The
// original has them; what they open is not built yet, so they are drawn and
// not wired to anything.
func (b *UI2DBackend) drawChatOrbs(capX, y float32) {
	tex, err := b.texCache.Load(chatInputOrb)
	if err != nil {
		return
	}

	r := b.ctx.Renderer()
	orbY := y + (chatInputH-chatOrb)/2

	// Right-aligned in the cap, with the same gap between them as after them.
	const gap float32 = 3
	for i := float32(0); i < 2; i++ {
		orbX := capX + chatCapR - (i+1)*(chatOrb+gap)
		r.DrawImage(tex.ID, orbX, orbY, chatOrb, chatOrb, ui2d.ColorWhite)
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

// QueueChatMessage puts a line in as though it had been typed, for --say.
//
// It writes the same field the input bar writes and leaves the whisper target
// alone, so a queued line is routed by exactly the rules a typed one is —
// including whatever is sitting in the name field, which is the whole point of
// being able to test that.
//
// Reports false when a line is already pending, so the caller can wait rather
// than overwrite one that has not been sent yet.
func (b *UI2DBackend) QueueChatMessage(text string) bool {
	if b.chatPending != "" {
		return false
	}

	b.chatPending = text
	b.chatPinned = true

	return true
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
		if (line.Kind == states.ChatDamage) == battle {
			filtered = append(filtered, line)
		}
	}

	return filtered
}
