package states

import (
	"fmt"

	"go.uber.org/zap"

	"strings"

	"github.com/Faultbox/midgard-ro/internal/engine/picking"
	"github.com/Faultbox/midgard-ro/internal/game/entity"
	"github.com/Faultbox/midgard-ro/internal/game/skills"

	"github.com/Faultbox/midgard-ro/internal/logger"
	"github.com/Faultbox/midgard-ro/internal/network/packets"
	"github.com/Faultbox/midgard-ro/internal/trace"
	"github.com/Faultbox/midgard-ro/pkg/math"
)

// What the server says about a cast.
//
// Three packets carry a skill going off and they differ only in what it landed
// on: a unit and damage, a unit and none, or a cell. What is done with them is
// the same, so they meet in applySkillUse.

// handleSkillFail is a cast the server refused.
//
// Worth saying out loud: a skill that does nothing and explains nothing is
// indistinguishable from a client that dropped the click. The server has a
// reason for every refusal and this is where it gets read.
func (s *InGameState) handleSkillFail(data []byte) error {
	fail, ok := packets.DecodeSkillFail(data)
	if !ok {
		logger.Warn("short skill failure packet", zap.Int("len", len(data)))

		return nil
	}

	trace.Emit(trace.HUD, "skill-fail",
		zap.Uint16("skill", fail.SkillID), zap.Uint8("cause", fail.Cause))

	reason := fail.Reason()
	if reason == "" {
		// A cause nobody named. Saying its number is worth more than saying
		// the wrong sentence, and it names what to go and read.
		reason = "That skill failed."
	}

	name := skills.Name(fail.SkillID)
	if name == "" {
		name = "That skill"
	}

	s.chat.AddLocal(ChatError, name+": "+reason)

	return nil
}

// handleSkillCast is a cast beginning, which is the only warning anything gets
// that something is coming.
func (s *InGameState) handleSkillCast(data []byte) error {
	cast, ok := packets.DecodeSkillCast(data)
	if !ok {
		logger.Warn("short skill cast packet", zap.Int("len", len(data)))

		return nil
	}

	trace.Emit(trace.HUD, "skill-cast",
		zap.Uint32("from", cast.SourceID), zap.Uint32("to", cast.TargetID),
		zap.Uint16("skill", cast.SkillID), zap.Uint32("castMs", cast.CastMs))

	// The caster turns to face what it is aiming at, the same way a blow makes
	// both sides look at each other.
	if cast.TargetID != 0 {
		s.faceTowards(cast.SourceID, cast.TargetID)
	}

	if cast.SourceID == s.selfAID() {
		s.beginCast(cast.SkillID, float32(cast.CastMs))
	}

	s.beginCastAura(cast)

	// What a cast sounds like, and its name over the caster's head for as
	// long as it runs. Only for a cast that takes time: an instant skill's
	// sound and name are the skill's own, shown when it goes off.
	if cast.CastMs > 0 {
		if effects, known := skills.EffectsOf(cast.SkillID); known {
			s.playSkillSounds(effects.BeginCast)
		}

		s.addSkillLabel(cast.SourceID, cast.SkillID, float32(cast.CastMs))
	}

	// No motion yet. The sprite has cast poses and the client's own table
	// names one per skill — ACTOR_STATE, read two changes ago — but they are
	// ACT sets this client does not bake, so playing something else would be
	// inventing a pose rather than using the sprite's.

	return nil
}

// handleSkillUse is a skill that did no damage: a buff, a heal, a teleport.
func (s *InGameState) handleSkillUse(data []byte) error {
	use, ok := packets.DecodeSkillUse(data)
	if !ok {
		logger.Warn("short skill use packet", zap.Int("len", len(data)))

		return nil
	}

	s.applySkillUse(use)

	return nil
}

// handleSkillDamage is a skill that did.
func (s *InGameState) handleSkillDamage(data []byte) error {
	use, ok := packets.DecodeSkillDamage(data)
	if !ok {
		logger.Warn("short skill damage packet", zap.Int("len", len(data)))

		return nil
	}

	s.applySkillUse(use)

	return nil
}

// handleGroundSkill is a skill placed on a cell.
func (s *InGameState) handleGroundSkill(data []byte) error {
	use, ok := packets.DecodeGroundSkill(data)
	if !ok {
		logger.Warn("short ground skill packet", zap.Int("len", len(data)))

		return nil
	}

	s.applySkillUse(use)

	return nil
}

