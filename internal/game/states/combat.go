package states

import (
	"go.uber.org/zap"

	"github.com/Faultbox/midgard-ro/internal/game/entity"
	"github.com/Faultbox/midgard-ro/internal/logger"
	"github.com/Faultbox/midgard-ro/internal/network/packets"
	"github.com/Faultbox/midgard-ro/internal/trace"
)

// attackRange is how close the character has to be to swing, in cells.
//
// One, which is melee. The server checks the attacker's real weapon range in
// battle_check_range and refuses from further off, so this is the client
// agreeing with the common case rather than the whole truth: a bow would be
// further, and this will need the range the server actually gave us when
// there is a weapon that reaches.
const attackRange = 1

// attackIdleGiveUpMs is how long the character may stand still on the way to
// a target before the attack is abandoned. Same reasoning as the pick-up: a
// walk pauses between acknowledged paths, so a stop has to be sustained
// before it counts as one.
const attackIdleGiveUpMs = 600

// AttackTarget attacks a unit, walking to it first if it is out of reach.
//
// The request repeats: a click on a monster in the original means keep
// swinging until it dies or something else is asked for, not swing once. That
// is also why nothing here has to send the second blow — the server does.
func (s *InGameState) AttackTarget(e *entity.Entity) {
	if e == nil || !s.isAttackable(e) || s.client == nil || e.Body == nil {
		return
	}

	s.targetID = e.ID

	if s.withinAttackRange(e) {
		s.sendAttack(e)

		return
	}

	// Out of reach: walk to it and swing on arrival. Walking onto the target's
	// own cell is not possible and not the point — the server's pathing stops
	// on the last free cell before it, which is where a melee blow lands from.
	cellX, cellY := e.Body.CurrentCell()

	trace.Emit(trace.HUD, "attack-approach",
		zap.Uint32("id", e.ID), zap.String("name", e.Name),
		zap.Int("x", cellX), zap.Int("y", cellY))

	if err := s.RequestMove(cellX, cellY); err != nil {
		logger.Warn("could not walk to that target", zap.Error(err))

		return
	}

	s.pendingAttack = e.ID
	s.pendingAttackIdleMs = 0
}

// sendAttack asks for the blow.
func (s *InGameState) sendAttack(e *entity.Entity) {
	trace.Emit(trace.HUD, "attack",
		zap.Uint32("id", e.ID), zap.String("name", e.Name),
		zap.Int("hp", e.HP), zap.Int("maxHP", e.MaxHP))

	if err := s.client.Send(packets.EncodeAttack(e.ID, true)); err != nil {
		logger.Warn("attack failed", zap.Error(err))
	}
}

// isAttackable reports whether a unit can be hit at all. Monsters only, for
// now: hitting other players needs the map's own rules about who may, which
// is a different feature.
func (s *InGameState) isAttackable(e *entity.Entity) bool {
	return e != nil && e.Type == entity.TypeMonster && !e.IsDead
}

// withinAttackRange reports whether the server would accept a blow from where
// the character stands.
func (s *InGameState) withinAttackRange(e *entity.Entity) bool {
	if s.player == nil || e == nil || e.Body == nil {
		return false
	}

	targetX, targetY := e.Body.CurrentCell()
	playerX, playerY := s.player.CurrentCell()

	return abs(targetX-playerX) <= attackRange && abs(targetY-playerY) <= attackRange
}

// updatePendingAttack swings once the walk to a target has ended.
//
// Nothing is sent while walking, for the reason the pick-up learned the hard
// way: the client reaches a cell before word of it reaches the server, so a
// request sent on arrival-as-the-client-sees-it is measured against a
// character the server still has further back, and is refused.
func (s *InGameState) updatePendingAttack(deltaMs float32, walking bool) {
	if s.pendingAttack == 0 || s.entityManager == nil {
		return
	}

	e := s.entityManager.Get(s.pendingAttack)
	if !s.isAttackable(e) {
		trace.Emit(trace.HUD, "attack-target-gone", zap.Uint32("id", s.pendingAttack))
		s.pendingAttack = 0

		return
	}

	if walking {
		s.pendingAttackIdleMs = 0

		return
	}

	if s.withinAttackRange(e) {
		s.pendingAttack = 0
		s.sendAttack(e)

		return
	}

	s.pendingAttackIdleMs += deltaMs
	if s.pendingAttackIdleMs >= attackIdleGiveUpMs {
		trace.Emit(trace.HUD, "attack-unreachable", zap.Uint32("id", s.pendingAttack))
		s.pendingAttack = 0
	}
}

// forgetAttack drops the target and any errand to reach it.
//
// Clicking somewhere else breaks off the fight. The server stops swinging
// when it is told to walk, so the walk request that follows is what actually
// cancels it — this is the client agreeing rather than the client deciding.
func (s *InGameState) forgetAttack() {
	if s.pendingAttack == 0 && s.targetID == 0 {
		return
	}

	trace.Emit(trace.HUD, "attack-cancelled", zap.Uint32("id", s.targetID))
	s.pendingAttack = 0
	s.pendingAttackIdleMs = 0
	s.targetID = 0
}

// TargetID is the unit being attacked, or zero.
func (s *InGameState) TargetID() uint32 {
	return s.targetID
}

// handleDamage applies a blow.
//
// The packet goes to everyone who can see the fight, so blows between two
// other units arrive here too and are worth drawing even though neither side
// is us. Nothing is deducted from a monster's health here: the server sends
// the real figure in ZC_MONSTER_HP_INFO, and subtracting locally as well
// would count every blow twice.
func (s *InGameState) handleDamage(data []byte) error {
	blow, ok := packets.DecodeDamage(data)
	if !ok {
		logger.Warn("short damage packet", zap.Int("len", len(data)))

		return nil
	}

	trace.Emit(trace.HUD, "damage",
		zap.Uint32("from", blow.SourceID), zap.Uint32("to", blow.TargetID),
		zap.Int("amount", blow.Amount), zap.Int("hits", blow.Hits),
		zap.Bool("miss", blow.Missed()), zap.Bool("critical", blow.Critical()))

	return nil
}

// handleMonsterHP takes the server's own figure for a monster's health, which
// is what the bar over its head shows.
func (s *InGameState) handleMonsterHP(data []byte) error {
	id, hp, maxHP, ok := packets.DecodeMonsterHP(data)
	if !ok {
		logger.Warn("short monster hp packet", zap.Int("len", len(data)))

		return nil
	}

	if s.entityManager == nil {
		return nil
	}

	e := s.entityManager.Get(id)
	if e == nil {
		return nil
	}

	e.HP, e.MaxHP = hp, maxHP
	e.IsDead = maxHP > 0 && hp <= 0

	trace.Emit(trace.HUD, "monster-hp",
		zap.Uint32("id", id), zap.Int("hp", hp), zap.Int("maxHP", maxHP))

	return nil
}
