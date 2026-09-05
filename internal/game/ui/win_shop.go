package ui

import (
	"strconv"

	"github.com/Faultbox/midgard-ro/internal/engine/ui2d"
	"github.com/Faultbox/midgard-ro/internal/game/items"
	"github.com/Faultbox/midgard-ro/internal/game/states"
	"github.com/Faultbox/midgard-ro/internal/network/packets"
)

// The counter, as the original lays it out: two windows side by side.
//
// One holds what can be had and the other what has been chosen, and a thing
// moves from the first to the second a line at a time. Buying, the first is
// the shopkeeper's shelf. Selling, it is the player's own bag — the inventory
// window itself, opened alongside, since that is where the things being sold
// already are and a second copy of it would be a second place to look.
//
// Neither is opened by the player, so both go through hostedWindow.

const (
	shopStockWindowID = "hud_win_shop"
	shopCartWindowID  = "hud_win_shop_cart"
	shopAskWindowID   = "hud_shop_ask"
)

const (
	shopW float32 = 300
	shopH float32 = 300

	shopCartW float32 = 260
	shopCartH float32 = 240

	// shopGap is the space between the two, which is what makes them read as
	// a pair rather than as one window with a line down it.
	shopGap float32 = 8

	shopPad     float32 = 8
	shopRowH    float32 = 28
	shopScrollW float32 = 14

	shopFooterH float32 = 52

	shopIcon      float32 = 24
	shopTextScale float32 = 0.7

	// shopStackMost is how many of one thing the amount dialog will offer
	// when buying. A shelf does not run down, so there is nothing else to
	// bound it by.
	shopStackMost = 999
)

var (
	shopRowHot    = ui2d.Color{R: 0.87, G: 0.91, B: 0.98, A: 1}
	shopText      = ui2d.Color{R: 0.13, G: 0.13, B: 0.13, A: 1}
	shopEmptyText = ui2d.Color{R: 0.45, G: 0.45, B: 0.45, A: 1}
	shopSlotMark  = ui2d.Color{R: 0.88, G: 0.90, B: 0.95, A: 1}
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
	// Whichever panel is not showing is told so: the one that shows next
	// clears its remembered flags on the way in only if it counts as having
	// left.
	switch state.Shop.Mode {
	case states.ShopChoosing:
		b.shopStockWindow.hide()
		b.shopCartWindow.hide()
		b.drawShopAsk(screenW, screenH)

	case states.ShopBuying:
		b.shopAskWindow.hide()
		b.shopBagOpened = false
		b.drawShopStock(state, screenW, screenH)
		b.drawShopCart(state, screenW, screenH)

	case states.ShopSelling:
		b.shopAskWindow.hide()
		b.shopStockWindow.hide()

		// The bag is the shelf on this side, so the inventory itself opens
		// beside the list rather than a second copy of it.
		//
		// Once, on the way in. Opened every frame it could not be closed
		// again: the X would put it away and the next frame would bring it
		// straight back, which reads as a window that will not shut.
		if !b.shopBagOpened {
			b.OpenWindow(WindowItem)

			b.shopBagOpened = true
		}

		b.drawShopCart(state, screenW, screenH)
		b.placeBagBesideList(screenW, screenH)

	default:
		b.shopAskWindow.hide()
		b.shopStockWindow.hide()
		b.shopCartWindow.hide()

		// Nothing open: forget whatever was in the basket, so the next shop
		// does not open with the last one's order still in it.
		b.shopOrder = nil
		b.shopBagOpened = false
	}
}

// drawShopAsk is the question a shopkeeper who does both asks first.
//
// The original's message window, and not the shop's own: the same one asks
// whether to accept a trade from another player, so it is handed the words
// and the choices rather than knowing them.
func (b *UI2DBackend) drawShopAsk(screenW, screenH float32) {
	pressed, closed := b.drawMessageBox(&b.shopAskWindow, screenW, screenH,
		"message", "Please select a Deal Type.", []msgButton{
			{id: "shop_ask_buy", label: "buy"},
			{id: "shop_ask_sell", label: "sell"},
			{id: "shop_ask_cancel", label: "cancel"},
		})

	switch {
	case closed, pressed == "shop_ask_cancel":
		b.shopAction = shopActionState{close: true}
	case pressed == "shop_ask_buy":
		b.shopAction = shopActionState{ask: true, deal: packets.DealBuy}
	case pressed == "shop_ask_sell":
		b.shopAction = shopActionState{ask: true, deal: packets.DealSell}
	}
}

