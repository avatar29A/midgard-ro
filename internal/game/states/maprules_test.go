package states

import (
	"math"
	"testing"

	"github.com/Faultbox/midgard-ro/pkg/formats"
)

func TestMapRulesFor(t *testing.T) {
	rules := &MapRules{
		Indoor: map[string]bool{"prt_in": true},
		Presets: map[string]formats.ViewPoint{
			"mosk_fild01": {RotationFrom: 30, RotationTo: 120, RotationIn: 0},
			"jupe_gate":   {RotationFrom: 0, RotationTo: 0, RotationIn: 0},
			"veins":       {RotationFrom: -360, RotationTo: 360, RotationIn: 0},
		},
	}

	if mc := rules.For("prontera.gat"); mc.Indoor || mc.Limits.YawLocked || mc.Limits.Arc {
		t.Fatalf("prontera restricted: %+v", mc)
	}
	if mc := rules.For("prt_in.gat"); !mc.Indoor || !mc.Limits.ZoomLocked || mc.Limits.YawLocked {
		t.Fatalf("prt_in should hold the zoom and allow turning: %+v", mc)
	}
	if mc := rules.For("prt_in.gat"); mc.Limits.FixedDistance != DefaultCameraZoom {
		t.Fatalf("indoor fixed distance %v, want the default %v", mc.Limits.FixedDistance, DefaultCameraZoom)
	}
	if mc := rules.For("PRT_IN"); !mc.Indoor {
		t.Fatal("the lookup must not care about case or extension")
	}

	mc := rules.For("mosk_fild01")
	if !mc.Limits.Arc || mc.Limits.YawLocked {
		t.Fatalf("an arc preset should bound the yaw: %+v", mc)
	}
	if got := mc.Limits.YawMax; math.Abs(float64(got)-120*math.Pi/180) > 1e-5 {
		t.Fatalf("arc max %v, want 120° in radians", got)
	}
	if mc := rules.For("jupe_gate"); !mc.Limits.YawLocked {
		t.Fatalf("a preset with from == to is a fixed camera: %+v", mc)
	}
	if mc := rules.For("veins"); mc.Limits.YawLocked || mc.Limits.Arc {
		t.Fatalf("±360 is unrestricted: %+v", mc)
	}

	var none *MapRules
	if mc := none.For("prt_in"); mc.Indoor {
		t.Fatal("no tables, no rules")
	}
}
