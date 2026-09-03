package states

import (
	"math"

	"go.uber.org/zap"

	"github.com/Faultbox/midgard-ro/internal/trace"
)

// Effects the original draws in code rather than from a file.
//
// Most of RO's skill effects are not STR animations. They are particle bursts
// the client generates: a handful of textured quads thrown outward from a
// point, each with its own size, speed and life, fading as they go. There is
// nothing in the archive to read them out of — only the textures they are made
// of — so the shape of each one has to be described.
//
// The numbers below are nostalro-client's, which derived them from the
// original; the file each came from is named where it is used. What is not
// theirs is the space: theirs are billboards in the world, these are quads
// around a projected point, the same way the STR effects this client already
// draws are. For a burst that lasts half a second at a fixed camera the two
// look alike, and it costs no renderer this client does not have.

// burstScale converts nostalro's world units into screen pixels.
//
// Their effects are measured in the game's own units, where a cell is five.
// At this client's default camera a cell is about thirty pixels across, so a
// unit is six.
const burstScale = float32(6)

// burstFPS is the rate every number in this file is counted at. The original
// ran at sixty frames a second and its effects are written as per-frame steps,
// so speeds and growths are per frame and lives are in frames.
const burstFPS = float32(60)

// burstFrames converts a count of the original's frames into milliseconds.
func burstFrames(n float32) float32 {
	return n / burstFPS * 1000
}

// burstParticle is one quad of a burst.
type burstParticle struct {
	// Where it starts and where it is going, in pixels from the burst's
	// anchor. Screen space: up is negative.
	x, y   float32
	vx, vy float32

	// ay is the acceleration downward, for a particle that rises and slows.
	ay float32

	// angle is its rotation and spin how fast that turns, in radians per
	// frame; spinAccel slows the turn, which is what makes a burst of rays
	// fan out and then settle rather than keep spinning.
	angle, spin, spinAccel float32

	// halfW and halfH are its size, growing by growW and growH each frame.
	// They are half-sizes because a quad is drawn around its center, and the
	// rays of a burst straddle the point they radiate from.
	halfW, halfH float32
	growW, growH float32

	// birthMs is when it appears and lifeMs how long it lasts.
	birthMs, lifeMs float32

	// fadeInMs is how long it takes to reach maxAlpha, fadeOutMs how long it
	// takes to leave at the end. With no fadeOutMs it fades over its whole
	// life instead, which is what a thrown shard does.
	fadeInMs, fadeOutMs float32
	maxAlpha            float32

	texture string
	tint    [3]float32

	// additive is how nearly every effect in RO is drawn — the exceptions are
	// the ones that darken as well as brighten, like Bash's halo.
	additive bool
}

// alphaAt is how strongly the particle draws at an age, and zero once it has
// gone.
func (p burstParticle) alphaAt(age float32) float32 {
	if age < 0 || age >= p.lifeMs {
		return 0
	}

	peak := p.maxAlpha
	if peak <= 0 {
		peak = 1
	}

	alpha := peak
	if p.fadeInMs > 0 && age < p.fadeInMs {
		alpha = peak * age / p.fadeInMs
	}

	// Out at the end if it says when, otherwise over the whole life so that
	// nothing vanishes mid-flight.
	out := peak * (1 - age/p.lifeMs)
	if p.fadeOutMs > 0 {
		out = peak

		if start := p.lifeMs - p.fadeOutMs; age > start {
			out = peak * (p.lifeMs - age) / p.fadeOutMs
		}
	}

	if out < alpha {
		alpha = out
	}

	return alpha
}

// activeBurst is one playing.
type activeBurst struct {
	parts []burstParticle

	// Where it plays, in world space, projected each frame so it stays on the
	// unit it was aimed at even as the camera turns.
	x, y, z float32

	ageMs float32
	runMs float32
}

// playBurst starts one over a world position.
func (s *InGameState) playBurst(name string, parts []burstParticle, runMs, x, y, z float32) {
	if len(parts) == 0 {
		return
	}

	trace.Emit(trace.HUD, "burst-play", zap.String("effect", name), zap.Int("parts", len(parts)))

	s.bursts = append(s.bursts, &activeBurst{parts: parts, x: x, y: y, z: z, runMs: runMs})
}

// advanceBursts ages them and drops what has finished.
func (s *InGameState) advanceBursts(deltaMs float32) {
	if len(s.bursts) == 0 {
		return
	}

	kept := s.bursts[:0]
	for _, b := range s.bursts {
		b.ageMs += deltaMs
		if b.ageMs < b.runMs {
			kept = append(kept, b)
		}
	}

	s.bursts = kept
}

