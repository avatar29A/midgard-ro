package effect

import (
	"testing"

	"github.com/Faultbox/midgard-ro/pkg/formats"
)

// oneLayer is an effect of a single layer, for stating key frames directly.
func oneLayer(textures []string, keys ...formats.STRKey) *formats.STR {
	return &formats.STR{
		FPS:    60,
		MaxKey: 160,
		Layers: []formats.STRLayer{{Textures: textures, Keys: keys}},
	}
}

// effectOriginX is the x every key frame in the archive's effects sits at:
// the middle of the original client's window.
const effectOriginX = 320

// basicKey is a key frame with the fields these tests care about.
func basicKey(frame int, y, alpha float32) formats.STRKey {
	return formats.STRKey{
		Frame: frame, Kind: formats.STRKeyBasic,
		X: effectOriginX, Y: y,
		U:         [8]float32{0, 0, 1, 1, 0, 0, 1, 1},
		Color:     [4]float32{255, 255, 255, alpha},
		DestAlpha: 2,
	}
}

// TestMorphKeysArePerFrameDeltas: this is the one reading of the format that
// everything else rests on, and it is checkable against the real file — a
// level-up layer sits at y 81 on frame 10, its morph key carries 32.25, and
// the next basic key at frame 14 is at 210, which is four steps of 32.25.
func TestMorphKeysArePerFrameDeltas(t *testing.T) {
	str := oneLayer([]string{"beam.bmp"},
		basicKey(10, 81, 255),
		formats.STRKey{Frame: 10, Kind: formats.STRKeyMorph, Y: 32.25},
		basicKey(14, 210, 255),
	)

	// Frame 14 is 233.3ms in at 60fps. One frame earlier — 13 — should be
	// three steps along, at 81 + 96.75.
	quads := Frames(str, 13.0*1000/60)
	if len(quads) != 1 {
		t.Fatalf("got %d quads, want 1", len(quads))
	}

	wantY := float32(81+3*32.25) - OriginY
	if got := quads[0].Corners[0][1]; got != wantY {
		t.Errorf("y at frame 13 = %v, want %v", got, wantY)
	}
}

// TestALayerIsSilentBeforeItStarts: the layers of an effect begin at different
// frames, which is what makes it build rather than appear whole.
func TestALayerIsSilentBeforeItStarts(t *testing.T) {
	str := oneLayer([]string{"beam.bmp"}, basicKey(10, 81, 255))

	if quads := Frames(str, 0); len(quads) != 0 {
		t.Errorf("a layer starting at frame 10 drew %d quads at time zero", len(quads))
	}
}

// TestALayerEndsAtItsLastKey: without this the final values hold forever, and
// a level-up leaves a flash on screen that never goes out.
func TestALayerEndsAtItsLastKey(t *testing.T) {
	str := oneLayer([]string{"beam.bmp"},
		basicKey(0, 81, 255),
		basicKey(10, 81, 255),
	)

	if quads := Frames(str, 5.0*1000/60); len(quads) != 1 {
		t.Errorf("mid-effect drew %d quads, want 1", len(quads))
	}
	if quads := Frames(str, 60.0*1000/60); len(quads) != 0 {
		t.Errorf("past the last key it drew %d quads, want none", len(quads))
	}
}

// TestATransparentLayerIsNotDrawn: a layer whose fade has finished costs a
// draw call and shows nothing.
func TestATransparentLayerIsNotDrawn(t *testing.T) {
	str := oneLayer([]string{"beam.bmp"},
		basicKey(0, 81, 0),
		basicKey(10, 81, 0),
	)

	if quads := Frames(str, 5.0*1000/60); len(quads) != 0 {
		t.Errorf("a fully transparent layer drew %d quads", len(quads))
	}
}

// TestTextureCoordinatesCoverTheWholeTexture: the eight floats are a source
// rectangle written twice, not a coordinate per corner. Read per corner, two
// corners share a coordinate and the art smears along a diagonal.
func TestTextureCoordinatesCoverTheWholeTexture(t *testing.T) {
	str := oneLayer([]string{"beam.bmp"},
		basicKey(0, 81, 255),
		basicKey(10, 81, 255),
	)

	quads := Frames(str, 5.0*1000/60)
	if len(quads) != 1 {
		t.Fatalf("got %d quads, want 1", len(quads))
	}

	want := [4][2]float32{{0, 0}, {1, 0}, {1, 1}, {0, 1}}
	if quads[0].UV != want {
		t.Errorf("UV = %v, want the whole texture as %v", quads[0].UV, want)
	}
}

// TestColorIsScaledToUnit: the file stores 0..255 and the renderer wants
// 0..1, and a color left unscaled is a quad drawn 255 times too bright.
func TestColorIsScaledToUnit(t *testing.T) {
	str := oneLayer([]string{"beam.bmp"},
		basicKey(0, 81, 255),
		basicKey(10, 81, 255),
	)

	quads := Frames(str, 5.0*1000/60)
	if len(quads) != 1 {
		t.Fatalf("got %d quads, want 1", len(quads))
	}

	if quads[0].Color != [4]float32{1, 1, 1, 1} {
		t.Errorf("color = %v, want white at full alpha in 0..1", quads[0].Color)
	}
}

// TestTextureIndexHoldsAtTheEnd: a layer that stepped past its last texture
// holds it rather than wrapping, or the art keeps flickering after everything
// else has faded.
func TestTextureIndexHoldsAtTheEnd(t *testing.T) {
	layer := formats.STRLayer{Textures: []string{"a.bmp", "b.bmp"}}

	for _, tc := range []struct {
		frame float32
		want  string
	}{
		{-1, "a.bmp"},
		{0, "a.bmp"},
		{1, "b.bmp"},
		{5, "b.bmp"},
	} {
		if got := textureAt(&layer, tc.frame); got != tc.want {
			t.Errorf("textureAt(%v) = %q, want %q", tc.frame, got, tc.want)
		}
	}

	if got := textureAt(&formats.STRLayer{}, 0); got != "" {
		t.Errorf("a layer with no texture gave %q", got)
	}
}

// TestFramesHandlesNothing: an effect that could not be loaded plays nothing
// rather than crashing whatever asked for it.
func TestFramesHandlesNothing(t *testing.T) {
	if quads := Frames(nil, 100); quads != nil {
		t.Errorf("a nil effect gave %d quads", len(quads))
	}
	if quads := Frames(&formats.STR{}, 100); len(quads) != 0 {
		t.Errorf("an effect with no frame rate gave %d quads", len(quads))
	}
}
