package states

import (
	"errors"
	"testing"

	"github.com/Faultbox/midgard-ro/internal/game/skills"
)

func TestMageTargetPlanPreservesBoltsImpactAndLightningLead(t *testing.T) {
	for _, id := range []uint16{14, 19, 20} {
		fx, _ := skills.EffectsOf(id)
		calls := targetVisualPlan(fx.OnTarget, 10)
		effects := map[string]int{}
		for _, c := range calls {
			effects[c.effect]++
		}
		if id == 20 {
			if effects["EF_LIGHTBOLT"] != 10 || effects["EF_WINDHIT"] != 10 {
				t.Fatal(effects)
			}
			if !calls[0].deferred || calls[0].delayMs != 0 || calls[0].sounds[0] != strikeLeadMs([]string{"EF_LIGHTBOLT"}) {
				t.Fatal(calls[0])
			}
		} else {
			if !calls[0].burstOnly || calls[0].hits != 10 || len(calls[0].sounds) != 10 {
				t.Fatal(calls[0])
			}
			impacts := 0
			for _, c := range calls {
				if c.deferred {
					if c.hits != 1 || c.delayMs < boltImpactMs(0) {
						t.Fatal(c)
					}
					impacts++
				}
			}
			if impacts != 10*(len(fx.OnTarget)-1) {
				t.Fatal(id, impacts)
			}
		}
	}
}
func TestMissingMageArtKeepsAvailableProceduralPartsAndReportsGap(t *testing.T) {
	p, err := LoadMagePreview(17, 1, func(string) ([]byte, error) { return nil, errors.New("fixture missing") })
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Warnings) == 0 || p.DurationMS <= 0 || len(p.Textures()) == 0 {
		t.Fatal(p)
	}
	project := func(x, y, z float32) (float32, float32, float32, bool) { return 100 + x, 100 + z - y, 2, true }
	if len(p.QuadsAt(100, [3]float32{}, [3]float32{12, 6, 0}, [3]float32{}, project)) == 0 {
		t.Fatal("lost HIT2 burst because sibling STR is unavailable")
	}
}