// shopRow is one line of either list, flattened so both sides draw the same.
type shopRow struct {
	// key is what an order names this by: the item for buying, the inventory
	// slot for selling. Two of the same thing in a bag are not worth the same
	// money, so selling cannot go by item.
	key   int
	id    uint32
	price uint32

	// have is how many there are to take, which bounds what may be chosen.
	have int
}

// shopPair is where the two windows sit, side by side and centered together.
func shopPair(screenW, screenH float32) (stockX, cartX, y float32) {
	both := shopW + shopGap + shopCartW

	stockX = (screenW - both) / 2
	cartX = stockX + shopW + shopGap
	y = (screenH - shopH) / 2

	return stockX, cartX, y
}

// drawShopStock draws what the shopkeeper has.
func (b *UI2DBackend) drawShopStock(state InGameUIState, screenW, screenH float32) {
	openX, _, openY := shopPair(screenW, screenH)

	draw, closed := b.shopStockWindow.begin(b.ctx, true,
		openX, openY, shopW, shopH, "Shop Items.", ui2d.WindowOptions{Closable: true})

	if closed {
		b.shopAction = shopActionState{close: true}
	}

	if !draw {
		return
	}

	x, y := b.shopStockWindow.rect(b.ctx, openX, openY)

	b.drawShopRows(shopStockRows(state),
		x+shopPad, y+ui2d.FrameTitleH+shopPad,
		shopW-2*shopPad, shopH-ui2d.FrameTitleH-2*shopPad)

	b.ctx.EndWindow()
}

// shopStockRows is the shelf.
//
// The discounted price rather than the list one: it is what this character
// pays, which a Merchant's Discount lowers, and the original shows that.
func shopStockRows(state InGameUIState) []shopRow {
	rows := make([]shopRow, 0, len(state.Shop.Buying))

	for _, item := range state.Shop.Buying {
		rows = append(rows, shopRow{
			key: int(item.ID), id: item.ID, price: item.Discount, have: shopStackMost,
		})
	}

	return rows
}

// drawShopCart draws what has been chosen, and the way out.
func (b *UI2DBackend) drawShopCart(state InGameUIState, screenW, screenH float32) {
	selling := state.Shop.Mode == states.ShopSelling

	title := "Item shopping list."
	if selling {
		title = "Item selling list."
	}

	_, openX, openY := shopPair(screenW, screenH)

	draw, closed := b.shopCartWindow.begin(b.ctx, true,
		openX, openY, shopCartW, shopCartH, title, ui2d.WindowOptions{Closable: true})

	if closed {
		b.shopAction = shopActionState{close: true}
	}

	if !draw {
		return
	}

	x, y := b.shopCartWindow.rect(b.ctx, openX, openY)

	rows := b.shopCartRows(state, selling)

	// What the shop will take, and how many of each, remembered for the drag
	// that carries something out of the bag: a release happens outside this
	// window and has no state of its own to ask.
	b.rememberSellable(state, selling)

	b.drawShopCartRows(rows,
		x+shopPad, y+ui2d.FrameTitleH+shopPad,
		shopCartW-2*shopPad, shopCartH-ui2d.FrameTitleH-shopFooterH-2*shopPad)

	b.drawShopFooter(state, rows, selling, x, y+shopCartH-shopFooterH)

	b.ctx.EndWindow()
}

// placeBagBesideList puts the bag where the shelf would be when it is in the
// way of the selling list.
//
// The two are a pair on this side of the counter — things are carried out of
// one and into the other — and a bag left wherever it was last dragged is
// commonly on top of a window that opens in the middle of the screen. Only
// when it actually overlaps: a bag already out of the way is where the player
// put it, and moving it then would be the interface tidying up after nobody.
func (b *UI2DBackend) placeBagBesideList(screenW, screenH float32) {
	if b.ctx == nil {
		return
	}

	bag, ok := b.ctx.WindowRect(itemsWindowID)
	if !ok {
		return
	}

	list, ok := b.ctx.WindowRect(shopCartWindowID)
	if !ok {
		return
	}

	if !overlaps(bag, list) {
		return
	}

	stockX, _, y := shopPair(screenW, screenH)

	b.ctx.MoveWindow(itemsWindowID, stockX, y)
}

// overlaps reports whether two windows are on top of one another.
func overlaps(a, c ui2d.Rect) bool {
	return a.X < c.X+c.W && c.X < a.X+a.W &&
		a.Y < c.Y+c.H && c.Y < a.Y+a.H
}

