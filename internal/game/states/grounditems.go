package states

import (
	"fmt"

	"go.uber.org/zap"

	"github.com/Faultbox/midgard-ro/internal/game/entity"
	"github.com/Faultbox/midgard-ro/internal/game/items"
	"github.com/Faultbox/midgard-ro/internal/logger"
	"github.com/Faultbox/midgard-ro/internal/network/packets"
	"github.com/Faultbox/midgard-ro/internal/trace"
)

// subCellsPerCell is how finely the server places an item inside its cell.
//
// rAthena picks subx/suby in 0..15 when an item lands, so two things dropped
// on the same cell come to rest a little apart instead of on the same point.
const subCellsPerCell = 16

// handleGroundItemEntry takes an item that was already lying there when we
// arrived. The server sends one of these per item, to us alone, as the map
// loads.
func (s *InGameState) handleGroundItemEntry(data []byte) error {
	item, ok := packets.DecodeGroundItemEntry(data)
	if !ok {
		logger.Warn("short ground item entry", zap.Int("len", len(data)))

		return nil
	}

	s.addGroundItem(item, "entry")

	return nil
}

// handleGroundItemFall takes an item dropping now. This one is broadcast to
// everyone who can see the cell, so it arrives for other people's drops and
// for monster loot as well as for our own.
func (s *InGameState) handleGroundItemFall(data []byte) error {
	item, ok := packets.DecodeGroundItemFall(data)
	if !ok {
		logger.Warn("short ground item drop", zap.Int("len", len(data)))

		return nil
	}

	s.addGroundItem(item, "fall")

	return nil
}

// addGroundItem puts an item on the map as an entity.
//
// Ground items are entities rather than a list of their own so that they sort
// into the scene by depth with everything else, answer the pointer through the
// same ray cast, and fade out the same way. What they are not is units: they
// never walk, and nothing about them arrives through the unit spawn packets.
func (s *InGameState) addGroundItem(item packets.GroundItem, how string) {
	if s.entityManager == nil || item.ID == 0 {
		return
	}

	e := s.entityManager.Get(item.ID)
	if e == nil {
		e = entity.NewEntity(item.ID, entity.TypeItem)
		s.entityManager.Add(e)
	}
	e.CancelLeaving()

	e.Type = entity.TypeItem
	e.ItemID = item.ItemID
	e.Amount = item.Amount
	e.Name = items.Name(item.ItemID)
	e.ShowName = false
	e.ShowHP = false

	if e.Body == nil {
		x, z := entity.CellToWorld(item.X, item.Y)
		e.Body = entity.NewCharacter(x, 0, z)
	}
	placeGroundItem(e.Body, item)

	trace.Emit(trace.HUD, "ground-item",
		zap.String("how", how),
		zap.Uint32("id", item.ID),
		zap.Uint32("item", item.ItemID),
		zap.String("name", e.Name),
		zap.Int("amount", item.Amount),
		zap.Int("x", item.X), zap.Int("y", item.Y))
}

// placeGroundItem puts an item on its cell, then nudges it by the sub-cell
// offset the server chose.
//
// SetCell is what samples the ground height, so it goes first and the nudge
// keeps the height it found. Within one cell the terrain does not move enough
// for that to show.
func placeGroundItem(body *entity.Character, item packets.GroundItem) {
	body.SetCell(item.X, item.Y)

	x, y, z := body.Position()
	offX := (float32(item.SubX)/subCellsPerCell - 0.5) * entity.CellSize
	offZ := (float32(item.SubY)/subCellsPerCell - 0.5) * entity.CellSize
	body.SetPosition(x+offX, y, z+offZ)
}

// handleGroundItemGone takes an item off the map — picked up by someone, or
// expired. The server says nothing about which, and neither do we.
func (s *InGameState) handleGroundItemGone(data []byte) error {
	id, ok := packets.DecodeGroundItemGone(data)
	if !ok {
		logger.Warn("short ground item disappear", zap.Int("len", len(data)))

		return nil
	}

	if s.entityManager == nil {
		return nil
	}

	if e := s.entityManager.Get(id); e != nil && e.Type == entity.TypeItem {
		trace.Emit(trace.HUD, "ground-item-gone", zap.Uint32("id", id))
		removeUnit(s.entityManager, id)
	}

	return nil
}

