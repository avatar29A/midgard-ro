package ui

import (
	"github.com/Faultbox/midgard-ro/internal/engine/ui2d"
	"github.com/Faultbox/midgard-ro/internal/game/items"
	"github.com/Faultbox/midgard-ro/internal/network/packets"
)

// The menu a right click on an item opens.
//
// A double click already uses or wears a thing, which is what the original
// does too, but it is the only thing a cell offered: there was no way to ask
// what an item was without carrying it to a shop.
//
// It opens where the pointer is, as a menu does, rather than in the middle of
// the screen — it belongs to the cell that was clicked.

const (
	itemMenuW float32 = 132
	itemMenuH float32 = 20
	itemMenuG float32 = 2
	itemMenuP float32 = 3
)

// itemMenu is the menu as it stands: which item it belongs to and where it
// was opened.
type itemMenu struct {
	open bool

	// id is what the item is, and index which inventory slot it is in — the
	// server's own numbering, which is what a request has to carry.
	id    uint32
	index int

	// equip is whether choosing the first entry wears the item rather than
	// using it, which depends on the tab it was opened from.
	equip bool

	// worn is whether it is already on, so the entry can offer to take it off.
	worn bool

	// item is the copy the menu was opened on, kept whole so the information
	// window can name what is in its slots.
	item packets.InventoryItem

	x, y float32
}

// itemMenuEntry is one line of it.
type itemMenuEntry struct {
	id    string
	label string

	// info marks the entry that opens the information window rather than
	// acting on the item.
	info bool
}

// entries are what this menu offers, which depends on what the item is.
//
// Something that cannot be used or worn — a card, a crafting material — gets
// only the one entry, rather than a "Use" that does nothing when pressed.
func (m itemMenu) entries() []itemMenuEntry {
	var out []itemMenuEntry

	switch {
	case m.equip && m.worn:
		out = append(out, itemMenuEntry{id: "item_menu_act", label: "Unequip"})
	case m.equip:
		out = append(out, itemMenuEntry{id: "item_menu_act", label: "Equip"})
	default:
		if detail, known := items.DetailOf(m.id); !known || detail.Type != "Etc" {
			out = append(out, itemMenuEntry{id: "item_menu_act", label: "Use"})
		}
	}

	return append(out, itemMenuEntry{id: "item_menu_info", label: "Item Info", info: true})
}

// action is what the first entry asks for.
//
// Three different requests, not two: taking a worn item off is its own, and
// sending an equip for something already worn — which is what the label said
// and the request did not — asks the server to put a thing where it already
// is, which it answers by doing nothing at all.
func (m itemMenu) action() ItemAction {
	switch {
	case m.equip && m.worn:
		return ItemAction{Index: m.index, Unequip: true}
	case m.equip:
		return ItemAction{Index: m.index, Equip: true}
	}

	return ItemAction{Index: m.index}
}

// height is however tall its entries make it.
func (m itemMenu) height() float32 {
	rows := float32(len(m.entries()))

	return 2*itemMenuP + rows*(itemMenuH+itemMenuG) - itemMenuG
}

// openItemMenu opens it over a cell.
func (b *UI2DBackend) openItemMenu(item packets.InventoryItem, equip bool, atX, atY float32) {
	b.itemMenu = itemMenu{
		open:  true,
		id:    item.ID,
		index: item.Index,
		equip: equip,
		worn:  item.Equipped,
		item:  item,
		x:     atX,
		y:     atY,
	}
}

// drawItemMenu draws it, and acts on whatever was pressed.
//
// Drawn after the windows so it lies over the one it was opened from, and
// before the tooltip so a name does not print through it.
func (b *UI2DBackend) drawItemMenu(screenW, screenH float32) {
	if !b.itemMenu.open {
		return
	}

	entries := b.itemMenu.entries()
	height := b.itemMenu.height()

	// Kept on screen: opened near the bottom edge it would otherwise run off
	// it, and a menu whose last entry cannot be reached is worse than one
	// that opens a little above the pointer.
	x := min(b.itemMenu.x, screenW-itemMenuW-2)
	y := min(b.itemMenu.y, screenH-height-2)

	box := ui2d.Rect{X: x, Y: y, W: itemMenuW, H: height}

	r := b.ctx.Renderer()
	r.DrawRect(box.X, box.Y, box.W, box.H, escBorder)
	r.DrawRect(box.X+1, box.Y+1, box.W-2, box.H-2, escFace)

	// The menu takes the pointer while it is up, so a click meant for an
	// entry does not also land on the cell underneath it.
	b.ctx.CaptureMouse(box)

	entryY := y + itemMenuP

	for _, entry := range entries {
		row := ui2d.Rect{X: x + itemMenuP, Y: entryY, W: itemMenuW - 2*itemMenuP, H: itemMenuH}
		entryY += itemMenuH + itemMenuG

		b.drawFlatButton(row, entry.label, false)

		if !b.ctx.InvisibleButtonAt(entry.id, row.X, row.Y, row.W, row.H) {
			continue
		}

		if entry.info {
			b.ShowItemInfoOf(b.itemMenu.item)
		} else {
			b.itemAction = b.itemMenu.action()
		}

		b.itemMenu.open = false
	}

	// A press anywhere else puts it away, which is what a menu does. Read
	// after the entries, so the press that chose one is not also the press
	// that dismisses it.
	if in := b.ctx.Input(); (in.MouseLeftPressed || in.MouseRightPressed) && !box.Contains(in.MouseX, in.MouseY) {
		b.itemMenu.open = false
	}
}
