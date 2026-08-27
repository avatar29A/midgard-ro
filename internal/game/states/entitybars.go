package states

import (
	"github.com/Faultbox/midgard-ro/internal/game/entity"
)

// EntityBar is one unit's bars, ready to draw.
//
// It lives here rather than in the ui package because the projection happens
// where the view matrix is, and ui already imports this package — the other
// direction would be a cycle. The UI layer gets finished positions and needs
// nothing from the scene, the same arrangement ChatLine has.
type EntityBar struct {
	// ScreenX, ScreenY is where the unit's feet are, in viewport pixels.
	ScreenX, ScreenY float32

	Type entity.Type

	HP, MaxHP int

	// HasSP is false for everything the server does not tell us the SP of,
	// which is everything except our own character.
	HasSP     bool
	SP, MaxSP int

	// Alpha fades the bars with the unit they belong to.
	Alpha float32
}
