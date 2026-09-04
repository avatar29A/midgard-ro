package states

import (
	"math"
	"testing"
)

// burstNames are the effects drawn in code. None of them has an STR in the
// archive — that is why they are here — so a burst is the only way they appear
// at all.
var burstNames = []string{"EF_COLDHIT", "EF_HIT2", "EF_BASH"}

// TestBurstsAreDefinedForTheEffectsThatNeedThem: and only for those. An effect
// with both a file and a burst would be drawn twice.
func TestBurstsAreDefinedForTheEffectsThatNeedThem(t *testing.T) {
	for _, name := range burstNames {
		spec, ok := burstFor(name, 1)
		if !ok {
			t.Errorf("%s has no burst", name)
			continue
		}
		if len(spec.parts) == 0 {
			t.Errorf("%s has a burst with no particles", name)
		}
		if spec.runMs <= 0 {
			t.Errorf("%s runs for %v", name, spec.runMs)
		}
	}

	if _, ok := burstFor("EF_FIREHIT", 1); ok {
		t.Error("EF_FIREHIT has a burst as well as an STR, which would draw it twice")
	}
}

// TestEveryParticleOutlivesItsBirth: a particle born after its burst has
// finished never appears, which is a particle written and not drawn.
func TestEveryParticleOutlivesItsBirth(t *testing.T) {
	for _, name := range burstNames {
		spec, _ := burstFor(name, 1)
		parts, runMs := spec.parts, spec.runMs

		for i, p := range parts {
			if p.lifeMs <= 0 {
				t.Errorf("%s particle %d lives %v", name, i, p.lifeMs)
			}
			if p.birthMs >= runMs {
				t.Errorf("%s particle %d is born at %v, after the burst ends at %v",
					name, i, p.birthMs, runMs)
			}
			if p.texture == "" {
				t.Errorf("%s particle %d has no texture", name, i)
			}
			if p.halfW <= 0 || p.halfH <= 0 {
				t.Errorf("%s particle %d starts at %vx%v, which draws nothing",
					name, i, p.halfW, p.halfH)
			}
		}
	}
}

// TestEveryParticleIsVisibleAtSomePoint: one that is transparent all the way
// through is a particle nobody sees.
func TestEveryParticleIsVisibleAtSomePoint(t *testing.T) {
	for _, name := range burstNames {
		parts := burstFor2(name)

		for i, p := range parts {
			var peak float32
			for step := float32(0); step < p.lifeMs; step += p.lifeMs / 20 {
				if a := p.alphaAt(step); a > peak {
					peak = a
				}
			}

			if peak < 0.1 {
				t.Errorf("%s particle %d never gets brighter than %v", name, i, peak)
			}
		}
	}
}

// TestParticlesFadeOutBeforeTheyGo: a particle at full strength on its last
// frame pops out of the scene.
func TestParticlesFadeOutBeforeTheyGo(t *testing.T) {
	for _, name := range burstNames {
		parts := burstFor2(name)

		for i, p := range parts {
			if a := p.alphaAt(p.lifeMs * 0.99); a > 0.2 {
				t.Errorf("%s particle %d is still at %v as it ends", name, i, a)
			}
			if a := p.alphaAt(p.lifeMs); a != 0 {
				t.Errorf("%s particle %d draws at %v past its life", name, i, a)
			}
			if a := p.alphaAt(-1); a != 0 {
				t.Errorf("%s particle %d draws at %v before it is born", name, i, a)
			}
		}
	}
}

// TestBurstsAgeOut: one that never ends is one that stays on screen forever.
func TestBurstsAgeOut(t *testing.T) {
	for _, name := range burstNames {
		s := &InGameState{}
		spec, _ := burstFor(name, 1)
		runMs := spec.runMs
		s.playBurst(name, spec, [3]float32{}, [3]float32{})

		if len(s.bursts) != 1 {
			t.Fatalf("%s: %d bursts playing, want 1", name, len(s.bursts))
		}

		s.advanceBursts(runMs / 2)
		if len(s.bursts) != 1 {
			t.Fatalf("%s: the burst ended early", name)
		}

		s.advanceBursts(runMs)
		if len(s.bursts) != 0 {
			t.Errorf("%s: %d bursts outlived their run", name, len(s.bursts))
		}
	}
}

