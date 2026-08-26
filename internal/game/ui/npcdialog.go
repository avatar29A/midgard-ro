package ui

import (
	"go.uber.org/zap"

	"github.com/Faultbox/midgard-ro/internal/engine/ui2d"
	"github.com/Faultbox/midgard-ro/internal/logger"
)

// The NPC dialog window. Measured off ref-01, a lossless capture of the
// original at native size, and agreeing with roBrowser's `NpcBox.css` once its
// 1px border is added to the content box it declares.
//
// It is deliberately not a `BeginWindow`: this window has no title bar. It is
// a plain panel — a thin grey rule around a near-white field — and giving it
// the standard chrome would put a gradient caption bar on something that has
// never had one.
const (
	npcWinW = float32(278)
	npcWinH = float32(178)

	// The border is one pixel, and the text field is inset from it.
	npcWinBorder = float32(1)
	npcTextInset = float32(9)

	// Where the window sits. The original does not fix it — the three
	// captures disagree — so it is centered horizontally and set near the
	// bottom, out of the way of the panel in the top-left.
	npcWinBottomGap = float32(120)

	// The message is set at the same reduced size as the rest of our RO
	// windows.
	npcTextScale = loginTextScale

	// Line spacing, and the lift that turns a cap position into the line box
	// DrawText actually places.
	//
	// 21 is measured, not chosen: in ref-01 the text baselines fall on rows
	// 138, 159, 180, 201 and 222 — an exact 21px pitch — with glyphs 9 to 11
	// px tall. Our glyphs are about the same size, so the original is nearly
	// double-spaced and 13 was cramped. It fits six or seven lines in this
	// window, which is what ref-01 shows.
	npcLineHeight = float32(21)
	npcTextLift   = charSelTextLift

	// The button sits at the bottom right inside the window, which is where
	// ref-02 shows it.
	npcBtnW      = float32(42)
	npcBtnH      = float32(20)
	npcBtnMargin = float32(8)
)

// npcDialogButtons is the three-state art for Next and Close.
//
// Both live at the root of the interface folder, not in `basic_interface` or
// `login_interface` — roBrowser names them without a prefix, and the archive
// has a full set there. The plan had pointed at two different buttons of the
// same name in subfolders, one of which genuinely has no hover art; these do.
var npcDialogButtons = map[string]string{
	"next":  "btn_next",
	"close": "btn_close",
}

var (
	npcWinBorderColor = ui2d.Color{R: 0.773, G: 0.773, B: 0.773, A: 1}
	npcWinFillColor   = ui2d.Color{R: 0.969, G: 0.969, B: 0.969, A: 1}
)

// fillNPCRect paints a rectangle in the image layer, so that widgets drawn
// after it — which are images — end up on top of it rather than under.
func (b *UI2DBackend) fillNPCRect(x, y, w, h float32, color ui2d.Color) {
	if b.whiteTex == 0 {
		b.whiteTex = b.ctx.Renderer().CreateTextureNearest(1, 1, []byte{255, 255, 255, 255})
	}

	b.ctx.Renderer().DrawImage(b.whiteTex, x, y, w, h, color)
}

// npcButtonSkin is one dialog button's three states.
type npcButtonSkin struct {
	normal, hover, pressed *TextureInfo
}

// loadNPCButton loads a dialog button, once. A miss returns nil and the button
// is left out — the conversation is then stuck, which is worse than ugly, so
// it warns.
func (b *UI2DBackend) loadNPCButton(name string) *npcButtonSkin {
	if skin, ok := b.npcButtons[name]; ok {
		return skin
	}

	if b.npcButtons == nil {
		b.npcButtons = map[string]*npcButtonSkin{}
	}

	base := npcDialogButtons[name]
	states := make([]*TextureInfo, 0, 3)

	for _, suffix := range []string{"", "_a", "_b"} {
		tex, err := b.texCache.Load(skinBasePath + base + suffix + ".bmp")
		if err != nil {
			logger.Warn("NPC dialog button art unavailable, the conversation cannot be advanced",
				zap.String("path", skinBasePath+base+suffix+".bmp"), zap.Error(err))

			b.npcButtons[name] = nil

			return nil
		}

		states = append(states, tex)
	}

	skin := &npcButtonSkin{normal: states[0], hover: states[1], pressed: states[2]}
	b.npcButtons[name] = skin

	return skin
}