// handlePickupAck applies the server's answer to picking something up.
//
// Nothing is added to the inventory here beyond the count. A refusal leaves
// the item where it is: the server keeps it on the ground and will not send a
// disappear for it, so showing it gone would leave a hole that only a map
// change would fill.
func (s *InGameState) handlePickupAck(data []byte) error {
	ack, ok := packets.DecodeItemPickupAck(data)
	if !ok {
		logger.Warn("short pickup ack", zap.Int("len", len(data)))

		return nil
	}

	if !ack.OK() {
		trace.Emit(trace.HUD, "pickup-refused",
			zap.Uint32("item", ack.ItemID), zap.Uint8("result", ack.Result))
		logger.Info("could not pick that up",
			zap.String("name", items.Name(ack.ItemID)),
			zap.Uint8("result", ack.Result))

		return nil
	}

	s.addToInventory(ack)

	trace.Emit(trace.HUD, "pickup",
		zap.Int("index", ack.Index),
		zap.Uint32("item", ack.ItemID),
		zap.Int("amount", ack.Amount))

	return nil
}

// addToInventory folds a picked-up item into the inventory we already hold.
//
// The slot decides identity, not the item id: a stack that grows comes back
// with the slot it already had, and a new item comes back with one we have
// never seen. Matching on the item id instead would merge two stacks of the
// same thing that the server keeps apart.
func (s *InGameState) addToInventory(ack packets.PickupAck) {
	for i := range s.inventory {
		if s.inventory[i].Index != ack.Index {
			continue
		}

		s.inventory[i].Count += ack.Amount

		return
	}

	s.inventory = append(s.inventory, packets.InventoryItem{
		Index: ack.Index,
		ID:    ack.ItemID,
		Count: ack.Amount,
	})
}

// pickupRange is how close the server needs us to be, in cells.
//
// rAthena's pc_takeitem checks check_distance_bl(fitem, sd, 2), so two cells
// in either direction. Asking from further away is refused, and the refusal
// is all the server does — it will not walk us there.
const pickupRange = 2

// pickupIdleGiveUpMs is how long the character may stand still on the way to
// an item before the pick-up is abandoned.
//
// Not cancelled the moment the character is not walking: a walk is
// acknowledged one path at a time, so there is a gap between steps where
// nothing is moving and the next acknowledgement is still in flight. This is
// comfortably longer than that gap and shorter than anyone's patience.
const pickupIdleGiveUpMs = 600

// PickUpItem picks up a ground item, walking to it first if it is out of
// reach.
//
// Nothing is changed locally either way. The item stays on screen until the
// server says it is gone, because this is a request the server refuses often
// — out of range, too heavy, someone else's loot — and clearing it early
// would show the player a pick-up that did not happen.
func (s *InGameState) PickUpItem(e *entity.Entity) {
	if e == nil || e.Type != entity.TypeItem || s.client == nil || e.Body == nil {
		return
	}

	if s.withinPickupRange(e) {
		s.sendPickUp(e)

		return
	}

	// Out of reach: walk to it and pick it up on arrival. The server answers
	// a distant pick-up with a refusal rather than a walk, so the walking is
	// ours to do.
	itemX, itemY := e.Body.CurrentCell()

	trace.Emit(trace.HUD, "pickup-approach",
		zap.Uint32("id", e.ID), zap.String("name", e.Name),
		zap.Int("x", itemX), zap.Int("y", itemY))

	if err := s.RequestMove(itemX, itemY); err != nil {
		logger.Warn("could not walk to that item", zap.Error(err))

		return
	}

	s.pendingPickup = e.ID
	s.pendingPickupIdleMs = 0
}

// sendPickUp reaches for an item and asks for it.
func (s *InGameState) sendPickUp(e *entity.Entity) {
	s.reachFor(e)

	trace.Emit(trace.HUD, "pickup-request",
		zap.Uint32("id", e.ID), zap.String("name", e.Name))

	if err := s.client.Send(packets.EncodePickUpItem(e.ID)); err != nil {
		logger.Warn("pick up failed", zap.Error(err))
	}
}

// withinPickupRange reports whether the server would accept a pick-up from
// where the character is standing.
func (s *InGameState) withinPickupRange(e *entity.Entity) bool {
	if s.player == nil || e == nil || e.Body == nil {
		return false
	}

	itemX, itemY := e.Body.CurrentCell()
	playerX, playerY := s.player.CurrentCell()

	return abs(itemX-playerX) <= pickupRange && abs(itemY-playerY) <= pickupRange
}

