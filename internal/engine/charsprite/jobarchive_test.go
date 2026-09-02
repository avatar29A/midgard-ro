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

	var archives []*grf.Archive
	for _, name := range []string{"data.grf", "rdata.grf"} {
		archive, err := grf.Open(filepath.Join(dir, name))
		if err != nil {
			continue
		}

		defer archive.Close()

		archives = append(archives, archive)
	}

	if len(archives) == 0 {
		t.Fatalf("no GRF opened under %s", dir)
	}

	has := func(path string) bool {
		key := string(encoding.UTF8ToEUCKR(path))
		for _, archive := range archives {
			if archive.Contains(key) {
				return true
			}
		}

		return false
	}

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