// applySkillUse draws a skill going off.
//
// The damage goes through the same queue an ordinary blow does, so a skill's
// figure and its target's flinch wait for the caster's animation exactly as a
// sword's do. A skill has no attack motion of its own on the wire, so the
// swing length is the reference one — which is what an instant skill looks
// like anyway.
func (s *InGameState) applySkillUse(use packets.SkillUse) {
	trace.Emit(trace.HUD, "skill-use",
		zap.Uint32("from", use.SourceID), zap.Uint32("to", use.TargetID),
		zap.Uint16("skill", use.SkillID), zap.Int("level", use.Level),
		zap.Int("amount", use.Amount),
		zap.Int("damage", use.Damage), zap.Bool("ground", use.Ground),
		zap.Int("cellX", use.CellX), zap.Int("cellY", use.CellY))

	if use.TargetID != 0 {
		s.faceTowards(use.SourceID, use.TargetID)
	}

	s.playSkillUseEffects(use)

	// A skill that hurt somebody is a blow, and is drawn like one.
	if use.Damage > 0 && use.TargetID != 0 {
		blow := packets.Damage{
			SourceID: use.SourceID,
			TargetID: use.TargetID,
			Amount:   use.Damage,
			Hits:     max(use.Hits, 1),
		}

		swing := blow.SwingDurationMs()
		if delay := s.hitDelayMs(use.SourceID, swing); delay > 0 {
			s.pendingBlows = append(s.pendingBlows,
				pendingBlow{blow: blow, remainingMs: delay})
		} else {
			s.landBlow(pendingBlow{blow: blow})
		}
	}

	s.reportSkill(use, s.selfAID())

	// Every skill shouts its name over the caster, damaging or not. It is the
	// original's own sign that something was cast, and for most buffs it is
	// the only one.
	s.addSkillLabel(use.SourceID, use.SkillID, skillLabelLifeMs)

	// The ones that restore hit points also show the figure, over whoever
	// they restored. Nothing on the wire tells them apart from a buff:
	// rAthena writes the amount and the skill level into the same field, so
	// Increase Agi at level 10 sends a 10 that means nothing and Heal sends
	// the hit points. Which skills restore is the client's own knowledge, and
	// healSkills is it.
	if use.Damage == 0 && healSkills[use.SkillID] && use.Amount > 0 {
		over := use.TargetID
		if over == 0 {
			over = use.SourceID
		}

		s.addHealNumber(over, use.Amount)
	}
}

// The battle log.
//
// The chat box has had a Battle tab and a color for its lines since it was
// written and nothing ever put anything in it. A skill going off is the first
// thing worth writing there: what it hit and for how much is otherwise a
// number that floats up and is gone, and what it cost is not shown anywhere at
// all.

// reportSkill writes what a skill did to the battle tab, when it is ours or it
// was aimed at us. Somebody else's fight across the field is not our log.
func (s *InGameState) reportSkill(use packets.SkillUse, self uint32) {
	if self == 0 {
		return
	}

	name := skills.Name(use.SkillID)
	if name == "" {
		name = "A skill"
	}

	switch {
	case use.SourceID == self:
		// What it cost, out of the skill list. The server's own figure for
		// the level we hold it at, rather than a table of the client's that
		// disagrees with it for twenty skills.
		if skill, known := s.findSkill(use.SkillID); known && skill.SP > 0 {
			s.chat.AddLocal(ChatDamage, fmt.Sprintf("%s: %d SP.", name, skill.SP))
		}

		if use.Damage > 0 {
			s.chat.AddLocal(ChatDamage, fmt.Sprintf("%s on %s: %s.",
				name, s.unitName(use.TargetID, self), damageFigure(use.Damage, use.Hits)))
		}

	case use.TargetID == self && use.Damage > 0:
		s.chat.AddLocal(ChatDamage, fmt.Sprintf("%s's %s on you: %s.",
			s.unitName(use.SourceID, self), name, damageFigure(use.Damage, use.Hits)))
	}
}

// damageFigure is what a blow did, and over how many hits when it was more
// than one. A bolt skill's figure is the total of its bolts, and reading it as
// a single hit understates what each one did by ten.
func damageFigure(damage, hits int) string {
	if hits > 1 {
		return fmt.Sprintf("%d damage over %d hits", damage, hits)
	}

	return fmt.Sprintf("%d damage", damage)
}

// unitName is what to call somebody in a line of the log.
func (s *InGameState) unitName(id, self uint32) string {
	switch id {
	case 0:
		return "nobody"
	case self:
		return "you"
	}

	if e := s.entityOf(id); e != nil && e.Name != "" {
		return e.Name
	}

	return "something"
}

// healSkills restore hit points, and show the figure rather than their name.
//
// Written out because nothing says it: the field rAthena puts the amount in is
// the same one it puts a skill level in for everything else, and neither the
// server's skill database nor the client's own table marks a skill as healing
// — Heal and Increase Agi are both Support and both NoDamage. So this is a
// list, and a skill that restores and is not on it shows its name instead,
// which is wrong but not misleading.
var healSkills = map[uint16]bool{
	28:   true, // AL_HEAL
	70:   true, // PR_SANCTUARY
	231:  true, // AM_POTIONPITCHER
	2051: true, // AB_HIGHNESSHEAL
}

// skillLabelLifeMs is how long a skill's name stays over its caster once the
// skill has gone off.
const skillLabelLifeMs = 1400

// skillLabelRise is how far above the head the name floats.
//
// Clear of the casting bar, which sits just above the head: the name is shown
// while the cast runs, and the two would otherwise be printed over each
// other. The plate hangs below its anchor by the same drop a monster's name
// does, so this is the height of the plate plus the bar plus a gap rather
// than a number picked to look right.
const skillLabelRise = float32(44)

// floatingSkillName is a skill's name over the unit it was cast on.
//
// The unit rather than the place: the name belongs over somebody's head and
// follows them, so a character buffed and then walking away carries it. Held
// as an id and looked up each frame, which is also what makes it vanish with
// whoever it was on.
type floatingSkillName struct {
	text   string
	target uint32

	ageMs  float32
	lifeMs float32
}

