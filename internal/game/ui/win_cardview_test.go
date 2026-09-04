package ui

import "testing"

// TestCardArtOfFindsTheDrawing: a card's icon is a blank card back shared by
// every card in the game, so the drawing has to be found by its own name —
// which is a table of its own, and the reason for one.
func TestCardArtOfFindsTheDrawing(t *testing.T) {
	art, has := CardArtOf(4005) // Santa Poring Card
	if !has {
		t.Fatal("the Santa Poring Card has no drawing")
	}
	if art == "" {
		t.Error("the drawing is named as an empty string")
	}

	// Two cards do not share a drawing, which is what would happen if this
	// read the icon table instead: every card in it is filed under the same
	// nameless card back.
	other, has := CardArtOf(4001) // Poring Card
	if !has {
		t.Fatal("the Poring Card has no drawing")
	}
	if other == art {
		t.Errorf("two cards share the drawing %q", art)
	}
}

// TestCardArtOfSaysNoForEverythingElse: the View button is offered on the
// strength of this, and a button that opens an empty window is worse than no
// button.
func TestCardArtOfSaysNoForEverythingElse(t *testing.T) {
	for _, id := range []uint32{501, 1101, 0} {
		if art, has := CardArtOf(id); has {
			t.Errorf("item %d offers the drawing %q", id, art)
		}
	}
}

// TestShowCardViewRefusesWhatItCannotDraw: opened on something with no
// drawing, the window would come up empty and have to be closed by hand.
func TestShowCardViewRefusesWhatItCannotDraw(t *testing.T) {
	b := &UI2DBackend{}

	b.ShowCardView(501)
	if b.cardViewID != 0 {
		t.Errorf("a potion opened the card window on %d", b.cardViewID)
	}

	b.ShowCardView(4005)
	if b.cardViewID != 4005 {
		t.Errorf("the card window is showing %d, want the card", b.cardViewID)
	}
}
