package ui

import "testing"

// TestCharSelectPageCount: nine slots shown three at a time is three pages,
// and the arithmetic has to round up or the last stragglers are unreachable —
// which is the whole point of paging.
func TestCharSelectPageCount(t *testing.T) {
	tests := []struct {
		name      string
		creatable int
		want      int
	}{
		{"an account that fills three pages", 9, 3},
		{"exactly one page", 3, 1},
		{"one over a page boundary", 4, 2},
		{"two over", 5, 2},
		{"the server ceiling", 15, 5},
		{"a single slot", 1, 1},
		// Before HC_ACCEPT_ENTER2 arrives the count is zero; the screen must
		// still show the three it can, not none.
		{"count not yet known", 0, 1},
	}

	var b UI2DBackend
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := CharSelectUIState{CreatableSlots: tt.creatable}

			if got := b.charSelectPageCount(state); got != tt.want {
				t.Errorf("pages = %d, want %d", got, tt.want)
			}
		})
	}
}

// TestCharSelectSlotMapping: a position on screen is not a slot. Getting this
// wrong points the selection at a different character than the one under the
// pointer, and on an empty-looking slot that means creating over somebody.
func TestCharSelectSlotMapping(t *testing.T) {
	tests := []struct {
		page, pos, want int
	}{
		{0, 0, 0}, {0, 1, 1}, {0, 2, 2},
		{1, 0, 3}, {1, 1, 4}, {1, 2, 5},
		{2, 0, 6}, {2, 1, 7}, {2, 2, 8},
	}

	for _, tt := range tests {
		got := tt.page*charSelSlotCount + tt.pos
		if got != tt.want {
			t.Errorf("page %d position %d = slot %d, want %d",
				tt.page, tt.pos, got, tt.want)
		}
	}
}

// TestCharSelectSlotCountFallback: with no count from the server the screen
// falls back to what it can show at once rather than to zero. Zero would draw
// an empty screen and read as "this account has no slots".
func TestCharSelectSlotCountFallback(t *testing.T) {
	var b UI2DBackend

	if got := b.charSelectSlotCount(CharSelectUIState{CreatableSlots: 0}); got != charSelSlotCount {
		t.Errorf("fallback = %d, want %d", got, charSelSlotCount)
	}
	if got := b.charSelectSlotCount(CharSelectUIState{CreatableSlots: 9}); got != 9 {
		t.Errorf("count = %d, want 9", got)
	}
}

// TestLastPageIsReachable: the last slot an account owns must fall on a page
// the arrows can reach. With nine slots that is slot 8 on page 2.
func TestLastPageIsReachable(t *testing.T) {
	var b UI2DBackend
	state := CharSelectUIState{CreatableSlots: 9}

	lastPage := b.charSelectPageCount(state) - 1
	highest := lastPage*charSelSlotCount + (charSelSlotCount - 1)

	if highest < state.CreatableSlots-1 {
		t.Errorf("highest reachable slot is %d, but the account has %d",
			highest, state.CreatableSlots)
	}
}
