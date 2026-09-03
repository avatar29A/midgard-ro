package states

import (
	"testing"

	"github.com/Faultbox/midgard-ro/internal/game/entity"
)

// atPlayer builds a state with the character standing at the origin.
func atPlayer() *InGameState {
	return &InGameState{player: entity.NewCharacter(0, 0, 0)}
}

// TestAnAmbientSourceFadesWithDistance: a world file says how far each of its
// sounds carries, and a fountain heard across the whole of Prontera would be
// as wrong as one that stops at the next tile.
func TestAnAmbientSourceFadesWithDistance(t *testing.T) {
	s := atPlayer()
	source := &ambient{carry: 100, volume: 1}

	if got := s.ambientGain(source); got != 1 {
		t.Errorf("standing on it the gain is %v, want 1", got)
	}

	source.x = 50
	if got := s.ambientGain(source); got < 0.45 || got > 0.55 {
		t.Errorf("halfway out the gain is %v, want about a half", got)
	}

	source.x = 100
	if got := s.ambientGain(source); got != 0 {
		t.Errorf("at the edge of its range the gain is %v, want silence", got)
	}

	source.x = 400
	if got := s.ambientGain(source); got != 0 {
		t.Errorf("far outside it the gain is %v", got)
	}
}

// TestAnAmbientSourceKeepsItsOwnVolume: the world file sets one per source,
// and a bell and a brook are not the same loudness.
func TestAnAmbientSourceKeepsItsOwnVolume(t *testing.T) {
	s := atPlayer()

	loud := s.ambientGain(&ambient{carry: 100, volume: 1})
	quiet := s.ambientGain(&ambient{carry: 100, volume: 0.25})

	if quiet >= loud {
		t.Errorf("a quiet source came out at %v against a loud one's %v", quiet, loud)
	}

	// A file that says nothing sensible is played at full rather than
	// silently: a source nobody can hear is the same as one that is missing.
	if got := s.ambientGain(&ambient{carry: 100, volume: 0}); got != 1 {
		t.Errorf("a source with no volume set came out at %v, want 1", got)
	}
}

// TestAmbientSourcesTakeTurns: each keeps its own clock, because they do not
// share a cycle — a fountain repeats every few seconds and a bell every half
// a minute, and one clock for all of them would fire everything at once.
func TestAmbientSourcesTakeTurns(t *testing.T) {
	s := atPlayer()
	s.ambient = []*ambient{
		{path: "brook.wav", carry: 100, volume: 1, everyMs: 4000, leftMs: 100},
		{path: "bell.wav", carry: 100, volume: 1, everyMs: 30000, leftMs: 20000},
	}

	s.advanceAmbientSounds(200)

	sounds := s.TakeSounds()
	if len(sounds) != 1 || sounds[0].Path != "brook.wav" {
		t.Fatalf("played %v, want just the brook", sounds)
	}

	// And the one that played waits its own cycle before the next.
	if got := s.ambient[0].leftMs; got != 4000 {
		t.Errorf("the brook waits %v before playing again, want its own 4000", got)
	}
	if got := s.ambient[1].leftMs; got != 19800 {
		t.Errorf("the bell's clock reads %v, want 19800", got)
	}
}

// TestAnAmbientSourceOutOfEarshotStaysQuiet: its clock still runs, so walking
// into range does not have to wait for a cycle that never started.
func TestAnAmbientSourceOutOfEarshotStaysQuiet(t *testing.T) {
	s := atPlayer()
	s.ambient = []*ambient{{path: "far.wav", x: 9999, carry: 100, volume: 1, everyMs: 1000}}

	s.advanceAmbientSounds(2000)

	if got := s.TakeSounds(); len(got) != 0 {
		t.Errorf("a source out of earshot played %v", got)
	}
	if got := s.ambient[0].leftMs; got != 1000 {
		t.Errorf("its clock reads %v; it should still be running", got)
	}
}
