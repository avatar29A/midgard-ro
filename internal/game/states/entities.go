package states

import (
	"github.com/Faultbox/midgard-ro/internal/engine/charsprite"
	"github.com/Faultbox/midgard-ro/internal/game/entity"
	"github.com/Faultbox/midgard-ro/internal/game/items"
	"github.com/Faultbox/midgard-ro/internal/network/packets"
)

// PathFunc reproduces the route between two cells. The server sends only the
// endpoints of a walk, so the client has to walk the same cells the server
// thinks it is walking or the two drift apart.
type PathFunc func(fromX, fromY, toX, toY int) [][2]int

// unitType maps a unit's wire objecttype onto the entity type the UI and the
// renderer work in. Anything not drawn as a character or a monster is treated
// as an NPC, which is the harmless default: it gets a name and no HP bar.
//
// Warps are the exception the server does not make for us: it sends them as
// NPCs, and only the job says otherwise. Both warp classes come out as
// TypeWarp; which of them is drawn is unitIsDrawable's decision.
func unitType(kind packets.EntityKind, job int16) entity.Type {
	switch kind {
	case packets.EntityPlayer, packets.EntityDisguised:
		return entity.TypePlayer
	case packets.EntityMob, packets.EntityABR, packets.EntityBionic:
		return entity.TypeMonster
	case packets.EntityItem:
		return entity.TypeItem
	default:
		if job == packets.JobWarpPortal || job == packets.JobHiddenWarp {
			return entity.TypeWarp
		}
		return entity.TypeNPC
	}
}

// upsertUnit creates or refreshes the entity for a unit the server has told us
// about, and returns it.
//
// The same unit arrives repeatedly — standing, then walking, then standing
// somewhere else — so this updates in place rather than replacing. Replacing
// would discard the walk in progress and make every unit teleport between
// cells instead of moving.
//
// Units are keyed by AID: rAthena fills GID with the character id, which is
// zero for every monster and NPC on the map, so keying by GID would collapse
// them all onto one entity.
func upsertUnit(m *entity.Manager, u *packets.Entity, path PathFunc) *entity.Entity {
	if m == nil || u == nil || u.AID == 0 {
		return nil
	}

	e := m.Get(u.AID)
	if e == nil {
		e = entity.NewEntity(u.AID, unitType(u.Kind, u.Job))
		m.Add(e)
	}
	e.CancelLeaving()

	e.Type = unitType(u.Kind, u.Job)
	e.Name = u.Name
	e.Job = int(u.Job)
	e.SpriteID = int(u.Job)
	// rAthena sends 0 for female and 1 for male.
	e.Female = u.Sex == 0
	e.HairStyle = int(u.HairStyle)
	e.HairColor = int(u.HairColor)
	e.ClothesColor = int(u.ClothesColor)
	e.Weapon = int(u.Weapon)
	e.Shield = int(u.Shield)
	e.Robe = int(u.Robe)
	e.Level = int(u.Level)
	e.HP = int(u.HP)
	e.MaxHP = int(u.MaxHP)
	e.IsDead = u.MaxHP > 0 && u.HP <= 0

	if e.Body == nil {
		e.Body = newUnitBody(u)
	}
	e.Body.WalkSpeedMs = walkSpeedOf(u)

	if u.Moving {
		startUnitWalk(e.Body, u, path)
	} else {
		placeUnit(e.Body, u)
	}

	return e
}

// newUnitBody puts a newly seen unit at the cell it was reported on. Its
// render position starts there too, so it fades in where it belongs instead of
// sliding in from wherever the zero value happens to be.
func newUnitBody(u *packets.Entity) *entity.Character {
	x, z := entity.CellToWorld(u.X, u.Y)
	return entity.NewCharacter(x, 0, z)
}

// placeUnit puts a standing unit on its cell.
//
// A unit that was walking is stopped here rather than left to finish — SetCell
// abandons the path — because the server has just said where it actually is,
// and finishing a stale route would walk it away from that.
func placeUnit(body *entity.Character, u *packets.Entity) {
	body.SetCell(u.X, u.Y)
	if u.Dir >= 0 {
		body.Direction = entity.DirectionFromServer(uint8(u.Dir))
	}
}

// startUnitWalk sets a unit walking the route between the cells the server
// reported. If the route cannot be reproduced, the unit is placed at the
// destination rather than left behind — being in the right cell without the
// animation beats drifting further out of step with every step.
//
// The unit is deliberately not placed on the start cell first. FollowPath
// already moves the authoritative position there while keeping the drawn one,
// and bleeds the difference off over the following frames; snapping first
// would throw that away and make every unit jerk backward at the start of
// each step, since word of a walk always reaches us slightly after it began.
func startUnitWalk(body *entity.Character, u *packets.Entity, path PathFunc) {
	var route [][2]int
	if path != nil {
		route = path(u.X, u.Y, u.ToX, u.ToY)
	}
	if len(route) < 2 {
		route = entity.CellLine(u.X, u.Y, u.ToX, u.ToY)
	}
	if len(route) < 2 {
		body.SetCell(u.ToX, u.ToY)
		return
	}

	body.FollowPath(route)
}

