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

// ChatLine is one line as the chat box shows it.
type ChatLine struct {
	Kind    packets.ChatKind
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

	l.lines = append(l.lines, ChatLine{
		Kind:    msg.Kind,
		Speaker: msg.Speaker,
		Text:    msg.Text,
	})

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
