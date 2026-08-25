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

	male   = `남`
	female = `여`
)

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
func (s Spec) BodyPaths() (sprPath, actPath string) {
	job, _ := JobSpriteName(s.Job)
	sex := s.sexSuffix()
	base := fmt.Sprintf(`%s%s\%s_%s`, bodyDir, sex, job, sex)
	return base + ".spr", base + ".act"
}

// HeadPaths returns the archive paths of the head SPR and ACT for the
// character's hair style.
func (s Spec) HeadPaths() (sprPath, actPath string) {
	sex := s.sexSuffix()
	hair := s.HairStyle
	if hair <= 0 {
		hair = 1 // style 0 isn't a file; every sex has a style 1
	}
	base := fmt.Sprintf(`%s%s\%d_%s`, headDir, sex, hair, sex)
	return base + ".spr", base + ".act"
}
