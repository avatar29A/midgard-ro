package states

import (
	"go.uber.org/zap"

	"github.com/Faultbox/midgard-ro/internal/game/entity"
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

// Portrait is the character's own sprite, for the equipment window to show
// what it is dressing.
//
// The idle facing the viewer, not whatever pose the character is in on the
// map: a character caught mid-swing in a window that is not about the fight
// reads as a fault. The size is the baked frame's own, so the interface can
// fit it without guessing the sprite's proportions.
func (s *InGameState) Portrait() (texture uint32, w, h float32) {
	if s.playerRender == nil {
		return 0, 0, 0
	}

	return s.playerRender.PortraitFrame(entity.ActionIdle, portraitFacing, 0)
}

// portraitFacing is which of the eight facings the portrait uses.
//
// South, which in RO's sprite art is index zero and the one drawn face on.
// The server's own compass runs the other way round the circle — north is its
// zero — and taking that one stood the character with its back to the window.
const portraitFacing = entity.DirS

// ShowEquipment tells the server whether other players may look at what this
// character is wearing, which is the checkbox on the equipment window.
//
// Nothing is remembered here. The server owns the setting — it is part of the
// character — and it answers with ZC_CONFIG, which is what moves the box.
func (s *InGameState) ShowEquipment(on bool) error {
	if s.client == nil {
		return nil
	}

	trace.Emit(trace.HUD, "show-equipment", zap.Bool("on", on))

	return s.client.Send(packets.EncodeConfig(packets.ConfigShowEquipment, on))
}

// handleConfigNotify takes the server's word on the equipment switch.
func (s *InGameState) handleConfigNotify(data []byte) error {
	on, ok := packets.DecodeConfigNotify(data)
	if !ok {
		logger.Warn("short config notify", zap.Int("len", len(data)))

		return nil
	}

	s.showEquipment = on

	trace.Emit(trace.HUD, "show-equipment-is", zap.Bool("on", on))

	return nil
}

// ShowEquipmentOn reports whether other players may look at what this
// character is wearing.
func (s *InGameState) ShowEquipmentOn() bool { return s.showEquipment }
