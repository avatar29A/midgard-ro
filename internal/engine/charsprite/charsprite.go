// Package charsprite resolves RO character sprite paths, loads the SPR/ACT
// pairs behind them, and bakes head+body into per-frame composites ready for
// upload to the GPU.
//
// It is deliberately GL-free: everything here is pixels and metadata, so it
// can be exercised in tests without a rendering context. Callers own the
// texture upload (see internal/engine/playerrender).
package charsprite

import (
	"fmt"

	"github.com/Faultbox/midgard-ro/internal/engine/sprite"
	"github.com/Faultbox/midgard-ro/pkg/formats"
)

// Loader reads a file out of the GRF archives. It matches the signature the
// game already passes around for textures; the asset manager handles the
// EUC-KR encoding of the Korean paths below.
type Loader func(path string) ([]byte, error)

// Logical actions. These are not ACT indices: players and monsters lay their
// animations out differently, and ActionIndex maps one of these onto the
// index that family actually uses.
const (
	ActionIdle = iota
	ActionWalk
	ActionPickup
	ActionAttack
	ActionHurt
	ActionDie

	// ActionStandby is the armed stance: weapon up, feet apart. It is the
	// only idle a weapon is drawn in, so it is what an armed character stands
	// in — but only while there is something to stand ready against, since
	// holding it the rest of the time reads as a character stuck mid-fight.
	ActionStandby

	// ActionSit is the seated pose, which a player holds until they get up
	// again. Only a player has one — nothing else in the game sits down.
	ActionSit

	// LogicalActions is how many of the above there are.
	LogicalActions
)

const (
	// LoadedActions bounds the ACT indices we bake composites for.
	//
	// Thirteen, which is every set a player's body has. It has to reach that
	// far because a weapon chooses which set it swings from and a sword's is
	// 11 — bounding this at eight left a dagger's attack at set 10 outside
	// the loops entirely, so it baked no frames and the swing ended the
	// instant it began.
	//
	// The count is not the cost: ActionMap.bakes keeps each appearance to the
	// handful of sets it actually maps something onto.
	LoadedActions = 13

	// Directions is the number of facings every action provides.
	Directions = 8

	// ActIntervalTickMs converts an ACT's stored animation interval into
	// milliseconds. RO records the interval in ticks of 25ms, so the 4.0 that
	// nearly every sprite carries means 100ms a frame.
	ActIntervalTickMs = 25.0

	// MaxAnimationFrames bounds how much of one action is baked.
	//
	// Every frame becomes a full-size composited texture, which is enormously
	// wasteful for a single-piece sprite: nothing is being composited, and the
	// same handful of images are re-baked at different offsets. A Kafra's idle
	// runs 99 frames, so baking it whole costs 27 MB and 1584 texture uploads
	// in the frame she first comes into view — a visible stall.
	//
	// The real fix is to stop pre-compositing sprites that have nothing to
	// composite, and draw the frames from their own images with per-frame
	// offsets. Until then this bounds the damage, and Sheet.Dropped reports
	// when it bites rather than leaving a shortened loop to be discovered.
	MaxAnimationFrames = 40

	// HeadStraight is the head direction for looking dead ahead. RO gives a
	// character three head poses — turned each way and straight — which the
	// head sprite stores as its three "frames". The server can change this
	// per entity; until that is wired up everyone looks straight ahead.
	HeadStraight = 0
)

// Spec identifies which character sprites to load.
type Spec struct {
	// Kind selects the sprite family. The zero value is a player.
	Kind Kind

	Job       int  // rAthena job/class id
	Female    bool // sex M/F selects the sprite folder and filename suffix
	HairStyle int  // head sprite number

	// Weapon is the look the server sent for the character's weapon, which is
	// either a weapon class or an item id. Zero is bare-handed. Players only.
	Weapon int

	// The three head gear views the server sent, as accessory ids rather than
	// item ids. Zero is nothing worn in that place. Players only.
	//
	// Kept in the spec because they are part of what the character looks like:
	// a sheet is cached by appearance, and two characters differing only in
	// their hats are two appearances.
	HeadTop, HeadMid, HeadLow int

	// Name identifies a KindItem sprite, which the archive files by the item
	// table's resource name rather than by any id. Ignored for every other
	// kind. Spec is used as a cache key, so this stays a plain string.
	Name string

	// HairColor is the palette the head sprite is drawn through, and the
	// value the server stores. RO recolors hair by swapping the palette
	// rather than by shipping the sprite again, so this picks a .pal file
	// rather than tinting anything.
	HairColor int

	// HeadDirection is which of the three head poses to bake (see
	// HeadStraight). Zero value looks straight ahead.
	HeadDirection int
}

