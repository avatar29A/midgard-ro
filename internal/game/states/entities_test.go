package states

import (
	"testing"

	"github.com/Faultbox/midgard-ro/internal/engine/charsprite"
	"github.com/Faultbox/midgard-ro/internal/game/entity"
	"github.com/Faultbox/midgard-ro/internal/network/packets"
)

func standingAt(aid uint32, x, y int) *packets.Entity {
	return &packets.Entity{
		Kind:    packets.EntityPlayer,
		AID:     aid,
		SpeedMs: 150,
		X:       x,
		Y:       y,
		ToX:     x,
		ToY:     y,
		Dir:     4,
	}
}

func TestUpsertUnitCreatesEntity(t *testing.T) {
	m := entity.NewManager()
	u := standingAt(2000042, 153, 244)
	u.Name = "Someone"
	u.Level = 42
	u.HP, u.MaxHP = 300, 500

	e := upsertUnit(m, u, nil)
	if e == nil {
		t.Fatal("upsertUnit returned nil for a well-formed unit")
	}
	if m.Count() != 1 {
		t.Errorf("manager holds %d entities, want 1", m.Count())
	}
	if e.Name != "Someone" || e.Level != 42 || e.HP != 300 || e.MaxHP != 500 {
		t.Errorf("entity = %q lvl %d hp %d/%d, want Someone lvl 42 hp 300/500",
			e.Name, e.Level, e.HP, e.MaxHP)
	}
	if e.Body == nil {
		t.Fatal("entity has no body, so nothing would be drawn for it")
	}
	if gotX, gotY := e.Body.CurrentCell(); gotX != 153 || gotY != 244 {
		t.Errorf("body is on cell (%d,%d), want (153,244)", gotX, gotY)
	}
}

// TestUpsertUnitKeyedByAID is the regression guard for the field that is easy
// to reach for and wrong: rAthena sends the character id in GID, which is zero
// for every monster and NPC. Keying on it would collapse the whole map onto a
// single entity.
func TestUpsertUnitKeyedByAID(t *testing.T) {
	m := entity.NewManager()

	for i, aid := range []uint32{110000001, 110000002, 110000003} {
		u := standingAt(aid, 100+i, 100)
		u.Kind = packets.EntityMob
		u.GID = 0 // as the server sends it for a monster
		upsertUnit(m, u, nil)
	}

	if got := m.Count(); got != 3 {
		t.Errorf("manager holds %d entities, want 3; monsters all share GID 0, "+
			"so they must be keyed by AID", got)
	}
}

// TestUpsertUnitUpdatesInPlace: the same unit is reported over and over, and
// replacing the entity each time would discard the walk in progress and make
// everything on the map teleport between cells.
func TestUpsertUnitUpdatesInPlace(t *testing.T) {
	m := entity.NewManager()

	first := upsertUnit(m, standingAt(2000042, 10, 10), nil)
	firstBody := first.Body

	second := upsertUnit(m, standingAt(2000042, 11, 10), nil)

	if m.Count() != 1 {
		t.Errorf("manager holds %d entities, want 1", m.Count())
	}
	if second != first {
		t.Error("a repeat report replaced the entity instead of updating it")
	}
	if second.Body != firstBody {
		t.Error("a repeat report replaced the body, discarding any walk in progress")
	}
	if gotX, gotY := second.Body.CurrentCell(); gotX != 11 || gotY != 10 {
		t.Errorf("body is on cell (%d,%d), want (11,10)", gotX, gotY)
	}
}

func TestUpsertUnitStartsWalk(t *testing.T) {
	m := entity.NewManager()
	u := standingAt(2000042, 10, 10)
	u.Moving = true
	u.ToX, u.ToY = 13, 10
	u.Dir = -1 // the walking packet carries no facing

	e := upsertUnit(m, u, nil)
	if e == nil || e.Body == nil {
		t.Fatal("no entity or body for a walking unit")
	}
	if !e.Body.IsWalkingPath() {
		t.Fatal("a unit reported as walking is not walking")
	}
	if e.Body.Direction != entity.DirE {
		t.Errorf("Direction = %d, want DirE(%d); facing has to come from the "+
			"route, since the packet has none", e.Body.Direction, entity.DirE)
	}

	// It should arrive after three straight cells, not sooner or later.
	for i := 0; i < 3; i++ {
		e.Body.Update(entity.DefaultWalkSpeedMs)
	}
	if e.Body.IsWalkingPath() {
		t.Error("still walking after three cell durations over three cells")
	}
	if gotX, gotY := e.Body.CurrentCell(); gotX != 13 || gotY != 10 {
		t.Errorf("ended on cell (%d,%d), want (13,10)", gotX, gotY)
	}
}

