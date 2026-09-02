package states

import (
	"go.uber.org/zap"

	"github.com/Faultbox/midgard-ro/internal/game/entity"
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

	// No motion yet. The sprite has cast poses and the client's own table
	// names one per skill — ACTOR_STATE, read in the last change — but they
	// are ACT sets this client does not bake, so playing something else would
	// be inventing a pose rather than using the sprite's. The cast bar this
	// packet's time is for comes with that.

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

	// The sprite's own cast motion, for a skill that named one. Skills the
	// client has no entry for use the plain attack, which is what an
	// unremarkable skill looks like.
	if body := s.bodyOf(use.SourceID); body != nil && use.Damage <= 0 {
		body.PlayOnce(entity.ActionAttack)
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