// ACT indices that are not logical actions but are needed to reason about
// them.
const (
	// actSit is the seated pose, ACT set 2 for a player. It sits between the
	// walk and the pick-up in the body's own order, which is why the pick-up
	// is 3 rather than 2.
	actSit = 2

	// actStandby is the armed stance, ACT set 4 for a player. A weapon is
	// drawn in this pose and in the attack, and in nothing else — which is
	// why an armed character stands in it rather than in the bare idle.
	actStandby = 4

	// actUnarmedAttack is the swing a player makes with nothing in hand.
	actUnarmedAttack = 5

	// actWeaponAttacks are the sets a weapon's own art may live in. Which one
	// a weapon uses is the weapon's business — a dagger is drawn in 10 and a
	// sword in 11 — so the sprite is asked rather than told.
	actWeaponAttacks0 = 10
	actWeaponAttacks1 = 11
	actWeaponAttacks2 = 12
)

// Which ACT index each family uses for each logical action.
//
// The two layouts are genuinely different, and the difference is not
// guesswork: a monster's ACT names its own sets through the sound events on
// their frames — poring's set 1 carries poring_move.wav, set 2
// poring_attack.wav, set 3 poring_damage.wav and set 4 poring_die.wav. A
// player's body has thirteen sets against a monster's nine, and follows the
// order the original client's own enum uses.
//
// -1 means the family has no animation for that action, which is not an error:
// nothing on the ground picks anything up but the player.
var actionIndices = map[Kind][LogicalActions]int{
	KindPlayer: {ActionIdle: 0, ActionWalk: 1, ActionPickup: 3, ActionAttack: actUnarmedAttack,
		ActionHurt: 6, ActionDie: 7, ActionStandby: actStandby, ActionSit: actSit},
	KindMonster: {ActionIdle: 0, ActionWalk: 1, ActionPickup: -1, ActionAttack: 2,
		ActionHurt: 3, ActionDie: 4, ActionStandby: -1, ActionSit: -1},
	KindNPC: {ActionIdle: 0, ActionWalk: 1, ActionPickup: -1, ActionAttack: -1,
		ActionHurt: -1, ActionDie: -1, ActionStandby: -1, ActionSit: -1},
	KindItem: {ActionIdle: 0, ActionWalk: -1, ActionPickup: -1, ActionAttack: -1,
		ActionHurt: -1, ActionDie: -1, ActionStandby: -1, ActionSit: -1},
}

// ActionMap is which ACT index each logical action resolves to for one
// appearance.
//
// Per appearance rather than per family because a weapon moves two of them: a
// dagger attacks from set 10 and a sword from set 11, and an armed character
// stands in the combat pose instead of the bare idle.
type ActionMap [LogicalActions]int

// ActionIndex is the ACT index a family uses for a logical action, or -1 when
// it has none.
func ActionIndex(kind Kind, logical int) int {
	if logical < 0 || logical >= LogicalActions {
		return -1
	}

	set, ok := actionIndices[kind]
	if !ok {
		return -1
	}

	return set[logical]
}

// DefaultActionMap is the mapping for a family before a weapon has its say.
func DefaultActionMap(kind Kind) ActionMap {
	if set, ok := actionIndices[kind]; ok {
		return set
	}

	return ActionMap{ActionIdle: 0, ActionWalk: -1, ActionPickup: -1,
		ActionAttack: -1, ActionHurt: -1, ActionDie: -1, ActionStandby: -1, ActionSit: -1}
}

// withWeapon moves the actions a weapon changes.
//
// Which of the attack sets a weapon swings from is the weapon's business, not
// the job's — a dagger's is 10 and a sword's is 11 — so the sprite is asked
// rather than told. The stance is left alone: it is set 4 for every player,
// armed or not, and whether to stand in it is the fight's decision rather
// than the sprite's.
func (m ActionMap) withWeapon(weapon *formats.ACT) ActionMap {
	if weapon == nil {
		return m
	}

	for _, set := range []int{actWeaponAttacks0, actWeaponAttacks1, actWeaponAttacks2} {
		if actHasArt(weapon, set) {
			m[ActionAttack] = set

			break
		}
	}

	return m
}

