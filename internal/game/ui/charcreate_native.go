package ui

import (
	"strconv"

	"go.uber.org/zap"

	"github.com/Faultbox/midgard-ro/internal/engine/charsprite"
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

	// The sex toggle, two 63x25 pills above the preview.
	charCreateSexW    = float32(63)
	charCreateSexH    = float32(25)
	charCreateSexMinX = float32(434)
	charCreateSexY    = float32(60)

	// Where the preview stands: the middle of the podium painted into the
	// frame, and the ground line its feet rest on.
	charCreatePodiumX = float32(500)
	charCreatePodiumY = float32(242)

	// How tall and wide the preview may be before it is scaled to fit, so a
	// tall sprite cannot climb out of the panel.
	charCreatePreviewMaxH = float32(150)
	charCreatePreviewMaxW = float32(110)

	// The turn arrows either side of the preview, 22x23.
	charCreateTurnW = float32(22)
	charCreateTurnH = float32(23)
	charCreateTurnY = float32(182)
	charCreateTurnL = float32(428)
	charCreateTurnR = float32(552)

	// The race cards on the left, which the frame leaves blank.
	charCreateCardX  = float32(18)
	charCreateCardW  = float32(367)
	charCreateCardH  = float32(120)
	charCreateHumanY = float32(45)
	charCreateDoramY = float32(180)
)

// The race cards. Drawn rather than skinned: the frame paints nothing on its
// left half, and the retail card art is a description and a job tree that say
// nothing our server acts on.
var (
	charCreateCardFace    = ui2d.Color{R: 0.97, G: 0.97, B: 0.99, A: 1}
	charCreateCardFaceSel = ui2d.Color{R: 0.87, G: 0.91, B: 0.99, A: 1}
	charCreateCardEdge    = ui2d.Color{R: 0.62, G: 0.66, B: 0.78, A: 1}
	charCreateCardEdgeSel = ui2d.Color{R: 0.25, G: 0.40, B: 0.75, A: 1}
	charCreateCardInk     = ui2d.Color{R: 0.16, G: 0.18, B: 0.24, A: 1}
)

// charCreateSkin is the creation screen's art.
//
// Only the window is required. Everything else is optional in the same sense
// character select's paging arrows are: without them the screen loses a
// control, which beats losing the screen.
type charCreateSkin struct {
	window *TextureInfo

	maleOff, maleOn     *TextureInfo
	femaleOff, femaleOn *TextureInfo
	turnLeft, turnRight *TextureInfo
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

	skin := &charCreateSkin{window: tex}

	skin.maleOff = b.optionalTexture(makeCharVer2TexBasePath + `bt_male_off.bmp`)
	skin.maleOn = b.optionalTexture(makeCharVer2TexBasePath + `bt_male_on.bmp`)
	skin.femaleOff = b.optionalTexture(makeCharVer2TexBasePath + `bt_female_off.bmp`)
	skin.femaleOn = b.optionalTexture(makeCharVer2TexBasePath + `bt_female_on.bmp`)
	skin.turnLeft = b.optionalTexture(makeCharVer2TexBasePath + `bt_leftturn_normal.bmp`)
	skin.turnRight = b.optionalTexture(makeCharVer2TexBasePath + `bt_rightturn_normal.bmp`)

	b.charCreateSkin = skin

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

	b.drawCharCreateRaces(state, x, y)
	b.drawCharCreateSex(skin, state, x, y)
	b.drawCharCreatePreview(state, x, y)
	b.drawCharCreateTurn(skin, state, x, y)
	b.drawCharCreateButtons(state, x, y)
	b.drawCharCreateMessages(state, x, y)

	return true
}

// drawCharCreateRaces draws the Human and Doram cards and switches between
// them.
//
// Both are live: our server is a RENEWAL build at a packet version where
// allowed_job_flag is 3, so Summoner is creatable alongside Novice. Retail
// shows Doram as "coming soon", which would be a lie here.
func (b *UI2DBackend) drawCharCreateRaces(state CharCreateUIState, x, y float32) {
	b.drawRaceCard(state, "charcreate_human", "Human  (Novice)",
		x+charCreateCardX, y+charCreateHumanY, state.Job == charsprite.JobNovice, charsprite.JobNovice)
	b.drawRaceCard(state, "charcreate_doram", "Doram  (Summoner)",
		x+charCreateCardX, y+charCreateDoramY, state.Job == charsprite.JobSummoner, charsprite.JobSummoner)
}

