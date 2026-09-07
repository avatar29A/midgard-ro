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
	for _, bad := range []Request{{Level: 11}, {Level: 10, Tick: 601}, {Level: 10, Separation: 12, Camera: Camera{Distance: 90, Focus: "center"}}} {
		if err := bad.Validate(); err == nil {
			t.Fatal("accepted out-of-range scene")
		}
	}
}
