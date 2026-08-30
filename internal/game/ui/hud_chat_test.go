package ui

import (
	"testing"

	"github.com/Faultbox/midgard-ro/internal/engine/ui2d"
	"github.com/Faultbox/midgard-ro/internal/game/states"
)

// TestChatKindColor pins the palette to the original's, which is the point of
// step 5 of #94: server replies were yellow and whispers purple, and both are
// wrong. The values are roBrowser's transcription of the original client,
// confirmed against rAthena's own colour table.
func TestChatKindColor(t *testing.T) {
	tests := []struct {
		name string
		kind states.ChatKind
		want uint32
	}{
		// The correction that matters. Every reply to an @ command arrives as
		// ChatSystem, and the original paints it green.
		{"a reply to an @ command", states.ChatSystem, 0x00FF00},
		{"our own words", states.ChatSelf, 0x00FF00},

		{"someone else speaking", states.ChatOther, 0xFFFFFF},
		{"a whisper", states.ChatWhisper, 0xFFFF00},
		{"a command that could not run", states.ChatError, 0xFF0000},
		{"the client answering for itself", states.ChatNotice, 0xFFFF63},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := packRGB(chatKindColor(tt.kind)); got != tt.want {
				t.Errorf("color = 0x%06X, want 0x%06X", got, tt.want)
			}
		})
	}
}

// TestWhisperAndNoticeAreDistinguishable: both are yellow, and if they were
// the same yellow a private message would be indistinguishable from the
// client talking to itself.
func TestWhisperAndNoticeAreDistinguishable(t *testing.T) {
	if packRGB(chatColorWhisper) == packRGB(chatColorNotice) {
		t.Error("whispers and notices are the same colour")
	}
}

// TestServerColorWins: ZC_NPC_CHAT and ZC_BROADCAST carry a colour the server
// chose. Overriding it with the kind's colour would discard the only thing
// those packets say that the others do not.
func TestServerColorWins(t *testing.T) {
	line := states.ChatLine{
		Kind:     states.ChatSystem,
		Text:     "Gained 100 cash points. Total 100 points.",
		Color:    0xB5FFB5,
		HasColor: true,
	}

	if got := packRGB(chatLineColor(line)); got != 0xB5FFB5 {
		t.Errorf("color = 0x%06X, want the server's 0xB5FFB5", got)
	}

	// Without the flag the kind decides, so a zero Color cannot be mistaken
	// for the server having chosen black.
	line.HasColor = false
	if got := packRGB(chatLineColor(line)); got != 0x00FF00 {
		t.Errorf("color = 0x%06X, want the kind's green 0x00FF00", got)
	}
}

// TestRGBColorRoundTrip: the conversion the chat box uses to honour a
// server-chosen colour must not lose the value on the way in.
func TestRGBColorRoundTrip(t *testing.T) {
	for _, rgb := range []uint32{0x000000, 0xFFFFFF, 0xFF8800, 0x123456, 0xB5FFB5} {
		if got := packRGB(rgbColor(rgb)); got != rgb {
			t.Errorf("round trip of 0x%06X gave 0x%06X", rgb, got)
		}
	}
}

// packRGB turns a drawable color back into 0xRRGGBB so tests can state their
// expectations the way the reference does — as hex, not as thirds.
func packRGB(c ui2d.Color) uint32 {
	return uint32(c.R*255+0.5)<<16 | uint32(c.G*255+0.5)<<8 | uint32(c.B*255+0.5)
}
