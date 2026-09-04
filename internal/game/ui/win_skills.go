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

	// skillsLevelArrowW is the pair of marks that pick which level a skill
	// goes off at, on the level's own line where the original puts them.
	skillsLevelArrowW float32 = 10
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
		row := ui2d.Rect{X: listX, Y: rowY, W: listW, H: skillsRowH}

		if b.skillChosen == skill.ID {
			r.DrawRect(row.X, row.Y, row.W, row.H, skillsChosenRow)
		}

		// A press on one of the level marks is a press on the mark, not on
		// the row: without this it also picked the skill up, and the row came
		// away on the pointer every time the level was changed.
		overArrow := b.drawSkillRow(skill, listX, rowY, listW, state.SkillPoints > 0)

		b.skillRowInput(skill, row, overArrow)

		if !overArrow {
			b.beginSkillDrag(skill, row)
		}
	}

	b.scrollSkills(state, listX, listY, listW, listH, visible)
}

// drawSkillRow draws one skill: its icon, then its name over its level, with
// what it costs against the right.
//
// affordable is whether there is a point left to spend, which decides — with
// the server's own Raisable — whether the row offers to spend one.
func (b *UI2DBackend) drawSkillRow(skill packets.Skill, x, y, w float32, affordable bool) (overArrow bool) {
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

	// The name and the level are one block, centered in the row together.
	_, lineH := r.MeasureText(name, skillsTextScale)
	top := y + (skillsRowH-(2*lineH+skillsLineGap))/2

	r.DrawText(textX, top, fitTextEnd(r, name, skillsTextScale, textW), skillsTextScale, ui2d.ColorText)
	overArrow = b.drawSkillLevel(skill, textX, top+lineH+skillsLineGap, lineH)

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
		return overArrow
	}

	if b.drawSkillRaise(skill.ID, x+w-skillsRaiseW, top+(lineH-skillsRaiseW)/2) {
		b.skillAction = SkillAction{Skill: skill.ID}
	}

	return overArrow
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
		b.skillDrag = skillDrag{active: true, skill: skill.ID, level: b.skillCastLevel(skill)}
	}
}

// skillDrag is a skill being dragged out of the Skill window.
type skillDrag struct {
	active bool
	skill  uint16

	// level is what the row was set to when it was picked up. The cell keeps
	// it, which is how the same skill sits on the bar twice at two levels.
	level int
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
		b.AssignHotkey(row, col, hotkeyCell{
			id: uint32(dragged.skill), skill: true, level: dragged.level,
		})
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

	// Use acts on the row that was last pressed, which is what the original's
	// does. A point is still spent on the row it goes to rather than from
	// here: raising is per skill and always was, and a footer button for it
	// would have to ask which skill afterwards.
	btnY := footerY + (skillsFooterH-skillsBtnH)/2
	closeX := x + skillsW - skillsPad - skillsBtnW
	useX := closeX - skillsPad - skillsBtnW

	chosen, canUse := b.chosenSkill(state)

	useBox := ui2d.Rect{X: useX, Y: btnY, W: skillsBtnW, H: skillsBtnH}
	b.drawFlatButton(useBox, "use", !canUse)

	if canUse && b.ctx.InvisibleButtonAt("hud_skills_use", useBox.X, useBox.Y, useBox.W, useBox.H) {
		b.castSkillFromWindow(chosen)
	}

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

	// skillsChosenRow is the wash behind the row the use button acts on.
	skillsChosenRow = ui2d.Color{R: 0.78, G: 0.84, B: 0.96, A: 1}

	// skillsArrowOff is a level mark that would go past either end. Faded
	// rather than hidden, so the pair stays put and the row does not shuffle
	// as the level changes.
	skillsArrowOff = ui2d.Color{R: 0.66, G: 0.66, B: 0.70, A: 1}

	// skillsArrowHot is a mark under the pointer. Darker than the text rather
	// than brighter: the status window's marks are images on a dark panel and
	// a pale tint lifts them, but these are letters on a white body, where
	// pale is invisible — which is what they went when pressed.
	skillsArrowHot = ui2d.Color{R: 0.13, G: 0.32, B: 0.78, A: 1}

	// skillsIconBg is the sunken square an icon sits in.
	skillsIconBg = ui2d.Color{R: 0.85, G: 0.85, B: 0.87, A: 1}
)

