package states

import (
	"github.com/Faultbox/midgard-ro/internal/network/packets"
)

// ChatBacklog is how many lines the chat box remembers.
//
// The original keeps a long history behind a scrollbar. This is bounded
// because the log grows for the whole session and nothing else trims it — a
// busy server would otherwise fill memory with text nobody will scroll back to.
const ChatBacklog = 200

// ChatKind says where a line came from, which is what decides its color.
//
// This is the client's own view of a line, deliberately not the wire's. Three
// of the kinds below — the command a player typed, the client's own answer to
// it, and the error saying a command does not exist — never travel in any
// packet, and putting them in packets.ChatKind would make the network layer
// describe states of the interface. packets.ChatKind stays describing only
// what the server sends; chatKindFromPacket folds it into this.
type ChatKind uint8

// Where a chat line came from.
const (
	// ChatOther is someone else speaking in range.
	ChatOther ChatKind = iota
	// ChatSelf is our own line, echoed back by the server.
	ChatSelf
	// ChatBroadcast is a server-wide announcement.
	ChatBroadcast
	// ChatSystem is the server talking to us rather than a person — the
	// welcome lines, and every reply to an @ command.
	ChatSystem
	// ChatWhisper is a private message.
	ChatWhisper
	// ChatDamage is a battle line. Nothing produces one yet — the combat
	// packets are Track F — but the chat box already knows how to color it.
	ChatDamage
	// ChatCommand is the command the player typed, echoed back by us. The
	// server never echoes a command: it answers one, or swallows it.
	ChatCommand
	// ChatNotice is the client answering for itself — what /where prints,
	// what /bgm says it did.
	ChatNotice
	// ChatError is a command that could not run: unknown, malformed, or
	// refused before anything was sent.
	ChatError
)

// chatKindFromPacket maps a wire kind onto ours.
//
// The two enums are separate on purpose and must not be assumed to share
// values; this switch is the only place that knows they relate at all.
func chatKindFromPacket(kind packets.ChatKind) ChatKind {
	switch kind {
	case packets.ChatSelf:
		return ChatSelf
	case packets.ChatBroadcast:
		return ChatBroadcast
	case packets.ChatSystem:
		return ChatSystem
	case packets.ChatWhisper:
		return ChatWhisper
	case packets.ChatDamage:
		return ChatDamage
	default:
		return ChatOther
	}
}

// ChatLine is one line as the chat box shows it.
type ChatLine struct {
	Kind    ChatKind
	Speaker string
	Text    string
}

// ChatLog is the bounded scrollback behind the chat box.
//
// The zero value is usable.
type ChatLog struct {
	lines []ChatLine
}

// Add appends a decoded message. A nil message or an entirely empty line is
// dropped: the server sends blank lines as spacing, and they would scroll real
// text off the top.
func (l *ChatLog) Add(msg *packets.ChatMessage) {
	if msg == nil || (msg.Text == "" && msg.Speaker == "") {
		return
	}

	l.append(ChatLine{
		Kind:    chatKindFromPacket(msg.Kind),
		Speaker: msg.Speaker,
		Text:    msg.Text,
	})
}

// AddLocal appends a line the client wrote itself — a command echo, an answer
// we produced without asking the server, or an error.
//
// Unlike Add this keeps a line with no text, because a caller asking for one
// meant it; the blank-line spacing Add guards against only comes off the wire.
func (l *ChatLog) AddLocal(kind ChatKind, text string) {
	l.append(ChatLine{Kind: kind, Text: text})
}

func (l *ChatLog) append(line ChatLine) {
	l.lines = append(l.lines, line)

	if len(l.lines) > ChatBacklog {
		// Re-slice from the excess rather than shifting: the backing array is
		// reused and the oldest lines are simply no longer reachable.
		l.lines = l.lines[len(l.lines)-ChatBacklog:]
	}
}

// Lines returns the backlog, oldest first. The slice must not be modified.
func (l *ChatLog) Lines() []ChatLine {
	return l.lines
}

// Len is how many lines are held.
func (l *ChatLog) Len() int {
	return len(l.lines)
}
