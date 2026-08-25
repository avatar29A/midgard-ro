package audio

import (
	"os"
	"path/filepath"
	"testing"
)

// fakeBGMPlayer records what a LocationPlayer asks for, so the tests need no
// audio device.
type fakeBGMPlayer struct {
	played []string
	stops  int
	err    error
}

func (f *fakeBGMPlayer) PlayBGMFile(path string, loop bool) error {
	if f.err != nil {
		return f.err
	}

	if !loop {
		return os.ErrInvalid // background music always repeats
	}

	f.played = append(f.played, filepath.Base(path))

	return nil
}

func (f *fakeBGMPlayer) StopBGM() {
	f.stops++
}

// newTestPlayer returns a player backed by a BGM directory holding the tracks
// the test name table refers to.
func newTestPlayer(t *testing.T, tracks ...string) (*LocationPlayer, *fakeBGMPlayer) {
	t.Helper()

	dir := t.TempDir()

	for _, track := range tracks {
		if err := os.WriteFile(filepath.Join(dir, track), []byte("audio"), 0o600); err != nil {
			t.Fatalf("writing %s: %v", track, err)
		}
	}

	fake := &fakeBGMPlayer{}

	return NewLocationPlayer(fake, ParseNameTable(testNameTable), dir), fake
}

func TestPlayLocation(t *testing.T) {
	player, fake := newTestPlayer(t, "08.mp3", "12.mp3", "01.mp3")

	tests := []struct {
		location string
		want     string
		fallback bool
	}{
		{"prontera", "08.mp3", false},
		{"prt_fild08", "12.mp3", false},
		{"data/prontera.gat", "08.mp3", false}, // an archive path resolves
		{"PRONTERA", "08.mp3", false},          // and so does any casing
		{"prontera_fld", "01.mp3", true},       // not a real map
		{"", "01.mp3", true},
	}

	for _, tt := range tests {
		if err := player.PlayLocation(tt.location); err != nil {
			t.Errorf("PlayLocation(%q): %v", tt.location, err)

			continue
		}

		if got := filepath.Base(player.Track()); got != tt.want {
			t.Errorf("PlayLocation(%q) plays %q, want %q", tt.location, player.Track(), tt.want)
		}

		if player.UsingFallback() != tt.fallback {
			t.Errorf("PlayLocation(%q) fallback = %v, want %v",
				tt.location, player.UsingFallback(), tt.fallback)
		}
	}

	if len(fake.played) == 0 {
		t.Fatal("nothing was played")
	}
}

// Neighboring maps share a track, and walking between them must not restart
// the music.
func TestPlayLocationSharedTrackDoesNotRestart(t *testing.T) {
	player, fake := newTestPlayer(t, "08.mp3", "12.mp3", "01.mp3")

	for _, location := range []string{"prontera", "prontera.rsw", "prontera.gat"} {
		if err := player.PlayLocation(location); err != nil {
			t.Fatalf("PlayLocation(%q): %v", location, err)
		}
	}

	if len(fake.played) != 1 {
		t.Errorf("started playback %d times, want 1: %v", len(fake.played), fake.played)
	}

	// A different track does start.
	if err := player.PlayLocation("prt_fild08"); err != nil {
		t.Fatalf("PlayLocation(prt_fild08): %v", err)
	}

	if len(fake.played) != 2 {
		t.Errorf("started playback %d times, want 2: %v", len(fake.played), fake.played)
	}
}

func TestPlayFallback(t *testing.T) {
	player, fake := newTestPlayer(t, "01.mp3")

	if err := player.PlayFallback(); err != nil {
		t.Fatalf("PlayFallback: %v", err)
	}

	if !player.UsingFallback() {
		t.Error("UsingFallback = false, want true")
	}

	if player.Location() != "" {
		t.Errorf("Location = %q, want empty on a non game screen", player.Location())
	}

	if len(fake.played) != 1 || fake.played[0] != "01.mp3" {
		t.Errorf("played %v, want [01.mp3]", fake.played)
	}
}

func TestSetFallbackTrackEmptyIsSilent(t *testing.T) {
	player, fake := newTestPlayer(t, "01.mp3")

	player.SetFallbackTrack("")

	if err := player.PlayLocation("prontera_fld"); err != nil {
		t.Fatalf("PlayLocation: %v", err)
	}

	if len(fake.played) != 0 {
		t.Errorf("played %v, want nothing", fake.played)
	}

	if fake.stops != 1 {
		t.Errorf("stops = %d, want 1", fake.stops)
	}
}

// A track the name table names but the BGM folder does not hold is an error,
// not a panic or silence.
func TestPlayLocationMissingFile(t *testing.T) {
	player, _ := newTestPlayer(t) // no tracks on disk at all

	if err := player.PlayLocation("prontera"); err == nil {
		t.Error("PlayLocation succeeded, want an error for a missing file")
	}

	if player.Track() != "" {
		t.Errorf("Track = %q, want empty after a failure", player.Track())
	}
}

func TestTrackFor(t *testing.T) {
	player, _ := newTestPlayer(t)

	tests := []struct {
		location string
		want     string
		known    bool
	}{
		{"prontera", "bgm/08.mp3", true},
		{"prt_fild08", "bgm/12.mp3", true},
		{"prontera_fld", "", false},
	}

	for _, tt := range tests {
		track, known := player.TrackFor(tt.location)
		if track != tt.want || known != tt.known {
			t.Errorf("TrackFor(%q) = (%q, %v), want (%q, %v)",
				tt.location, track, known, tt.want, tt.known)
		}
	}
}

func TestStop(t *testing.T) {
	player, fake := newTestPlayer(t, "08.mp3")

	if err := player.PlayLocation("prontera"); err != nil {
		t.Fatalf("PlayLocation: %v", err)
	}

	player.Stop()

	if fake.stops != 1 {
		t.Errorf("stops = %d, want 1", fake.stops)
	}

	if player.Track() != "" || player.Location() != "" || player.UsingFallback() {
		t.Errorf("state after Stop: track=%q location=%q fallback=%v",
			player.Track(), player.Location(), player.UsingFallback())
	}
}

func TestDefaultBGMDir(t *testing.T) {
	tests := []struct {
		grfPath string
		want    string
	}{
		{filepath.Join("data", "data.grf"), filepath.Join("data", "BGM")},
		{"", ""},
	}

	for _, tt := range tests {
		if got := DefaultBGMDir(tt.grfPath); got != tt.want {
			t.Errorf("DefaultBGMDir(%q) = %q, want %q", tt.grfPath, got, tt.want)
		}
	}
}
