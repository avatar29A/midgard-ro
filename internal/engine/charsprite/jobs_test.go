package charsprite

import "testing"

// TestTranscendedJobsHaveTheirOwnBody: rebirth is a different sprite, not a
// different name for the same one. A job the table does not know falls back to
// the Novice, so a missing entry is a Champion standing there as a beginner —
// which is what it did.
func TestTranscendedJobsHaveTheirOwnBody(t *testing.T) {
	for job, want := range map[int]string{
		4008: `로드나이트`,  // Lord Knight
		4013: `어쌔신크로스`, // Assassin Cross
		4016: `챔피온`,    // Champion, the rebirthed Monk
		4019: `크리에이터`,  // Creator
		4021: `집시`,     // Gypsy
	} {
		name, ok := JobSpriteName(job)
		if !ok {
			t.Errorf("job %d is not in the table", job)
			continue
		}
		if name != want {
			t.Errorf("job %d = %q, want %q", job, name, want)
		}
	}
}

// TestTranscendedFirstClassesKeepTheirBody: rebirthing a first class changes
// nothing about how it looks, and the archive agrees — its `_h` twins are the
// same art, a byte or two apart. Pointing them somewhere else would change an
// appearance the game does not change.
func TestTranscendedFirstClassesKeepTheirBody(t *testing.T) {
	for high, base := range map[int]int{
		4001: 0, // Novice
		4002: 1, // Swordman
		4003: 2, // Mage
		4004: 3, // Archer
		4005: 4, // Acolyte
		4006: 5, // Merchant
		4007: 6, // Thief
	} {
		got, ok := JobSpriteName(high)
		if !ok {
			t.Errorf("job %d is not in the table", high)
			continue
		}

		want, _ := JobSpriteName(base)
		if got != want {
			t.Errorf("job %d = %q, want the same body as %d, %q", high, got, base, want)
		}
	}
}

// TestUnknownJobFallsBackToTheNovice: every account has a Novice sprite, so a
// job id newer than this table draws a character rather than nothing.
func TestUnknownJobFallsBackToTheNovice(t *testing.T) {
	name, ok := JobSpriteName(9999)
	if ok {
		t.Error("an unknown job reported itself as known")
	}

	novice, _ := JobSpriteName(FallbackJob)
	if name != novice {
		t.Errorf("an unknown job gave %q, want the Novice's %q", name, novice)
	}
}

// TestTranscendedJobsAreDistinct: the second classes rebirth into bodies of
// their own, and two sharing one would put a Champion in a Paladin's armour.
func TestTranscendedJobsAreDistinct(t *testing.T) {
	seen := map[string]int{}

	for job := 4008; job <= 4022; job++ {
		// The two mounted forms are the same rider on a Peco, and the mount
		// is what tells them apart, so they are not compared here.
		if job == 4014 || job == 4022 {
			continue
		}

		name, ok := JobSpriteName(job)
		if !ok {
			t.Errorf("job %d is not in the table", job)
			continue
		}

		if first, clash := seen[name]; clash {
			t.Errorf("jobs %d and %d both draw %q", first, job, name)
		}
		seen[name] = job
	}
}
