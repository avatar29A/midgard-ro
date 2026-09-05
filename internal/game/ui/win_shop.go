package ui

import (
	"strconv"

	"github.com/Faultbox/midgard-ro/internal/engine/ui2d"
	"github.com/Faultbox/midgard-ro/internal/game/items"
	"github.com/Faultbox/midgard-ro/internal/game/states"
	"github.com/Faultbox/midgard-ro/internal/network/packets"
)

// The counter.
//
// One window for both sides of it, because they are the same window: a list of
// rows with a price against each, an order built up a click at a time, and a
// total at the bottom. What differs is where the rows come from and which way
// the zeny goes.
//
// It is not one of the menu windows. A shop is open because a shopkeeper
// opened it, so there is no button that brings it back and nothing to
// remember: closing it is done with it.

const shopWindowID = "hud_win_shop"

const (
	shopW float32 = 300
	shopH float32 = 280

	shopPad     float32 = 8
	shopRowH    float32 = 26
	shopScrollW float32 = 14

	shopFooterH float32 = 46

	shopIcon      float32 = 22
	shopTextScale float32 = 0.65

	// shopAskW is the little panel a shopkeeper who does both asks from.
	shopAskW float32 = escBtnW + 2*escPad
	shopAskH         = ui2d.FrameTitleH + escPad + 2*(escBtnH+escBtnG) - escBtnG + escPad
)

var (
	shopRowChosen = ui2d.Color{R: 0.85, G: 0.89, B: 0.97, A: 1}
	shopPriceText = ui2d.Color{R: 0.15, G: 0.35, B: 0.15, A: 1}
	shopTotalText = ui2d.Color{R: 0.11, G: 0.11, B: 0.11, A: 1}
	shopEmptyText = ui2d.Color{R: 0.45, G: 0.45, B: 0.45, A: 1}
)

// ShopAction is what the player asked the counter for, read once and cleared.
type ShopAction struct {
	// Deal answers the buy-or-sell question when Ask is set.
	Ask  bool
	Deal uint8

	// Order is what to buy or sell, and Sell says which.
	Order []packets.ShopOrder
	Sell  bool

	// Close is the counter being put away, which needs no packet.
	Close bool
}

// TakeShopAction returns what was asked for and clears it. The interface has
// no connection, so acting on it is the caller's job.
func (b *UI2DBackend) TakeShopAction() (ShopAction, bool) {
	action := b.shopAction
	if !action.asked() {
		return ShopAction{}, false
	}

	b.shopAction = shopActionState{}

	return ShopAction{
		Ask: action.ask, Deal: action.deal,
		Order: action.order, Sell: action.sell, Close: action.close,
	}, true
}

// shopActionState is the same thing while it waits.
type shopActionState struct {
	ask   bool
	deal  uint8
	sell  bool
	close bool
	order []packets.ShopOrder
}

// asked reports whether anything is waiting to be sent.
func (a shopActionState) asked() bool {
	return a.ask || a.close || len(a.order) > 0
}

// drawShop draws whichever side of the counter is open.
func (b *UI2DBackend) drawShop(state InGameUIState, screenW, screenH float32) {
	switch state.Shop.Mode {
	case states.ShopChoosing:
		b.drawShopAsk(screenW, screenH)
	case states.ShopBuying, states.ShopSelling:
		b.drawShopList(state, screenW, screenH)
	default:
		// Nothing open: forget whatever was in the basket, so the next shop
		// does not open with the last one's order still in it.
		b.shopOrder = nil
	}
}

// drawShopAsk is the two buttons a shopkeeper who does both asks from.
func (b *UI2DBackend) drawShopAsk(screenW, screenH float32) {
	openX := (screenW - shopAskW) / 2
	openY := (screenH - shopAskH) / 2

	if !b.ctx.BeginWindowEx("hud_shop_ask", openX, openY, shopAskW, shopAskH,
		"Shop", ui2d.WindowOptions{Closable: true}) {
		if b.ctx.WindowClosed("hud_shop_ask") {
			b.shopAction = shopActionState{close: true}
		}

		return
	}

	x, y := openX, openY
	if rect, ok := b.ctx.WindowRect("hud_shop_ask"); ok {
		x, y = rect.X, rect.Y
	}

	btnY := y + ui2d.FrameTitleH + escPad

	for _, choice := range []struct {
		id    string
		label string
		deal  uint8
	}{
		{"shop_ask_buy", "Buy", packets.DealBuy},
		{"shop_ask_sell", "Sell", packets.DealSell},
	} {
		box := ui2d.Rect{X: x + escPad, Y: btnY, W: escBtnW, H: escBtnH}
		btnY += escBtnH + escBtnG

		b.drawFlatButton(box, choice.label, false)

		if b.ctx.InvisibleButtonAt(choice.id, box.X, box.Y, box.W, box.H) {
			b.shopAction = shopActionState{ask: true, deal: choice.deal}
		}
	}

	b.ctx.EndWindow()
}

