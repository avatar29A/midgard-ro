package game

import (
	"testing"

	"github.com/Faultbox/midgard-ro/internal/engine/cursor"
	"github.com/Faultbox/midgard-ro/internal/game/entity"
)

// TestCursorForUnitKinds pins which of the original's cursors each kind of
// unit gets: talk over an NPC, the door over a warp, nothing special
// otherwise.
func TestCursorForUnitKinds(t *testing.T) {
	tests := []struct {
		name string
		e    *entity.Entity
		want cursor.State
	}{
		{"nothing", nil, cursor.StateDefault},
		{"an NPC", &entity.Entity{Type: entity.TypeNPC}, cursor.StateTalk},
		{"a warp", &entity.Entity{Type: entity.TypeWarp}, cursor.StateWarp},
		{"a monster", &entity.Entity{Type: entity.TypeMonster}, cursor.StateAttack},
		{"a ground item", &entity.Entity{Type: entity.TypeItem}, cursor.StatePick},
		{"a player", &entity.Entity{Type: entity.TypePlayer}, cursor.StateDefault},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cursorFor(tt.e); got != tt.want {
				t.Fatalf("cursorFor = %v, want %v", got, tt.want)
			}
		})
	}
}
