package states

import (
	"go.uber.org/zap"

	"github.com/Faultbox/midgard-ro/internal/logger"
	"github.com/Faultbox/midgard-ro/internal/network/packets"
	"github.com/Faultbox/midgard-ro/internal/trace"
)

// RaiseStat spends a status point on one stat.
//
// Nothing is changed locally. The server decides whether the point is
// affordable and answers with the stat's new value, so raising it here as
// well would show a number that the next status packet would quietly take
// back.
func (s *InGameState) RaiseStat(statID uint16) error {
	if s.client == nil {
		return nil
	}

	trace.Emit(trace.HUD, "raise-stat", zap.Uint16("stat", statID))

	return s.client.Send(packets.EncodeStatusUp(statID, 1))
}

// RaiseSkill spends a skill point on one skill.
//
// One point at a time, which is what the original sends: the packet carries
// no amount, so the server raises the skill by exactly one.
func (s *InGameState) RaiseSkill(skillID uint16) error {
	if s.client == nil {
		return nil
	}

	trace.Emit(trace.HUD, "raise-skill", zap.Uint16("skill", skillID))

	return s.client.Send(packets.EncodeSkillUp(skillID))
}

// UnequipItem takes off what is worn in an inventory slot.
func (s *InGameState) UnequipItem(index int) error {
	if s.client == nil {
		return nil
	}

	trace.Emit(trace.HUD, "unequip-item", zap.Int("index", index))

	return s.client.Send(packets.EncodeUnequip(index))
}

// handleLevelUpEffect notes a level or job level being reached.
//
// The server announces both through ZC_NOTIFY_EFFECT, the same packet it uses
// for a refine succeeding or a pharmacy brew — the code says which. Reaching
// both at once arrives as two of these, which is what lets the two be shown
// one after the other rather than on top of each other.
func (s *InGameState) handleLevelUpEffect(data []byte) error {
	effect, ok := packets.DecodeNotifyEffect(data)
	if !ok {
		logger.Warn("short effect packet", zap.Int("len", len(data)))

		return nil
	}

	// Only our own advancement raises a button; somebody else levelling
	// beside us is their business.
	if effect.AID != s.selfAID() {
		return nil
	}

	switch effect.Effect {
	case packets.EffectBaseLevelUp:
		trace.Emit(trace.HUD, "level-up", zap.String("which", "base"))
		s.pendingLevelUp = true

	case packets.EffectJobLevelUp:
		trace.Emit(trace.HUD, "level-up", zap.String("which", "job"))
		s.pendingJobLevelUp = true
	}

	return nil
}

// LevelUpPending reports whether a base or job level has been reached and not
// yet acknowledged, which is what the buttons at the foot of the screen are
// for.
func (s *InGameState) LevelUpPending() (base, job bool) {
	return s.pendingLevelUp, s.pendingJobLevelUp
}

// AcknowledgeLevelUp puts one of those buttons away, which is what opening
// the window it points at means.
func (s *InGameState) AcknowledgeLevelUp(base bool) {
	if base {
		s.pendingLevelUp = false

		return
	}

	s.pendingJobLevelUp = false
}