// actHasArt reports whether an ACT set draws anything at all.
//
// A weapon's ACT carries an entry for every set the body has, and leaves the
// ones it takes no part in pointing at sprite -1. That is the difference
// between "this weapon swings here" and "this set exists".
func actHasArt(act *formats.ACT, set int) bool {
	for dir := 0; dir < Directions; dir++ {
		idx := set*Directions + dir
		if idx >= len(act.Actions) {
			return false
		}

		for _, frame := range act.Actions[idx].Frames {
			for _, layer := range frame.Layers {
				if layer.SpriteID >= 0 {
					return true
				}
			}
		}
	}

	return false
}

// bakes reports whether an ACT index is one this appearance actually uses.
//
// Only the indices the map names: a monster has nothing at 5 or above, so
// baking those would be eight directions of frames nobody looks at — and a
// poring's attack alone is twenty-eight frames a direction.
func (m ActionMap) bakes(action int) bool {
	for _, index := range m {
		if index == action {
			return true
		}
	}

	return false
}

// bakedAction reports whether an ACT index is worth compositing for a family.
//
// Only the indices that family maps a logical action onto: a monster has
// nothing at 5 or above, so baking those would be eight directions of frames
// nobody looks at — and a poring's attack alone is twenty-eight frames a
// direction.
func bakedAction(kind Kind, action int) bool {
	set, ok := actionIndices[kind]
	if !ok {
		return action == ActionIdle
	}

	for _, index := range set {
		if index == action {
			return true
		}
	}

	return false
}

// Sheet is a complete set of pre-composited frames for one character, keyed
// by action*8+direction. Every frame is padded to the same Width x Height so
// they can all share one billboard quad without the sprite jumping between
// frames.
type Sheet struct {
	Width  int
	Height int
	Frames map[int][]Frame

	// IntervalMs is how long each frame of an action is held, taken from the
	// ACT. Zero means the file did not say and the caller should fall back to
	// its own rate.
	IntervalMs [LoadedActions]float32

	// Dropped counts frames left unbaked by MaxAnimationFrames, so a
	// shortened animation is reported rather than silently shortened.
	Dropped int

	// Actions is which ACT index each logical action resolves to for this
	// appearance, after the weapon has had its say.
	Actions ActionMap

	// OriginX and OriginY are where the sprite's own origin sits inside the
	// frame, in pixels from its top left.
	//
	// Every pose is placed by that origin, so it is the one point every frame
	// shares — and it is what a billboard has to stand on. Standing the quad
	// on the frame's bottom edge instead lifts the character by however much
	// empty room the tallest pose in the sheet needed: a knife or a hat grows
	// the frame, and the character rises off the ground with it.
	OriginX, OriginY int

	// Where the standing frame's own art sits inside the padded frame.
	//
	// Every frame is padded onto a canvas as big as the widest and tallest
	// the sheet holds — a swing with a spear in it, or a hat that reaches
	// above the head — so a standing character occupies a fraction of it and
	// the rest is empty. Anything drawing the character flat, rather than as
	// a billboard the camera is looking at, wants that fraction and not the
	// canvas.
	PortraitX, PortraitY, PortraitW, PortraitH int
}

// Frame is one baked animation frame: RGBA pixels at the sheet's dimensions.
type Frame struct {
	Pixels []byte
}

// Assets is everything loaded for one character.
type Assets struct {
	BodySPR *formats.SPR
	BodyACT *formats.ACT
	HeadSPR *formats.SPR
	HeadACT *formats.ACT
	Sheet   *Sheet

	// Gear is what the character wears on its head, lowest first. Empty for
	// a bare head, which is most of them.
	Gear []sprite.Gear

	// WeaponSPR and WeaponACT are the weapon in the character's hand, nil when
	// bare-handed or when the archive has no art for what is held.
	WeaponSPR *formats.SPR
	WeaponACT *formats.ACT

	// BodyPath is the archive path the body actually loaded from, for logs.
	BodyPath string

	// WeaponPath is empty when no weapon sprite was found.
	WeaponPath string
	// HeadPath is empty when no head sprite was found.
	HeadPath string
}

