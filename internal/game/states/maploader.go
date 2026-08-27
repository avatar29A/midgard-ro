package states

import (
	"fmt"
	"math/rand/v2"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/Faultbox/midgard-ro/internal/logger"
	"github.com/Faultbox/midgard-ro/pkg/formats"
)

// MapSink is where a loaded map goes — the scene — in the phases the scene
// exposes. It is an interface so the loader's phase machine can be exercised
// without a GL context.
type MapSink interface {
	BeginMap(gnd *formats.GND, rsw *formats.RSW, load func(string) ([]byte, error))
	LoadTerrain(gnd *formats.GND, load func(string) ([]byte, error)) error
	ModelCount(rsw *formats.RSW) int
	BeginModels(rsw *formats.RSW)
	LoadModelRange(rsw *formats.RSW, load func(string) ([]byte, error), from, to int)
	EndMap(rsw *formats.RSW, load func(string) ([]byte, error))
}

// LoadPhase is one step of loading a map, in the order they run.
type LoadPhase int

// The phases. GAT and RSW failures are not fatal — a map without walkability
// or objects is still a map — but a GND failure is: there is nothing to stand
// on.
const (
	PhaseGAT LoadPhase = iota
	PhaseGND
	PhaseRSW
	PhasePrepare
	PhaseTerrain
	PhaseModels
	PhaseFinish
	PhaseDone
)

var phaseNames = [...]string{"gat", "gnd", "rsw", "prepare", "terrain", "models", "finish", "done"}

func (p LoadPhase) String() string {
	if p < 0 || int(p) >= len(phaseNames) {
		return fmt.Sprintf("phase(%d)", int(p))
	}
	return phaseNames[p]
}

// phaseWeight is each phase's share of the progress bar, from the measured
// split of a Prontera load: the models are most of it.
var phaseWeight = [...]float32{
	PhaseGAT:     0.02,
	PhaseGND:     0.05,
	PhaseRSW:     0.02,
	PhasePrepare: 0.03,
	PhaseTerrain: 0.23,
	PhaseModels:  0.62,
	PhaseFinish:  0.03,
}

// PhaseTiming is how long one phase took, and how many items it covered.
type PhaseTiming struct {
	Phase LoadPhase
	Ms    float64
	Count int
}

// LoadingImages is how many loading screens the archive ships
// (loading01.jpg … loading10.jpg). The original picks one at random per load.
const LoadingImages = 10

// StepBudget is how much of a frame one Step may spend before handing back
// so the loading screen can draw. The big phases overrun it — a terrain
// upload is one call — but the models come in as many as fit.
const StepBudget = 24 * time.Millisecond

// modelChunk is how many models are built between budget checks.
const modelChunk = 16

// MapLoader loads one map into a MapSink over several calls to Step, one
// phase or a chunk of models at a time, so the caller can draw between them.
//
// It runs on the caller's thread: the sink is the scene, and the scene wants
// the GL thread. The phases are what make the loading screen honest — the bar
// moves because work was done, not because time passed.
type MapLoader struct {
	// Name is the map without its extension.
	Name string

	// ImageIndex is which loading screen to show for this load, 1-based.
	ImageIndex int

	load func(string) ([]byte, error)
	sink MapSink

	// Parsers, replaceable so the machine can be tested on fake data.
	parseGAT func([]byte) (*formats.GAT, error)
	parseGND func([]byte) (*formats.GND, error)
	parseRSW func([]byte) (*formats.RSW, error)

	gat *formats.GAT
	gnd *formats.GND
	rsw *formats.RSW

	phase       LoadPhase
	modelsBegun bool
	modelsDone  int
	modelCount  int
	err         error

	budget     time.Duration
	started    time.Time
	phaseStart time.Time
	timings    []PhaseTiming
}

// NewMapLoader prepares to load a map. Nothing is read until the first Step.
func NewMapLoader(name string, load func(string) ([]byte, error), sink MapSink) *MapLoader {
	return &MapLoader{
		Name:       strings.TrimSuffix(strings.ToLower(name), ".gat"),
		ImageIndex: rand.IntN(LoadingImages) + 1,
		load:       load,
		sink:       sink,
		parseGAT:   formats.ParseGAT,
		parseGND:   formats.ParseGND,
		parseRSW:   formats.ParseRSW,
		budget:     StepBudget,
	}
}

// Step does up to a budget's worth of loading. It returns true once there is
// nothing left to do — successfully, or not: check Err.
func (l *MapLoader) Step() bool {
	if l.phase == PhaseDone {
		return true
	}
	if l.started.IsZero() {
		l.started = time.Now()
		l.phaseStart = l.started
	}
	deadline := time.Now().Add(l.budget)

	for l.phase != PhaseDone {
		complete, err := l.runPhase()
		if err != nil {
			l.err = fmt.Errorf("%s: %w", l.phase, err)
			l.finishPhase()
			l.phase = PhaseDone
			return true
		}
		if !complete {
			if time.Now().After(deadline) {
				return false
			}
			continue
		}

		l.finishPhase()
		l.phase++
		if l.phase != PhaseDone && time.Now().After(deadline) {
			return false
		}
	}
	return true
}

