package items

import "testing"

// TestDetailsAreEmbedded: the generator is a parser for one file's shape, and
// a parser that stops matching writes an empty table rather than failing.
func TestDetailsAreEmbedded(t *testing.T) {
	if DetailCount() < 20000 {
		t.Fatalf("the table holds %d items, want the server's whole database", DetailCount())
	}
}

// TestDetailOfKnownItems: a handful checked against rAthena's item_db by hand.
// Between them they cover every shape a row is written in — a weapon with a
// job list, armour with a place to wear it, and a consumable that is only a
// weight and a price.
func TestDetailOfKnownItems(t *testing.T) {
	sword, ok := DetailOf(1101)
	if !ok {
		t.Fatal("the Sword is not in the table")
	}

	if sword.Type != "Weapon" || sword.SubType != "1hSword" {
		t.Errorf("the Sword is %q/%q, want Weapon/1hSword", sword.Type, sword.SubType)
	}
	if sword.Attack != 25 || sword.Slots != 3 || sword.Weight != 500 {
		t.Errorf("the Sword is %+v", sword)
	}
	if sword.Level != 1 || sword.MinLevel != 2 || !sword.Refineable {
		t.Errorf("the Sword's levels are %d/%d, refineable %v",
			sword.Level, sword.MinLevel, sword.Refineable)
	}
	if len(sword.Locations) != 1 || sword.Locations[0] != "Right_Hand" {
		t.Errorf("the Sword is worn at %v", sword.Locations)
	}
	if len(sword.Jobs) < 5 {
		t.Errorf("the Sword lists %d jobs, want the several it is restricted to", len(sword.Jobs))
	}
	if !sword.Worn() {
		t.Error("a sword is not worn")
	}

	potion, ok := DetailOf(501)
	if !ok {
		t.Fatal("the Red Potion is not in the table")
	}

	if potion.Type != "Healing" || potion.Weight != 70 || potion.Buy != 10 {
		t.Errorf("the Red Potion is %+v", potion)
	}
	if potion.Worn() || potion.Attack != 0 || potion.Slots != 0 {
		t.Errorf("a potion came out as something worn: %+v", potion)
	}
	if potion.Jobs != nil {
		t.Errorf("a potion is restricted to %v, want anybody", potion.Jobs)
	}
}

// TestDetailOfSomethingUnknown: an id past the tree this was generated from
// answers rather than returning a zero entry that reads as a real one.
func TestDetailOfSomethingUnknown(t *testing.T) {
	if _, ok := DetailOf(0); ok {
		t.Error("item 0 is in the table")
	}
	if _, ok := DetailOf(4294967295); ok {
		t.Error("an id nothing uses is in the table")
	}
}
