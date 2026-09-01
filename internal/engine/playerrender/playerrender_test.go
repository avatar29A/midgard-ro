package playerrender

import "testing"

// TestOriginOffsetStandsTheSpriteOnItsOrigin: the frame is as big as the
// tallest pose in the sheet needs, and the standing pose sits somewhere inside
// it. The quad has to be moved so the sprite's own origin lands on the unit,
// or the character is lifted by whatever room the rest of the sheet wanted —
// which is why putting a hat on made everyone hover.
func TestOriginOffsetStandsTheSpriteOnItsOrigin(t *testing.T) {
	r := &Renderer{}

	// A frame twice as tall as it needs to be, with the origin in the middle.
	sh := &sheet{width: 100, height: 200}
	sh.originX = 50
	sh.originY = 100

	x, y := r.originOffset(sh, 10, 20)

	if x != 0 {
		t.Errorf("origin offset x = %v, want none for an origin on the middle", x)
	}

	// The origin is halfway down, so the quad has to drop half its height for
	// that row to reach the unit.
	if y != 10 {
		t.Errorf("origin offset y = %v, want half the quad's height", y)
	}
}

// TestOriginOffsetForAFrameThatNeedsNoMoving: an origin already on the
// bottom-middle is where the quad stands anyway.
func TestOriginOffsetForAFrameThatNeedsNoMoving(t *testing.T) {
	r := &Renderer{}

	sh := &sheet{width: 100, height: 200}
	sh.originX = 50
	sh.originY = 200

	if x, y := r.originOffset(sh, 10, 20); x != 0 || y != 0 {
		t.Errorf("origin offset = (%v %v), want none", x, y)
	}
}

// TestOriginOffsetWithoutASheet: nothing baked yet answers rather than
// dividing by zero.
func TestOriginOffsetWithoutASheet(t *testing.T) {
	r := &Renderer{}

	if x, y := r.originOffset(nil, 10, 20); x != 0 || y != 0 {
		t.Errorf("a nil sheet gave (%v %v)", x, y)
	}
	if x, y := r.originOffset(&sheet{}, 10, 20); x != 0 || y != 0 {
		t.Errorf("an empty sheet gave (%v %v)", x, y)
	}
}
