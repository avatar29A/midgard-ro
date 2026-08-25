package ui2d

import "testing"

// The renderer needs a GL context, so widgets cannot be driven in a test. This
// covers the dispatch a widget relies on; Button's single call site sits next
// to where it sets clicked.
func TestClickSound(t *testing.T) {
	var c Context

	// A context without a sound is the default, and must stay quiet rather
	// than panic.
	c.playClickSound()

	calls := 0
	c.SetClickSound(func() { calls++ })

	c.playClickSound()
	c.playClickSound()

	if calls != 2 {
		t.Errorf("click sound played %d times, want 2", calls)
	}

	// Passing nil turns it back off.
	c.SetClickSound(nil)
	c.playClickSound()

	if calls != 2 {
		t.Errorf("click sound played %d times after being cleared, want 2", calls)
	}
}
