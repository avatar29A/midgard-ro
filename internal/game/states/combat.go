package states

import (
	"go.uber.org/zap"

	"github.com/Faultbox/midgard-ro/internal/game/entity"
	"github.com/Faultbox/midgard-ro/internal/logger"
	"github.com/Faultbox/midgard-ro/internal/network/packets"
	"github.com/Faultbox/midgard-ro/internal/trace"
)

// defaultAttackRange is the reach assumed until the server says otherwise.
//
// One cell, which is a bare fist. The server sends the real figure in
// ZC_ATTACK_RANGE whenever the weapon changes, and that is what is used once
// it arrives — this only covers the moment before the first one.
const defaultAttackRange = 1

// targetRepathMs is how often the walk towards a target is reissued while
// chasing it.
//
// A target that walks is the normal case — porings wander, and anything
// hostile closes on you — so a single walk to where it stood when you clicked
// arrives at an empty cell. The route is reissued as it moves, throttled so
// this does not become a move request every frame.
const targetRepathMs = 350

// attackResendMs is how often the attack is reissued while in range.
//
// The request repeats server-side, so once is usually enough; this is the
// safety net for the case where the target stepped out of reach and back,
// which stops the server's repeat without telling us.
const attackResendMs = 1200

// AttackTarget attacks a unit, walking to it first if it is out of reach.
//
// The request repeats: a click on a monster in the original means keep
// swinging until it dies or something else is asked for, not swing once. That
// is also why nothing here has to send the second blow — the server does.
func (s *InGameState) AttackTarget(e *entity.Entity) {
	if e == nil || !s.isAttackable(e) || s.client == nil || e.Body == nil {
		return
	}

	trace.Emit(trace.HUD, "attack-target",
		zap.Uint32("id", e.ID), zap.String("name", e.Name))

	s.targetID = e.ID
	s.attacking = false
	s.repathMs = 0
	s.resendMs = 0

	// The rest is updateCombat's, every frame: closing the distance, swinging
	// when close enough, and closing it again when the target moves off.
	// Doing it here as well would be the same decision made twice.
}

// updateCombat chases the target and hits it.
//
// A target is kept until it dies, leaves, or another click countermands it —
// not until one walk finishes. Anything worth fighting moves, so a single
// walk to where it stood when you clicked arrives at an empty cell, which is
// what made the first version give up a step short of everything.
func (s *InGameState) updateCombat(deltaMs float32, walking bool) {
	if s.targetID == 0 || s.entityManager == nil {
		return
	}

	e := s.entityManager.Get(s.targetID)
	if !s.isAttackable(e) {
		trace.Emit(trace.HUD, "attack-target-gone", zap.Uint32("id", s.targetID))
		s.forgetAttack()

		return
	}

	if s.withinAttackRange(e) {
		s.closeWith(e, deltaMs, walking)

		return
	}

	s.chase(e, deltaMs)
}

// closeWith swings at a target that is already within reach.
//
// Nothing is sent while the character is still walking, for the reason the
// pick-up learned: the client reaches a cell before word of it reaches the
// server, so a blow measured from where the client thinks it stands is
// refused.
func (s *InGameState) closeWith(e *entity.Entity, deltaMs float32, walking bool) {
	s.repathMs = 0

	if walking {
		return
	}

	s.resendMs -= deltaMs
	if s.attacking && s.resendMs > 0 {
		return
	}

	s.sendAttack(e)
	s.attacking = true
	s.resendMs = attackResendMs
}

// chase walks towards a target that is out of reach, reissuing the route as
// it moves.
func (s *InGameState) chase(e *entity.Entity, deltaMs float32) {
	// Out of reach means the server has stopped swinging, whatever it was
	// last told.
	s.attacking = false

	s.repathMs -= deltaMs
	if s.repathMs > 0 {
		return
	}
	s.repathMs = targetRepathMs

	// Walking onto the target's own cell is not possible and not the point —
	// the server's pathing stops on the last free cell before it, which is
	// where a melee blow lands from.
	cellX, cellY := e.Body.CurrentCell()

	trace.Emit(trace.HUD, "attack-approach",
		zap.Uint32("id", e.ID), zap.String("name", e.Name),
		zap.Int("x", cellX), zap.Int("y", cellY))

	if err := s.RequestMove(cellX, cellY); err != nil {
		logger.Warn("could not walk to that target", zap.Error(err))
	}
}

