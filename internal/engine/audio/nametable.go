package audio

import (
	"fmt"
	"strings"
)

// NameTablePath is the GRF entry mapping map resources to their background
// music track.
const NameTablePath = "data/mp3nametable.txt"

// mapFileExtensions are the files a map is made of. A location is often at hand
// as one of them rather than as a bare name.
var mapFileExtensions = []string{".rsw", ".gat", ".gnd"}

// NameTable maps a map resource name ("prontera.rsw") to the background music
// track that plays on it ("bgm/08.mp3").
type NameTable map[string]string

// AssetLoader reads a file out of the game's archives. It is satisfied by
// assets.Manager.Load.
type AssetLoader interface {
	Load(path string) ([]byte, error)
}

// LoadNameTable reads data/mp3nametable.txt out of the game archives.
//
// Entries look like `prontera.rsw#bgm\\08.mp3#`. Lines starting with `//` are
// comments, written in CP949, and are skipped without being decoded.
func LoadNameTable(loader AssetLoader) (NameTable, error) {
	data, err := loader.Load(NameTablePath)
	if err != nil {
		return nil, fmt.Errorf("loading %s: %w", NameTablePath, err)
	}

	return ParseNameTable(data), nil
}

// ParseNameTable reads the entries out of an mp3 name table.
func ParseNameTable(data []byte) NameTable {
	table := NameTable{}

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}

		// An entry is `<resource>#<track>#`, so a well formed line splits into
		// at least three fields.
		fields := strings.Split(line, "#")
		if len(fields) < 3 {
			continue
		}

		resource := strings.ToLower(strings.TrimSpace(fields[0]))
		track := NormalizeTrack(fields[1])
		if resource == "" || track == "" {
			continue
		}

		table[resource] = track
	}

	return table
}

// Lookup returns the background music track of a location, accepting anything
// NormalizeLocation does.
func (t NameTable) Lookup(location string) (string, bool) {
	track, found := t[NormalizeLocation(location)+".rsw"]

	return track, found
}

// NormalizeLocation reduces anything that names a map to the bare location id
// the name table is keyed by. It accepts a bare name ("prontera"), a resource
// name ("prontera.rsw"), an archive path (`data\prontera.gat`) and any casing.
func NormalizeLocation(location string) string {
	location = strings.ToLower(strings.TrimSpace(location))
	location = strings.ReplaceAll(location, `\`, "/")

	if i := strings.LastIndex(location, "/"); i >= 0 {
		location = location[i+1:]
	}

	for _, extension := range mapFileExtensions {
		if strings.HasSuffix(location, extension) {
			return strings.TrimSuffix(location, extension)
		}
	}

	return location
}

// NormalizeTrack turns a track as written in the name table into a slash
// separated relative path. Tracks are Windows paths whose separator is escaped
// (`bgm\\08.mp3`), so both the escaping and the separator have to go.
func NormalizeTrack(track string) string {
	path := strings.ReplaceAll(strings.TrimSpace(track), `\`, "/")

	for strings.Contains(path, "//") {
		path = strings.ReplaceAll(path, "//", "/")
	}

	return strings.Trim(path, "/")
}