// Choosing what level a skill goes off at.
//
// Most skills are learned to a level and cast at it, but they can be cast at
// any level up to it, and a lower one costs less. The original puts a pair of
// marks around the level in the row so it can be picked, and a skill dragged
// to the quick panel goes there at whatever was picked — which is how the same
// skill ends up on the bar several times, cheap on one key and full on
// another.
//
// The pick is the window's own state rather than the character's. Nothing on
// the wire carries it, and it means nothing to the server: what goes out is
// the level on the cast.

// skillCastLevel is the level a skill is set to go off at, held inside what
// the character has learned.
//
// Held rather than remembered exactly: a skill picked at five and then reset
// to three would otherwise ask for a level the server refuses.
func (b *UI2DBackend) skillCastLevel(skill packets.Skill) int {
	chosen := b.skillLevels[skill.ID]
	if chosen <= 0 || chosen > skill.Level {
		return skill.Level
	}

	return chosen
}

// drawSkillLevel draws the level, with the marks that change it when there is
// more than one to choose from.
//
// A passive skill and a skill known at level one both have nothing to pick, so
// they show the level plainly. Marks that do nothing are worse than none.
func (b *UI2DBackend) drawSkillLevel(skill packets.Skill, x, y, lineH float32) (overArrow bool) {
	r := b.ctx.Renderer()

	level := b.skillCastLevel(skill)

	if skill.Inf == 0 || skill.Level <= 1 {
		r.DrawText(x, y, "Lv : "+strconv.Itoa(skill.Level), skillsTextScale, ui2d.ColorText)

		return false
	}

	text := "Lv : " + strconv.Itoa(level) + " / " + strconv.Itoa(skill.Level)
	textW, _ := r.MeasureText(text, skillsTextScale)

	down := ui2d.Rect{X: x, Y: y, W: skillsLevelArrowW, H: lineH}
	if b.drawSkillLevelArrow(skill.ID, "down", "<", down, level > 1) {
		b.setSkillLevel(skill.ID, level-1)
	}

	textX := x + skillsLevelArrowW + skillsLineGap
	r.DrawText(textX, y, text, skillsTextScale, ui2d.ColorText)

	up := ui2d.Rect{X: textX + textW + skillsLineGap, Y: y, W: skillsLevelArrowW, H: lineH}
	if b.drawSkillLevelArrow(skill.ID, "up", ">", up, level < skill.Level) {
		b.setSkillLevel(skill.ID, level+1)
	}

	in := b.ctx.Input()

	return down.Contains(in.MouseX, in.MouseY) || up.Contains(in.MouseX, in.MouseY)
}

// setSkillLevel records what a skill is set to go off at.
func (b *UI2DBackend) setSkillLevel(id uint16, level int) {
	if b.skillLevels == nil {
		b.skillLevels = map[uint16]int{}
	}

	b.skillLevels[id] = level
}

// drawSkillLevelArrow draws one of the marks and reports a press on it.
//
// One that would go past either end is drawn faded and does nothing: the
// original greys them out rather than hiding them, so the pair stays where it
// was and the row does not shuffle as the level changes.
func (b *UI2DBackend) drawSkillLevelArrow(id uint16, which, mark string, box ui2d.Rect, live bool) bool {
	r := b.ctx.Renderer()

	color := skillsArrowOff
	if live {
		color = ui2d.ColorText
		if box.Contains(b.ctx.Input().MouseX, b.ctx.Input().MouseY) {
			color = skillsArrowHot
		}
	}

	markW, _ := r.MeasureText(mark, skillsTextScale)
	r.DrawText(box.X+(box.W-markW)/2, box.Y, mark, skillsTextScale, color)

	if !live {
		return false
	}

	widget := "hud_skill_lv_" + which + "_" + strconv.Itoa(int(id))

	return b.ctx.InvisibleButtonAt(widget, box.X, box.Y, box.W, box.H)
}

