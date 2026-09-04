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
// original; the file each came from is named where it is used. Everything is
// in the game's own units, where a cell is five, and every particle stands at
// a place in the world rather than at an offset on the screen.
//
// That was not so at first, and it was wrong. Laid out around a single
// projected point, a line of ice spikes came out as a row of equal shapes at
// the same height, none of them sitting on the ground and none of them
// smaller for being further away. What the original draws are billboards in
// the world; what these are is a quad per particle, at that particle's own
// world position, sized by how many pixels a world unit is worth there. The
// quads still face the screen, which is what RO's effects do anyway.

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
	// Where it starts and where it is going, in world units from the burst's
	// anchor. Y is up, as everywhere else in the world.
	x, y, z    float32
	vx, vy, vz float32

	// ay is the acceleration on it, negative for a particle that rises and
	// falls back.
	ay float32

	// angle is its rotation and spin how fast that turns, in radians per
	// frame; spinAccel slows the turn, which is what makes a burst of rays
	// fan out and then settle rather than keep spinning.
	angle, spin, spinAccel float32

	// halfW and halfH are its size in world units, growing by growW and growH
	// each frame. They are half-sizes because a quad is drawn around its
	// center, and the rays of a burst straddle the point they radiate from.
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

	// jitter shifts where it starts, in world units, so ten of them along the
	// same line do not sit on top of each other.
	jitter [3]float32

	// across bows it out sideways from the burst's own line, in world units at
	// the widest.
	//
	// A bow rather than a shift: nought at each end and widest halfway, so a
	// bolt leaves where the burst starts and arrives where it is aimed
	// however far out it went on the way. Shifted instead, five bolts appear
	// out of the air in a fan beside the caster rather than out of the caster.
	//
	// Not a jitter either, because which way sideways is is not known until
	// the burst is played: the caster can stand anywhere around the target,
	// and bolts spread along the world's x axis fly abreast from one side and
	// single file from another.
	across float32
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

	// onGround puts the whole burst at ground level under whatever it was
	// aimed at, rather than around the middle of it. Ice grows out of the
	// ground; a hit flashes on a body.
	onGround bool
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

	if spec.onGround {
		at = s.groundUnder(at)
		from = s.groundUnder(from)
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
//
// A quad per particle, at that particle's own place in the world. The scale
// comes from the same projection: how many pixels a world unit is worth is
// measured where the particle stands, so two spikes of the same height at
// different distances are drawn at different sizes, which is the whole reason
// this is not laid out on the screen.
func (s *InGameState) burstQuads(viewportW, viewportH float32) []EffectQuad {
	if len(s.bursts) == 0 || s.scene == nil || !s.SceneReady {
		return nil
	}

	var out []EffectQuad

	for _, b := range s.bursts {
		quads := s.burstQuadsOf(b, viewportW, viewportH)

		if trace.On(trace.HUD) && !b.traced && len(quads) > 0 {
			b.traced = true

			trace.Emit(trace.HUD, "burst-quads", zap.Int("quads", len(quads)))
		}

		out = append(out, quads...)
	}

	return out
}

// burstQuadsOf is one burst's worth.
func (s *InGameState) burstQuadsOf(b *activeBurst, viewportW, viewportH float32) []EffectQuad {
	return b.quadsAt(func(x, y, z float32) (screenX, screenY, perUnit float32, ok bool) {
		screenX, screenY = s.projectToScreen(x, y, z, viewportW, viewportH)
		if screenX < 0 {
			return 0, 0, 0, false
		}

		perUnit = s.pixelsPerUnit(x, y, z, screenX, screenY, viewportW, viewportH)

		return screenX, screenY, perUnit, perUnit > 0
	})
}

// projection turns a world point into pixels and says how many of them a
// world unit is worth there.
//
// Passed in rather than reached for, so the shape of a burst over time can be
// looked at without a camera: the projection needs a scene and a camera, and
// where the particles are and how big they get needs neither.
type projection func(x, y, z float32) (screenX, screenY, perUnit float32, ok bool)

// quadsAt lays a burst's particles out through a projection.
func (b *activeBurst) quadsAt(to projection) []EffectQuad {
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

		// How far along the burst's own line it stands. A shot travels the
		// whole of it as it falls; everything else stays where it was put.
		along := p.atOther
		if p.fallsIn {
			along *= 1 - age/p.lifeMs
		}

		x := b.x + b.otherX*along + p.x + p.jitter[0] + p.vx*frames
		y := b.y + b.otherY*along + p.y + p.jitter[1] + p.vy*frames + 0.5*p.ay*frames*frames
		z := b.z + b.otherZ*along + p.z + p.jitter[2] + p.vz*frames

		// Bowed out from the line, once there is a line to bow out of. Widest
		// halfway along it and nothing at either end, which is what keeps
		// every bolt of a volley leaving the same point.
		if p.across != 0 {
			if run := float32(math.Hypot(float64(b.otherX), float64(b.otherZ))); run > 0 {
				bow := 4 * along * (1 - along)

				x += -b.otherZ / run * p.across * bow
				z += b.otherX / run * p.across * bow
			}
		}

		// A world unit in pixels comes back with the point, measured there
		// rather than assumed: it is what makes something further away
		// smaller.
		screenX, screenY, perUnit, ok := to(x, y, z)
		if !ok {
			continue
		}

		halfW := (p.halfW + p.growW*frames) * perUnit
		halfH := (p.halfH + p.growH*frames) * perUnit
		if halfW <= 0 || halfH <= 0 {
			continue
		}

		// The spin decelerates, so the angle is the integral of it rather
		// than a rate times a time.
		angle := p.angle + p.spin*frames + p.spinAccel*frames*(frames+1)/2

		if p.fallsIn {
			// Turned to lie along the way it is going, which is the line from
			// where it started to where it lands, in pixels.
			//
			// The quad's own long axis is across rather than up: the shard in
			// icearrow.tga lies on its side in the square it is drawn in, and
			// a quad built tall makes a wide shape into a stretched smear.
			fromX, fromY, _, started := to(
				b.x+b.otherX+p.x+p.jitter[0],
				b.y+b.otherY+p.y+p.jitter[1],
				b.z+b.otherZ+p.z+p.jitter[2])

			if started {
				angle = float32(math.Atan2(float64(screenY-fromY), float64(screenX-fromX)))
			}
		}

		out = append(out, EffectQuad{
			Texture:  p.texture,
			Corners:  quadCorners(screenX, screenY, halfW, halfH, angle),
			UV:       [4][2]float32{{0, 0}, {1, 0}, {1, 1}, {0, 1}},
			Color:    [4]float32{p.tint[0], p.tint[1], p.tint[2], alpha},
			Additive: p.additive,
		})
	}

	return out
}

