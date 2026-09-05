package states

import (
	"go.uber.org/zap"

	"github.com/Faultbox/midgard-ro/internal/logger"
	"github.com/Faultbox/midgard-ro/internal/network/packets"
	"github.com/Faultbox/midgard-ro/internal/trace"
)

// Buying from and selling to an NPC.
//
// A shop is not a dialog. Talking to a shopkeeper who only sells opens the buy
// list straight away and the conversation is over; one who does both asks
// which first, and that question is its own packet rather than a menu in the
// dialog box.
//
// What is open is held here rather than in the interface because the answer
// has to survive the window being dragged, and because the interface has no
// connection to send an order with.

// ShopMode is which side of the counter is open.
type ShopMode int

const (
	// ShopClosed is no shop open.
	ShopClosed ShopMode = iota

	// ShopChoosing is the shopkeeper asking whether we mean to buy or sell.
	ShopChoosing

	// ShopBuying and ShopSelling are the two lists.
	ShopBuying
	ShopSelling
)

// Shop is what is open, for the interface to draw.
type Shop struct {
	Mode ShopMode

	// NPC is who is behind the counter, needed to answer the buy-or-sell
	// question and to say who refused an order.
	NPC uint32

	// Buying is what the shop sells, and Selling what it will take, by
	// inventory slot.
	Buying  []packets.ShopItem
	Selling []packets.SellItem
}

// Open reports whether anything is open.
func (s Shop) Open() bool {
	return s.Mode != ShopClosed
}

// Shop returns what is open.
func (s *InGameState) Shop() Shop {
	return s.shop
}

// handleDealType is a shopkeeper asking whether we mean to buy or to sell.
func (s *InGameState) handleDealType(data []byte) error {
	npc, ok := packets.DecodeDealType(data)
	if !ok {
		logger.Warn("short deal type packet", zap.Int("len", len(data)))

		return nil
	}

	trace.Emit(trace.HUD, "shop-ask", zap.Uint32("npc", npc))

	// The conversation is over: the question is asked by the shop rather than
	// by the dialog box, and a dialog left open behind it is a window with a
	// next button that goes nowhere.
	s.dropDialog()

	s.shop = Shop{Mode: ShopChoosing, NPC: npc}

	return nil
}

// ChooseDeal answers that question.
func (s *InGameState) ChooseDeal(deal uint8) error {
	if s.shop.Mode != ShopChoosing || s.client == nil {
		return nil
	}

	trace.Emit(trace.HUD, "shop-choose",
		zap.Uint32("npc", s.shop.NPC), zap.Uint8("deal", deal))

	// The list is not asked for here; the answer brings it. Setting a mode
	// now would draw an empty shop for as long as the round trip takes.
	return s.client.Send(packets.EncodeDealType(s.shop.NPC, deal))
}

// handleShopItems takes what a shop sells.
func (s *InGameState) handleShopItems(data []byte) error {
	items := packets.DecodeShopItems(data)

	trace.Emit(trace.HUD, "shop-buy-list", zap.Int("count", len(items)))

	// The NPC is kept from the question if there was one. A shopkeeper who
	// only sells opens the list without asking, and then nobody has said who
	// they are — which is fine, since nothing is sent back to them by id.
	s.shop.Mode = ShopBuying
	s.shop.Buying = items
	s.shop.Selling = nil

	s.dropDialog()

	return nil
}

// handleSellItems takes what a shop will buy from us.
func (s *InGameState) handleSellItems(data []byte) error {
	items := packets.DecodeSellItems(data)

	trace.Emit(trace.HUD, "shop-sell-list", zap.Int("count", len(items)))

	s.shop.Mode = ShopSelling
	s.shop.Selling = items
	s.shop.Buying = nil

	s.dropDialog()

	return nil
}

// Buy asks for an order.
func (s *InGameState) Buy(order []packets.ShopOrder) error {
	if s.client == nil || len(order) == 0 {
		return nil
	}

	trace.Emit(trace.HUD, "shop-buy", zap.Int("lines", len(order)))

	return s.client.Send(packets.EncodeBuy(order))
}

// Sell asks to sell one.
func (s *InGameState) Sell(order []packets.ShopOrder) error {
	if s.client == nil || len(order) == 0 {
		return nil
	}

	trace.Emit(trace.HUD, "shop-sell", zap.Int("lines", len(order)))

	return s.client.Send(packets.EncodeSell(order))
}

// CloseShop puts the counter away and tells the server the conversation is
// over.
//
// It has to be told. A shop is a conversation as far as the server is
// concerned — npc_click refuses to start another while one is open, and says
// so in its log — so a counter closed without a word leaves the shopkeeper
// holding it and every click on them afterwards does nothing at all.
//
// Whoever opened it: the question names the shopkeeper, and a shop that does
// not ask which way round names nobody, so the one we walked up to is kept.
func (s *InGameState) CloseShop() {
	if !s.shop.Open() {
		return
	}

	npc := s.shop.NPC
	if npc == 0 {
		npc = s.talkingTo
	}

	trace.Emit(trace.HUD, "shop-close",
		zap.Int("mode", int(s.shop.Mode)), zap.Uint32("npc", npc))

	s.shop = Shop{}

	if s.client == nil || npc == 0 {
		return
	}

	if err := s.client.Send(packets.CloseDialogPacket(npc)); err != nil {
		logger.Warn("could not tell the shop we had finished", zap.Error(err))
	}
}

// handleBuyResult and handleSellResult report what became of an order.
//
// The list is not asked for again. A sale that went through leaves the shelf
// as it was — an NPC's stock does not run down — and the bag arrives on its
// own in the packets that add and remove items.
func (s *InGameState) handleBuyResult(data []byte) error {
	return s.shopResult(data, "bought")
}

func (s *InGameState) handleSellResult(data []byte) error {
	return s.shopResult(data, "sold")
}

func (s *InGameState) shopResult(data []byte, what string) error {
	result, ok := packets.DecodeShopResult(data)
	if !ok {
		logger.Warn("short shop result packet", zap.Int("len", len(data)))

		return nil
	}

	trace.Emit(trace.HUD, "shop-result",
		zap.String("what", what), zap.Uint8("result", result))

	if refusal := packets.ShopRefusal(result); refusal != "" {
		s.chat.AddLocal(ChatError, refusal)
	}

	return nil
}
