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
	// ZC_WHISPER is a private message. The id moves with the packet version;
	// 0x09DE is ours, not the 0x0097 older clients use.
	ZC_WHISPER uint16 = 0x09DE
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
	// ChatSystem is the server talking to us rather than a person — the
	// welcome lines, and anything else with no speaker in front of it.
	ChatSystem
	// ChatWhisper is a private message.
	ChatWhisper
	// ChatDamage is a battle line. Nothing produces one yet — the combat
	// packets are Track F — but the chat box already knows how to color it.
	ChatDamage
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

	// A line with no speaker came from the server, not from us. The two
	// arrive through the same packet and only the prefix tells them apart,
	// which is why the split happens before the kind is decided.
	kind := ChatSelf
	if speaker == "" {
		kind = ChatSystem
	}

	return &ChatMessage{Kind: kind, Speaker: speaker, Text: text}
}

// DecodeWhisper parses a private message (ZC_WHISPER). Returns nil on short
// data.
//
// Unlike public chat the sender is a field of its own, so nothing has to be
// split out of the text.
func DecodeWhisper(data []byte) *ChatMessage {
	const senderAt, messageAt = 8, 33

	if len(data) < messageAt {
		return nil
	}

	return &ChatMessage{
		Kind:    ChatWhisper,
		GID:     readU32(data, 4),
		Speaker: trimName(data[senderAt : senderAt+nameLength]),
		Text:    chatText(data, messageAt),
	}
}

// trimName reads a fixed-width name field, which the server pads with NULs.
func trimName(raw []byte) string {
	if end := bytes.IndexByte(raw, 0); end >= 0 {
		raw = raw[:end]
	}

	return string(raw)
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

// CZ_REQUEST_CHAT sends a line of public chat.
//
// The id is shuffled per packet version. At PACKETVER 20211103 it is 0x00F3:
// rAthena declares clif_parse_GlobalMessage several times behind version
// guards, and this is the last one whose guard we satisfy — the shuffle block
// our client already speaks (the one with 0x035F WalkToXY) does not redefine
// it.
const CZ_REQUEST_CHAT uint16 = 0x00F3

// ChatSeparator is what sits between a speaker and what they said, on the wire
// in both directions.
const ChatSeparator = " : "

// EncodeChat builds a public chat packet.
//
// The message must be "<the speaker's own name> : <text>", and this is not a
// display convention — the server verifies it. clif_process_message compares
// the prefix against the character's name and, on a mismatch, logs "sent a
// message using an incorrect name!" and forces a relog. Sending a bare message
// disconnects you.
//
// Returns nil when there is no name or nothing to say, since neither can
// produce a packet the server will accept.
func EncodeChat(charName, message string) []byte {
	if charName == "" || message == "" {
		return nil
	}

	body := charName + ChatSeparator + message

	// Header, the line, and the terminator the server reads to.
	length := 4 + len(body) + 1
	pkt := make([]byte, length)

	pkt[0] = byte(CZ_REQUEST_CHAT)
	pkt[1] = byte(CZ_REQUEST_CHAT >> 8)
	pkt[2] = byte(length)
	pkt[3] = byte(length >> 8)
	copy(pkt[4:], body)

	return pkt
}