// FrameCount returns how many frames the given action/direction has.
func (s *Sheet) FrameCount(action, direction int) int {
	if s == nil {
		return 0
	}
	return len(s.Frames[action*Directions+direction])
}

// Load resolves the sprite paths for spec, reads them from the archives and
// bakes the composite sheet. A missing head is not fatal — the body renders
// on its own — but a missing body is.
func Load(load Loader, spec Spec) (*Assets, error) {
	candidates := spec.BodyPathCandidates()
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no sprite known for job %d", spec.Job)
	}

	var (
		bodySPR      *formats.SPR
		bodyACT      *formats.ACT
		bodySPRPath  string
		err          error
		firstAttempt = candidates[0][0]
	)
	for _, candidate := range candidates {
		bodySPR, bodyACT, err = loadPair(load, candidate[0], candidate[1], nil)
		if err == nil {
			bodySPRPath = candidate[0]
			break
		}
	}
	if err != nil {
		return nil, fmt.Errorf("body sprite %q: %w", firstAttempt, err)
	}

	a := &Assets{
		BodySPR:  bodySPR,
		BodyACT:  bodyACT,
		BodyPath: bodySPRPath,
	}

	// The head is a separate sprite anchored to the body. Losing it costs us
	// a face, not the character, so failure here is only worth a note.
	headSPRPath, headACTPath := spec.HeadPaths()
	if headSPR, headACT, headErr := loadPair(load, headSPRPath, headACTPath, spec.hairPalette(load)); headErr == nil {
		a.HeadSPR = headSPR
		a.HeadACT = headACT
		a.HeadPath = headSPRPath
	}

	// The weapon is a third sprite, and losing it costs a knife rather than a
	// character, so failure here is worth no more than the head's.
	for _, candidate := range spec.WeaponPathCandidates() {
		if weaponSPR, weaponACT, weaponErr := loadPair(load, candidate[0], candidate[1], nil); weaponErr == nil {
			a.WeaponSPR = weaponSPR
			a.WeaponACT = weaponACT
			a.WeaponPath = candidate[0]

			break
		}
	}

	// What is worn on the head, lowest first so a hat is drawn over a mask.
	// A piece that will not load costs that piece and nothing else.
	for _, view := range [3]int{spec.HeadLow, spec.HeadMid, spec.HeadTop} {
		sprPath, actPath := spec.GearPaths(view)
		if sprPath == "" {
			continue
		}

		gearSPR, gearACT, gearErr := loadPair(load, sprPath, actPath, nil)
		if gearErr != nil {
			// Losing a hat costs a hat, not a character, so it is passed over
			// as quietly as a missing weapon is.
			continue
		}

		a.Gear = append(a.Gear, sprite.Gear{SPR: gearSPR, ACT: gearACT})
	}

	a.Sheet = BuildSheet(a.BodySPR, a.BodyACT, a.HeadSPR, a.HeadACT,
		a.WeaponSPR, a.WeaponACT, a.Gear, spec.HeadDirection, spec.Kind)
	if a.Sheet == nil {
		return nil, fmt.Errorf("body sprite %q produced no frames", bodySPRPath)
	}

	return a, nil
}

// posedSet reports whether an ACT set holds head poses rather than an
// animation.
//
// A player's idle and seated sets each carry three "frames" that are the same
// body art with the head turned three ways. Cycling them animates nothing —
// it swivels the head forever, which is what a seated character was doing
// before this covered the sit as well as the idle.
//
// Only players, and only these two: the combat stance is a six-frame loop, and
// baking one frame of that left the character apparently frozen mid-swing. A
// monster or NPC has no head to pose and its idle is a real animation — a
// Kafra has 99 idle frames of standing and shifting.
func posedSet(kind Kind, action int) bool {
	return kind == KindPlayer && (action == 0 || action == actSit)
}

