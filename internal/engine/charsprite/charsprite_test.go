package charsprite

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Faultbox/midgard-ro/pkg/formats"
)

func TestBodyPaths(t *testing.T) {
	tests := []struct {
		name    string
		spec    Spec
		wantSPR string
	}{
		{
			// The seeded test character: class 0, male, hair 1. Verified
			// present in data.grf.
			name:    "male novice",
			spec:    Spec{Job: 0, HairStyle: 1},
			wantSPR: `data\sprite\인간족\몸통\남\초보자_남.spr`,
		},
		{
			name:    "female novice",
			spec:    Spec{Job: 0, Female: true, HairStyle: 1},
			wantSPR: `data\sprite\인간족\몸통\여\초보자_여.spr`,
		},
		{
			name:    "male swordman",
			spec:    Spec{Job: 1},
			wantSPR: `data\sprite\인간족\몸통\남\검사_남.spr`,
		},
		{
			// An id we don't have a name for must still resolve, as a novice.
			name:    "unknown job falls back to novice",
			spec:    Spec{Job: 4211},
			wantSPR: `data\sprite\인간족\몸통\남\초보자_남.spr`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotSPR, gotACT := tt.spec.BodyPaths()
			if gotSPR != tt.wantSPR {
				t.Errorf("body SPR = %q, want %q", gotSPR, tt.wantSPR)
			}
			if want := strings.TrimSuffix(tt.wantSPR, ".spr") + ".act"; gotACT != want {
				t.Errorf("body ACT = %q, want %q", gotACT, want)
			}
		})
	}
}

func TestHeadPaths(t *testing.T) {
	tests := []struct {
		name    string
		spec    Spec
		wantSPR string
	}{
		{
			name:    "male hair 1",
			spec:    Spec{HairStyle: 1},
			wantSPR: `data\sprite\인간족\머리통\남\1_남.spr`,
		},
		{
			name:    "female hair 12",
			spec:    Spec{Female: true, HairStyle: 12},
			wantSPR: `data\sprite\인간족\머리통\여\12_여.spr`,
		},
		{
			// Style 0 isn't a file in the archive; every sex has a style 1.
			name:    "hair 0 becomes hair 1",
			spec:    Spec{HairStyle: 0},
			wantSPR: `data\sprite\인간족\머리통\남\1_남.spr`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotSPR, _ := tt.spec.HeadPaths()
			if gotSPR != tt.wantSPR {
				t.Errorf("head SPR = %q, want %q", gotSPR, tt.wantSPR)
			}
		})
	}
}

func TestJobSpriteNameReportsUnknown(t *testing.T) {
	if _, ok := JobSpriteName(0); !ok {
		t.Error("job 0 (Novice) should be known")
	}
	name, ok := JobSpriteName(9999)
	if ok {
		t.Error("job 9999 should report as unknown")
	}
	if novice, _ := JobSpriteName(FallbackJob); name != novice {
		t.Errorf("unknown job returned %q, want the novice sprite %q", name, novice)
	}
}

func TestBuildSheetPadsFramesToUniformSize(t *testing.T) {
	// Two directions whose sprites differ in size: the sheet must pad both to
	// the larger one, so switching facing doesn't resize the billboard.
	bodySPR := &formats.SPR{Images: []formats.SPRImage{
		makeImage(20, 40),
		makeImage(30, 60),
	}}

	act := &formats.ACT{}
	for dir := 0; dir < Directions*LoadedActions; dir++ {
		spriteID := int32(0)
		if dir == 3 {
			spriteID = 1 // one oversized direction
		}
		act.Actions = append(act.Actions, formats.Action{
			Frames: []formats.Frame{{
				Layers:       []formats.Layer{{SpriteID: spriteID, ScaleX: 1, ScaleY: 1}},
				AnchorPoints: []formats.AnchorPoint{{X: 0, Y: 0}},
			}},
		})
	}

	sheet := BuildSheet(bodySPR, act, nil, nil, HeadStraight)
	if sheet == nil {
		t.Fatal("BuildSheet returned nil")
	}
	if sheet.Width != 30 || sheet.Height != 60 {
		t.Errorf("sheet = %dx%d, want 30x60 (the largest frame)", sheet.Width, sheet.Height)
	}

	want := sheet.Width * sheet.Height * 4
	for key, frames := range sheet.Frames {
		for i, f := range frames {
			if len(f.Pixels) != want {
				t.Errorf("frame %d of set %d has %d bytes, want %d", i, key, len(f.Pixels), want)
			}
		}
	}
}

func TestBuildSheetCoversAllDirections(t *testing.T) {
	bodySPR := &formats.SPR{Images: []formats.SPRImage{makeImage(20, 40)}}
	act := &formats.ACT{}
	for i := 0; i < Directions*LoadedActions; i++ {
		act.Actions = append(act.Actions, formats.Action{
			Frames: []formats.Frame{
				{Layers: []formats.Layer{{SpriteID: 0, ScaleX: 1, ScaleY: 1}}},
				{Layers: []formats.Layer{{SpriteID: 0, ScaleX: 1, ScaleY: 1}}},
			},
		})
	}

	sheet := BuildSheet(bodySPR, act, nil, nil, HeadStraight)
	if sheet == nil {
		t.Fatal("BuildSheet returned nil")
	}

	for dir := 0; dir < Directions; dir++ {
		// Idle collapses to the single chosen head pose; walk keeps its
		// animation frames.
		if got := sheet.FrameCount(ActionIdle, dir); got != 1 {
			t.Errorf("idle dir %d has %d frames, want 1 (one head pose, not an animation)", dir, got)
		}
		if got := sheet.FrameCount(ActionWalk, dir); got != 2 {
			t.Errorf("walk dir %d has %d frames, want 2", dir, got)
		}
	}
}

func TestBuildSheetNilBody(t *testing.T) {
	if got := BuildSheet(nil, nil, nil, nil, HeadStraight); got != nil {
		t.Error("BuildSheet with no body should return nil")
	}
}

func TestLoadMissingBodyIsAnError(t *testing.T) {
	loader := func(string) ([]byte, error) { return nil, fmt.Errorf("not found") }
	if _, err := Load(loader, Spec{Job: 0}); err == nil {
		t.Error("a missing body sprite must be an error")
	}
}

func makeImage(w, h int) formats.SPRImage {
	px := make([]byte, w*h*4)
	for i := 0; i < w*h; i++ {
		px[i*4+3] = 255
	}
	return formats.SPRImage{Width: uint16(w), Height: uint16(h), Pixels: px}
}
