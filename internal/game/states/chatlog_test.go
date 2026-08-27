package states

import (
	"fmt"
	"testing"

	"github.com/Faultbox/midgard-ro/internal/network/packets"
)

func TestChatLogKeepsOrder(t *testing.T) {
	var log ChatLog

	for i := 0; i < 3; i++ {
		log.Add(&packets.ChatMessage{Kind: packets.ChatOther, Text: fmt.Sprintf("line %d", i)})
	}

	lines := log.Lines()
	if len(lines) != 3 {
		t.Fatalf("held %d lines, want 3", len(lines))
	}
	for i, line := range lines {
		if want := fmt.Sprintf("line %d", i); line.Text != want {
			t.Errorf("line %d = %q, want %q — oldest should come first", i, line.Text, want)
		}
	}
}

// TestChatLogIsBounded: the log grows for the whole session and nothing else
// trims it, so a busy server would fill memory with text nobody scrolls to.
func TestChatLogIsBounded(t *testing.T) {
	var log ChatLog

	for i := 0; i < ChatBacklog*2; i++ {
		log.Add(&packets.ChatMessage{Kind: packets.ChatOther, Text: fmt.Sprintf("line %d", i)})
	}

	if got := log.Len(); got != ChatBacklog {
		t.Errorf("held %d lines, want the cap of %d", got, ChatBacklog)
	}

	// The newest must survive; the oldest must be the ones dropped.
	lines := log.Lines()
	if want := fmt.Sprintf("line %d", ChatBacklog*2-1); lines[len(lines)-1].Text != want {
		t.Errorf("newest line = %q, want %q", lines[len(lines)-1].Text, want)
	}
	if want := fmt.Sprintf("line %d", ChatBacklog); lines[0].Text != want {
		t.Errorf("oldest kept line = %q, want %q", lines[0].Text, want)
	}
}

// TestChatLogDropsEmptyLines: the server sends blank lines as spacing, and
// keeping them would scroll real text off the top for nothing.
func TestChatLogDropsEmptyLines(t *testing.T) {
	var log ChatLog

	log.Add(nil)
	log.Add(&packets.ChatMessage{Kind: packets.ChatOther})
	log.Add(&packets.ChatMessage{Kind: packets.ChatOther, Text: "real"})

	if got := log.Len(); got != 1 {
		t.Errorf("held %d lines, want only the one with text", got)
	}
}

// TestChatLogKeepsSpeakerAndKind: the box colors a line by where it came from
// and draws the speaker separately, so both have to survive the log.
func TestChatLogKeepsSpeakerAndKind(t *testing.T) {
	var log ChatLog

	log.Add(&packets.ChatMessage{Kind: packets.ChatBroadcast, Text: "server up"})
	log.Add(&packets.ChatMessage{Kind: packets.ChatOther, Speaker: "Someone", Text: "hello"})

	lines := log.Lines()
	if lines[0].Kind != packets.ChatBroadcast {
		t.Errorf("kind = %d, want ChatBroadcast", lines[0].Kind)
	}
	if lines[1].Speaker != "Someone" {
		t.Errorf("speaker = %q, want Someone", lines[1].Speaker)
	}
}
