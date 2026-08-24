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

// TestCompositeHeadTracksBodyFrameAnchor is the regression guard for heads
// detaching from bodies. Head ACTs carry one anchor per frame that tracks the
// body's; pinning the head to frame 0 mismatches the pair on every frame where
// the body's anchor moves, which left the novice's head floating during idle.
//
// Both frames below place body and head anchors at the same spot, so a
// correctly aligned composite is the same size for both frames. Pinning the
// head to frame 0 would drag it 40px away on frame 1 and balloon the canvas.
func TestCompositeHeadTracksBodyFrameAnchor(t *testing.T) {
	bodySPR := solidSPR(1, 20, 40, 255, 0, 0)
	headSPR := solidSPR(1, 16, 16, 0, 0, 255)

	bodyACT := actWithAnchors([][2]int16{{0, -30}, {0, -70}})
	headACT := actWithAnchors([][2]int16{{0, -30}, {0, -70}})

	frame0 := CompositeSprites(bodySPR, bodyACT, headSPR, headACT, 0, 0, 0)
	frame1 := CompositeSprites(bodySPR, bodyACT, headSPR, headACT, 0, 0, 1)

	if frame0.Width == 0 || frame1.Width == 0 {
		t.Fatal("composites came back empty")
	}
	if frame0.Width != frame1.Width || frame0.Height != frame1.Height {
		t.Errorf("frame sizes differ: frame0 %dx%d, frame1 %dx%d — the head did not "+
			"follow the body's per-frame anchor",
			frame0.Width, frame0.Height, frame1.Width, frame1.Height)
	}
}

// TestCompositeHeadOptional covers monsters and any character whose head
// sprite is missing from the archive: the body must still render.
func TestCompositeHeadOptional(t *testing.T) {
	bodySPR := solidSPR(1, 20, 40, 255, 0, 0)
	bodyACT := actWithAnchors([][2]int16{{0, -30}})

	got := CompositeSprites(bodySPR, bodyACT, nil, nil, 0, 0, 0)

	if got.Width != 20 || got.Height != 40 {
		t.Errorf("body-only composite = %dx%d, want 20x40", got.Width, got.Height)
	}
	if len(got.Pixels) != 20*40*4 {
		t.Errorf("pixel buffer = %d bytes, want %d", len(got.Pixels), 20*40*4)
	}
}

func TestCompositeNilBody(t *testing.T) {
	if got := CompositeSprites(nil, nil, nil, nil, 0, 0, 0); got.Pixels != nil {
		t.Error("nil body should produce an empty result, not a panic or pixels")
	}
}

// TestCompositeFrameIndexWraps checks that a body with more frames than the
// head doesn't index past the head's frame list.
func TestCompositeFrameIndexWraps(t *testing.T) {
	bodySPR := solidSPR(1, 20, 40, 255, 0, 0)
	headSPR := solidSPR(1, 16, 16, 0, 0, 255)

	bodyACT := actWithAnchors([][2]int16{{0, -30}, {0, -30}, {0, -30}})
	headACT := actWithAnchors([][2]int16{{0, -30}}) // only one head frame

	for frame := 0; frame < 3; frame++ {
		got := CompositeSprites(bodySPR, bodyACT, headSPR, headACT, 0, 0, frame)
		if got.Width == 0 {
			t.Errorf("frame %d produced an empty composite", frame)
		}
	}
}
