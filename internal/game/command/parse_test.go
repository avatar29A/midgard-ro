package command

import "testing"

func TestParse(t *testing.T) {
	tests := []struct {
		name  string
		in    string
		sigil Sigil
		cmd   string
		args  string
	}{
		{"plain speech", "hello there", Speech, "", ""},
		{"empty", "", Speech, "", ""},
		{"only spaces", "   ", Speech, "", ""},

		{"slash, no args", "/where", Slash, "where", ""},
		{"slash with args", "/mm prontera 150 150", Slash, "mm", "prontera 150 150"},
		{"at, no args", "@commands", At, "commands", ""},
		{"at with args", "@go 1", At, "go", "1"},
		{"char command", "#zeny MidgardTest 100", Char, "zeny", "MidgardTest 100"},

		// The sigil only counts at the start. A slash inside a sentence is a
		// slash, not a command, or "and/or" would never reach anyone.
		{"slash mid-sentence", "and/or", Speech, "", ""},
		{"at mid-sentence", "mail me @ home", Speech, "", ""},
		{"hash mid-sentence", "channel #1 please", Speech, "", ""},

		// The name is matched case-insensitively, as the server does...
		{"name is lowercased", "@GO 1", At, "go", "1"},
		{"mixed case name", "/WhErE", Slash, "where", ""},
		// ...but the arguments are not: character names live there.
		{"args keep their case", "#zeny MidgardTest 100", Char, "zeny", "MidgardTest 100"},

		// Whitespace between the command and its arguments collapses; inside
		// them it does not, because a script argument may want it.
		{"extra spaces before args", "@go    1", At, "go", "1"},
		{"tab separator", "@go\t1", At, "go", "1"},
		{"spaces inside args are kept", "/b hello  there", Slash, "b", "hello  there"},
		{"leading whitespace before sigil", "  @go 1", At, "go", "1"},

		// A bare sigil is a command with no name. It must not be mistaken for
		// speech — saying "/" out loud is exactly the accident to avoid.
		{"bare slash", "/", Slash, "", ""},
		{"bare at", "@", At, "", ""},
		{"bare hash", "#", Char, "", ""},
		{"slash then space", "/ ", Slash, "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Parse(tt.in)
			if got.Sigil != tt.sigil {
				t.Errorf("sigil = %v, want %v", got.Sigil, tt.sigil)
			}
			if got.Name != tt.cmd {
				t.Errorf("name = %q, want %q", got.Name, tt.cmd)
			}
			if got.Args != tt.args {
				t.Errorf("args = %q, want %q", got.Args, tt.args)
			}
			if got.Raw != tt.in {
				t.Errorf("raw = %q, want the line as typed %q", got.Raw, tt.in)
			}
		})
	}
}

// TestIsServerCommand: only @ and # are the server's. Getting this wrong in
// either direction is bad — a / sent as chat is said out loud, and an @ not
// sent is silently swallowed.
func TestIsServerCommand(t *testing.T) {
	tests := []struct {
		in     string
		server bool
		anyCmd bool
	}{
		{"@go 1", true, true},
		{"#zeny Someone 1", true, true},
		{"/where", false, true},
		{"hello", false, false},
		{"/", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got := Parse(tt.in)
			if got.IsServerCommand() != tt.server {
				t.Errorf("IsServerCommand() = %v, want %v", got.IsServerCommand(), tt.server)
			}
			if got.IsCommand() != tt.anyCmd {
				t.Errorf("IsCommand() = %v, want %v", got.IsCommand(), tt.anyCmd)
			}
		})
	}
}

// TestRawIsWhatGetsSent: an @ command is sent to the server verbatim, because
// the server parses it again and anything reassembled here could differ from
// what the player wrote.
func TestRawIsWhatGetsSent(t *testing.T) {
	const line = "@item   Red_Potion   10"

	got := Parse(line)
	if got.Raw != line {
		t.Errorf("Raw = %q, want the original %q", got.Raw, line)
	}
	if got.Name != "item" {
		t.Errorf("Name = %q, want item", got.Name)
	}
	// The parsed args are for tracing; note they are not what is transmitted.
	if got.Args != "Red_Potion   10" {
		t.Errorf("Args = %q, want the run of spaces after the name removed only", got.Args)
	}
}

func TestSigilString(t *testing.T) {
	tests := []struct {
		sigil Sigil
		want  string
	}{
		{Speech, "speech"},
		{Slash, "slash"},
		{At, "at"},
		{Char, "char"},
	}

	for _, tt := range tests {
		if got := tt.sigil.String(); got != tt.want {
			t.Errorf("Sigil(%d).String() = %q, want %q", tt.sigil, got, tt.want)
		}
	}
}
