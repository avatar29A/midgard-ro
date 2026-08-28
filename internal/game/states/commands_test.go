package states

import (
	"strings"
	"testing"

	"github.com/Faultbox/midgard-ro/internal/game/command"
)

// TestRouteLine: where each kind of line goes, and in particular that a
// command ignores the whisper name field.
//
// This is the subtlest rule in the feature and the one with no visible
// symptom: only the public chat path runs commands, so an @ command sent as a
// whisper is not refused — it is quietly said to one person, and looks exactly
// like the command having done nothing.
func TestRouteLine(t *testing.T) {
	tests := []struct {
		name      string
		text      string
		hasTarget bool
		want      chatIntent
	}{
		{"speech, no target", "hello", false, intentSpeak},
		{"speech, with target", "hello", true, intentWhisper},

		{"at command, no target", "@go 1", false, intentServerCommand},
		{"at command with target", "@go 1", true, intentServerCommand},
		{"char command, no target", "#zeny Someone 1", false, intentServerCommand},
		{"char command with target", "#zeny Someone 1", true, intentServerCommand},

		{"slash command, no target", "/where", false, intentLocalCommand},
		{"slash command with target", "/where", true, intentLocalCommand},
		{"bare slash with target", "/", true, intentLocalCommand},

		// A sigil in the middle of a sentence is not a command, so the whisper
		// field still decides.
		{"slash mid-sentence, no target", "and/or", false, intentSpeak},
		{"slash mid-sentence, with target", "and/or", true, intentWhisper},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := routeLine(command.Parse(tt.text), tt.hasTarget)
			if got != tt.want {
				t.Errorf("routeLine(%q, target=%v) = %d, want %d",
					tt.text, tt.hasTarget, got, tt.want)
			}
		})
	}
}

// TestUnknownSlashCommandIsNotSent: a / command we do not implement must
// answer in the box and send nothing.
//
// Nothing on the server parses a leading slash, so a / line that escaped to
// the network would not fail quietly — it would be said out loud to everyone
// in range.
func TestUnknownSlashCommandIsNotSent(t *testing.T) {
	// A zero InGameState has no client, so anything that tried to send would
	// panic here rather than pass quietly. That is the assertion.
	var s InGameState

	s.runLocalCommand(command.Parse("/nonsense"))

	lines := s.chat.Lines()
	if len(lines) != 1 {
		t.Fatalf("added %d lines, want exactly one", len(lines))
	}
	if lines[0].Kind != ChatError {
		t.Errorf("kind = %d, want ChatError", lines[0].Kind)
	}
	if !strings.Contains(lines[0].Text, "nonsense") {
		t.Errorf("text = %q, want it to name the command", lines[0].Text)
	}
}

// TestBareSlashIsHandled: "/" alone must not crash or be sent.
func TestBareSlashIsHandled(t *testing.T) {
	var s InGameState

	s.runLocalCommand(command.Parse("/"))

	lines := s.chat.Lines()
	if len(lines) != 1 {
		t.Fatalf("added %d lines, want exactly one", len(lines))
	}
	if lines[0].Kind != ChatError {
		t.Errorf("kind = %d, want ChatError", lines[0].Kind)
	}
}

// TestLocalCommandAnswers: a command in the table runs and its answer reaches
// the scrollback with the kind it asked for.
func TestLocalCommandAnswers(t *testing.T) {
	restore := localCommands
	localCommands = map[string]localCommand{
		"ping": func(_ *InGameState, args string) (ChatKind, string) {
			return ChatNotice, "pong " + args
		},
		"quiet": func(_ *InGameState, _ string) (ChatKind, string) {
			return ChatNotice, ""
		},
	}
	defer func() { localCommands = restore }()

	t.Run("answer is shown", func(t *testing.T) {
		var s InGameState
		s.runLocalCommand(command.Parse("/ping there"))

		lines := s.chat.Lines()
		if len(lines) != 1 {
			t.Fatalf("added %d lines, want one", len(lines))
		}
		if lines[0].Text != "pong there" {
			t.Errorf("text = %q, want %q", lines[0].Text, "pong there")
		}
		if lines[0].Kind != ChatNotice {
			t.Errorf("kind = %d, want ChatNotice", lines[0].Kind)
		}
	})

	t.Run("an empty answer adds no line", func(t *testing.T) {
		var s InGameState
		s.runLocalCommand(command.Parse("/quiet"))

		if got := s.chat.Len(); got != 0 {
			t.Errorf("added %d lines, want none", got)
		}
	})

	t.Run("lookup is case-insensitive", func(t *testing.T) {
		var s InGameState
		s.runLocalCommand(command.Parse("/PING x"))

		if got := s.chat.Len(); got != 1 {
			t.Fatalf("added %d lines, want one", got)
		}
		if lines := s.chat.Lines(); lines[0].Kind != ChatNotice {
			t.Errorf("kind = %d, want ChatNotice — /PING should resolve to /ping",
				lines[0].Kind)
		}
	})
}

