package ui

import (
	"testing"

	"github.com/Faultbox/midgard-ro/internal/network/packets"
)

// TestItemMenuOffersWhatTheItemCanDo: a card cannot be used and a sword cannot
// be drunk, and a menu entry that does nothing when pressed reads as a fault.
func TestItemMenuOffersWhatTheItemCanDo(t *testing.T) {
	for _, tc := range []struct {
		name  string
		menu  itemMenu
		first string
	}{
		{"a potion is used", itemMenu{id: 501}, "Use"},
		{"a sword is worn", itemMenu{id: 1101, equip: true}, "Equip"},
		{"one already worn comes off", itemMenu{id: 1101, equip: true, worn: true}, "Unequip"},
	} {
		entries := tc.menu.entries()

		if len(entries) != 2 {
			t.Fatalf("%s: %d entries, want the action and the information",
				tc.name, len(entries))
		}
		if entries[0].label != tc.first {
			t.Errorf("%s: the first entry reads %q, want %q",
				tc.name, entries[0].label, tc.first)
		}
		if !entries[1].info {
			t.Errorf("%s: the last entry is not the information one", tc.name)
		}
	}
}

// TestAMaterialOffersOnlyInformation: a gemstone is neither used nor worn, so
// the menu says the one thing there is to say about it.
func TestAMaterialOffersOnlyInformation(t *testing.T) {
	// 716 is a Red Gemstone: Etc, and the tab it sits on is not the equipment
	// one, so nothing would happen if it offered "Use".
	entries := itemMenu{id: 716}.entries()

	if len(entries) != 1 {
		t.Fatalf("a gemstone offers %d entries: %+v", len(entries), entries)
	}
	if !entries[0].info {
		t.Errorf("the one entry is %q, want the information one", entries[0].label)
	}
}

// TestItemMenuHeightFollowsItsEntries: the box is drawn from the count, so one
// that did not would cut the last entry off or leave a gap under it.
func TestItemMenuHeightFollowsItsEntries(t *testing.T) {
	two := itemMenu{id: 501}
	one := itemMenu{id: 716}

	if len(two.entries()) <= len(one.entries()) {
		t.Fatal("the fixture items no longer differ in what they offer")
	}
	if two.height() <= one.height() {
		t.Errorf("a two-entry menu is %v tall and a one-entry menu %v",
			two.height(), one.height())
	}

	// Every entry fits inside, padding included.
	rows := float32(len(two.entries()))
	if two.height() < rows*itemMenuH {
		t.Errorf("a menu %v tall cannot hold %v rows of %v", two.height(), rows, itemMenuH)
	}
}

// TestOpeningTheMenuRemembersTheSlot: acting on the item needs the server's
// own inventory index, not the cell it happened to be drawn in.
func TestOpeningTheMenuRemembersTheSlot(t *testing.T) {
	b := &UI2DBackend{}

	b.openItemMenu(packets.InventoryItem{ID: 1101, Index: 7, Equipped: true}, true, 100, 200)

	if !b.itemMenu.open {
		t.Fatal("the menu did not open")
	}
	if b.itemMenu.id != 1101 || b.itemMenu.index != 7 {
		t.Errorf("the menu came out as %+v", b.itemMenu)
	}
	if !b.itemMenu.equip || !b.itemMenu.worn {
		t.Errorf("a worn item opened a menu that does not know it: %+v", b.itemMenu)
	}
	if b.itemMenu.x != 100 || b.itemMenu.y != 200 {
		t.Errorf("the menu opened at %v,%v rather than under the pointer",
			b.itemMenu.x, b.itemMenu.y)
	}
}
