package ui

import (
	"fmt"
	"strings"

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

	// The reduced form is the same bitmap clipped to its top 53 rows, which
	// ends exactly above the HP trough. There is a `basewin_mini.bmp` in the
	// archive but it is 280x34 — a different, smaller window, not this one.
	hudReducedH = float32(53)

	// Where the panel starts. The original keeps it in the top-left corner;
	// after that it is wherever it was dragged.
	hudMarginX = float32(5)
	hudMarginY = float32(5)

	// The system button in the title bar, which folds the panel away.
	hudSysBtn      = float32(11)
	hudSysBtnRight = float32(2)
	hudSysBtnY     = float32(3)

	// The reduced form's three lines. The first sits in the title bar, where
	// the caption is in the large form — folded up, the panel shows the
	// character's name rather than the window's.
	hudSmallLine1X = float32(18)
	hudSmallLine1Y = float32(2)
	hudSmallLine2X = float32(10)
	hudSmallLine2Y = float32(20)
	hudSmallLine3X = float32(10)
	hudSmallLine3Y = float32(36)

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

	// The experience bars are not artwork: roBrowser draws them as a 1px
	// bordered box with a solid fill, and so does the original. 110x4 of
	// content inside that border.
	hudExpX      = float32(84)
	hudBaseExpY  = float32(89)
	hudJobExpY   = float32(101)
	hudExpW      = float32(110)
	hudExpH      = float32(4)
	hudExpBorder = float32(1)

	// Weight and Zeny share one right-aligned line on the striped footer.
	hudExtraY     = float32(119)
	hudExtraRight = float32(215)
	hudExtraGap   = float32(6)

	// The menu buttons hang below the panel, four to a row.
	hudBtnW    = float32(54)
	hudBtnH    = float32(18)
	hudBtnCols = 4

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

// The experience bar's colors, taken from roBrowser's stylesheet, which took
// them from the original: a grey rule, a white well and a solid blue fill.
var (
	hudExpBorderColor = ui2d.Color{R: 0.686, G: 0.686, B: 0.686, A: 1}
	hudExpWellColor   = ui2d.Color{R: 1, G: 1, B: 1, A: 1}
	hudExpFillColor   = ui2d.Color{R: 0.259, G: 0.384, B: 0.647, A: 1}

	// The original turns the weight red once the character is carrying half
	// of what they can, which is where the penalties start.
	hudWeightWarnColor = ui2d.Color{R: 0.8, G: 0.1, B: 0.1, A: 1}
)

// hudButtonOptions is how every button on this panel behaves. They are
// silent: folding the panel and opening other windows is arranging the
// interface, not doing something in the game, and the menu buttons do not
// open anything yet — a click that makes a noise and changes nothing reads
// as a fault.
var hudButtonOptions = ui2d.ButtonOptions{Silent: true}

// hudMenuButtons are the buttons under the panel, in the order the original
// lays them out. Each has three bitmaps: `<name>1` normal, `2` hovered and
// `3` pressed.
var hudMenuButtons = []string{
	"info", "skill", "item", "map",
	"party", "guild", "quest", "option",
	"booking", "recruit",
}

// gaugeSkin is one gauge's three-slice fill.
type gaugeSkin struct {
	left, mid, right *TextureInfo
}

// menuButtonSkin is one menu button's three states.
type menuButtonSkin struct {
	name                   string
	normal, hover, pressed *TextureInfo
}

// basicInfoSkin holds the panel art.
type basicInfoSkin struct {
	panel  *TextureInfo
	hp, sp *gaugeSkin
	menu   []*menuButtonSkin

	sysOff, sysOn *TextureInfo
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

	skin := &basicInfoSkin{panel: panel, hp: hp, sp: sp, menu: b.loadMenuButtons()}

	// The fold control. Missing art costs the button, not the panel — Ctrl+V
	// does the same thing.
	if off, err := b.texCache.Load(basicInterfacePath + "sys_mini_off.bmp"); err == nil {
		skin.sysOff = off
	}

	if on, err := b.texCache.Load(basicInterfacePath + "sys_mini_on.bmp"); err == nil {
		skin.sysOn = on
	}

	b.hudSkin = skin

	return b.hudSkin
}

// loadMenuButtons loads the button strip. A button whose art is missing is
// left out rather than blanking the row.
func (b *UI2DBackend) loadMenuButtons() []*menuButtonSkin {
	buttons := make([]*menuButtonSkin, 0, len(hudMenuButtons))

	for _, name := range hudMenuButtons {
		states := make([]*TextureInfo, 0, 3)

		for state := 1; state <= 3; state++ {
			path := fmt.Sprintf("%s%s%d.bmp", basicInterfacePath, name, state)

			tex, err := b.texCache.Load(path)
			if err != nil {
				logger.Warn("menu button art unavailable, leaving it out",
					zap.String("path", path), zap.Error(err))

				break
			}

			states = append(states, tex)
		}

		if len(states) != 3 {
			continue
		}

		buttons = append(buttons, &menuButtonSkin{
			name:    name,
			normal:  states[0],
			hover:   states[1],
			pressed: states[2],
		})
	}

	return buttons
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

	if state.ToggleBasicInfo {
		b.hudReduced = !b.hudReduced
	}

	if !b.hudPlaced {
		b.hudX, b.hudY = hudMarginX, hudMarginY
		b.hudPlaced = true
	}

	height := hudPanelH
	if b.hudReduced {
		height = hudReducedH
	}

	titleBar := ui2d.Rect{X: b.hudX, Y: b.hudY, W: hudPanelW, H: ui2d.FrameTitleH}

	// Double-clicking the title bar folds the panel, as it does in most
	// windowed interfaces. Checked before the drag so a double click that
	// wanders a pixel still counts.
	if b.ctx.DoubleClickedIn("basicinfo_titlebar", titleBar) {
		b.hudReduced = !b.hudReduced
	}

	// Dragged by its title bar, like every other window.
	b.ctx.DragHandle("basicinfo_titlebar", titleBar, &b.hudX, &b.hudY)

	x, y := b.hudX, b.hudY
	r := b.ctx.Renderer()

	// The reduced form is the same bitmap with its lower part cut away.
	r.DrawImageUV(skin.panel.ID, x, y, hudPanelW, height, 0, 0, 1, height/hudPanelH, ui2d.ColorWhite)

	if b.hudReduced {
		b.drawBasicInfoReduced(state, x, y)
	} else {
		b.drawBasicInfoLarge(skin, state, x, y)
	}

	b.drawFoldButton(skin, x, y)
	b.drawMenuButtons(skin, x, y+height)

	// The panel and its buttons swallow clicks. Without this a click on the
	// interface also reaches the world behind it, and the character walks
	// off while you are dragging the panel around.
	rows := float32((len(skin.menu) + hudBtnCols - 1) / hudBtnCols)
	b.ctx.CaptureMouse(ui2d.Rect{X: x, Y: y, W: hudPanelW, H: height})
	b.ctx.CaptureMouse(ui2d.Rect{
		X: x, Y: y + height,
		W: float32(hudBtnCols) * hudBtnW,
		H: rows * hudBtnH,
	})
}

// drawBasicInfoLarge draws the full panel.
func (b *UI2DBackend) drawBasicInfoLarge(skin *basicInfoSkin, state InGameUIState, x, y float32) {
	r := b.ctx.Renderer()

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

	b.drawExpBar(x+hudExpX, y+hudBaseExpY, state.PlayerBaseExp, state.PlayerNextBaseExp)
	b.drawExpBar(x+hudExpX, y+hudJobExpY, state.PlayerJobExp, state.PlayerNextJobExp)

	b.drawWeightAndZeny(state, x, y)
}

// drawBasicInfoReduced draws the folded panel: the name in the title bar, then
// levels and gauges as two lines of text.
func (b *UI2DBackend) drawBasicInfoReduced(state InGameUIState, x, y float32) {
	r := b.ctx.Renderer()

	r.DrawText(x+hudSmallLine1X, y+hudSmallLine1Y-hudTextLift,
		state.PlayerName, hudTextScale, ui2d.ColorText)

	r.DrawText(x+hudSmallLine2X, y+hudSmallLine2Y-hudGaugeTextLift,
		fmt.Sprintf("Lv.%d / %s / Lv.%d / Exp. %d",
			state.PlayerLevel, getJobName(uint16(state.PlayerClass)),
			state.PlayerJobLevel, state.PlayerBaseExp),
		hudGaugeTextScale, ui2d.ColorText)

	r.DrawText(x+hudSmallLine3X, y+hudSmallLine3Y-hudGaugeTextLift,
		fmt.Sprintf("HP. %d / %d | SP. %d / %d",
			state.PlayerHP, state.PlayerMaxHP, state.PlayerSP, state.PlayerMaxSP),
		hudGaugeTextScale, ui2d.ColorText)
}

// drawFoldButton draws the title bar's system button, which folds the panel
// down to its reduced form and back. Ctrl+V does the same.
func (b *UI2DBackend) drawFoldButton(skin *basicInfoSkin, x, y float32) {
	if skin.sysOff == nil || skin.sysOn == nil {
		return
	}

	if b.ctx.ImageButtonAtOpts("basicinfo_fold",
		x+hudPanelW-hudSysBtnRight-hudSysBtn, y+hudSysBtnY, hudSysBtn, hudSysBtn,
		skin.sysOff.ID, skin.sysOn.ID, skin.sysOn.ID, hudButtonOptions) {
		b.hudReduced = !b.hudReduced
	}
}

// drawExpBar draws one experience bar: a grey rule around a white well, with
// a solid fill for the progress made toward the next level.
//
// A next-level total of zero means the server has not sent one — at max level
// it never will — and draws an empty well rather than a full bar.
func (b *UI2DBackend) drawExpBar(x, y float32, current, next int64) {
	// Filled in the image pass, not with DrawRect.
	//
	// These are the only painted rectangles on the panel — the gauges above
	// them are skin textures — and as solids they floated over every window
	// on screen and over the ESC menu's backdrop, because solid quads paint
	// over every image in the frame by design. In the image pass they are
	// ordered by call, and the windows are drawn after the panel.
	fill := b.ctx.Renderer().DrawRect

	fill(x, y, hudExpW+2*hudExpBorder, hudExpH+2*hudExpBorder, hudExpBorderColor)
	fill(x+hudExpBorder, y+hudExpBorder, hudExpW, hudExpH, hudExpWellColor)

	if filled := expFillWidth(current, next, hudExpW); filled > 0 {
		fill(x+hudExpBorder, y+hudExpBorder, filled, hudExpH, hudExpFillColor)
	}
}

// expFillWidth is how much of an experience bar is filled. Unlike the health
// gauges this keeps its fractional width: the bar is 110px for a whole level,
// so rounding down would show nothing at all for the first percent.
func expFillWidth(current, next int64, track float32) float32 {
	if next <= 0 || current <= 0 || track <= 0 {
		return 0
	}

	if current >= next {
		return track
	}

	return track * float32(current) / float32(next)
}

// drawWeightAndZeny draws the footer line. It is right-aligned as one run, so
// the two halves are measured before either is drawn.
func (b *UI2DBackend) drawWeightAndZeny(state InGameUIState, x, y float32) {
	r := b.ctx.Renderer()

	weight := fmt.Sprintf("Weight : %d / %d", state.PlayerWeight, state.PlayerMaxWeight)
	zeny := "Zeny : " + withThousands(state.PlayerZeny)

	weightW, _ := r.MeasureText(weight, hudGaugeTextScale)
	zenyW, _ := r.MeasureText(zeny, hudGaugeTextScale)

	left := x + hudExtraRight - (weightW + hudExtraGap + zenyW)
	textY := y + hudExtraY - hudGaugeTextLift

	// The original turns the weight red at half load, where the penalties
	// begin, so the color is the warning rather than decoration.
	weightColor := ui2d.ColorText
	if state.PlayerMaxWeight > 0 && state.PlayerWeight*2 >= state.PlayerMaxWeight {
		weightColor = hudWeightWarnColor
	}

	r.DrawText(left, textY, weight, hudGaugeTextScale, weightColor)
	r.DrawText(left+weightW+hudExtraGap, textY, zeny, hudGaugeTextScale, ui2d.ColorText)
}

// withThousands groups a number with commas, the way the original prints Zeny.
func withThousands(v int64) string {
	digits := fmt.Sprintf("%d", v)

	sign := ""
	if strings.HasPrefix(digits, "-") {
		sign, digits = "-", digits[1:]
	}

	var out strings.Builder
	for i, d := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			out.WriteByte(',')
		}

		out.WriteRune(d)
	}

	return sign + out.String()
}