// updatePendingPickup finishes a pick-up the character had to walk to.
//
// Given up on if the item goes — someone else was closer — or if the
// character stops short of it, which is what an unreachable cell looks like
// from here. Neither is worth a message: the item is still on the ground and
// still clickable.
func (s *InGameState) updatePendingPickup(deltaMs float32, walking bool) {
	if s.pendingPickup == 0 || s.entityManager == nil {
		return
	}

	e := s.entityManager.Get(s.pendingPickup)
	if e == nil || e.Type != entity.TypeItem {
		trace.Emit(trace.HUD, "pickup-gone", zap.Uint32("id", s.pendingPickup))
		s.pendingPickup = 0

		return
	}

	if s.withinPickupRange(e) {
		s.pendingPickup = 0
		s.sendPickUp(e)

		return
	}

	if walking {
		s.pendingPickupIdleMs = 0

		return
	}

	s.pendingPickupIdleMs += deltaMs
	if s.pendingPickupIdleMs >= pickupIdleGiveUpMs {
		trace.Emit(trace.HUD, "pickup-unreachable", zap.Uint32("id", s.pendingPickup))
		s.pendingPickup = 0
	}
}

// reachFor turns the character towards an item and plays the pick-up motion.
//
// Both happen on the click rather than on the server's answer, because both
// are feedback rather than state: nothing the server owns is changed, and a
// refused pick-up costs one motion and a turn nobody will mistake for an item
// arriving. Waiting instead would put the motion a round trip after the
// click, which reads as the click having missed.
//
// An item on the character's own cell leaves the facing alone — there is no
// direction to face, and DirectionFromCellDelta says so with -1.
func (s *InGameState) reachFor(e *entity.Entity) {
	if s.player == nil || e.Body == nil {
		return
	}

	itemX, itemY := e.Body.CurrentCell()
	playerX, playerY := s.player.CurrentCell()

	if dir := entity.DirectionFromCellDelta(itemX-playerX, itemY-playerY); dir >= 0 {
		s.player.Direction = dir
	}

	s.player.PlayPickup()
}

// DropItem asks to put some of an inventory stack on the ground.
//
// As with picking up, nothing changes locally: the drop arrives back as a
// ground item and an inventory update, and acting first would show an item
// leaving a bag the server still has it in.
func (s *InGameState) DropItem(index, amount int) error {
	if amount <= 0 {
		return nil
	}

	trace.Emit(trace.HUD, "drop-item",
		zap.Int("index", index), zap.Int("amount", amount))

	return s.client.Send(packets.EncodeDropItem(index, amount))
}

// handleDropAck takes dropped items out of the inventory.
//
// The count is how many left the bag rather than how many remain, so this
// subtracts where the use acknowledgement assigns. A count of zero is the
// server refusing, and changes nothing.
func (s *InGameState) handleDropAck(data []byte) error {
	ack, ok := packets.DecodeDropAck(data)
	if !ok {
		logger.Warn("short drop ack", zap.Int("len", len(data)))

		return nil
	}

	if ack.Count == 0 {
		trace.Emit(trace.HUD, "drop-refused", zap.Int("index", ack.Index))
		logger.Info("that item cannot be dropped here")

		return nil
	}

	for i := range s.inventory {
		if s.inventory[i].Index != ack.Index {
			continue
		}

		s.inventory[i].Count -= ack.Count
		if s.inventory[i].Count <= 0 {
			s.inventory = append(s.inventory[:i], s.inventory[i+1:]...)
		}

		break
	}

	trace.Emit(trace.HUD, "drop-ack",
		zap.Int("index", ack.Index), zap.Int("count", ack.Count))

	return nil
}

// HoverLabel is a name to draw over something in the world, already projected
// into viewport pixels.
type HoverLabel struct {
	Text             string
	ScreenX, ScreenY float32
}

// SetHoverEntity records what the pointer is over this frame, or nil.
//
// The cursor already works this out once a frame; keeping the answer means the
// label costs no second ray cast, and means the label and the cursor can never
// disagree about what is under the pointer.
func (s *InGameState) SetHoverEntity(e *entity.Entity) {
	s.hoverEntity = e
}

// HoverItemLabel is the name to show for a ground item under the pointer.
//
// Only ground items get one. Units carry their names above their heads all the
// time, which is a different thing drawn from a different place; an item is
// unlabelled until you point at it, the way the original does it.
func (s *InGameState) HoverItemLabel(viewportW, viewportH float32) (HoverLabel, bool) {
	e := s.hoverEntity
	if e == nil || e.Type != entity.TypeItem || e.Body == nil || e.Name == "" {
		return HoverLabel{}, false
	}

	x, y := s.projectToScreen(e.Body.RenderX, e.Body.RenderY, e.Body.RenderZ,
		viewportW, viewportH)
	if x < 0 {
		return HoverLabel{}, false
	}

	text := e.Name
	if e.Amount > 1 {
		text = fmt.Sprintf("%s %d ea.", e.Name, e.Amount)
	}

	return HoverLabel{Text: text, ScreenX: x, ScreenY: y}, true
}
