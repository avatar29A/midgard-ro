package states

import (
	"fmt"
	"strconv"
	"strings"

	"go.uber.org/zap"

	"github.com/Faultbox/midgard-ro/internal/game/command"
	"github.com/Faultbox/midgard-ro/internal/logger"
	"github.com/Faultbox/midgard-ro/internal/network/packets"
	"github.com/Faultbox/midgard-ro/internal/trace"
)

// localCommand runs one `/` command and returns the line to show for it.
//
// Every `/` command must answer with something: nothing goes to the server, so
// a command that returned silence would be indistinguishable from one that was
// never recognized.
type localCommand func(s *InGameState, args string) (ChatKind, string)

// CommandHost is what the game layer lends to a `/` command — the things the
// game state itself cannot reach.
//
// Audio lives on Game, along with the settings file it is persisted to, and
// this package has no business importing either to turn the music off.
type CommandHost interface {
	// ToggleBGM turns background music on or off, reporting the new state.
	ToggleBGM() bool
	// ToggleSFX does the same for sound effects.
	ToggleSFX() bool
}

// host is the game layer's CommandHost, or nil when there is none — which is
// the case in a test, and before the game layer has wired one up.
func (s *InGameState) host() CommandHost {
	if s.manager == nil {
		return nil
	}

	return s.manager.CommandHost
}

// localCommands is the `/` table — the commands this client answers itself.
//
// A `/` line that is not in here is reported unknown and sent nowhere. That is
// not a fallback but the only safe answer: nothing on the server parses a
// leading slash, so a `/` line handed to the network would be said out loud to
// everyone in range.
//
// Aliases are separate entries pointing at the same function, as the original
// treats them — `/h` and `/help` are both real names, not one with a lookup
// through the other.
var localCommands = map[string]localCommand{
	"where": cmdWhere,
	"who":   cmdWho,
	"w":     cmdWho,
	"h":     cmdHelp,
	"help":  cmdHelp,
	"bgm":   cmdBGM,
	"music": cmdBGM,
	"sound": cmdSound,

	// The GM three. Aliases as tools/chatcmds/slash.json records them.
	"mm":      cmdMapMove,
	"mapmove": cmdMapMove,
	"b":       cmdBroadcast,
	"nb":      cmdBroadcast,
	"lb":      cmdLocalBroadcast,
	"nlb":     cmdLocalBroadcast,
}

// cmdWhere prints the map and cell the character is standing on.
//
// Answered here rather than by @where, which needs a character name and is a
// GM command. This is the client reading its own state, which is what makes it
// the cheapest check that our idea of the position matches the server's.
func cmdWhere(s *InGameState, _ string) (ChatKind, string) {
	player := s.GetPlayer()
	if player == nil {
		return ChatError, "You are not on a map yet."
	}

	cellX, cellY := player.CurrentCell()

	// The server names maps with a .gat suffix in some packets and without it
	// in others; the player should always see the plain name.
	return ChatNotice, fmt.Sprintf("%s : %d, %d",
		packets.MapBaseName(s.MapName), cellX, cellY)
}

// cmdWho asks the server how many players are online.
//
// The only `/` command here that needs a round trip: the count is the server's
// to know. The answer arrives on ZC_USER_COUNT and is printed by its handler,
// so nothing is returned now.
func cmdWho(s *InGameState, _ string) (ChatKind, string) {
	if s.client == nil {
		return ChatError, "Not connected."
	}

	trace.Emit(trace.Cmd, "who-request")

	if err := s.client.Send(packets.EncodeUserCount()); err != nil {
		logger.Warn("could not ask how many players are online", zap.Error(err))

		return ChatError, "Could not ask the server."
	}

	return ChatNotice, ""
}

// commandHelp is what /help lists, in the order it lists them.
//
// Kept apart from localCommands for two reasons. Reading the table directly
// would be an initialization cycle — the table holds cmdHelp — and it would
// print every alias flat, so "/bgm, /h, /help, /music, /sound, /w, /where,
// /who" instead of five commands with their alternatives. A test asserts the
// two never drift.
var commandHelp = []struct {
	name    string
	aliases []string
}{
	{"where", nil},
	{"who", []string{"w"}},
	{"help", []string{"h"}},
	{"bgm", []string{"music"}},
	{"sound", nil},
	{"mm", []string{"mapmove"}},
	{"b", []string{"nb"}},
	{"lb", []string{"nlb"}},
}

