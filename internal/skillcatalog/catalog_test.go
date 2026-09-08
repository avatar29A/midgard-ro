package skillcatalog

import (
	"testing"

	"github.com/Faultbox/midgard-ro/internal/game/skills"
)

func TestCatalogueMatchesClientPagesWithoutEquatingBindingsWithPlayback(t *testing.T) {
	c := Build()
	if len(c.Jobs) != skills.Jobs() {
		t.Fatal("missing job pages")
	}
	byID := map[uint16]Skill{}
	for _, s := range c.Skills {
		if _, ok := byID[s.ID]; ok {
			t.Fatal("duplicate skill")
		}
		byID[s.ID] = s
	}
	for _, j := range c.Jobs {
		for _, id := range j.Skills {
			if _, ok := byID[id]; !ok {
				t.Fatal("missing skill", id)
			}
		}
	}
	for id, want := range map[uint16]string{13: "preview", 10: "preview", 12: "preview", 18: "preview", 11: "preview", 14: "preview", 15: "preview", 16: "partial", 17: "preview", 19: "preview", 20: "preview", 21: "preview", 9: "mapped", 1: "unmapped"} {
		if byID[id].Studio != want {
			t.Fatal(id, byID[id].Studio, want)
		}
	}
	if byID[13].Hits[9] != 5 || byID[19].Hits[9] != 10 {
		t.Fatal("lost level metadata")
	}
	found := false
	for _, j := range c.Jobs {
		if j.ID == 2 {
			found = true
			if j.Name != "Mage" || len(j.Skills) != 14 {
				t.Fatal(j)
			}
		}
	}
	if !found {
		t.Fatal("missing Mage")
	}
}
