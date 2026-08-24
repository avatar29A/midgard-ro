package sprite

import (
	"testing"

	"github.com/Faultbox/midgard-ro/pkg/formats"
)

// solidSPR builds an SPR with one opaque w x h image per entry.
func solidSPR(count, w, h int, r, g, b byte) *formats.SPR {
	spr := &formats.SPR{}
	for i := 0; i < count; i++ {
		px := make([]byte, w*h*4)
		for p := 0; p < w*h; p++ {
			px[p*4], px[p*4+1], px[p*4+2], px[p*4+3] = r, g, b, 255
		}
		spr.Images = append(spr.Images, formats.SPRImage{
			Width:  uint16(w),
			Height: uint16(h),
			Pixels: px,
		})
	}
	return spr
}

// actWithAnchors builds an ACT with a single action whose frames each carry
// one layer at the origin and the given anchor point.
func actWithAnchors(anchors [][2]int16) *formats.ACT {
	act := &formats.ACT{}
	action := formats.Action{}
	for _, a := range anchors {
		action.Frames = append(action.Frames, formats.Frame{
			Layers:       []formats.Layer{{SpriteID: 0, X: 0, Y: 0, ScaleX: 1, ScaleY: 1}},
			AnchorPoints: []formats.AnchorPoint{{X: int32(a[0]), Y: int32(a[1])}},
		})
	}
	act.Actions = append(act.Actions, action)
	return act
}

// TestCompositeAlignsChosenHeadPose is the regression guard for heads
// detaching from bodies: whichever head pose the caller picks must be aligned
// against the body frame it is drawn with, using both of their anchors.
//
// Both pairs below place body and head anchors at the same spot, so a
// correctly aligned composite is the same size either way. Ignoring one of the
// two anchors drags the head 40px off and balloons the canvas.
func TestCompositeAlignsChosenHeadPose(t *testing.T) {
	bodySPR := solidSPR(1, 20, 40, 255, 0, 0)
	headSPR := solidSPR(1, 16, 16, 0, 0, 255)

	bodyACT := actWithAnchors([][2]int16{{0, -30}, {0, -70}})
	headACT := actWithAnchors([][2]int16{{0, -30}, {0, -70}})

	frame0 := CompositeSprites(bodySPR, bodyACT, headSPR, headACT, 0, 0, 0, 0)
	frame1 := CompositeSprites(bodySPR, bodyACT, headSPR, headACT, 0, 0, 1, 1)

	if frame0.Width == 0 || frame1.Width == 0 {
		t.Fatal("composites came back empty")
	}
	if frame0.Width != frame1.Width || frame0.Height != frame1.Height {
		t.Errorf("frame sizes differ: frame0 %dx%d, frame1 %dx%d — the head was not "+
			"aligned against the body frame it was drawn with",
			frame0.Width, frame0.Height, frame1.Width, frame1.Height)
	}
}

// TestCompositeHeadOptional covers monsters and any character whose head
// sprite is missing from the archive: the body must still render.
func TestCompositeHeadOptional(t *testing.T) {
	bodySPR := solidSPR(1, 20, 40, 255, 0, 0)
	bodyACT := actWithAnchors([][2]int16{{0, -30}})

	got := CompositeSprites(bodySPR, bodyACT, nil, nil, 0, 0, 0, 0)

	if got.Width != 20 || got.Height != 40 {
		t.Errorf("body-only composite = %dx%d, want 20x40", got.Width, got.Height)
	}
	if len(got.Pixels) != 20*40*4 {
		t.Errorf("pixel buffer = %d bytes, want %d", len(got.Pixels), 20*40*4)
	}
}

func TestCompositeNilBody(t *testing.T) {
	if got := CompositeSprites(nil, nil, nil, nil, 0, 0, 0, 0); got.Pixels != nil {
		t.Error("nil body should produce an empty result, not a panic or pixels")
	}
}

// TestCompositeFrameIndexWraps checks that a body with more frames than the
// head doesn't index past the head's frame list. This is the normal case while
// walking: 8 body frames against 3 head poses.
func TestCompositeFrameIndexWraps(t *testing.T) {
	bodySPR := solidSPR(1, 20, 40, 255, 0, 0)
	headSPR := solidSPR(1, 16, 16, 0, 0, 255)

	bodyACT := actWithAnchors([][2]int16{{0, -30}, {0, -30}, {0, -30}})
	headACT := actWithAnchors([][2]int16{{0, -30}}) // only one head frame

	for frame := 0; frame < 3; frame++ {
		got := CompositeSprites(bodySPR, bodyACT, headSPR, headACT, 0, 0, frame, 0)
		if got.Width == 0 {
			t.Errorf("frame %d produced an empty composite", frame)
		}
	}
}

// TestCompositeHeadPoseIsIndependentOfBodyFrame is the guard for the swivelling
// head: walking cycles 8 body frames against a head that must hold one pose.
// Feeding the body's frame index to the head made it turn left/right/straight
// on a 3-frame loop while the legs ran on an 8-frame one.
func TestCompositeHeadPoseIsIndependentOfBodyFrame(t *testing.T) {
	bodySPR := solidSPR(1, 20, 40, 255, 0, 0)
	headSPR := solidSPR(2, 16, 16, 0, 0, 255)

	// Body walk cycle: anchor stays put, so the head should too.
	bodyACT := actWithAnchors([][2]int16{{0, -30}, {0, -30}, {0, -30}})
	// Head poses: straight, then turned well off to one side.
	headACT := actWithAnchors([][2]int16{{0, -30}, {40, -30}, {-40, -30}})

	first := CompositeSprites(bodySPR, bodyACT, headSPR, headACT, 0, 0, 0, 0)
	for frame := 1; frame < 3; frame++ {
		got := CompositeSprites(bodySPR, bodyACT, headSPR, headACT, 0, 0, frame, 0)
		if got.Width != first.Width || got.Height != first.Height {
			t.Errorf("body frame %d changed the composite to %dx%d (want %dx%d) — "+
				"the head pose moved with the body frame",
				frame, got.Width, got.Height, first.Width, first.Height)
		}
	}
}
