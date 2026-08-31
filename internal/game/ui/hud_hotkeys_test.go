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

// TestAssignHotkeyReplaces: dropping onto an occupied cell takes it over,
// which is what the bar is for — the old shortcut is not worth a prompt.
func TestAssignHotkeyReplaces(t *testing.T) {
	b := &UI2DBackend{hotkeyRows: hotkeyMaxRows}

	if !b.AssignHotkey(0, 0, 501) {
		t.Fatal("could not assign to the first cell")
	}
	if !b.AssignHotkey(0, 0, 512) {
		t.Fatal("could not assign over an occupied cell")
	}
	if got := b.hotkeyItems[0][0]; got != 512 {
		t.Errorf("cell holds %d, want the item that replaced it", got)
	}
}

// TestAssignHotkeyRejectsCellsOffTheBar: nothing outside the four rows of nine
// is a cell, and writing past them would be a silent memory scribble.
func TestAssignHotkeyRejectsCellsOffTheBar(t *testing.T) {
	b := &UI2DBackend{hotkeyRows: hotkeyMaxRows}

	for _, tt := range []struct{ row, col int }{
		{-1, 0}, {0, -1}, {hotkeyMaxRows, 0}, {0, hotkeySlots},
	} {
		if b.AssignHotkey(tt.row, tt.col, 501) {
			t.Errorf("assigned to (%d,%d), which is not a cell", tt.row, tt.col)
		}
	}
}

// TestAssignHotkeyToAClosedRow: the bar can be pulled shut over a row without
// emptying it, so a row that is not open still holds what it was given.
func TestAssignHotkeyToAClosedRow(t *testing.T) {
	b := &UI2DBackend{hotkeyRows: 1}

	if !b.AssignHotkey(3, 8, 501) {
		t.Fatal("could not assign to a row that is currently shut")
	}
	if b.hotkeyItems[3][8] != 501 {
		t.Error("a shut row did not keep what it was given")
	}
}
