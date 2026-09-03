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

	// atOther is where along the burst's own line the particle sits: nought
	// at the anchor, one at the other end. A burst with no other end has all
	// of them at nought, which is a burst around a point.
	atOther float32

	// fallsIn makes it travel from there to the anchor over its life instead
	// of staying put, turned to lie along the way it is going — which is what
	// makes a bolt read as falling rather than sliding.
	fallsIn bool

	// jitter shifts where it starts, so ten of them along the same line do
	// not sit on top of each other.
	jitter [2]float32
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

// burstSpec is a burst before it is played: its particles, how long the whole
// thing runs, and where its other end is.
//
// Some effects are drawn between two places rather than around one — a bolt
// falls out of the sky onto what it hits, Frost Diver walks a line of spikes
// from the caster to the target. The other end is a world offset from the
// anchor rather than a screen one because it has to be projected: a bolt that
// dropped a fixed number of pixels would come down at the wrong angle the
// moment the camera turned, and a line of spikes would point the wrong way.
type burstSpec struct {
	parts []burstParticle
	runMs float32

	otherX, otherY, otherZ float32

	// fromCaster says the other end is wherever the caster is standing, which
	// is not a fixed offset and is filled in when the burst is played.
	fromCaster bool
}

// activeBurst is one playing.
type activeBurst struct {
	parts []burstParticle

	// otherX, otherY and otherZ are the burst's other end, as a world offset
	// from the anchor.
	otherX, otherY, otherZ float32

	// Where it plays, in world space, projected each frame so it stays on the
	// unit it was aimed at even as the camera turns.
	x, y, z float32

	ageMs float32
	runMs float32

	// traced marks that this burst's first drawn frame has been reported, so
	// the trace says where it landed once rather than sixty times a second.
	// The first drawn frame rather than the first frame at all: a volley's
	// opening frames are empty while the first shot is still on its way.
	traced bool
}

// playBurst starts one over a world position, with the caster at from.
func (s *InGameState) playBurst(name string, spec burstSpec, from, at [3]float32) {
	if len(spec.parts) == 0 {
		return
	}

	other := [3]float32{spec.otherX, spec.otherY, spec.otherZ}
	if spec.fromCaster {
		// The caster is wherever they are standing, so the other end is only
		// known now. Without one the burst collapses onto the target, which
		// for a line of spikes is a heap of them in one place.
		if from == ([3]float32{}) {
			return
		}

		other = [3]float32{from[0] - at[0], from[1] - at[1], from[2] - at[2]}
	}

	trace.Emit(trace.HUD, "burst-play",
		zap.String("effect", name), zap.Int("parts", len(spec.parts)))

	s.bursts = append(s.bursts, &activeBurst{
		parts:  spec.parts,
		otherX: other[0], otherY: other[1], otherZ: other[2],
		x: at[0], y: at[1], z: at[2],
		runMs: spec.runMs,
	})
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

		// The other end in pixels: projected each frame, so the line the
		// burst is drawn along follows the camera.
		var otherX, otherY float32
		if b.otherX != 0 || b.otherY != 0 || b.otherZ != 0 {
			ox, oy := s.projectToScreen(b.x+b.otherX, b.y+b.otherY, b.z+b.otherZ,
				viewportW, viewportH)
			if ox >= 0 {
				otherX, otherY = ox-originX, oy-originY
			}
		}

		quads := b.quadsAt(originX, originY, otherX, otherY)
		if trace.On(trace.HUD) && !b.traced && len(quads) > 0 {
			b.traced = true

			trace.Emit(trace.HUD, "burst-quads",
				zap.Int("quads", len(quads)),
				zap.Float32("x", originX), zap.Float32("y", originY))
		}

		out = append(out, quads...)
	}

	return out
}

