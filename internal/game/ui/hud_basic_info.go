package ui

import (
	"fmt"

	"go.uber.org/zap"

	"github.com/Faultbox/midgard-ro/internal/engine/ui2d"
	"github.com/Faultbox/midgard-ro/internal/logger"
)

// The Basic Info panel is one 220x135 bitmap with the chrome painted in: the
// title bar, the two gauge troughs, the grey block the levels sit on and the
// striped footer. Only text and the gauge fills are drawn over it.
//
// The offsets below come from roBrowser's BasicInfo.css, checked against the
// bitmap itself — its trough runs x35..169 at y53, which is roBrowser's
// (35, 53) 135x8 to the pixel, so the rest of that stylesheet is trustworthy.
const (
	hudPanelW = float32(220)
	hudPanelH = float32(135)

	// Where the panel sits. The original keeps it in the top-left corner.
	hudMarginX = float32(5)
	hudMarginY = float32(5)

	// Title bar: 16 rows of gradient, then the black rule at y16. The caption
	// is centered in that band rather than sat on its top edge.
	hudTitleX = float32(18)
	hudTitleY = float32(4)

	// The white body. Nothing is painted here, so both lines are ours.
	hudNameX = float32(10)
	hudNameY = float32(20)
	hudJobX  = float32(10)
	hudJobY  = float32(33)

	// The grey block, which the bitmap runs from y85 to y109.
	hudBaseLvX = float32(15)
	hudBaseLvY = float32(86)
	hudJobLvX  = float32(15)
	hudJobLvY  = float32(97)

	// The panel is small, so it takes the same reduced text as the login and
	// character select windows rather than the default UI size.
	hudTextScale = loginTextScale

	// DrawText places the top of the line box, which sits this far above the
	// glyph caps at this scale. The offsets above are cap positions, so every
	// one of them is lifted by this much. Same measurement as character
	// select, where it was taken against the artwork's own painted labels.
	hudTextLift = charSelTextLift
)

// hudPanelFile is the panel background in the archive.
const hudPanelFile = "basewin_bg2.bmp"

// basicInfoSkin holds the panel art.
type basicInfoSkin struct {
	panel *TextureInfo
}

// loadBasicInfoSkin loads the panel background, once. A miss leaves the skin
// nil and the HUD is skipped: a missing bitmap should cost the panel, not the
// game.
func (b *UI2DBackend) loadBasicInfoSkin() *basicInfoSkin {
	if b.hudSkin != nil {
		return b.hudSkin
	}

	if b.hudTried {
		return nil
	}

	b.hudTried = true

	panel, err := b.texCache.Load(basicInterfacePath + hudPanelFile)
	if err != nil {
		logger.Warn("basic info panel art unavailable, skipping the HUD",
			zap.String("path", basicInterfacePath+hudPanelFile), zap.Error(err))

		return nil
	}

	b.hudSkin = &basicInfoSkin{panel: panel}

	return b.hudSkin
}

// renderBasicInfo draws the Basic Info panel.
func (b *UI2DBackend) renderBasicInfo(state InGameUIState) {
	skin := b.loadBasicInfoSkin()
	if skin == nil {
		return
	}

	x, y := hudMarginX, hudMarginY
	r := b.ctx.Renderer()

	r.DrawImage(skin.panel.ID, x, y, hudPanelW, hudPanelH, ui2d.ColorWhite)

	// The title bar is bare gradient — the original's caption is not painted
	// into it, so it is drawn like everything else here.
	r.DrawText(x+hudTitleX, y+hudTitleY-hudTextLift, "Basic Info", hudTextScale, ui2d.ColorText)

	r.DrawText(x+hudNameX, y+hudNameY-hudTextLift, state.PlayerName, hudTextScale, ui2d.ColorText)
	r.DrawText(x+hudJobX, y+hudJobY-hudTextLift, getJobName(uint16(state.PlayerClass)),
		hudTextScale, ui2d.ColorText)

	r.DrawText(x+hudBaseLvX, y+hudBaseLvY-hudTextLift,
		fmt.Sprintf("Base Lv. %d", state.PlayerLevel), hudTextScale, ui2d.ColorText)
	r.DrawText(x+hudJobLvX, y+hudJobLvY-hudTextLift,
		fmt.Sprintf("Job Lv. %d", state.PlayerJobLevel), hudTextScale, ui2d.ColorText)
}
