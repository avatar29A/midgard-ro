package states

import (
	"reflect"
	"testing"

	"github.com/Faultbox/midgard-ro/internal/engine/skillvisual"
)

func TestSoulPreviewMatchesGameAndSeeksIndependently(t *testing.T) {
	d := skillvisual.BuiltInSoulDefinition()
	from, at := [3]float32{10, 30, 15}, [3]float32{50, 25, 30}
	project := func(x, y, z float32) (float32, float32, float32, bool) { return x + 100, y + z + 100, 2, true }
	for _, hits := range []int{1, 3, 5, 20} {
		spec := soulStrikeParts(hits)
		game := activeBurst{parts: spec.parts, runMs: spec.runMs, x: at[0], y: at[1], z: at[2], otherX: from[0] - at[0], otherY: from[1] - at[1], otherZ: from[2] - at[2]}
		preview := NewSoulStrikePreview(hits, from, at, d)
		for _, tick := range []int{0, 1, 25, 26, 40, 99, 10, 26, 0} {
			game.ageMs = float32(tick) * 1000 / 60
			if !reflect.DeepEqual(game.quadsAt(project), preview.QuadsAt(tick, project)) {
				t.Fatalf("hits=%d tick=%d diverged from game", hits, tick)
			}
		}
	}
}
func TestSoulDefinitionChangesSharedGeometry(t *testing.T) {
	d := skillvisual.BuiltInSoulDefinition()
	a := soulStrikePartsWithDefinition(5, d)
	d.HalfSize *= 2
	d.Spread = 0
	b := soulStrikePartsWithDefinition(5, d)
	if b.parts[0].halfW != 2*a.parts[0].halfW || b.parts[0].across != 0 {
		t.Fatal("parameters did not reach generator")
	}
	if a.runMs != b.runMs {
		t.Fatal("spatial parameters changed timing")
	}
}

func TestCastAuraSharesGameShapeAndStopsAtBoundary(t *testing.T) {
	at := CastAuraAt(250, 500)
	if at.Bottom != castAuraRadius || at.Height != castAuraHeightMax/2 || at.Alpha != 1 {
		t.Fatal(at)
	}
	if a := CastAuraAt(499, 500); a.Alpha <= 0 || a.Alpha >= 1 {
		t.Fatal(a)
	}
	if a := CastAuraAt(500, 500); a != (CastAuraShape{}) {
		t.Fatal(a)
	}
	if a := CastAuraAt(0, 0); a != (CastAuraShape{}) {
		t.Fatal(a)
	}
}