// TestTheSameBurstLooksTheSame: the spread comes from the particle's index
// rather than from a clock, so a burst played twice is the same burst — a
// shimmer that differs every time reads as a bug.
func TestTheSameBurstLooksTheSame(t *testing.T) {
	for _, name := range burstNames {
		first, second := burstFor2(name), burstFor2(name)

		if len(first) != len(second) {
			t.Fatalf("%s: %d particles then %d", name, len(first), len(second))
		}

		for i := range first {
			if first[i] != second[i] {
				t.Errorf("%s: particle %d differs between two plays", name, i)
			}
		}
	}
}

// TestBashRaysRadiateInEveryDirection: twenty rays that all pointed the same
// way would be a bar rather than a burst.
func TestBashRaysRadiateInEveryDirection(t *testing.T) {
	parts := burstFor2("EF_BASH")

	quadrants := map[int]int{}
	rays := 0

	for _, p := range parts {
		if p.texture != effectTexturePath+"alpha_center.tga" {
			continue
		}

		rays++
		quadrants[int(p.angle/(math.Pi/2))]++
	}

	if rays != 20 {
		t.Errorf("%d rays, want the original's twenty", rays)
	}
	if len(quadrants) != 4 {
		t.Errorf("the rays fall in %d quarters of the circle, want all four", len(quadrants))
	}
}

// TestBashRaysSlowToAStop: the original decelerates each ray so the burst
// settles. One that kept its speed would spin for as long as it was drawn, and
// one accelerating the wrong way would reverse halfway through.
func TestBashRaysSlowToAStop(t *testing.T) {
	spec, _ := burstFor("EF_BASH", 1)
	parts, runMs := spec.parts, spec.runMs

	for i, p := range parts {
		if p.spin == 0 {
			continue
		}

		frames := runMs / 1000 * burstFPS
		if end := p.spin + p.spinAccel*frames; end/p.spin < 0 {
			t.Errorf("ray %d turns back on itself: %v becomes %v", i, p.spin, end)
		}
		if p.spin*p.spinAccel >= 0 {
			t.Errorf("ray %d speeds up rather than slowing: spin %v accel %v", i, p.spin, p.spinAccel)
		}
	}
}

// TestBashIsDrawnWithOrdinaryAlpha: added, its haloes wash out the unit they
// are supposed to frame.
func TestBashIsDrawnWithOrdinaryAlpha(t *testing.T) {
	parts := burstFor2("EF_BASH")

	for i, p := range parts {
		if p.additive {
			t.Errorf("bash particle %d is additive", i)
		}
	}
}

// TestRotatedQuadKeepsItsSize: the corners turn about the center, so a rotated
// quad covers the same area in a different direction — one that grew or
// drifted as it turned would swell through the burst.
func TestRotatedQuadKeepsItsSize(t *testing.T) {
	const halfW, halfH = 3, 20

	flat := quadCorners(100, 50, halfW, halfH, 0)
	turned := quadCorners(100, 50, halfW, halfH, math.Pi/2)

	for i := range flat {
		got := math.Hypot(float64(turned[i][0]-100), float64(turned[i][1]-50))
		want := math.Hypot(halfW, halfH)

		if math.Abs(got-want) > 0.01 {
			t.Errorf("corner %d sits %v from the center, want %v", i, got, want)
		}
	}

	// A quarter turn puts the long axis across instead of down.
	w, h := quadSpan(flat)
	if math.Abs(float64(w)-2*halfW) > 0.01 || math.Abs(float64(h)-2*halfH) > 0.01 {
		t.Errorf("an upright quad measures %vx%v, want %vx%v", w, h, 2*halfW, 2*halfH)
	}

	w, h = quadSpan(turned)
	if math.Abs(float64(w)-2*halfH) > 0.01 || math.Abs(float64(h)-2*halfW) > 0.01 {
		t.Errorf("a quarter-turned quad measures %vx%v, want %vx%v", w, h, 2*halfH, 2*halfW)
	}
}

