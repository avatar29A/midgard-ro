package states

import (
	"testing"

	"github.com/Faultbox/midgard-ro/pkg/formats"
)

// threeFrames is an ACT of three frames at a hundred milliseconds each,
// pointing at sprite frames 5, 6 and 7 with an offset of its own.
func threeFrames() *formats.ACT {
	frame := func(id int32) formats.Frame {
		return formats.Frame{Layers: []formats.Layer{{SpriteID: id, X: 3, Y: -4}}}
	}

	return &formats.ACT{
		Actions:   []formats.Action{{Frames: []formats.Frame{frame(5), frame(6), frame(7)}}},
		Intervals: []float32{4}, // four ticks of twenty-five milliseconds
	}
}

// TestASpriteEffectRunsAtItsOwnRate: the ACT says how long a frame is held,
// and a Fire Wall burns at a different pace from a Sight turning.
func TestASpriteEffectRunsAtItsOwnRate(t *testing.T) {
	effect := &spriteEffect{act: threeFrames()}

	for _, tc := range []struct {
		ageMs float32
		want  int
	}{
		{0, 5}, {99, 5}, {100, 6}, {201, 7},
	} {
		at, ok := effect.frameAt(tc.ageMs)
		if !ok {
			t.Fatalf("nothing to draw at %vms", tc.ageMs)
		}

		if at.frame != tc.want {
			t.Errorf("at %vms it draws frame %d, want %d", tc.ageMs, at.frame, tc.want)
		}
	}
}

// TestAStandingEffectLoops: a wall stands until the server takes it away, and
// one frozen on its last frame reads as a fire that has gone out.
func TestAStandingEffectLoops(t *testing.T) {
	effect := &spriteEffect{act: threeFrames()}

	at, ok := effect.frameAt(300)
	if !ok || at.frame != 5 {
		t.Errorf("after its last frame it draws %d, want it back at 5", at.frame)
	}
}

// TestAnEffectWithAnEndStopsOnItsLastFrame: one that was given a length plays
// through rather than repeating.
func TestAnEffectWithAnEndStopsOnItsLastFrame(t *testing.T) {
	effect := &spriteEffect{act: threeFrames(), runMs: 300}

	at, ok := effect.frameAt(1000)
	if !ok || at.frame != 7 {
		t.Errorf("past its end it draws %d, want its last frame 7", at.frame)
	}
}

// TestAFrameKeepsItsOwnOffset: the ACT says where each frame sits around the
// point the effect plays at, and drawn centered they jump about.
func TestAFrameKeepsItsOwnOffset(t *testing.T) {
	effect := &spriteEffect{act: threeFrames()}

	at, _ := effect.frameAt(0)
	if at.offX != 3 || at.offY != -4 {
		t.Errorf("the frame sits at %v,%v, want the act's 3,-4", at.offX, at.offY)
	}
}

// TestAnEffectWithNothingToDraw: an ACT with no frames, or a frame pointing at
// no sprite, draws nothing rather than reaching past the end of the sprite.
func TestAnEffectWithNothingToDraw(t *testing.T) {
	for _, tc := range []struct {
		name string
		act  *formats.ACT
	}{
		{"no act at all", nil},
		{"no actions", &formats.ACT{}},
		{"no frames", &formats.ACT{Actions: []formats.Action{{}}}},
		{"a frame with no layers", &formats.ACT{
			Actions: []formats.Action{{Frames: []formats.Frame{{}}}},
		}},
		{"a layer pointing at nothing", &formats.ACT{
			Actions: []formats.Action{{Frames: []formats.Frame{
				{Layers: []formats.Layer{{SpriteID: -1}}},
			}}},
		}},
	} {
		effect := &spriteEffect{act: tc.act}
		if _, ok := effect.frameAt(0); ok {
			t.Errorf("%s: something was drawn", tc.name)
		}
	}
}

// TestAWallGoesWhenItsUnitDoes: a ground skill's effect stands for as long as
// the server says the unit stands there, and no longer.
func TestAWallGoesWhenItsUnitDoes(t *testing.T) {
	s := &InGameState{}
	s.spriteEffects = []*spriteEffect{
		{name: "firewall", unit: 10},
		{name: "firewall", unit: 11},
		{name: "sight"},
	}

	s.hideSkillUnit(10)

	if len(s.spriteEffects) != 2 {
		t.Fatalf("%d effects left, want the other wall and the sight", len(s.spriteEffects))
	}
	for _, effect := range s.spriteEffects {
		if effect.unit == 10 {
			t.Error("the wall whose unit went is still burning")
		}
	}
}

// TestAUnitEffectLoopsUntilItsUnitGoes: a Safety Wall is an STR rather than a
// sprite, and its animation is a second long where the wall stands for half a
// minute. Played once it is a flash on a cell; dropped with the unit, it is a
// wall that goes when the server says so.
func TestAUnitEffectLoopsUntilItsUnitGoes(t *testing.T) {
	const runMs = 1000

	s := &InGameState{}
	s.effects = []*activeEffect{
		{str: &formats.STR{FPS: 60, MaxKey: 60}, unit: 20, loop: true},
		{str: &formats.STR{FPS: 60, MaxKey: 60}, unit: 21, loop: true},
		{str: &formats.STR{FPS: 60, MaxKey: 60}},
	}

	// Three times round and it is still there, at the age its own animation
	// is up to rather than at three seconds.
	for i := 0; i < 3; i++ {
		s.updateEffects(runMs)
	}

	if len(s.effects) != 2 {
		t.Fatalf("%d effects left after three seconds, want the two walls", len(s.effects))
	}
	for _, e := range s.effects {
		if e.ageMs >= runMs {
			t.Errorf("a looping effect is %vms old, past the end of its own animation", e.ageMs)
		}
	}

	s.hideUnitEffect(20)

	if len(s.effects) != 1 || s.effects[0].unit != 21 {
		t.Fatalf("hiding unit 20 left %d effects, want only the other wall", len(s.effects))
	}
}
