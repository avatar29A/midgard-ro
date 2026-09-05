package states

import (
	"encoding/binary"
	"testing"

	"github.com/Faultbox/midgard-ro/internal/network/packets"
)

// dealTypePacket builds a ZC_SELECT_DEALTYPE the way the server does.
func dealTypePacket(npc uint32) []byte {
	pkt := make([]byte, 6)
	binary.LittleEndian.PutUint16(pkt, packets.ZC_SELECT_DEALTYPE)
	binary.LittleEndian.PutUint32(pkt[2:], npc)

	return pkt
}

// TestAShopHoldsTheWorldStill: the counter is a conversation with somebody
// standing in front of you. Walking off mid-purchase leaves the window open
// over a character halfway across the map, and the server refuses the order
// for being too far away — which reads as the shop being broken.
func TestAShopHoldsTheWorldStill(t *testing.T) {
	for _, mode := range []ShopMode{ShopChoosing, ShopBuying, ShopSelling} {
		s := &InGameState{shop: Shop{Mode: mode}}

		if !s.shop.Open() {
			t.Errorf("mode %d does not read as open", mode)
		}
	}

	if (&InGameState{}).shop.Open() {
		t.Error("no shop reads as open")
	}
}

// TestTheQuestionIsRememberedByWho: answering it names the shopkeeper, and an
// answer sent to nobody is one the server drops.
func TestTheQuestionIsRememberedByWho(t *testing.T) {
	s := &InGameState{}

	if err := s.handleDealType(dealTypePacket(110001763)); err != nil {
		t.Fatalf("handling the question: %v", err)
	}

	if s.shop.Mode != ShopChoosing {
		t.Errorf("the shop is in mode %d, want it asking", s.shop.Mode)
	}
	if s.shop.NPC != 110001763 {
		t.Errorf("the shopkeeper is %d", s.shop.NPC)
	}

	// A short packet leaves it shut rather than opening on nobody.
	shut := &InGameState{}
	if err := shut.handleDealType(dealTypePacket(1)[:4]); err != nil {
		t.Fatalf("handling a short question: %v", err)
	}
	if shut.shop.Open() {
		t.Errorf("a short packet opened a shop: %+v", shut.shop)
	}
}

// TestClosingForgetsBothSides: a counter reopened on the other side of itself
// would show the last one's rows until the list arrived.
func TestClosingForgetsBothSides(t *testing.T) {
	s := &InGameState{shop: Shop{
		Mode:   ShopBuying,
		Buying: []packets.ShopItem{{ID: 501}},
	}}

	s.CloseShop()

	if s.shop.Open() || len(s.shop.Buying) != 0 {
		t.Errorf("the counter is still there: %+v", s.shop)
	}
}

// TestOnlyOneSideAtATime: the lists replace each other rather than piling up,
// so a shop asked for both ways round does not draw the first one's rows
// under the second one's title.
func TestOnlyOneSideAtATime(t *testing.T) {
	// Talking to somebody, since a list that belongs to nobody is dropped.
	s := &InGameState{talkingTo: 110001763}

	buy := make([]byte, 4+19)
	binary.LittleEndian.PutUint16(buy, packets.ZC_PC_PURCHASE_ITEMLIST)
	binary.LittleEndian.PutUint16(buy[2:], uint16(len(buy)))
	binary.LittleEndian.PutUint32(buy[4:], 501)

	if err := s.handleShopItems(buy); err != nil {
		t.Fatalf("handling a buy list: %v", err)
	}
	if s.shop.Mode != ShopBuying || len(s.shop.Buying) != 1 {
		t.Fatalf("the buy list came out as %+v", s.shop)
	}

	sell := make([]byte, 4+10)
	binary.LittleEndian.PutUint16(sell, packets.ZC_PC_SELL_ITEMLIST)
	binary.LittleEndian.PutUint16(sell[2:], uint16(len(sell)))
	binary.LittleEndian.PutUint16(sell[4:], 7)

	if err := s.handleSellItems(sell); err != nil {
		t.Fatalf("handling a sell list: %v", err)
	}

	if s.shop.Mode != ShopSelling || len(s.shop.Selling) != 1 {
		t.Errorf("the sell list came out as %+v", s.shop)
	}
	if len(s.shop.Buying) != 0 {
		t.Errorf("the buy list is still there under the sell one: %+v", s.shop.Buying)
	}
}

// TestALateListDoesNotReopenTheCounter: closing and the shop's answer cross in
// the post whenever the two are close together, and the answer would reopen a
// counter the player has just shut. We are the ones who said we had finished.
func TestALateListDoesNotReopenTheCounter(t *testing.T) {
	s := &InGameState{talkingTo: 110001763, shop: Shop{Mode: ShopChoosing, NPC: 110001763}}

	s.CloseShop()

	if s.talkingTo != 0 {
		t.Errorf("still talking to %d after closing", s.talkingTo)
	}

	buy := make([]byte, 4+19)
	binary.LittleEndian.PutUint16(buy, packets.ZC_PC_PURCHASE_ITEMLIST)
	binary.LittleEndian.PutUint16(buy[2:], uint16(len(buy)))
	binary.LittleEndian.PutUint32(buy[4:], 501)

	if err := s.handleShopItems(buy); err != nil {
		t.Fatalf("handling a late list: %v", err)
	}

	if s.shop.Open() {
		t.Errorf("a list that arrived late reopened the counter: %+v", s.shop)
	}
}

// shopResultPacket builds a ZC_PC_SELL_RESULT the way the server does.
func shopResultPacket(result uint8) []byte {
	pkt := make([]byte, 3)
	binary.LittleEndian.PutUint16(pkt, packets.ZC_PC_SELL_RESULT)
	pkt[2] = result

	return pkt
}

// TestADealClosesTheCounter: one deal is what the window is for. The original
// takes it away as soon as the server answers, and a window left standing
// after a sale reads as a sale that did not happen.
func TestADealClosesTheCounter(t *testing.T) {
	s := &InGameState{shop: Shop{Mode: ShopSelling, NPC: 7}, talkingTo: 7}

	if err := s.handleSellResult(shopResultPacket(packets.ShopOK)); err != nil {
		t.Fatalf("handling the result: %v", err)
	}

	if s.shop.Open() {
		t.Errorf("the counter is still open in mode %d", s.shop.Mode)
	}
	if s.talkingTo != 0 {
		t.Errorf("still talking to %d", s.talkingTo)
	}

	lines := s.chat.Lines()
	if len(lines) != 1 || lines[0].Kind != ChatNotice {
		t.Fatalf("the chat says %+v, want one notice", lines)
	}
}

// TestARefusalClosesItToo, and says why.
//
// The same as the original, which does not tell the two apart: what went
// wrong is said in the chat, where everything else a shop says to us is said,
// rather than by a window left up with no sign on it of what happened.
func TestARefusalClosesItToo(t *testing.T) {
	s := &InGameState{shop: Shop{Mode: ShopSelling, NPC: 7}, talkingTo: 7}

	if err := s.handleSellResult(shopResultPacket(packets.ShopNoZeny)); err != nil {
		t.Fatalf("handling the result: %v", err)
	}

	if s.shop.Open() {
		t.Error("the counter is still open after a refusal")
	}

	lines := s.chat.Lines()
	if len(lines) != 1 || lines[0].Kind != ChatError {
		t.Fatalf("the chat says %+v, want one error", lines)
	}
}