// quadSpan is how wide and how tall a quad is.
func quadSpan(corners [4][2]float32) (w, h float32) {
	minX, maxX := corners[0][0], corners[0][0]
	minY, maxY := corners[0][1], corners[0][1]

	for _, c := range corners[1:] {
		minX, maxX = min(minX, c[0]), max(maxX, c[0])
		minY, maxY = min(minY, c[1]), max(maxY, c[1])
	}

	return maxX - minX, maxY - minY
}

// quadsThrough lays a burst out at ten points across its run.
func quadsThrough(name string) (frames [][]EffectQuad) {
	b := playing(name)

	for step := 0; step < 10; step++ {
		b.ageMs = b.runMs * float32(step) / 10
		frames = append(frames, b.quadsAt(flatView))
	}

	return frames
}

// burstFor2 is the particles of a burst, for the tests that only look at
// those.
func burstFor2(name string) []burstParticle {
	spec, _ := burstFor(name, 1)

	return spec.parts
}

// playing is a burst mid-flight, ready to be laid out. One blow, which is
// every burst but a volley and the first shot of one.
func playing(name string) *activeBurst {
	spec, _ := burstFor(name, 1)

	return &activeBurst{
		parts:  spec.parts,
		otherX: spec.otherX, otherY: spec.otherY, otherZ: spec.otherZ,
		runMs: spec.runMs,
	}
}

// flatView stands in for the camera: an isometric-ish projection with a scale
// that falls off with distance, so a test can tell a particle that moved from
// one that only looks as though it did, and one that is further away from one
// that is smaller.
func flatView(x, y, z float32) (screenX, screenY, perUnit float32, ok bool) {
	perUnit = 60 / (30 + z)

	return 400 + (x-z)*2, 300 - y*2 + (x+z)/2, perUnit, true
}

// TestEveryBurstDrawsSomething: the whole point. A burst that lays out no
// quads is an effect that plays and is not seen, which is what Cold Bolt did
// before there was one.
func TestEveryBurstDrawsSomething(t *testing.T) {
	for _, name := range burstNames {
		frames := quadsThrough(name)

		var most int
		for _, quads := range frames {
			if len(quads) > most {
				most = len(quads)
			}
		}

		if most == 0 {
			t.Errorf("%s never draws a quad", name)
		}
	}
}

// TestBurstsAreBigEnoughToSee: a quad a few pixels across is not an effect.
// The world units these came from are converted by hand, and getting the
// conversion wrong is how a burst becomes a speck or fills the screen.
func TestBurstsAreBigEnoughToSee(t *testing.T) {
	for _, name := range burstNames {
		frames := quadsThrough(name)

		var widest, tallest float32
		for _, quads := range frames {
			for _, q := range quads {
				w, h := quadSpan(q.Corners)
				widest, tallest = max(widest, w), max(tallest, h)
			}
		}

		// A character's sprite is around a hundred pixels tall at this
		// client's camera, so an effect on one is tens of pixels, not ones
		// and not thousands.
		if widest < 10 || tallest < 10 {
			t.Errorf("%s is at most %vx%v pixels, too small to see", name, widest, tallest)
		}
		if widest > 1000 || tallest > 1000 {
			t.Errorf("%s grows to %vx%v pixels, which covers the screen", name, widest, tallest)
		}
	}
}

// TestBurstsStayNearWhatTheyHit: an effect that drifts off is one drawn over
// somebody else. Half a screen is the limit — Bash's rays are long by design.
func TestBurstsStayNearWhatTheyHit(t *testing.T) {
	const originX, originY = 400, 300

	for _, name := range burstNames {
		b := playing(name)

		for step := 0; step < 10; step++ {
			b.ageMs = b.runMs * float32(step) / 10

			for _, q := range b.quadsAt(flatView) {
				for _, c := range q.Corners {
					if dx := c[0] - originX; dx < -400 || dx > 400 {
						t.Fatalf("%s reaches %v across at %vms", name, dx, b.ageMs)
					}
					if dy := c[1] - originY; dy < -400 || dy > 400 {
						t.Fatalf("%s reaches %v up or down at %vms", name, dy, b.ageMs)
					}
				}
			}
		}
	}
}

