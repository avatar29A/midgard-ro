package states

import (
	"testing"

	"github.com/Faultbox/midgard-ro/internal/game/entity"
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
