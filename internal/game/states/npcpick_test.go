package states

import (
	"testing"

	"github.com/Faultbox/midgard-ro/internal/engine/scene"
	"github.com/Faultbox/midgard-ro/internal/game/entity"
	"github.com/Faultbox/midgard-ro/internal/network/packets"
	"github.com/Faultbox/midgard-ro/pkg/formats"
)

// npcAt builds an NPC standing at a world position, with the Body that makes
// it drawable and therefore clickable.
func npcAt(id uint32, x, y, z float32) *entity.Entity {
	return &entity.Entity{
		ID:   id,
		Type: entity.TypeNPC,
		Body: &entity.Character{RenderX: x, RenderY: y, RenderZ: z},
	}
}

func TestIsClickable(t *testing.T) {
	var s InGameState

	tests := []struct {
		name string
		e    *entity.Entity
		want bool
	}{
		{"an NPC", npcAt(100, 0, 0, 0), true},
		{"nothing", nil, false},
		{
			name: "an NPC we have never drawn has no body to stand on",
			e:    &entity.Entity{ID: 100, Type: entity.TypeNPC},
			want: false,
		},
		{
			name: "a monster is not talked to",
			e:    &entity.Entity{ID: 101, Type: entity.TypeMonster, Body: &entity.Character{}},
			want: false,
		},
		{
			name: "another player is not talked to",
			e:    &entity.Entity{ID: 102, Type: entity.TypePlayer, Body: &entity.Character{}},
			want: false,
		},
		{
			name: "a portal is walked into",
			e:    &entity.Entity{ID: 103, Type: entity.TypeWarp, Job: packets.JobWarpPortal, Body: &entity.Character{}},
			want: true,
		},
		{
			name: "a hidden warp shows nothing and takes no click",
			e:    &entity.Entity{ID: 104, Type: entity.TypeWarp, Job: packets.JobHiddenWarp, Body: &entity.Character{}},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := s.isClickable(tt.e); got != tt.want {
				t.Errorf("isClickable(%+v) = %v, want %v", tt.e, got, tt.want)
			}
		})
	}
}

// TestUnitBoxStandsOnItsFeet pins the box against the quad the unit is drawn
// as: centered horizontally on its position, rising from the ground it stands
// on. A box centered vertically instead would sit half underground and take
// clicks aimed at the floor.
func TestUnitBoxStandsOnItsFeet(t *testing.T) {
	var s InGameState // no renderer, so the fallback size applies

	box := s.unitBox(npcAt(1, 100, 20, 300))

	wantMin := [3]float32{100 - fallbackUnitHalfWidth, 20, 300 - fallbackUnitHalfWidth}
	wantMax := [3]float32{100 + fallbackUnitHalfWidth, 20 + fallbackUnitHeight, 300 + fallbackUnitHalfWidth}

	if box.Min != wantMin {
		t.Errorf("Min = %v, want %v", box.Min, wantMin)
	}
	if box.Max != wantMax {
		t.Errorf("Max = %v, want %v", box.Max, wantMax)
	}
}

// TestPickEntityWithoutSceneIsSafe covers the paths that run before a map is
// ready — a click cannot reach them in practice, but nothing here should
// depend on that being true.
func TestPickEntityWithoutSceneIsSafe(t *testing.T) {
	var s InGameState

	if got := s.PickEntity(100, 100, 1280, 720); got != nil {
		t.Errorf("PickEntity with no scene = %+v, want nil", got)
	}

	var nilState *InGameState
	if got := nilState.PickEntity(100, 100, 1280, 720); got != nil {
		t.Errorf("PickEntity on nil = %+v, want nil", got)
	}
}

// TestWarpBoxIsThePortalsSize pins the hit box of a warp to the effect that is
// drawn for it: there is no sprite to measure, and the fallback box would be
// a third of the portal's width.
func TestWarpBoxIsThePortalsSize(t *testing.T) {
	var s InGameState
	e := &entity.Entity{ID: 1, Type: entity.TypeWarp, Job: packets.JobWarpPortal,
		Body: &entity.Character{RenderX: 100, RenderY: 20, RenderZ: 300}}

	box := s.unitBox(e)
	if w := box.Max[0] - box.Min[0]; w != 2*scene.PortalRadius {
		t.Fatalf("warp box width %v, want %v", w, 2*scene.PortalRadius)
	}
	if h := box.Max[1] - box.Min[1]; h != scene.PortalHeight {
		t.Fatalf("warp box height %v, want %v", h, scene.PortalHeight)
	}
	if box.Min[1] != 20 {
		t.Fatalf("warp box floor %v, want the ground at 20", box.Min[1])
	}
}

// gatWith builds a walkability grid where every cell is blocked except the
// ones named.
func gatWith(width, height int, walkable ...[2]int) *formats.GAT {
	g := &formats.GAT{
		Width:  uint32(width),
		Height: uint32(height),
		Cells:  make([]formats.GATCell, width*height),
	}
	for i := range g.Cells {
		g.Cells[i].Type = formats.GATBlocked
	}
	for _, c := range walkable {
		g.Cells[c[1]*width+c[0]].Type = formats.GATWalkable
	}
	return g
}

// TestWarpApproachStandsWhereItCan pins the fix for a dead click: the gate
// out of a field sits inside the wall's arch, where nobody can stand, and
// rAthena answers an unpathable walk with silence. The click has to aim at a
// cell inside the warp's trigger box that the player can actually reach.
func TestWarpApproachStandsWhereItCan(t *testing.T) {
	t.Run("the warp's own cell, when it can be stood on", func(t *testing.T) {
		s := InGameState{gat: gatWith(10, 10, [2]int{5, 5})}
		x, y, ok := s.WarpApproach(5, 5)
		if !ok || x != 5 || y != 5 {
			t.Fatalf("got %d,%d ok=%v, want the warp's cell", x, y, ok)
		}
	})

	t.Run("the nearest walkable cell otherwise", func(t *testing.T) {
		// As prt_fild08: the warp at (5,8) is in the wall, the ground stops
		// one cell short of it.
		s := InGameState{gat: gatWith(10, 10, [2]int{5, 7}, [2]int{4, 7})}
		x, y, ok := s.WarpApproach(5, 8)
		if !ok || x != 5 || y != 7 {
			t.Fatalf("got %d,%d ok=%v, want 5,7", x, y, ok)
		}
	})

	t.Run("of several, the one nearest the player", func(t *testing.T) {
		s := InGameState{
			gat:    gatWith(20, 20, [2]int{9, 10}, [2]int{11, 10}),
			player: entity.NewCharacter(0, 0, 0),
		}
		s.player.SetCell(15, 10)
		x, y, ok := s.WarpApproach(10, 10)
		if !ok || x != 11 || y != 10 {
			t.Fatalf("got %d,%d ok=%v, want 11,10 — the side the player is on", x, y, ok)
		}
	})

	t.Run("nowhere to stand", func(t *testing.T) {
		s := InGameState{gat: gatWith(20, 20)}
		if _, _, ok := s.WarpApproach(10, 10); ok {
			t.Fatal("reported a way into a warp walled in on every side")
		}
	})

	t.Run("no walkability data", func(t *testing.T) {
		var s InGameState
		x, y, ok := s.WarpApproach(3, 4)
		if !ok || x != 3 || y != 4 {
			t.Fatalf("got %d,%d ok=%v; without a GAT the warp's cell is the only guess", x, y, ok)
		}
	})
}
