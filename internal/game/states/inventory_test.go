package states

import (
	"encoding/binary"
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

// TestASoldItemLeavesTheBag: the server says what has gone with its own
// packet, and a client that does not listen for it goes on drawing what the
// character no longer has. Selling three of a stack of five leaves two.
func TestASoldItemLeavesTheBag(t *testing.T) {
	s := &InGameState{inventory: []packets.InventoryItem{
		{Index: 3, ID: 507, Count: 5},
		{Index: 4, ID: 501, Count: 1},
	}}

	if err := s.handleItemDeleted(itemDeletedPacket(3, 3, packets.ItemDeletedSold)); err != nil {
		t.Fatalf("handling the deletion: %v", err)
	}

	if len(s.inventory) != 2 {
		t.Fatalf("the bag holds %d rows, want both still there: %+v", len(s.inventory), s.inventory)
	}
	if s.inventory[0].Count != 2 {
		t.Errorf("the stack is %d, want the two that were not sold", s.inventory[0].Count)
	}

	// And the whole of a stack takes the row with it, rather than leaving a
	// line with nought against it.
	if err := s.handleItemDeleted(itemDeletedPacket(3, 2, packets.ItemDeletedSold)); err != nil {
		t.Fatalf("handling the deletion: %v", err)
	}

	if len(s.inventory) != 1 || s.inventory[0].Index != 4 {
		t.Errorf("the bag holds %+v, want only the other slot", s.inventory)
	}
}

// TestADeletionForASlotWeDoNotHoldIsIgnored: a packet for a slot the bag has
// never heard of changes nothing, rather than taking out whatever is first.
func TestADeletionForASlotWeDoNotHoldIsIgnored(t *testing.T) {
	s := &InGameState{inventory: []packets.InventoryItem{{Index: 3, ID: 507, Count: 5}}}

	if err := s.handleItemDeleted(itemDeletedPacket(99, 1, packets.ItemDeletedNormal)); err != nil {
		t.Fatalf("handling the deletion: %v", err)
	}

	if len(s.inventory) != 1 || s.inventory[0].Count != 5 {
		t.Errorf("the bag came out as %+v", s.inventory)
	}
}

// itemDeletedPacket builds a ZC_DELETE_ITEM_FROM_BODY the way the server does.
func itemDeletedPacket(index, count int, reason uint16) []byte {
	pkt := make([]byte, 8)
	binary.LittleEndian.PutUint16(pkt, packets.ZC_DELETE_ITEM_FROM_BODY)
	binary.LittleEndian.PutUint16(pkt[2:], reason)
	binary.LittleEndian.PutUint16(pkt[4:], uint16(index))
	binary.LittleEndian.PutUint16(pkt[6:], uint16(count))

	return pkt
}
