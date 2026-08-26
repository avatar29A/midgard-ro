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

	// The gauges. The troughs are painted into the panel — only the fill is
	// drawn, inside the same rectangle: rows 53-61 and 68-76, columns 35-169,
	// measured off the bitmap.
	hudGaugeX   = float32(35)
	hudGaugeW   = float32(135)
	hudGaugeH   = float32(8)
	hudHPGaugeY = float32(53)
	hudSPGaugeY = float32(68)

	// The fill is three-sliced horizontally: 4px caps that keep their shape
	// and a 1px middle stretched to whatever is left.
	hudGaugeCapW = float32(4)

	// `HP` and `SP` sit left of their gauge; the percentage is right-aligned
	// against the info block's right edge.
	hudGaugeLabelX  = float32(15)
	hudPercentRight = float32(215)

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

	// The gauge row is set smaller than the rest of the panel. The bar is 8px
	// tall and the reading sits on it, so at the body's size the glyphs stand
	// proud of the bar top and bottom.
	hudGaugeTextScale = float32(0.6)
	hudGaugeTextLift  = hudTextLift * hudGaugeTextScale / hudTextScale
)

// hudPanelFile is the panel background in the archive.
const hudPanelFile = "basewin_bg2.bmp"

// gaugeSkin is one gauge's three-slice fill.
type gaugeSkin struct {
	left, mid, right *TextureInfo
}

// basicInfoSkin holds the panel art.
type basicInfoSkin struct {
	panel  *TextureInfo
	hp, sp *gaugeSkin
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

	hp := b.loadGaugeSkin("gzered")
	sp := b.loadGaugeSkin("gzeblue")

	if hp == nil || sp == nil {
		logger.Warn("gauge art unavailable, the panel will draw without its bars")
	}

	b.hudSkin = &basicInfoSkin{panel: panel, hp: hp, sp: sp}

	return b.hudSkin
}

// loadGaugeSkin loads one gauge's three pieces, named `<prefix>_left.bmp` and
// so on. A miss returns nil, which costs the bar and not the panel.
func (b *UI2DBackend) loadGaugeSkin(prefix string) *gaugeSkin {
	pieces := make([]*TextureInfo, 0, 3)

	for _, part := range []string{"left", "mid", "right"} {
		path := fmt.Sprintf("%s%s_%s.bmp", basicInterfacePath, prefix, part)

		tex, err := b.texCache.Load(path)
		if err != nil {
			logger.Warn("gauge art unavailable",
				zap.String("path", path), zap.Error(err))

			return nil
		}

		pieces = append(pieces, tex)
	}

	return &gaugeSkin{left: pieces[0], mid: pieces[1], right: pieces[2]}
}

// gaugeFillWidth is how much of a track a value fills.
//
// A maximum of zero means the server has not told us yet, and draws empty
// rather than dividing by zero. The width is whole pixels — the art is
// nearest-filtered, so a fractional edge shimmers as the value changes — but
// anything alive keeps at least one pixel, so a character on their last hit
// point does not read as a corpse.
func gaugeFillWidth(current, maximum int, track float32) float32 {
	if maximum <= 0 || current <= 0 || track <= 0 {
		return 0
	}

	if current >= maximum {
		return track
	}

	width := float32(int(track * float32(current) / float32(maximum)))
	if width < 1 {
		width = 1
	}

	return width
}

// drawGauge fills one bar. The trough underneath is part of the panel art, so
// an empty gauge draws nothing at all.
func (b *UI2DBackend) drawGauge(skin *gaugeSkin, x, y, fill float32) {
	if skin == nil || fill <= 0 {
		return
	}

	r := b.ctx.Renderer()

	// Too narrow for both caps: stretch the middle across the whole of it,
	// rather than drawing caps that would overlap and darken the tip.
	if fill < 2*hudGaugeCapW {
		r.DrawImage(skin.mid.ID, x, y, fill, hudGaugeH, ui2d.ColorWhite)
		return
	}

	r.DrawImage(skin.left.ID, x, y, hudGaugeCapW, hudGaugeH, ui2d.ColorWhite)
	r.DrawImage(skin.mid.ID, x+hudGaugeCapW, y, fill-2*hudGaugeCapW, hudGaugeH, ui2d.ColorWhite)
	r.DrawImage(skin.right.ID, x+fill-hudGaugeCapW, y, hudGaugeCapW, hudGaugeH, ui2d.ColorWhite)
}

// drawGaugeRow draws one gauge and its three pieces of text: the label to the
// left, the reading on the bar, and the percentage to the right.
func (b *UI2DBackend) drawGaugeRow(skin *gaugeSkin, label string, current, maximum int, x, y float32) {
	b.drawGauge(skin, x+hudGaugeX, y, gaugeFillWidth(current, maximum, hudGaugeW))

	r := b.ctx.Renderer()
	textY := y - hudGaugeTextLift

	r.DrawText(x+hudGaugeLabelX, textY, label, hudGaugeTextScale, ui2d.ColorText)

	reading := fmt.Sprintf("%d / %d", current, maximum)
	readingW, _ := r.MeasureText(reading, hudGaugeTextScale)
	r.DrawText(x+hudGaugeX+(hudGaugeW-readingW)/2, textY, reading, hudGaugeTextScale, ui2d.ColorText)

	percent := fmt.Sprintf("%d%%", percentOf(current, maximum))
	percentW, _ := r.MeasureText(percent, hudGaugeTextScale)
	r.DrawText(x+hudPercentRight-percentW, textY, percent, hudGaugeTextScale, ui2d.ColorText)
}

// percentOf is the reading the original prints beside each bar. It rounds
// down, so 99% means not quite full rather than nearly so.
func percentOf(current, maximum int) int {
	if maximum <= 0 || current <= 0 {
		return 0
	}

	if current >= maximum {
		return 100
	}

	return current * 100 / maximum
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

	b.drawGaugeRow(skin.hp, "HP", state.PlayerHP, state.PlayerMaxHP, x, y+hudHPGaugeY)
	b.drawGaugeRow(skin.sp, "SP", state.PlayerSP, state.PlayerMaxSP, x, y+hudSPGaugeY)

	r.DrawText(x+hudBaseLvX, y+hudBaseLvY-hudTextLift,
		fmt.Sprintf("Base Lv. %d", state.PlayerLevel), hudTextScale, ui2d.ColorText)
	r.DrawText(x+hudJobLvX, y+hudJobLvY-hudTextLift,
		fmt.Sprintf("Job Lv. %d", state.PlayerJobLevel), hudTextScale, ui2d.ColorText)
}
