package audio

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DefaultFallbackTrack is the title theme, which is what the game plays when it
// is not on a map: the login, character select and loading screens, and any
// location the name table does not know.
const DefaultFallbackTrack = "bgm/01.mp3"

// bgmDirName is the leading path segment of every track in the name table
// ("bgm/08.mp3"). The BGM directory already points at that folder, so the
// segment is stripped before resolving a track on disk.
const bgmDirName = "bgm"

// BGMPlayer is the part of Manager that a LocationPlayer drives.
type BGMPlayer interface {
	// PlayBGMFile plays a music file from disk, looping it when loop is set.
	PlayBGMFile(path string, loop bool) error

	// StopBGM stops whatever is playing.
	StopBGM()
}

// LocationPlayer turns a location into background music.
//
// Callers hand it a location id — the map's resource name, as the server sends
// it ("prontera", "prt_fild08") — and it starts that location's track on
// repeat. Locations the name table does not know fall back to the title theme,
// so a wrong or made up id ("prontera_fld") is never an error and never
// silence.
//
// It is a thin wrapper over Manager and holds no audio state of its own, so a
// Manager may still be driven directly for anything the player does not cover.
type LocationPlayer struct {
	player BGMPlayer
	table  NameTable

	// bgmDir holds the .mp3 files. Ragnarok clients ship them in a BGM folder
	// next to the GRF archives; they are never inside the archives themselves.
	bgmDir string

	fallbackTrack string

	location      string
	track         string
	usingFallback bool
}

// NewLocationPlayer returns a player that reads its music from bgmDir and falls
// back to DefaultFallbackTrack.
func NewLocationPlayer(player BGMPlayer, table NameTable, bgmDir string) *LocationPlayer {
	return &LocationPlayer{
		player:        player,
		table:         table,
		bgmDir:        bgmDir,
		fallbackTrack: DefaultFallbackTrack,
	}
}

// DefaultBGMDir returns the BGM folder of the client a GRF archive belongs to,
// which is where Ragnarok clients keep their background music.
func DefaultBGMDir(grfPath string) string {
	if grfPath == "" {
		return ""
	}

	return filepath.Join(filepath.Dir(grfPath), "BGM")
}

// SetFallbackTrack replaces the track played for unknown locations and by
// PlayFallback. An empty track disables the fallback, which makes those cases
// silent instead.
func (p *LocationPlayer) SetFallbackTrack(track string) {
	p.fallbackTrack = NormalizeTrack(track)
}

// FallbackTrack returns the track played for unknown locations.
func (p *LocationPlayer) FallbackTrack() string {
	return p.fallbackTrack
}

// PlayLocation starts the background music of a location and repeats it until
// another location is played or the player is stopped. The id may be a bare map
// name ("prontera"), a resource name ("prontera.rsw") or an archive path
// ("data/prontera.gat"), in any casing.
//
// An unknown location is not an error: it plays the fallback track, which
// UsingFallback reports. An error means the audio itself could not be played,
// which for a well formed location id means the track is missing from the BGM
// directory.
//
// Locations that share a track — neighboring fields usually do — keep the
// music running instead of restarting it.
func (p *LocationPlayer) PlayLocation(location string) error {
	location = NormalizeLocation(location)

	track, found := p.table.Lookup(location)
	if !found {
		if err := p.PlayFallback(); err != nil {
			return err
		}

		p.location = location

		return nil
	}

	if err := p.play(track); err != nil {
		return err
	}

	p.location = location
	p.usingFallback = false

	return nil
}

// PlayFallback starts the fallback track on repeat. It is what the non game
// screens — login, character select, loading — play, and where an unknown
// location ends up. It is silent, and not an error, when the fallback track is
// disabled.
func (p *LocationPlayer) PlayFallback() error {
	p.location = ""
	p.usingFallback = true

	if p.fallbackTrack == "" {
		p.Stop()
		p.usingFallback = true

		return nil
	}

	if err := p.play(p.fallbackTrack); err != nil {
		return err
	}

	return nil
}

// Stop stops the background music.
func (p *LocationPlayer) Stop() {
	p.player.StopBGM()

	p.location = ""
	p.track = ""
	p.usingFallback = false
}

// Location returns the location whose music is playing. It is empty on the non
// game screens.
func (p *LocationPlayer) Location() string {
	return p.location
}

// Track returns the track that is playing, which is the fallback track when the
// location is unknown, and empty when nothing plays.
func (p *LocationPlayer) Track() string {
	return p.track
}

// UsingFallback reports whether what is playing is the fallback track rather
// than a location's own music.
func (p *LocationPlayer) UsingFallback() bool {
	return p.usingFallback
}

// TrackFor returns the track a location would play and whether the name table
// knows it. It resolves without touching playback, so callers can tell a real
// location from one that would fall back.
func (p *LocationPlayer) TrackFor(location string) (string, bool) {
	return p.table.Lookup(location)
}

// play starts a track unless it is already playing, so moving between maps that
// share a track does not restart the music.
func (p *LocationPlayer) play(track string) error {
	if p.track == track {
		return nil
	}

	path, err := p.resolve(track)
	if err != nil {
		return err
	}

	if err := p.player.PlayBGMFile(path, true); err != nil {
		return fmt.Errorf("playing background music %s: %w", track, err)
	}

	p.track = track

	return nil
}

// resolve turns a name table track ("bgm/08.mp3") into a path inside the BGM
// directory.
func (p *LocationPlayer) resolve(track string) (string, error) {
	rel := NormalizeTrack(track)

	// Every track is prefixed with the client's BGM folder, which is what
	// bgmDir already points at.
	if segments := strings.SplitN(rel, "/", 2); len(segments) == 2 && strings.EqualFold(segments[0], bgmDirName) {
		rel = segments[1]
	}

	path := filepath.Join(p.bgmDir, filepath.FromSlash(rel))
	if _, err := os.Stat(path); err == nil {
		return path, nil
	}

	// The name table is written for Windows, so its casing need not match a
	// case sensitive file system.
	if match, found := findCaseInsensitive(path); found {
		return match, nil
	}

	return "", fmt.Errorf("background music %s not found in %s", track, p.bgmDir)
}

// findCaseInsensitive looks for path in its parent directory, ignoring the
// casing of the file name.
func findCaseInsensitive(path string) (string, bool) {
	dir, name := filepath.Split(path)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", false
	}

	for _, entry := range entries {
		if strings.EqualFold(entry.Name(), name) {
			return filepath.Join(dir, entry.Name()), true
		}
	}

	return "", false
}
