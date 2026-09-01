package states

import (
	"go.uber.org/zap"

	"github.com/Faultbox/midgard-ro/internal/logger"
	"github.com/Faultbox/midgard-ro/internal/network/packets"
	"github.com/Faultbox/midgard-ro/internal/trace"
)

// Wearing and taking off equipment.
//
// Nothing here changes the inventory before the server has answered. Both
// requests can be refused — the level is too low, the job is wrong, a status
// forbids it — and rAthena answers every one of them, so an optimistic client
// would show a hat worn that the next item list quietly takes back off.
//
// Swapping needs no code at all, which is worth saying because it looks like
// it should: pc_equipitem unequips whatever holds the slot first, and that
// path sends its own take-off ack. So a sword replacing a sword arrives as
// two packets, and handling each one on its own gets the swap right.

// EquipItemAt asks to wear an inventory item in a particular place on the
// body.
//
// The mask narrows what the item itself allows: an accessory names both
// accessory slots and a dagger both hands, and dropping one on the left-hand
// oval means that slot rather than whichever the server would have chosen.
// The server still has the last word — it intersects what we ask with what
// the item allows — so a mask that makes no sense is refused rather than
// obeyed.
//
// A zero mask means "wherever it goes", which is what a double click means.
func (s *InGameState) EquipItemAt(index int, mask uint32) error {
	for _, item := range s.inventory {
		if item.Index != index {
			continue
		}

		position := item.EquipPositions
		if mask != 0 {
			position &= mask
		}

		if position == 0 {
			s.chat.AddLocal(ChatError, "That cannot be worn there.")

			return nil
		}

		trace.Emit(trace.HUD, "equip-item",
			zap.Int("index", index), zap.Uint32("position", position))

		return s.client.Send(packets.EncodeEquipItem(index, position))
	}

	return nil
}

// handleEquipAck applies the server's answer to wearing something.
func (s *InGameState) handleEquipAck(data []byte) error {
	ack, ok := packets.DecodeEquipAck(data)
	if !ok {
		logger.Warn("short equip ack", zap.Int("len", len(data)))

		return nil
	}

	if !ack.OK() {
		// Said in the chat box rather than only logged: a double click that
		// does nothing visible reads as a broken interface, and the reason is
		// the answer to why it did nothing.
		s.chat.AddLocal(ChatError, ack.Reason())

		trace.Emit(trace.HUD, "equip-refused",
			zap.Int("index", ack.Index), zap.Uint8("result", ack.Result))

		return nil
	}

	// The place the server chose, not the place that was asked for.
	s.setWearState(ack.Index, ack.Position)

	trace.Emit(trace.HUD, "equip-ack",
		zap.Int("index", ack.Index), zap.Uint32("position", ack.Position))

	return nil
}

// handleUnequipAck applies the server's answer to taking something off.
func (s *InGameState) handleUnequipAck(data []byte) error {
	ack, ok := packets.DecodeUnequipAck(data)
	if !ok {
		logger.Warn("short unequip ack", zap.Int("len", len(data)))

		return nil
	}

	if !ack.OK {
		s.chat.AddLocal(ChatError, "You cannot take that off.")

		trace.Emit(trace.HUD, "unequip-refused", zap.Int("index", ack.Index))

		return nil
	}

	s.setWearState(ack.Index, 0)

	trace.Emit(trace.HUD, "unequip-ack",
		zap.Int("index", ack.Index), zap.Uint32("position", ack.Position))

	return nil
}

// setWearState files an inventory slot under where it is now worn.
func (s *InGameState) setWearState(index int, position uint32) {
	for i := range s.inventory {
		if s.inventory[i].Index != index {
			continue
		}

		s.inventory[i].WearState = position
		s.inventory[i].Equipped = position != 0

		return
	}
}

// Equipment returns what is worn, keyed by the slot it is worn in.
//
// Keyed by slot rather than returned as a list because that is the question
// the equipment window asks: it has ten fixed places and wants to know what is
// in each. An item filling two slots — a two-handed weapon holds both hands —
// appears under each of them, which is what the window should draw.
func (s *InGameState) Equipment() map[uint32]packets.InventoryItem {
	worn := make(map[uint32]packets.InventoryItem)

	for _, item := range s.inventory {
		if item.WearState == 0 {
			continue
		}

		for _, slot := range packets.EquipSlots {
			if item.WearState&slot != 0 {
				worn[slot] = item
			}
		}
	}

	return worn
}
