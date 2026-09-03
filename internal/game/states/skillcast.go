package states

import (
	"go.uber.org/zap"

	"github.com/Faultbox/midgard-ro/internal/game/skills"
	"github.com/Faultbox/midgard-ro/internal/logger"
	"github.com/Faultbox/midgard-ro/internal/network/packets"
	"github.com/Faultbox/midgard-ro/internal/trace"
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

	// What a skill restored, over whoever got it. Nothing else about a heal
	// shows on screen — no flinch, no death, no figure — so without this a
	// heal that worked and one that never fired look the same.
	if use.Damage <= 0 && use.Amount > 0 && use.TargetID != 0 {
		s.addHealNumber(use.TargetID, use.Amount)
	}
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
// A support skill can go on anybody, and until one is picked there is nothing
// to send: casting it at whatever happened to be attacked last would heal a
// monster, and casting it always at the caster would make half the skill
// unreachable — which is what it did.
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
		// Whoever is under the pointer, and the caster when that is nobody:
		// a support skill always has one target that is certainly valid, and
		// a click on empty ground meaning "on me" is what the original does.
		target := s.selfAID()
		if e := s.PickEntity(mouseX, mouseY, viewportW, viewportH); e != nil {
			target = e.ID
		}

		trace.Emit(trace.HUD, "cast-at-unit",
			zap.Uint16("skill", skill), zap.Uint32("target", target))

		if err := s.castAt(skill, level, target); err != nil {
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
	s.placingSkill, s.placingLevel = 0, 0

	return s.UseSkillAt(skill, level, cellX, cellY)
}

// PlayerCell is the cell the character is standing on.
func (s *InGameState) PlayerCell() (int, int) {
	if s.player == nil {
		return 0, 0
	}

	return s.player.CurrentCell()
}
