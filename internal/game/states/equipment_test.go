package states

import (
	"encoding/binary"
	"testing"

	"github.com/Faultbox/midgard-ro/internal/game/entity"
	"github.com/Faultbox/midgard-ro/internal/network/packets"
)

// wearAckPacket and takeoffAckPacket build what the server sends back.
func wearAckPacket(index int, position uint32, result uint8) []byte {
	b := make([]byte, 11)
	binary.LittleEndian.PutUint16(b, packets.ZC_REQ_WEAR_EQUIP_ACK)
	binary.LittleEndian.PutUint16(b[2:], uint16(index))
	binary.LittleEndian.PutUint32(b[4:], position)
	b[10] = result

	return b
}

func takeoffAckPacket(index int, position uint32, flag uint8) []byte {
	b := make([]byte, 9)
	binary.LittleEndian.PutUint16(b, packets.ZC_REQ_TAKEOFF_EQUIP_ACK)
	binary.LittleEndian.PutUint16(b[2:], uint16(index))
	binary.LittleEndian.PutUint32(b[4:], position)
	b[8] = flag

	return b
}

// withInventory is a state carrying items, without a connection.
func withInventory(items ...packets.InventoryItem) *InGameState {
	s := &InGameState{}
	s.inventory = items

	return s
}

// TestEquipAckFilesTheItemWhereTheServerPutIt: the ack's position wins over
// what was asked for. An accessory names both slots and the server chooses
// one, so filing it under the request would put a ring in two places.
func TestEquipAckFilesTheItemWhereTheServerPutIt(t *testing.T) {
	s := withInventory(packets.InventoryItem{
		Index:          5,
		ID:             2611,
		EquipPositions: packets.EQP_ACC_R | packets.EQP_ACC_L,
	})

	_ = s.handleEquipAck(wearAckPacket(5, packets.EQP_ACC_L, packets.EquipAckOK))

	worn := s.Equipment()
	if _, ok := worn[packets.EQP_ACC_R]; ok {
		t.Error("the ring went into the slot that was asked for, not the one it was put in")
	}
	if item, ok := worn[packets.EQP_ACC_L]; !ok || item.Index != 5 {
		t.Errorf("the left accessory slot holds %+v, want the ring", item)
	}
	if !s.inventory[0].Equipped {
		t.Error("a worn item is not marked worn in the inventory")
	}
}

// TestEquipRefusalChangesNothing: the server keeps the item, and showing it
// worn would be a lie the next item list quietly takes back.
func TestEquipRefusalChangesNothing(t *testing.T) {
	s := withInventory(packets.InventoryItem{
		Index:          5,
		EquipPositions: packets.EQP_ARMOR,
	})

	_ = s.handleEquipAck(wearAckPacket(5, packets.EQP_ARMOR, packets.EquipAckFailLevel))

	if len(s.Equipment()) != 0 {
		t.Error("a refused item is shown worn")
	}
	if s.inventory[0].Equipped {
		t.Error("a refused item is marked worn in the inventory")
	}

	// The reason is said, not only logged: a double click that does nothing
	// visible reads as a broken interface.
	if s.chat.Len() == 0 {
		t.Error("a refusal said nothing in the chat box")
	}
}

// TestUnequipAckTakesItOff: flag zero is success at this packetver.
func TestUnequipAckTakesItOff(t *testing.T) {
	s := withInventory(packets.InventoryItem{
		Index:     5,
		WearState: packets.EQP_SHOES,
		Equipped:  true,
	})

	_ = s.handleUnequipAck(takeoffAckPacket(5, packets.EQP_SHOES, 0))

	if len(s.Equipment()) != 0 {
		t.Error("the shoes are still worn after coming off")
	}
	if s.inventory[0].Equipped {
		t.Error("the inventory still marks them worn")
	}
}

// TestUnequipRefusalLeavesItWorn: flag one is a refusal, and the item stays
// where it was.
func TestUnequipRefusalLeavesItWorn(t *testing.T) {
	s := withInventory(packets.InventoryItem{
		Index:     5,
		WearState: packets.EQP_SHOES,
		Equipped:  true,
	})

	_ = s.handleUnequipAck(takeoffAckPacket(5, packets.EQP_SHOES, 1))

	if _, ok := s.Equipment()[packets.EQP_SHOES]; !ok {
		t.Error("a refused unequip took the item off anyway")
	}
}

// TestSwapIsTwoPackets: rAthena takes the old item off before putting the new
// one on, and each answer is handled on its own. Nothing here has to know
// that a swap happened.
func TestSwapIsTwoPackets(t *testing.T) {
	s := withInventory(
		packets.InventoryItem{Index: 1, ID: 1201, WearState: packets.EQP_HAND_R, Equipped: true},
		packets.InventoryItem{Index: 2, ID: 1202, EquipPositions: packets.EQP_HAND_R},
	)

	_ = s.handleUnequipAck(takeoffAckPacket(1, packets.EQP_HAND_R, 0))
	_ = s.handleEquipAck(wearAckPacket(2, packets.EQP_HAND_R, packets.EquipAckOK))

	worn := s.Equipment()
	if item, ok := worn[packets.EQP_HAND_R]; !ok || item.Index != 2 {
		t.Errorf("the hand holds %+v, want the second dagger", item)
	}
	if s.inventory[0].Equipped {
		t.Error("the replaced dagger is still marked worn, so the bag shows two")
	}
}

// TestTwoHandedWeaponFillsBothHands: an item worn across two slots is shown in
// each of them, which is what the window should draw.
func TestTwoHandedWeaponFillsBothHands(t *testing.T) {
	s := withInventory(packets.InventoryItem{
		Index:     3,
		ID:        1101,
		WearState: packets.EQP_HAND_R | packets.EQP_HAND_L,
		Equipped:  true,
	})

	worn := s.Equipment()
	if len(worn) != 2 {
		t.Fatalf("a two-hander fills %d slots, want 2", len(worn))
	}
	if worn[packets.EQP_HAND_R].Index != 3 || worn[packets.EQP_HAND_L].Index != 3 {
		t.Error("the two hands do not both hold the weapon")
	}
}

// TestSittingBlocksWalking: unit_can_move is false while a player sits, so a
// walk request would never be acknowledged — and the client, left waiting for
// one, slid a seated character across the ground. Turning is all a click can
// mean here.
func TestSittingBlocksWalking(t *testing.T) {
	s := &InGameState{}
	s.player = &entity.Character{Sitting: true}
	s.player.WorldX, s.player.WorldZ = entity.CellToWorld(100, 100)

	before := s.player.Direction

	if err := s.RequestMove(100, 105); err != nil {
		t.Fatalf("RequestMove while seated returned %v", err)
	}

	if s.hasDest {
		t.Error("a seated character was given somewhere to walk to")
	}
	if s.player.Direction == before {
		t.Error("a seated character did not turn toward the click")
	}
}