// TestBurstParticlesMove: one whose quads are in the same place from one frame
// to the next is a still picture rather than a burst.
func TestBurstParticlesMove(t *testing.T) {
	for _, name := range burstNames {
		b := playing(name)

		b.ageMs = b.runMs * 0.3
		early := b.quadsAt(flatView)

		b.ageMs = b.runMs * 0.6
		late := b.quadsAt(flatView)

		if len(early) == 0 || len(late) == 0 {
			t.Errorf("%s draws nothing in the middle of its run", name)
			continue
		}

		same := 0
		for i := range early {
			if i < len(late) && early[i].Corners == late[i].Corners {
				same++
			}
		}

		if same == len(early) {
			t.Errorf("%s is the same picture at 30%% and 60%% of its run", name)
		}
	}
}

// TestAVolleyIsOneShotPerBlow: what makes a level ten bolt read as ten times a
// level one. Drawn as a single flash, the only difference between them is the
// size of the figure that floats up.
func TestAVolleyIsOneShotPerBlow(t *testing.T) {
	for _, tc := range []struct{ hits, want int }{
		{1, 1}, {3, 3}, {10, 10},
		{0, 1},   // a skill that reported no blows still shows one shot
		{99, 10}, // and one that reported more than the original ever draws
	} {
		spec, ok := burstFor("EF_ICEARROW", tc.hits)
		if !ok {
			t.Fatal("EF_ICEARROW has no burst")
		}

		if len(spec.parts) != tc.want {
			t.Errorf("%d blows drew %d shots, want %d", tc.hits, len(spec.parts), tc.want)
		}
	}
}

// TestShotsFallOneAfterAnother: a volley arrives in one packet and is drawn
// over two seconds. All ten at once would be a single flash again.
func TestShotsFallOneAfterAnother(t *testing.T) {
	spec, _ := burstFor("EF_ICEARROW", 10)

	for i := 1; i < len(spec.parts); i++ {
		if spec.parts[i].birthMs <= spec.parts[i-1].birthMs {
			t.Errorf("shot %d appears at %v, no later than shot %d at %v",
				i, spec.parts[i].birthMs, i-1, spec.parts[i-1].birthMs)
		}
		if got := boltImpactMs(i) - boltImpactMs(i-1); got <= 0 {
			t.Errorf("shot %d lands %v after shot %d", i, got, i-1)
		}
	}

	// The volley has to outlast its own last shot, or the last one is cut off
	// halfway down.
	if last := boltImpactMs(len(spec.parts) - 1); spec.runMs <= last {
		t.Errorf("the volley ends at %v, before its last shot lands at %v", spec.runMs, last)
	}
}

// TestAShotArrivesWhereItWasAimed: it starts where the burst falls from and
// comes down onto the anchor. One that stopped short, or overshot, would land
// somewhere the target is not.
func TestAShotArrivesWhereItWasAimed(t *testing.T) {
	const originX, originY = 400, 300

	b := playing("EF_ICEARROW")
	shot := b.parts[0]

	middle := func(quads []EffectQuad) (x, y float32) {
		for _, c := range quads[0].Corners {
			x, y = x+c[0]/4, y+c[1]/4
		}

		return x, y
	}

	// Just born, it is up by the fall.
	b.ageMs = shot.birthMs + 1
	quads := b.quadsAt(flatView)
	if len(quads) != 1 {
		t.Fatalf("%d quads for one shot just born", len(quads))
	}

	x, y := middle(quads)
	if y > originY-40 {
		t.Errorf("the shot starts at %v, which is not above the target at %v", y, originY)
	}
	if x == originX {
		t.Error("the shot starts straight overhead, which is not off to one side")
	}

	// About to land, it is on the target.
	b.ageMs = shot.birthMs + shot.lifeMs*0.98
	quads = b.quadsAt(flatView)
	if len(quads) != 1 {
		t.Fatalf("%d quads for one shot about to land", len(quads))
	}

	x, y = middle(quads)
	if dx, dy := x-originX, y-originY; dx*dx+dy*dy > 30*30 {
		t.Errorf("the shot lands at %v,%v, %v from the target at %v,%v",
			x, y, math.Hypot(float64(dx), float64(dy)), originX, originY)
	}
}

