package charsprite

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Faultbox/midgard-ro/pkg/formats"
)

func TestBodyPaths(t *testing.T) {
	tests := []struct {
		name    string
		spec    Spec
		wantSPR string
	}{
		{
			// The seeded test character: class 0, male, hair 1. Verified
			// present in data.grf.
			name:    "male novice",
			spec:    Spec{Job: 0, HairStyle: 1},
			wantSPR: `data\sprite\인간족\몸통\남\초보자_남.spr`,
		},
		{
			name:    "female novice",
			spec:    Spec{Job: 0, Female: true, HairStyle: 1},
			wantSPR: `data\sprite\인간족\몸통\여\초보자_여.spr`,
		},
		{
			name:    "male swordman",
			spec:    Spec{Job: 1},
			wantSPR: `data\sprite\인간족\몸통\남\검사_남.spr`,
		},
		{
			// An id we don't have a name for must still resolve, as a novice.
			name:    "unknown job falls back to novice",
			spec:    Spec{Job: 4211},
			wantSPR: `data\sprite\인간족\몸통\남\초보자_남.spr`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotSPR, gotACT := tt.spec.BodyPaths()
			if gotSPR != tt.wantSPR {
				t.Errorf("body SPR = %q, want %q", gotSPR, tt.wantSPR)
			}
			if want := strings.TrimSuffix(tt.wantSPR, ".spr") + ".act"; gotACT != want {
				t.Errorf("body ACT = %q, want %q", gotACT, want)
			}
		})
	}
}

func TestHeadPaths(t *testing.T) {
	tests := []struct {
		name    string
		spec    Spec
		wantSPR string
	}{
		{
			name:    "male hair 1",
			spec:    Spec{HairStyle: 1},
			wantSPR: `data\sprite\인간족\머리통\남\1_남.spr`,
		},
		{
			name:    "female hair 12",
			spec:    Spec{Female: true, HairStyle: 12},
			wantSPR: `data\sprite\인간족\머리통\여\12_여.spr`,
		},
		{
			// Style 0 isn't a file in the archive; every sex has a style 1.
			name:    "hair 0 becomes hair 1",
			spec:    Spec{HairStyle: 0},
			wantSPR: `data\sprite\인간족\머리통\남\1_남.spr`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotSPR, _ := tt.spec.HeadPaths()
			if gotSPR != tt.wantSPR {
				t.Errorf("head SPR = %q, want %q", gotSPR, tt.wantSPR)
			}
		})
	}
}

func TestJobSpriteNameReportsUnknown(t *testing.T) {
	if _, ok := JobSpriteName(0); !ok {
		t.Error("job 0 (Novice) should be known")
	}
	name, ok := JobSpriteName(9999)
	if ok {
		t.Error("job 9999 should report as unknown")
	}
	if novice, _ := JobSpriteName(FallbackJob); name != novice {
		t.Errorf("unknown job returned %q, want the novice sprite %q", name, novice)
	}
}

func TestBuildSheetPadsFramesToUniformSize(t *testing.T) {
	// Two directions whose sprites differ in size: the sheet must pad both to
	// the larger one, so switching facing doesn't resize the billboard.
	bodySPR := &formats.SPR{Images: []formats.SPRImage{
		makeImage(20, 40),
		makeImage(30, 60),
	}}

	act := &formats.ACT{}
	for dir := 0; dir < Directions*LoadedActions; dir++ {
		spriteID := int32(0)
		if dir == 3 {
			spriteID = 1 // one oversized direction
		}
		act.Actions = append(act.Actions, formats.Action{
			Frames: []formats.Frame{{
				Layers:       []formats.Layer{{SpriteID: spriteID, ScaleX: 1, ScaleY: 1}},
				AnchorPoints: []formats.AnchorPoint{{X: 0, Y: 0}},
			}},
		})
	}

	sheet := BuildSheet(bodySPR, act, nil, nil, nil, nil, nil, HeadStraight, KindPlayer)
	if sheet == nil {
		t.Fatal("BuildSheet returned nil")
	}
	if sheet.Width != 30 || sheet.Height != 60 {
		t.Errorf("sheet = %dx%d, want 30x60 (the largest frame)", sheet.Width, sheet.Height)
	}

	want := sheet.Width * sheet.Height * 4
	for key, frames := range sheet.Frames {
		for i, f := range frames {
			if len(f.Pixels) != want {
				t.Errorf("frame %d of set %d has %d bytes, want %d", i, key, len(f.Pixels), want)
			}
		}
	}
}

