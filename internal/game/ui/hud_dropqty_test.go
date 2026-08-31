package ui

import "testing"

// TestDigitsOnly: the amount field takes a number and nothing else.
//
// Filtered as it is typed rather than validated on OK — there is nothing
// sensible to do with "12a" afterwards, and a field that will not accept the
// letter says so more clearly than a message would.
func TestDigitsOnly(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"12", "12"},
		{"1a2", "12"},
		{"abc", ""},
		{"-5", "5"},
		{"1234567", "123456"}, // capped at six digits
		{"", ""},
	}

	for _, tt := range tests {
		if got := digitsOnly(tt.in); got != tt.want {
			t.Errorf("digitsOnly(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
