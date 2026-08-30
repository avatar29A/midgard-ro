package packets

import (
	"encoding/binary"
	"strings"
	"testing"
)

// chatPacket builds a variable-length chat packet the way the server does:
// header, declared length, optional GID, then a NUL-terminated message.
func chatPacket(id uint16, gid uint32, hasGID bool, message string) []byte {
	body := append([]byte(message), 0)

	head := 4
	if hasGID {
		head = 8
	}

	pkt := make([]byte, head+len(body))
	pkt[0] = byte(id)
	pkt[1] = byte(id >> 8)
	length := uint16(len(pkt))
	pkt[2] = byte(length)
	pkt[3] = byte(length >> 8)

	if hasGID {
		pkt[4] = byte(gid)
		pkt[5] = byte(gid >> 8)
		pkt[6] = byte(gid >> 16)
		pkt[7] = byte(gid >> 24)
	}

	copy(pkt[head:], body)

	return pkt
}

func TestDecodeChat(t *testing.T) {
	pkt := chatPacket(ZC_NOTIFY_CHAT, 110000123, true, "Someone : hello there")

	msg := DecodeChat(pkt)
	if msg == nil {
		t.Fatal("DecodeChat returned nil for a well-formed packet")
	}
	if msg.Kind != ChatOther {
		t.Errorf("Kind = %d, want ChatOther", msg.Kind)
	}
	if msg.GID != 110000123 {
		t.Errorf("GID = %d, want 110000123", msg.GID)
	}
	if msg.Speaker != "Someone" {
		t.Errorf("Speaker = %q, want %q", msg.Speaker, "Someone")
	}
	if msg.Text != "hello there" {
		t.Errorf("Text = %q, want %q", msg.Text, "hello there")
	}
}

func TestDecodePlayerChat(t *testing.T) {
	pkt := chatPacket(ZC_NOTIFY_PLAYERCHAT, 0, false, "MidgardTest : testing")

	msg := DecodePlayerChat(pkt)
	if msg == nil {
		t.Fatal("DecodePlayerChat returned nil for a well-formed packet")
	}
	if msg.Kind != ChatSelf {
		t.Errorf("Kind = %d, want ChatSelf", msg.Kind)
	}
	if msg.Speaker != "MidgardTest" || msg.Text != "testing" {
		t.Errorf("split = %q / %q, want MidgardTest / testing", msg.Speaker, msg.Text)
	}
}

// TestServerMessagesHaveNoSpeaker: rAthena's welcome lines arrive through the
// same packet as our own chat, but carry no "Name : " prefix. Inventing a
// speaker for them would put half the sentence in the name color.
func TestServerMessagesHaveNoSpeaker(t *testing.T) {
	pkt := chatPacket(ZC_NOTIFY_PLAYERCHAT, 0, false, "Welcome to the Midgard server!")

	msg := DecodePlayerChat(pkt)
	if msg == nil {
		t.Fatal("nil for a server message")
	}
	if msg.Speaker != "" {
		t.Errorf("Speaker = %q, want empty for a line with no prefix", msg.Speaker)
	}
	if msg.Text != "Welcome to the Midgard server!" {
		t.Errorf("Text = %q", msg.Text)
	}
}

// TestSpeakerSplitNeedsTheFullSeparator guards against splitting on any colon.
// NPC names carry them ("Guide#01prontera"), and so do ordinary sentences.
func TestSpeakerSplitNeedsTheFullSeparator(t *testing.T) {
	tests := []struct {
		message     string
		wantSpeaker string
		wantText    string
	}{
		{"Kafra : Welcome", "Kafra", "Welcome"},
		{"note: no space before the colon", "", "note: no space before the colon"},
		{"12:30 is the time", "", "12:30 is the time"},
		{"", "", ""},
		{"Someone : it is 12:30 : really", "Someone", "it is 12:30 : really"},
	}

	for _, tt := range tests {
		speaker, text := splitSpeaker(tt.message)
		if speaker != tt.wantSpeaker || text != tt.wantText {
			t.Errorf("splitSpeaker(%q) = %q / %q, want %q / %q",
				tt.message, speaker, text, tt.wantSpeaker, tt.wantText)
		}
	}
}

