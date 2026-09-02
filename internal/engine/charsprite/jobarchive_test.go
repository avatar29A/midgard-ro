package charsprite

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Faultbox/midgard-ro/pkg/encoding"
	"github.com/Faultbox/midgard-ro/pkg/grf"
)

// TestEveryJobHasABodyInTheArchive walks the whole job table and asks the
// archive whether the name it gives actually names a sprite.
//
// The table is written by hand — the client keeps job-to-body in its own
// binary, and nothing in the archive pairs the two — so this is what stands in
// for a generator: a name that is misspelled, or a job pointed at art that is not
// there, is a character drawn as a Novice, which is exactly the fault this
// table was filled in to fix.
//
// It needs the client's GRFs, which the repository does not carry, so it runs
// only when MIDGARD_GRF names the directory holding them:
//
//	MIDGARD_GRF=/path/to/data go test ./internal/engine/charsprite/
func TestEveryJobHasABodyInTheArchive(t *testing.T) {
	dir := os.Getenv("MIDGARD_GRF")
	if dir == "" {
		t.Skip("set MIDGARD_GRF to the directory holding data.grf to run this")
	}

	archives, closeAll := openArchives(t, dir)
	defer closeAll()

	has := func(path string) bool { return hasFile(archives, path) }

	checked := 0
	for job, name := range jobSpriteNames {
		male, _ := (Spec{Job: job}).BodyPaths()
		female, _ := (Spec{Job: job, Female: true}).BodyPaths()

		// One sex is enough: several jobs are open to only one, and Clown,
		// Gypsy, Minstrel, Wanderer, Troubadour and Trouvere are each half of
		// a pair the game splits by job id rather than by sex.
		if !has(male) && !has(female) {
			t.Errorf("job %d names %q, which the archive does not have: %s",
				job, name, strings.TrimPrefix(male, `data\sprite\`))
		}

		checked++
	}

	t.Logf("%d jobs checked against the archive", checked)
}

// TestEveryJobCanHoldAWeapon: a job's weapons are not always filed where its
// body is. Rebirth draws none of its own — there is no 챔피온 folder at all —
// so a Champion holds a Monk's knuckle, and getting that wrong is a character
// swinging at things with an empty hand.
//
// Checked the same way and under the same MIDGARD_GRF: the class sprite of one
// weapon it can hold has to be a file. The costume jobs are left out, since
// there is no weapon art for a wedding dress to hold and nothing to find.
func TestEveryJobCanHoldAWeapon(t *testing.T) {
	dir := os.Getenv("MIDGARD_GRF")
	if dir == "" {
		t.Skip("set MIDGARD_GRF to the directory holding data.grf to run this")
	}

	archives, closeAll := openArchives(t, dir)
	defer closeAll()

	// Jobs that hold nothing, and are right not to. The costumes have no
	// weapon art at all — there is nothing for a wedding dress to carry. The
	// Taekwon line fights with its hands. And a character on a mount or in a
	// machine has its weapon drawn into the body: those folders carry shields
	// and the blade's glow, and no weapon.
	emptyHanded := map[int]bool{
		22: true, 26: true, 27: true, 28: true, 29: true, 30: true, // costumes
		4046: true, 4225: true, // Taekwon and its baby
		4084: true, 4085: true, 4111: true, // Ranger on a wolf
		4086: true, 4087: true, 4112: true, // Mechanic in a madogear
		4279: true, // Meister in a madogear
	}

	for job := range jobSpriteNames {
		if emptyHanded[job] {
			continue
		}

		found := false
		for _, weapon := range []int{1, 2, 4, 6, 8, 10, 11, 12, 13, 14, 15, 16, 17, 22, 23} {
			for _, female := range []bool{false, true} {
				spec := Spec{Job: job, Female: female, Weapon: weapon}
				for _, candidate := range spec.WeaponPathCandidates() {
					if hasFile(archives, candidate[0]) {
						found = true
					}
				}
			}
		}

		if !found {
			t.Errorf("job %d draws no weapon at all: nothing under %q",
				job, JobWeaponName(job))
		}
	}
}

// openArchives opens the client's GRFs, and a function that closes them.
func openArchives(t *testing.T, dir string) ([]*grf.Archive, func()) {
	t.Helper()

	var archives []*grf.Archive
	for _, name := range []string{"data.grf", "rdata.grf"} {
		archive, err := grf.Open(filepath.Join(dir, name))
		if err != nil {
			continue
		}

		archives = append(archives, archive)
	}

	if len(archives) == 0 {
		t.Fatalf("no GRF opened under %s", dir)
	}

	return archives, func() {
		for _, archive := range archives {
			archive.Close()
		}
	}
}

// hasFile reports whether any of the archives holds a path, which is written
// here in UTF-8 and stored there in the client's Korean encoding.
func hasFile(archives []*grf.Archive, path string) bool {
	key := string(encoding.UTF8ToEUCKR(path))
	for _, archive := range archives {
		if archive.Contains(key) {
			return true
		}
	}

	return false
}
