package ui

import (
	"testing"

	"github.com/Faultbox/midgard-ro/internal/game/states"
	"github.com/Faultbox/midgard-ro/internal/network/packets"
)

// sellingState is a shop that will take three things out of a bag holding two
// of them, so the row that has nothing behind it can be told from the rest.
func sellingState() InGameUIState {
	return InGameUIState{
		Shop: states.Shop{
			Mode: states.ShopSelling,
			Selling: []packets.SellItem{
				{Index: 3, Price: 5, Overcharge: 6},
				{Index: 7, Price: 100, Overcharge: 110},
				{Index: 99, Price: 1, Overcharge: 1},
			},
		},
		Inventory: []packets.InventoryItem{
			{Index: 3, ID: 501, Count: 12},
			{Index: 7, ID: 1101, Count: 1},
		},
	}
}

// TestSellRowsComeFromTheBag: the sell list names slots and says nothing about
// what is in them, so the bag is what says which item a row is — and a slot
// the bag has never heard of is left out rather than drawn as a nameless row.
func TestSellRowsComeFromTheBag(t *testing.T) {
	rows := shopRows(sellingState(), true)

	if len(rows) != 2 {
		t.Fatalf("got %d rows, want the two the bag can name: %+v", len(rows), rows)
	}

	if rows[0].id != 501 || rows[0].have != 12 || rows[0].key != 3 {
		t.Errorf("the first row is %+v", rows[0])
	}

	// The overcharge rather than the plain price: it is what a Merchant
	// actually gets, and the original shows that.
	if rows[0].price != 6 {
		t.Errorf("the first row asks %d, want the overcharge of 6", rows[0].price)
	}
}

// TestBuyRowsNameItems: every copy on a shelf is the same, so a row is the
// item and the discounted price is what the character pays.
func TestBuyRowsNameItems(t *testing.T) {
	state := InGameUIState{Shop: states.Shop{
		Mode:   states.ShopBuying,
		Buying: []packets.ShopItem{{ID: 501, Price: 50, Discount: 44}},
	}}

	rows := shopRows(state, false)

	if len(rows) != 1 {
		t.Fatalf("got %d rows, want one", len(rows))
	}
	if rows[0].id != 501 || rows[0].key != 501 {
		t.Errorf("the row is %+v, want the item both ways", rows[0])
	}
	if rows[0].price != 44 {
		t.Errorf("the row asks %d, want the discounted 44", rows[0].price)
	}
}

// TestSellingCannotOfferMoreThanIsHeld: the server refuses an order for more
// than is in the bag, and refuses the whole of it rather than the line.
func TestSellingCannotOfferMoreThanIsHeld(t *testing.T) {
	b := &UI2DBackend{}
	row := shopRow{key: 7, id: 1101, price: 110, have: 1}

	for i := 0; i < 5; i++ {
		b.addToShopOrder(row, true, 1)
	}

	if got := b.shopOrder[row.key]; got != 1 {
		t.Errorf("offered %d of a thing there is one of", got)
	}

	// Buying is not bounded that way: a shelf does not run down.
	shelf := shopRow{key: 501, id: 501, price: 44}
	for i := 0; i < 5; i++ {
		b.addToShopOrder(shelf, false, 1)
	}

	if got := b.shopOrder[shelf.key]; got != 5 {
		t.Errorf("ordered %d off a shelf, want 5", got)
	}
}

// TestTakingOneBackEmptiesTheLine: a right click is how one comes off the
// order, and the last one off takes the line with it rather than leaving a
// row marked as ordered with nothing on it.
func TestTakingOneBackEmptiesTheLine(t *testing.T) {
	b := &UI2DBackend{}
	row := shopRow{key: 501, id: 501, price: 44}

	b.addToShopOrder(row, false, 1)
	b.addToShopOrder(row, false, -1)

	if _, still := b.shopOrder[row.key]; still {
		t.Errorf("the line is still on the order: %+v", b.shopOrder)
	}

	// And it does not go below nothing.
	b.addToShopOrder(row, false, -1)
	if got := b.shopOrder[row.key]; got != 0 {
		t.Errorf("the order went to %d", got)
	}
}

// TestOrderLinesNameWhatThePacketWants: item ids one way across the counter
// and inventory slots the other.
func TestOrderLinesNameWhatThePacketWants(t *testing.T) {
	b := &UI2DBackend{shopOrder: map[int]int{3: 2, 7: 1}}

	rows := shopRows(sellingState(), true)
	lines := b.shopOrderLines(rows, true)

	if len(lines) != 2 {
		t.Fatalf("got %d lines, want two: %+v", len(lines), lines)
	}
	for _, line := range lines {
		if line.Index == 0 || line.ID != 0 {
			t.Errorf("a sell line names an item rather than a slot: %+v", line)
		}
	}

	// The order they are shown in, so a packet trace reads the same way.
	if lines[0].Index != 3 || lines[1].Index != 7 {
		t.Errorf("the lines came out as %+v", lines)
	}

	b = &UI2DBackend{shopOrder: map[int]int{501: 3}}
	buy := b.shopOrderLines([]shopRow{{key: 501, id: 501}}, false)

	if len(buy) != 1 || buy[0].ID != 501 || buy[0].Amount != 3 || buy[0].Index != 0 {
		t.Errorf("the buy line came out as %+v", buy)
	}
}

// TestTakeShopActionClears: the pick is read once and gone, or the order goes
// out again every frame.
func TestTakeShopActionClears(t *testing.T) {
	b := &UI2DBackend{}

	if _, ok := b.TakeShopAction(); ok {
		t.Error("a backend nobody asked anything of returned an action")
	}

	b.shopAction = shopActionState{close: true}

	if action, ok := b.TakeShopAction(); !ok || !action.Close {
		t.Errorf("TakeShopAction = %+v, %v", action, ok)
	}
	if _, ok := b.TakeShopAction(); ok {
		t.Error("the action was still there on the second read")
	}
}
