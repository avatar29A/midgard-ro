package skillvisual

import (
	"fmt"
	"strings"
)

// Preview identifies adapters supported by both the catalog and Studio worker.
type Preview struct {
	SkillID               uint16
	ID, Generator, Source string
}

func PreviewOf(id uint16) (Preview, bool) {
	if IsMageAttack(id) {
		return Preview{id, fmt.Sprintf("mage_%d.client", id), "client.mage_sequence", "internal/game/states/magepreview.go"}, true
	}
	switch id {
	case 10:
		return Preview{10, "sight.client", "procedural.sight", "internal/game/states/sightaura.go"}, true
	case 12:
		return Preview{12, "safety_wall.client", "str.safety_wall", "internal/game/states/effects.go"}, true
	case 13:
		return Preview{13, "soul_strike.default", "procedural.soul_strike", "internal/game/states/burst.go"}, true
	case 18:
		return Preview{18, "fire_wall.client", "sprite.fire_wall", "internal/game/states/spriteeffect.go"}, true
	}
	return Preview{}, false
}

func IsMageAttack(id uint16) bool {
	switch id {
	case 11, 14, 15, 16, 17, 19, 20, 21:
		return true
	}
	return false
}
func IsPersistent(id uint16) bool { return id == 10 || id == 12 || id == 18 }

// EffectFileFor is shared by the game and library; a path mapping does not
// guarantee that an archive contains the file or that STR is the whole effect.
func EffectFileFor(name string) string {
	if !strings.HasPrefix(name, "EF_") || len(name) <= 3 {
		return ""
	}
	if name == "EF_LIGHTBOLT" {
		return "lightning.str"
	}
	return strings.ToLower(name[3:]) + ".str"
}