func TestBuildSheetCoversAllDirections(t *testing.T) {
	bodySPR := &formats.SPR{Images: []formats.SPRImage{makeImage(20, 40)}}
	act := &formats.ACT{}
	for i := 0; i < Directions*LoadedActions; i++ {
		act.Actions = append(act.Actions, formats.Action{
			Frames: []formats.Frame{
				{Layers: []formats.Layer{{SpriteID: 0, ScaleX: 1, ScaleY: 1}}},
				{Layers: []formats.Layer{{SpriteID: 0, ScaleX: 1, ScaleY: 1}}},
			},
		})
	}

	sheet := BuildSheet(bodySPR, act, nil, nil, nil, nil, nil, HeadStraight, KindPlayer)
	if sheet == nil {
		t.Fatal("BuildSheet returned nil")
	}

	for dir := 0; dir < Directions; dir++ {
		// Idle collapses to the single chosen head pose; walk keeps its
		// animation frames.
		if got := sheet.FrameCount(ActionIdle, dir); got != 1 {
			t.Errorf("idle dir %d has %d frames, want 1 (one head pose, not an animation)", dir, got)
		}
		if got := sheet.FrameCount(ActionWalk, dir); got != 2 {
			t.Errorf("walk dir %d has %d frames, want 2", dir, got)
		}
	}
}

func TestBuildSheetNilBody(t *testing.T) {
	if got := BuildSheet(nil, nil, nil, nil, nil, nil, nil, HeadStraight, KindPlayer); got != nil {
		t.Error("BuildSheet with no body should return nil")
	}
}

func TestLoadMissingBodyIsAnError(t *testing.T) {
	loader := func(string) ([]byte, error) { return nil, fmt.Errorf("not found") }
	if _, err := Load(loader, Spec{Job: 0}); err == nil {
		t.Error("a missing body sprite must be an error")
	}
}

func makeImage(w, h int) formats.SPRImage {
	px := make([]byte, w*h*4)
	for i := 0; i < w*h; i++ {
		px[i*4+3] = 255
	}
	return formats.SPRImage{Width: uint16(w), Height: uint16(h), Pixels: px}
}

// TestSpriteNameTableIsPopulated guards the generated table against being
// regenerated into nothing. It is produced by walking Lua bytecode, and a
// change in that format fails by yielding a handful of entries rather than by
// erroring, which would quietly stop every monster and NPC from being drawn.
func TestSpriteNameTableIsPopulated(t *testing.T) {
	if len(spriteNames) < 1500 {
		t.Errorf("sprite name table has %d entries; the client's own table has "+
			"about 2000, so this has probably been regenerated wrong", len(spriteNames))
	}

	// A few well-known ids, spread across the ranges the table covers.
	tests := []struct {
		job  int
		want string
	}{
		{1002, "Poring"},
		{1113, "DROPS"},
		{45, "1_ETC_01"}, // warp portal
	}

	for _, tt := range tests {
		got, ok := SpriteName(tt.job)
		if !ok {
			t.Errorf("job %d is not in the table", tt.job)
			continue
		}
		if got != tt.want {
			t.Errorf("SpriteName(%d) = %q, want %q", tt.job, got, tt.want)
		}
	}
}

func TestSpriteNameReportsUnknown(t *testing.T) {
	if _, ok := SpriteName(999999); ok {
		t.Error("an id with no sprite must report unknown, not a wrong name")
	}
}