// walkSpeedOf returns the unit's milliseconds per cell, falling back to the
// default when the server sends a value no real unit has. A zero would make
// every step take no time at all.
func walkSpeedOf(u *packets.Entity) float32 {
	speed := float32(u.SpeedMs)
	if speed < 20 || speed > 2000 {
		return entity.DefaultWalkSpeedMs
	}
	return speed
}

// unitSpec describes which sprites to draw for a unit.
//
// Players are composited from a body and a head chosen by job, sex and hair.
// Everything else is a single sprite named by the client's own table, where
// the job id is the whole of the identity.
func unitSpec(e *entity.Entity) charsprite.Spec {
	switch e.Type {
	case entity.TypeMonster:
		return charsprite.Spec{Kind: charsprite.KindMonster, Job: e.Job}
	case entity.TypeNPC, entity.TypeWarp:
		return charsprite.Spec{Kind: charsprite.KindNPC, Job: e.Job}
	case entity.TypeItem:
		info, _ := items.Lookup(e.ItemID)

		return charsprite.Spec{Kind: charsprite.KindItem, Name: info.Resource}
	default:
		return charsprite.Spec{
			Job:       e.Job,
			Female:    e.Female,
			HairStyle: e.HairStyle,
		}
	}
}

// unitIsDrawable reports whether we can draw a unit truthfully.
//
// A player always resolves, falling back to a Novice at worst. Monsters and
// NPCs are drawn only when the client's own table names their sprite: an
// unknown job would otherwise fall through to the player path and put a
// person on screen wherever a Poring stands.
//
// A dropped item is drawn only when the item table names a resource for it.
// It is filed under its own directory by name rather than by job id, so
// without that name there is nothing to look up — and falling back to the job
// table would draw whichever monster happens to share the number.
//
// A warp is drawn as the portal effect, never as a sprite — the table names
// one for class 45, and the original ignores it. The hidden class is not
// drawn at all.
func unitIsDrawable(e *entity.Entity) bool {
	if e == nil || e.Body == nil {
		return false
	}
	switch e.Type {
	case entity.TypePlayer:
		return true
	case entity.TypeMonster, entity.TypeNPC:
		_, known := charsprite.SpriteName(e.Job)
		return known
	case entity.TypeWarp:
		return e.Job == packets.JobWarpPortal
	case entity.TypeItem:
		info, known := items.Lookup(e.ItemID)

		return known && info.Resource != ""
	default:
		return false
	}
}

// removeUnit starts a unit the server says is gone fading out. It is dropped
// once the fade finishes, in updateUnits.
//
// Not removed outright: the server drops units the moment they cross its
// area_size, so removing on the packet makes them vanish mid-stride.
func removeUnit(m *entity.Manager, aid uint32) {
	if m == nil || aid == 0 || aid == m.PlayerID() {
		return
	}
	if e := m.Get(aid); e != nil {
		e.BeginLeaving()
	}
}

// UnitAnimFunc reports how a unit's sprite animates for an action and facing:
// how many frames it has, so the loop is the right length, and how long each
// is held, so it runs at the rate its own ACT specifies.
type UnitAnimFunc func(e *entity.Entity, action, direction int) (frames int, intervalMs float32)

// updateUnits advances every remote unit's walk and animation by one frame.
//
// The local player is not in here: its body is owned by the game state and
// updated on its own path, because it alone has prediction and acknowledgement
// to reconcile.
//
// A nil anim parks every unit on frame 0, which is what happens before any
// sprites have loaded.
func updateUnits(m *entity.Manager, deltaMs float32, anim UnitAnimFunc) {
	if m == nil {
		return
	}

	for _, e := range m.All() {
		e.FadeMs += deltaMs
		if e.FadedOut() {
			m.Remove(e.ID)
			continue
		}

		if e.Body == nil {
			continue
		}

		e.Body.Update(deltaMs)
		e.Body.UpdateRenderPosition(deltaMs)

		idle, walk, pickup := 0, 0, 0
		if anim != nil {
			var idleMs, walkMs, pickupMs float32
			idle, idleMs = anim(e, entity.ActionIdle, e.Body.Direction)
			walk, walkMs = anim(e, entity.ActionWalk, e.Body.Direction)
			pickup, pickupMs = anim(e, entity.ActionPickup, e.Body.Direction)
			e.Body.AnimIntervalMs = [entity.LoadedActions]float32{
				entity.ActionIdle:   idleMs,
				entity.ActionWalk:   walkMs,
				entity.ActionPickup: pickupMs,
			}
		}
		e.Body.AdvanceAnimation(deltaMs, idle, walk, pickup)
	}
}