// sendAttack asks for the blow.
//
// The character turns to face what it is hitting first. That happens on the
// request rather than on the server's answer, for the same reason reaching
// for an item does: it is feedback, and a turn that arrives a round trip
// after the swing reads as the character hitting over its shoulder.
func (s *InGameState) sendAttack(e *entity.Entity) {
	s.faceTowards(s.selfAID(), e.ID)

	trace.Emit(trace.HUD, "attack",
		zap.Uint32("id", e.ID), zap.String("name", e.Name),
		zap.Int("hp", e.HP), zap.Int("maxHP", e.MaxHP))

	if err := s.client.Send(packets.EncodeAttack(e.ID, true)); err != nil {
		logger.Warn("attack failed", zap.Error(err))
	}
}

// AttackNearest picks a fight with the closest monster in view, and reports
// whether it found one.
//
// For unattended runs. Combat otherwise begins with a click on a monster, and
// nothing about a blow can be watched without a hand to click with.
func (s *InGameState) AttackNearest() bool {
	if s.entityManager == nil || s.player == nil {
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

	trace.Emit(trace.HUD, "attack-nearest",
		zap.Uint32("aid", nearest.ID), zap.String("name", nearest.Name))

	s.AttackTarget(nearest)

	return true
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
	reach := s.reach()

	return abs(targetX-playerX) <= reach && abs(targetY-playerY) <= reach
}

// reach is how far the character can strike, in cells.
//
// Whatever the server last said, which is the figure battle_check_range will
// measure the blow against. Stopping short of it is what keeps the character
// from walking onto whatever it is fighting, and a weapon that reaches makes
// the difference much larger than one cell.
func (s *InGameState) reach() int {
	if s.attackRange > 0 {
		return s.attackRange
	}

	return defaultAttackRange
}

// handleAttackRange takes the reach of the equipped weapon.
func (s *InGameState) handleAttackRange(data []byte) error {
	reach, ok := packets.DecodeAttackRange(data)
	if !ok {
		logger.Warn("short attack range packet", zap.Int("len", len(data)))

		return nil
	}

	// A nonsense figure is ignored rather than believed: standing off at a
	// range the server will refuse is worse than standing too close.
	if reach < 1 || reach > 20 {
		logger.Warn("ignoring an implausible attack range", zap.Int("range", reach))

		return nil
	}

	s.attackRange = reach
	trace.Emit(trace.HUD, "attack-range", zap.Int("cells", reach))

	return nil
}

// forgetAttack drops the target and any errand to reach it.
//
// Clicking somewhere else breaks off the fight. The server stops swinging
// when it is told to walk, so the walk request that follows is what actually
// cancels it — this is the client agreeing rather than the client deciding.
func (s *InGameState) forgetAttack() {
	if s.targetID == 0 {
		return
	}

	trace.Emit(trace.HUD, "attack-canceled", zap.Uint32("id", s.targetID))
	s.targetID = 0
	s.attacking = false
	s.repathMs = 0
	s.resendMs = 0
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

	// The packet carries gestures as well as blows: picking something up,
	// sitting, standing. Each is broadcast so everyone nearby sees it, and
	// reading them as damage made a character swing at the ground it had just
	// taken a potato off.
	if !blow.IsBlow() {
		s.playGesture(blow)

		return nil
	}

	// The two numbers to watch: how long the swing takes, which is the attack
	// motion, and how far into it the blow lands. A zero delay means the swing
	// and its outcome went out together, which is the fault this replaced.
	swing := blow.SwingDurationMs()
	delay := s.hitDelayMs(blow.SourceID, swing)

	trace.Emit(trace.HUD, "damage",
		zap.Uint32("from", blow.SourceID), zap.Uint32("to", blow.TargetID),
		zap.Int("amount", blow.Amount), zap.Int("hits", blow.Hits),
		zap.Bool("miss", blow.Missed()), zap.Bool("critical", blow.Critical()),
		zap.Int("srcSpeed", blow.SourceSpeed), zap.Float32("swingMs", swing),
		zap.Float32("hitDelayMs", delay))

	// Both sides turn to face each other. The server says nothing about which
	// way anyone is looking when a blow lands, so without this a monster that
	// walked past and then attacked would strike sideways, and a character
	// whose target circled it would go on swinging at where it used to be.
	s.faceTowards(blow.SourceID, blow.TargetID)

	// The swing starts now — it is the answer to a blow being thrown, and a
	// swing that waits reads as a character that hesitated.
	//
	// What the swing does waits for the swing to get there. An animation is
	// not instantaneous and its outcome does not belong at its start: a
	// Swordman's sword takes nine frames and lands on the sixth, so a figure,
	// a flinch and a death shown at once resolve the exchange half a second
	// before the blade arrives.
	s.playAttackAnimation(blow.SourceID, swing)

	if delay > 0 {
		s.pendingBlows = append(s.pendingBlows, pendingBlow{blow: blow, remainingMs: delay})

		return nil
	}

	s.landBlow(pendingBlow{blow: blow})

	return nil
}

// pendingBlow is what a swing does, waiting for the swing to reach the frame
// where it does it.
type pendingBlow struct {
	blow packets.Damage

	// remainingMs counts down to the hit frame.
	remainingMs float32

	// kills marks a blow the target did not survive. The server says so in a
	// packet of its own that arrives while the sword is still traveling, and
	// a unit that falls over then is a unit that died before it was hit.
	kills bool
}

// hitDelayMs is how far into a swing of the given length the blow lands.
//
// The attacker's own sprite says where in the animation that is; the share of
// the whole it represents is what survives the swing being sped up or slowed
// down. A sheet that has not been baked answers zero, which lands the blow at
// once — the right answer when there is nothing on screen to wait for.
func (s *InGameState) hitDelayMs(attacker uint32, durationMs float32) float32 {
	if s.playerRender == nil || attacker == 0 || durationMs <= 0 {
		return 0
	}

	var frame, frames int

	if attacker == s.selfAID() {
		if s.player == nil {
			return 0
		}

		frame = s.playerRender.HitFrame(entity.ActionAttack)
		frames = s.playerRender.FrameCount(entity.ActionAttack, s.player.Direction)
	} else {
		if s.entityManager == nil {
			return 0
		}

		e := s.entityManager.Get(attacker)
		if e == nil || e.Body == nil || !unitIsDrawable(e) {
			return 0
		}

		spec := unitSpec(e)
		frame = s.playerRender.UnitHitFrame(spec, entity.ActionAttack)
		frames = s.playerRender.UnitFrameCount(spec, entity.ActionAttack, e.Body.Direction)
	}

	if frames <= 0 || frame <= 0 {
		return 0
	}

	return durationMs * float32(frame) / float32(frames)
}

// hitSound is the sound the attacker's swing plays where it lands.
func (s *InGameState) hitSound(attacker uint32) string {
	if s.playerRender == nil || attacker == 0 {
		return ""
	}

	if attacker == s.selfAID() {
		return s.playerRender.HitSound(entity.ActionAttack)
	}

	if s.entityManager == nil {
		return ""
	}

	e := s.entityManager.Get(attacker)
	if e == nil || !unitIsDrawable(e) {
		return ""
	}

	return s.playerRender.UnitHitSound(unitSpec(e), entity.ActionAttack)
}

// landBlow applies what a swing did, at the moment it did it.
func (s *InGameState) landBlow(p pendingBlow) {
	blow := p.blow

	s.addDamageNumber(blow)

	// The sprite names the sound on the frame the blow lands, which is the
	// same frame as everything else here.
	if sound := s.hitSound(blow.SourceID); sound != "" {
		s.soundRequest = worldSoundDir + sound
	}

	if !blow.Missed() {
		s.faceTowards(blow.TargetID, blow.SourceID)
		s.playAnimation(blow.TargetID, entity.ActionHurt)
	}

	if p.kills {
		s.killUnit(blow.TargetID)
	}
}

// advancePendingBlows lands every swing that has reached its hit frame.
func (s *InGameState) advancePendingBlows(deltaMs float32) {
	if len(s.pendingBlows) == 0 {
		return
	}

	kept := s.pendingBlows[:0]
	for _, pending := range s.pendingBlows {
		pending.remainingMs -= deltaMs
		if pending.remainingMs > 0 {
			kept = append(kept, pending)

			continue
		}

		s.landBlow(pending)
	}

	s.pendingBlows = kept
}

// killDeferred marks a target's blow in flight as fatal, and reports whether
// there was one.
//
// The server sends the death the moment it decides it, which is while the
// sword that caused it is still on its way. Holding it until the blow lands
// is what keeps a monster on its feet until it is hit.
func (s *InGameState) killDeferred(aid uint32) bool {
	// The last one queued, since that is the blow that killed it. Attaching
	// the death to an earlier one would drop the target on a hit it survived
	// and leave the fatal blow landing on a corpse.
	for i := len(s.pendingBlows) - 1; i >= 0; i-- {
		if s.pendingBlows[i].blow.TargetID == aid {
			s.pendingBlows[i].kills = true

			return true
		}
	}

	return false
}

// forgetPendingBlows drops every blow in flight.
//
// A map change or a death takes away everything they were going to happen to,
// and a figure floating over a monster on the last map is worse than one that
// never appeared.
func (s *InGameState) forgetPendingBlows() {
	s.pendingBlows = nil
}

// playGesture plays one of the things ZC_NOTIFY_ACT carries that is not a
// blow.
func (s *InGameState) playGesture(blow packets.Damage) {
	switch blow.Type {
	case packets.ActPickupItem:
		trace.Emit(trace.HUD, "gesture-pickup",
			zap.Uint32("who", blow.SourceID), zap.Uint32("item", blow.TargetID))

		s.faceTowards(blow.SourceID, blow.TargetID)

		if body := s.bodyOf(blow.SourceID); body != nil {
			body.PlayPickup()
		}

	case packets.ActSitDown, packets.ActStandUp:
		// The server's answer, not the key that asked: sitting can be refused
		// — a character whose Basic Skill is below three cannot — and this
		// packet is the only thing that says it happened.
		sitting := blow.Type == packets.ActSitDown

		trace.Emit(trace.HUD, "gesture-sit",
			zap.Uint32("who", blow.SourceID), zap.Bool("sitting", sitting))

		if body := s.bodyOf(blow.SourceID); body != nil {
			body.Sitting = sitting
		}
	}
}

// ToggleSit sits the character down, or stands it back up.
//
// One key for both, as the original has it: whether it means sit or stand is
// read off what the character is doing now. Nothing changes here — the server
// answers with ZC_NOTIFY_ACT and that is what moves the sprite, because the
// request can be refused and a character sitting on a refusal would be seated
// in a game where it is standing.
func (s *InGameState) ToggleSit() error {
	if s.client == nil || s.player == nil {
		return nil
	}

	action := packets.ActionSit
	if s.player.Sitting {
		action = packets.ActionStand
	}

	trace.Emit(trace.HUD, "toggle-sit", zap.Bool("sitting", s.player.Sitting))

	return s.client.Send(packets.EncodeAction(s.selfAID(), action))
}

// Sitting reports whether the character is seated.
func (s *InGameState) Sitting() bool {
	return s.player != nil && s.player.Sitting
}

// playAttackAnimation starts a swing that takes as long as the attack motion.
func (s *InGameState) playAttackAnimation(id uint32, durationMs float32) {
	if body := s.bodyOf(id); body != nil {
		body.PlayAttackFor(durationMs)
	}
}

// playAnimation starts a one-shot on whichever unit the id names, including
// the player.
//
// The player is not in the entity manager as something to animate — their
// body is held separately — so both are tried. A blow between two units we
// cannot see names neither, and nothing happens, which is correct: there is
// nothing on screen to animate.
func (s *InGameState) playAnimation(id uint32, action int) {
	if body := s.bodyOf(id); body != nil {
		body.PlayOnce(action)
	}
}

// bodyOf is the drawable body behind a unit id, ours or anyone's.
func (s *InGameState) bodyOf(id uint32) *entity.Character {
	if id == 0 {
		return nil
	}

	if id == s.selfAID() {
		return s.player
	}

	if s.entityManager == nil {
		return nil
	}

	if e := s.entityManager.Get(id); e != nil {
		return e.Body
	}

	return nil
}

// faceTowards turns a unit to look at whatever just hit it.
//
// The server does not say which way a unit is facing when it is struck, and a
// monster that walked past you and then took a blow to the back of the head
// reads as a bug rather than as a monster that has not turned round yet.
func (s *InGameState) faceTowards(id, towards uint32) {
	body, at := s.bodyOf(id), s.bodyOf(towards)
	if body == nil || at == nil {
		return
	}

	// A unit in the middle of a walk is facing the way it is going, and the
	// path will overwrite this on the next step anyway. Turning it here only
	// makes it flicker between the two.
	if body.IsWalkingPath() {
		return
	}

	fromX, fromY := body.CurrentCell()
	toX, toY := at.CurrentCell()

	if dir := entity.DirectionFromCellDelta(toX-fromX, toY-fromY); dir >= 0 {
		body.Direction = dir
	}
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

// TargetMarker is where to draw the mark over the unit being fought, in
// viewport pixels.
type TargetMarker struct {
	ScreenX, ScreenY float32
}

// TargetMarker is the mark over the target's head, or nothing when there is
// no target on screen.
//
// Above the head rather than at the feet, which is where the original puts
// it and where it does not fight the HP bar and the name for the same pixels.
func (s *InGameState) TargetMarker(viewportW, viewportH float32) (TargetMarker, bool) {
	if s.targetID == 0 || s.entityManager == nil {
		return TargetMarker{}, false
	}

	e := s.entityManager.Get(s.targetID)
	if e == nil || e.Body == nil {
		return TargetMarker{}, false
	}

	// The top of the box the unit is drawn in, so the mark sits over a poring
	// and over something tall alike.
	box := s.unitBox(e)

	x, y := s.projectToScreen(e.Body.RenderX, box.Max[1], e.Body.RenderZ,
		viewportW, viewportH)
	if x < 0 {
		return TargetMarker{}, false
	}

	return TargetMarker{ScreenX: x, ScreenY: y}, true
}