// renderNPCDialog draws what the NPC said, and reports whether anything was
// drawn.
func (b *UI2DBackend) renderNPCDialog(state InGameUIState, width, height float32) bool {
	if state.DialogMessage == "" {
		return false
	}

	if !b.npcWinPlaced {
		b.npcWinX = float32(int((width - npcWinW) / 2))
		b.npcWinY = float32(int(height - npcWinBottomGap - npcWinH))
		b.npcWinPlaced = true
	}

	// Dragged by the window itself, since it has no title bar to grab. The
	// button is drawn afterwards and claims the pointer for itself, so
	// pressing it does not also drag. Where it is put is kept for the next
	// conversation — a window that jumped back to the middle every time you
	// spoke to someone would be worth moving only once.
	b.ctx.DragHandle("npc_dialog", ui2d.Rect{X: b.npcWinX, Y: b.npcWinY, W: npcWinW, H: npcWinH},
		&b.npcWinX, &b.npcWinY)

	x, y := b.npcWinX, b.npcWinY
	r := b.ctx.Renderer()

	// Drawn as tinted images rather than with DrawRect, which would be the
	// obvious way and is wrong here. The renderer batches images, then solids,
	// then text — so a solid background is painted *over* the button, which is
	// an image. The button was invisible and still clickable until this
	// changed: hit tests do not care what is on top.
	b.fillNPCRect(x, y, npcWinW, npcWinH, npcWinBorderColor)
	b.fillNPCRect(x+npcWinBorder, y+npcWinBorder,
		npcWinW-2*npcWinBorder, npcWinH-2*npcWinBorder, npcWinFillColor)

	textX := x + npcTextInset
	textY := y + npcTextInset
	textW := npcWinW - 2*npcTextInset
	textBottom := y + npcWinH - npcTextInset

	measure := func(s string) float32 {
		w, _ := r.MeasureText(s, npcTextScale)
		return w
	}

	// A button eats the last line's worth of space, so the text stops above it.
	if state.DialogShowNext || state.DialogShowClose {
		textBottom -= npcBtnH
	}

	lines := WrapNPCText(ParseNPCText(state.DialogMessage), textW, measure)

	// Show the end of a conversation that has outgrown its box, not the
	// beginning. Successive messages append, so what was just said is what
	// the player wants to read — this is the scroll, without a scrollbar.
	if fits := int((textBottom - textY) / npcLineHeight); fits > 0 && len(lines) > fits {
		lines = lines[len(lines)-fits:]
	}

	for _, line := range lines {
		runX := textX
		for _, run := range line {
			r.DrawText(runX, textY-npcTextLift, run.Text, npcTextScale, run.Color)
			runX += measure(run.Text)
		}

		textY += npcLineHeight
	}

	b.drawNPCDialogButton(state, x, y)

	// The window swallows clicks, so talking to an NPC does not also walk you
	// into the scenery behind the window.
	b.ctx.CaptureMouse(ui2d.Rect{X: x, Y: y, W: npcWinW, H: npcWinH})

	return true
}

// drawNPCDialogButton draws whichever button the server asked for.
//
// Never both: the script is either going to say more or it is finished, and
// the packets that ask for them are separate.
func (b *UI2DBackend) drawNPCDialogButton(state InGameUIState, x, y float32) {
	name, label, action := "", "", state.OnDialogNext

	switch {
	case state.DialogShowNext:
		name, label = "next", "next"
	case state.DialogShowClose:
		name, label, action = "close", "close", state.OnDialogClose
	default:
		return
	}

	skin := b.loadNPCButton(name)
	if skin == nil {
		return
	}

	btnX := x + npcWinW - npcBtnMargin - npcBtnW
	btnY := y + npcWinH - npcBtnMargin - npcBtnH

	// The archive's art has its caption baked in, in Korean — both families
	// of this button do, so there is no English one to pick instead. Masked
	// and relabelled the same way the login window's buttons are, by the
	// helper that already knows where the ink sits in a 42x20 button.
	if b.skinButton("npc_dialog_"+name, btnX, btnY,
		skin.normal, skin.hover, skin.pressed, label) && action != nil {
		action()
	}
}
