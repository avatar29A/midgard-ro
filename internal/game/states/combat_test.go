package states

import (
	"testing"

	"github.com/Faultbox/midgard-ro/internal/game/entity"
)

// mobAt puts a monster on a cell.
func mobAt(id uint32, cellX, cellY int) *entity.Entity {
	e := entity.NewEntity(id, entity.TypeMonster)
	x, z := entity.CellToWorld(cellX, cellY)
	e.Body = entity.NewCharacter(x, 0, z)
	e.Body.SetCell(cellX, cellY)
	e.HP, e.MaxHP = 50, 50

	return e
}

// TestWithinAttackRange follows the server: a melee blow lands from an
// adjacent cell, including diagonally, and not from two away.
func TestWithinAttackRange(t *testing.T) {
	s := &InGameState{player: entity.NewCharacter(0, 0, 0)}
	s.player.SetCell(100, 100)

	tests := []struct {
		name       string
		x, y       int
		wantInside bool
	}{
		{"same cell", 100, 100, true},
		{"adjacent", 101, 100, true},
		{"diagonal", 101, 101, true},
		{"two across", 102, 100, false},
		{"two diagonally", 102, 102, false},
		{"behind, adjacent", 99, 99, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := s.withinAttackRange(mobAt(1, tt.x, tt.y)); got != tt.wantInside {
				t.Errorf("withinAttackRange(%d,%d) = %v, want %v",
					tt.x, tt.y, got, tt.wantInside)
			}
		})
	}
}

// TestIsAttackable: monsters, and only ones still alive. Hitting other
// players needs the map's own rules about who may, which this does not model.
func TestIsAttackable(t *testing.T) {
	var s InGameState

	dead := mobAt(1, 0, 0)
	dead.IsDead = true

	tests := []struct {
		name string
		e    *entity.Entity
		want bool
	}{
		{"a monster", mobAt(1, 0, 0), true},
		{"a dead monster", dead, false},
		{"an NPC", entity.NewEntity(2, entity.TypeNPC), false},
		{"a ground item", entity.NewEntity(3, entity.TypeItem), false},
		{"another player", entity.NewEntity(4, entity.TypePlayer), false},
		{"nothing", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := s.isAttackable(tt.e); got != tt.want {
				t.Errorf("isAttackable = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestPendingAttackWaitsForTheWalkToEnd is the same guard the pick-up needed:
// the client reaches a cell before word of it reaches the server, so a blow
// swung while still walking is measured against a character the server has
// further back, and refused.
func TestPendingAttackWaitsForTheWalkToEnd(t *testing.T) {
	m := entity.NewManager()
	m.Add(mobAt(42, 100, 100))

	s := &InGameState{
		player:        entity.NewCharacter(0, 0, 0),
		entityManager: m,
		pendingAttack: 42,
	}
	s.player.SetCell(100, 100)

	// Standing on the target, but still walking: nothing is sent, which is
	// visible in the errand still being pending — sending clears it.
	for i := 0; i < 30; i++ {
		s.updatePendingAttack(16, true)
	}

	if s.pendingAttack != 42 {
		t.Error("swung mid-walk; the server measures from where it thinks the " +
			"character is, and refuses")
	}
}

// TestPendingAttackGivesUpWhenTheTargetDies: nothing left to hit.
func TestPendingAttackGivesUpWhenTheTargetDies(t *testing.T) {
	m := entity.NewManager()
	dead := mobAt(42, 100, 100)
	dead.IsDead = true
	m.Add(dead)

	s := &InGameState{
		player:        entity.NewCharacter(0, 0, 0),
		entityManager: m,
		pendingAttack: 42,
	}

	s.updatePendingAttack(16, true)

	if s.pendingAttack != 0 {
		t.Error("still walking towards a target that is dead")
	}
}

// TestPendingAttackGivesUpAfterStandingStill: the character stopped short,
// which is what an unreachable target looks like from here. Walking resets
// the clock, because a walk pauses between acknowledged paths.
func TestPendingAttackGivesUpAfterStandingStill(t *testing.T) {
	m := entity.NewManager()
	m.Add(mobAt(42, 200, 200))

	s := &InGameState{
		player:        entity.NewCharacter(0, 0, 0),
		entityManager: m,
		pendingAttack: 42,
	}
	s.player.SetCell(100, 100)

	s.updatePendingAttack(attackIdleGiveUpMs/2, false)
	if s.pendingAttack != 42 {
		t.Fatal("gave up during the gap between two acknowledged paths")
	}

	s.updatePendingAttack(attackIdleGiveUpMs, false)
	if s.pendingAttack != 0 {
		t.Error("still waiting after standing still well past the limit")
	}
}

// TestForgetAttack: another click breaks off the fight.
func TestForgetAttack(t *testing.T) {
	s := &InGameState{targetID: 42, pendingAttack: 42, pendingAttackIdleMs: 120}

	s.forgetAttack()

	if s.targetID != 0 || s.pendingAttack != 0 {
		t.Error("still fighting after a click elsewhere")
	}
	if s.pendingAttackIdleMs != 0 {
		t.Error("the idle clock carried over into the next errand")
	}
}
