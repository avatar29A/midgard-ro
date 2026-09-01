package states

import (
	"go.uber.org/zap"

	"github.com/Faultbox/midgard-ro/internal/engine/picking"
	"github.com/Faultbox/midgard-ro/internal/engine/scene"
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

// PickEntity returns the unit under a screen position, nearest first, or nil,
// reporting what it considered on the `npc` trace.
//
// The test is against an axis-aligned box around each unit rather than against
// the sprite itself: the billboard turns to face the camera, so its plane has
// no fixed orientation, and a square column of the sprite's own width covers
// wherever it happens to be pointing. That over-selects at the corners, which
// is the forgiving direction for a thing you are trying to click on.
func (s *InGameState) PickEntity(screenX, screenY, viewportW, viewportH float32) *entity.Entity {
	return s.pickEntity(screenX, screenY, viewportW, viewportH, true)
}

// HoverEntity is PickEntity without the trace, for the test that runs every
// frame to decide which cursor to show. Tracing that would put sixty lines a
// second in the log and bury the clicks it exists to explain.
func (s *InGameState) HoverEntity(screenX, screenY, viewportW, viewportH float32) *entity.Entity {
	return s.pickEntity(screenX, screenY, viewportW, viewportH, false)
}

func (s *InGameState) pickEntity(screenX, screenY, viewportW, viewportH float32, tracing bool) *entity.Entity {
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

		if tracing && trace.On(trace.NPC) {
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
	if tracing && trace.On(trace.NPC) {
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

// isClickable reports whether a unit answers to the pointer: an NPC, which
// can be talked to, a visible warp, which can be walked into, or an item on
// the ground, which can be picked up. Never the player — a click on yourself
// is a click on the ground you are standing on — and never a hidden warp,
// which the original shows nothing for.
//
// An item we cannot draw is not clickable either. It is not on screen, so a
// hit box for it would take clicks meant for the ground underneath.
func (s *InGameState) isClickable(e *entity.Entity) bool {
	if e == nil || e.Body == nil {
		return false
	}
	switch e.Type {
	case entity.TypeNPC:
		return e.ID != s.selfAID()
	case entity.TypeWarp:
		return e.Job == packets.JobWarpPortal
	case entity.TypeItem:
		return unitIsDrawable(e)
	default:
		return false
	}
}

// unitBox is the volume a unit occupies, matching the quad it is drawn as:
// centered horizontally on its position and standing on its feet.
func (s *InGameState) unitBox(e *entity.Entity) picking.AABB {
	halfWidth, height := fallbackUnitHalfWidth, fallbackUnitHeight

	if e.Type == entity.TypeWarp {
		// The portal's own size; there is no sprite to measure.
		halfWidth, height = scene.PortalRadius, scene.PortalHeight
	} else if s.playerRender != nil {
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

// maxWarpApproach is how far from a warp we will look for somewhere to stand.
//
// A warp's trigger is a box around its cell — rAthena's xs, ys, which are
// half-extents of one to three cells in the scripts we run — so a cell this
// close is inside it. Looking further would walk the player to a warp they
// then fail to take.
const maxWarpApproach = 3

// WarpApproach returns the cell to walk to in order to take a warp.
//
// It is not always the warp's own cell: warps sit where the map wants the
// player to leave from, which is often somewhere nobody can stand. The gate
// out of prt_fild08 is at 170,378, inside the arch of Prontera's wall, and
// the walkable ground stops at 377. Asking the server to walk onto it is
// asking for the one thing rAthena answers with silence — an unpathable
// walk — so the click did nothing at all.
//
// The trigger box saves us: standing next to the warp is standing in it. So
// when the warp's cell cannot be stood on, this looks outward for the
// walkable cell closest to it, preferring the one nearest the player, and
// walks there instead.
func (s *InGameState) WarpApproach(warpX, warpY int) (x, y int, ok bool) {
	if s.gat == nil {
		// Without a walkability grid the warp's own cell is the only guess
		// we have, and the server will tell us if it is wrong.
		return warpX, warpY, true
	}
	if s.gat.IsWalkable(warpX, warpY) {
		return warpX, warpY, true
	}

	fromX, fromY := warpX, warpY
	if s.player != nil {
		fromX, fromY = s.player.CurrentCell()
	}

	for r := 1; r <= maxWarpApproach; r++ {
		bestX, bestY, bestDist, found := 0, 0, 0, false
		for dy := -r; dy <= r; dy++ {
			for dx := -r; dx <= r; dx++ {
				// The ring at this radius; the inside was covered already.
				if abs(dx) != r && abs(dy) != r {
					continue
				}
				cx, cy := warpX+dx, warpY+dy
				if !s.gat.IsWalkable(cx, cy) {
					continue
				}
				d := (cx-fromX)*(cx-fromX) + (cy-fromY)*(cy-fromY)
				if !found || d < bestDist {
					bestX, bestY, bestDist, found = cx, cy, d, true
				}
			}
		}
		if found {
			return bestX, bestY, true
		}
	}

	return 0, 0, false
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
