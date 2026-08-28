package states

import (
	"fmt"

	"go.uber.org/zap"

	"github.com/Faultbox/midgard-ro/internal/game/command"
	"github.com/Faultbox/midgard-ro/internal/trace"
)

// localCommand runs one `/` command and returns the line to show for it.
//
// Every `/` command must answer with something: nothing goes to the server, so
// a command that returned silence would be indistinguishable from one that was
// never recognised.
type localCommand func(s *InGameState, args string) (ChatKind, string)

// localCommands is the `/` table — the commands this client answers itself.
//
// Empty until step 3 of #94 fills it. Until then every `/` line reports itself
// unknown, which is the right answer for a command we do not implement and is
// emphatically better than the alternative: a `/` line sent as chat is said out
// loud to everyone in range, because nothing on the server parses a slash.
var localCommands = map[string]localCommand{}

// chatIntent is what should become of a line the player entered.
type chatIntent uint8

const (
	// intentSpeak says the line out loud, to everyone in range.
	intentSpeak chatIntent = iota
	// intentWhisper says it privately to the name in the chat's name field.
	intentWhisper
	// intentServerCommand sends it as public chat for the server to run.
	intentServerCommand
	// intentLocalCommand answers it here, sending nothing.
	intentLocalCommand
)

// routeLine decides where a line goes. Pure, so the rule below can be tested
// without a server.
//
// The rule that matters: a command ignores the whisper name field entirely.
// That is not a convenience — only the public chat path runs commands
// (rAthena's clif_process_message), and clif_parse_WisMessage never looks. An
// `@` command sent as a whisper is not refused; it is quietly said to one
// person instead of being run, which looks like the command silently failing.
func routeLine(line command.Line, hasTarget bool) chatIntent {
	switch {
	case line.IsServerCommand():
		return intentServerCommand
	case line.Sigil == command.Slash:
		return intentLocalCommand
	case hasTarget:
		return intentWhisper
	default:
		return intentSpeak
	}
}

// SubmitLine acts on one line the player entered in the chat box.
//
// target is the whisper name field, which applies to speech only — see
// routeLine for why a command must ignore it.
func (s *InGameState) SubmitLine(target, text string) error {
	line := command.Parse(text)
	intent := routeLine(line, target != "")

	trace.Emit(trace.Cmd, "parse",
		zap.String("sigil", line.Sigil.String()),
		zap.String("name", line.Name),
		zap.Bool("whisperField", target != ""))

	switch intent {
	case intentServerCommand:
		return s.sendServerCommand(line)
	case intentLocalCommand:
		s.runLocalCommand(line)

		return nil
	case intentWhisper:
		return s.SendWhisper(target, text)
	default:
		return s.SendChat(text)
	}
}

// sendServerCommand hands an `@` or `#` line to the server as public chat.
//
// The raw line goes, not anything reassembled from the parse: the server
// parses it again itself, and it verifies the whole message against the
// character's name, so the bytes it reads should be the bytes that were typed.
//
// Nothing is added to the scrollback here. The original does not echo a
// command, and there is nothing to echo: a command the server runs is answered
// on 0x008E, and one it refuses comes back as ordinary chat with our own name
// in front of it — which is the server telling us it was refused.
func (s *InGameState) sendServerCommand(line command.Line) error {
	trace.Emit(trace.Cmd, "server",
		zap.String("sigil", line.Sigil.String()),
		zap.String("name", line.Name))

	return s.SendChat(line.Raw)
}

// runLocalCommand answers a `/` command, or says it is not one we know.
//
// Nothing is ever sent from here. A `/` line the client does not implement
// must not reach the server: it would not be recognised as a command — nothing
// server-side parses a leading slash — and would be broadcast to everyone in
// range instead.
func (s *InGameState) runLocalCommand(line command.Line) {
	if line.Name == "" {
		s.chat.AddLocal(ChatError, "Type a command after the /.")
		trace.Emit(trace.Cmd, "unknown", zap.String("name", ""))

		return
	}

	run, ok := localCommands[line.Name]
	if !ok {
		s.chat.AddLocal(ChatError, fmt.Sprintf("Unknown command: /%s", line.Name))
		trace.Emit(trace.Cmd, "unknown", zap.String("name", line.Name))

		return
	}

	kind, answer := run(s, line.Args)
	if answer != "" {
		s.chat.AddLocal(kind, answer)
	}

	trace.Emit(trace.Cmd, "local",
		zap.String("name", line.Name),
		zap.Bool("answered", answer != ""))
}