// shopRow is one line of either list, flattened so both sides draw the same.
type shopRow struct {
	// key is what an order names this by: the item for buying, the inventory
	// slot for selling. Two of the same thing in a bag are not worth the same
	// money, so selling cannot go by item.
	key   int
	id    uint32
	price uint32

	// have is how many are in the bag, for the selling side.
	have int
}

// drawShopList draws the counter and takes an order.
func (b *UI2DBackend) drawShopList(state InGameUIState, screenW, screenH float32) {
	selling := state.Shop.Mode == states.ShopSelling

	title := "Shop"
	if selling {
		title = "Sell"
	}

	openX := (screenW - shopW) / 2
	openY := (screenH - shopH) / 2

	if !b.ctx.BeginWindowEx(shopWindowID, openX, openY, shopW, shopH,
		title, ui2d.WindowOptions{Closable: true}) {
		if b.ctx.WindowClosed(shopWindowID) {
			b.shopAction = shopActionState{close: true}
		}

		return
	}

	x, y := openX, openY
	if rect, ok := b.ctx.WindowRect(shopWindowID); ok {
		x, y = rect.X, rect.Y
	}

	rows := shopRows(state, selling)

	listX := x + shopPad
	listY := y + ui2d.FrameTitleH + shopPad
	listH := shopH - ui2d.FrameTitleH - shopFooterH - 2*shopPad
	listW := shopW - 2*shopPad

	b.drawShopRows(rows, selling, listX, listY, listW, listH)
	b.drawShopFooter(state, rows, selling, x, y+shopH-shopFooterH)

	b.ctx.EndWindow()
}

// shopRows is the open side of the counter as rows.
func shopRows(state InGameUIState, selling bool) []shopRow {
	if !selling {
		rows := make([]shopRow, 0, len(state.Shop.Buying))
		for _, item := range state.Shop.Buying {
			rows = append(rows, shopRow{key: int(item.ID), id: item.ID, price: item.Discount})
		}

		return rows
	}

	rows := make([]shopRow, 0, len(state.Shop.Selling))
	for _, item := range state.Shop.Selling {
		row := shopRow{key: item.Index, price: item.Overcharge}

		// The sell list names slots and says nothing about what is in them,
		// so the bag is what says which item a row is.
		for _, held := range state.Inventory {
			if held.Index == item.Index {
				row.id = held.ID
				row.have = held.Count

				break
			}
		}

		if row.id == 0 {
			continue
		}

		rows = append(rows, row)
	}

	return rows
}

// drawShopRows lists them, scrolled to wherever the bar is.
func (b *UI2DBackend) drawShopRows(rows []shopRow, selling bool, x, y, w, h float32) {
	r := b.ctx.Renderer()

	if len(rows) == 0 {
		r.DrawText(x, y, "Nothing here.", shopTextScale, shopEmptyText)

		return
	}

	visible := max(int(h/shopRowH), 1)
	maxOffset := max(0, len(rows)-visible)

	offset := min(b.shopScroll, maxOffset)

	if maxOffset > 0 {
		b.shopScroll = b.scrollbar("hud_shop", x+w-shopScrollW, y, h, offset, maxOffset, visible)
		offset = b.shopScroll
		w -= shopScrollW
	} else {
		b.shopScroll = 0
		offset = 0
	}

	for i := 0; i < visible && offset+i < len(rows); i++ {
		row := rows[offset+i]
		b.drawShopRow(row, selling, ui2d.Rect{X: x, Y: y + float32(i)*shopRowH, W: w, H: shopRowH})
	}

	b.scrollShop(len(rows), visible, x, y, w, h)
}

