package packets

import (
	"bytes"
	"strings"
)

// Chat, as the server sends it.
//
// Offsets from tools/packetlen/layout.py against the server's headers at
// PACKETVER 20211103.
//
//	ZC_NOTIFY_CHAT        type(2) len(2) GID(4) message[]
//	ZC_NOTIFY_PLAYERCHAT  type(2) len(2)        message[]
//	ZC_BROADCAST          type(2) len(2)        message[]
const (
	// ZC_NOTIFY_CHAT is someone else speaking, carrying who said it.
	ZC_NOTIFY_CHAT uint16 = 0x008D
	// ZC_NOTIFY_PLAYERCHAT is our own line echoed back, and is also what
	// carries the server's own messages to us — rAthena's welcome lines
	// arrive this way.
	ZC_NOTIFY_PLAYERCHAT uint16 = 0x008E
	// ZC_BROADCAST is a server-wide announcement.
	ZC_BROADCAST uint16 = 0x009A
)

// ChatKind says where a line came from, which is what decides its color.
type ChatKind uint8

// Where a chat line came from.
const (
	// ChatOther is someone else speaking in range.
	ChatOther ChatKind = iota
	// ChatSelf is our own line, echoed back by the server.
	ChatSelf
	// ChatBroadcast is a server-wide announcement.
	ChatBroadcast
)

// ChatMessage is one line for the chat box.
type ChatMessage struct {
	Kind ChatKind

	// GID is who said it, for ChatOther. Zero otherwise.
	GID uint32

	// Speaker is the name the line was prefixed with, empty when the line
	// carries none. Server messages usually do not.
	Speaker string

	// Text is the line without the speaker prefix.
	Text string
}

// DecodeChat parses a line someone else spoke (ZC_NOTIFY_CHAT).
// Returns nil on short data.
func DecodeChat(data []byte) *ChatMessage {
	if len(data) < 8 {
		return nil
	}

	speaker, text := splitSpeaker(chatText(data, 8))

	return &ChatMessage{
		Kind:    ChatOther,
		GID:     readU32(data, 4),
		Speaker: speaker,
		Text:    text,
	}
}

// DecodePlayerChat parses our own line echoed back, or a message the server
// sent us directly (ZC_NOTIFY_PLAYERCHAT). Returns nil on short data.
func DecodePlayerChat(data []byte) *ChatMessage {
	if len(data) < 4 {
		return nil
	}

	speaker, text := splitSpeaker(chatText(data, 4))

	return &ChatMessage{Kind: ChatSelf, Speaker: speaker, Text: text}
}

// DecodeBroadcast parses a server-wide announcement (ZC_BROADCAST).
// Returns nil on short data.
func DecodeBroadcast(data []byte) *ChatMessage {
	if len(data) < 4 {
		return nil
	}

	return &ChatMessage{Kind: ChatBroadcast, Text: chatText(data, 4)}
}

// chatText reads the message body, which runs from offset to the length the
// packet declares.
//
// The declared length is trusted over the buffer: the framing layer has
// already cut the packet to it, and a message is NUL-terminated inside that
// rather than filling it.
func chatText(data []byte, offset int) string {
	end := int(readU16(data, 2))
	if end > len(data) || end < offset {
		end = len(data)
	}

	raw := data[offset:end]
	if stop := bytes.IndexByte(raw, 0); stop >= 0 {
		raw = raw[:stop]
	}

	return string(raw)
}

// splitSpeaker separates "Name : said something" into its two halves.
//
// RO puts the speaker in the line itself rather than a field of its own, so
// this is the only way to color a name differently from what it said. A line
// with no separator is all text and no speaker — which is what server
// messages look like.
func splitSpeaker(message string) (speaker, text string) {
	// The separator is a space-colon-space; a bare colon appears inside
	// ordinary sentences and inside NPC names like "Guide#01prontera".
	const sep = " : "

	idx := strings.Index(message, sep)
	if idx < 0 {
		return "", message
	}

	return message[:idx], message[idx+len(sep):]
}
