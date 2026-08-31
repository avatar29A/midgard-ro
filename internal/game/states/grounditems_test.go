package states

import (
	"testing"

	"github.com/Faultbox/midgard-ro/internal/game/entity"
)

// itemAt puts a ground item entity on a cell, the way addGroundItem would.
func itemAt(id uint32, cellX, cellY int) *entity.Entity {
	e := entity.NewEntity(id, entity.TypeItem)
	x, z := entity.CellToWorld(cellX, cellY)
	e.Body = entity.NewCharacter(x, 0, z)
	e.Body.SetCell(cellX, cellY)

	return e
}

// TestWithinPickupRange follows the server's own check: rAthena accepts a
// pick-up from two cells away and refuses from three, so the client has to
// agree or it will either walk when it need not or ask and be refused.
func TestWithinPickupRange(t *testing.T) {
	s := &InGameState{player: entity.NewCharacter(0, 0, 0)}
	s.player.SetCell(100, 100)

	tests := []struct {
		name       string
		cellX      int
		cellY      int
		wantInside bool
	}{
		{"same cell", 100, 100, true},
		{"one across", 101, 100, true},
		{"two across", 102, 100, true},
		{"two diagonally", 102, 102, true},
		{"three across", 103, 100, false},
		{"three down", 100, 103, false},
		{"two across and three down", 102, 103, false},
		{"behind, within range", 98, 98, true},
		{"behind, out of range", 97, 100, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := s.withinPickupRange(itemAt(1, tt.cellX, tt.cellY)); got != tt.wantInside {
				t.Errorf("withinPickupRange(%d,%d) = %v, want %v",
					tt.cellX, tt.cellY, got, tt.wantInside)
			}
		})
	}
}

// TestWithinPickupRangeWithoutPlayer: no player is not in range of anything,
// rather than a nil dereference.
func TestWithinPickupRangeWithoutPlayer(t *testing.T) {
	var s InGameState
	if s.withinPickupRange(itemAt(1, 0, 0)) {
		t.Error("in range with no player")
	}
	if (&InGameState{player: entity.NewCharacter(0, 0, 0)}).withinPickupRange(nil) {
		t.Error("in range of no item")
	}
}

// TestPendingPickupGivesUpWhenTheItemGoes: someone else picked it up while we
// were walking, so there is nothing left to ask for.
func TestPendingPickupGivesUpWhenTheItemGoes(t *testing.T) {
	s := &InGameState{
		player:        entity.NewCharacter(0, 0, 0),
		entityManager: entity.NewManager(),
		pendingPickup: 42,
	}

	s.updatePendingPickup(16, true)

	if s.pendingPickup != 0 {
		t.Error("still waiting for an item that is not on the map")
	}
}

// TestPendingPickupGivesUpAfterStandingStill: the character stopped short of
// the item, which is what an unreachable cell looks like from here. Walking
// resets the clock, because a walk pauses between acknowledged paths.
func TestPendingPickupGivesUpAfterStandingStill(t *testing.T) {
	m := entity.NewManager()
	m.Add(itemAt(42, 200, 200))

	s := &InGameState{
		player:        entity.NewCharacter(0, 0, 0),
		entityManager: m,
		pendingPickup: 42,
	}
	s.player.SetCell(100, 100)

	// Walking, however long for, never gives up.
	for i := 0; i < 100; i++ {
		s.updatePendingPickup(16, true)
	}
	if s.pendingPickup != 42 {
		t.Fatal("gave up while still walking")
	}

	// Standing still briefly is the gap between paths, not a stop.
	s.updatePendingPickup(pickupIdleGiveUpMs/2, false)
	if s.pendingPickup != 42 {
		t.Error("gave up during the gap between two acknowledged paths")
	}

	s.updatePendingPickup(pickupIdleGiveUpMs, false)
	if s.pendingPickup != 0 {
		t.Error("still waiting after standing still well past the limit")
	}
}