// TestAShotLiesAlongItsFall: the shard points the way it is going. Built tall
// instead of wide it also came out stretched, because the art lies on its
// side — a wide shape squeezed into a narrow quad is a smear.
func TestAShotLiesAlongItsFall(t *testing.T) {
	b := playing("EF_ICEARROW")
	b.ageMs = b.parts[0].birthMs + b.parts[0].lifeMs/2

	quads := b.quadsAt(flatView)
	if len(quads) != 1 {
		t.Fatalf("%d quads", len(quads))
	}

	// The long axis runs from the left pair of corners to the right pair: the
	// shard in icearrow.tga lies on its side in the square it is drawn in, so
	// the quad is built wide and turned onto its path.
	long := [2]float32{
		(quads[0].Corners[1][0] + quads[0].Corners[2][0]) / 2,
		(quads[0].Corners[1][1] + quads[0].Corners[2][1]) / 2,
	}
	long[0] -= (quads[0].Corners[0][0] + quads[0].Corners[3][0]) / 2
	long[1] -= (quads[0].Corners[0][1] + quads[0].Corners[3][1]) / 2

	// The way it is going, through the same projection: from where the shot
	// started, down onto the target.
	startX, startY, _, _ := flatView(b.x+b.otherX, b.y+b.otherY, b.z+b.otherZ)
	landX, landY, _, _ := flatView(b.x, b.y, b.z)
	fall := [2]float32{landX - startX, landY - startY}

	dot := long[0]*fall[0] + long[1]*fall[1]
	lenLong := math.Hypot(float64(long[0]), float64(long[1]))
	lenFall := math.Hypot(float64(fall[0]), float64(fall[1]))

	if cos := float64(dot) / (lenLong * lenFall); cos < 0.99 {
		t.Errorf("the shot lies %.0f degrees off the way it is falling",
			math.Acos(min(max(cos, -1), 1))*180/math.Pi)
	}
}

// TestBothBoltsHaveArt: the fire shot is filed under a Korean name and the ice
// one is not. A name that is wrong loads nothing and draws nothing, which is
// the fault this whole change was made to fix.
func TestBothBoltsHaveArt(t *testing.T) {
	for name, want := range map[string]string{
		"EF_ICEARROW":  effectTexturePath + "icearrow.tga",
		"EF_FIREARROW": effectTexturePath + "불화살1.tga",
	} {
		spec, ok := burstFor(name, 1)
		if !ok {
			t.Errorf("%s has no burst", name)

			continue
		}

		if got := spec.parts[0].texture; got != want {
			t.Errorf("%s draws %q, want %q", name, got, want)
		}
		if spec.otherY <= 0 {
			t.Errorf("%s falls from %v above, want it overhead", name, spec.otherY)
		}
	}
}

// TestTheSpikeLineRunsFromTheCaster: Frost Diver throws a line of spikes out
// of the ground from the mage to what was aimed at. Drawn around the target
// alone it is a heap of ice with nothing thrown at all.
func TestTheSpikeLineRunsFromTheCaster(t *testing.T) {
	spec, ok := burstFor("EF_FROSTDIVER", 1)
	if !ok {
		t.Fatal("EF_FROSTDIVER has no burst")
	}

	if !spec.fromCaster {
		t.Error("the line is not drawn from the caster")
	}

	// In order, from the caster's end to the target's, and each one a little
	// after the one before it.
	for i := 1; i < len(spec.parts); i++ {
		if spec.parts[i].atOther >= spec.parts[i-1].atOther {
			t.Errorf("spike %d stands at %v, no nearer the target than spike %d at %v",
				i, spec.parts[i].atOther, i-1, spec.parts[i-1].atOther)
		}
		if spec.parts[i].birthMs <= spec.parts[i-1].birthMs {
			t.Errorf("spike %d comes up at %v, no later than spike %d at %v",
				i, spec.parts[i].birthMs, i-1, spec.parts[i-1].birthMs)
		}
	}

	if first := spec.parts[0].atOther; first < 0.9 {
		t.Errorf("the line starts %v of the way along, want it at the caster", first)
	}
	if last := spec.parts[len(spec.parts)-1].atOther; last > 0.15 {
		t.Errorf("the line stops %v of the way along, want it at the target", last)
	}
}