// pixelsPerUnit is how many pixels one world unit is worth at a place, found
// by projecting a point one unit above it.
//
// Above rather than to one side: a unit of height is a unit of height
// whichever way the camera is facing, where a unit east becomes shorter as
// the camera turns to look along it.
func (s *InGameState) pixelsPerUnit(x, y, z, screenX, screenY, viewportW, viewportH float32) float32 {
	upX, upY := s.projectToScreen(x, y+1, z, viewportW, viewportH)
	if upX < 0 {
		return 0
	}

	return float32(math.Hypot(float64(upX-screenX), float64(upY-screenY)))
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
		// Out in every direction and upward, which is a spray rather than a
		// ring: thrown flat they all cross the same line on the ground.
		angle := 2 * math.Pi * float64(hash01(uint32(i), 1))
		speed := 0.3 + 0.5*hash01(uint32(i), 2)

		parts = append(parts, burstParticle{
			vx:       float32(math.Cos(angle)) * speed,
			vz:       float32(math.Sin(angle)) * speed,
			vy:       0.2 + 0.4*hash01(uint32(i), 6),
			ay:       -0.03,
			halfW:    0.5,
			halfH:    1.5 + hash01(uint32(i), 3),
			birthMs:  burstFrames(hash01(uint32(i), 4) * 4),
			lifeMs:   burstFrames(15),
			fadeInMs: burstFrames(1),
			texture:  effectTexturePath + "lens1.tga",
			tint:     tint,
			additive: true,
		})
	}

	// Two puffs, born a little apart, growing where the shards left.
	for i, birth := range [2]float32{0, 7} {
		parts = append(parts, burstParticle{
			x:        hash01(uint32(i), 5) - 0.5,
			halfW:    1.0,
			halfH:    1.0,
			growW:    0.15,
			growH:    0.15,
			birthMs:  burstFrames(birth),
			lifeMs:   burstFrames(25),
			fadeInMs: burstFrames(3),
			texture:  effectTexturePath + "smoke.tga",
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
		speed := (0.5 + 4.5*hash01(uint32(i), 11)) * sizeF * 0.25
		angle := 2 * math.Pi * float64(hash01(uint32(i), 12))
		radius := 5 * sizeF * hash01(uint32(i), 13)

		texture := "lens1.tga"
		if i%2 == 1 {
			texture = "lens2.tga"
		}

		parts = append(parts, burstParticle{
			x:        float32(math.Cos(angle)) * radius,
			z:        float32(math.Sin(angle)) * radius,
			vy:       speed,
			ay:       -speed / 25,
			halfW:    (5 + 15*hash01(uint32(i), 14)) * sizeF / 2,
			halfH:    (20 + 20*hash01(uint32(i), 15)) * sizeF / 2,
			lifeMs:   burstFrames(10 + 20*hash01(uint32(i), 16)),
			fadeInMs: burstFrames(8),
			texture:  effectTexturePath + texture,
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
			halfW:     halo.radius,
			halfH:     halo.radius,
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
			halfW:  0.5 / 2,
			halfH:  2.8 + 2.8*hash01(uint32(i), 23),
			growH:  0.28 + 0.42*hash01(uint32(i), 24),
			lifeMs: burstFrames(runFrames),

			fadeInMs:  burstFrames(10),
			fadeOutMs: burstFrames(runFrames / 3),
			maxAlpha:  spikeAlpha,

			texture: effectTexturePath + "alpha_center.tga",
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

	// halfLen is along the way it is falling and halfWid across it, in world
	// units. The art lies on its side — the shard in icearrow.tga runs left
	// to right across the square it is drawn in — so the long one is the
	// quad's width and the shard is turned onto its path from there.
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
			jitter: [3]float32{
				(hash01(uint32(i), 31) - 0.5) * 10,
				(hash01(uint32(i), 32) - 0.5) * 4,
				(hash01(uint32(i), 33) - 0.5) * 10,
			},

			halfW:   style.halfLen,
			halfH:   style.halfWid,
			tint:    style.tint,
			texture: effectTexturePath + style.texture,

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
			burstFrames(spikeStepFrames*float32(i)), 8, 13))
	}

	return burstSpec{
		parts:      parts,
		runMs:      frostDiverReachMs() + burstFrames(spikeRunFrames),
		fromCaster: true,
		onGround:   true,
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
		part := spikeParticle(uint32(i)+100, 0, frostDiverReachMs(), 11, 16)

		// Around the target rather than along a line: a ring of a cell or so,
		// spread by the same hash everything else here is spread by.
		angle := 2 * math.Pi * float64(hash01(uint32(i), 41))
		radius := 1.5 + 2.5*hash01(uint32(i), 42)

		// A ring on the ground around it, not an oval on the screen.
		part.jitter = [3]float32{
			float32(math.Cos(angle)) * radius,
			0,
			float32(math.Sin(angle)) * radius,
		}

		parts = append(parts, part)
	}

	return burstSpec{
		parts:    parts,
		runMs:    frostDiverReachMs() + burstFrames(spikeRunFrames),
		onGround: true,
	}
}

// frostDiverReachMs is how long the line takes to run out to the target.
func frostDiverReachMs() float32 {
	return burstFrames(spikeStepFrames * frostDiverSpikes)
}

// spikeParticle is one of them, a height within the range given in world
// units, drawn from the frozen sprite's own art.
//
// Which frame is chosen by the seed rather than fixed, so a line of them is
// five different shapes rather than one repeated — and each keeps the
// proportions its art was drawn in.
func spikeParticle(seed uint32, along, birthMs, minHigh, maxHigh float32) burstParticle {
	height := minHigh + (maxHigh-minHigh)*hash01(seed, 44)
	frame := frozenShard + int(hash01(seed, 46)*frozenShards)

	p := frozenPart(min(frame, frozenShard+frozenShards-1), height)
	p.atOther = along

	// Up over the rise and no further. Half the growth each frame in size and
	// half in position keeps the base still while the point climbs, so it
	// reads as ice breaking through rather than as a shard floating up.
	grow := p.halfH / spikeRiseFrames

	// Width along with it, or the shard changes shape as it comes up.
	p.growW = p.halfW / spikeRiseFrames
	p.halfW /= spikeRiseFrames

	p.halfH /= spikeRiseFrames
	p.growH = grow
	p.vy = grow

	// A few degrees off upright, so a line of them is not a row of posts.
	p.angle = (hash01(seed, 45) - 0.5) * 0.35

	p.birthMs = birthMs
	p.lifeMs = burstFrames(spikeRunFrames)
	p.fadeInMs = burstFrames(2)
	p.fadeOutMs = burstFrames(spikeFadeFrames)
	p.maxAlpha = spikeAlpha

	return p
}

// Soul Strike, which the archive has no file for at all.
//
// There is no soulstrike.str and never was: the original draws it in code,
// and roBrowser's table marks it FUNC with sprite/이팩트/particle1 named in a
// comment beside it. What it draws is a volley of glowing orbs, one per blow
// the strike lands, each arcing out of the caster and into whoever was hit,
// dragging a tail behind it.
//
// The numbers are nostalro-client's, from effects/soul_strike.rs.
const (
	// soulSegments is how many quads make one bolt's tail, each born a little
	// after the one in front so the tail strings out behind the head as it
	// flies. Fewer than the original's twelve, and further apart: the orbs
	// are drawn big enough to read, and big enough to read is big enough that
	// twelve of them at two frames apart run together into one smear.
	soulSegments = 6

	// soulTrailFrames is the gap between one segment and the next.
	soulTrailFrames = 3

	// soulSpawnFrames is the gap between one bolt and the next, and
	// soulFlightFrames how long a bolt takes to arrive.
	//
	// Short: one leaves and the next is close behind it, which is the volley
	// the original throws. What keeps that from being a heap of light is not
	// the gap but the spread — five orbs down the same path at any spacing
	// are one thick line, and five orbs on five arcs read as five however
	// close together they leave.
	soulSpawnFrames  = 14
	soulFlightFrames = 26

	// soulBoltsMax is the most a strike lands, which is a level ten one.
	soulBoltsMax = 5

	// soulHalf is an orb's half-size in world units, soulRise how far a bolt
	// lofts on the way over, and soulSpread how far off the line the bolts
	// are rolled from each other.
	//
	// The spread is wide — four cells off the line at the widest — because it
	// is what tells the bolts apart. Rolled a little from each other they
	// arrive as one stream from one direction; rolled this far they come at
	// what they hit from over it, from each side and from along the ground,
	// which is the volley the original throws.
	//
	// The orb is much smaller than its quad. particle1 is a glow that fades
	// out to nothing over the whole sixty-four pixels, with a bright core
	// about a sixth of that across, so a quad the size the orb should be
	// draws a speck with a halo round it — and one big enough for the core to
	// read is a quad whose halo swallows the rest of the tail. This is the
	// size where the beads are still beads.
	soulHalf   = 18.0
	soulRise   = 16.0
	soulSpread = 20.0

	// soulFlashHalf is how wide the orb bursts when it lands, and
	// soulFlashFrames how long that lasts.
	soulFlashHalf   = 9.0
	soulFlashFrames = 12

	// soulFade is how much of a segment's life is spent leaving. The head of
	// a tail is solid and the end of it is nearly gone, which is what makes
	// it read as a tail rather than as a string of beads.
	soulFade = 0.55
)

// soulSprite is the orb. One frame, sixty-four pixels square, and the same
// one Napalm Beat and half a dozen other effects are built from.
const soulSprite = `data\sprite\이팩트\particle1.spr`

// soulRollDegrees is the angle a bolt is rolled to around the line it flies
// along, which is what keeps five of them from flying down the same path.
//
// The original's own table: one bolt goes straight down the middle, and the
// more there are the tighter the fan they are spread into.
func soulRollDegrees(bolts, i int) float32 {
	start, step := float32(-90), float32(45)

	switch bolts {
	case 1:
		start, step = 0, 1
	case 2:
		step = 180
	case 3:
		step = 90
	case 4:
		step = 60
	}

	return start + float32(i+1)*step
}

// soulStrikeParts is the volley.
func soulStrikeParts(hits int) burstSpec {
	bolts := min(max(hits, 1), soulBoltsMax)

	// A segment lives exactly as long as the flight, so every one of them
	// arrives rather than fading out somewhere over the field.
	life := burstFrames(soulFlightFrames)

	parts := make([]burstParticle, 0, bolts*soulSegments)

	for bolt := 0; bolt < bolts; bolt++ {
		roll := float64(soulRollDegrees(bolts, bolt)) * math.Pi / 180

		// The roll turns around the line it is flying down, and the two ways
		// perpendicular to a line along the ground are up and sideways. No
		// roll is straight up and over, which is what a single bolt does; a
		// quarter turn is all the way out to one side.
		across := soulSpread * float32(math.Sin(roll))
		rise := soulRise + soulSpread*float32(math.Cos(roll))

		// Up and back down over the flight, so the bolt arrives at the height
		// it was aimed at rather than above it.
		vy := 4 * rise / soulFlightFrames
		ay := -2 * vy / soulFlightFrames

		for seg := 0; seg < soulSegments; seg++ {
			// The tail thins as it goes back, which is the segment nearest
			// the caster being the faintest and the smallest.
			back := float32(seg) / soulSegments

			parts = append(parts, burstParticle{
				vy: vy, ay: ay,
				halfW: soulHalf * (1 - 0.6*back),
				halfH: soulHalf * (1 - 0.6*back),

				birthMs: burstFrames(float32(bolt*soulSpawnFrames + seg*soulTrailFrames)),
				lifeMs:  life,

				fadeOutMs: life * soulFade,
				maxAlpha:  0.65 - 0.35*back,

				texture:  spriteFrameKey(soulSprite, 0),
				tint:     [3]float32{1, 1, 1},
				additive: true,

				// Thrown from the caster and traveling the whole way in.
				atOther: 1,
				fallsIn: true,
				across:  across,
			})
		}

		// The orb bursting on what it hit.
		//
		// Five blows arrive as one packet with one figure over the target,
		// and without something for each of them a volley reads as one strike
		// rather than as five.
		parts = append(parts, burstParticle{
			halfW: soulFlashHalf / 2,
			halfH: soulFlashHalf / 2,
			growW: soulFlashHalf / 2 / soulFlashFrames,
			growH: soulFlashHalf / 2 / soulFlashFrames,

			birthMs: burstFrames(float32(bolt*soulSpawnFrames) + soulFlightFrames),
			lifeMs:  burstFrames(soulFlashFrames),

			fadeInMs: burstFrames(2),
			maxAlpha: 0.9,

			texture:  spriteFrameKey(soulSprite, 0),
			tint:     [3]float32{1, 1, 1},
			additive: true,
		})
	}

	// The last bolt lands here, and its tail and its flash finish after it.
	last := float32((bolts-1)*soulSpawnFrames) + soulFlightFrames

	return burstSpec{
		parts: parts,
		runMs: burstFrames(max(
			last+float32((soulSegments-1)*soulTrailFrames),
			last+soulFlashFrames)),
		fromCaster: true,
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

	case "EF_SOULSTRIKE":
		return soulStrikeParts(hits), true
	}

	return burstSpec{}, false
}
