package charsprite

import (
	"strings"
	"testing"
)

// TestWeaponSuffixResolvesAClass: the login path. The character list carries
// the weapon class out of the server's char table, and a Swordman holding a
// knife arrives as class 1.
func TestWeaponSuffixResolvesAClass(t *testing.T) {
	for class, want := range map[int]string{
		1:  "_단검",      // dagger
		2:  "_검",       // one-handed sword
		10: "_롯드",      // staff
		11: "_활",       // bow
		16: "_카타르_카타르", // katar, which doubles its own name
	} {
		got, ok := WeaponSuffix(class)
		if !ok {
			t.Errorf("class %d resolved to nothing", class)
			continue
		}
		if got != want {
			t.Errorf("class %d = %q, want %q", class, got, want)
		}
	}
}

// TestWeaponSuffixResolvesAnItemID is the bug this table exists for. Every
// change of weapon after login carries the item's own id, because rAthena's
// item database sets a view id for one weapon of 2806 and sends the id for the
// rest. Read as a class it names nothing, and the character keeps whatever was
// in its hand before.
func TestWeaponSuffixResolvesAnItemID(t *testing.T) {
	// 1201 is Knife, an ordinary dagger with no art of its own.
	got, ok := WeaponSuffix(1201)
	if !ok {
		t.Fatal("item 1201 resolved to nothing; a knife equipped in game draws no weapon")
	}
	if got != "_단검" {
		t.Errorf("item 1201 = %q, want the dagger sprite", got)
	}

	// 1207 is Main Gauche, which does have art of its own — the client files
	// it under its id rather than under its class.
	if got, ok := WeaponSuffix(1207); !ok || got != "_1207" {
		t.Errorf("item 1207 = %q (%v), want _1207", got, ok)
	}
}

// TestWeaponSuffixKnowsTheDualWieldPairs: two weapons at once is a class of
// its own, six of them, and they are what a character arrives holding after a
// relog. Missing, an Assassin with two daggers stands there empty-handed.
func TestWeaponSuffixKnowsTheDualWieldPairs(t *testing.T) {
	// rAthena's W_DOUBLE_DD through W_DOUBLE_SA, which follow the end marker
	// at 24.
	for class := 25; class <= 30; class++ {
		suffix, ok := WeaponSuffix(class)
		if !ok {
			t.Errorf("dual-wield class %d resolved to nothing", class)
			continue
		}

		// Each names two weapons, so the suffix carries two underscores.
		if strings.Count(suffix, "_") != 2 {
			t.Errorf("dual-wield class %d = %q, want a pair", class, suffix)
		}
	}
}

// TestWeaponSuffixRefusesWhatItDoesNotKnow: a look with no art has to be
// refused rather than answered with something, since the caller decides
// whether to disarm the character on it.
func TestWeaponSuffixRefusesWhatItDoesNotKnow(t *testing.T) {
	for _, look := range []int{24, 999999, -1} {
		if suffix, ok := WeaponSuffix(look); ok {
			t.Errorf("look %d resolved to %q, want no answer", look, suffix)
		}
	}
}

// TestWeaponLookRangesDoNotMeet: WeaponSuffix tells a class from an item id by
// trying the class table first, which only works while the two do not overlap.
// The classes stop well below where item ids begin, and if a data update ever
// closed that gap the same number would mean two things.
func TestWeaponLookRangesDoNotMeet(t *testing.T) {
	highestClass := 0
	for class := range weaponSpriteNames {
		if class > highestClass {
			highestClass = class
		}
	}

	lowestItem := 0
	for item := range itemSpriteClass {
		if lowestItem == 0 || item < lowestItem {
			lowestItem = item
		}
	}

	if highestClass >= lowestItem {
		t.Errorf("weapon classes reach %d and item ids start at %d; a look in between is ambiguous",
			highestClass, lowestItem)
	}
}

// TestWeaponPathCandidatesNameTheDaggerFile: the suffix has to land in the
// path the archive files the sprite under, which is the job folder twice over
// — once as the directory and once as the start of the name.
func TestWeaponPathCandidatesNameTheDaggerFile(t *testing.T) {
	// A Swordman, male, holding a Knife by item id.
	spec := Spec{Job: 1, Weapon: 1201}

	candidates := spec.WeaponPathCandidates()
	if len(candidates) == 0 {
		t.Fatal("no candidate paths for a knife")
	}

	const want = `data\sprite\인간족\검사\검사_남_단검.spr`
	if candidates[0][0] != want {
		t.Errorf("first candidate is %q, want %q", candidates[0][0], want)
	}

	// The ACT sits beside the SPR under the same name.
	if act := strings.TrimSuffix(candidates[0][0], ".spr") + ".act"; candidates[0][1] != act {
		t.Errorf("ACT path is %q, want %q", candidates[0][1], act)
	}
}

// TestWeaponPathCandidatesDoNotRepeatThemselves: a weapon with art of its own
// resolves to a suffix that is already its id, and offering that path twice
// would spend a second archive lookup proving the same thing.
func TestWeaponPathCandidatesDoNotRepeatThemselves(t *testing.T) {
	spec := Spec{Job: 1, Weapon: 1207}

	seen := map[string]bool{}
	for _, candidate := range spec.WeaponPathCandidates() {
		if seen[candidate[0]] {
			t.Errorf("path %q is offered twice", candidate[0])
		}
		seen[candidate[0]] = true
	}
}

// TestWeaponPathCandidatesStopAtTheUnarmed: no weapon is not a weapon whose
// sprite failed to load, and asking for its paths would have the loader look
// for a file named after nothing.
func TestWeaponPathCandidatesStopAtTheUnarmed(t *testing.T) {
	for _, weapon := range []int{0, -1} {
		if got := (Spec{Job: 1, Weapon: weapon}).WeaponPathCandidates(); got != nil {
			t.Errorf("weapon %d asked for %v", weapon, got)
		}
	}

	if got := (Spec{Kind: KindMonster, Job: 1002, Weapon: 1201}).WeaponPathCandidates(); got != nil {
		t.Errorf("a monster asked for weapon paths: %v", got)
	}
}
