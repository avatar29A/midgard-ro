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
		parts, runMs, ok := burstFor(name)
		if !ok {
			t.Errorf("%s has no burst", name)
			continue
		}
		if len(parts) == 0 {
			t.Errorf("%s has a burst with no particles", name)
		}
		if runMs <= 0 {
			t.Errorf("%s runs for %v", name, runMs)
		}
	}

	if _, _, ok := burstFor("EF_FIREHIT"); ok {
		t.Error("EF_FIREHIT has a burst as well as an STR, which would draw it twice")
	}
}

// TestEveryParticleOutlivesItsBirth: a particle born after its burst has
// finished never appears, which is a particle written and not drawn.
func TestEveryParticleOutlivesItsBirth(t *testing.T) {
	for _, name := range burstNames {
		parts, runMs, _ := burstFor(name)

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
		parts, _, _ := burstFor(name)

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
		parts, _, _ := burstFor(name)

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
		parts, runMs, _ := burstFor(name)
		s.playBurst(name, parts, runMs, 0, 0, 0)

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
		first, _, _ := burstFor(name)
		second, _, _ := burstFor(name)

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
	parts, _, _ := burstFor("EF_BASH")

	quadrants := map[int]int{}
	rays := 0

	for _, p := range parts {
		if p.texture != "alpha_center.tga" {
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
	parts, runMs, _ := burstFor("EF_BASH")

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
	parts, _, _ := burstFor("EF_BASH")

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