// TestTheSealWaitsForTheLine: closed at the moment the packet lands, the ice
// shuts over the target before the spell has reached them.
func TestTheSealWaitsForTheLine(t *testing.T) {
	line, _ := burstFor("EF_FROSTDIVER", 1)
	seal, ok := burstFor("EF_FROSTDIVER2", 1)
	if !ok {
		t.Fatal("EF_FROSTDIVER2 has no burst")
	}

	reach := line.parts[len(line.parts)-1].birthMs
	for i, p := range seal.parts {
		if p.birthMs < reach {
			t.Errorf("seal spike %d closes at %v, before the line arrives at %v",
				i, p.birthMs, reach)
		}
	}

	if seal.fromCaster {
		t.Error("the seal is drawn from the caster; it closes over the target")
	}
}

// TestASpikeGrowsOutOfTheGround: its base stays where it broke through while
// the point climbs. One that rises whole reads as a shard floating up.
func TestASpikeGrowsOutOfTheGround(t *testing.T) {
	spec, _ := burstFor("EF_FROSTDIVER2", 1)

	for i, p := range spec.parts {
		if p.growH <= 0 {
			t.Errorf("spike %d does not grow", i)
		}

		// Up by half of what it grows: the top rises by the whole growth and
		// the bottom stays put. World space, so up is positive.
		if want := p.growH; p.vy != want {
			t.Errorf("spike %d moves %v while growing %v, want %v so its base stays still",
				i, p.vy, p.growH, want)
		}
	}
}

// TestSpikesStandUpright: near enough. A line of them all at the same angle is
// a fence, and one lying over is not a spike at all.
func TestSpikesStandUpright(t *testing.T) {
	for _, name := range []string{"EF_FROSTDIVER", "EF_FROSTDIVER2"} {
		spec, _ := burstFor(name, 1)

		angles := map[float32]bool{}
		for i, p := range spec.parts {
			if p.angle < -0.4 || p.angle > 0.4 {
				t.Errorf("%s spike %d leans %v radians, which is on its side", name, i, p.angle)
			}
			angles[p.angle] = true
		}

		if len(angles) < 2 {
			t.Errorf("%s stands every spike at the same angle, which is a fence", name)
		}
	}
}

// TestParticlesShrinkWithDistance: the point of putting them in the world.
// Laid out around a single projected point, two spikes of the same height are
// drawn the same size however far apart they are, and a line of them running
// away from the camera comes out as a row of equal shapes.
func TestParticlesShrinkWithDistance(t *testing.T) {
	spec, _ := burstFor("EF_FROSTDIVER2", 1)

	near := &activeBurst{parts: spec.parts, ageMs: burstFrames(spikeRiseFrames)}
	far := &activeBurst{parts: spec.parts, z: 60, ageMs: burstFrames(spikeRiseFrames)}

	nearQuads, farQuads := near.quadsAt(flatView), far.quadsAt(flatView)
	if len(nearQuads) == 0 || len(nearQuads) != len(farQuads) {
		t.Fatalf("%d spikes near and %d far", len(nearQuads), len(farQuads))
	}

	for i := range nearQuads {
		_, high := quadSpan(nearQuads[i].Corners)
		_, low := quadSpan(farQuads[i].Corners)

		if low >= high {
			t.Errorf("spike %d is %v tall up close and %v far away", i, high, low)
		}
	}
}