// addSkillLabel shouts a skill's name over its caster, for as long as asked.
//
// Over the caster rather than over what it was cast on: the original puts the
// name where the shout comes from, so a Heal on somebody else names itself
// over the healer while the figure goes over the healed.
func (s *InGameState) addSkillLabel(casterID uint32, skillID uint16, lifeMs float32) {
	name := skills.Name(skillID)
	if name == "" || s.bodyOf(casterID) == nil {
		return
	}

	// The original's own punctuation. A skill going off is a shout, and the
	// two marks are how it reads as one.
	label := floatingSkillName{text: name + " !!", target: casterID, lifeMs: lifeMs}

	// One name at a time over the same head. Two skills in a row would
	// otherwise stack into an unreadable pile — and a cast that finishes
	// replaces the name it was showing while it ran.
	for i := range s.skillLabels {
		if s.skillLabels[i].target == casterID {
			s.skillLabels[i] = label

			return
		}
	}

	s.skillLabels = append(s.skillLabels, label)
}

// advanceSkillLabels ages the names out.
func (s *InGameState) advanceSkillLabels(deltaMs float32) {
	if len(s.skillLabels) == 0 {
		return
	}

	kept := s.skillLabels[:0]
	for _, label := range s.skillLabels {
		label.ageMs += deltaMs

		// Gone with whoever it was over: a name floating where a monster used
		// to stand is worse than no name.
		if label.ageMs < label.lifeMs && s.bodyOf(label.target) != nil {
			kept = append(kept, label)
		}
	}

	s.skillLabels = kept
}

// SkillLabels are the skill names to draw over the world this frame.
func (s *InGameState) SkillLabels(viewportW, viewportH float32) []HoverLabel {
	if len(s.skillLabels) == 0 {
		return nil
	}

	labels := make([]HoverLabel, 0, len(s.skillLabels))
	for _, label := range s.skillLabels {
		body := s.bodyOf(label.target)
		if body == nil {
			continue
		}

		// Over the head, wherever the head is now.
		top := body.RenderY
		if e := s.entityOf(label.target); e != nil {
			top = s.unitBox(e).Max[1]
		} else if s.playerRender != nil {
			if _, height := s.playerRender.QuadSize(); height > 0 {
				top = body.RenderY + height
			}
		}

		x, y := s.projectToScreen(body.RenderX, top, body.RenderZ, viewportW, viewportH)
		labels = append(labels, HoverLabel{Text: label.text, ScreenX: x, ScreenY: y - skillLabelRise})
	}

	return labels
}

// UseSkillAt casts a skill at a cell.
//
// Separate from UseSkill because it is a different packet, not a different
// argument: rAthena parses the two through different handlers, and a
// ground-placed skill sent as a unit-targeted one is refused rather than
// misplaced.
func (s *InGameState) UseSkillAt(skillID uint16, level, cellX, cellY int) error {
	if s.client == nil {
		return nil
	}

	skill, known := s.findSkill(skillID)
	if !known {
		return nil
	}

	if skill.Inf&packets.InfGround == 0 {
		s.chat.AddLocal(ChatError, "That skill is not placed on the ground.")

		return nil
	}

	if level <= 0 {
		level = skill.Level
	}

	trace.Emit(trace.HUD, "use-skill-at",
		zap.Uint16("skill", skillID), zap.Int("level", level),
		zap.Int("cellX", cellX), zap.Int("cellY", cellY))

	return s.client.Send(packets.EncodeUseSkillAt(skillID, level, cellX, cellY))
}

// Placing a skill on the ground.
//
// A ground skill is asked for twice: once to choose it, and once to say where.
// Between the two the client is holding a skill and waiting for a cell, which
// is a state of its own — the next click means "here" rather than "walk there"
// or "attack that".

// BeginPlacing holds a skill until a cell is chosen for it.
func (s *InGameState) BeginPlacing(skillID uint16, level int) {
	s.beginHolding(skillID, level, false, "choose where to place it")
}

// BeginTargeting holds a skill until somebody is chosen for it.
//
// A targeted skill can go on the caster, another player, or a monster, and
// which of those this particular skill allows is the server's question — Heal
// goes on people and on the undead, an attack skill is refused against a
// player who is not in a fight. So anything picked is sent and the refusal, if
// there is one, comes back and is printed.
func (s *InGameState) BeginTargeting(skillID uint16, level int) {
	s.beginHolding(skillID, level, true, "choose who to cast it on")
}

func (s *InGameState) beginHolding(skillID uint16, level int, atUnit bool, what string) {
	s.placingSkill = skillID
	s.placingLevel = level
	s.placingAtUnit = atUnit

	trace.Emit(trace.HUD, "placing-begin",
		zap.Uint16("skill", skillID), zap.Int("level", level),
		zap.Bool("atUnit", atUnit))

	name := skills.Name(skillID)
	if name == "" {
		name = "That skill"
	}

	s.chat.AddLocal(ChatNotice, name+": "+what+".")
}

// Placing reports the skill waiting for a cell, and whether one is.
func (s *InGameState) Placing() (uint16, bool) {
	return s.placingSkill, s.placingSkill != 0
}

