package skills

import "testing"

// TestNamesGenerated guards against a generator that ran but produced an
// empty or truncated table, which would leave every skill nameless.
func TestNamesGenerated(t *testing.T) {
	if Count() < 1000 {
		t.Fatalf("Count() = %d, want the full table — did the generator match the yml?", Count())
	}

	// Id 1 is NV_BASIC, the skill every character has.
	if got := Name(1); got != "Basic Skill" {
		t.Errorf("Name(1) = %q, want %q", got, "Basic Skill")
	}
}

// TestNameUnknown: an id newer than the tree this was generated from returns
// empty rather than a placeholder, so the caller can decide what to show.
func TestNameUnknown(t *testing.T) {
	if got := Name(65535); got != "" {
		t.Errorf("Name(65535) = %q, want empty", got)
	}
}