// TestTheSpikeLineIsSpreadOutInTheWorld: the spikes stand at their own places
// on the ground, so the line reads as running away from the caster. Anchored
// to one point they were a row at the same height, all the same size.
func TestTheSpikeLineIsSpreadOutInTheWorld(t *testing.T) {
	spec, _ := burstFor("EF_FROSTDIVER", 1)

	// A caster five cells off. Not along the diagonal: the stand-in camera
	// maps that direction onto a single screen column, and a test that used
	// it would be measuring the projection rather than the layout.
	b := &activeBurst{parts: spec.parts, otherX: 25, otherZ: 5, ageMs: spec.runMs / 2}

	quads := b.quadsAt(flatView)
	if len(quads) < 4 {
		t.Fatalf("%d spikes standing halfway through the line", len(quads))
	}

	// The middles, not a corner: a corner moves with the size as well as with
	// the place, and the sizes differ down the line.
	var xs, ys []float32
	for _, q := range quads {
		var cx, cy float32
		for _, c := range q.Corners {
			cx, cy = cx+c[0]/4, cy+c[1]/4
		}

		xs, ys = append(xs, cx), append(ys, cy)
	}

	spread := func(v []float32) float32 {
		lo, hi := v[0], v[0]
		for _, f := range v[1:] {
			lo, hi = min(lo, f), max(hi, f)
		}

		return hi - lo
	}

	if spread(xs) < 20 {
		t.Errorf("the line covers %v across, which is a heap rather than a line", spread(xs))
	}
	if spread(ys) < 8 {
		t.Errorf("the line covers %v up and down; spikes further off should sit higher",
			spread(ys))
	}
}

// soulParts is a bolt's tail and the flash it lands with, which is how
// soulStrikeParts lays each one out.
const soulParts = soulSegments + 1

// soulBolt is one bolt out of a volley: its tail, then its flash.
func soulBolt(spec burstSpec, i int) ([]burstParticle, burstParticle) {
	parts := spec.parts[i*soulParts : (i+1)*soulParts]

	return parts[:soulSegments], parts[soulSegments]
}

// TestSoulStrikeThrowsOneBoltPerBlow: the strike lands as many blows as its
// level buys and the volley is one orb for each of them. Drawn as a single
// bolt however hard it hit, a level ten strike looks like a level one.
func TestSoulStrikeThrowsOneBoltPerBlow(t *testing.T) {
	for _, tc := range []struct {
		hits  int
		bolts int
	}{
		{0, 1}, {1, 1}, {3, 3}, {5, 5}, {9, 5},
	} {
		spec := soulStrikeParts(tc.hits)

		if got := len(spec.parts) / soulParts; got != tc.bolts {
			t.Errorf("%d blows threw %d bolts, want %d", tc.hits, got, tc.bolts)
		}
		if len(spec.parts)%soulParts != 0 {
			t.Errorf("%d blows left %d parts, not a whole number of bolts",
				tc.hits, len(spec.parts))
		}
	}
}

// TestSoulStrikeBoltsArrive: every segment is thrown from the caster and
// travels the whole way in. One that stayed where it was put is an orb hanging
// over the field.
func TestSoulStrikeBoltsArrive(t *testing.T) {
	spec := soulStrikeParts(5)

	if !spec.fromCaster {
		t.Error("the volley is not thrown from the caster")
	}

	for bolt := 0; bolt < 5; bolt++ {
		tail, _ := soulBolt(spec, bolt)

		for i, p := range tail {
			if p.atOther != 1 || !p.fallsIn {
				t.Fatalf("bolt %d segment %d starts at %v and fallsIn=%v, want 1 and true",
					bolt, i, p.atOther, p.fallsIn)
			}

			// Up and back down over the flight: the loft has to come to
			// nothing by the time the orb arrives, or it lands above what it
			// was aimed at.
			life := p.lifeMs / 1000 * burstFPS
			if end := p.vy*life + 0.5*p.ay*life*life; end < -0.5 || end > 0.5 {
				t.Errorf("bolt %d segment %d is %v units off the ground when it lands",
					bolt, i, end)
			}
		}
	}

	for i, p := range spec.parts {
		if p.birthMs+p.lifeMs > spec.runMs {
			t.Errorf("part %d is still alight at %vms when the burst ends at %v",
				i, p.birthMs+p.lifeMs, spec.runMs)
		}
	}
}