// burstQuads projects what is playing into viewport quads.
func (s *InGameState) burstQuads(viewportW, viewportH float32) []EffectQuad {
	if len(s.bursts) == 0 || s.scene == nil || !s.SceneReady {
		return nil
	}

	var out []EffectQuad

	for _, b := range s.bursts {
		originX, originY := s.projectToScreen(b.x, b.y, b.z, viewportW, viewportH)
		if originX < 0 {
			continue
		}

		out = append(out, b.quadsAt(originX, originY)...)
	}

	return out
}

// quadsAt lays a burst's particles out around a point on the screen.
//
// Apart from the projection this is the whole of what a burst is, which is
// why it is here rather than in the loop above: the projection needs a scene
// and a camera, and the shape of an effect over time can be looked at without
// either.
func (b *activeBurst) quadsAt(originX, originY float32) []EffectQuad {
	out := make([]EffectQuad, 0, len(b.parts))

	for _, p := range b.parts {
		age := b.ageMs - p.birthMs

		alpha := p.alphaAt(age)
		if alpha <= 0 {
			continue
		}

		// Frames rather than milliseconds: the numbers came from a
		// sixty-a-second original and read as steps per frame.
		frames := age / 1000 * burstFPS

		x := originX + p.x + p.vx*frames
		y := originY + p.y + p.vy*frames + 0.5*p.ay*frames*frames

		halfW := p.halfW + p.growW*frames
		halfH := p.halfH + p.growH*frames
		if halfW <= 0 || halfH <= 0 {
			continue
		}

		// The spin decelerates, so the angle is the integral of it
		// rather than a rate times a time.
		angle := p.angle + p.spin*frames + p.spinAccel*frames*(frames+1)/2

		out = append(out, EffectQuad{
			Texture:  p.texture,
			Corners:  quadCorners(x, y, halfW, halfH, angle),
			UV:       [4][2]float32{{0, 0}, {1, 0}, {1, 1}, {0, 1}},
			Color:    [4]float32{p.tint[0], p.tint[1], p.tint[2], alpha},
			Additive: p.additive,
		})
	}

	return out
}

// quadCorners lays a quad of a given half-size around a point, turned by an
// angle.
func quadCorners(x, y, halfW, halfH, angle float32) [4][2]float32 {
	if angle == 0 {
		return [4][2]float32{
			{x - halfW, y - halfH},
			{x + halfW, y - halfH},
			{x + halfW, y + halfH},
			{x - halfW, y + halfH},
		}
	}

	sin, cos := math.Sincos(float64(angle))
	s32, c32 := float32(sin), float32(cos)

	var out [4][2]float32
	for i, corner := range [4][2]float32{{-halfW, -halfH}, {halfW, -halfH}, {halfW, halfH}, {-halfW, halfH}} {
		out[i] = [2]float32{
			x + corner[0]*c32 - corner[1]*s32,
			y + corner[0]*s32 + corner[1]*c32,
		}
	}

	return out
}

// hash01 is the spread these bursts are built with: the same particle index
// always lands in the same place, so a burst does not shimmer differently
// every time it is played.
func hash01(i, salt uint32) float32 {
	x := i*2654435761 + salt*40503 + 0x9E3779B9
	x ^= x >> 15

	return float32(x%100000) / 100000
}

// coldHitParts is EF_COLDHIT: ice shards thrown out of the target with two
// puffs of smoke behind them. From nostalro-client's coldhit.rs — nine shards
// of lens1.tga tinted towards white-blue, and smoke.tga growing behind.
func coldHitParts() ([]burstParticle, float32) {
	const shards = 9

	tint := [3]float32{0.88, 0.94, 1.0}
	parts := make([]burstParticle, 0, shards+2)

	for i := 0; i < shards; i++ {
		angle := 2 * math.Pi * float64(hash01(uint32(i), 1))
		speed := 1.5 + 2.5*hash01(uint32(i), 2)

		parts = append(parts, burstParticle{
			vx:       float32(math.Cos(angle)) * speed * burstScale * 0.2,
			vy:       float32(math.Sin(angle)) * speed * burstScale * 0.2,
			halfW:    0.5 * burstScale,
			halfH:    (1.5 + hash01(uint32(i), 3)) * burstScale,
			birthMs:  burstFrames(hash01(uint32(i), 4) * 4),
			lifeMs:   burstFrames(15),
			fadeInMs: burstFrames(1),
			texture:  "lens1.tga",
			tint:     tint,
			additive: true,
		})
	}

	// Two puffs, born a little apart, growing where the shards left.
	for i, birth := range [2]float32{0, 7} {
		parts = append(parts, burstParticle{
			x:        (hash01(uint32(i), 5) - 0.5) * burstScale,
			halfW:    1.0 * burstScale,
			halfH:    1.0 * burstScale,
			growW:    0.5 * burstScale,
			growH:    0.5 * burstScale,
			birthMs:  burstFrames(birth),
			lifeMs:   burstFrames(25),
			fadeInMs: burstFrames(3),
			texture:  "smoke.tga",
			tint:     [3]float32{0.8, 0.88, 1.0},
			additive: true,
		})
	}

	return parts, 550
}

