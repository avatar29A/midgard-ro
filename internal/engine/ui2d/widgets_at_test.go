package ui2d

import "testing"

// TestTrimLastRune is the guard for backspacing multi-byte text: names and
// messages arrive as UTF-8, and dropping a byte would leave a broken sequence
// that the font renders as a replacement glyph.
func TestTrimLastRune(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{"ascii", "abc", "ab"},
		{"multibyte", "прив", "при"},
		{"last multibyte rune", "п", ""},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := trimLastRune(tt.value); got != tt.want {
				t.Errorf("trimLastRune(%q) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}

// TestMaskRunesCountsRunes checks the password mask hides the character count
// of the value, not its byte count — a multi-byte password would otherwise
// show more stars than it has characters.
func TestMaskRunesCountsRunes(t *testing.T) {
	if got := maskRunes("прив"); got != "****" {
		t.Errorf("maskRunes = %q, want four stars", got)
	}
}