// TestSoulStrikeFlashesWhereEachOrbLands: five blows arrive as one figure over
// the target, and without a flash for each of them the volley reads as one
// strike. The flash goes off when its own orb gets there, not before.
func TestSoulStrikeFlashesWhereEachOrbLands(t *testing.T) {
	spec := soulStrikeParts(5)

	for bolt := 0; bolt < 5; bolt++ {
		tail, flash := soulBolt(spec, bolt)

		if flash.fallsIn || flash.atOther != 0 {
			t.Errorf("bolt %d flashes on its way in rather than at the target", bolt)
		}

		// The head of the tail arrives when its life is up.
		arrives := tail[0].birthMs + tail[0].lifeMs
		if diff := flash.birthMs - arrives; diff < -1 || diff > 1 {
			t.Errorf("bolt %d lands at %vms and flashes at %v", bolt, arrives, flash.birthMs)
		}
	}
}

// TestSoulStrikeBoltsFlyApart: the bolts are rolled around the line they fly
// down so that five of them are a fan rather than five orbs down one path.
func TestSoulStrikeBoltsFlyApart(t *testing.T) {
	spec := soulStrikeParts(5)

	seen := map[[2]float32]int{}
	for bolt := 0; bolt < 5; bolt++ {
		tail, _ := soulBolt(spec, bolt)
		seen[[2]float32{tail[0].across, tail[0].vy}]++
	}

	if len(seen) != 5 {
		t.Errorf("five bolts fly along %d different paths, want 5", len(seen))
	}

	// One bolt arcs up and over the line rather than out to one side, which
	// is the roll the original gives it: a degree off nothing.
	only, _ := soulBolt(soulStrikeParts(1), 0)
	if only[0].across < -0.5 || only[0].across > 0.5 {
		t.Errorf("a single bolt flies %v units off the line, want it down the middle",
			only[0].across)
	}
	if only[0].vy <= 0 {
		t.Errorf("a single bolt rises by %v, want it to arc over", only[0].vy)
	}
}

// TestSoulStrikeBoltsComeFromDifferentSides: the bolts leave close together,
// so what tells them apart is the arc each takes. Five orbs down one path at
// any spacing are one thick line; on five arcs they read as five however close
// together they left, which is what the original throws.
func TestSoulStrikeBoltsComeFromDifferentSides(t *testing.T) {
	spec := soulStrikeParts(5)

	// Far enough apart to be told apart: a cell across or the best part of
	// one in height.
	const apart = 5

	for a := 0; a < 5; a++ {
		one, _ := soulBolt(spec, a)

		for b := a + 1; b < 5; b++ {
			other, _ := soulBolt(spec, b)

			// The loft is a speed rather than a height, so it is compared as
			// the height it reaches: a quarter of speed times flight.
			side := one[0].across - other[0].across
			high := (one[0].vy - other[0].vy) * soulFlightFrames / 4

			if side*side+high*high < apart*apart {
				t.Errorf("bolts %d and %d fly within %v across and %v high of each other",
					a, b, side, high)
			}
		}
	}

	// And they leave one close behind the next rather than one at a time.
	first, _ := soulBolt(spec, 0)
	second, _ := soulBolt(spec, 1)

	if second[0].birthMs >= first[0].birthMs+first[0].lifeMs {
		t.Errorf("the second bolt leaves at %vms, after the first landed at %v",
			second[0].birthMs, first[0].birthMs+first[0].lifeMs)
	}
}

// TestSoulStrikeBoltsFollowOneAnother: the volley arrives as a stream, and
// each bolt's tail strings out behind its own head.
func TestSoulStrikeBoltsFollowOneAnother(t *testing.T) {
	spec := soulStrikeParts(5)

	heads := make([]float32, 0, 5)
	for bolt := 0; bolt < 5; bolt++ {
		tail, _ := soulBolt(spec, bolt)

		heads = append(heads, tail[0].birthMs)

		for i := 1; i < len(tail); i++ {
			if tail[i].birthMs <= tail[i-1].birthMs {
				t.Fatalf("bolt %d segment %d is born no later than the one in front", bolt, i)
			}
			if tail[i].halfW >= tail[i-1].halfW {
				t.Errorf("bolt %d segment %d is no smaller than the one in front", bolt, i)
			}
		}
	}

	for i := 1; i < len(heads); i++ {
		if heads[i] <= heads[i-1] {
			t.Errorf("bolt %d leaves at %v, no later than the one before at %v",
				i, heads[i], heads[i-1])
		}
	}
}