// hit2Parts is EF_HIT2: petals thrown upward out of whatever was hit, rising
// and slowing. From nostalro-client's hit2.rs — eight of them, sized and sped
// at random within its ranges, off lens1 and lens2.
func hit2Parts() ([]burstParticle, float32) {
	const (
		petals = 8
		sizeF  = 1.0 / 3
	)

	parts := make([]burstParticle, 0, petals)
	for i := 0; i < petals; i++ {
		speed := 0.5 + 4.5*hash01(uint32(i), 11)
		angle := 2 * math.Pi * float64(hash01(uint32(i), 12))
		radius := 5 * sizeF * hash01(uint32(i), 13) * burstScale

		texture := "lens1.tga"
		if i%2 == 1 {
			texture = "lens2.tga"
		}

		parts = append(parts, burstParticle{
			x:        float32(math.Cos(angle)) * radius,
			y:        float32(math.Sin(angle)) * radius,
			vy:       -speed * burstScale * 0.25,
			ay:       0.25 * burstScale * 0.05,
			halfW:    (5 + 15*hash01(uint32(i), 14)) * sizeF * burstScale / 2,
			halfH:    (20 + 20*hash01(uint32(i), 15)) * sizeF * burstScale / 2,
			lifeMs:   burstFrames(10 + 20*hash01(uint32(i), 16)),
			fadeInMs: burstFrames(8),
			texture:  texture,
			tint:     [3]float32{1, 1, 1},
			additive: true,
		})
	}

	return parts, 500
}

// bashParts is EF_BASH: the flash a heavy blow lands with. From
// nostalro-client's bash.rs and the spike_burst.rs it shares with HasteUp and
// Flasher — two haloes of alpha_down.tga over twenty rays of alpha_center.tga
// that lengthen as they turn, and slow as they turn.
//
// Drawn with ordinary alpha rather than additively, which is the one thing
// that separates it from the rest of this file: added, the haloes wash the
// target out instead of framing it.
func bashParts() ([]burstParticle, float32) {
	const (
		runFrames  = 40
		spikes     = 20
		spikeAlpha = 200.0 / 255
	)

	haloTint := [3]float32{1.0, 0.95, 0.75}
	parts := make([]burstParticle, 0, 2+spikes)

	// The wide halo first and the tight bright one over it.
	for _, halo := range [2]struct{ radius, alpha float32 }{
		{9.0, 130.0 / 255},
		{3.5, 220.0 / 255},
	} {
		parts = append(parts, burstParticle{
			halfW:     halo.radius * burstScale,
			halfH:     halo.radius * burstScale,
			lifeMs:    burstFrames(runFrames),
			fadeInMs:  burstFrames(6),
			fadeOutMs: burstFrames(10),
			maxAlpha:  halo.alpha,
			texture:   "alpha_down.tga",
			tint:      haloTint,
		})
	}

	for i := 0; i < spikes; i++ {
		// Degrees a frame in the original, turned the other way: its screen
		// space counts angles the opposite way round from this one.
		spinDeg := -(1 + 6*hash01(uint32(i), 21))
		spin := float32(float64(spinDeg) * math.Pi / 180)

		parts = append(parts, burstParticle{
			angle: 2 * math.Pi * hash01(uint32(i), 22),
			spin:  spin,

			// The original decelerates a ray to a stop over one and a half
			// times the burst, so it is still drifting when it goes out.
			spinAccel: -spin / runFrames / 1.5,

			// A ray straddles the point it comes from: the texture is
			// brightest along its middle, so the quad is its full length and
			// the anchor is halfway along.
			halfW:  0.5 / 2 * burstScale,
			halfH:  (2.8 + 2.8*hash01(uint32(i), 23)) * burstScale,
			growH:  (0.28 + 0.42*hash01(uint32(i), 24)) * burstScale,
			lifeMs: burstFrames(runFrames),

			fadeInMs:  burstFrames(10),
			fadeOutMs: burstFrames(runFrames / 3),
			maxAlpha:  spikeAlpha,

			texture: "alpha_center.tga",
			tint:    [3]float32{1, 1, 1},
		})
	}

	return parts, burstFrames(runFrames)
}

// burstFor is the burst a named effect plays, and whether there is one.
func burstFor(effect string) (parts []burstParticle, runMs float32, ok bool) {
	switch effect {
	case "EF_COLDHIT":
		parts, runMs = coldHitParts()
	case "EF_HIT2":
		parts, runMs = hit2Parts()
	case "EF_BASH":
		parts, runMs = bashParts()
	default:
		return nil, 0, false
	}

	return parts, runMs, true
}
