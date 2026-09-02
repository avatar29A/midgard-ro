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

	// 도람족 "Doram race" — the Summoner and what it grows into are drawn
	// from a directory of their own rather than from the human one, so the
	// job decides which root its body comes from.
	doramSpriteRoot = `data\sprite\도람족\`
	doramBodyDir    = doramSpriteRoot + `몸통\`
	headDir         = spriteRoot + `머리통\`

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

	// The transcended jobs, which rebirth reaches. The client keeps this
	// mapping in its own binary rather than in a table in the archive — the
	// lua files name NPCs and monsters, not player bodies — so unlike the
	// sprite table next door this is written out rather than generated. Every
	// name below was checked against the archive: each has a body sprite for
	// the sex that can hold the job, and none for the sex that cannot.
	//
	// The first classes rebirth into look no different, and share their
	// sprites. The archive does carry `_h` twins of them — 기사_h against 기사
	// — but those are the same art: a byte or two apart in a quarter of a
	// megabyte, against 로드나이트 which differs by thirteen thousand.
	4001: `초보자`,    // High Novice
	4002: `검사`,     // High Swordman
	4003: `마법사`,    // High Mage
	4004: `궁수`,     // High Archer
	4005: `성직자`,    // High Acolyte
	4006: `상인`,     // High Merchant
	4007: `도둑`,     // High Thief
	4008: `로드나이트`,  // Lord Knight
	4009: `하이프리`,   // High Priest
	4010: `하이위저드`,  // High Wizard
	4011: `화이트스미스`, // Whitesmith
	4012: `스나이퍼`,   // Sniper
	4013: `어쌔신크로스`, // Assassin Cross
	4014: `로드페코`,   // Lord Knight on a Peco
	4015: `팔라딘`,    // Paladin
	4016: `챔피온`,    // Champion
	4017: `프로페서`,   // Professor
	4018: `스토커`,    // Stalker
	4019: `크리에이터`,  // Creator
	4020: `클라운`,    // Clown, male only
	4021: `집시`,     // Gypsy, female only
	4022: `페코팔라딘`,  // Paladin on a Peco

	// The Doram jobs, whose bodies come from a directory of their own — see
	// bodyRoot. The archive names them in English, as it does the fourth
	// classes.
	4218: `summoner`,       // Summoner
	4220: `summoner`,       // Baby Summoner
	4308: `spirit_handler`, // Spirit Handler

	// Mounted and costume forms of the first classes.
	// A knight on its Peco and a character in a wedding dress are jobs of
	// their own as far as the protocol is concerned.
	13: `페코페코_기사`,  // Knight2
	21: `신페코크루세이더`, // Crusader2
	22: `결혼`,       // Wedding
	26: `산타`,       // Xmas
	27: `여름`,       // Summer
	28: `한복`,       // Hanbok
	29: `옥토버패스트`,   // Oktoberfest
	30: `여름2`,      // Summer2

	// The baby jobs.
	// A baby is the same drawing as the grown job — the archive has no body
	// of its own for any of them — so each points at what it grows into.
	4023: `초보자`,      // Baby
	4024: `검사`,       // Baby Swordman
	4025: `마법사`,      // Baby Mage
	4026: `궁수`,       // Baby Archer
	4027: `성직자`,      // Baby Acolyte
	4028: `상인`,       // Baby Merchant
	4029: `도둑`,       // Baby Thief
	4030: `기사`,       // Baby Knight
	4031: `프리스트`,     // Baby Priest
	4032: `위저드`,      // Baby Wizard
	4033: `제철공`,      // Baby Blacksmith
	4034: `헌터`,       // Baby Hunter
	4035: `어세신`,      // Baby Assassin
	4036: `페코페코_기사`,  // Baby Knight2
	4037: `크루세이더`,    // Baby Crusader
	4038: `몽크`,       // Baby Monk
	4039: `세이지`,      // Baby Sage
	4040: `로그`,       // Baby Rogue
	4041: `연금술사`,     // Baby Alchemist
	4042: `바드`,       // Baby Bard
	4043: `무희`,       // Baby Dancer
	4044: `신페코크루세이더`, // Baby Crusader2
	4045: `슈퍼노비스`,    // Super Baby

	// The expanded jobs.
	4046: `태권소년`, // Taekwon
	4047: `권성`,   // Star Gladiator
	4048: `권성융합`, // Star Gladiator2
	4049: `소울링커`, // Soul Linker

	// The third classes, and their transcended and mounted forms.
	// Rebirth changes nothing about a third class on screen, so the T forms
	// share their sprite; the mounted ones do not, and have their own.
	4054: `룬나이트`,   // Rune Knight
	4055: `워록`,     // Warlock
	4056: `레인져`,    // Ranger
	4057: `아크비숍`,   // Arch Bishop
	4058: `미케닉`,    // Mechanic
	4059: `길로틴크로스`, // Guillotine Cross
	4060: `룬나이트`,   // Rune Knight T
	4061: `워록`,     // Warlock T
	4062: `레인져`,    // Ranger T
	4063: `아크비숍`,   // Arch Bishop T
	4064: `미케닉`,    // Mechanic T
	4065: `길로틴크로스`, // Guillotine Cross T
	4066: `가드`,     // Royal Guard
	4067: `소서러`,    // Sorcerer
	4068: `민스트럴`,   // Minstrel
	4069: `원더러`,    // Wanderer
	4070: `슈라`,     // Sura
	4071: `제네릭`,    // Genetic
	4072: `쉐도우체이서`, // Shadow Chaser
	4073: `가드`,     // Royal Guard T
	4074: `소서러`,    // Sorcerer T
	4075: `민스트럴`,   // Minstrel T
	4076: `원더러`,    // Wanderer T
	4077: `슈라`,     // Sura T
	4078: `제네릭`,    // Genetic T
	4079: `쉐도우체이서`, // Shadow Chaser T
	4080: `룬나이트쁘띠`, // Rune Knight2
	4081: `룬나이트쁘띠`, // Rune Knight T2
	4082: `그리폰가드`,  // Royal Guard2
	4083: `그리폰가드`,  // Royal Guard T2
	4084: `레인져늑대`,  // Ranger2
	4085: `레인져늑대`,  // Ranger T2
	4086: `마도기어`,   // Mechanic2
	4087: `마도기어`,   // Mechanic T2

	// Baby third classes.
	4096: `룬나이트`,   // Baby Rune Knight
	4097: `워록`,     // Baby Warlock
	4098: `레인져`,    // Baby Ranger
	4099: `아크비숍`,   // Baby Arch Bishop
	4100: `미케닉`,    // Baby Mechanic
	4101: `길로틴크로스`, // Baby Guillotine Cross
	4102: `가드`,     // Baby Royal Guard
	4103: `소서러`,    // Baby Sorcerer
	4104: `민스트럴`,   // Baby Minstrel
	4105: `원더러`,    // Baby Wanderer
	4106: `슈라`,     // Baby Sura
	4107: `제네릭`,    // Baby Genetic
	4108: `쉐도우체이서`, // Baby Shadow Chaser
	4109: `룬나이트쁘띠`, // Baby Rune Knight2
	4110: `그리폰가드`,  // Baby Royal Guard2
	4111: `레인져늑대`,  // Baby Ranger2
	4112: `마도기어`,   // Baby Mechanic2

	// Later expanded jobs and their babies.
	4190: `슈퍼노비스`,   // Super Novice E
	4191: `슈퍼노비스`,   // Super Baby E
	4211: `kagerou`, // Kagerou
	4212: `oboro`,   // Oboro
	4215: `리벨리온`,    // Rebellion
	4222: `닌자`,      // Baby Ninja
	4223: `kagerou`, // Baby Kagerou
	4224: `oboro`,   // Baby Oboro
	4225: `태권소년`,    // Baby Taekwon
	4226: `권성`,      // Baby Star Gladiator
	4227: `소울링커`,    // Baby Soul Linker
	4228: `건너`,      // Baby Gunslinger
	4229: `리벨리온`,    // Baby Rebellion
	4238: `권성융합`,    // Baby Star Gladiator2
	4239: `성제`,      // Star Emperor
	4240: `소울리퍼`,    // Soul Reaper
	4241: `성제`,      // Baby Star Emperor
	4242: `소울리퍼`,    // Baby Soul Reaper
	4243: `해태성제`,    // Star Emperor2
	4244: `해태성제`,    // Baby Star Emperor2

	// The fourth classes, which the archive names in English.
	// Not a rule worth trusting on its own — every one below was looked up
	// in the archive, including elemetal_master, which is spelt that way.
	4252: `dragon_knight`,         // Dragon Knight
	4253: `meister`,               // Meister
	4254: `shadow_cross`,          // Shadow Cross
	4255: `arch_mage`,             // Arch Mage
	4256: `cardinal`,              // Cardinal
	4257: `windhawk`,              // Windhawk
	4258: `imperial_guard`,        // Imperial Guard
	4259: `biolo`,                 // Biolo
	4260: `abyss_chaser`,          // Abyss Chaser
	4261: `elemetal_master`,       // Elemental Master
	4262: `inquisitor`,            // Inquisitor
	4263: `troubadour`,            // Troubadour
	4264: `trouvere`,              // Trouvere
	4278: `wolf_windhawk`,         // Windhawk2
	4279: `meister_riding`,        // Meister2
	4280: `dragon_knight_riding`,  // Dragon Knight2
	4281: `imperial_guard_riding`, // Imperial Guard2
	4302: `sky_emperor`,           // Sky Emperor
	4303: `soul_ascetic`,          // Soul Ascetic
	4304: `shinkiro`,              // Shinkiro
	4305: `shiranui`,              // Shiranui
	4306: `night_watch`,           // Night Watch
	4307: `hyper_novice`,          // Hyper Novice
	4316: `sky_emperor_riding`,    // Sky Emperor2

	// The fourth-era reissues of the third classes, same art.
	4332: `룬나이트`,   // Rune Knight 2Nd
	4333: `미케닉`,    // Mechanic 2Nd
	4334: `길로틴크로스`, // Guillotine Cross 2Nd
	4335: `워록`,     // Warlock 2Nd
	4336: `아크비숍`,   // Archbishop 2Nd
	4337: `레인져`,    // Ranger 2Nd
	4338: `가드`,     // Royal Guard 2Nd
	4339: `제네릭`,    // Genetic 2Nd
	4340: `쉐도우체이서`, // Shadow Chaser 2Nd
	4341: `소서러`,    // Sorcerer 2Nd
	4342: `슈라`,     // Sura 2Nd
	4343: `민스트럴`,   // Minstrel 2Nd
	4344: `원더러`,    // Wanderer 2Nd
}

// doramJobs are the jobs drawn from the Doram directory rather than the human
// one: the Summoner, its baby, and the Spirit Handler it becomes.
var doramJobs = map[int]bool{4218: true, 4220: true, 4308: true}

// bodyRoot is the directory a job's body sprite lives under.
func bodyRoot(job int) string {
	if doramJobs[job] {
		return doramBodyDir
	}

	return bodyDir
}

// weaponRoot is the directory a job's weapon art lives under. A Doram's
// weapons sit beside its body, in the Doram tree rather than the human one.
func weaponRoot(job int) string {
	if doramJobs[job] {
		return doramSpriteRoot
	}

	return spriteRoot
}

// jobWeaponNames is the folder a job's weapon art lives in, where that is not
// the folder its body lives in.
//
// The two usually agree, and where they do this says nothing. Where they do
// not, the archive is the reason:
//
//   - Rebirth draws no weapons of its own. There is no 챔피온 folder at all —
//     a Champion holds a Monk's knuckle, a Lord Knight a Knight's sword — so
//     every transcended job points back at the class it came from. A Champion
//     with nothing in its hand was what said so.
//   - A Royal Guard's body is 가드 and its weapons are 로얄가드, which is
//     simply two different names for the same job.
//   - A mount is a different body and a different set of weapons both, and
//     the two are not named alike: 룬나이트쁘띠 rides, 페코페코_룬나이트 swings.
//
// A job with no entry uses its body's folder, and one whose folder holds
// nothing draws no weapon — which is right for the costume jobs, where there
// is no weapon art to draw.
var jobWeaponNames = map[int]string{
	// The transcended jobs, back to the class they came from.
	4008: `기사`, 4009: `프리스트`, 4010: `위저드`, 4011: `제철공`,
	4012: `헌터`, 4013: `어세신`, 4014: `페코페코_기사`, 4015: `크루세이더`,
	4016: `몽크`, 4017: `세이지`, 4018: `로그`, 4019: `연금술사`,
	4020: `바드`, 4021: `무희`, 4022: `신페코크루세이더`,

	// The Rogue line past the second class keeps drawing from the Rogue: the
	// Stalker folder holds no weapon at all and the Shadow Chaser one holds a
	// single item's art, the rest shields.
	4072: `로그`, 4079: `로그`, 4108: `로그`, 4340: `로그`,

	// Royal Guard, whose body and weapons are named differently.
	4066: `로얄가드`, 4073: `로얄가드`, 4102: `로얄가드`, 4338: `로얄가드`,

	// The mounted third classes.
	4080: `페코페코_룬나이트`, 4081: `페코페코_룬나이트`, 4109: `페코페코_룬나이트`,
	4082: `신페코로얄가드`, 4083: `신페코로얄가드`, 4110: `신페코로얄가드`,

	// Later expanded jobs whose weapons sit under another name.
	4048: `권성`, 4238: `권성`,
	4215: `rebellion`, 4229: `rebellion`,
	4243: `성제`, 4244: `성제`,

	// The mounted fourth classes.
	4278: `windhawk`, 4279: `meister_madogear`,
	4280: `dragon_knight_chicken`, 4281: `imperial_guard_chicken`,
	4316: `sky_emperor`,
}

// JobWeaponName is the folder a job's weapon art lives in.
func JobWeaponName(job int) string {
	if name, ok := jobWeaponNames[job]; ok {
		return name
	}

	name, _ := JobSpriteName(job)

	return name
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
		base := fmt.Sprintf(`%s%s\%s_%s`, bodyRoot(s.Job), sex, job, sex)
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

	// The folder name is also the start of the file name inside it, so the
	// weapon's job stands in for the body's throughout.
	job := JobWeaponName(s.Job)
	sex := s.sexSuffix()
	dir := fmt.Sprintf(`%s%s\`, weaponRoot(s.Job), job)

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
