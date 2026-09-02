package states

import (
	"strings"
	"testing"
)

// TestValidateName pins the rules our server actually applies, from
// char_athena.conf: at least 4 characters, at most 23, and letters, digits or
// spaces only.
//
// Checked locally so a name the server would certainly refuse costs no packet
// and gets a specific reason rather than the server's single "denied". What is
// deliberately not checked is whether the name is free — nothing asks that,
// and the only way to find out is to be refused.
func TestValidateName(t *testing.T) {
	tests := []struct {
		name string
		in   string
		ok   bool
	}{
		{"a plain name", "Midgard", true},
		{"the shortest allowed", "Abcd", true},
		{"the longest allowed", strings.Repeat("a", 23), true},
		{"digits are allowed", "Novice2026", true},
		{"a space is allowed", "Two Words", true},

		{"empty", "", false},
		{"one character", "a", false},
		{"one short of the minimum", "abc", false},
		{"one past the maximum", strings.Repeat("a", 24), false},
		{"an exclamation mark", "Hello!", false},
		{"an underscore", "snake_case", false},
		{"a hyphen", "well-known", false},
		{"a full stop", "Mr.Novice", false},
		{"non-latin letters", "캐릭터", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reason := ValidateName(tt.in)

			if tt.ok && reason != "" {
				t.Errorf("ValidateName(%q) refused: %s", tt.in, reason)
			}
			if !tt.ok && reason == "" {
				t.Errorf("ValidateName(%q) accepted, want refused", tt.in)
			}
		})
	}
}

// TestValidateNameSaysWhy: a refusal that does not say what is wrong leaves
// the player guessing between three different rules.
func TestValidateNameSaysWhy(t *testing.T) {
	if got := ValidateName("ab"); !strings.Contains(got, "4") {
		t.Errorf("too-short reason = %q, want it to name the minimum", got)
	}
	if got := ValidateName(strings.Repeat("a", 30)); !strings.Contains(got, "23") {
		t.Errorf("too-long reason = %q, want it to name the maximum", got)
	}
	if got := ValidateName("bad!"); !strings.Contains(got, "letters") {
		t.Errorf("bad-character reason = %q, want it to name what is allowed", got)
	}
}