// TestChatTextTrustsTheDeclaredLength: the message is NUL-terminated inside a
// packet that may be longer, and reading to the end of the buffer would append
// whatever padding followed.
func TestChatTextTrustsTheDeclaredLength(t *testing.T) {
	pkt := chatPacket(ZC_NOTIFY_PLAYERCHAT, 0, false, "hello")
	pkt = append(pkt, 'X', 'X', 'X') // trailing bytes past the declared length

	msg := DecodePlayerChat(pkt)
	if msg == nil {
		t.Fatal("nil")
	}
	if msg.Text != "hello" {
		t.Errorf("Text = %q, want %q — the declared length was not respected", msg.Text, "hello")
	}
}

func TestDecodeChatShortData(t *testing.T) {
	if DecodeChat(make([]byte, 7)) != nil {
		t.Error("decoded a chat packet one byte short of its header")
	}
	if DecodePlayerChat(make([]byte, 3)) != nil {
		t.Error("decoded a player chat packet one byte short of its header")
	}
	if DecodeBroadcast(make([]byte, 3)) != nil {
		t.Error("decoded a broadcast one byte short of its header")
	}
}

func TestDecodeBroadcast(t *testing.T) {
	pkt := chatPacket(ZC_BROADCAST, 0, false, "Server restarting")

	msg := DecodeBroadcast(pkt)
	if msg == nil {
		t.Fatal("nil for a well-formed broadcast")
	}
	if msg.Kind != ChatBroadcast || msg.Text != "Server restarting" {
		t.Errorf("broadcast = kind %d %q", msg.Kind, msg.Text)
	}
}

// TestEncodeChatCarriesTheNamePrefix is a disconnect guard, not a formatting
// one. rAthena's clif_process_message compares the prefix against the
// character's own name and forces a relog when it does not match, so a bare
// message is not a cosmetic mistake — it drops the connection.
func TestEncodeChatCarriesTheNamePrefix(t *testing.T) {
	pkt := EncodeChat("MidgardTest", "hello there")
	if pkt == nil {
		t.Fatal("EncodeChat returned nothing for a well-formed line")
	}

	if id := uint16(pkt[0]) | uint16(pkt[1])<<8; id != CZ_REQUEST_CHAT {
		t.Errorf("packet id = 0x%04X, want 0x%04X", id, CZ_REQUEST_CHAT)
	}

	length := int(uint16(pkt[2]) | uint16(pkt[3])<<8)
	if length != len(pkt) {
		t.Errorf("declared length %d, actual %d", length, len(pkt))
	}

	body := string(pkt[4 : len(pkt)-1])
	if want := "MidgardTest : hello there"; body != want {
		t.Errorf("body = %q, want %q", body, want)
	}
	if pkt[len(pkt)-1] != 0 {
		t.Error("the line is not terminated; the server reads to the terminator")
	}
}

// TestEncodeChatRoundTripsThroughTheDecoder: what we send is what the server
// echoes back, so our own encoder and decoder have to agree on the separator.
func TestEncodeChatRoundTrips(t *testing.T) {
	pkt := EncodeChat("MidgardTest", "hello there")

	// The echo comes back as ZC_NOTIFY_PLAYERCHAT, same body layout.
	echo := make([]byte, len(pkt))
	copy(echo, pkt)
	echo[0] = byte(ZC_NOTIFY_PLAYERCHAT)
	echo[1] = byte(ZC_NOTIFY_PLAYERCHAT >> 8)

	msg := DecodePlayerChat(echo)
	if msg == nil {
		t.Fatal("our own packet did not decode")
	}
	if msg.Speaker != "MidgardTest" || msg.Text != "hello there" {
		t.Errorf("round trip gave %q / %q", msg.Speaker, msg.Text)
	}
}

// TestEncodeChatRefusesWhatCannotBeSent: without a name there is no valid
// prefix, and an empty line has nothing to say — building either would only
// produce a packet the server rejects.
func TestEncodeChatRefusesWhatCannotBeSent(t *testing.T) {
	if EncodeChat("", "hello") != nil {
		t.Error("encoded a line with no speaker name; the server would force a relog")
	}
	if EncodeChat("MidgardTest", "") != nil {
		t.Error("encoded an empty line")
	}
}

