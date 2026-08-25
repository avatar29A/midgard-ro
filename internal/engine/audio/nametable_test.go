package audio

import (
	"testing"
)

// The real table is CRLF terminated, has CP949 comments and keeps retired
// entries around as comments.
var testNameTable = []byte("" +
	"// ==========================================================================\r\n" +
	"// mp3NameTable.txt \xba\xbb\xbc\xad\xb9\xf6\r\n" +
	"//\xb8\xca\xc0̸\xa7.rsw#bgm\\\xc0\xbd\xbeǹ\xf8ȣ#\r\n" +
	"\r\n" +
	"prt_church.rsw#bgm\\10.mp3#\r\n" +
	"prontera.rsw#bgm\\08.mp3#\r\n" +
	"//prontera.rsw#bgm\\55.mp3#\r\n" +
	"prt_fild08.rsw#bgm\\12.mp3#\r\n" +
	"broken\r\n")

func TestParseNameTable(t *testing.T) {
	table := ParseNameTable(testNameTable)

	want := map[string]string{
		"prt_church.rsw": "bgm/10.mp3",
		"prontera.rsw":   "bgm/08.mp3",
		"prt_fild08.rsw": "bgm/12.mp3",
	}

	if len(table) != len(want) {
		t.Fatalf("parsed %d entries, want %d: %v", len(table), len(want), table)
	}

	for resource, track := range want {
		if got := table[resource]; got != track {
			t.Errorf("table[%q] = %q, want %q", resource, got, track)
		}
	}
}

func TestNameTableLookup(t *testing.T) {
	table := ParseNameTable(testNameTable)

	// Everything that names the same map has to reach the same track.
	found := []string{
		"prontera",
		"prontera.rsw",
		"prontera.gat",
		"prontera.gnd",
		"PRONTERA",
		"  prontera  ",
		"data/prontera.gat",
		`data\prontera.rsw`,
	}

	for _, location := range found {
		track, ok := table.Lookup(location)
		if !ok {
			t.Errorf("Lookup(%q) not found", location)

			continue
		}

		if track != "bgm/08.mp3" {
			t.Errorf("Lookup(%q) = %q, want %q", location, track, "bgm/08.mp3")
		}
	}

	// A location that does not exist is the fallback's job, not a lookup hit.
	for _, location := range []string{"no_such_map", "prontera_fld", ""} {
		if _, ok := table.Lookup(location); ok {
			t.Errorf("Lookup(%q) found a track, want none", location)
		}
	}
}

func TestNormalizeLocation(t *testing.T) {
	tests := []struct {
		location string
		want     string
	}{
		{"prontera", "prontera"},
		{"PRONTERA.RSW", "prontera"},
		{"prontera.gat", "prontera"},
		{"data/prontera.gnd", "prontera"},
		{`data\prontera.rsw`, "prontera"},
		{"  prt_fild08.rsw  ", "prt_fild08"},
		{"new_1-1", "new_1-1"},
		{"", ""},
	}

	for _, tt := range tests {
		if got := NormalizeLocation(tt.location); got != tt.want {
			t.Errorf("NormalizeLocation(%q) = %q, want %q", tt.location, got, tt.want)
		}
	}
}

func TestNormalizeTrack(t *testing.T) {
	tests := []struct {
		track string
		want  string
	}{
		{`bgm\\08.mp3`, "bgm/08.mp3"}, // the escaped separator the table really uses
		{`bgm\08.mp3`, "bgm/08.mp3"},
		{"bgm/08.mp3", "bgm/08.mp3"},
		{"  bgm/08.mp3  ", "bgm/08.mp3"},
		{`\bgm\08.mp3`, "bgm/08.mp3"},
		{"", ""},
	}

	for _, tt := range tests {
		if got := NormalizeTrack(tt.track); got != tt.want {
			t.Errorf("NormalizeTrack(%q) = %q, want %q", tt.track, got, tt.want)
		}
	}
}