// runPhase does one unit of the current phase and reports whether the phase
// is complete.
func (l *MapLoader) runPhase() (bool, error) {
	switch l.phase {
	case PhaseGAT:
		path := "data\\" + l.Name + ".gat"
		data, err := l.load(path)
		if err == nil {
			l.gat, err = l.parseGAT(data)
		}
		if err != nil {
			logger.Warn("map has no walkability data", zap.String("path", path), zap.Error(err))
			l.gat = nil
		}
		return true, nil

	case PhaseGND:
		path := "data\\" + l.Name + ".gnd"
		data, err := l.load(path)
		if err != nil {
			return true, fmt.Errorf("loading %s: %w", path, err)
		}
		l.gnd, err = l.parseGND(data)
		if err != nil {
			return true, fmt.Errorf("parsing %s: %w", path, err)
		}
		return true, nil

	case PhaseRSW:
		path := "data\\" + l.Name + ".rsw"
		data, err := l.load(path)
		if err == nil {
			l.rsw, err = l.parseRSW(data)
		}
		if err != nil {
			logger.Warn("map has no object data", zap.String("path", path), zap.Error(err))
			l.rsw = nil
		}
		return true, nil

	case PhasePrepare:
		l.sink.BeginMap(l.gnd, l.rsw, l.load)
		return true, nil

	case PhaseTerrain:
		return true, l.sink.LoadTerrain(l.gnd, l.load)

	case PhaseModels:
		// No RSW, no objects: the sink is not asked.
		if l.rsw == nil {
			return true, nil
		}
		if !l.modelsBegun {
			l.modelsBegun = true
			l.modelCount = l.sink.ModelCount(l.rsw)
			l.sink.BeginModels(l.rsw)
		}
		if l.modelsDone >= l.modelCount {
			return true, nil
		}
		to := l.modelsDone + modelChunk
		if to > l.modelCount {
			to = l.modelCount
		}
		l.sink.LoadModelRange(l.rsw, l.load, l.modelsDone, to)
		l.modelsDone = to
		return l.modelsDone >= l.modelCount, nil

	case PhaseFinish:
		l.sink.EndMap(l.rsw, l.load)
		return true, nil
	}
	return true, nil
}

// finishPhase records how long the phase took and starts the clock on the
// next one.
func (l *MapLoader) finishPhase() {
	now := time.Now()
	t := PhaseTiming{Phase: l.phase, Ms: float64(now.Sub(l.phaseStart).Microseconds()) / 1000}
	if l.phase == PhaseModels {
		t.Count = l.modelsDone
	}
	l.timings = append(l.timings, t)
	l.phaseStart = now
}

// Done reports whether Step has nothing left to do.
func (l *MapLoader) Done() bool {
	return l.phase == PhaseDone
}

// Err is what stopped the load, or nil.
func (l *MapLoader) Err() error {
	return l.err
}

// Phase is the phase in progress.
func (l *MapLoader) Phase() LoadPhase {
	return l.phase
}

// Progress is how far along the load is, 0 to 1, weighted by how long each
// phase takes rather than by counting phases. Within the models it is linear
// in models built, which is what makes the bar move smoothly through the
// longest part.
func (l *MapLoader) Progress() float32 {
	if l.phase == PhaseDone {
		return 1
	}
	var done float32
	for p := PhaseGAT; p < l.phase && p < PhaseDone; p++ {
		done += phaseWeight[p]
	}
	if l.phase == PhaseModels && l.modelCount > 0 {
		done += phaseWeight[PhaseModels] * float32(l.modelsDone) / float32(l.modelCount)
	}
	if done > 1 {
		done = 1
	}
	return done
}

// GAT is the walkability grid, once PhaseGAT has run; nil if the map has none.
func (l *MapLoader) GAT() *formats.GAT {
	return l.gat
}

// ModelCount is how many models the map places, once known.
func (l *MapLoader) ModelCount() int {
	return l.modelCount
}

// Timings is how long each finished phase took.
func (l *MapLoader) Timings() []PhaseTiming {
	return l.timings
}

// TotalMs is how long the load has taken so far, or took.
func (l *MapLoader) TotalMs() float64 {
	if l.started.IsZero() {
		return 0
	}
	end := time.Now()
	if l.phase == PhaseDone {
		end = l.phaseStart
	}
	return float64(end.Sub(l.started).Microseconds()) / 1000
}

// TimingSummary is the phase breakdown as one line, for the log and the F3
// overlay: "gat 3 · gnd 210 · rsw 12 · prepare 4 · terrain 380 · models 620 (1304) · finish 18".
func (l *MapLoader) TimingSummary() string {
	parts := make([]string, 0, len(l.timings))
	for _, t := range l.timings {
		if t.Count > 0 {
			parts = append(parts, fmt.Sprintf("%s %.0f (%d)", t.Phase, t.Ms, t.Count))
		} else {
			parts = append(parts, fmt.Sprintf("%s %.0f", t.Phase, t.Ms))
		}
	}
	return strings.Join(parts, " · ")
}
