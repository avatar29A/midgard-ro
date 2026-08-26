package states

import "testing"

func TestDialogPhaseString(t *testing.T) {
	tests := []struct {
		phase DialogPhase
		want  string
	}{
		{DialogIdle, "idle"},
		{DialogText, "text"},
		{DialogWaitingNext, "waiting-next"},
		{DialogWaitingClose, "waiting-close"},
		{DialogMenu, "menu"},
		{DialogPhase(99), "unknown(99)"},
	}

	for _, tt := range tests {
		if got := tt.phase.String(); got != tt.want {
			t.Errorf("DialogPhase(%d).String() = %q, want %q", int(tt.phase), got, tt.want)
		}
	}
}

// TestDialogZeroValueIsIdle pins that a state which has never talked to anyone
// reports idle rather than needing to be initialized first.
func TestDialogZeroValueIsIdle(t *testing.T) {
	var s InGameState

	if got := s.Dialog(); got.Phase != DialogIdle || got.NPCID != 0 {
		t.Errorf("Dialog() = %+v, want an idle zero value", got)
	}

	var nilState *InGameState
	if got := nilState.Dialog(); got.Phase != DialogIdle {
		t.Errorf("Dialog() on nil = %+v, want idle", got)
	}
}