// drawRaceCard draws one card and reports a click by switching to its job.
func (b *UI2DBackend) drawRaceCard(state CharCreateUIState, id, label string, x, y float32, selected bool, job int) {
	r := b.ctx.Renderer()

	face, edge := charCreateCardFace, charCreateCardEdge
	if selected {
		face, edge = charCreateCardFaceSel, charCreateCardEdgeSel
	}

	r.DrawRect(x, y, charCreateCardW, charCreateCardH, face)
	r.DrawRectOutline(x, y, charCreateCardW, charCreateCardH, 2, edge)

	tw, th := r.MeasureText(label, loginTextScale)
	r.DrawText(x+(charCreateCardW-tw)/2, y+(charCreateCardH-th)/2, label,
		loginTextScale, charCreateCardInk)

	if b.ctx.InvisibleButtonAt(id, x, y, charCreateCardW, charCreateCardH) &&
		state.OnSetJob != nil {
		state.OnSetJob(job)
	}
}

// drawCharCreateSex draws the male/female toggle.
//
// At our packet version sex is the client's to choose and travels in
// CH_MAKE_CHAR, so this is a real control rather than a report of the
// account's own sex — which is only where it starts.
func (b *UI2DBackend) drawCharCreateSex(skin *charCreateSkin, state CharCreateUIState, x, y float32) {
	if skin.maleOff == nil || skin.maleOn == nil || skin.femaleOff == nil || skin.femaleOn == nil {
		return
	}

	r := b.ctx.Renderer()
	top := y + charCreateSexY
	maleX := x + charCreateSexMinX
	femaleX := maleX + charCreateSexW

	male, female := skin.maleOff, skin.femaleOff
	if state.Sex == SexMale {
		male = skin.maleOn
	} else {
		female = skin.femaleOn
	}

	r.DrawImage(male.ID, maleX, top, charCreateSexW, charCreateSexH, ui2d.ColorWhite)
	r.DrawImage(female.ID, femaleX, top, charCreateSexW, charCreateSexH, ui2d.ColorWhite)

	if b.ctx.InvisibleButtonAt("charcreate_male", maleX, top, charCreateSexW, charCreateSexH) &&
		state.OnSetSex != nil {
		state.OnSetSex(SexMale)
	}

	if b.ctx.InvisibleButtonAt("charcreate_female", femaleX, top, charCreateSexW, charCreateSexH) &&
		state.OnSetSex != nil {
		state.OnSetSex(SexFemale)
	}
}

// drawCharCreatePreview draws the character as it is being built, standing on
// the podium painted into the frame.
func (b *UI2DBackend) drawCharCreatePreview(state CharCreateUIState, x, y float32) {
	portrait := b.portraitForSpec(charsprite.Spec{
		Job:       state.Job,
		Female:    state.Sex == SexFemale,
		HairStyle: state.HairStyle,
	}, state.Facing)
	if portrait == nil {
		return
	}

	w, h := portrait.width, portrait.height

	if h > charCreatePreviewMaxH {
		w *= charCreatePreviewMaxH / h
		h = charCreatePreviewMaxH
	}
	if w > charCreatePreviewMaxW {
		h *= charCreatePreviewMaxW / w
		w = charCreatePreviewMaxW
	}

	b.ctx.Renderer().DrawImage(portrait.texture,
		x+charCreatePodiumX-w/2, y+charCreatePodiumY-h, w, h, ui2d.ColorWhite)
}

// drawCharCreateTurn draws the arrows that rotate the preview.
func (b *UI2DBackend) drawCharCreateTurn(skin *charCreateSkin, state CharCreateUIState, x, y float32) {
	if skin.turnLeft == nil || skin.turnRight == nil {
		return
	}

	r := b.ctx.Renderer()
	top := y + charCreateTurnY
	leftX := x + charCreateTurnL
	rightX := x + charCreateTurnR

	r.DrawImage(skin.turnLeft.ID, leftX, top, charCreateTurnW, charCreateTurnH, ui2d.ColorWhite)
	r.DrawImage(skin.turnRight.ID, rightX, top, charCreateTurnW, charCreateTurnH, ui2d.ColorWhite)

	if b.ctx.InvisibleButtonAt("charcreate_turn_l", leftX, top, charCreateTurnW, charCreateTurnH) &&
		state.OnTurn != nil {
		state.OnTurn(-1)
	}

	if b.ctx.InvisibleButtonAt("charcreate_turn_r", rightX, top, charCreateTurnW, charCreateTurnH) &&
		state.OnTurn != nil {
		state.OnTurn(1)
	}
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
