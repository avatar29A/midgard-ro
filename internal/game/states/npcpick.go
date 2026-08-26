package states

import (
	"go.uber.org/zap"

	"github.com/Faultbox/midgard-ro/internal/engine/picking"
	"github.com/Faultbox/midgard-ro/internal/game/entity"
	"github.com/Faultbox/midgard-ro/internal/logger"
	"github.com/Faultbox/midgard-ro/internal/network/packets"
	"github.com/Faultbox/midgard-ro/internal/trace"
	"github.com/Faultbox/midgard-ro/pkg/math"
)

// fallbackUnitHalfWidth and fallbackUnitHeight size a hit box for a unit whose
// sprite sheet has not been baked yet, in world units.
//
// A sheet is baked on the unit's first draw, so this only applies to something
// that has never been on screen — which cannot have been clicked either. It is
// here so the hit test degrades to a plausible box rather than to a zero-sized
// one that can never be hit.
const (
	fallbackUnitHalfWidth = float32(4)
	fallbackUnitHeight    = float32(15)
)

// PickEntity returns the unit under a screen position, nearest first, or nil.
//
// The test is against an axis-aligned box around each unit rather than against
// the sprite itself: the billboard turns to face the camera, so its plane has
// no fixed orientation, and a square column of the sprite's own width covers
// wherever it happens to be pointing. That over-selects at the corners, which
// is the forgiving direction for a thing you are trying to click on.
func (s *InGameState) PickEntity(screenX, screenY, viewportW, viewportH float32) *entity.Entity {
	if s == nil || s.scene == nil || s.entityManager == nil || viewportW <= 0 || viewportH <= 0 {
		return nil
	}

	ray := picking.ScreenToRay(screenX, screenY, viewportW, viewportH, s.scene.LastViewProj().Inverse())

	var (
		nearest    *entity.Entity
		nearestT   float32
		candidates int
	)

	for _, e := range s.entityManager.All() {
		if !s.isClickable(e) {
			continue
		}
		candidates++

		box := s.unitBox(e)
		t, hit := ray.IntersectAABB(box)

		if trace.On(trace.NPC) {
			// Where the unit is on screen, which is what you need to know when
			// a click that looked right missed: it tells a box in the wrong
			// place from a hand in the wrong place.
			sx, sy := s.projectToScreen(e.Body.RenderX, e.Body.RenderY, e.Body.RenderZ, viewportW, viewportH)

			trace.Emit(trace.NPC, "candidate",
				zap.Uint32("npcID", e.ID), zap.String("name", e.Name),
				zap.Float32("x", e.Body.RenderX), zap.Float32("y", e.Body.RenderY),
				zap.Float32("z", e.Body.RenderZ),
				zap.Float32("screenX", sx), zap.Float32("screenY", sy),
				zap.Float32("boxW", box.Max[0]-box.Min[0]),
				zap.Float32("boxH", box.Max[1]-box.Min[1]),
				zap.Bool("hit", hit), zap.Float32("t", t))
		}

		if !hit || t < 0 {
			continue
		}

		if nearest == nil || t < nearestT {
			nearest, nearestT = e, t
		}
	}

	// A click that hits nothing is the common case — most of the screen is
	// ground — so this says how many units were even eligible. Zero
	// candidates and a missed ray are very different faults: the first means
	// no NPC is being tracked, the second that the boxes are in the wrong
	// place.
	if trace.On(trace.NPC) {
		trace.Emit(trace.NPC, "pick",
			zap.Int("units", len(s.entityManager.All())),
			zap.Int("candidates", candidates),
			zap.Bool("hit", nearest != nil))
	}

	return nearest
}

// projectToScreen puts a world position into viewport pixels, for the trace.
// A point behind the camera comes back as (-1, -1) rather than as the mirrored
// nonsense the divide would otherwise give.
func (s *InGameState) projectToScreen(x, y, z, viewportW, viewportH float32) (float32, float32) {
	clip := s.scene.LastViewProj().MulVec4(math.Vec4{x, y, z, 1})
	if clip[3] <= 0 {
		return -1, -1
	}

	return (clip[0]/clip[3]*0.5 + 0.5) * viewportW,
		(0.5 - clip[1]/clip[3]*0.5) * viewportH
}

// isClickable reports whether a unit can be talked to.
//
// Only NPCs, and never the player: a click on yourself is a click on the
// ground you are standing on.
func (s *InGameState) isClickable(e *entity.Entity) bool {
	if e == nil || e.Body == nil || e.Type != entity.TypeNPC {
		return false
	}

	return e.ID != s.selfAID()
}

// unitBox is the volume a unit occupies, matching the quad it is drawn as:
// centered horizontally on its position and standing on its feet.
func (s *InGameState) unitBox(e *entity.Entity) picking.AABB {
	halfWidth, height := fallbackUnitHalfWidth, fallbackUnitHeight

	if s.playerRender != nil {
		if w, h := s.playerRender.UnitQuadSize(unitSpec(e)); w > 0 && h > 0 {
			halfWidth, height = w/2, h
		}
	}

	x, y, z := e.Body.RenderX, e.Body.RenderY, e.Body.RenderZ

	return picking.AABB{
		Min: [3]float32{x - halfWidth, y, z - halfWidth},
		Max: [3]float32{x + halfWidth, y + height, z + halfWidth},
	}
}

// ContactNPC asks the server to start a conversation.
func (s *InGameState) ContactNPC(e *entity.Entity) {
	if s == nil || e == nil || s.client == nil {
		return
	}

	if err := s.client.Send(packets.ContactNPC(e.ID)); err != nil {
		logger.Warn("could not contact NPC",
			zap.Uint32("npcID", e.ID), zap.String("name", e.Name), zap.Error(err))

		return
	}

	trace.Emit(trace.NPC, "contact", zap.Uint32("npcID", e.ID), zap.String("name", e.Name))
}