// TestMonsterAndNPCPathsTryBothDirectories: the archive does not separate the
// two cleanly — the same sprite can appear under either, and some monsters
// live only under the NPC directory — so both are tried, expected one first.
func TestMonsterAndNPCPathsTryBothDirectories(t *testing.T) {
	monster := Spec{Kind: KindMonster, Job: 1002}.BodyPathCandidates()
	if len(monster) != 2 {
		t.Fatalf("monster gave %d candidates, want 2", len(monster))
	}
	if !strings.Contains(monster[0][0], `몬스터`) {
		t.Errorf("monster tries %q first, want the monster directory", monster[0][0])
	}
	if !strings.Contains(monster[1][0], `npc`) {
		t.Errorf("monster falls back to %q, want the npc directory", monster[1][0])
	}

	npc := Spec{Kind: KindNPC, Job: 1002}.BodyPathCandidates()
	if len(npc) != 2 {
		t.Fatalf("npc gave %d candidates, want 2", len(npc))
	}
	if !strings.Contains(npc[0][0], `npc`) {
		t.Errorf("npc tries %q first, want the npc directory", npc[0][0])
	}

	// Both must ask for a matching .spr and .act.
	for _, candidate := range append(monster, npc...) {
		if !strings.HasSuffix(candidate[0], ".spr") || !strings.HasSuffix(candidate[1], ".act") {
			t.Errorf("candidate %v is not an spr/act pair", candidate)
		}
	}
}

func TestUnknownMonsterHasNoCandidates(t *testing.T) {
	if got := (Spec{Kind: KindMonster, Job: 999999}).BodyPathCandidates(); got != nil {
		t.Errorf("candidates = %v, want none; guessing would resolve to a player sprite", got)
	}
}

// TestNonPlayersHaveNoHead: monsters and NPCs are one whole sprite, and asking
// for a head would composite a hairstyle onto a Poring.
func TestNonPlayersHaveNoHead(t *testing.T) {
	for _, kind := range []Kind{KindMonster, KindNPC} {
		spr, act := Spec{Kind: kind, Job: 1002}.HeadPaths()
		if spr != "" || act != "" {
			t.Errorf("kind %d asked for a head sprite (%q, %q)", kind, spr, act)
		}
	}
}

// TestIdleIsAnimatedForNonPlayers is the difference between a town that moves
// and one that is frozen. A player's idle action holds three head poses rather
// than an animation, so only one is baked; a monster or NPC has no head to
// pose and its idle is a real animation, so all of it is baked.
func TestIdleIsAnimatedForNonPlayers(t *testing.T) {
	const idleFrames = 5

	act := &formats.ACT{}
	for i := 0; i < LoadedActions*Directions; i++ {
		action := formats.Action{}
		for f := 0; f < idleFrames; f++ {
			action.Frames = append(action.Frames, formats.Frame{
				Layers:       []formats.Layer{{SpriteID: 0, ScaleX: 1, ScaleY: 1}},
				AnchorPoints: []formats.AnchorPoint{{X: 0, Y: 0}},
			})
		}
		act.Actions = append(act.Actions, action)
	}
	bodySPR := &formats.SPR{Images: []formats.SPRImage{makeImage(8, 8)}}

	player := BuildSheet(bodySPR, act, nil, nil, nil, nil, nil, HeadStraight, KindPlayer)
	if player == nil {
		t.Fatal("player sheet is nil")
	}
	if got := player.FrameCount(ActionIdle, 0); got != 1 {
		t.Errorf("player idle baked %d frames, want 1; the extra entries are head "+
			"poses, and cycling them swivels a standing character's head", got)
	}

	for _, kind := range []Kind{KindMonster, KindNPC} {
		sheet := BuildSheet(bodySPR, act, nil, nil, nil, nil, nil, HeadStraight, kind)
		if sheet == nil {
			t.Fatalf("kind %d sheet is nil", kind)
		}
		if got := sheet.FrameCount(ActionIdle, 0); got != idleFrames {
			t.Errorf("kind %d idle baked %d frames, want %d; standing still is a "+
				"real animation for anything without a head", kind, got, idleFrames)
		}
	}
}

