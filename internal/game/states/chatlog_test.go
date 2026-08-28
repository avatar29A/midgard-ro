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
	if lines[0].Kind != ChatBroadcast {
		t.Errorf("kind = %d, want ChatBroadcast", lines[0].Kind)
	}
	if lines[1].Speaker != "Someone" {
		t.Errorf("speaker = %q, want Someone", lines[1].Speaker)
	}
}

// TestChatKindFromPacket: the wire kinds and ours are separate enums that
// happen to overlap today, and nothing may assume they stay numerically equal.
// Every wire kind must land on the kind the box colors by.
func TestChatKindFromPacket(t *testing.T) {
	tests := []struct {
		name string
		wire packets.ChatKind
		want ChatKind
	}{
		{"other", packets.ChatOther, ChatOther},
		{"self", packets.ChatSelf, ChatSelf},
		{"broadcast", packets.ChatBroadcast, ChatBroadcast},
		{"system", packets.ChatSystem, ChatSystem},
		{"whisper", packets.ChatWhisper, ChatWhisper},
		{"damage", packets.ChatDamage, ChatDamage},
		// A kind we do not know is somebody talking, not a silent drop.
		{"unknown falls back to other", packets.ChatKind(200), ChatOther},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := chatKindFromPacket(tt.wire); got != tt.want {
				t.Errorf("chatKindFromPacket(%d) = %d, want %d", tt.wire, got, tt.want)
			}
		})
	}
}

// TestAddLocalKeepsEmptyText: Add drops a blank line because the server sends
// those as spacing, but a caller asking for one here meant it.
func TestAddLocalKeepsEmptyText(t *testing.T) {
	var log ChatLog

	log.Add(&packets.ChatMessage{Kind: packets.ChatOther})
	if got := log.Len(); got != 0 {
		t.Fatalf("Add kept %d blank lines from the wire, want 0", got)
	}

	log.AddLocal(ChatNotice, "")
	if got := log.Len(); got != 1 {
		t.Fatalf("AddLocal held %d lines, want 1", got)
	}
}

// TestAddLocalKinds: the client-side kinds exist so the box can color them
// apart from anything the server said.
func TestAddLocalKinds(t *testing.T) {
	var log ChatLog

	log.AddLocal(ChatNotice, "prontera 156, 191")
	log.AddLocal(ChatError, "Unknown command")

	lines := log.Lines()
	want := []ChatKind{ChatNotice, ChatError}
	for i, w := range want {
		if lines[i].Kind != w {
			t.Errorf("line %d kind = %d, want %d", i, lines[i].Kind, w)
		}
		if lines[i].Speaker != "" {
			t.Errorf("line %d speaker = %q, want empty", i, lines[i].Speaker)
		}
	}
}

// TestAddLocalIsBounded: the backlog cap applies however a line got in.
func TestAddLocalIsBounded(t *testing.T) {
	var log ChatLog

	for i := 0; i < ChatBacklog+50; i++ {
		log.AddLocal(ChatNotice, fmt.Sprintf("line %d", i))
	}

	if got := log.Len(); got != ChatBacklog {
		t.Errorf("held %d lines, want the cap of %d", got, ChatBacklog)
	}
}
