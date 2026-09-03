package states

import (
	"math"
	"strings"

	"go.uber.org/zap"

	"github.com/Faultbox/midgard-ro/internal/trace"
)

// The noises a map makes on its own.
//
// A world file lists them beside its models and its lights: a file to play, a
// place to play it, how far it carries and how long to wait before playing it
// again. Prontera has its fountain, the smithies have their hammers, the
// fields have their birds. The parser has read them since it was written and
// nothing had ever played one, so every map was silent but for what moved on
// it.
//
// Each source keeps its own clock rather than sharing one, because they do
// not share a cycle: a fountain repeats every few seconds and a bell every
// half a minute, and one clock for all of them would fire everything at once.

// ambientSourceDir is where a world file's sounds live. The file names it
// carries have no directory of their own.
const ambientSourceDir = `data\wav\`

// ambient is one of the map's sound sources, counting down to its next turn.
type ambient struct {
	path string

	x, y, z float32

	// carry is how far it is heard, and volume how loud it is at the middle
	// of that. Both are the world file's own.
	carry  float32
	volume float32

	// everyMs is how long between plays, and leftMs how much of that is left.
	everyMs, leftMs float32
}

// ambientLeastCycle is the shortest gap between plays of one source.
//
// A world file may say zero, which means the sound runs continuously — a
// river, a fire. There is no looping here, so it is replayed instead, and
// something has to stop that being once a frame. Four seconds is longer than
// the sounds these are and short enough that a river does not fall silent.
const ambientLeastCycle = float32(4000)

// loadAmbientSounds takes the map's sources off the scene, once it has one.
func (s *InGameState) loadAmbientSounds() {
	s.ambient = nil

	if s.scene == nil {
		return
	}

	for _, source := range s.scene.Sounds {
		if source == nil || source.File == "" {
			continue
		}

		every := source.Cycle * 1000
		if every < ambientLeastCycle {
			every = ambientLeastCycle
		}

		s.ambient = append(s.ambient, &ambient{
			path: ambientSourceDir + strings.ReplaceAll(source.File, "/", `\`),

			// The world file's own axes, which are the ones the map is built
			// on: what the scene calls Y is up in both.
			x: source.Position[0], y: source.Position[1], z: source.Position[2],

			carry:  source.Range,
			volume: source.Volume,

			everyMs: every,

			// Spread out rather than all at once: every source starting
			// together on the first frame of a map is a bang.
			leftMs: every * hash01(uint32(len(s.ambient)), 71),
		})
	}

	trace.Emit(trace.HUD, "ambient-sounds", zap.Int("count", len(s.ambient)))
}

// advanceAmbientSounds plays the ones whose turn has come and that can be
// heard from where the character is standing.
func (s *InGameState) advanceAmbientSounds(deltaMs float32) {
	for _, source := range s.ambient {
		source.leftMs -= deltaMs
		if source.leftMs > 0 {
			continue
		}

		source.leftMs = source.everyMs

		if gain := s.ambientGain(source); gain > 0 {
			s.playSoundGain(source.path, gain)
		}
	}
}

// ambientGain is how loud a source is from where the character stands, and
// zero when it is out of earshot.
//
// Its own range rather than the one a blow is heard at: a world file says how
// far each of its sounds carries, and a fountain heard across the whole of
// Prontera would be as wrong as one that stops at the next tile.
func (s *InGameState) ambientGain(source *ambient) float32 {
	if s.player == nil || source.carry <= 0 {
		return 0
	}

	px, _, pz := s.player.RenderPosition()
	dx, dz := source.x-px, source.z-pz

	away := float32(math.Sqrt(float64(dx*dx + dz*dz)))
	if away >= source.carry {
		return 0
	}

	volume := source.volume
	if volume <= 0 || volume > 1 {
		volume = 1
	}

	return volume * (1 - away/source.carry)
}
