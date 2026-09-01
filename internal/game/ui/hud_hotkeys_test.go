package ui

import (
	"testing"

	"github.com/Faultbox/midgard-ro/internal/network/packets"
)

// TestInventoryCountAddsEveryStack: the same item can sit in more than one
// slot, and a shortcut names the item rather than a slot, so the count on a
// cell is everything carried of it.
func TestInventoryCountAddsEveryStack(t *testing.T) {
	inventory := []packets.InventoryItem{
		{Index: 2, ID: 501, Count: 12},
		{Index: 3, ID: 512, Count: 4},
		{Index: 4, ID: 501, Count: 3},
	}

	if got := inventoryCount(inventory, 501); got != 15 {
		t.Errorf("count of 501 = %d, want 15 across both stacks", got)
	}
	if got := inventoryCount(inventory, 512); got != 4 {
		t.Errorf("count of 512 = %d, want 4", got)
	}
	if got := inventoryCount(inventory, 999); got != 0 {
		t.Errorf("count of an item not carried = %d, want 0", got)
	}
	if got := inventoryCount(nil, 501); got != 0 {
		t.Errorf("count with no inventory = %d, want 0", got)
	}
}

// TestHotkeyCellAtFindsOnlyOpenRows: the bar is pulled open a row at a time,
// and a cell in a row that is shut is not on screen to be dropped on.
func TestHotkeyCellAtFindsOnlyOpenRows(t *testing.T) {
	b := &UI2DBackend{hotkeyX: 100, hotkeyY: 50, hotkeyRows: 2}

	// The middle of row 1, column 3.
	want := b.hotkeyCellRect(1, 3)
	row, col, ok := b.hotkeyCellAt(want.X+want.W/2, want.Y+want.H/2)
	if !ok || row != 1 || col != 3 {
		t.Errorf("cell at row 1 col 3 = (%d,%d,%v), want (1,3,true)", row, col, ok)
	}

	// Row 2 is shut, so nothing is there even though the geometry exists.
	shut := b.hotkeyCellRect(2, 0)
	if _, _, ok := b.hotkeyCellAt(shut.X+shut.W/2, shut.Y+shut.H/2); ok {
		t.Error("found a cell in a row that is not open")
	}

	// The gap between two cells belongs to neither.
	first := b.hotkeyCellRect(0, 0)
	if _, _, ok := b.hotkeyCellAt(first.X+first.W+2, first.Y+first.H/2); ok {
		t.Error("found a cell in the gap between two of them")
	}

	// Well away from the bar.
	if _, _, ok := b.hotkeyCellAt(0, 0); ok {
		t.Error("found a cell nowhere near the bar")
	}
}

// TestSetHotkeyItemReplaces: dropping onto an occupied cell takes it over,
// which is what the bar is for — the old shortcut is not worth a prompt.
func TestSetHotkeyItemReplaces(t *testing.T) {
	b := &UI2DBackend{hotkeyRows: hotkeyMaxRows}

	if !b.setHotkeyItem(0, 0, hotkeyCell{id: 501}) {
		t.Fatal("could not assign to the first cell")
	}
	if !b.setHotkeyItem(0, 0, hotkeyCell{id: 512}) {
		t.Fatal("could not assign over an occupied cell")
	}
	if got := b.hotkeyItems[0][0]; got != (hotkeyCell{id: 512}) {
		t.Errorf("cell holds %+v, want the item that replaced it", got)
	}

	// A skill takes an item's cell over too, and stops being an item.
	if !b.setHotkeyItem(0, 0, hotkeyCell{id: 28, skill: true}) {
		t.Fatal("could not assign a skill over an item")
	}
	if got := b.hotkeyItems[0][0]; !got.skill || got.id != 28 {
		t.Errorf("cell holds %+v, want the skill that replaced it", got)
	}
}

// TestSetHotkeyItemRejectsCellsOffTheBar: nothing outside the four rows of nine
// is a cell, and writing past them would be a silent memory scribble.
func TestSetHotkeyItemRejectsCellsOffTheBar(t *testing.T) {
	b := &UI2DBackend{hotkeyRows: hotkeyMaxRows}

	for _, tt := range []struct{ row, col int }{
		{-1, 0}, {0, -1}, {hotkeyMaxRows, 0}, {0, hotkeySlots},
	} {
		if b.setHotkeyItem(tt.row, tt.col, hotkeyCell{id: 501}) {
			t.Errorf("assigned to (%d,%d), which is not a cell", tt.row, tt.col)
		}
	}
}