// BuildSheet composites every frame of every loaded action/direction and pads
// them to a common size. Padding centers horizontally and aligns to the
// bottom, so the character's feet sit at the same place in every frame — the
// billboard quad is foot-anchored, so that keeps them planted on the ground
// as the animation plays.
//
// headDir picks which of the three head poses to bake in (see HeadStraight).
//
// What "standing still" means depends on the kind. For a player it is a single
// frame, not three: RO's idle action stores one entry per head direction
// rather than an animation — the body art is identical across all three, only
// the head turns — so cycling them makes a standing character swivel their
// head forever. Walking is a real animation, and its head stays on the chosen
// pose while the legs cycle.
//
// A monster or NPC has no head to pose, and its idle action is a real
// animation: a Kafra has 99 idle frames of her standing and shifting. Baking
// only the first leaves every one of them frozen, which is what treating them
// like players did.
func BuildSheet(bodySPR *formats.SPR, bodyACT *formats.ACT, headSPR *formats.SPR, headACT *formats.ACT,
	weaponSPR *formats.SPR, weaponACT *formats.ACT, gear []sprite.Gear, headDir int, kind Kind) *Sheet {
	actions := DefaultActionMap(kind).withWeapon(weaponACT)
	if bodySPR == nil || bodyACT == nil || len(bodyACT.Actions) == 0 {
		return nil
	}
	if headDir < 0 {
		headDir = HeadStraight
	}

	// frameIndices returns the (bodyFrame, headFrame) pairs to bake for an
	// action. A player's posed sets index the body by head direction too,
	// because the body's neck anchor moves with the head pose.
	frameIndices := func(action, available int) [][2]int {
		if posedSet(kind, action) {
			return [][2]int{{headDir, headDir}}
		}
		if available > MaxAnimationFrames {
			available = MaxAnimationFrames
		}
		pairs := make([][2]int, 0, available)
		for i := 0; i < available; i++ {
			pairs = append(pairs, [2]int{i, headDir})
		}
		return pairs
	}

	// Count what the cap leaves out, so the caller can report it. A player's
	// unbaked idle entries do not count: those are head poses deliberately
	// left alone, not animation that went missing.
	dropped := 0
	for action := 0; action < LoadedActions; action++ {
		if !actions.bakes(action) {
			continue
		}
		if posedSet(kind, action) {
			continue
		}
		for dir := 0; dir < Directions; dir++ {
			idx := action*Directions + dir
			if idx >= len(bodyACT.Actions) {
				continue
			}
			if extra := len(bodyACT.Actions[idx].Frames) - MaxAnimationFrames; extra > 0 {
				dropped += extra
			}
		}
	}

	// First pass: the frame is the box every pose needs, measured from the
	// sprite's own origin rather than from each pose's own edges.
	//
	// From the origin because that is the one point every pose shares. Sized
	// and placed by their edges instead, a pose that happens to be a few
	// pixels shorter than its neighbor sits a few pixels higher in the frame
	// — and since the frame is what the billboard stands on, the character
	// hops as it turns. The idle alone varies by six pixels between facings.
	left, top := 1<<30, 1<<30
	right, bottom := -(1 << 30), -(1 << 30)
	for action := 0; action < LoadedActions; action++ {
		if !actions.bakes(action) {
			continue
		}
		for dir := 0; dir < Directions; dir++ {
			idx := action*Directions + dir
			if idx >= len(bodyACT.Actions) {
				continue
			}
			for _, fp := range frameIndices(action, len(bodyACT.Actions[idx].Frames)) {
				r := sprite.CompositeWithGear(bodySPR, bodyACT, headSPR, headACT,
					weaponSPR, weaponACT, gear, action, dir, fp[0], fp[1])
				if r.Width == 0 || r.Height == 0 {
					continue
				}

				if r.OffsetX < left {
					left = r.OffsetX
				}
				if r.OffsetY < top {
					top = r.OffsetY
				}
				if r.OffsetX+r.Width > right {
					right = r.OffsetX + r.Width
				}
				if r.OffsetY+r.Height > bottom {
					bottom = r.OffsetY + r.Height
				}
			}
		}
	}
	maxW, maxH := right-left, bottom-top
	if maxW <= 0 || maxH <= 0 {
		return nil
	}

	// Second pass: bake each frame onto a canvas of that size.
	sheet := &Sheet{
		Actions: actions,
		Width:   maxW,
		Height:  maxH,
		OriginX: -left,
		OriginY: -top,
		Frames:  make(map[int][]Frame, LoadedActions*Directions),
		Dropped: dropped,
	}

	// Every direction of an action shares one interval, so the first is the
	// action's rate.
	for action := 0; action < LoadedActions; action++ {
		idx := action * Directions
		if idx < len(bodyACT.Intervals) {
			sheet.IntervalMs[action] = bodyACT.Intervals[idx] * ActIntervalTickMs
		}
	}

	for action := 0; action < LoadedActions; action++ {
		if !actions.bakes(action) {
			continue
		}
		for dir := 0; dir < Directions; dir++ {
			idx := action*Directions + dir
			if idx >= len(bodyACT.Actions) {
				continue
			}
			available := len(bodyACT.Actions[idx].Frames)
			if available == 0 {
				continue
			}

			pairs := frameIndices(action, available)
			frames := make([]Frame, len(pairs))
			for i, fp := range pairs {
				r := sprite.CompositeWithGear(bodySPR, bodyACT, headSPR, headACT,
					weaponSPR, weaponACT, gear, action, dir, fp[0], fp[1])
				if r.Pixels == nil || r.Width == 0 || r.Height == 0 {
					// Keep the slot so frame indices stay contiguous.
					frames[i] = Frame{Pixels: make([]byte, maxW*maxH*4)}
					continue
				}
				frames[i] = Frame{Pixels: place(r, left, top, maxW, maxH)}

				// The standing frame facing the viewer is the one anything
				// drawing the character flat will ask for.
				if action == actions[ActionIdle] && dir == 0 && i == 0 {
					sheet.PortraitX = r.OffsetX - left
					sheet.PortraitY = r.OffsetY - top
					sheet.PortraitW = r.Width
					sheet.PortraitH = r.Height
				}
			}
			sheet.Frames[idx] = frames
		}
	}

	if len(sheet.Frames) == 0 {
		return nil
	}
	return sheet
}

