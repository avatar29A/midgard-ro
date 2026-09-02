package charsprite

import (
	"testing"

	"github.com/Faultbox/midgard-ro/pkg/formats"
)

// frames builds an action whose named frames carry an event.
func frames(count int, events map[int]int32) []formats.Frame {
	out := make([]formats.Frame, count)
	for i := range out {
		out[i].EventID = -1
		if id, ok := events[i]; ok {
			out[i].EventID = id
		}
	}

	return out
}

// TestHitFrameTakesTheMarkedFrame: the sprite says where its blow lands, on
// the frame carrying an event. A Swordman's sword swing is nine frames and
// marks the sixth with attack_sword.wav, which is where the blade arrives.
func TestHitFrameTakesTheMarkedFrame(t *testing.T) {
	frame, sound := hitFrame(frames(9, map[int]int32{5: 0}), []string{"attack_sword.wav"})

	if frame != 5 {
		t.Errorf("hit frame = %d, want 5", frame)
	}
	if sound != "attack_sword.wav" {
		t.Errorf("hit sound = %q, want attack_sword.wav", sound)
	}
}

// TestHitFrameFallsBackToTheFrameBeforeLast: rAthena's note on clif_damage
// says the client shows the damage at the second to last frame when the
// animation marks none, and the server times its own damage against that. A
// different fallback puts the two out of step.
func TestHitFrameFallsBackToTheFrameBeforeLast(t *testing.T) {
	for _, tc := range []struct{ count, want int }{
		{9, 7}, {4, 2}, {2, 0},
	} {
		frame, sound := hitFrame(frames(tc.count, nil), nil)

		if frame != tc.want {
			t.Errorf("%d frames: hit frame = %d, want %d", tc.count, frame, tc.want)
		}
		if sound != "" {
			t.Errorf("%d frames: unmarked action named the sound %q", tc.count, sound)
		}
	}
}

// TestHitFrameOfAShortAction: one frame, or none, has nowhere to wait — the
// blow lands as it starts, which is what a zero says.
func TestHitFrameOfAShortAction(t *testing.T) {
	for _, count := range []int{0, 1} {
		if frame, _ := hitFrame(frames(count, nil), nil); frame != 0 {
			t.Errorf("%d frames: hit frame = %d, want 0", count, frame)
		}
	}
}

// TestHitFramePrefersTheDamageMark: every monster in the archive marks its
// damage frame with "atk", which is a marker rather than a sound and sits a
// frame or so after the noise. A Poring makes its noise on frame 11 and lands
// on 12.
func TestHitFramePrefersTheDamageMark(t *testing.T) {
	frame, sound := hitFrame(
		frames(28, map[int]int32{11: 1, 12: 0}),
		[]string{damageMark, "poring_attack.wav"})

	if frame != 12 {
		t.Errorf("hit frame = %d, want the atk mark at 12", frame)
	}
	if sound != "poring_attack.wav" {
		t.Errorf("hit sound = %q, want poring_attack.wav", sound)
	}
}

// TestHitFrameTakesTheLastSound: without an "atk" the sound is the best the
// sprite offers, and the last one is the blow — a hornet buzzes its wind-up on
// frame 0 and stings on frame 7, so the first mark is the wrong one.
func TestHitFrameTakesTheLastSound(t *testing.T) {
	frame, sound := hitFrame(
		frames(11, map[int]int32{0: 0, 7: 1}),
		[]string{"hornet_attack1.wav", "hornet_attack2.wav"})

	if frame != 7 {
		t.Errorf("hit frame = %d, want the sting at 7", frame)
	}
	if sound != "hornet_attack2.wav" {
		t.Errorf("hit sound = %q, want the sting", sound)
	}
}

// TestHitFrameIgnoresAnEventIDPastTheTable: a mark pointing at a name that is
// not there says nothing, and is passed over rather than trusted — falling
// back to the frame before the last, which is where an unmarked action lands.
func TestHitFrameIgnoresAnEventIDPastTheTable(t *testing.T) {
	frame, sound := hitFrame(frames(9, map[int]int32{5: 7}), []string{"only.wav"})

	if frame != 7 {
		t.Errorf("hit frame = %d, want the fallback at 7", frame)
	}
	if sound != "" {
		t.Errorf("hit sound = %q, want none", sound)
	}
}
