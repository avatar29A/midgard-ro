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
// what is in them, so the bag is what says which item a row is — and a slot the
// bag has never heard of is left out rather than drawn as a nameless row.
func TestSellRowsComeFromTheBag(t *testing.T) {
	rows := shopSellableRows(sellingState())

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

	rows := shopStockRows(state)

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

// TestTheCartHoldsWhatWasChosen: the list on the right is the basket walked in
// the order the other side shows it, so an order goes out the way it is read.
func TestTheCartHoldsWhatWasChosen(t *testing.T) {
	b := &UI2DBackend{shopOrder: map[int]int{7: 1, 3: 4}}

	rows := b.shopCartRows(sellingState(), true)

	if len(rows) != 2 {
		t.Fatalf("the basket holds %d lines, want two: %+v", len(rows), rows)
	}
	if rows[0].key != 3 || rows[1].key != 7 {
		t.Errorf("the basket came out as %+v, want the shop's own order", rows)
	}

	// And nothing that was not chosen.
	b.shopOrder = map[int]int{7: 1}

	if rows := b.shopCartRows(sellingState(), true); len(rows) != 1 || rows[0].key != 7 {
		t.Errorf("the basket came out as %+v, want the one line", rows)
	}
}

// TestALineIsTakenOffWhenItEmpties: an amount of nothing is not a line with
// nought against it — it is a line that is no longer there.
func TestALineIsTakenOffWhenItEmpties(t *testing.T) {
	b := &UI2DBackend{}

	b.addShopLine(501, 3)
	if b.shopOrder[501] != 3 {
		t.Errorf("the line came out as %d", b.shopOrder[501])
	}

	b.addShopLine(501, 0)
	if _, still := b.shopOrder[501]; still {
		t.Errorf("the line is still on the order: %+v", b.shopOrder)
	}
}

// TestOrderLinesNameWhatThePacketWants: item ids one way across the counter
// and inventory slots the other.
func TestOrderLinesNameWhatThePacketWants(t *testing.T) {
	b := &UI2DBackend{shopOrder: map[int]int{3: 2, 7: 1}}

	rows := b.shopCartRows(sellingState(), true)
	lines := b.shopOrderLines(rows, true)

	if len(lines) != 2 {
		t.Fatalf("got %d lines, want two: %+v", len(lines), lines)
	}
	for _, line := range lines {
		if line.Index == 0 || line.ID != 0 {
			t.Errorf("a sell line names an item rather than a slot: %+v", line)
		}
	}
	if lines[0].Index != 3 || lines[1].Index != 7 {
		t.Errorf("the lines came out as %+v", lines)
	}

	b = &UI2DBackend{shopOrder: map[int]int{501: 3}}
	buy := b.shopOrderLines([]shopRow{{key: 501, id: 501}}, false)

	if len(buy) != 1 || buy[0].ID != 501 || buy[0].Amount != 3 || buy[0].Index != 0 {
		t.Errorf("the buy line came out as %+v", buy)
	}
}

// TestOnlyWhatTheShopNamedCanBeSold: a thing the shop did not name cannot be
// sold however it is carried over, and the drag that carries one is resolved
// on a frame the list may not be drawn on — so what it will take is noted
// while it is.
func TestOnlyWhatTheShopNamedCanBeSold(t *testing.T) {
	b := &UI2DBackend{}

	b.rememberSellable(sellingState(), true)

	if have, will := b.shopSellable[3]; !will || have != 12 {
		t.Errorf("slot 3 is remembered as %d, %v", have, will)
	}
	if _, will := b.shopSellable[99]; will {
		t.Error("a slot the bag cannot name is remembered as sellable")
	}
	if _, will := b.shopSellable[42]; will {
		t.Error("a slot the shop never named is remembered as sellable")
	}

	// Buying, there is nothing to carry into the list, so nothing is kept.
	b.rememberSellable(sellingState(), false)

	if b.shopSellable != nil {
		t.Errorf("the buying side remembered %+v", b.shopSellable)
	}
}

// TestZenyIsGroupedInThrees: a billion with no separators is a row of digits
// nobody can read, and the prices at the top of a shop list run that long.
func TestZenyIsGroupedInThrees(t *testing.T) {
	for _, tc := range []struct {
		amount int
		want   string
	}{
		{0, "0"},
		{5, "5"},
		{50, "50"},
		{500, "500"},
		{5000, "5,000"},
		{1000000000, "1,000,000,000"},
		{1500000000, "1,500,000,000"},
		{-250, "-250"},
	} {
		if got := shopZeny(tc.amount); got != tc.want {
			t.Errorf("shopZeny(%d) = %q, want %q", tc.amount, got, tc.want)
		}
	}
}

// TestTheTwoWindowsDoNotOverlap: they are a pair side by side, and one drawn
// over the other is a shelf that cannot be read while choosing from it.
func TestTheTwoWindowsDoNotOverlap(t *testing.T) {
	stockX, cartX, _ := shopPair(1280, 720)

	if right := stockX + shopW; cartX < right {
		t.Errorf("the list starts at %v, inside the shelf ending at %v", cartX, right)
	}
	if stockX < 0 || cartX+shopCartW > 1280 {
		t.Errorf("the pair runs from %v to %v, off a 1280 screen", stockX, cartX+shopCartW)
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
