package states

import (
	"encoding/binary"
	"testing"

	"github.com/Faultbox/midgard-ro/internal/game/entity"
	"github.com/Faultbox/midgard-ro/internal/network/packets"
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

// TestCombatKeepsTheTargetWhileWalking: nothing is swung mid-walk, for the
// reason the pick-up learned — the client reaches a cell before word of it
// reaches the server, so a blow measured from where the client thinks it
// stands is refused. The target is kept, which is how we know nothing was
// sent: sending would have set attacking.
func TestCombatKeepsTheTargetWhileWalking(t *testing.T) {
	m := entity.NewManager()
	m.Add(mobAt(42, 100, 100))

	s := &InGameState{
		player:        entity.NewCharacter(0, 0, 0),
		entityManager: m,
		targetID:      42,
	}
	s.player.SetCell(100, 100)

	for i := 0; i < 30; i++ {
		s.updateCombat(16, true)
	}

	if s.targetID != 42 {
		t.Error("lost the target while walking to it")
	}
	if s.attacking {
		t.Error("swung mid-walk; the server measures from where it thinks the " +
			"character is, and refuses")
	}
}

// TestCombatDropsADeadTarget: there is nothing left to hit, and holding the
// target would keep the marker over a corpse.
func TestCombatDropsADeadTarget(t *testing.T) {
	m := entity.NewManager()
	dead := mobAt(42, 100, 100)
	dead.IsDead = true
	m.Add(dead)

	s := &InGameState{
		player:        entity.NewCharacter(0, 0, 0),
		entityManager: m,
		targetID:      42,
	}

	s.updateCombat(16, false)

	if s.targetID != 0 {
		t.Error("still fighting something that is dead")
	}
}

// TestCombatDropsAVanishedTarget: it walked out of view, or someone else
// killed it.
func TestCombatDropsAVanishedTarget(t *testing.T) {
	s := &InGameState{
		player:        entity.NewCharacter(0, 0, 0),
		entityManager: entity.NewManager(),
		targetID:      42,
	}

	s.updateCombat(16, false)

	if s.targetID != 0 {
		t.Error("still fighting something that is not on the map")
	}
}

// TestChaseIsThrottled: the route to a moving target is reissued as it moves,
// but not every frame — that would be sixty move requests a second.
//
// The throttle is checked before anything is sent, which is what lets this
// run without a connection.
func TestChaseIsThrottled(t *testing.T) {
	m := entity.NewManager()
	m.Add(mobAt(42, 200, 200))

	s := &InGameState{
		player:        entity.NewCharacter(0, 0, 0),
		entityManager: m,
		targetID:      42,
		repathMs:      targetRepathMs,
	}
	s.player.SetCell(100, 100)

	// Well inside the interval: the target is far away, so this is the chase
	// path, and it returns before asking for anything.
	s.updateCombat(16, false)

	if s.repathMs != targetRepathMs-16 {
		t.Errorf("repathMs = %v, want the interval counted down by one frame",
			s.repathMs)
	}
	if s.targetID != 42 {
		t.Error("gave up on a target that is merely far away")
	}
}

// TestCombatGivesUpOnNothing: no target is not a crash.
func TestCombatGivesUpOnNothing(t *testing.T) {
	var s InGameState
	s.updateCombat(16, false)

	if s.targetID != 0 {
		t.Error("acquired a target from nowhere")
	}
}

// TestForgetAttack: another click breaks off the fight.
func TestForgetAttack(t *testing.T) {
	s := &InGameState{targetID: 42, attacking: true, repathMs: 120, resendMs: 90}

	s.forgetAttack()

	if s.targetID != 0 {
		t.Error("still fighting after a click elsewhere")
	}
	if s.attacking || s.repathMs != 0 || s.resendMs != 0 {
		t.Error("the old fight's state carried into the next one")
	}
}

// TestReachFollowsTheServer: the range is the server's to decide, and it
// sends it whenever the weapon changes. Guessing one cell put the character
// on top of everything it fought.
func TestReachFollowsTheServer(t *testing.T) {
	var s InGameState

	if got := s.reach(); got != defaultAttackRange {
		t.Errorf("reach before the server has said = %d, want %d", got, defaultAttackRange)
	}

	s.attackRange = 5
	if got := s.reach(); got != 5 {
		t.Errorf("reach = %d, want the 5 the server gave", got)
	}
}

// TestAttackRangeIgnoresNonsense: standing off at a range the server will
// refuse is worse than standing too close, so an implausible figure is left
// alone rather than believed.
func TestAttackRangeIgnoresNonsense(t *testing.T) {
	pkt := func(reach int16) []byte {
		b := make([]byte, 4)
		binary.LittleEndian.PutUint16(b, packets.ZC_ATTACK_RANGE)
		binary.LittleEndian.PutUint16(b[2:], uint16(reach))

		return b
	}

	s := &InGameState{attackRange: 3}

	for _, reach := range []int16{0, -1, 21, 1000} {
		_ = s.handleAttackRange(pkt(reach))
		if s.attackRange != 3 {
			t.Errorf("took %d as a range, replacing the sensible one", reach)
			s.attackRange = 3
		}
	}

	_ = s.handleAttackRange(pkt(9))
	if s.attackRange != 9 {
		t.Errorf("attackRange = %d, want the 9 the server sent", s.attackRange)
	}

	if _, ok := packets.DecodeAttackRange(make([]byte, 3)); ok {
		t.Error("a 3-byte range decoded, but the packet is 4")
	}
}

// TestFaceTowardsTurnsAUnit: the server says nothing about which way anyone
// is looking when a blow lands, so the client turns both sides itself —
// otherwise a monster that walked past and then attacked strikes sideways.
func TestFaceTowardsTurnsAUnit(t *testing.T) {
	m := entity.NewManager()
	attacker := mobAt(1, 100, 100)
	victim := mobAt(2, 105, 100) // due east
	m.Add(attacker)
	m.Add(victim)

	s := &InGameState{entityManager: m}

	attacker.Body.Direction = entity.DirectionFromCellDelta(0, -1) // looking north
	s.faceTowards(1, 2)

	want := entity.DirectionFromCellDelta(1, 0)
	if attacker.Body.Direction != want {
		t.Errorf("Direction = %d, want %d (towards the unit it is hitting)",
			attacker.Body.Direction, want)
	}
}

// TestFaceTowardsLeavesAWalkerAlone: a unit mid-walk is facing the way it is
// going and the path overwrites this on the next step, so turning it here
// only makes it flicker between the two.
func TestFaceTowardsLeavesAWalkerAlone(t *testing.T) {
	m := entity.NewManager()
	walker := mobAt(1, 100, 100)
	target := mobAt(2, 105, 100)
	m.Add(walker)
	m.Add(target)

	walker.Body.FollowPath([][2]int{{100, 100}, {100, 101}, {100, 102}})
	before := walker.Body.Direction

	s := &InGameState{entityManager: m}
	s.faceTowards(1, 2)

	if walker.Body.Direction != before {
		t.Error("turned a unit that is in the middle of a walk")
	}
}

// TestFaceTowardsSurvivesTheUnknown: a blow between two units we cannot see
// names neither, and must not be a crash.
func TestFaceTowardsSurvivesTheUnknown(t *testing.T) {
	s := &InGameState{entityManager: entity.NewManager()}

	s.faceTowards(1, 2)
	s.faceTowards(0, 0)
}