func TestEncodeWhisperLayout(t *testing.T) {
	pkt := EncodeWhisper("Kafra", "hi")

	if pkt == nil {
		t.Fatal("EncodeWhisper returned nil for a valid whisper")
	}

	// id, length, name[24], message, terminator.
	want := 4 + 24 + len("hi") + 1
	if len(pkt) != want {
		t.Fatalf("len = %d, want %d", len(pkt), want)
	}
	if got := binary.LittleEndian.Uint16(pkt[0:2]); got != CZ_WHISPER {
		t.Errorf("id = 0x%04X, want 0x%04X", got, CZ_WHISPER)
	}
	if got := int(binary.LittleEndian.Uint16(pkt[2:4])); got != want {
		t.Errorf("declared length = %d, want %d", got, want)
	}

	// The server rejects an unterminated name, so the field must be padded.
	name := pkt[4:28]
	if string(name[:5]) != "Kafra" {
		t.Errorf("name = %q, want it to start with Kafra", name)
	}
	if name[5] != 0 {
		t.Error("the name field must be zero-terminated")
	}
	if string(pkt[28:30]) != "hi" {
		t.Errorf("message = %q, want hi", pkt[28:30])
	}
	if pkt[len(pkt)-1] != 0 {
		t.Error("the message must be zero-terminated")
	}
}

// TestEncodeWhisperRejectsOverlongName guards the terminator: a name that
// fills the field leaves no room for one, and the server drops the line.
func TestEncodeWhisperRejectsOverlongName(t *testing.T) {
	long := strings.Repeat("a", 24)

	if pkt := EncodeWhisper(long, "hi"); pkt != nil {
		t.Error("a 24-character name cannot be terminated and must be refused")
	}
}

// TestDecodeWhisperAck covers the packet that says whether a whisper landed.
// Its length is what the framing table already declares for 0x09DF: the id,
// the result, and our own character id.
func TestDecodeWhisperAck(t *testing.T) {
	if n, ok := Length(ZC_ACK_WHISPER); !ok || n != 7 {
		t.Fatalf("length table says %d, %v; want 7, true", n, ok)
	}

	pkt := []byte{0xDF, 0x09, WhisperOffline, 1, 0, 0, 0}

	got, ok := DecodeWhisperAck(pkt)
	if !ok {
		t.Fatal("a full packet must decode")
	}
	if got != WhisperOffline {
		t.Errorf("result = %d, want %d", got, WhisperOffline)
	}
}

func TestDecodeWhisperAckShort(t *testing.T) {
	if _, ok := DecodeWhisperAck([]byte{0xDF, 0x09}); ok {
		t.Error("a packet with no result byte must not decode")
	}
}

// TestWhisperFailureOnlyForFailures: a successful whisper is displayed as the
// line you sent, so treating result 0 as an error message would replace it.
func TestWhisperFailureOnlyForFailures(t *testing.T) {
	if got := WhisperFailure(WhisperOK, "Kafra"); got != "" {
		t.Errorf("WhisperFailure(ok) = %q, want empty", got)
	}
	if got := WhisperFailure(WhisperOffline, "Kafra"); got == "" {
		t.Error("an offline target must produce a message")
	}
	if got := WhisperFailure(200, "Kafra"); got != "" {
		t.Errorf("WhisperFailure(unknown) = %q, want empty", got)
	}
}

// broadcastPacket builds a ZC_BROADCAST carrying exactly text.
func broadcastPacket(text string) []byte {
	pkt := make([]byte, 4+len(text))
	binary.LittleEndian.PutUint16(pkt[0:2], ZC_BROADCAST)
	binary.LittleEndian.PutUint16(pkt[2:4], uint16(len(pkt)))
	copy(pkt[4:], text)

	return pkt
}

