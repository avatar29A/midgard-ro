package skillstudio

import (
	"reflect"
	"testing"

	"github.com/Faultbox/midgard-ro/internal/game/states"
)

func TestTimelineCoversCastReleaseImpactAndCancellation(t *testing.T) {
	seq := &Sequence{CastMS: -1, Reaction: true}
	tape := makeTimeline(seq, 5, 97, 216)
	if tape.CastMS != 500 || tape.ReleaseTick != 30 || tape.DurationTicks != 157 {
		t.Fatal(tape)
	}
	if !reflect.DeepEqual(tape.ImpactTicks, []int{56, 70, 84, 98, 112}) {
		t.Fatal(tape.ImpactTicks)
	}
	sounds := 0
	for _, e := range tape.Events {
		if e.Sound != "" {
			sounds++
		}
	}
	if sounds != 6 {
		t.Fatal("cast + five impact sounds", sounds)
	}
	for tick, want := range map[int]string{0: "cast", 29: "cast", 30: "volley", 112: "tail", 127: "finished"} {
		if got := tape.phase(tick); got != want {
			t.Fatal(tick, got, want)
		}
	}
	cancel := 15
	seq.CancelTick = &cancel
	canceled := makeTimeline(seq, 5, 97, 216)
	if len(canceled.ImpactTicks) != 0 || len(canceled.ReactionTicks) != 0 || canceled.phase(15) != "canceled" {
		t.Fatal(canceled)
	}
	for _, e := range canceled.Events {
		if e.Kind == "cast.released" || e.Kind == "target.hurt" || e.Kind == "visual.impact" {
			t.Fatal("event escaped cancellation", e)
		}
	}
}
func TestLegacyAndInstantFixtures(t *testing.T) {
	legacy := makeTimeline(nil, 1, 41, 216)
	if legacy.ReleaseTick != 0 || legacy.DurationTicks != 41 || len(legacy.ReactionTicks) != 0 {
		t.Fatal(legacy)
	}
	instant := makeTimeline(&Sequence{CastMS: 0}, 1, 41, 216)
	for _, e := range instant.Events {
		if e.ID == "cast-audio" || e.Kind == "cast.started" {
			t.Fatal("instant cast has pre-cast", e)
		}
	}
}

func TestPersistentEventsEndAndCancelWithoutInventingHits(t *testing.T) {
	for _, id := range []uint16{10, 12, 18} {
		req := Request{SkillID: id, Level: 1, EffectDurationMS: 1000, Sequence: &Sequence{CastMS: 500, Reaction: true}}
		tape := persistentTimeline(req)
		if tape.ReleaseTick != 30 || tape.EndTick != 90 || tape.DurationTicks != 120 {
			t.Fatal(id, tape)
		}
		if tape.phase(30) != "active" || tape.phase(90) != "finished" {
			t.Fatal(id, tape)
		}
		if len(tape.ImpactTicks) > 0 || len(tape.ReactionTicks) > 0 {
			t.Fatal("invented server hits", id)
		}
		created, removed := 0, 0
		for _, e := range tape.Events {
			if e.Kind == "unit.created" || e.Kind == "status.applied" {
				created++
			}
			if e.Kind == "unit.removed" || e.Kind == "status.removed" {
				removed++
			}
		}
		want := 1
		if id == 18 {
			want = 3
		}
		if created != want || removed != want {
			t.Fatal(id, created, removed)
		}
		cancel := 15
		req.Sequence.CancelTick = &cancel
		tape = persistentTimeline(req)
		for _, e := range tape.Events {
			if e.Kind == "cast.released" || e.Kind == "unit.created" || e.Kind == "status.applied" {
				t.Fatal("created after cancellation", id, e)
			}
		}
	}
}

func TestMageStatusAndCancellationAreExplicitFixtures(t *testing.T) {
	p := &states.MagePreview{DurationMS: 900, Cues: []states.MageCue{{ID: "fx", Kind: "effect.started", Owner: "target"}, {ID: "audio", Kind: "sound", Sound: "test.wav", Owner: "target", AtMS: 200}}}
	r := Request{SkillID: 15, Level: 1, TargetStatus: "frozen", StatusDelayMS: 500, EffectDurationMS: 3000, Sequence: &Sequence{CastMS: 500, Reaction: true}}
	tape := mageTimeline(r, p, 216)
	if tape.EndTick != 240 || len(tape.ReactionTicks) != 1 {
		t.Fatal(tape)
	}
	found := false
	for _, e := range tape.Events {
		if e.Kind == "status.applied" {
			found = true
			if e.Tick != 60 {
				t.Fatal(e)
			}
		}
	}
	if !found {
		t.Fatal("missing explicit state input")
	}
	cancel := 15
	r.Sequence.CancelTick = &cancel
	tape = mageTimeline(r, p, 216)
	for _, e := range tape.Events {
		if e.Tick > cancel || e.Kind == "status.applied" || e.Kind == "target.hurt" || e.Kind == "effect.started" {
			t.Fatal("event escaped canceled cast", e)
		}
	}
	r.Sequence.CancelTick = nil
	r.TargetStatus = "none"
	r.SkillID = 21
	tape = mageTimeline(r, p, 216)
	if len(tape.ReactionTicks) != 0 || len(tape.ImpactTicks) != 0 {
		t.Fatal("invented Thunder Storm server hits", tape)
	}
}