// drawMenuButtons draws the strip below the panel, four to a row.
//
// None of them opens anything yet — the windows they belong to are their own
// features — but they are drawn with their real hover and pressed art, which
// is what the archive ships and what tells you the button is alive.
func (b *UI2DBackend) drawMenuButtons(skin *basicInfoSkin, x, y float32) {
	for i, button := range skin.menu {
		col := i % hudBtnCols
		row := i / hudBtnCols

		window, opens := opensWindow(button.name)

		// A button whose window is open stays drawn in its pressed art, so the
		// strip shows what is on screen. The other two states still follow the
		// pointer, which is why this swaps the normal texture rather than
		// forcing all three.
		normal := button.normal.ID
		if opens && b.IsWindowOpen(window) {
			normal = button.pressed.ID
		}

		// Buttons that open something are worth a click sound; the rest still
		// do nothing, and announcing that reads as a fault.
		opts := hudButtonOptions
		if opens {
			opts.Silent = false
		}

		clicked := b.ctx.ImageButtonAtOpts("hud_menu_"+button.name,
			x+float32(col)*hudBtnW, y+float32(row)*hudBtnH,
			hudBtnW, hudBtnH,
			normal, button.hover.ID, button.pressed.ID, opts)

		if clicked && opens {
			b.ToggleWindow(window)
		}
	}
}
