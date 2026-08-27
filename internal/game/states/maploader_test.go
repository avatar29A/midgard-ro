package states

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/Faultbox/midgard-ro/pkg/formats"
)

// fakeSink records the calls a loader makes, in order, and can fail the
// terrain phase on request.
type fakeSink struct {
	calls       []string
	models      int
	failTerrain error
}

func (f *fakeSink) BeginMap(*formats.GND, *formats.RSW, func(string) ([]byte, error)) {
	f.calls = append(f.calls, "begin")
}

func (f *fakeSink) LoadTerrain(*formats.GND, func(string) ([]byte, error)) error {
	f.calls = append(f.calls, "terrain")
	return f.failTerrain
}

func (f *fakeSink) ModelCount(*formats.RSW) int { return f.models }

func (f *fakeSink) BeginModels(*formats.RSW) { f.calls = append(f.calls, "models-begin") }

func (f *fakeSink) LoadModelRange(_ *formats.RSW, _ func(string) ([]byte, error), from, to int) {
	f.calls = append(f.calls, fmt.Sprintf("models[%d,%d)", from, to))
}

func (f *fakeSink) EndMap(*formats.RSW, func(string) ([]byte, error)) {
	f.calls = append(f.calls, "end")
}

// newTestLoader builds a loader whose parsers accept anything and whose
// archive holds the three map files, unless missing lists them.
func newTestLoader(t *testing.T, sink *fakeSink, missing ...string) *MapLoader {
	t.Helper()
	load := func(path string) ([]byte, error) {
		for _, m := range missing {
			if strings.HasSuffix(path, m) {
				return nil, errors.New("not in archive")
			}
		}
		return []byte(path), nil
	}
	l := NewMapLoader("prontera.gat", load, sink)
	l.parseGAT = func([]byte) (*formats.GAT, error) { return &formats.GAT{}, nil }
	l.parseGND = func([]byte) (*formats.GND, error) { return &formats.GND{}, nil }
	l.parseRSW = func([]byte) (*formats.RSW, error) { return &formats.RSW{}, nil }
	return l
}

// stepAll runs Step until done, guarding against a machine that never ends.
func stepAll(t *testing.T, l *MapLoader) {
	t.Helper()
	for i := 0; i < 10000; i++ {
		if l.Step() {
			return
		}
	}
	t.Fatal("loader never finished")
}

func TestLoaderRunsThePhasesInOrderAndChunksTheModels(t *testing.T) {
	sink := &fakeSink{models: 40}
	l := newTestLoader(t, sink)
	l.budget = 0 // one unit of work per Step, so every chunk boundary shows

	stepAll(t, l)

	want := []string{"begin", "terrain", "models-begin", "models[0,16)", "models[16,32)", "models[32,40)", "end"}
	if strings.Join(sink.calls, " ") != strings.Join(want, " ") {
		t.Fatalf("calls:\n got %v\nwant %v", sink.calls, want)
	}
	if l.Err() != nil {
		t.Fatalf("unexpected error: %v", l.Err())
	}
	if l.Name != "prontera" {
		t.Fatalf("Name = %q, want the extension stripped", l.Name)
	}
	if l.ImageIndex < 1 || l.ImageIndex > LoadingImages {
		t.Fatalf("ImageIndex = %d, want 1..%d", l.ImageIndex, LoadingImages)
	}
	if l.GAT() == nil {
		t.Fatal("GAT was parsed but not kept")
	}
}

func TestLoaderProgressNeverGoesBackwardsAndEndsAtOne(t *testing.T) {
	sink := &fakeSink{models: 100}
	l := newTestLoader(t, sink)
	l.budget = 0

	last := float32(-1)
	for !l.Step() {
		p := l.Progress()
		if p < last {
			t.Fatalf("progress went backwards: %v after %v (phase %s)", p, last, l.Phase())
		}
		if p > 1 {
			t.Fatalf("progress above one: %v", p)
		}
		last = p
	}
	if l.Progress() != 1 {
		t.Fatalf("finished at %v, want 1", l.Progress())
	}
	if l.Phase() != PhaseDone || !l.Done() {
		t.Fatalf("phase %s after finishing", l.Phase())
	}
}

func TestLoaderMissingGATAndRSWAreNotFatal(t *testing.T) {
	sink := &fakeSink{models: 5}
	l := newTestLoader(t, sink, ".gat", ".rsw")

	stepAll(t, l)

	if l.Err() != nil {
		t.Fatalf("a map without GAT or RSW must still load, got %v", l.Err())
	}
	if l.GAT() != nil {
		t.Fatal("GAT reported for a map that has none")
	}
	// With no RSW there are no models, and the sink must not be asked for any.
	for _, c := range sink.calls {
		if strings.HasPrefix(c, "models[") {
			t.Fatalf("models were loaded without an RSW: %v", sink.calls)
		}
	}
}

func TestLoaderMissingGNDIsFatalAndNamesThePhase(t *testing.T) {
	sink := &fakeSink{models: 5}
	l := newTestLoader(t, sink, ".gnd")

	stepAll(t, l)

	if l.Err() == nil {
		t.Fatal("a map without ground loaded")
	}
	if !strings.HasPrefix(l.Err().Error(), "gnd:") {
		t.Fatalf("error does not name the phase: %v", l.Err())
	}
	if len(sink.calls) != 0 {
		t.Fatalf("the sink was touched after a fatal parse: %v", sink.calls)
	}
}

func TestLoaderTerrainFailureStopsTheLoad(t *testing.T) {
	sink := &fakeSink{models: 5, failTerrain: errors.New("no GL")}
	l := newTestLoader(t, sink)

	stepAll(t, l)

	if l.Err() == nil || !strings.Contains(l.Err().Error(), "no GL") {
		t.Fatalf("terrain error not surfaced: %v", l.Err())
	}
	for _, c := range sink.calls {
		if c == "end" || strings.HasPrefix(c, "models") {
			t.Fatalf("loading went on after the terrain failed: %v", sink.calls)
		}
	}
}

func TestLoaderTimingsCoverEveryPhaseOnce(t *testing.T) {
	sink := &fakeSink{models: 3}
	l := newTestLoader(t, sink)

	stepAll(t, l)

	seen := map[LoadPhase]int{}
	for _, tm := range l.Timings() {
		seen[tm.Phase]++
		if tm.Phase == PhaseModels && tm.Count != 3 {
			t.Fatalf("models timing counted %d, want 3", tm.Count)
		}
	}
	for p := PhaseGAT; p < PhaseDone; p++ {
		if seen[p] != 1 {
			t.Fatalf("phase %s timed %d times", p, seen[p])
		}
	}
	if s := l.TimingSummary(); !strings.Contains(s, "models") || !strings.Contains(s, "(3)") {
		t.Fatalf("summary %q lacks the model count", s)
	}
}