// Targeting reports whether the held skill is waiting for a unit rather than
// for a cell. The two are aimed differently and cannot stand in for each
// other: a cell given to a targeted skill is refused by the server.
func (s *InGameState) Targeting() bool {
	return s.placingSkill != 0 && s.placingAtUnit
}

// CancelPlacing puts the held skill down without casting it.
func (s *InGameState) CancelPlacing() {
	if s.placingSkill == 0 {
		return
	}

	trace.Emit(trace.HUD, "placing-cancel", zap.Uint16("skill", s.placingSkill))

	s.placingSkill = 0
	s.placingLevel = 0
	s.placingAtUnit = false
}

// placeHeldSkill casts the held skill at the cell under the cursor, and
// reports whether it took the click.
//
// The cursor's cell rather than the click's own reading of the screen: the
// marker is what the player is aiming with, and casting anywhere else would
// put the skill where the marker was not.
func (s *InGameState) placeHeldSkill(mouseX, mouseY, viewportW, viewportH float32) bool {
	if s.placingSkill == 0 {
		return false
	}

	skill, level, atUnit := s.placingSkill, s.placingLevel, s.placingAtUnit
	s.placingSkill, s.placingLevel, s.placingAtUnit = 0, 0, false

	if atUnit {
		// Whoever is under the pointer. A click on anything that is not a
		// target simply puts the skill down: the original drops it rather
		// than casting on the caster, and casting on the caster would spend
		// SP nobody asked to spend.
		//
		// Whether this particular target is allowed is the server's to say —
		// an attack skill on a player who is not in a fight is refused, and
		// the refusal already reaches the chat.
		var (
			target uint32
			name   string
		)

		if e := s.PickEntity(mouseX, mouseY, viewportW, viewportH); e != nil {
			target, name = e.ID, e.Name
		} else if s.pickedSelf(mouseX, mouseY, viewportW, viewportH) {
			// The character is not in the entity registry — it is driven by
			// its own prediction rather than by unit reports — so clicking it
			// picks nothing, and a skill anybody casts on themselves would be
			// dropped. It is tested for separately.
			target, name = s.selfAID(), "you"
		}

		if target == 0 {
			trace.Emit(trace.HUD, "cast-no-target", zap.Uint16("skill", skill))

			return true
		}

		trace.Emit(trace.HUD, "cast-at-unit",
			zap.Uint16("skill", skill), zap.Uint32("target", target),
			zap.String("name", name))

		if err := s.castOrApproach(skill, level, target); err != nil {
			logger.Warn("could not cast that skill", zap.Uint16("skill", skill), zap.Error(err))
		}

		return true
	}

	if !s.hoverValid {
		trace.Emit(trace.HUD, "placing-off-map", zap.Uint16("skill", skill))

		return true
	}

	if err := s.UseSkillAt(skill, level, s.hoverCellX, s.hoverCellY); err != nil {
		logger.Warn("could not place that skill", zap.Uint16("skill", skill), zap.Error(err))
	}

	return true
}

// CastHeldAtNearest aims the held skill at the closest monster in view, and
// reports whether it found one.
//
// For unattended runs, the same way AttackNearest is: an attack skill is
// chosen and then clicked onto something, and nothing an attack skill draws
// can be watched without a hand to click with.
func (s *InGameState) CastHeldAtNearest() bool {
	if s.placingSkill == 0 || !s.placingAtUnit || s.entityManager == nil || s.player == nil {
		return false
	}

	var (
		nearest *entity.Entity
		best    float32
	)

	px, _, pz := s.player.RenderPosition()

	for _, e := range s.entityManager.All() {
		if !s.isAttackable(e) || e.Body == nil {
			continue
		}

		x, _, z := e.Body.RenderPosition()
		dx, dz := x-px, z-pz

		if d := dx*dx + dz*dz; nearest == nil || d < best {
			nearest, best = e, d
		}
	}

	if nearest == nil {
		return false
	}

	skill, level := s.placingSkill, s.placingLevel
	s.placingSkill, s.placingLevel, s.placingAtUnit = 0, 0, false

	trace.Emit(trace.HUD, "cast-at-nearest",
		zap.Uint16("skill", skill), zap.Uint32("target", nearest.ID),
		zap.String("name", nearest.Name))

	if err := s.castOrApproach(skill, level, nearest.ID); err != nil {
		logger.Warn("could not cast that skill", zap.Uint16("skill", skill), zap.Error(err))
	}

	return true
}

// castAt sends a cast at a named unit, past the targeting UseSkill does.
func (s *InGameState) castAt(skillID uint16, level int, target uint32) error {
	if s.client == nil {
		return nil
	}

	return s.client.Send(packets.EncodeUseSkill(skillID, level, target))
}

// The cast bar.

// CastProgress is how far through a cast the character is, from zero to one,
// and whether one is running.
//
// The server sends the length in ZC_USESKILL_ACK and nothing else about it:
// there is no tick, and no "still casting". So the bar is run off that one
// number, and a cast that is interrupted is ended by the interruption rather
// than by running out.
func (s *InGameState) CastProgress() (float32, uint16, bool) {
	if s.castTotalMs <= 0 {
		return 0, 0, false
	}

	done := 1 - s.castLeftMs/s.castTotalMs

	return min(max(done, 0), 1), s.castSkill, true
}