// TestUpsertUnitUsesSuppliedPath checks that the server's route is preferred
// over a straight line, since the client has to walk the cells the server
// thinks it is walking or the two drift apart around obstacles.
func TestUpsertUnitUsesSuppliedPath(t *testing.T) {
	m := entity.NewManager()
	u := standingAt(2000042, 0, 0)
	u.Moving = true
	u.ToX, u.ToY = 2, 0

	// A detour, as a pathfinder would produce around an obstacle.
	detour := [][2]int{{0, 0}, {0, 1}, {1, 1}, {2, 1}, {2, 0}}
	called := false
	path := func(fromX, fromY, toX, toY int) [][2]int {
		called = true
		if fromX != 0 || fromY != 0 || toX != 2 || toY != 0 {
			t.Errorf("path asked for (%d,%d)->(%d,%d), want (0,0)->(2,0)",
				fromX, fromY, toX, toY)
		}
		return detour
	}

	e := upsertUnit(m, u, path)
	if !called {
		t.Fatal("the supplied path function was never consulted")
	}
	if got := len(e.Body.Path()); got != len(detour) {
		t.Errorf("walking a %d-cell path, want the %d-cell detour", got, len(detour))
	}
}

// TestUpsertUnitFallsBackToStraightLine: when no route can be produced the
// unit still has to end up where the server says, or it drifts further out of
// step with every step.
func TestUpsertUnitFallsBackToStraightLine(t *testing.T) {
	m := entity.NewManager()
	u := standingAt(2000042, 5, 5)
	u.Moving = true
	u.ToX, u.ToY = 8, 5

	e := upsertUnit(m, u, func(_, _, _, _ int) [][2]int { return nil })
	if !e.Body.IsWalkingPath() {
		t.Fatal("no walk started when the path function returned nothing")
	}

	for i := 0; i < 10 && e.Body.IsWalkingPath(); i++ {
		e.Body.Update(entity.DefaultWalkSpeedMs)
	}
	if gotX, gotY := e.Body.CurrentCell(); gotX != 8 || gotY != 5 {
		t.Errorf("ended on cell (%d,%d), want (8,5)", gotX, gotY)
	}
}

// TestStandingReportStopsAWalk: the server has just said where the unit
// actually is, so finishing a stale route would walk it away from that.
func TestStandingReportStopsAWalk(t *testing.T) {
	m := entity.NewManager()

	moving := standingAt(2000042, 0, 0)
	moving.Moving = true
	moving.ToX, moving.ToY = 9, 0
	e := upsertUnit(m, moving, nil)
	if !e.Body.IsWalkingPath() {
		t.Fatal("test setup: the unit should be walking")
	}

	upsertUnit(m, standingAt(2000042, 3, 0), nil)

	if e.Body.IsWalkingPath() {
		t.Error("a standing report left the old walk running")
	}
	if gotX, gotY := e.Body.CurrentCell(); gotX != 3 || gotY != 0 {
		t.Errorf("body is on cell (%d,%d), want (3,0)", gotX, gotY)
	}
}

// TestWalkKeepsRenderPosition is the smoothing property. Word of a walk always
// reaches us after it began, so the authoritative start cell is behind where
// the unit is drawn. That gap has to be carried and bled off, not snapped
// away, or every unit jerks backward at the start of each step.
func TestWalkKeepsRenderPosition(t *testing.T) {
	m := entity.NewManager()
	e := upsertUnit(m, standingAt(2000042, 10, 10), nil)

	// Draw it a little ahead of where the server last placed it.
	e.Body.RenderX += 3
	drawnBefore := e.Body.RenderX

	walk := standingAt(2000042, 10, 10)
	walk.Moving = true
	walk.ToX, walk.ToY = 14, 10
	upsertUnit(m, walk, nil)

	if e.Body.RenderX != drawnBefore {
		t.Errorf("drawn position jumped from %v to %v when the walk started",
			drawnBefore, e.Body.RenderX)
	}
}