// TestHelpCoversEveryCommand: /help is written by hand so it can group aliases,
// which means it can drift from the table it describes. It must not.
func TestHelpCoversEveryCommand(t *testing.T) {
	listed := map[string]bool{}
	for _, c := range commandHelp {
		listed[c.name] = true
		for _, a := range c.aliases {
			listed[a] = true
		}
	}

	for name := range localCommands {
		if !listed[name] {
			t.Errorf("/%s is a command but /help never mentions it", name)
		}
	}
	for name := range listed {
		if _, ok := localCommands[name]; !ok {
			t.Errorf("/help offers /%s, which is not a command", name)
		}
	}
}

// TestHelpNamesTheServerList: /help covers only the / commands, so it has to
// point at @commands for the rest — the client cannot know what the account
// is allowed to run.
func TestHelpNamesTheServerList(t *testing.T) {
	var s InGameState

	kind, text := cmdHelp(&s, "")
	if kind != ChatNotice {
		t.Errorf("kind = %d, want ChatNotice", kind)
	}
	for _, want := range []string{"/where", "/who (/w)", "/help (/h)", "@commands"} {
		if !strings.Contains(text, want) {
			t.Errorf("help text is missing %q: %s", want, text)
		}
	}
}

// TestWhereNeedsAPlayer: /where before the character exists must say so rather
// than reporting a position of zero, which reads as a real cell.
func TestWhereNeedsAPlayer(t *testing.T) {
	var s InGameState

	kind, text := cmdWhere(&s, "")
	if kind != ChatError {
		t.Errorf("kind = %d, want ChatError", kind)
	}
	if strings.Contains(text, "0, 0") {
		t.Errorf("reported a position anyway: %q", text)
	}
}

// TestSoundCommandsWithoutAHost: with no host the sound commands report it
// instead of pretending to have worked.
func TestSoundCommandsWithoutAHost(t *testing.T) {
	var s InGameState

	for _, run := range []localCommand{cmdBGM, cmdSound} {
		kind, text := run(&s, "")
		if kind != ChatError {
			t.Errorf("kind = %d, want ChatError", kind)
		}
		if text == "" {
			t.Error("said nothing at all")
		}
	}
}

// fakeHost records what was toggled.
type fakeHost struct{ bgm, sfx bool }

func (f *fakeHost) ToggleBGM() bool { f.bgm = !f.bgm; return f.bgm }
func (f *fakeHost) ToggleSFX() bool { f.sfx = !f.sfx; return f.sfx }

// TestSoundCommandsReportTheNewState: the answer has to say which way it went,
// because there is nothing else on screen that does.
func TestSoundCommandsReportTheNewState(t *testing.T) {
	host := &fakeHost{}
	s := InGameState{manager: &Manager{CommandHost: host}}

	if _, text := cmdBGM(&s, ""); !strings.Contains(text, "on") {
		t.Errorf("first toggle said %q, want it to report on", text)
	}
	if _, text := cmdBGM(&s, ""); !strings.Contains(text, "off") {
		t.Errorf("second toggle said %q, want it to report off", text)
	}
	if host.bgm {
		t.Error("after two toggles the music should be back off")
	}

	if _, text := cmdSound(&s, ""); !strings.Contains(text, "on") {
		t.Errorf("sfx toggle said %q, want it to report on", text)
	}
	if host.bgm {
		t.Error("toggling sound effects also moved the music")
	}
}