// skillRowInput picks a row out and casts what is on it.
//
// A press chooses the skill, which is what the use button acts on, and a
// double click casts it there and then — the same two ways the original lets
// a skill be used from this window.
func (b *UI2DBackend) skillRowInput(skill packets.Skill, row ui2d.Rect, overArrow bool) {
	in := b.ctx.Input()

	if overArrow {
		return
	}

	if in.MouseLeftPressed && row.Contains(in.MouseX, in.MouseY) {
		b.skillChosen = skill.ID
	}

	id := "hud_skill_row_" + strconv.Itoa(int(skill.ID))
	if b.ctx.DoubleClickedIn(id, row) {
		b.castSkillFromWindow(skill)
	}

	if !b.skillDrag.active && !b.itemDrag.active && row.Contains(in.MouseX, in.MouseY) {
		b.setSkillTooltip(skill, in.MouseX, in.MouseY)
	}
}

// castSkillFromWindow uses a skill at the level the row is set to.
//
// A passive skill is not cast at all: there is nothing to send, and the server
// would refuse it.
func (b *UI2DBackend) castSkillFromWindow(skill packets.Skill) {
	if skill.Inf == 0 {
		return
	}

	b.skillCast = SkillCast{Skill: skill.ID, Level: b.skillCastLevel(skill)}
}

// setSkillTooltip says what a row is.
//
// The packet carries five things — the id, the level, the SP, the range and
// the targeting bits — and a panel built from those alone tells a player that
// Stone Curse is active, costs SP and reaches two cells. What it is made of,
// how long it takes and what it spends come from the server's own database,
// which is what the rest of this reads.
//
// At the level the picker is on rather than the level learned: the figures are
// what would happen if the skill were cast now, and half of them change with
// the level chosen.
func (b *UI2DBackend) setSkillTooltip(skill packets.Skill, atX, atY float32) {
	name := skills.Name(skill.ID)
	if name == "" {
		name = "Skill #" + strconv.Itoa(int(skill.ID))
	}

	level := b.skillCastLevel(skill)

	lines := []string{"Lv " + strconv.Itoa(level) + " of " + strconv.Itoa(skill.Level)}

	info, known := skills.InfoOf(skill.ID)

	if skill.Inf == 0 {
		lines = append(lines, "Passive")
	} else {
		lines = append(lines,
			"Active — "+skillTargetWords(skill.Inf),
			"SP "+strconv.Itoa(skill.SP)+"   Range "+strconv.Itoa(skill.Range))
	}

	if known {
		lines = append(lines, skillInfoLines(info, level)...)
	}

	lines = append(lines, skillNeedLines(skill.ID)...)

	b.setTooltip(name, lines, atX, atY)
}

// skillInfoLines is what the database says, in the order a player reads it:
// what the skill is, then what it takes to use, then what it costs.
func skillInfoLines(info skills.Info, level int) []string {
	var lines []string

	// Magic, Fire. The element is named beside the kind rather than on a line
	// of its own — together they are one fact about the skill, and a panel is
	// read at a glance.
	if kind := skillKindWords(info, level); kind != "" {
		lines = append(lines, kind)
	}

	if hits, ok := skills.At(info.Hits, level); ok && hits > 1 {
		lines = append(lines, "Hits "+strconv.Itoa(hits))
	}

	if cast := skillCastWords(info, level); cast != "" {
		lines = append(lines, cast)
	}

	// The two waits. Named apart because they are: one is how long until any
	// skill may be used again and the other how long until this one may.
	if delay, ok := skills.At(info.DelayMs, level); ok && delay > 0 {
		lines = append(lines, "Delay "+seconds(delay))
	}
	if cool, ok := skills.At(info.CooldownMs, level); ok && cool > 0 {
		lines = append(lines, "Cooldown "+seconds(cool))
	}

	for _, cost := range info.Catalyst {
		amount := ""
		if cost.Amount > 1 {
			amount = " ×" + strconv.Itoa(cost.Amount)
		}

		lines = append(lines, "Needs "+cost.Item+amount)
	}

	return lines
}