// advanceCast runs the cast bar down.
func (s *InGameState) advanceCast(deltaMs float32) {
	if s.castTotalMs <= 0 {
		return
	}

	s.castLeftMs -= deltaMs
	if s.castLeftMs <= 0 {
		s.castTotalMs, s.castLeftMs, s.castSkill = 0, 0, 0
	}
}

// beginCast starts the bar for our own cast.
func (s *InGameState) beginCast(skillID uint16, castMs float32) {
	if castMs <= 0 {
		// Instant, which most skills are. A bar that appears and vanishes in
		// one frame is worse than no bar.
		return
	}

	s.castSkill = skillID
	s.castTotalMs = castMs
	s.castLeftMs = castMs
}

// PlaceHeldSkillAt casts the held skill at a named cell.
//
// For a caller that has a cell already rather than a cursor over one — an
// unattended run, and whatever else eventually aims a skill without a mouse.
func (s *InGameState) PlaceHeldSkillAt(cellX, cellY int) error {
	if s.placingSkill == 0 {
		return nil
	}

	skill, level := s.placingSkill, s.placingLevel
	s.placingSkill, s.placingLevel, s.placingAtUnit = 0, 0, false

	return s.UseSkillAt(skill, level, cellX, cellY)
}

// PlayerCell is the cell the character is standing on.
func (s *InGameState) PlayerCell() (int, int) {
	if s.player == nil {
		return 0, 0
	}

	return s.player.CurrentCell()
}

// pickedSelf reports whether a click landed on our own character.
//
// Everything else on the map is picked out of the entity registry, and the
// character being played is deliberately not in it. So its box is built here
// from the same two things a unit's is: where the body is standing, and how
// big its billboard was baked.
func (s *InGameState) pickedSelf(screenX, screenY, viewportW, viewportH float32) bool {
	if s.player == nil || s.scene == nil || s.playerRender == nil ||
		viewportW <= 0 || viewportH <= 0 {
		return false
	}

	width, height := s.playerRender.QuadSize()
	if width <= 0 || height <= 0 {
		return false
	}

	x, y, z := s.player.RenderX, s.player.RenderY, s.player.RenderZ
	half := width / 2

	box := picking.AABB{
		Min: [3]float32{x - half, y, z - half},
		Max: [3]float32{x + half, y + height, z + half},
	}

	ray := picking.ScreenToRay(screenX, screenY, viewportW, viewportH,
		s.scene.LastViewProj().Inverse())

	t, hit := ray.IntersectAABB(box)

	return hit && t >= 0
}

// CastBar is the cast in progress, ready to draw.
type CastBar struct {
	// Progress runs 0 to 1 across the cast.
	Progress float32

	// ScreenX, ScreenY is where the caster's feet are, in viewport pixels.
	ScreenX, ScreenY float32

	// Name is the skill being cast, for whatever wants to say so.
	Name string
}

// CastingBar is the cast to draw this frame, and whether there is one.
//
// Over the caster's head rather than under its feet: the ring the cast draws on
// the ground is already down there, and a bar in the same place is read as part
// of it. There is nothing to draw for an instant skill, which is most of them.
func (s *InGameState) CastingBar(viewportW, viewportH float32) (CastBar, bool) {
	progress, skill, casting := s.CastProgress()
	if !casting || s.player == nil {
		return CastBar{}, false
	}

	top := s.player.RenderY
	if s.playerRender != nil {
		if _, height := s.playerRender.QuadSize(); height > 0 {
			top += height
		}
	}

	x, y := s.projectToScreen(s.player.RenderX, top, s.player.RenderZ,
		viewportW, viewportH)

	return CastBar{
		Progress: progress,
		ScreenX:  x,
		ScreenY:  y,
		Name:     skills.Name(skill),
	}, true
}

// The casting aura: the ring that lies under somebody while they cast.
//
// EF_BEGINSPELL in the original, which nostalro-client's port builds from four
// rising cone emitters around the caster. This is the ring those emitters
// carry, lying flat and growing, which is what it reads as from a distance and
// what the client can draw today — the cones want a renderer it does not have.
// The table says which skills call for it; this is the drawing.

const (
	// The aura stands around the caster rather than lying under it: in the
	// original the character is inside it, and drawn flat it reads as the
	// wrong thing entirely.
	//
	// Radius at the foot and at the top, and how tall. The original's four
	// emitters start at 4.1 units out and lean from 80 degrees to 10 as they
	// rise, which is a wall that starts near vertical and opens outward; a
	// tube with a wider top is that shape without the four separate cones.
	castAuraRadius    = float32(4.1)
	castAuraFlare     = float32(1.6)
	castAuraHeightMax = float32(20)

	// castAuraFade is how much of the cast is spent fading out at the end, so
	// the aura does not vanish mid-turn.
	castAuraFade = float32(0.25)
)

// castingAura is one caster's ring.
type castingAura struct {
	// caster is who it is under, looked up each frame so it follows them.
	caster uint32

	// leftMs counts down, and totalMs is what it started at.
	leftMs, totalMs float32
}

