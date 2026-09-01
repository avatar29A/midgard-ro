package ui

import "testing"

// TestOnlyTheImplementedButtonsOpenWindows: the strip has eleven buttons and
// only five lead anywhere yet. A button with no window has to stay quiet — a
// click that makes a noise and changes nothing reads as a fault.
func TestOnlyTheImplementedButtonsOpenWindows(t *testing.T) {
	opens := map[string]bool{
		"info": true, "equip": true, "skill": true, "item": true, "map": true,
		"party": false, "guild": false, "quest": false, "option": false,
		"booking": false, "recruit": false,
	}

	// Info leads to the equipment window rather than to one of its own: the
	// modern client folds the status block into it, so both buttons go to the
	// same place.
	named := map[string]bool{"info": false}

	for _, name := range hudMenuButtons {
		want, listed := opens[name]
		if !listed {
			t.Errorf("button %q is drawn but this test does not say whether it "+
				"opens a window; add it either way", name)

			continue
		}

		window, got := opensWindow(name)
		if got != want {
			t.Errorf("opensWindow(%q) = %v, want %v", name, got, want)
		}
		if shares, listed := named[name]; got && (!listed || shares) && string(window) != name {
			t.Errorf("button %q opens window %q; the two are meant to share a name", name, window)
		}
	}
}

func TestToggleWindowOpensAndCloses(t *testing.T) {
	b := &UI2DBackend{}

	if b.IsWindowOpen(WindowEquip) {
		t.Error("a window is open before anything opened it")
	}
	if b.OpenWindowCount() != 0 {
		t.Errorf("OpenWindowCount = %d before anything opened", b.OpenWindowCount())
	}

	if open := b.ToggleWindow(WindowEquip); !open {
		t.Error("the first toggle should open the window")
	}
	if !b.IsWindowOpen(WindowEquip) {
		t.Error("the window did not stay open")
	}

	if open := b.ToggleWindow(WindowEquip); open {
		t.Error("the second toggle should close the window")
	}
	if b.IsWindowOpen(WindowEquip) {
		t.Error("the window did not close")
	}
	if b.OpenWindowCount() != 0 {
		t.Errorf("OpenWindowCount = %d after closing the only window", b.OpenWindowCount())
	}
}

// TestWindowsToggleIndependently: opening one window must not disturb another,
// since the original lets several sit on screen at once.
func TestWindowsToggleIndependently(t *testing.T) {
	b := &UI2DBackend{}

	for _, w := range []HUDWindow{WindowEquip, WindowSkill, WindowItem, WindowMap} {
		b.ToggleWindow(w)
	}
	if got := b.OpenWindowCount(); got != 4 {
		t.Fatalf("OpenWindowCount = %d, want all 4 open", got)
	}

	b.ToggleWindow(WindowItem)
	if b.IsWindowOpen(WindowItem) {
		t.Error("Item stayed open")
	}
	for _, w := range []HUDWindow{WindowEquip, WindowSkill, WindowMap} {
		if !b.IsWindowOpen(w) {
			t.Errorf("closing Item also closed %s", w)
		}
	}
	if got := b.OpenWindowCount(); got != 3 {
		t.Errorf("OpenWindowCount = %d, want 3", got)
	}
}

// TestEveryWindowHasATitle guards the table the windows draw from: a window
// with no title would open with a blank frame.
func TestEveryWindowHasATitle(t *testing.T) {
	for _, w := range []HUDWindow{WindowEquip, WindowSkill, WindowItem, WindowMap} {
		if hudWindowTitles[w] == "" {
			t.Errorf("window %q has no title", w)
		}
	}
}

// TestDescribeHUDListsInStripOrder: the overlay is read off screenshots, so
// the same set of open windows has to read the same every time. Map order
// would not.
func TestDescribeHUDListsInStripOrder(t *testing.T) {
	b := &UI2DBackend{}

	if got := b.describeHUD(); got != "no windows open" {
		t.Errorf("describeHUD with nothing open = %q", got)
	}

	// Opened back to front; the description should still read front to back.
	b.ToggleWindow(WindowMap)
	b.ToggleWindow(WindowEquip)
	b.ToggleWindow(WindowSkill)

	// Named once, though two buttons lead to it.
	if got, want := b.describeHUD(), "Equip, Skill, Map"; got != want {
		t.Errorf("describeHUD = %q, want %q", got, want)
	}
}