// TestDecodeBroadcastStripsTheColorMarker: ZC_BROADCAST has no color field.
// The server puts a literal word at the front of the message and expects the
// client to cut it off. Leaving it in renders a blue "hi" as "bluehi".
func TestDecodeBroadcastStripsTheColorMarker(t *testing.T) {
	tests := []struct {
		name      string
		wire      string
		wantText  string
		wantColor uint32
	}{
		{"blue marker", "bluehello", "hello", BroadcastColorBlue},
		{"WoE marker", "sssshello", "hello", BroadcastColorYellow},
		{"no marker", "hello", "hello", BroadcastColorYellow},

		// The marker is only a marker at the very start.
		{"marker word later in the line", "the sky is blue", "the sky is blue", BroadcastColorYellow},

		// The protocol cannot tell a marker from the first four letters of a
		// sentence, and neither can the original client. Pinned so the
		// behavior is a decision rather than a surprise.
		{"message that begins with the marker word", "blueberries", "berries", BroadcastColorBlue},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := DecodeBroadcast(broadcastPacket(tt.wire))
			if msg == nil {
				t.Fatal("DecodeBroadcast returned nil")
			}
			if msg.Text != tt.wantText {
				t.Errorf("text = %q, want %q", msg.Text, tt.wantText)
			}
			if !msg.HasColor {
				t.Fatal("HasColor is false; a broadcast always resolves to a color")
			}
			if msg.Color != tt.wantColor {
				t.Errorf("color = 0x%06X, want 0x%06X", msg.Color, tt.wantColor)
			}
			if msg.Kind != ChatBroadcast {
				t.Errorf("kind = %d, want ChatBroadcast", msg.Kind)
			}
		})
	}
}

// TestDecodeNPCChatSwapsBGR: clif_messagecolor_target swaps rAthena's RGB to
// BGR before sending. Reading it back as RGB turns the server's light green
// into pink, which is the whole reason this decoder exists.
func TestDecodeNPCChatSwapsBGR(t *testing.T) {
	const text = "Gained 100 cash points. Total 100 points."

	// An asymmetric color on purpose. rAthena's light green 0xB5FFB5 is the
	// realistic value but its red and blue bytes are equal, so a decoder that
	// forgot to swap would pass with it and fail in the field.
	const wantRGB uint32 = 0xFF8800
	const onWire uint32 = 0x0088FF

	pkt := make([]byte, 12+len(text))
	binary.LittleEndian.PutUint16(pkt[0:2], ZC_NPC_CHAT)
	binary.LittleEndian.PutUint16(pkt[2:4], uint16(len(pkt)))
	binary.LittleEndian.PutUint32(pkt[4:8], 2000000)
	binary.LittleEndian.PutUint32(pkt[8:12], onWire)
	copy(pkt[12:], text)

	msg := DecodeNPCChat(pkt)
	if msg == nil {
		t.Fatal("DecodeNPCChat returned nil")
	}
	if msg.Text != text {
		t.Errorf("text = %q, want %q", msg.Text, text)
	}
	if msg.Color != wantRGB {
		t.Errorf("color = 0x%06X, want 0x%06X", msg.Color, wantRGB)
	}
	if !msg.HasColor {
		t.Error("HasColor is false; the packet exists to carry one")
	}
	if msg.GID != 2000000 {
		t.Errorf("GID = %d, want 2000000", msg.GID)
	}
}

// TestBGRSwapIsItsOwnInverse: the server applies the same expression we do, so
// applying it twice must give back what went in. An asymmetric color is the
// only kind that proves it.
func TestBGRSwapIsItsOwnInverse(t *testing.T) {
	const rgb uint32 = 0x123456

	if got := bgrToRGB(rgb); got != 0x563412 {
		t.Errorf("bgrToRGB(0x%06X) = 0x%06X, want 0x563412", rgb, got)
	}
	if got := bgrToRGB(bgrToRGB(rgb)); got != rgb {
		t.Errorf("swapping twice gave 0x%06X, want 0x%06X", got, rgb)
	}
}

// TestDecodeNPCChatRefusesShortData: the header alone is 12 bytes.
func TestDecodeNPCChatRefusesShortData(t *testing.T) {
	if DecodeNPCChat(make([]byte, 11)) != nil {
		t.Error("decoded a packet too short to hold the header")
	}
	if DecodeNPCChat(nil) != nil {
		t.Error("decoded nil")
	}
}

// TestNPCChatIsFramed: 0x02C1 is variable-length, and the framing layer needs
// to know that or the stream desynchronizes the first time one arrives.
func TestNPCChatIsFramed(t *testing.T) {
	size, ok := Length(ZC_NPC_CHAT)
	if !ok {
		t.Fatal("0x02C1 has no length: the framing layer will lose the stream")
	}
	if size != -1 {
		t.Errorf("0x02C1 length = %d, want -1 (variable)", size)
	}
}