func TestUnitTypeMapping(t *testing.T) {
	tests := []struct {
		kind packets.EntityKind
		want entity.Type
	}{
		{packets.EntityPlayer, entity.TypePlayer},
		{packets.EntityDisguised, entity.TypePlayer},
		{packets.EntityMob, entity.TypeMonster},
		{packets.EntityABR, entity.TypeMonster},
		{packets.EntityBionic, entity.TypeMonster},
		{packets.EntityItem, entity.TypeItem},
		{packets.EntityNPC, entity.TypeNPC},
		{packets.EntityWalkingNPC, entity.TypeNPC},
		{packets.EntitySkill, entity.TypeNPC},
	}

	for _, tt := range tests {
		if got := unitType(tt.kind); got != tt.want {
			t.Errorf("unitType(0x%X) = %d, want %d", tt.kind, got, tt.want)
		}
	}
}

// TestWalkSpeedFallback: a zero would make every step take no time at all,
// which reads as units teleporting rather than as a bad speed value.
func TestWalkSpeedFallback(t *testing.T) {
	tests := []struct {
		name  string
		speed int16
		want  float32
	}{
		{"normal", 150, 150},
		{"slow but plausible", 900, 900},
		{"zero", 0, entity.DefaultWalkSpeedMs},
		{"negative", -1, entity.DefaultWalkSpeedMs},
		{"absurdly large", 30000, entity.DefaultWalkSpeedMs},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := walkSpeedOf(&packets.Entity{SpeedMs: tt.speed}); got != tt.want {
				t.Errorf("walkSpeedOf(%d) = %v, want %v", tt.speed, got, tt.want)
			}
		})
	}
}

func TestUpsertUnitIgnoresUnusableInput(t *testing.T) {
	m := entity.NewManager()

	if upsertUnit(m, nil, nil) != nil {
		t.Error("accepted a nil unit")
	}
	if upsertUnit(nil, standingAt(1, 0, 0), nil) != nil {
		t.Error("accepted a nil manager")
	}
	if upsertUnit(m, standingAt(0, 0, 0), nil) != nil {
		t.Error("accepted a unit with no id; it would collide with every other")
	}
	if m.Count() != 0 {
		t.Errorf("manager holds %d entities after rejecting everything", m.Count())
	}
}

// TestRemoveUnitKeepsPlayer: the local player is in the same registry, and the
// server sends vanish packets for units leaving our view. Dropping ourselves
// would remove the character being played.
func TestRemoveUnitKeepsPlayer(t *testing.T) {
	m := entity.NewManager()
	player := entity.NewEntity(2000042, entity.TypePlayer)
	m.SetPlayer(player)
	upsertUnit(m, standingAt(110000001, 5, 5), nil)

	removeUnit(m, 110000001)
	if m.Count() != 1 {
		t.Errorf("manager holds %d entities, want 1 after removing the other", m.Count())
	}

	removeUnit(m, 2000042)
	if m.Player() == nil || m.Count() != 1 {
		t.Error("a vanish packet for our own id removed the local player")
	}
}

// TestUpdateUnitsAdvancesWalks checks that units actually move between frames.
// Without this they would receive their walks and stand still, since nothing
// else drives a remote unit's body.
func TestUpdateUnitsAdvancesWalks(t *testing.T) {
	m := entity.NewManager()
	u := standingAt(2000042, 0, 0)
	u.Moving = true
	u.ToX, u.ToY = 4, 0
	e := upsertUnit(m, u, nil)

	// Four straight cells, advanced in slices the size of real frames.
	const frame = 16.0
	for elapsed := 0.0; e.Body.IsWalkingPath() && elapsed < 5000; elapsed += frame {
		updateUnits(m, frame, nil)
	}

	if gotX, gotY := e.Body.CurrentCell(); gotX != 4 || gotY != 0 {
		t.Errorf("unit ended on cell (%d,%d), want (4,0)", gotX, gotY)
	}
}