// TestSetHotkeyItemToAClosedRow: the bar can be pulled shut over a row without
// emptying it, so a row that is not open still holds what it was given.
func TestSetHotkeyItemToAClosedRow(t *testing.T) {
	b := &UI2DBackend{hotkeyRows: 1}

	if !b.setHotkeyItem(3, 8, hotkeyCell{id: 501}) {
		t.Fatal("could not assign to a row that is currently shut")
	}
	if b.hotkeyItems[3][8] != (hotkeyCell{id: 501}) {
		t.Error("a shut row did not keep what it was given")
	}
}

// TestHotkeyCellKeyRoundTrip: what is written out reads back as the same cell.
func TestHotkeyCellKeyRoundTrip(t *testing.T) {
	for row := 0; row < hotkeyMaxRows; row++ {
		for col := 0; col < hotkeySlots; col++ {
			gotRow, gotCol, ok := parseHotkeyCellKey(hotkeyCellKey(row, col))
			if !ok || gotRow != row || gotCol != col {
				t.Errorf("round trip of (%d,%d) = (%d,%d,%v)", row, col, gotRow, gotCol, ok)
			}
		}
	}
}

// TestParseHotkeyCellKeyRejectsRubbish: the config is a file people can edit,
// so a key that is not a cell has to be refused rather than parsed into one.
func TestParseHotkeyCellKeyRejectsRubbish(t *testing.T) {
	for _, key := range []string{"", "0", "a,0", "0,b", ",", "0,", ",0"} {
		if _, _, ok := parseHotkeyCellKey(key); ok {
			t.Errorf("parsed %q as a cell", key)
		}
	}
}

// TestLoadHotkeyItemsSkipsWhatIsNotACell: a config written when the bar had
// more rows should lose the rows that are gone and keep the rest, not refuse
// to load.
func TestLoadHotkeyItemsSkipsWhatIsNotACell(t *testing.T) {
	b := &UI2DBackend{}

	b.loadHotkeyItems(map[string]uint32{
		"0,0":  501,
		"9,0":  512, // a row this bar does not have
		"0,99": 512, // a column it does not have
		"junk": 512,
		"1,1":  0, // an empty cell, saved by mistake
	}, map[string]uint32{
		"0,1": 28,
		"9,9": 28, // not a cell either
	})

	if b.hotkeyItems[0][0] != (hotkeyCell{id: 501}) {
		t.Error("the one good entry did not load")
	}
	if b.hotkeyItems[1][1] != (hotkeyCell{}) {
		t.Error("a zero id was loaded as a shortcut")
	}
	if got := b.hotkeyItems[0][1]; !got.skill || got.id != 28 {
		t.Errorf("the saved skill loaded as %+v", got)
	}
}

// TestLoadHotkeyItemsKeepsSkillsAndItemsApart: the two are saved in separate
// maps because an id alone cannot say which it is — item 1 and skill 1 are
// both real, and folding them together would put a potion where a skill was.
func TestLoadHotkeyItemsKeepsSkillsAndItemsApart(t *testing.T) {
	b := &UI2DBackend{}

	b.loadHotkeyItems(map[string]uint32{"0,0": 1}, map[string]uint32{"0,1": 1})

	if b.hotkeyItems[0][0].skill {
		t.Error("an item loaded as a skill")
	}
	if !b.hotkeyItems[0][1].skill {
		t.Error("a skill loaded as an item")
	}
}

// TestSavedHotkeyItemsWritesOnlyFilledCells: the bar is mostly empty, and
// writing every cell would put four rows of nine zeroes in every config file.
func TestSavedHotkeyItemsWritesOnlyFilledCells(t *testing.T) {
	b := &UI2DBackend{}
	b.setHotkeyItem(0, 0, hotkeyCell{id: 501})
	b.setHotkeyItem(3, 8, hotkeyCell{id: 512})
	b.setHotkeyItem(1, 0, hotkeyCell{id: 28, skill: true})

	saved, skills := b.savedHotkeyItems()

	if len(saved) != 2 {
		t.Errorf("saved %d item cells, want the 2 that are filled", len(saved))
	}
	if saved["0,0"] != 501 || saved["3,8"] != 512 {
		t.Errorf("saved the wrong contents: %v", saved)
	}
	if len(skills) != 1 || skills["1,0"] != 28 {
		t.Errorf("saved %v for skills, want the one that is filled", skills)
	}
}
