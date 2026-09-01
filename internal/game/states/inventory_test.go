package states

import (
	"testing"

	"github.com/Faultbox/midgard-ro/internal/network/packets"
)

// TestInventoryListDeliveredTwiceDoesNotDouble is the guard for the bug this
// was written for: the server sends the inventory again on a map change, and
// appending gave a second row for every item each time you walked through a
// warp.
func TestInventoryListDeliveredTwiceDoesNotDouble(t *testing.T) {
	list := []packets.InventoryItem{
		{Index: 2, ID: 501, Count: 7},
		{Index: 3, ID: 909, Count: 10},
	}

	var s InGameState
	s.mergeInventory(list)
	s.mergeInventory(list)

	if len(s.inventory) != 2 {
		t.Fatalf("holding %d rows after two deliveries of two items, want 2", len(s.inventory))
	}
}

// TestInventoryMergeTakesTheNewerCount: a repeat delivery is the server's
// current word on a slot, so it replaces rather than being ignored.
func TestInventoryMergeTakesTheNewerCount(t *testing.T) {
	var s InGameState
	s.mergeInventory([]packets.InventoryItem{{Index: 2, ID: 501, Count: 7}})
	s.mergeInventory([]packets.InventoryItem{{Index: 2, ID: 501, Count: 3}})

	if len(s.inventory) != 1 {
		t.Fatalf("holding %d rows, want 1", len(s.inventory))
	}
	if s.inventory[0].Count != 3 {
		t.Errorf("Count = %d, want the 3 the newer delivery gave", s.inventory[0].Count)
	}
}

// TestInventoryMergeKeepsWhatTheListDoesNotMention: the two lists arrive
// separately and each covers half the bag, so dropping what is missing from
// one would empty the other.
func TestInventoryMergeKeepsWhatTheListDoesNotMention(t *testing.T) {
	var s InGameState
	s.mergeInventory([]packets.InventoryItem{{Index: 2, ID: 501, Count: 7}})
	s.mergeInventory([]packets.InventoryItem{{Index: 9, ID: 1201, Count: 1, Equipped: true}})

	if len(s.inventory) != 2 {
		t.Fatalf("holding %d rows, want both halves of the bag", len(s.inventory))
	}
}

// TestInventoryMergeReportsWhatItDid: the counts go into the trace, which is
// how a repeat delivery is told from a first one in a log.
func TestInventoryMergeReportsWhatItDid(t *testing.T) {
	var s InGameState

	added, replaced := s.mergeInventory([]packets.InventoryItem{{Index: 2, ID: 501, Count: 7}})
	if added != 1 || replaced != 0 {
		t.Errorf("first delivery reported added=%d replaced=%d, want 1 and 0", added, replaced)
	}

	added, replaced = s.mergeInventory([]packets.InventoryItem{{Index: 2, ID: 501, Count: 7}})
	if added != 0 || replaced != 1 {
		t.Errorf("repeat delivery reported added=%d replaced=%d, want 0 and 1", added, replaced)
	}
}
