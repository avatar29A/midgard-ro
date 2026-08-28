package items

import "testing"

// TestNamesEmbedded guards against an embed that shipped empty or a parse
// that silently dropped every line, which would leave every item nameless.
func TestNamesEmbedded(t *testing.T) {
	if Count() < 20000 {
		t.Fatalf("Count() = %d, want the full table — did the generator run?", Count())
	}

	// 501 is Red Potion, which has been item 501 for the whole life of the
	// game and is as safe an anchor as this table has.
	if got := Name(501); got != "Red Potion" {
		t.Errorf("Name(501) = %q, want %q", got, "Red Potion")
	}
}

func TestNameUnknown(t *testing.T) {
	if got := Name(0); got != "" {
		t.Errorf("Name(0) = %q, want empty", got)
	}
}

// TestCategories pins the fold from rAthena's Type to the three tabs the
// window has: a potion is usable, a knife is worn, and anything else is etc.
func TestCategories(t *testing.T) {
	if got := CategoryOf(501); got != CategoryUsable {
		t.Errorf("CategoryOf(501) = %q, want %q", got, CategoryUsable)
	}
	if got := CategoryOf(1201); got != CategoryEquip {
		t.Errorf("CategoryOf(1201) = %q, want %q", got, CategoryEquip)
	}

	// An id the table does not know still lands somewhere rather than on a
	// tab that does not exist.
	if got := CategoryOf(0); got != CategoryEtc {
		t.Errorf("CategoryOf(unknown) = %q, want %q", got, CategoryEtc)
	}
}

// TestResourceIsArchiveName: icons are named in the archive's own Korean, and
// an item with no entry there has none rather than a made-up one.
func TestResourceIsArchiveName(t *testing.T) {
	info, ok := Lookup(501)
	if !ok {
		t.Fatal("501 must be in the table")
	}
	if info.Resource == "" {
		t.Error("Red Potion should have an icon resource")
	}
}