// TestUpdateUnitsSkipsBodilessEntities guards against a nil dereference on the
// local player, which lives in the same registry but has no body of its own.
func TestUpdateUnitsSkipsBodilessEntities(t *testing.T) {
	m := entity.NewManager()
	m.SetPlayer(entity.NewEntity(2000042, entity.TypePlayer))
	upsertUnit(m, standingAt(110000001, 5, 5), nil)

	updateUnits(m, 16, nil) // must not panic
	updateUnits(nil, 16, nil)
}

// TestUpdateUnitsUsesFrameCounts checks the animation is driven by the unit's
// own sprite length rather than the player's.
func TestUpdateUnitsUsesFrameCounts(t *testing.T) {
	m := entity.NewManager()
	e := upsertUnit(m, standingAt(2000042, 5, 5), nil)

	asked := 0
	frames := func(unit *entity.Entity, action, _ int) (int, float32) {
		if unit != e {
			t.Errorf("asked for frame counts of an entity that is not the unit")
		}
		asked++
		if action == entity.ActionWalk {
			return 8, 70
		}
		return 1, 100
	}

	updateUnits(m, 16, frames)
	if asked != 2 {
		t.Errorf("asked for %d frame counts, want 2 (idle and walk)", asked)
	}
}

func TestUnitSpecFollowsAppearance(t *testing.T) {
	m := entity.NewManager()
	u := standingAt(2000042, 5, 5)
	u.Job = 4054
	u.HairStyle = 12
	u.Sex = 0 // rAthena sends 0 for female

	e := upsertUnit(m, u, nil)
	spec := unitSpec(e)

	if spec.Job != 4054 || spec.HairStyle != 12 || !spec.Female {
		t.Errorf("spec = job %d hair %d female %v, want 4054 / 12 / true",
			spec.Job, spec.HairStyle, spec.Female)
	}

	u.Sex = 1
	spec = unitSpec(upsertUnit(m, u, nil))
	if spec.Female {
		t.Error("sex 1 should select the male sprite")
	}
}

// TestDrawableNeedsANamedSprite: a player always resolves, but a monster or
// NPC is only drawn when the client's own table names its sprite. Without that
// check an unknown job falls through to the player path and puts a person on
// screen wherever a Poring stands.
func TestDrawableNeedsANamedSprite(t *testing.T) {
	m := entity.NewManager()

	tests := []struct {
		name string
		kind packets.EntityKind
		job  int16
		want bool
	}{
		{"player", packets.EntityPlayer, 0, true},
		{"disguised player", packets.EntityDisguised, 0, true},
		{"poring", packets.EntityMob, 1002, true},
		{"warp portal", packets.EntityNPC, 45, true},
		{"monster with no sprite name", packets.EntityMob, 30000, false},
		{"npc with no sprite name", packets.EntityNPC, 30000, false},
		{"dropped item", packets.EntityItem, 1002, false},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := standingAt(uint32(1000+i), 5, 5)
			u.Kind = tt.kind
			u.Job = tt.job
			if got := unitIsDrawable(upsertUnit(m, u, nil)); got != tt.want {
				t.Errorf("drawable = %v, want %v", got, tt.want)
			}
		})
	}

	if unitIsDrawable(nil) {
		t.Error("a nil entity is not drawable")
	}
	if unitIsDrawable(entity.NewEntity(1, entity.TypePlayer)) {
		t.Error("an entity with no body has nothing to draw")
	}
}

// TestUnitSpecPicksTheSpriteFamily: a player is composited from a body and a
// head, everything else is one whole sprite, and asking for the wrong family
// resolves to a different unit rather than to nothing.
func TestUnitSpecPicksTheSpriteFamily(t *testing.T) {
	m := entity.NewManager()

	tests := []struct {
		name string
		kind packets.EntityKind
		want charsprite.Kind
	}{
		{"player", packets.EntityPlayer, charsprite.KindPlayer},
		{"monster", packets.EntityMob, charsprite.KindMonster},
		{"npc", packets.EntityNPC, charsprite.KindNPC},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := standingAt(uint32(2000+i), 5, 5)
			u.Kind = tt.kind
			u.Job = 1002
			if got := unitSpec(upsertUnit(m, u, nil)).Kind; got != tt.want {
				t.Errorf("spec kind = %d, want %d", got, tt.want)
			}
		})
	}
}
