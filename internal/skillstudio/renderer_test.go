package skillstudio

import "testing"

func TestMissingProjectReturnsErrorBeforeGL(t *testing.T) {
	r, err := New(t.TempDir())
	if err == nil || r != nil {
		t.Fatal("missing config must fail before initializing renderers")
	}
}
func TestSceneRejectsUnsupportedBounds(t *testing.T) {
	r := Request{Level: 10, Separation: 12, Camera: Camera{Distance: 190, Focus: "center"}}
	if err := r.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []Request{{Level: 11}, {Level: 10, Tick: 3601}, {Level: 10, Separation: 12, Camera: Camera{Distance: 90, Focus: "center"}}} {
		if err := bad.Validate(); err == nil {
			t.Fatal("accepted out-of-range scene")
		}
	}
}

func TestCancelMustPrecedeRelease(t *testing.T) {
	at := 30
	req := Request{Level: 10, Separation: 12, Camera: Camera{Distance: 190, Focus: "center"}, Sequence: &Sequence{CastMS: -1, CancelTick: &at}}
	if err := req.Validate(); err == nil {
		t.Fatal("accepted cancellation at release")
	}
	at = 15
	if err := req.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestMageStatusInputsAreBoundToTheirSkill(t *testing.T) {
	r := Request{SkillID: 15, Level: 1, Separation: 12, Camera: Camera{Distance: 190, Focus: "center"}, TargetStatus: "frozen", StatusDelayMS: 10000, EffectDurationMS: 30000, Tick: 3000}
	if err := r.Validate(); err != nil {
		t.Fatal(err)
	}
	r.SkillID = 19
	if r.Validate() == nil {
		t.Fatal("Fire Bolt accepted frozen body fixture")
	}
	r.SkillID = 16
	r.TargetStatus = "stone"
	if err := r.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestRawSTRRejectsPathAndSkillSequence(t *testing.T) {
	r := Request{Level: 1, Separation: 12, Camera: Camera{Distance: 190, Focus: "center"}, STRResourceID: "../../somewhere"}
	if r.Validate() == nil {
		t.Fatal("accepted a path in place of browser ID")
	}
	r.STRResourceID = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	if err := r.Validate(); err != nil {
		t.Fatal(err)
	}
	r.Sequence = &Sequence{CastMS: 500}
	if r.Validate() == nil {
		t.Fatal("raw animation accepted skill cast fixture")
	}
}