// beginCastAura starts the ring under a caster, if this skill calls for one.
func (s *InGameState) beginCastAura(cast packets.SkillCast) {
	effects, known := skills.EffectsOf(cast.SkillID)
	if !known {
		return
	}

	wanted := false
	for _, name := range effects.BeginCast {
		if name == castAuraEffect {
			wanted = true
		}
	}

	if !wanted || cast.CastMs == 0 {
		return
	}

	// One ring per caster: a second cast replaces the first rather than
	// stacking two rings in the same place.
	for i := range s.castAuras {
		if s.castAuras[i].caster == cast.SourceID {
			s.castAuras[i] = castingAura{
				caster:  cast.SourceID,
				leftMs:  float32(cast.CastMs),
				totalMs: float32(cast.CastMs),
			}

			return
		}
	}

	s.castAuras = append(s.castAuras, castingAura{
		caster:  cast.SourceID,
		leftMs:  float32(cast.CastMs),
		totalMs: float32(cast.CastMs),
	})
}

// castAuraEffect is the effect name the table uses for the casting circle.
const castAuraEffect = "EF_BEGINSPELL"

// castAuraHoldLoopMs is how long one cycle of the held ring takes. The
// original's is 56 frames at 60fps, which is what nostalro-client reads out of
// it, so a held one repeats at the same rate rather than at a pace of its own.
const castAuraHoldLoopMs = float32(56) / 60 * 1000

// HoldCastAura keeps a ring under the character for as long as the client
// runs, so the effect can be looked at rather than glimpsed.
func (s *InGameState) HoldCastAura() {
	s.holdCastAura = true
}

// advanceCastAuras runs the rings down.
func (s *InGameState) advanceCastAuras(deltaMs float32) {
	// Held for inspection: one ring under the character, never expiring, its
	// growth looping so the whole of it can be seen.
	if s.holdCastAura && s.player != nil {
		if len(s.castAuras) == 0 {
			s.castAuras = []castingAura{{
				caster:  s.selfAID(),
				totalMs: castAuraHoldLoopMs,
				leftMs:  castAuraHoldLoopMs,
			}}
		}

		s.castAuras[0].leftMs -= deltaMs
		if s.castAuras[0].leftMs <= 0 {
			s.castAuras[0].leftMs = castAuraHoldLoopMs
		}

		return
	}

	if len(s.castAuras) == 0 {
		return
	}

	kept := s.castAuras[:0]
	for _, aura := range s.castAuras {
		aura.leftMs -= deltaMs

		// Gone with the caster, the same way a skill's name is.
		if aura.leftMs > 0 && s.bodyOf(aura.caster) != nil {
			kept = append(kept, aura)
		}
	}

	s.castAuras = kept
}

// drawCastAuras puts a ring under everybody casting.
func (s *InGameState) drawCastAuras(viewProj math.Mat4) {
	if s.castAura == nil {
		return
	}

	for _, aura := range s.castAuras {
		body := s.bodyOf(aura.caster)
		if body == nil || aura.totalMs <= 0 {
			continue
		}

		done := 1 - aura.leftMs/aura.totalMs

		alpha := float32(1)
		if done > 1-castAuraFade {
			alpha = (1 - done) / castAuraFade
		}

		// It rises over the cast and opens outward as it goes, which is the
		// four leaning emitters the original builds it from, drawn as one
		// wall.
		height := castAuraHeightMax * done
		top := castAuraRadius + (castAuraFlare-1)*castAuraRadius*done

		s.castAura.RenderTube(viewProj,
			body.RenderX, s.terrainHeight(body.RenderX, body.RenderZ), body.RenderZ,
			castAuraRadius, top, height, alpha)
	}
}

// Playing a skill's effects.
//
// The ported table says which effect belongs where; this puts the ones the
// archive has art for on screen. Thirty-five of the hundred and sixty-five it
// names are STR animations sitting loose in the effect directory under the
// name the table uses, which the client's existing player can already run —
// the hit sparks among them, which is what shows in an ordinary fight.
//
// The rest are the ones the original draws in code rather than from a file.
// They are named and not drawn, and the miss is logged once each, which makes
// the log the list of what is left.