// shopCartRows is what has been chosen, walked in the order the other side
// shows it so an order reads the same way in a packet trace.
func (b *UI2DBackend) shopCartRows(state InGameUIState, selling bool) []shopRow {
	source := shopStockRows(state)
	if selling {
		source = shopSellableRows(state)
	}

	rows := make([]shopRow, 0, len(b.shopOrder))
	for _, row := range source {
		if b.shopOrder[row.key] > 0 {
			rows = append(rows, row)
		}
	}

	return rows
}

// shopSellableRows is what the shop will take out of the bag.
//
// The sell list names slots and says nothing about what is in them, so the bag
// is what says which item a row is; a slot the bag has never heard of is left
// out rather than drawn as a nameless row.
func shopSellableRows(state InGameUIState) []shopRow {
	rows := make([]shopRow, 0, len(state.Shop.Selling))

	for _, item := range state.Shop.Selling {
		row := shopRow{key: item.Index, price: item.Overcharge}

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

// rememberSellable notes what the shop will take out of the bag.
//
// The shop names what it wants and says nothing about the rest, so a thing it
// did not name cannot be sold however it is carried over — and the drag that
// carries one there is resolved in another file, on a frame this window may
// not even be drawn on.
func (b *UI2DBackend) rememberSellable(state InGameUIState, selling bool) {
	if !selling {
		b.shopSellable = nil

		return
	}

	rows := shopSellableRows(state)

	b.shopSellable = make(map[int]int, len(rows))
	for _, row := range rows {
		b.shopSellable[row.key] = row.have
	}
}

// drawShopRows lists a shelf, scrolled to wherever the bar is.
func (b *UI2DBackend) drawShopRows(rows []shopRow, x, y, w, h float32) {
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
		box := ui2d.Rect{X: x, Y: y + float32(i)*shopRowH, W: w, H: shopRowH}

		b.drawShopRow(row, box, 0)

		if b.ctx.InvisibleButtonAt("shop_row_"+strconv.Itoa(row.key), box.X, box.Y, box.W, box.H) {
			b.beginShopPrompt(row.key, row.id, row.have, false)
		}
	}

	b.scrollShop(len(rows), visible, x, y, w, h)
}

// drawShopCartRows lists what has been chosen.
//
// The empty places below are drawn as the slot marks the original has there,
// so a list with two things in it still reads as a list rather than as two
// lines floating in a panel.
func (b *UI2DBackend) drawShopCartRows(rows []shopRow, x, y, w, h float32) {
	places := max(int(h/shopRowH), 1)

	for i := 0; i < places; i++ {
		box := ui2d.Rect{X: x, Y: y + float32(i)*shopRowH, W: w, H: shopRowH}

		if i >= len(rows) {
			b.ctx.FillEllipseImageLayer(
				box.X+4, box.Y+(shopRowH-10)/2, shopIcon-2, 10, shopSlotMark)

			continue
		}

		row := rows[i]

		b.drawShopRow(row, box, b.shopOrder[row.key])

		// Clicking a line takes it back off, which is how a mind is changed
		// without closing the counter.
		if b.ctx.InvisibleButtonAt("shop_cart_"+strconv.Itoa(row.key), box.X, box.Y, box.W, box.H) {
			delete(b.shopOrder, row.key)
		}
	}
}

// drawShopRow draws one line: the icon, the name, and the money.
func (b *UI2DBackend) drawShopRow(row shopRow, box ui2d.Rect, amount int) {
	r := b.ctx.Renderer()

	if box.Contains(b.ctx.Input().MouseX, b.ctx.Input().MouseY) {
		r.DrawRect(box.X, box.Y, box.W, box.H, shopRowHot)
	}

	if info, ok := items.Lookup(row.id); ok && info.Resource != "" {
		if tex, err := b.texCache.Load(itemIconPath + info.Resource + ".bmp"); err == nil {
			r.DrawImage(tex.ID, box.X+2, box.Y+2, shopIcon, shopIcon, ui2d.ColorWhite)
		}
	}

	name := itemDisplayName(row.id)
	if amount > 0 {
		name += " ×" + strconv.Itoa(amount)
	}

	r.DrawText(box.X+shopIcon+8, box.Y+7, name, shopTextScale, shopText)

	money := int(row.price)
	if amount > 0 {
		money *= amount
	}

	price := shopZeny(money) + " Z"
	priceW, _ := r.MeasureText(price, shopTextScale)

	r.DrawText(box.X+box.W-priceW-4, box.Y+7, price, shopTextScale, shopText)
}

// shopZeny groups a figure in threes, as every price in the original is.
//
// A billion zeny with no separators is a row of digits nobody can read at a
// glance, and the prices at the top of a shop list run that long.
func shopZeny(amount int) string {
	digits := strconv.Itoa(amount)

	sign := ""
	if amount < 0 {
		sign, digits = "-", digits[1:]
	}

	// The first group is the short one, and every group after it is three.
	lead := len(digits) % 3
	if lead == 0 {
		lead = 3
	}

	out := digits[:lead]
	for at := lead; at < len(digits); at += 3 {
		out += "," + digits[at:at+3]
	}

	return sign + out
}

// scrollShop moves a list under the wheel, over the list rather than over the
// window: the wheel belongs to what is under it.
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

// addShopLine puts a line on the order, or takes it off when nothing is left.
func (b *UI2DBackend) addShopLine(key, amount int) {
	if b.shopOrder == nil {
		b.shopOrder = map[int]int{}
	}

	if amount <= 0 {
		delete(b.shopOrder, key)

		return
	}

	b.shopOrder[key] = amount
}

// drawShopFooter is the total, the purse, and the two buttons.
func (b *UI2DBackend) drawShopFooter(state InGameUIState, rows []shopRow, selling bool, x, y float32) {
	r := b.ctx.Renderer()

	total := 0
	for _, row := range rows {
		total += int(row.price) * b.shopOrder[row.key]
	}

	r.DrawText(x+shopPad, y+6, "Total : "+shopZeny(total)+" Zeny", shopTextScale, shopText)

	purse := "Zeny : " + shopZeny(int(state.PlayerZeny))
	purseW, _ := r.MeasureText(purse, shopTextScale)
	r.DrawText(x+shopCartW-purseW-shopPad, y+6, purse, shopTextScale, shopText)

	action := "buy"
	if selling {
		action = "sell"
	}

	btnY := y + 24

	confirm := shopFooterButton(x, btnY, 0)
	b.drawFlatButton(confirm, action, len(b.shopOrder) == 0)

	if len(b.shopOrder) > 0 &&
		b.ctx.InvisibleButtonAt("shop_confirm", confirm.X, confirm.Y, confirm.W, confirm.H) {
		b.shopAction = shopActionState{order: b.shopOrderLines(rows, selling), sell: selling}
		b.shopOrder = nil
	}

	cancel := shopFooterButton(x, btnY, 1)
	b.drawFlatButton(cancel, "cancel", false)

	if b.ctx.InvisibleButtonAt("shop_close", cancel.X, cancel.Y, cancel.W, cancel.H) {
		b.shopAction = shopActionState{close: true}
	}
}

// shopFooterButton is where one of the two sits: against the right edge, in
// the order the original has them, with cancel in the corner.
func shopFooterButton(x, y float32, fromLeft int) ui2d.Rect {
	fromRight := 1 - fromLeft

	left := x + shopCartW - shopPad -
		float32(fromRight+1)*msgBoxBtnW - float32(fromRight)*msgBoxBtnGap

	return ui2d.Rect{X: left, Y: y, W: msgBoxBtnW, H: msgBoxBtnH}
}

// shopOrderLines turns the basket into what the packet wants: item ids one
// way across the counter and inventory slots the other.
func (b *UI2DBackend) shopOrderLines(rows []shopRow, selling bool) []packets.ShopOrder {
	order := make([]packets.ShopOrder, 0, len(b.shopOrder))

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

// shopCartRect is where the list is, for a drag to be let go over.
//
// Empty when no counter is open, so a drag out of the bag at any other time is
// not swallowed by a window that is not there.
func (b *UI2DBackend) shopCartRect() ui2d.Rect {
	if !b.shopCartWindow.shown || b.ctx == nil {
		return ui2d.Rect{}
	}

	at, ok := b.ctx.WindowRect(shopCartWindowID)
	if !ok {
		return ui2d.Rect{}
	}

	return at
}

// sellDragged puts something carried out of the bag onto the selling list.
//
// A stack is asked about, opening with the whole of it: at a counter the usual
// answer is all of them.
func (b *UI2DBackend) sellDragged(dragged itemDrag) {
	have, will := b.shopSellable[dragged.index]
	if !will {
		// The shop said what it wants, and this is not among it.
		return
	}

	b.beginShopPrompt(dragged.index, dragged.itemID, have, true)
}
