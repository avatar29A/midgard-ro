package states

import (
	"reflect"
	"testing"

	"github.com/Faultbox/midgard-ro/pkg/formats"
)

func TestSightPreviewUsesGameParticlesAndKeepsWorldAnchor(t *testing.T) {
	p := PersistentPreview{skill: 10}
	at := [3]float32{30, 5, 10}
	project := func(x, y, z float32) (float32, float32, float32, bool) { return x + 100, z - y + 100, 2, true }
	for _, age := range []float32{0, 100, 3000, 25} {
		game := activeBurst{parts: sightTrailParts(age), x: at[0], y: at[1], z: at[2]}
		if !reflect.DeepEqual(game.quadsAt(project), p.QuadsAt(age, at, project)) {
			t.Fatal(age)
		}
	}
	if p.QuadsAt(-1, at, project) != nil {
		t.Fatal("visible before activation")
	}
}
func TestFireWallPreviewUsesACTTimingOffsetsAndLoop(t *testing.T) {
	images := make([]formats.SPRImage, 8)
	for i := range images {
		images[i].Width = 64
		images[i].Height = 64
	}
	p := PersistentPreview{skill: 18, act: threeFrames(), spr: &formats.SPR{Images: images}}
	project := func(x, y, z float32) (float32, float32, float32, bool) { return x + 100, z - y + 100, 2, true }
	first := p.QuadsAt(0, [3]float32{}, project)
	if len(first) != 1 {
		t.Fatal(first)
	}
	if first[0].Texture == p.QuadsAt(100, [3]float32{}, project)[0].Texture {
		t.Fatal("ACT did not advance")
	}
	if !reflect.DeepEqual(first, p.QuadsAt(300, [3]float32{}, project)) {
		t.Fatal("loop did not return to first frame")
	}
	if first[0].Corners[0][1] >= first[0].Corners[2][1] {
		t.Fatal("inverted vertical placement")
	}
}
