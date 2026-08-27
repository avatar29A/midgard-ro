package entity

import "testing"

func TestClearKeepsThePlayer(t *testing.T) {
	m := NewManager()
	player := NewEntity(2000000, TypePlayer)
	m.SetPlayer(player)
	m.Add(NewEntity(110000001, TypeNPC))
	m.Add(NewEntity(110000002, TypeMonster))

	m.Clear()

	if m.Count() != 1 {
		t.Fatalf("%d entities after Clear, want only the player", m.Count())
	}
	if m.Get(player.ID) != player || m.Player() != player || m.PlayerID() != player.ID {
		t.Fatal("Clear lost the player")
	}
}

func TestClearAllDropsThePlayerToo(t *testing.T) {
	m := NewManager()
	m.SetPlayer(NewEntity(7, TypePlayer))
	m.ClearAll()
	if m.Count() != 0 || m.Player() != nil || m.PlayerID() != 0 {
		t.Fatal("ClearAll kept something")
	}
}
