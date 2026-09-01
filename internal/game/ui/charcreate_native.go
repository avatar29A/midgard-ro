package ui

import (
	"strconv"

	"go.uber.org/zap"

	"github.com/Faultbox/midgard-ro/internal/engine/ui2d"
	"github.com/Faultbox/midgard-ro/internal/logger"
	"github.com/Faultbox/midgard-ro/internal/trace"
)

// The character creation screen.
//
// Like character select it is one background with everything drawn on top,
// but this frame carries less: the title bar, the pastel panel, the podium the
// sprite stands on, the name well, the swatch box and the scrollbar track are
// painted in, and nothing else is. The race cards, the sex toggle, the sprite,
// the hair grid, the swatches and both bottom buttons are ours to draw.
const (
	// charCreateWinW and charCreateWinH are bg_makebg.bmp's own size. Drawn
	// at native resolution rather than stretched, as character select is.
	charCreateWinW = float32(794)
	charCreateWinH = float32(422)

	// The bottom row. Neither button is painted into the frame.
	charCreateBtnW    = float32(160)
	charCreateBtnH    = float32(25)
	charCreateBtnPadX = float32(18)
	charCreateBtnY    = charCreateWinH - 41
)

// charCreateSkin is the creation screen's art.
type charCreateSkin struct {
	window *TextureInfo
}

// loadCharCreateSkin loads the creation frame. A miss leaves the skin nil and
// the caller falls back to a themed window, the same bargain character select
// makes.
func (b *UI2DBackend) loadCharCreateSkin() *charCreateSkin {
	if b.charCreateSkin != nil {
		return b.charCreateSkin
	}

	if b.charCreateTried {
		return nil
	}

	b.charCreateTried = true

	tex, err := b.texCache.Load(makeCharVer2TexBasePath + `bg_makebg.bmp`)
	if err != nil {
		logger.Warn("character creation art unavailable, falling back to the themed window",
			zap.String("path", makeCharVer2TexBasePath+`bg_makebg.bmp`), zap.Error(err))

		return nil
	}

	b.charCreateSkin = &charCreateSkin{window: tex}

	return b.charCreateSkin
}

// RenderCharCreateUI renders the character creation screen.
func (b *UI2DBackend) RenderCharCreateUI(state CharCreateUIState, width, height float32) {
	// The same backdrop character select uses, so moving between the two does
	// not change the scenery behind them.
	b.loadLoginTextures()
	if b.loginBgTex != nil {
		b.ctx.Renderer().DrawImage(b.loginBgTex.ID, 0, 0, width, height, ui2d.ColorWhite)
	}

	if b.renderNativeCharCreate(state, width, height) {
		return
	}

	b.renderFallbackCharCreate(state, width, height)
}

// renderNativeCharCreate draws the original frame, reporting whether it could.
func (b *UI2DBackend) renderNativeCharCreate(state CharCreateUIState, width, height float32) bool {
	skin := b.loadCharCreateSkin()
	if skin == nil {
		return false
	}

	x := (width - charCreateWinW) / 2
	y := (height - charCreateWinH) / 2

	b.ctx.Renderer().DrawImage(skin.window.ID, x, y, charCreateWinW, charCreateWinH, ui2d.ColorWhite)

	b.drawCharCreateButtons(state, x, y)
	b.drawCharCreateMessages(state, x, y)

	return true
}

// drawCharCreateButtons draws Go back and Create along the bottom.
//
// Neither is in the frame, so both are drawn rather than skinned. Create sends
// nothing yet — the packet is step 5 — but it is drawn now because a screen
// with only a way out reads as broken.
func (b *UI2DBackend) drawCharCreateButtons(state CharCreateUIState, x, y float32) {
	backX := x + charCreateBtnPadX
	createX := x + charCreateWinW - charCreateBtnPadX - charCreateBtnW
	btnY := y + charCreateBtnY

	if b.ctx.ButtonAt("charcreate_back", backX, btnY, charCreateBtnW, charCreateBtnH, "Go back") {
		trace.Emit(trace.Char, "create-cancel-click", zap.Int("slot", state.Slot))

		if state.OnCancel != nil {
			state.OnCancel()
		}
	}

	if b.ctx.ButtonAt("charcreate_create", createX, btnY, charCreateBtnW, charCreateBtnH, "Create") {
		// Deliberately inert until step 5 wires CH_MAKE_CHAR. Traced so a
		// press is visible rather than looking like a dead button.
		trace.Emit(trace.Char, "create-click", zap.Int("slot", state.Slot))
	}
}

// drawCharCreateMessages puts the status and any refusal under the buttons.
func (b *UI2DBackend) drawCharCreateMessages(state CharCreateUIState, x, y float32) {
	r := b.ctx.Renderer()
	textY := y + charCreateWinH + 6

	if state.ErrorMessage != "" {
		r.DrawText(x+charCreateBtnPadX, textY, state.ErrorMessage, loginTextScale,
			ui2d.Color{R: 1, G: 0.35, B: 0.35, A: 1})

		return
	}

	if state.StatusMessage != "" {
		r.DrawText(x+charCreateBtnPadX, textY, state.StatusMessage, loginTextScale,
			ui2d.ColorWhite)
	}
}

// renderFallbackCharCreate draws a themed window when the art is missing, so
// the screen is still usable and still has a way back.
func (b *UI2DBackend) renderFallbackCharCreate(state CharCreateUIState, width, height float32) {
	const w, h = float32(420), float32(200)

	x := (width - w) / 2
	y := (height - h) / 2

	if !b.ctx.BeginWindowEx("charcreate", x, y, w, h, "Create Character", ui2d.WindowOptions{}) {
		return
	}

	b.ctx.Row(20)
	b.ctx.Label("Character creation art is unavailable.")
	b.ctx.Row(20)
	b.ctx.Label("Slot " + strconv.Itoa(state.Slot))

	if state.ErrorMessage != "" {
		b.ctx.Row(20)
		b.ctx.Label(state.ErrorMessage)
	}

	b.ctx.Row(28)
	if b.ctx.Button("charcreate_back_fallback", 120, "Go back") && state.OnCancel != nil {
		state.OnCancel()
	}

	b.ctx.EndWindow()
}
