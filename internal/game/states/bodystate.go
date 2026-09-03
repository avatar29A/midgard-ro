package states

import (
	"go.uber.org/zap"

	"github.com/Faultbox/midgard-ro/internal/logger"
	"github.com/Faultbox/midgard-ro/internal/network/packets"
	"github.com/Faultbox/midgard-ro/internal/trace"
)

// The states a unit is drawn in.
//
// Petrified, frozen, stunned and asleep are not icons beside a health bar.
// The original draws them on the unit itself, and the one this client needs
// first is the ice a frozen target is sealed in: Frost Diver throws its line
// of spikes, the target freezes, and without this the ice is a moment of
// effect and then nothing, when it should stand for as long as the freeze
// lasts.
//
// Only the state is followed here, not how long it has left. The server never
// says: it sends the state when it starts and again when it ends, and a
// client that guessed a duration would thaw a target the server still has
// frozen.

// handleStateChange is a unit's states changing.
func (s *InGameState) handleStateChange(data []byte) error {
	change, ok := packets.DecodeStateChange(data)
	if !ok {
		logger.Warn("short state change packet", zap.Int("len", len(data)))

		return nil
	}

	e := s.entityOf(change.AID)
	if e == nil {
		// Ours, or somebody not in view. The character being played is not in
		// the registry, so its own states are kept beside it.
		if change.AID == s.selfAID() {
			s.playerBodyState = change.Body
		}

		return nil
	}

	if e.BodyState != change.Body {
		trace.Emit(trace.HUD, "body-state",
			zap.Uint32("aid", change.AID), zap.String("name", e.Name),
			zap.Uint16("was", e.BodyState), zap.Uint16("now", change.Body))
	}

	e.BodyState = change.Body

	return nil
}

// frozenUnits are the ones sealed in ice this frame, with where they stand.
//
// Looked up each frame rather than remembered, so the ice follows a unit that
// is pushed and goes the moment the server says the freeze is over — or the
// moment the unit does.
func (s *InGameState) frozenUnits() []uint32 {
	var frozen []uint32

	if s.playerBodyState == packets.BodyFreeze {
		if aid := s.selfAID(); aid != 0 {
			frozen = append(frozen, aid)
		}
	}

	if s.entityManager == nil {
		return frozen
	}

	for _, e := range s.entityManager.All() {
		if e.BodyState == packets.BodyFreeze && !e.IsDead && e.Body != nil {
			frozen = append(frozen, e.ID)
		}
	}

	return frozen
}

// IceQuads are the ice standing over whoever is frozen.
//
// The same spikes Frost Diver seals a target in, held at their full height
// rather than played through: this is a state that lasts rather than a thing
// that happens, so the ice stands still until the freeze is lifted.
func (s *InGameState) IceQuads(viewportW, viewportH float32) []EffectQuad {
	if s.scene == nil || !s.SceneReady {
		return nil
	}

	frozen := s.frozenUnits()
	if len(frozen) == 0 {
		return nil
	}

	seal := frostDiver2Parts()

	var out []EffectQuad
	for _, id := range frozen {
		x, y, z, ok := s.footingOf(id)
		if !ok {
			continue
		}

		// Held at the moment the spikes have finished growing: past that they
		// would begin to fade, and this ice is not going anywhere.
		held := &activeBurst{
			parts: seal.parts,
			x:     x, y: y, z: z,
			ageMs: frostDiverReachMs() + burstFrames(spikeRiseFrames),
		}

		out = append(out, s.burstQuadsOf(held, viewportW, viewportH)...)
	}

	return out
}
