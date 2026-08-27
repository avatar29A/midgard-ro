package items

import "testing"

// TestNamesEmbedded guards against an embed that shipped empty or a parse
// that silently dropped every line, which would leave every item nameless.
func TestNamesEmbedded(t *testing.T) {
	if Count() < 20000 {
		t.Fatalf("Count() = %d, want the full table — did the generator run?", Count())
	}

	// 501 is Red Potion, which has been item 501 for the whole life of the
	// game and is as safe an anchor as this table has.
	if got := Name(501); got != "Red Potion" {
		t.Errorf("Name(501) = %q, want %q", got, "Red Potion")
	}
}

func TestNameUnknown(t *testing.T) {
	if got := Name(0); got != "" {
		t.Errorf("Name(0) = %q, want empty", got)
	}
}
