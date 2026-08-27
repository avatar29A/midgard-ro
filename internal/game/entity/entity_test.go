package entity

import "testing"

// TestSetVitalsMovesABar is the seam Track F will call. It is tested before
// anything calls it so the combat work connects to something already known to
// work rather than debugging two new things at once.
func TestSetVitals(t *testing.T) {
	m := NewManager()
	e := NewEntity(110000001, TypeMonster)
	e.HP, e.MaxHP = 100, 100
	m.Add(e)

	if !m.SetVitals(110000001, 40, 100) {
		t.Fatal("SetVitals did not find a unit that is in the manager")
	}
	if e.HP != 40 || e.MaxHP != 100 {
		t.Errorf("vitals = %d/%d, want 40/100", e.HP, e.MaxHP)
	}

	// A unit that has already despawned must report the miss rather than
	// silently doing nothing, so a caller can tell the two apart.
	if m.SetVitals(999, 10, 10) {
		t.Error("SetVitals reported success for a unit that is not there")
	}
}

// TestShowHPFollowsKind: whether a bar appears at all is decided per type, and
// the defaults are what NewEntity sets.
func TestShowHPFollowsKind(t *testing.T) {
	if NewEntity(1, TypeMonster).ShowHP != true {
		t.Error("a monster should show its HP bar")
	}
	if NewEntity(2, TypeNPC).ShowHP != false {
		t.Error("an NPC should not show an HP bar")
	}
	if NewEntity(3, TypePlayer).ShowHP != false {
		t.Error("another player shows a bar only once damaged")
	}
}