// drawShopRow draws one, and takes the clicks on it.
func (b *UI2DBackend) drawShopRow(row shopRow, selling bool, box ui2d.Rect) {
	r := b.ctx.Renderer()

	ordered := b.shopOrder[row.key]
	if ordered > 0 {
		r.DrawRect(box.X, box.Y, box.W, box.H, shopRowChosen)
	}

	if info, ok := items.Lookup(row.id); ok && info.Resource != "" {
		if tex, err := b.texCache.Load(itemIconPath + info.Resource + ".bmp"); err == nil {
			r.DrawImage(tex.ID, box.X+1, box.Y+2, shopIcon, shopIcon, ui2d.ColorWhite)
		}
	}

	name := itemDisplayName(row.id)
	if ordered > 0 {
		name += " ×" + strconv.Itoa(ordered)
	}

	r.DrawText(box.X+shopIcon+6, box.Y+5, name, shopTextScale, itemInfoValue)

	price := strconv.FormatUint(uint64(row.price), 10) + "z"
	priceW, _ := r.MeasureText(price, shopTextScale)
	r.DrawText(box.X+box.W-priceW-2, box.Y+5, price, shopTextScale, shopPriceText)

	in := b.ctx.Input()
	if !box.Contains(in.MouseX, in.MouseY) {
		return
	}

	// A left click adds one and a right click takes one back, which is the
	// whole of building an order without a second window to drag into.
	if b.ctx.InvisibleButtonAt("shop_row_"+strconv.Itoa(row.key), box.X, box.Y, box.W, box.H) {
		b.addToShopOrder(row, selling, 1)
	}

	if in.MouseRightPressed {
		b.addToShopOrder(row, selling, -1)
	}
}

// addToShopOrder changes how many of a row are on the order.
//
// Never past what can be had: the selling side is bounded by what is in the
// bag, since the server refuses an order for more than is there and refuses
// the whole of it rather than the line.
func (b *UI2DBackend) addToShopOrder(row shopRow, selling bool, by int) {
	if b.shopOrder == nil {
		b.shopOrder = map[int]int{}
	}

	want := b.shopOrder[row.key] + by

	if selling && want > row.have {
		want = row.have
	}

	if want <= 0 {
		delete(b.shopOrder, row.key)

		return
	}

	b.shopOrder[row.key] = want
}

// scrollShop moves the list under the wheel, over the list rather than over
// the window: the wheel belongs to what is under it.
func (b *UI2DBackend) scrollShop(rows, visible int, x, y, w, h float32) {
	in := b.ctx.Input()
	if in == nil || in.ScrollY == 0 {
		return
	}

	if !(ui2d.Rect{X: x, Y: y, W: w, H: h}).Contains(in.MouseX, in.MouseY) {
		return
	}

	b.shopScroll = min(max(b.shopScroll-int(in.ScrollY), 0), max(0, rows-visible))
}

// drawShopFooter is the total, what is in the purse, and the two buttons.
func (b *UI2DBackend) drawShopFooter(state InGameUIState, rows []shopRow, selling bool, x, y float32) {
	r := b.ctx.Renderer()

	total := 0
	for _, row := range rows {
		total += int(row.price) * b.shopOrder[row.key]
	}

	label := "Total: " + strconv.Itoa(total) + "z"
	if selling {
		label = "You get: " + strconv.Itoa(total) + "z"
	}

	r.DrawText(x+shopPad, y+4, label, shopTextScale, shopTotalText)

	purse := "Zeny: " + strconv.FormatInt(state.PlayerZeny, 10)
	purseW, _ := r.MeasureText(purse, shopTextScale)
	r.DrawText(x+shopW-purseW-shopPad, y+4, purse, shopTextScale, shopTotalText)

	btnW := (shopW - 3*shopPad) / 2
	btnY := y + 20

	action := "Buy"
	if selling {
		action = "Sell"
	}

	confirm := ui2d.Rect{X: x + shopPad, Y: btnY, W: btnW, H: escBtnH}
	b.drawFlatButton(confirm, action, len(b.shopOrder) == 0)

	if len(b.shopOrder) > 0 && b.ctx.InvisibleButtonAt("shop_confirm", confirm.X, confirm.Y, confirm.W, confirm.H) {
		b.shopAction = shopActionState{order: b.shopOrderLines(rows, selling), sell: selling}
		b.shopOrder = nil
	}

	cancel := ui2d.Rect{X: x + 2*shopPad + btnW, Y: btnY, W: btnW, H: escBtnH}
	b.drawFlatButton(cancel, "Close", false)

	if b.ctx.InvisibleButtonAt("shop_close", cancel.X, cancel.Y, cancel.W, cancel.H) {
		b.shopAction = shopActionState{close: true}
	}
}

// shopOrderLines turns the basket into what the packet wants: item ids one
// way across the counter and inventory slots the other.
func (b *UI2DBackend) shopOrderLines(rows []shopRow, selling bool) []packets.ShopOrder {
	order := make([]packets.ShopOrder, 0, len(b.shopOrder))

	// Walked in list order rather than over the map, so an order goes out in
	// the order it is shown and reads the same in a packet trace.
	for _, row := range rows {
		amount := b.shopOrder[row.key]
		if amount <= 0 {
			continue
		}

		if selling {
			order = append(order, packets.ShopOrder{Index: row.key, Amount: amount})

			continue
		}

		order = append(order, packets.ShopOrder{ID: row.id, Amount: amount})
	}

	return order
}
