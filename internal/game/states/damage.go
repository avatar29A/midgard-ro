package states

import (
	"github.com/Faultbox/midgard-ro/internal/game/entity"
	"github.com/Faultbox/midgard-ro/internal/network/packets"
)

// Figures that float up from a blow and fade.
//
// Kept in world space and projected when the interface asks for them, the way
// the bars and labels are: the view matrix lives here, and the UI wants
// finished positions.
const (
	// damageLifeMs is how long a figure lasts. Long enough to read at a
	// glance, short enough that a fast exchange does not stack them.
	damageLifeMs = 900

	// damageMaxOnScreen caps how many are kept. A fight against several
	// monsters produces a blow every few frames, and there is no point
	// holding figures nobody can still read.
	damageMaxOnScreen = 48
)

// floatingDamage is one figure, before projection.
type floatingDamage struct {
	amount   int
	miss     bool
	critical bool

	// Where it started, in world space. It rises on screen rather than in the
	// world, so this does not move.
	x, y, z float32

	ageMs float32
}

// DamageNumber is one figure ready to draw, in viewport pixels.
type DamageNumber struct {
	Amount   int
	Miss     bool
	Critical bool

	ScreenX, ScreenY float32

	// Progress runs 0 to 1 across the figure's life, for the rise and fade.
	Progress float32
}

// addDamageNumber starts a figure over whoever took the blow.
//
// Over the target rather than the attacker: the figure says what was done to
// something, and in a crowd the attacker may be off screen entirely while the
// thing being hit is the reason you are watching.
func (s *InGameState) addDamageNumber(blow packets.Damage) {
	body := s.bodyOf(blow.TargetID)
	if body == nil {
		return
	}

	top := body.RenderY
	if e := s.entityOf(blow.TargetID); e != nil {
		top = s.unitBox(e).Max[1]
	}

	if len(s.damageNumbers) >= damageMaxOnScreen {
		// Drop the oldest rather than refusing the newest: the blow that just
		// landed is the one worth seeing.
		s.damageNumbers = s.damageNumbers[1:]
	}

	s.damageNumbers = append(s.damageNumbers, floatingDamage{
		amount:   blow.Amount,
		miss:     blow.Missed(),
		critical: blow.Critical(),
		x:        body.RenderX,
		y:        top,
		z:        body.RenderZ,
	})
}

// entityOf is the entity behind an id, or nil for the player and the unknown.
func (s *InGameState) entityOf(id uint32) *entity.Entity {
	if s.entityManager == nil {
		return nil
	}

	return s.entityManager.Get(id)
}

// updateDamageNumbers ages the figures and drops the ones that have finished.
func (s *InGameState) updateDamageNumbers(deltaMs float32) {
	if len(s.damageNumbers) == 0 {
		return
	}

	kept := s.damageNumbers[:0]
	for _, d := range s.damageNumbers {
		d.ageMs += deltaMs
		if d.ageMs < damageLifeMs {
			kept = append(kept, d)
		}
	}

	s.damageNumbers = kept
}

// DamageNumbers are the figures to draw this frame, already projected.
func (s *InGameState) DamageNumbers(viewportW, viewportH float32) []DamageNumber {
	if len(s.damageNumbers) == 0 || s.scene == nil || !s.SceneReady {
		return nil
	}

	out := make([]DamageNumber, 0, len(s.damageNumbers))
	for _, d := range s.damageNumbers {
		x, y := s.projectToScreen(d.x, d.y, d.z, viewportW, viewportH)
		if x < 0 {
			continue
		}

		out = append(out, DamageNumber{
			Amount:   d.amount,
			Miss:     d.miss,
			Critical: d.critical,
			ScreenX:  x,
			ScreenY:  y,
			Progress: d.ageMs / damageLifeMs,
		})
	}

	return out
}