// skillKindWords is what the skill is: Magic, Weapon or Misc, and the element
// it hits with where that is worth saying.
//
// Weapon element is not: it means the skill hits with whatever is being
// swung, which is not a fact about the skill at all.
func skillKindWords(info skills.Info, level int) string {
	kind := info.Kind
	if kind == "Misc" {
		// Which is rAthena's word for "neither of the other two", and says
		// nothing to a player.
		kind = ""
	}

	element := skills.ElementAt(info.Element, level)
	if element == "Weapon" || element == "Neutral" {
		element = ""
	}

	switch {
	case kind != "" && element != "":
		return kind + ", " + element
	case kind != "":
		return kind
	case element != "":
		return element
	}

	return ""
}

// skillCastWords is how long it takes to get off.
//
// The fixed part is named separately because nothing shortens it: a Wizard
// with every cast reduction there is still waits that long, and a panel that
// added the two together would promise a cast that can be reached and cannot.
func skillCastWords(info skills.Info, level int) string {
	cast, _ := skills.At(info.CastMs, level)
	fixed, _ := skills.At(info.FixedMs, level)

	switch {
	case cast > 0 && fixed > 0:
		return "Cast " + seconds(cast) + " + " + seconds(fixed) + " fixed"
	case cast > 0:
		return "Cast " + seconds(cast)
	case fixed > 0:
		return "Cast " + seconds(fixed) + " fixed"
	}

	return ""
}

// skillNeedLines is what a skill has to be learned after.
func skillNeedLines(id uint16) []string {
	var lines []string

	for _, need := range skills.Needs(id) {
		name := skills.Name(need.Skill)
		if name == "" {
			name = "Skill #" + strconv.Itoa(int(need.Skill))
		}

		lines = append(lines, "After "+name+" Lv "+strconv.Itoa(int(need.Level)))
	}

	return lines
}

// seconds writes a duration in milliseconds the way a player counts it.
func seconds(ms int) string {
	whole := ms / 1000
	tenths := (ms % 1000) / 100

	if tenths == 0 {
		return strconv.Itoa(whole) + "s"
	}

	return strconv.Itoa(whole) + "." + strconv.Itoa(tenths) + "s"
}

// skillTargetWords says what a skill is aimed at, from the server's own
// targeting bits.
func skillTargetWords(inf int) string {
	switch {
	case inf&packets.InfGround != 0:
		return "cast on a place"
	case inf&packets.InfSupport != 0, inf&packets.InfAttack != 0:
		return "cast on somebody"
	case inf&packets.InfSelf != 0:
		return "cast on yourself"
	}

	return "cast"
}

// chosenSkill is the row the use button acts on, and whether it can be used.
//
// The chosen id is looked up in what the server last sent rather than kept as
// a row: the list is rebuilt every frame and a skill learned in between would
// shift every row after it.
func (b *UI2DBackend) chosenSkill(state InGameUIState) (packets.Skill, bool) {
	if b.skillChosen == 0 {
		return packets.Skill{}, false
	}

	skill, known := findSkillIn(state.Skills, b.skillChosen)
	if !known {
		return packets.Skill{}, false
	}

	// A passive skill is not cast at all — there is nothing to send, and the
	// server would refuse it — so the button offers nothing for one.
	return skill, skill.Inf != 0
}

// scrollSkills moves the list under the wheel.
//
// Over the list rather than over the window: the wheel belongs to what is
// under it, and a window that scrolled while the pointer was on its footer
// would move under a button being aimed at.
func (b *UI2DBackend) scrollSkills(state InGameUIState, listX, listY, listW, listH float32, visible int) {
	in := b.ctx.Input()
	if in == nil || in.ScrollY == 0 {
		return
	}

	if !(ui2d.Rect{X: listX, Y: listY, W: listW, H: listH}).Contains(in.MouseX, in.MouseY) {
		return
	}

	maxOffset := max(0, len(state.Skills)-visible)
	b.skillScroll = min(max(b.skillScroll-int(in.ScrollY), 0), maxOffset)
}