// TestAnimationIsCappedAndReported: every frame becomes a full-size texture,
// so a long animation is expensive out of proportion to what it shows — a
// Kafra's 99-frame idle costs 27 MB and 1584 uploads in the frame she appears.
// The cap bounds that, and Dropped says when it applied, so a shortened loop
// is reported rather than found later by eye.
func TestAnimationIsCappedAndReported(t *testing.T) {
	const frames = MaxAnimationFrames + 17

	act := &formats.ACT{}
	for i := 0; i < LoadedActions*Directions; i++ {
		action := formats.Action{}
		for f := 0; f < frames; f++ {
			action.Frames = append(action.Frames, formats.Frame{
				Layers:       []formats.Layer{{SpriteID: 0, ScaleX: 1, ScaleY: 1}},
				AnchorPoints: []formats.AnchorPoint{{X: 0, Y: 0}},
			})
		}
		act.Actions = append(act.Actions, action)
	}
	bodySPR := &formats.SPR{Images: []formats.SPRImage{makeImage(8, 8)}}

	sheet := BuildSheet(bodySPR, act, nil, nil, nil, nil, nil, HeadStraight, KindNPC)
	if sheet == nil {
		t.Fatal("BuildSheet returned nil")
	}
	if got := sheet.FrameCount(ActionIdle, 0); got != MaxAnimationFrames {
		t.Errorf("baked %d idle frames, want the cap of %d", got, MaxAnimationFrames)
	}

	// Every baked action and direction over the cap contributes its excess.
	// Actions inside LoadedActions that we deliberately do not bake — sitting
	// — contribute nothing, because nothing of theirs was dropped.
	baked := 0
	for action := 0; action < LoadedActions; action++ {
		if bakedAction(KindNPC, action) {
			baked++
		}
	}

	want := 17 * baked * Directions
	if sheet.Dropped != want {
		t.Errorf("Dropped = %d, want %d", sheet.Dropped, want)
	}
}

// TestPlayerHeadPosesAreNotCountedAsDropped: a player's idle holds three head
// poses and only one is baked, by design. Counting the other two as dropped
// animation would report a problem that is not there.
func TestPlayerHeadPosesAreNotCountedAsDropped(t *testing.T) {
	act := &formats.ACT{}
	for i := 0; i < LoadedActions*Directions; i++ {
		action := formats.Action{}
		for f := 0; f < 3; f++ {
			action.Frames = append(action.Frames, formats.Frame{
				Layers:       []formats.Layer{{SpriteID: 0, ScaleX: 1, ScaleY: 1}},
				AnchorPoints: []formats.AnchorPoint{{X: 0, Y: 0}},
			})
		}
		act.Actions = append(act.Actions, action)
	}
	bodySPR := &formats.SPR{Images: []formats.SPRImage{makeImage(8, 8)}}

	sheet := BuildSheet(bodySPR, act, nil, nil, nil, nil, nil, HeadStraight, KindPlayer)
	if sheet == nil {
		t.Fatal("BuildSheet returned nil")
	}
	if sheet.Dropped != 0 {
		t.Errorf("Dropped = %d, want 0; the unbaked idle entries are head poses", sheet.Dropped)
	}
}

// TestLoadedActionsReachesTheWeaponAttacks: a weapon chooses which set it
// swings from, and a sword's is 11. Bounding the bake below that left the
// attack with no frames at all, which showed up as a swing that ended the
// instant it began.
func TestLoadedActionsReachesTheWeaponAttacks(t *testing.T) {
	for _, set := range []int{actStandby, actUnarmedAttack, actWeaponAttacks0, actWeaponAttacks1, actWeaponAttacks2} {
		if set >= LoadedActions {
			t.Errorf("set %d is outside the bake range of %d, so nothing would be baked for it",
				set, LoadedActions)
		}
	}
}