// place copies a composite onto the sheet's common canvas, at the position its
// own origin puts it.
//
// left and top are where the canvas begins in the sprite's coordinates, so
// every pose lands with its origin on the same pixel — which is what stops a
// character shifting as it turns from one facing to the next.
func place(r sprite.CompositeResult, left, top, maxW, maxH int) []byte {
	out := make([]byte, maxW*maxH*4)
	offsetX := r.OffsetX - left
	offsetY := r.OffsetY - top

	for py := 0; py < r.Height; py++ {
		dstY := offsetY + py
		if dstY < 0 || dstY >= maxH {
			continue
		}
		for px := 0; px < r.Width; px++ {
			dstX := offsetX + px
			if dstX < 0 || dstX >= maxW {
				continue
			}
			src := (py*r.Width + px) * 4
			dst := (dstY*maxW + dstX) * 4
			copy(out[dst:dst+4], r.Pixels[src:src+4])
		}
	}
	return out
}

// hairPalette reads the palette this look's hair is drawn through, or nil to
// use the sprite's own.
//
// A missing file is not an error worth failing a character over: it means this
// style has no such color, and the sprite's own palette is a correct answer
// rather than a broken one.
func (s Spec) hairPalette(load Loader) *formats.SPRPalette {
	if load == nil || s.Kind != KindPlayer {
		return nil
	}

	path := s.HairPalettePath()
	if path == "" {
		return nil
	}

	data, err := load(path)
	if err != nil {
		return nil
	}

	pal, err := formats.ParsePAL(data)
	if err != nil {
		return nil
	}

	return pal
}

func loadPair(load Loader, sprPath, actPath string, pal *formats.SPRPalette) (*formats.SPR, *formats.ACT, error) {
	if load == nil {
		return nil, nil, fmt.Errorf("no asset loader")
	}

	sprData, err := load(sprPath)
	if err != nil {
		return nil, nil, fmt.Errorf("reading %s: %w", sprPath, err)
	}
	spr, err := formats.ParseSPRWithPalette(sprData, pal)
	if err != nil {
		return nil, nil, fmt.Errorf("parsing %s: %w", sprPath, err)
	}

	actData, err := load(actPath)
	if err != nil {
		return nil, nil, fmt.Errorf("reading %s: %w", actPath, err)
	}
	act, err := formats.ParseACT(actData)
	if err != nil {
		return nil, nil, fmt.Errorf("parsing %s: %w", actPath, err)
	}

	return spr, act, nil
}