// quadsAt lays a burst's particles out around a point on the screen.
//
// Apart from the projection this is the whole of what a burst is, which is
// why it is here rather than in the loop above: the projection needs a scene
// and a camera, and the shape of an effect over time can be looked at without
// either.
func (b *activeBurst) quadsAt(originX, originY, otherX, otherY float32) []EffectQuad {
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

		if p.atOther != 0 || p.fallsIn {
			fromX := otherX*p.atOther + p.jitter[0]
			fromY := otherY*p.atOther + p.jitter[1]

			x = originX + fromX
			y = originY + fromY

			if p.fallsIn {
				// All the way in over its life, so it arrives as it goes out,
				// and turned to lie along the way it is going. The quad is
				// drawn upright, so this is the angle that puts its long axis
				// on the line from where it started to where it lands.
				left := 1 - age/p.lifeMs

				x = originX + fromX*left
				y = originY + fromY*left
				angle = float32(math.Atan2(float64(fromX), float64(-fromY)))
			}

			x += p.vx * frames
			y += p.vy*frames + 0.5*p.ay*frames*frames
		}

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

// Falling bolts.
//
// Cold Bolt and Fire Bolt are not one flash on the target. Each is a volley of
// shots that fall out of the sky one after another, one per blow the skill
// landed, and it is the volley that makes a level ten bolt read as ten times
// a level one rather than as the same picture with a bigger figure on it.
//
// From nostalro-client's magic_bolt.rs. The shots come from the same place
// every time — up, and off to one side — which is the original's own habit
// rather than anything derived.

// Where a shot falls from, in world units relative to what it is aimed at,
// and how fast it comes down. A cell is five units, so this is four cells up
// and a couple across.
var (
	boltFall  = [3]float32{21, 42, 14}
	boltSpeed = float32(1.925)
)

// boltSpawnFrames is when the first shot appears, and boltPeriodFrames how
// long until the next. Ten shots take a little over two seconds, which is
// what a level ten bolt looks like.
const (
	boltSpawnFrames  = 12
	boltPeriodFrames = 10
	boltMax          = 10
)

// boltStyle is what one kind of bolt looks like.
type boltStyle struct {
	texture string
	tint    [3]float32

	// halfLen and halfWid are its size in world units.
	halfLen, halfWid float32
}

var (
	iceBolt = boltStyle{
		texture: "icearrow.tga",
		tint:    [3]float32{0.7, 0.85, 1.0},
		halfLen: 11.5,
		halfWid: 3.8,
	}

	// The archive files the fire shot under its Korean name, as eight frames
	// of an animation. One of them, because a burst particle draws one
	// texture: the shot is on screen for under half a second on its way down.
	fireBolt = boltStyle{
		texture: "불화살1.tga",
		tint:    [3]float32{1.0, 0.85, 0.4},
		halfLen: 14.0,
		halfWid: 3.5,
	}
)

// boltTravelFrames is how long a shot takes to come down.
func boltTravelFrames() float32 {
	fall := boltFall

	return float32(math.Sqrt(float64(fall[0]*fall[0]+fall[1]*fall[1]+fall[2]*fall[2]))) / boltSpeed
}

// boltImpactMs is when the nth shot of a volley lands, counted from the start
// of the burst. What the shot hits is drawn then rather than at the start:
// a flash on a target nothing has reached yet is the thing that made a bolt
// skill look like a single blow.
func boltImpactMs(i int) float32 {
	return burstFrames(boltSpawnFrames + boltPeriodFrames*float32(i) + boltTravelFrames())
}

// boltParts is a volley of them.
func boltParts(hits int, style boltStyle) burstSpec {
	shots := min(max(hits, 1), boltMax)

	parts := make([]burstParticle, 0, shots)
	for i := 0; i < shots; i++ {
		parts = append(parts, burstParticle{
			atOther: 1,
			fallsIn: true,

			// Five units either way, so ten shots at one unit do not all come
			// down the same line.
			jitter: [2]float32{
				(hash01(uint32(i), 31) - 0.5) * 10 * burstScale,
				(hash01(uint32(i), 32) - 0.5) * 4 * burstScale,
			},

			halfW:   style.halfWid * burstScale,
			halfH:   style.halfLen * burstScale,
			tint:    style.tint,
			texture: style.texture,

			birthMs: burstFrames(boltSpawnFrames + boltPeriodFrames*float32(i)),
			lifeMs:  burstFrames(boltTravelFrames()),

			// Full strength all the way down and gone on arrival: what it hit
			// takes over from there.
			fadeInMs:  burstFrames(2),
			fadeOutMs: burstFrames(2),

			additive: true,
		})
	}

	return burstSpec{
		parts:  parts,
		runMs:  boltImpactMs(shots-1) + burstFrames(4),
		otherX: boltFall[0], otherY: boltFall[1], otherZ: boltFall[2],
	}
}

// Ice spikes.
//
// Frost Diver walks a line of them out of the ground from the caster to what
// it was aimed at, and then seals the target in a ring of larger ones. Neither
// is in the archive as an animation — there is no frostdiver.str — only the
// one texture they are all cut from.
//
// From nostalro-client's frost_diver.rs.

// A spike pushes up out of the ground over its first third and then stands
// still until it goes. It grows rather than rises: the base stays where it
// came out of the ground while the point climbs, which is what makes it read
// as ice breaking through rather than as a shard floating up.
const (
	spikeRunFrames  = 40
	spikeRiseFrames = 20
	spikeFadeFrames = 10
	spikeAlpha      = 200.0 / 255

	// spikeStepFrames is how long the line takes to reach the next spike
	// along. One a frame, so the line runs out as fast as the original's
	// projectile walks it.
	spikeStepFrames = 1

	// frostDiverSpikes is how many stand in the line. The original puts one
	// every two units and so has as many as the distance calls for; a fixed
	// number spreads them along whatever the distance turns out to be, which
	// looks the same and does not need the distance to build the burst.
	frostDiverSpikes = 14
)

// frostDiverParts is the line of spikes from the caster to the target.
func frostDiverParts() burstSpec {
	parts := make([]burstParticle, 0, frostDiverSpikes)
	for i := 0; i < frostDiverSpikes; i++ {
		// From just clear of the caster to the target, in order, so the line
		// runs the way the spell was thrown.
		along := 1 - float32(i)/frostDiverSpikes

		parts = append(parts, spikeParticle(uint32(i), along,
			burstFrames(spikeStepFrames*float32(i)), 0.3, 0.6, 7, 10))
	}

	return burstSpec{
		parts:      parts,
		runMs:      frostDiverReachMs() + burstFrames(spikeRunFrames),
		fromCaster: true,
	}
}

// frostDiver2Parts is the ring that closes over whatever was hit — the eight
// larger spikes the original seals a frozen target in.
//
// It waits for the line to arrive. Closed at the moment the packet lands it
// would seal the target before the spell had reached them.
func frostDiver2Parts() burstSpec {
	const spikes = 8

	parts := make([]burstParticle, 0, spikes)
	for i := 0; i < spikes; i++ {
		part := spikeParticle(uint32(i)+100, 0, frostDiverReachMs(), 0.6, 1.4, 4, 6.5)

		// Around the target rather than along a line: a ring of a cell or so,
		// spread by the same hash everything else here is spread by.
		angle := 2 * math.Pi * float64(hash01(uint32(i), 41))
		radius := (1.5 + 3.5*hash01(uint32(i), 42)) * burstScale

		part.jitter = [2]float32{
			float32(math.Cos(angle)) * radius,
			float32(math.Sin(angle)) * radius / 2,
		}
		part.atOther = 0.0001 // enough to take the jittered path

		parts = append(parts, part)
	}

	return burstSpec{parts: parts, runMs: frostDiverReachMs() + burstFrames(spikeRunFrames)}
}

// frostDiverReachMs is how long the line takes to run out to the target.
func frostDiverReachMs() float32 {
	return burstFrames(spikeStepFrames * frostDiverSpikes)
}

// spikeParticle is one of them, sized within the ranges given in world units.
func spikeParticle(seed uint32, along, birthMs, minWid, maxWid, minHigh, maxHigh float32) burstParticle {
	halfWid := (minWid + (maxWid-minWid)*hash01(seed, 43)) * burstScale
	height := (minHigh + (maxHigh-minHigh)*hash01(seed, 44)) * burstScale

	// Up over the rise and no further. Half the growth each frame in size and
	// half in position keeps the base still while the point climbs.
	grow := height / 2 / spikeRiseFrames

	return burstParticle{
		atOther: along,

		// It starts as a sliver at ground level and is grown into.
		halfW: halfWid,
		halfH: height / 2 / spikeRiseFrames,
		growH: grow,
		vy:    -grow,

		// A few degrees off upright, so a line of them is not a row of posts.
		angle: (hash01(seed, 45) - 0.5) * 0.35,

		birthMs:   birthMs,
		lifeMs:    burstFrames(spikeRunFrames),
		fadeInMs:  burstFrames(2),
		fadeOutMs: burstFrames(spikeFadeFrames),
		maxAlpha:  spikeAlpha,

		texture:  "ice.tga",
		tint:     [3]float32{0.78, 0.9, 1.0},
		additive: true,
	}
}

// burstFor is the burst a named effect plays, and whether there is one.
//
// hits is how many blows the skill landed, which the bolt skills draw one shot
// each for. Everything else ignores it.
func burstFor(effect string, hits int) (burstSpec, bool) {
	switch effect {
	case "EF_COLDHIT":
		parts, runMs := coldHitParts()

		return burstSpec{parts: parts, runMs: runMs}, true

	case "EF_HIT2":
		parts, runMs := hit2Parts()

		return burstSpec{parts: parts, runMs: runMs}, true

	case "EF_BASH":
		parts, runMs := bashParts()

		return burstSpec{parts: parts, runMs: runMs}, true

	case "EF_ICEARROW":
		return boltParts(hits, iceBolt), true

	case "EF_FIREARROW":
		return boltParts(hits, fireBolt), true

	case "EF_FROSTDIVER":
		return frostDiverParts(), true

	case "EF_FROSTDIVER2":
		return frostDiver2Parts(), true
	}

	return burstSpec{}, false
}
