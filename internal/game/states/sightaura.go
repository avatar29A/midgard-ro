package states

import (
	"math"

	"github.com/Faultbox/midgard-ro/internal/network/packets"
)

// Sight, which is not a skill effect at all.
//
// The table names nothing for it, and that is not an omission: the original
// draws Sight off the option word rather than off the cast. The spell sets
// OPTION_SIGHT on whoever cast it and the client draws an aura for as long as
// the bit is set, which is why a Sight that is still running is still drawn
// when a character walks back into view, and why nobody has to be told how
// long it lasts.
//
// What it draws is a light circling the caster, leaving a trail behind it. The
// trail is not one thing moving: it is a light dropped every couple of frames
// and left to fade where it fell, so what turns is the point they are being
// dropped at.
//
// The numbers are nostalro-client's, from effects/sight.rs.

// sightSprite is the light. Five frames, sixty-four pixels square, of which
// the lit part is about half.
const sightSprite = `data\sprite\이팩트\sight.spr`

const (
	// sightRadius is how far out from the caster it circles, in world units.
	// Three cells: Sight lights up what is around a character rather than
	// what is on them.
	sightRadius = 15.0

	// sightHeight is how far above the ground the lights ride. Around a
	// character rather than over them: higher and the ring reads as hanging
	// in the air beside whoever cast it.
	sightHeight = 10.0

	// sightTurnPerFrame is how far round the drop point moves each frame, in
	// degrees. Negative, so it circles the way the original's does.
	sightTurnPerFrame = -5.0

	// sightDropFrames is how often a light is dropped and sightTrail how many
	// of them are alight at once. Twenty frames of life at one every two
	// frames is ten.
	sightDropFrames = 2
	sightTrail      = 10

	// sightHalf is a light's half-size in world units when it is dropped, and
	// sightShrink how much of that it has lost by the time it goes out.
	//
	// Big, because the art is mostly the space around the light: about half
	// of sight.spr's sixty-four pixels are lit at all, and the bright part of
	// that is smaller again. A light sized to the flame draws a spark.
	sightHalf   = 9.0
	sightShrink = 0.55

	// sightAlpha is how strongly the newest light draws. The original's is
	// a hundred and fifty of two hundred and fifty-five.
	sightAlpha = 200.0 / 255
)

// advanceSightAuras turns the aura.
//
// Its own clock rather than the world's, because there is no world clock: each
// of these systems is handed the frame's delta and keeps what it needs.
func (s *InGameState) advanceSightAuras(deltaMs float32) {
	if len(s.sightingUnits()) == 0 {
		// Back to the start when nobody is sighting, so an aura does not
		// begin part way round because somebody cast one an hour ago.
		s.sightMs = 0

		return
	}

	s.sightMs += deltaMs
}

// sightingUnits are the ones with Sight or Ruwach running this frame.
//
// Looked up each frame, the way the ice over a frozen unit is: the aura
// follows whoever it belongs to and goes the moment the server clears the bit.
func (s *InGameState) sightingUnits() []uint32 {
	var sighting []uint32

	if s.playerOptions&(packets.OptionSight|packets.OptionRuwach) != 0 {
		if aid := s.selfAID(); aid != 0 {
			sighting = append(sighting, aid)
		}
	}

	if s.entityManager == nil {
		return sighting
	}

	for _, e := range s.entityManager.All() {
		if e.Options&(packets.OptionSight|packets.OptionRuwach) != 0 && !e.IsDead {
			sighting = append(sighting, e.ID)
		}
	}

	return sighting
}

// sightTrailParts is the trail as it stands at an age, oldest light last.
//
// Laid out where each light is now rather than played through: they were
// dropped at different moments and have been fading for different lengths of
// time, which is a position and a strength each, and none of them moves after
// it lands.
func sightTrailParts(ageMs float32) []burstParticle {
	frames := ageMs / 1000 * burstFPS

	parts := make([]burstParticle, 0, sightTrail)

	for i := 0; i < sightTrail; i++ {
		// Where the drop point was that many frames ago.
		at := frames - float32(i*sightDropFrames)
		angle := float64(at*sightTurnPerFrame) * math.Pi / 180

		x := sightRadius * float32(math.Cos(angle))
		z := sightRadius * float32(math.Sin(angle))

		// How far through its life this one is, nought for the newest.
		gone := float32(i) / sightTrail

		parts = append(parts, burstParticle{
			x: x, y: sightHeight, z: z,

			halfW: sightHalf * (1 - sightShrink*gone),
			halfH: sightHalf * (1 - sightShrink*gone),

			// Drawn as it is now rather than aged: the age is in where it
			// sits and how faint it is, which is worked out here.
			lifeMs:   1,
			maxAlpha: sightAlpha * (1 - gone),

			texture:  spriteFrameKey(sightSprite, 0),
			tint:     [3]float32{1, 1, 1},
			additive: true,
		})
	}

	return parts
}

// SightQuads is the aura around whoever is sighting.
func (s *InGameState) SightQuads(viewportW, viewportH float32) []EffectQuad {
	if s.scene == nil || !s.SceneReady {
		return nil
	}

	sighting := s.sightingUnits()
	if len(sighting) == 0 {
		return nil
	}

	parts := sightTrailParts(s.sightMs)

	var out []EffectQuad

	for _, id := range sighting {
		x, y, z, ok := s.footingOf(id)
		if !ok {
			continue
		}

		turning := &activeBurst{parts: parts, x: x, y: y, z: z}

		out = append(out, s.burstQuadsOf(turning, viewportW, viewportH)...)
	}

	return out
}
