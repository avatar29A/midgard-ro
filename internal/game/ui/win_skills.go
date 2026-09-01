package ui

import (
	"strconv"

	"github.com/Faultbox/midgard-ro/internal/engine/ui2d"
	"github.com/Faultbox/midgard-ro/internal/game/skills"
	"github.com/Faultbox/midgard-ro/internal/network/packets"
)

// The Skill Tree window the Skill button opens.
//
// There is no single bitmap for this one the way statwin_bg.bmp is the status
// window's, so it is drawn: a scrolling list of two-line rows — icon, name,
// level, and either the SP it costs or the word Passive — over a footer that
// carries the skill points and the buttons.
const (
	skillsWindowID = "hud_win_skills"

	skillsW float32 = 280
	skillsH float32 = 250

	skillsPad     float32 = 6
	skillsRowH    float32 = 40
	skillsIcon    float32 = 28
	skillsScrollW float32 = 14

	// skillsIconGap is the air between the icon and the text beside it, and
	// skillsLineGap the air between a skill's name and its level. The two
	// lines are one block: set apart they read as separate rows.
	skillsIconGap float32 = 10
	skillsLineGap float32 = 2

	// skillsFooterH is the strip along the bottom, with the points on the
	// left and the buttons on the right.
	skillsFooterH float32 = 26
	skillsBtnW    float32 = 46
	skillsBtnH    float32 = 18

	// skillsRightW is the column the SP cost and "Passive" are right-aligned
	// in, kept clear of the name beside it.
	skillsRightW float32 = 58

	// skillsRaiseW is the raise button, the same mark and size the status
	// window spends a point with. It sits on the name's line, in the column
	// the cost is right-aligned in below it.
	skillsRaiseW float32 = 11

	skillsTextScale float32 = 0.75
)

