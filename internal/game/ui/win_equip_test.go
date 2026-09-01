package ui

import (
	"testing"

	"github.com/Faultbox/midgard-ro/internal/engine/ui2d"
	"github.com/Faultbox/midgard-ro/internal/network/packets"
)

// TestEquipSlotsStayInsideTheWindow: the ovals are painted into the bitmap and
// the icons are bigger than they are, so the top and bottom rows hang over the
// body unless they are nudged back in.
func TestEquipSlotsStayInsideTheWindow(t *testing.T) {
	const x, pageY = 100, 200

	for _, place := range equipLayout {
		cell := equipSlotRect(x, pageY, place.right, place.row)

		if cell.Y < pageY {
			t.Errorf("slot %#x row %d starts %v above the body", place.slot, place.row, pageY-cell.Y)
		}
		if bottom := cell.Y + cell.H; bottom > pageY+equipPageH {
			t.Errorf("slot %#x row %d ends %v below the body", place.slot, place.row, bottom-(pageY+equipPageH))
		}
		if cell.X < x || cell.X+cell.W > x+equipW {
			t.Errorf("slot %#x is outside the window horizontally: x=%v w=%v", place.slot, cell.X, cell.W)
		}
	}
}

// TestEquipSlotsDoNotOverlap: two slots that share pixels would take each
// other's clicks, and the one drawn second would swallow the drop.
func TestEquipSlotsDoNotOverlap(t *testing.T) {
	const x, pageY = 0, 0

	cells := make([]ui2d.Rect, 0, len(equipLayout))
	for _, place := range equipLayout {
		cells = append(cells, equipSlotRect(x, pageY, place.right, place.row))
	}

	for i := range cells {
		for j := i + 1; j < len(cells); j++ {
			a, b := cells[i], cells[j]

			if a.X < b.X+b.W && b.X < a.X+a.W && a.Y < b.Y+b.H && b.Y < a.Y+a.H {
				t.Errorf("slots %#x and %#x overlap: %+v and %+v",
					equipLayout[i].slot, equipLayout[j].slot, a, b)
			}
		}
	}
}

// TestEquipLayoutCoversEverySlot: the window has ten places and the packet
// layer names ten slots. One missing means an item that is worn and shown
// nowhere, which reads as an item that vanished.
func TestEquipLayoutCoversEverySlot(t *testing.T) {
	laid := map[uint32]bool{}
	for _, place := range equipLayout {
		if laid[place.slot] {
			t.Errorf("slot %#x is laid out twice", place.slot)
		}
		laid[place.slot] = true
	}

	for _, slot := range packets.EquipSlots {
		if !laid[slot] {
			t.Errorf("slot %#x has nowhere to be drawn", slot)
		}
	}

	if len(equipLayout) != len(packets.EquipSlots) {
		t.Errorf("the window lays out %d slots for %d", len(equipLayout), len(packets.EquipSlots))
	}
}

// TestEquipSlotColumnsMatchTheBitmap: the labels painted into equipwin_bg.bmp
// decide which side each slot is on, and getting it wrong shows a hat where
// the shoes go. The left column reads head/head/R-hand/robe/accessary top to
// bottom and the right head/body/L-hand/shoes/accessary.
func TestEquipSlotColumnsMatchTheBitmap(t *testing.T) {
	want := []struct {
		slot  uint32
		right bool
		row   int
	}{
		{packets.EQP_HEAD_TOP, false, 0},
		{packets.EQP_HEAD_MID, false, 1},
		{packets.EQP_HAND_R, false, 2},
		{packets.EQP_GARMENT, false, 3},
		{packets.EQP_ACC_R, false, 4},
		{packets.EQP_HEAD_LOW, true, 0},
		{packets.EQP_ARMOR, true, 1},
		{packets.EQP_HAND_L, true, 2},
		{packets.EQP_SHOES, true, 3},
		{packets.EQP_ACC_L, true, 4},
	}

	for _, w := range want {
		found := false

		for _, place := range equipLayout {
			if place.slot != w.slot {
				continue
			}

			found = true

			if place.right != w.right || place.row != w.row {
				t.Errorf("slot %#x is at right=%v row=%d, want right=%v row=%d",
					w.slot, place.right, place.row, w.right, w.row)
			}
		}

		if !found {
			t.Errorf("slot %#x is not laid out at all", w.slot)
		}
	}
}
