package ui

import "testing"

// TestDeadMenuOffersTheThreeWaysOut: a character at nought hit points cannot
// move, and these are the only things left to do. One missing is a corpse with
// nothing to press.
func TestDeadMenuOffersTheThreeWaysOut(t *testing.T) {
	want := map[DeadAction]string{
		DeadRespawn:    "Return to last save point",
		DeadCharSelect: "Character Select",
		DeadQuit:       "Exit to Windows",
	}

	seen := map[DeadAction]bool{}
	ids := map[string]bool{}

	for _, item := range deadMenuItems {
		if item.action == DeadNone {
			t.Errorf("%q does nothing", item.label)
		}
		if seen[item.action] {
			t.Errorf("%q repeats an action", item.label)
		}
		seen[item.action] = true

		if label, offered := want[item.action]; !offered {
			t.Errorf("%q is not one of the three", item.label)
		} else if item.label != label {
			t.Errorf("the button reads %q, want %q", item.label, label)
		}

		if ids[item.id] {
			t.Errorf("two buttons share the id %q", item.id)
		}
		ids[item.id] = true
	}

	for action, label := range want {
		if !seen[action] {
			t.Errorf("%q is not offered", label)
		}
	}
}

// TestDeadMenuFitsItsButtons: the height is worked out from the buttons rather
// than fixed, and a window shorter than its contents draws them outside itself.
func TestDeadMenuFitsItsButtons(t *testing.T) {
	buttons := float32(len(deadMenuItems)) * (escBtnH + escBtnG)

	if deadMenuH < buttons {
		t.Errorf("the window is %v tall for %v of buttons", deadMenuH, buttons)
	}
	if deadMenuW <= escBtnW {
		t.Errorf("the window is %v wide for %v buttons", deadMenuW, escBtnW)
	}
}

// TestTakeDeadActionClears: the pick is read once and gone, the same way the
// ESC menu's is. Left set, the request goes out again every frame.
func TestTakeDeadActionClears(t *testing.T) {
	b := &UI2DBackend{}

	if got := b.TakeDeadAction(); got != DeadNone {
		t.Errorf("a backend nobody pressed anything on returned %v", got)
	}

	b.deadAction = DeadRespawn

	if got := b.TakeDeadAction(); got != DeadRespawn {
		t.Errorf("TakeDeadAction = %v, want DeadRespawn", got)
	}
	if got := b.TakeDeadAction(); got != DeadNone {
		t.Errorf("the pick was still there on the second read: %v", got)
	}
}