// cmdHelp lists the commands this client answers.
//
// Deliberately only the `/` ones. The `@` list is the server's, and it already
// has a command for it — @commands — which prints what this account may
// actually use, something the client has no way to know.
func cmdHelp(_ *InGameState, _ string) (ChatKind, string) {
	parts := make([]string, 0, len(commandHelp))
	for _, c := range commandHelp {
		entry := "/" + c.name
		if len(c.aliases) > 0 {
			entry += " (/" + strings.Join(c.aliases, ", /") + ")"
		}
		parts = append(parts, entry)
	}

	return ChatNotice, "Commands: " + strings.Join(parts, ", ") +
		". For server commands use @commands."
}

// cmdBGM turns the background music on or off.
func cmdBGM(s *InGameState, _ string) (ChatKind, string) {
	host := s.host()
	if host == nil {
		return ChatError, "Sound is not available."
	}

	return ChatNotice, "Background music " + onOff(host.ToggleBGM()) + "."
}

// cmdSound turns sound effects on or off.
func cmdSound(s *InGameState, _ string) (ChatKind, string) {
	host := s.host()
	if host == nil {
		return ChatError, "Sound is not available."
	}

	return ChatNotice, "Sound effects " + onOff(host.ToggleSFX()) + "."
}

func onOff(on bool) string {
	if on {
		return "on"
	}

	return "off"
}

// The three `/` commands that carry their own packet.
//
// Each is a GM command the server converts straight back into an atcommand.
// They are sent as packets rather than as `@` text for one reason: a refused
// atcommand falls through to ordinary chat, so a non-GM typing `@kami hi`
// shouts "@kami hi" at the map. As packets they are refused in silence.
//
// Nothing is printed locally on success. Whatever the command did is the
// server's to report — @mapmove answers on 0x008E, @kami announces — and
// printing our own line first would claim something happened before the
// server had agreed. For a non-GM that silence *is* the answer, and it is
// the same silence the original client gives.

// cmdMapMove warps to a map and cell.
//
// Coordinates are optional: @mapmove picks a walkable cell itself when given
// zero, which is what the original does for a bare `/mm <map>`.
func cmdMapMove(s *InGameState, args string) (ChatKind, string) {
	fields := strings.Fields(args)
	if len(fields) == 0 {
		return ChatError, "Usage: /mm <map> [<x> <y>]"
	}

	mapName := fields[0]

	var x, y uint16
	if len(fields) >= 3 {
		px, errX := strconv.ParseUint(fields[1], 10, 16)
		py, errY := strconv.ParseUint(fields[2], 10, 16)
		if errX != nil || errY != nil {
			return ChatError, "Usage: /mm <map> [<x> <y>]"
		}
		x, y = uint16(px), uint16(py)
	}

	pkt := packets.EncodeMapMove(mapName, x, y)
	if pkt == nil {
		// The only way to get here is a name too long for the 16-byte field.
		return ChatError, fmt.Sprintf("Map name is too long: %s", mapName)
	}

	return sendCommandPacket(s, pkt, "mapmove")
}

// cmdBroadcast announces to the whole server (@kami).
func cmdBroadcast(s *InGameState, args string) (ChatKind, string) {
	if args == "" {
		return ChatError, "Usage: /b <message>"
	}

	return sendCommandPacket(s, packets.EncodeBroadcast(args), "broadcast")
}

// cmdLocalBroadcast announces on this map only (@lkami).
func cmdLocalBroadcast(s *InGameState, args string) (ChatKind, string) {
	if args == "" {
		return ChatError, "Usage: /lb <message>"
	}

	return sendCommandPacket(s, packets.EncodeLocalBroadcast(args), "localbroadcast")
}

// sendCommandPacket sends one of the GM packets and answers with nothing on
// success, so the server is the only thing that reports what happened.
func sendCommandPacket(s *InGameState, pkt []byte, what string) (ChatKind, string) {
	if s.client == nil {
		return ChatError, "Not connected."
	}
	if pkt == nil {
		return ChatError, "Nothing to send."
	}

	trace.Emit(trace.Cmd, "server-packet", zap.String("command", what))

	if err := s.client.Send(pkt); err != nil {
		logger.Warn("could not send a command packet",
			zap.String("command", what), zap.Error(err))

		return ChatError, "Could not reach the server."
	}

	return ChatNotice, ""
}

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
// must not reach the server: it would not be recognized as a command — nothing
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