// TestActionMapBakesOnlyWhatItUses: the bake range reaches thirteen sets, but
// an appearance uses a handful. Baking the rest would be eight directions of
// frames nobody looks at — and a poring's attack alone is twenty-eight frames
// a direction.
func TestActionMapBakesOnlyWhatItUses(t *testing.T) {
	m := DefaultActionMap(KindMonster)

	baked := 0
	for action := 0; action < LoadedActions; action++ {
		if m.bakes(action) {
			baked++
		}
	}

	// Idle, walk, attack, hurt, die — and nothing else.
	if baked != 5 {
		t.Errorf("a monster bakes %d sets, want the 5 it maps something onto", baked)
	}

	if m.bakes(actWeaponAttacks0) {
		t.Error("a monster bakes a player's weapon attack set")
	}
}

// TestPosedSetsAreTheIdleAndTheSit: these two ACT sets hold three head poses
// rather than three frames of animation, so baking them as a loop swivels the
// head forever. Nothing else is posed — the combat stance is a real six-frame
// loop, and a monster has no head to pose at all.
func TestPosedSetsAreTheIdleAndTheSit(t *testing.T) {
	for _, tc := range []struct {
		kind   Kind
		action int
		want   bool
	}{
		{KindPlayer, 0, true},
		{KindPlayer, actSit, true},
		{KindPlayer, actStandby, false},
		{KindPlayer, 1, false},
		{KindMonster, 0, false},
		{KindMonster, actSit, false},
		{KindNPC, 0, false},
	} {
		if got := posedSet(tc.kind, tc.action); got != tc.want {
			t.Errorf("posedSet(%v, %d) = %v, want %v", tc.kind, tc.action, got, tc.want)
		}
	}
}

// TestGearPaths: head gear is filed by sex under its own folder, named by the
// client's own table — and the names in that table already begin with the
// underscore, so the file is the sex marker with the name appended straight
// on.
func TestGearPaths(t *testing.T) {
	spec := Spec{Job: 0, HairStyle: 1}

	spr, act := spec.GearPaths(1)
	if spr != `data\sprite\악세사리\남\남_고글.spr` {
		t.Errorf("male goggles spr = %q", spr)
	}
	if act != `data\sprite\악세사리\남\남_고글.act` {
		t.Errorf("male goggles act = %q", act)
	}

	female := Spec{Job: 0, HairStyle: 1, Female: true}
	if spr, _ := female.GearPaths(1); spr != `data\sprite\악세사리\여\여_고글.spr` {
		t.Errorf("female goggles spr = %q", spr)
	}
}

// TestGearPathsForNothingWorn: zero is a bare head, and a view the table does
// not know is gear newer than the table. Both draw the character without it
// rather than failing to draw the character.
func TestGearPathsForNothingWorn(t *testing.T) {
	spec := Spec{Job: 0, HairStyle: 1}

	for _, view := range []int{0, -1, 999999} {
		if spr, act := spec.GearPaths(view); spr != "" || act != "" {
			t.Errorf("view %d gave %q / %q, want nothing", view, spr, act)
		}
	}

	// Nothing but a player wears gear.
	monster := Spec{Kind: KindMonster, Job: 1002}
	if spr, _ := monster.GearPaths(1); spr != "" {
		t.Errorf("a monster was given head gear: %q", spr)
	}
}

// TestAccessoryTableHasTheOnesTheClientShips: the table is generated, so this
// is a check that it was generated at all and joined onto real names rather
// than left empty.
func TestAccessoryTableHasTheOnesTheClientShips(t *testing.T) {
	if len(accessoryNames) < 500 {
		t.Errorf("the accessory table holds %d entries, which is too few to be the client's",
			len(accessoryNames))
	}

	for _, view := range []int{1, 3} {
		if name, ok := AccessoryName(view); !ok || name == "" {
			t.Errorf("view %d is missing from the table", view)
		}
	}

	if _, ok := AccessoryName(0); ok {
		t.Error("zero is nothing worn and should not name a sprite")
	}
}
