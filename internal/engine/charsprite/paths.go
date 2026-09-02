package charsprite

import "fmt"

// RO stores character art under Korean folder names, as raw EUC-KR bytes in
// the archive. We write them as UTF-8 here and let assets.Manager re-encode
// on lookup — the same arrangement the UI window skin uses.
//
//	인간족   "human race"  — the character sprite root
//	몸통     "body"        — job sprites, one per job/sex
//	머리통   "head"        — hair sprites, numbered per sex
//	남 / 여  male / female — both a folder and a filename suffix
const (
	spriteRoot = `data\sprite\인간족\`
	bodyDir    = spriteRoot + `몸통\`
	headDir    = spriteRoot + `머리통\`

	// Monsters and NPCs are single sprites rather than a body with a head
	// anchored to it, and live outside the character tree.
	//
	//	몬스터  "monster"
	monsterDir = `data\sprite\몬스터\`
	npcDir     = `data\sprite\npc\`

	// A player's weapon is a third sprite, filed under the job rather than
	// with the bodies:
	//
	//	data\sprite\인간족\초보자\초보자_남_단검.spr
	//
	// so the directory is the job's own name, which is also its body sprite's
	// basename.
	weaponRoot = spriteRoot

	// Items lying on the ground are named by the archive's own item table
	// rather than by a job id, so they are the one family identified by a
	// name instead of a number.
	//
	//	아이템  "item"
	itemDir = `data\sprite\아이템\`

	// Head gear — hats, masks, glasses — is filed by sex under its own
	// folder, named by the table the client itself ships:
	//
	//	data\sprite\악세사리\남\남_고글.spr
	//
	//	악세사리  "accessory"
	//
	// The names in that table already begin with the underscore, so the file
	// is the sex marker with the name appended straight on.
	gearDir = `data\sprite\악세사리\`

	male   = `남`
	female = `여`
)

// Kind selects which family of sprites a unit is drawn from. Players are
// composited from a body and a head; everything else is a single sprite named
// by the generated table in spritenames.go.
type Kind uint8

// Sprite families. The zero value is a player, so a Spec that predates this
// still resolves the way it did.
const (
	KindPlayer Kind = iota
	KindMonster
	KindNPC
	KindItem
)

// WeaponSuffix is the suffix the sprite of a weapon look is filed under, and
// whether the look resolved to one at all.
//
// A look is one of two things and nothing on the wire says which. The
// character list at login carries the weapon *class*, straight out of the
// server's char table. Every change after that carries the item *id*: rAthena
// sends an item's view id if its database row has one and the item's own id if
// it does not, and in the renewal database exactly one weapon of 2806 has a
// view id. So the class arrives once, at login, and the id arrives every time
// after.
//
// The two ranges do not meet — classes stop at 102 and item ids start at 501 —
// so the value answers the question itself, and the class table is tried
// first.
func WeaponSuffix(look int) (string, bool) {
	if suffix, ok := weaponSpriteNames[look]; ok {
		return suffix, true
	}

	if class, ok := itemSpriteClass[look]; ok {
		suffix, ok := weaponSpriteNames[class]

		return suffix, ok
	}

	return "", false
}

// SpriteName returns the sprite basename for a monster or NPC job id, and
// whether the id was in the client's table.
//
// The name can carry a subdirectory: a few sprites sit in a folder under the
// monster directory rather than directly in it.
func SpriteName(job int) (string, bool) {
	name, ok := spriteNames[job]
	return name, ok
}

// jobSpriteNames maps rAthena job ids to the body sprite basename. Only the
// jobs a character can currently be are listed; anything unknown falls back
// to Novice rather than failing to render.
//
// The trailing `_남` / `_여` suffix is appended by BodyPaths.
var jobSpriteNames = map[int]string{
	0:  `초보자`,   // Novice
	1:  `검사`,    // Swordman
	2:  `마법사`,   // Magician
	3:  `궁수`,    // Archer
	4:  `성직자`,   // Acolyte
	5:  `상인`,    // Merchant
	6:  `도둑`,    // Thief
	7:  `기사`,    // Knight
	8:  `프리스트`,  // Priest
	9:  `위저드`,   // Wizard
	10: `제철공`,   // Blacksmith
	11: `헌터`,    // Hunter
	12: `어세신`,   // Assassin
	14: `크루세이더`, // Crusader
	15: `몽크`,    // Monk
	16: `세이지`,   // Sage
	17: `로그`,    // Rogue
	18: `연금술사`,  // Alchemist
	19: `바드`,    // Bard
	20: `무희`,    // Dancer (female only)
	23: `슈퍼노비스`, // Super Novice
	24: `건너`,    // Gunslinger
	25: `닌자`,    // Ninja
}

// FallbackJob is the job we render when the class id isn't one we know. Every
// account has a Novice sprite available, so it always resolves.
const FallbackJob = 0

// JobSpriteName returns the body sprite basename for a job id, and whether
// the id was recognized.
func JobSpriteName(job int) (string, bool) {
	name, ok := jobSpriteNames[job]
	if !ok {
		return jobSpriteNames[FallbackJob], false
	}
	return name, true
}

// sexSuffix is the Korean male/female marker used for both the folder and the
// filename suffix.
func (s Spec) sexSuffix() string {
	if s.Female {
		return female
	}
	return male
}

// BodyPaths returns the archive paths of the body SPR and ACT.
//
// For anything that is not a player this is the first candidate only; use
// BodyPathCandidates, which also offers the other directory.
func (s Spec) BodyPaths() (sprPath, actPath string) {
	candidates := s.BodyPathCandidates()
	if len(candidates) == 0 {
		return "", ""
	}
	return candidates[0][0], candidates[0][1]
}

// BodyPathCandidates returns the archive paths to try, in order.
//
// Monsters and NPCs are not cleanly separated in the archive: the same sprite
// can appear under either directory, and some monsters are only under the NPC
// one. Trying the other directory after the expected one costs a failed
// lookup and covers those.
func (s Spec) BodyPathCandidates() [][2]string {
	if s.Kind == KindPlayer {
		job, _ := JobSpriteName(s.Job)
		sex := s.sexSuffix()
		base := fmt.Sprintf(`%s%s\%s_%s`, bodyDir, sex, job, sex)
		return [][2]string{{base + ".spr", base + ".act"}}
	}

	// A ground item is named, not numbered: the item table gives a resource
	// name and that is the whole of its identity. Without one there is
	// nothing to draw, and falling through to the job table below would name
	// a monster sprite for an item id that happens to collide with one.
	if s.Kind == KindItem {
		if s.Name == "" {
			return nil
		}
		base := itemDir + s.Name

		return [][2]string{{base + ".spr", base + ".act"}}
	}

	name, ok := SpriteName(s.Job)
	if !ok {
		// No name for this id: there is nothing to guess at. A player sprite
		// would resolve, but it would be the wrong thing entirely.
		return nil
	}

	dirs := [2]string{monsterDir, npcDir}
	if s.Kind == KindNPC {
		dirs = [2]string{npcDir, monsterDir}
	}

	candidates := make([][2]string, 0, len(dirs))
	for _, dir := range dirs {
		base := dir + name
		candidates = append(candidates, [2]string{base + ".spr", base + ".act"})
	}
	return candidates
}

// WeaponPathCandidates returns the archive paths of the weapon sprite to try,
// in order, or nothing when the character is bare-handed or is not a player.
//
// Two namings, both real. A weapon with art of its own is filed under the item
// id — the Novice has 1207, 1208, 1216 and a handful more — and everything
// else under its class, which is what the great majority use. The look the
// server sends is one or the other, so both are tried and the archive settles
// it.
func (s Spec) WeaponPathCandidates() [][2]string {
	if s.Kind != KindPlayer || s.Weapon <= 0 {
		return nil
	}

	job, _ := JobSpriteName(s.Job)
	sex := s.sexSuffix()
	dir := fmt.Sprintf(`%s%s\`, weaponRoot, job)

	candidates := make([][2]string, 0, 2)

	base := ""
	if suffix, ok := WeaponSuffix(s.Weapon); ok {
		base = fmt.Sprintf(`%s%s_%s%s`, dir, job, sex, suffix)
		candidates = append(candidates, [2]string{base + ".spr", base + ".act"})
	}

	// The look as a file name of its own, for art the tables do not know: a
	// weapon added to the server after this client's data was made is filed by
	// its id, and trying it costs one load that fails. For a weapon that does
	// have art of its own the suffix above is already that id, so the two
	// agree and only one is offered.
	byID := fmt.Sprintf(`%s%s_%s_%d`, dir, job, sex, s.Weapon)
	if byID != base {
		candidates = append(candidates, [2]string{byID + ".spr", byID + ".act"})
	}

	return candidates
}

// AccessoryName is the sprite basename for a head gear view id, from the
// client's own table.
func AccessoryName(view int) (string, bool) {
	name, ok := accessoryNames[view]

	return name, ok
}

// GearPaths returns the archive paths of one piece of head gear.
//
// A view id of zero is nothing worn, and one the table does not know is gear
// newer than the table — both give no paths, which draws the character
// without it rather than failing to draw the character.
func (s Spec) GearPaths(view int) (sprPath, actPath string) {
	if s.Kind != KindPlayer || view <= 0 {
		return "", ""
	}

	name, ok := AccessoryName(view)
	if !ok {
		return "", ""
	}

	sex := s.sexSuffix()
	base := fmt.Sprintf(`%s%s\%s%s`, gearDir, sex, sex, name)

	return base + ".spr", base + ".act"
}

// HeadPaths returns the archive paths of the head SPR and ACT for the
// character's hair style. Only players have a separate head; for anything else
// the sprite is whole and this returns nothing.
func (s Spec) HeadPaths() (sprPath, actPath string) {
	if s.Kind != KindPlayer {
		return "", ""
	}
	sex := s.sexSuffix()
	hair := s.HairStyle
	if hair <= 0 {
		hair = 1 // style 0 isn't a file; every sex has a style 1
	}
	base := fmt.Sprintf(`%s%s\%d_%s`, headDir, sex, hair, sex)
	return base + ".spr", base + ".act"
}
