package skills

import "testing"

// TestInfoTableIsPopulated: the generator is a parser for one file's shape,
// and a parser that stops matching produces an empty map rather than an error.
func TestInfoTableIsPopulated(t *testing.T) {
	if InfoCount() < 1000 {
		t.Fatalf("the table holds %d skills, want the server's whole tree", InfoCount())
	}
}

// TestInfoOfKnownSkills: a handful checked against rAthena's skill_db by hand.
// They are the ones whose figures the client is drawing anyway, and between
// them they cover every shape the file writes a field in — one figure, one per
// level, and a catalyst.
func TestInfoOfKnownSkills(t *testing.T) {
	for _, tc := range []struct {
		id      uint16
		name    string
		kind    string
		target  string
		element string

		// hitsAt is the blows landed at a level, and castAt the cast in
		// milliseconds. Zero means the field is not checked.
		level    int
		hitsAt   int
		castAt   int
		catalyst string
	}{
		{
			id: 13, name: "Soul Strike", kind: "Magic", target: "Attack",
			element: "Ghost", level: 10, hitsAt: 5, castAt: 400,
		},
		{
			// The same skill lower down: the hit count is per level, and a
			// reader that took the first entry for all of them would say one.
			id: 13, name: "Soul Strike", kind: "Magic", target: "Attack",
			element: "Ghost", level: 5, hitsAt: 3, castAt: 400,
		},
		{
			id: 16, name: "Stone Curse", kind: "Magic", target: "Attack",
			element: "Earth", level: 1, hitsAt: 1, castAt: 800,
			catalyst: "Red Gemstone",
		},
		{
			id: 12, name: "Safety Wall", kind: "Magic", target: "Ground",
			element: "Ghost", level: 10, hitsAt: 1, castAt: 320,
			catalyst: "Blue Gemstone",
		},
		{
			id: 21, name: "Thunderstorm", kind: "Magic", target: "Ground",
			element: "Wind", level: 10, hitsAt: 10, castAt: 4500,
		},
	} {
		info, ok := InfoOf(tc.id)
		if !ok {
			t.Errorf("%s (%d) is not in the table", tc.name, tc.id)

			continue
		}

		if info.Kind != tc.kind {
			t.Errorf("%s is %q, want %q", tc.name, info.Kind, tc.kind)
		}
		if info.Target != tc.target {
			t.Errorf("%s targets %q, want %q", tc.name, info.Target, tc.target)
		}
		if got := ElementAt(info.Element, tc.level); got != tc.element {
			t.Errorf("%s at Lv %d is %q, want %q", tc.name, tc.level, got, tc.element)
		}
		if got, _ := At(info.Hits, tc.level); got != tc.hitsAt {
			t.Errorf("%s at Lv %d lands %d blows, want %d", tc.name, tc.level, got, tc.hitsAt)
		}
		if got, _ := At(info.CastMs, tc.level); got != tc.castAt {
			t.Errorf("%s at Lv %d casts in %dms, want %d", tc.name, tc.level, got, tc.castAt)
		}

		if tc.catalyst == "" {
			continue
		}

		if len(info.Catalyst) != 1 {
			t.Errorf("%s spends %d things, want one", tc.name, len(info.Catalyst))

			continue
		}
		if info.Catalyst[0].Item != tc.catalyst {
			t.Errorf("%s spends %q, want %q", tc.name, info.Catalyst[0].Item, tc.catalyst)
		}
	}
}

// TestAtHoldsTheLastEntry: a field that does not vary by level is written once
// rather than ten times over, so every level past the first has to read back
// as that one figure.
func TestAtHoldsTheLastEntry(t *testing.T) {
	one := []int{400}

	for _, level := range []int{1, 5, 10, 99} {
		if got, ok := At(one, level); !ok || got != 400 {
			t.Errorf("At(one entry, %d) = %v, %v; want 400, true", level, got, ok)
		}
	}

	// And a level below the first reads the first, rather than off the front.
	if got, _ := At([]int{1, 2, 3}, 0); got != 1 {
		t.Errorf("At(.., 0) = %d, want the first entry", got)
	}
	if got, _ := At([]int{1, 2, 3}, 2); got != 2 {
		t.Errorf("At(.., 2) = %d, want the second entry", got)
	}

	if _, ok := At(nil, 1); ok {
		t.Error("an absent field read back as present")
	}
	if got := ElementAt(nil, 1); got != "" {
		t.Errorf("an absent element read back as %q", got)
	}
}

// TestElementVariesByLevel: a few skills change element as they go up, and
// reading one entry for all of them would name the wrong one nine times.
func TestElementVariesByLevel(t *testing.T) {
	// SA_ELEMENTGROUND — Earth, Wind, Water, Fire and on.
	info, ok := InfoOf(425)
	if !ok {
		t.Skip("425 is not in this server's tree")
	}

	if len(info.Element) < 2 {
		t.Fatalf("skill 425 has %d elements, want one per level", len(info.Element))
	}

	if first, second := ElementAt(info.Element, 1), ElementAt(info.Element, 2); first == second {
		t.Errorf("levels 1 and 2 are both %q", first)
	}
}
