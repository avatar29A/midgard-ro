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
	// ZC_NPC_CHAT carries a line with the colour the server picked for it.
	// A handful of commands answer this way instead of on 0x008E — @cash,
	// @points, @request and @auction, 7 call sites against 1035 for the
	// plain path. All four are gated behind server config we do not set, so
	// this is proved by test rather than by a live packet; see
	// docs/research/chat-commands.md §5.
	ZC_NPC_CHAT uint16 = 0x02C1
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

	// Color is an RGB the server chose for this line, valid only when
	// HasColor is set. Carried as a plain number rather than a colour type
	// because this layer has no business knowing how anything is drawn.
	Color uint32

	// HasColor says the server picked a colour rather than leaving it to the
	// kind. Distinguishing this from Color == 0 matters: black is a colour
	// the server can legitimately send.
	HasColor bool
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

// EncodeWhisper builds a private message to target.
//
// clif_process_message reads this with whisperFormat set: the name is a fixed
// 24-byte field that has to be zero-terminated, and the message follows it,
// also terminated. A name that fills the field with no room for the
// terminator is rejected as "an unterminated name" and the line is dropped.
//
// Returns nil when either half is missing or the name cannot fit, since none
// of those produce a packet the server will accept.
func EncodeWhisper(target, message string) []byte {
	if target == "" || message == "" || len(target) >= whisperNameLen {
		return nil
	}

	length := 4 + whisperNameLen + len(message) + 1
	pkt := make([]byte, length)

	pkt[0] = byte(CZ_WHISPER)
	pkt[1] = byte(CZ_WHISPER >> 8)
	pkt[2] = byte(length)
	pkt[3] = byte(length >> 8)
	copy(pkt[4:4+whisperNameLen], target)
	copy(pkt[4+whisperNameLen:], message)

	return pkt
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

// Broadcast colours.
//
// ZC_BROADCAST has no colour field. The server encodes one as a literal word
// at the front of the message and the client is expected to recognize it and
// cut it off (clif_broadcast). Only these two exact words, only at the very
// start; anything else is an ordinary yellow announcement.
const (
	// broadcastBlue is what BC_BLUE prepends.
	broadcastBlue = "blue"
	// broadcastWoE is what BC_WOE prepends, marking a War of Emperium line.
	broadcastWoE = "ssss"

	// BroadcastColorBlue and BroadcastColorYellow are what those two stand
	// for. WoE lines are yellow like an ordinary announcement — the marker
	// distinguishes the event, not the colour.
	BroadcastColorBlue   uint32 = 0x00FFFF
	BroadcastColorYellow uint32 = 0xFFFF00
)

// DecodeBroadcast parses a server-wide announcement (ZC_BROADCAST).
// Returns nil on short data.
//
// The colour marker is stripped here rather than left for the UI. It is not
// text anyone should see: leaving it in renders "@kami hi" from a blue
// broadcast as "bluehi".
//
// A message that genuinely begins with one of those words loses it. That is
// the protocol's own ambiguity — there is no length or flag to tell a marker
// from the first four letters of a sentence — and the original client reads
// it exactly this way.
func DecodeBroadcast(data []byte) *ChatMessage {
	if len(data) < 4 {
		return nil
	}

	text := chatText(data, 4)

	color := BroadcastColorYellow
	switch {
	case strings.HasPrefix(text, broadcastBlue):
		text = text[len(broadcastBlue):]
		color = BroadcastColorBlue
	case strings.HasPrefix(text, broadcastWoE):
		text = text[len(broadcastWoE):]
	}

	return &ChatMessage{
		Kind:     ChatBroadcast,
		Text:     text,
		Color:    color,
		HasColor: true,
	}
}

// DecodeNPCChat parses a line carrying its own colour (ZC_NPC_CHAT).
// Returns nil on short data.
//
//	02c1 <len>.W <GID>.L <color>.L <message>.?B
//
// The colour arrives as BGR. clif_messagecolor_target swaps rAthena's RGB
// before sending it, so reading it back as RGB turns the server's light green
// into pink. The swap is its own inverse, which is why the same expression
// undoes it.
func DecodeNPCChat(data []byte) *ChatMessage {
	const messageAt = 12

	if len(data) < messageAt {
		return nil
	}

	return &ChatMessage{
		Kind:     ChatSystem,
		GID:      readU32(data, 4),
		Text:     chatText(data, messageAt),
		Color:    bgrToRGB(readU32(data, 8)),
		HasColor: true,
	}
}

// bgrToRGB swaps the red and blue bytes of a 24-bit colour, discarding
// anything above them.
func bgrToRGB(v uint32) uint32 {
	return (v&0x0000FF)<<16 | (v & 0x00FF00) | (v&0xFF0000)>>16
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

// CZ_WHISPER carries a private message: name[24] then the text. Unlike public
// chat it does not want the "<name> : " prefix, because the recipient travels
// in its own fixed-width field.
const CZ_WHISPER uint16 = 0x0096

// ZC_ACK_WHISPER reports what became of a whisper we sent.
//
// It carries a result code and our own character id — it is not an echo. The
// server never sends the message back, so the client is what displays a line
// you sent, and only once this says it arrived. The id moved with the packet
// version: 0x0098 was three bytes, and from PACKETVER 20131223 it is 0x09DF
// with the character id appended.
const ZC_ACK_WHISPER uint16 = 0x09DF

// Whisper results, from rAthena's e_ack_whisper.
const (
	WhisperOK uint8 = iota
	WhisperOffline
	WhisperIgnored
	WhisperAllIgnored
)

// WhisperFailure describes a non-zero whisper result in the words the official
// client uses for it. Returns "" for WhisperOK, which is not a failure.
func WhisperFailure(result uint8, target string) string {
	switch result {
	case WhisperOffline:
		return target + " is not online."
	case WhisperIgnored:
		return target + " is ignoring you."
	case WhisperAllIgnored:
		return target + " is ignoring everyone."
	default:
		return ""
	}
}

// DecodeWhisperAck reads the result code. Reports false when the packet is too
// short to hold one.
func DecodeWhisperAck(data []byte) (uint8, bool) {
	if len(data) < 3 {
		return 0, false
	}

	return data[2], true
}

// whisperNameLen is NAME_LENGTH from the server. The name field is exactly
// this wide and must hold a terminator, so 23 usable characters.
const whisperNameLen = 24

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