// skillIconPath is where a skill's icon lives, named for its technical name.
const skillIconPath = skinBasePath + `item\`

// drawSkillsWindow draws the Skill Tree window when its button has opened it.
func (b *UI2DBackend) drawSkillsWindow(state InGameUIState, screenW, screenH float32) {
	if !b.IsWindowOpen(WindowSkill) {
		return
	}

	openX := (screenW - skillsW) / 2
	openY := (screenH - skillsH) / 2

	// The frame must not paint the body: it does so as a solid, and solids
	// cover every image drawn this frame — including the skill icons. The
	// body is filled below in the image layer instead, before the icons, so
	// the two stack in the order they are drawn.
	opts := ui2d.DefaultWindowOptions()

	if !b.ctx.BeginWindowEx(skillsWindowID, openX, openY, skillsW, skillsH, "Skill Tree", opts) {
		if b.ctx.WindowClosed(skillsWindowID) {
			b.ToggleWindow(WindowSkill)
		}

		return
	}

	x, y := openX, openY
	if rect, ok := b.ctx.WindowRect(skillsWindowID); ok {
		x, y = rect.X, rect.Y
	}

	b.ctx.CaptureMouse(ui2d.Rect{X: x, Y: y, W: skillsW, H: skillsH})

	bodyY := y + ui2d.FrameTitleH
	b.ctx.Renderer().DrawRect(x, bodyY, skillsW, skillsH-ui2d.FrameTitleH, ui2d.ColorWindowBody)

	b.drawSkillRows(state, x, y)
	b.drawSkillFooter(state, x, y)
	b.ctx.EndWindow()

	b.finishSkillDrag()
}

// drawSkillRows lists the skills, scrolled to wherever the bar is.
func (b *UI2DBackend) drawSkillRows(state InGameUIState, x, y float32) {
	r := b.ctx.Renderer()

	listX := x + skillsPad
	listY := y + ui2d.FrameTitleH + skillsPad
	listH := skillsH - ui2d.FrameTitleH - skillsFooterH - 2*skillsPad

	if len(state.Skills) == 0 {
		// Said rather than left blank: an empty window looks like one that
		// failed, and a character with no skills is a real state.
		r.DrawText(listX, listY, "No skills.", skillsTextScale, skillsEmptyText)

		return
	}

	visible := int(listH / skillsRowH)
	if visible < 1 {
		visible = 1
	}

	maxOffset := max(0, len(state.Skills)-visible)

	offset := min(b.skillScroll, maxOffset)
	listW := skillsW - 2*skillsPad

	if maxOffset > 0 {
		b.skillScroll = b.scrollbar("hud_skills", x+skillsW-skillsPad-skillsScrollW, listY,
			listH, offset, maxOffset, visible)
		offset = b.skillScroll
		listW -= skillsScrollW
	}

	for i := 0; i < visible && offset+i < len(state.Skills); i++ {
		skill := state.Skills[offset+i]
		rowY := listY + float32(i)*skillsRowH

		b.drawSkillRow(skill, listX, rowY, listW, state.SkillPoints > 0)
		b.beginSkillDrag(skill, ui2d.Rect{X: listX, Y: rowY, W: listW, H: skillsRowH})
	}
}

// drawSkillRow draws one skill: its icon, then its name over its level, with
// what it costs against the right.
//
// affordable is whether there is a point left to spend, which decides — with
// the server's own Raisable — whether the row offers to spend one.
func (b *UI2DBackend) drawSkillRow(skill packets.Skill, x, y, w float32, affordable bool) {
	r := b.ctx.Renderer()

	b.drawSkillIcon(skill.ID, x, y+(skillsRowH-skillsIcon)/2)

	textX := x + skillsIcon + skillsIconGap
	textW := w - skillsIcon - skillsIconGap - skillsRightW

	// An id the table does not know is a skill newer than the table, so it
	// shows as its id rather than as a blank row.
	name := skills.Name(skill.ID)
	if name == "" {
		name = "Skill #" + strconv.Itoa(int(skill.ID))
	}

	level := "Lv : " + strconv.Itoa(skill.Level)

	// The name and the level are one block, centered in the row together.
	_, lineH := r.MeasureText(name, skillsTextScale)
	top := y + (skillsRowH-(2*lineH+skillsLineGap))/2

	r.DrawText(textX, top, fitTextEnd(r, name, skillsTextScale, textW), skillsTextScale, ui2d.ColorText)
	r.DrawText(textX, top+lineH+skillsLineGap, level, skillsTextScale, ui2d.ColorText)

	// A skill that targets nothing is passive and has no cost to show.
	cost := "Passive"
	if skill.Inf != 0 {
		cost = "Sp : " + strconv.Itoa(skill.SP)
	}

	// Against the right, on the level's line: that is where the original
	// lines them up, not against the name above it.
	costW, _ := r.MeasureText(cost, skillsTextScale)
	r.DrawText(x+w-costW, top+lineH+skillsLineGap, cost, skillsTextScale, ui2d.ColorText)

	// The raise button goes on the name's line, above the cost. Whether a
	// skill can be leveled at all is the server's answer rather than a rule
	// worked out here: prerequisites, job and ceiling all feed Raisable, and
	// none of them are knowable from the packet alone.
	if !affordable || !skill.Raisable {
		return
	}

	if b.drawSkillRaise(skill.ID, x+w-skillsRaiseW, top+(lineH-skillsRaiseW)/2) {
		b.skillAction = SkillAction{Skill: skill.ID}
	}
}

// beginSkillDrag starts dragging a skill out of its row, so it can be put on
// the quick panel.
//
// Only what can be cast is draggable. A passive skill on the bar would be a
// key that does nothing, and the original does not let you put one there.
func (b *UI2DBackend) beginSkillDrag(skill packets.Skill, row ui2d.Rect) {
	if skill.Inf == 0 || b.skillDrag.active || b.itemDrag.active {
		return
	}

	if in := b.ctx.Input(); in.MouseLeftPressed && row.Contains(in.MouseX, in.MouseY) {
		b.skillDrag = skillDrag{active: true, skill: skill.ID}
	}
}

// skillDrag is a skill being dragged out of the Skill window.
type skillDrag struct {
	active bool
	skill  uint16
}

// finishSkillDrag puts a dragged skill in the cell it was let go over.
//
// Released anywhere else it is simply dropped: a skill is not a thing that can
// be thrown away, and the row it came from is still there.
func (b *UI2DBackend) finishSkillDrag() {
	if !b.skillDrag.active || b.ctx.Input().MouseLeftDown {
		return
	}

	in := b.ctx.Input()
	dragged := b.skillDrag
	b.skillDrag = skillDrag{}

	if row, col, ok := b.hotkeyCellAt(in.MouseX, in.MouseY); ok {
		b.AssignHotkey(row, col, hotkeyCell{id: uint32(dragged.skill), skill: true})
	}
}

// drawSkillRaise draws the button that spends a skill point on one skill, and
// reports a press. The same mark as the status window's, for the same act.
func (b *UI2DBackend) drawSkillRaise(id uint16, x, y float32) bool {
	tex, err := b.texCache.Load(statsArrow)
	if err != nil {
		return false
	}

	widget := "hud_skill_raise_" + strconv.Itoa(int(id))
	box := ui2d.Rect{X: x, Y: y, W: skillsRaiseW, H: skillsRaiseW}

	tint := ui2d.ColorWhite
	if box.Contains(b.ctx.Input().MouseX, b.ctx.Input().MouseY) {
		tint = statsArrowHot
	}

	b.ctx.Renderer().DrawImage(tex.ID, box.X, box.Y, box.W, box.H, tint)

	return b.ctx.InvisibleButtonAt(widget, box.X, box.Y, box.W, box.H)
}

// SkillAction is a skill point being spent on one skill.
type SkillAction struct {
	// Skill is the skill id. Zero is no skill: NV_BASIC is 1, so nothing real
	// collides with the empty value.
	Skill uint16
}

// TakeSkillAction returns a skill the player asked to raise and clears it.
func (b *UI2DBackend) TakeSkillAction() (SkillAction, bool) {
	action := b.skillAction
	if action.Skill == 0 {
		return SkillAction{}, false
	}

	b.skillAction = SkillAction{}

	return action, true
}

// drawSkillIcon draws a skill's icon, or the empty frame the original leaves
// when it has none.
func (b *UI2DBackend) drawSkillIcon(id uint16, x, y float32) {
	r := b.ctx.Renderer()

	var icon uint32
	if sprite := skills.Sprite(id); sprite != "" {
		if tex, err := b.texCache.Load(skillIconPath + sprite + ".bmp"); err == nil {
			icon = tex.ID
		}
	}

	// The cell is filled in the image layer, before the icon: the icons are
	// pale line art meant for a grey panel, and on a white body they all but
	// vanish. A DrawRect would not do — solid quads paint over every image.
	r.DrawRect(x, y, skillsIcon, skillsIcon, skillsIconBg)

	if icon != 0 {
		r.DrawImage(icon, x+1, y+1, skillsIcon-2, skillsIcon-2, ui2d.ColorWhite)
	}

	r.DrawRectOutline(x, y, skillsIcon, skillsIcon, 1, ui2d.ColorPanelBorder)
}

// drawSkillFooter draws the strip along the bottom: the points on the left
// and the buttons on the right.
func (b *UI2DBackend) drawSkillFooter(state InGameUIState, x, y float32) {
	r := b.ctx.Renderer()

	footerY := y + skillsH - skillsFooterH
	r.DrawRect(x+1, footerY, skillsW-2, 1, ui2d.ColorPanelBorder)

	points := "Skill Point : " + strconv.Itoa(state.SkillPoints)
	_, capH := r.MeasureText(points, skillsTextScale)
	r.DrawText(x+skillsPad, footerY+(skillsFooterH-capH)/2, points, skillsTextScale, ui2d.ColorText)

	// No "use" button beside close: a point is spent on the row it goes to,
	// which is where the original puts it too. A footer button would have to
	// ask which skill afterwards.
	btnY := footerY + (skillsFooterH-skillsBtnH)/2
	closeX := x + skillsW - skillsPad - skillsBtnW

	closeBox := ui2d.Rect{X: closeX, Y: btnY, W: skillsBtnW, H: skillsBtnH}
	b.drawFlatButton(closeBox, "close", false)

	if b.ctx.InvisibleButtonAt("hud_skills_close", closeBox.X, closeBox.Y, closeBox.W, closeBox.H) {
		b.ToggleWindow(WindowSkill)
	}
}

// fitTextEnd trims a name that does not fit its column, marking that it was
// cut rather than letting it run under what sits beside it.
func fitTextEnd(r *ui2d.Renderer, text string, scale, maxW float32) string {
	if w, _ := r.MeasureText(text, scale); w <= maxW {
		return text
	}

	runes := []rune(text)
	for i := len(runes); i > 0; i-- {
		candidate := string(runes[:i]) + "…"
		if w, _ := r.MeasureText(candidate, scale); w <= maxW {
			return candidate
		}
	}

	return ""
}

var (
	// skillsEmptyText is the note shown when there is nothing to list, dimmer
	// than a real row so it does not read as one.
	skillsEmptyText = ui2d.Color{R: 0.45, G: 0.45, B: 0.48, A: 1}

	// skillsIconBg is the sunken square an icon sits in.
	skillsIconBg = ui2d.Color{R: 0.85, G: 0.85, B: 0.87, A: 1}
)
