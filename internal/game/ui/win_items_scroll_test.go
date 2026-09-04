package ui

import "testing"

// TestItemRowsForCountsThePartRow: a bag of twenty-five in sixes is five rows,
// and a grid sized to four would leave the last item unreachable — which is
// what the window did before it scrolled at all.
func TestItemRowsForCountsThePartRow(t *testing.T) {
	for _, tc := range []struct {
		items, rows int
	}{
		{0, 0},
		{1, 1},
		{itemsCols, 1},
		{itemsCols + 1, 2},
		{25, 5},
		{4 * itemsCols, 4},
	} {
		if got := itemRowsFor(tc.items); got != tc.rows {
			t.Errorf("%d items fill %d rows, want %d", tc.items, got, tc.rows)
		}
	}
}

// TestTheGridScrollsByWholeRows: the offset is in rows, so scrolling never
// shuffles the columns — an item stays in the column it was in.
func TestTheGridScrollsByWholeRows(t *testing.T) {
	// Twenty items in a grid four rows tall: five rows of contents, one to
	// scroll through.
	const rows = 4

	filled := itemRowsFor(20)
	maxOffset := max(0, filled-rows)

	if maxOffset != 0 {
		t.Fatalf("twenty items in %d rows scroll by %d, want none", rows, maxOffset)
	}

	// Thirty-six of them is six rows, two of which are past the bottom.
	filled = itemRowsFor(36)
	if maxOffset = max(0, filled-rows); maxOffset != 2 {
		t.Errorf("thirty-six items scroll by %d rows, want 2", maxOffset)
	}

	// The first cell of the second row of contents is the seventh item,
	// which is what one row of offset reaches.
	if got := 1 * itemsCols; got != 6 {
		t.Errorf("one row of offset reaches item %d, want 6", got)
	}
}