// effectSoundDir is where the archive keeps what an effect sounds like.
const effectSoundDir = `data\wav\effect\`

// effectSoundFor is the wav an effect plays, from the name the table uses.
//
// Named after the effect rather than after the skill: the archive has
// ef_bash.wav and ef_frostdiver.wav and no sm_bash.wav at all. Twenty-six of
// the effects the table names have one, and between them they cover a hundred
// and twenty-four of its entries — including the casting sound, which belongs
// to EF_BEGINSPELL and so to every skill that casts.
func effectSoundFor(effect string) string {
	if len(effect) <= 3 || effect[:3] != "EF_" {
		return ""
	}

	return effectSoundDir + strings.ToLower(effect) + ".wav"
}

// effectFileFor is the STR the archive files an effect under, from the name
// the table uses: EF_FIREHIT is firehit.str.
func effectFileFor(effect string) string {
	if len(effect) <= 3 || effect[:3] != "EF_" {
		return ""
	}

	return strings.ToLower(effect[3:]) + ".str"
}

// playSkillEffects puts a list of them over a world position.
func (s *InGameState) playSkillEffects(effects []string, x, y, z float32) {
	for _, effect := range effects {
		file := effectFileFor(effect)
		if file == "" {
			continue
		}

		s.PlayEffectAt(file, x, y, z)
	}
}

// playSkillBursts starts the ones drawn in code rather than read from a file.
//
// from is where the caster is standing, for the effects drawn between the two
// of them rather than around one — Frost Diver walks its spikes out from
// there. Zero when there is nobody to draw from, and those effects are then
// left out rather than heaped on the target.
func (s *InGameState) playSkillBursts(effects []string, hits int, from, at [3]float32) {
	for _, effect := range effects {
		spec, ok := burstFor(effect, hits)
		if !ok {
			continue
		}

		s.playBurst(effect, spec, from, at)
	}
}

// casterAt is where a caster is standing, for an effect drawn from them.
func (s *InGameState) casterAt(id uint32) [3]float32 {
	x, y, z, ok := s.effectHeight(id)
	if !ok {
		return [3]float32{}
	}

	return [3]float32{x, y, z}
}

// A volley's shots land one after another.
//
// A bolt skill arrives as one packet saying how much it did over how many
// blows, and the client draws the blows. What each shot hits has to wait for
// it to come down: played at the start, ten flashes go off on a target that
// nothing has reached yet, which is what made a bolt skill look like one
// blow with a large figure over it.

// delayedEffect is something waiting to be drawn on a unit.
type delayedEffect struct {
	effect string
	target uint32

	// caster is who threw it, for the effects drawn from them to the target.
	caster uint32

	delayMs float32
}

// advanceDelayedEffects plays the ones whose moment has come.
func (s *InGameState) advanceDelayedEffects(deltaMs float32) {
	if len(s.delayedEffects) == 0 {
		return
	}

	kept := s.delayedEffects[:0]
	for _, waiting := range s.delayedEffects {
		waiting.delayMs -= deltaMs
		if waiting.delayMs > 0 {
			kept = append(kept, waiting)

			continue
		}

		// Gone with whoever it was aimed at. A flash where a monster used to
		// stand is worse than no flash.
		x, y, z, ok := s.effectHeight(waiting.target)
		if !ok {
			continue
		}

		one := []string{waiting.effect}
		s.playSkillEffects(one, x, y, z)
		s.playSkillBursts(one, 1, s.casterAt(waiting.caster), [3]float32{x, y, z})
	}

	s.delayedEffects = kept
}

// boltEffects are the falling shots. What a skill lists beside one of these is
// what its shots hit with, and is drawn once for each of them.
var boltEffects = map[string]bool{
	"EF_ICEARROW":  true,
	"EF_FIREARROW": true,
}

// splitBolts separates a skill's shots from what they hit with.
func splitBolts(effects []string) (bolts, onImpact []string) {
	for _, effect := range effects {
		if boltEffects[effect] {
			bolts = append(bolts, effect)

			continue
		}

		onImpact = append(onImpact, effect)
	}

	return bolts, onImpact
}

// playSkillSounds asks for what a list of effects sounds like.
//
// Separate from the drawing because the two do not overlap: an effect can have
// a sound and no animation, which is most of them, or an animation and no
// sound.
func (s *InGameState) playSkillSounds(effects []string) {
	for _, effect := range effects {
		s.playSound(effectSoundFor(effect))
	}
}

// effectHeight is where on a unit an effect plays: its middle, rather than the
// ground it stands on. A spark at a monster's feet reads as a spark at the
// ground beside it.
func (s *InGameState) effectHeight(id uint32) (x, y, z float32, ok bool) {
	body := s.bodyOf(id)
	if body == nil {
		return 0, 0, 0, false
	}

	middle := body.RenderY
	if e := s.entityOf(id); e != nil {
		box := s.unitBox(e)
		middle = (box.Min[1] + box.Max[1]) / 2
	} else if s.playerRender != nil {
		if _, height := s.playerRender.QuadSize(); height > 0 {
			middle = body.RenderY + height/2
		}
	}

	return body.RenderX, middle, body.RenderZ, true
}

// playSkillUseEffects draws what a skill did, wherever it did it.
func (s *InGameState) playSkillUseEffects(use packets.SkillUse) {
	effects, known := skills.EffectsOf(use.SkillID)
	if !known {
		return
	}

	hits := max(use.Hits, 1)
	from := s.casterAt(use.SourceID)

	if x, y, z, ok := s.effectHeight(use.SourceID); ok {
		s.playSkillEffects(effects.OnCaster, x, y, z)
		s.playSkillBursts(effects.OnCaster, hits, from, [3]float32{x, y, z})

		// And the flash a skill starts with, for the ones that start here.
		// A skill with a cast time shows that when the cast begins; every
		// skill that has one of these — Bash, Mammonite, Double Strafe — is
		// instant, and an instant skill has no cast packet to show it from.
		//
		// Bursts only. The begin-cast effect that comes from a file is the
		// casting circle, and beginCastAura owns that.
		s.playSkillBursts(effects.BeginCast, hits, from, [3]float32{x, y, z})
	}
	s.playSkillSounds(effects.OnCaster)

	if use.TargetID != 0 {
		bolts, onImpact := splitBolts(effects.OnTarget)

		if len(bolts) > 0 {
			// The volley now, and what its shots hit with as each one lands.
			if x, y, z, ok := s.effectHeight(use.TargetID); ok {
				s.playSkillBursts(bolts, hits, from, [3]float32{x, y, z})
			}

			for i := 0; i < min(hits, boltMax); i++ {
				for _, effect := range onImpact {
					s.delayedEffects = append(s.delayedEffects, delayedEffect{
						effect: effect, target: use.TargetID, caster: use.SourceID,
						delayMs: boltImpactMs(i),
					})
				}
			}
		} else if x, y, z, ok := s.effectHeight(use.TargetID); ok {
			s.playSkillEffects(effects.OnTarget, x, y, z)
			s.playSkillBursts(effects.OnTarget, hits, from, [3]float32{x, y, z})
		}

		s.playSkillSounds(effects.OnTarget)
	}

	if use.Ground {
		x, z := entity.CellToWorld(use.CellX, use.CellY)
		s.playSkillEffects(effects.OnGround, x, s.terrainHeight(x, z), z)
		s.playSkillBursts(effects.OnGround, hits, from, [3]float32{x, s.terrainHeight(x, z), z})
		s.playSkillSounds(effects.OnGround)
	}
}

// Casting at something too far away.
//
// A skill aimed at a target out of its reach used to be sent anyway and
// refused. The original walks: the character closes on the target, stops as
// soon as it is near enough, and casts. Held here rather than sent because the
// server refuses a cast from too far and says so, which is a message instead
// of a spell.

// pendingSkillCast is a skill waiting for the character to get close enough.
type pendingSkillCast struct {
	skill  uint16
	level  int
	target uint32

	// repathMs throttles reissuing the walk while the target moves, the same
	// way chasing something to hit it does.
	repathMs float32
}

// castOrApproach casts at a unit, or walks towards it first.
func (s *InGameState) castOrApproach(skillID uint16, level int, target uint32) error {
	if s.withinSkillRange(skillID, target) {
		s.forgetPendingSkill()

		return s.castAt(skillID, level, target)
	}

	if s.entityOf(target) == nil {
		// Nothing to walk towards — the caster itself, or somebody who has
		// gone. Send it and let the server answer.
		return s.castAt(skillID, level, target)
	}

	trace.Emit(trace.HUD, "cast-approach",
		zap.Uint16("skill", skillID), zap.Uint32("target", target),
		zap.Int("range", s.skillRange(skillID)))

	s.pendingSkill = &pendingSkillCast{skill: skillID, level: level, target: target}

	return nil
}

// skillRange is how far a skill reaches, in cells, as the server reported it.
//
// Zero for a skill the server has not listed, and a reach of one for a skill
// it listed with none — that is a melee skill, and treating no range as
// unlimited would have the character cast Bash across the map.
func (s *InGameState) skillRange(skillID uint16) int {
	skill, known := s.findSkill(skillID)
	if !known {
		return 0
	}

	if skill.Range <= 0 {
		return 1
	}

	return skill.Range
}

// withinSkillRange reports whether a cast would reach.
func (s *InGameState) withinSkillRange(skillID uint16, target uint32) bool {
	if s.player == nil {
		return true
	}

	if target == s.selfAID() {
		return true
	}

	e := s.entityOf(target)
	if e == nil || e.Body == nil {
		return true
	}

	reach := s.skillRange(skillID)
	if reach <= 0 {
		return true
	}

	targetX, targetY := e.Body.CurrentCell()
	playerX, playerY := s.player.CurrentCell()

	return abs(targetX-playerX) <= reach && abs(targetY-playerY) <= reach
}

// advancePendingSkill walks towards what is being cast at, and casts when it
// is near enough.
func (s *InGameState) advancePendingSkill(deltaMs float32) {
	pending := s.pendingSkill
	if pending == nil {
		return
	}

	e := s.entityOf(pending.target)
	if e == nil || e.Body == nil || e.IsDead {
		s.forgetPendingSkill()

		return
	}

	if s.withinSkillRange(pending.skill, pending.target) {
		skill, level := pending.skill, pending.level
		target := pending.target
		s.forgetPendingSkill()

		// Stopped where it stands: the walk is canceled by arriving rather
		// than by reaching the cell it was sent to, so the character casts
		// from the furthest point that reaches.
		s.cancelWalk()

		if err := s.castAt(skill, level, target); err != nil {
			logger.Warn("could not cast on approach",
				zap.Uint16("skill", skill), zap.Error(err))
		}

		return
	}

	pending.repathMs -= deltaMs
	if pending.repathMs > 0 {
		return
	}
	pending.repathMs = targetRepathMs

	// Towards the target's own cell. The server stops the walk on the last
	// free cell before it, and this stops itself as soon as the range check
	// passes, which is sooner.
	cellX, cellY := e.Body.CurrentCell()
	if err := s.RequestMove(cellX, cellY); err != nil {
		logger.Warn("could not walk to cast", zap.Error(err))
	}
}

// forgetPendingSkill drops a cast that was waiting to get close.
func (s *InGameState) forgetPendingSkill() {
	if s.pendingSkill == nil {
		return
	}

	trace.Emit(trace.HUD, "cast-approach-dropped",
		zap.Uint16("skill", s.pendingSkill.skill))

	s.pendingSkill = nil
}
