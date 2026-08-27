package config

import (
	"os"
	"path/filepath"
	"testing"
)

// chdirTemp runs the test in a scratch directory, since UIStatePath is
// relative to the working directory.
func chdirTemp(t *testing.T) {
	t.Helper()

	old, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })
}

func TestUIStateRoundTrips(t *testing.T) {
	chdirTemp(t)

	if err := SaveUIState(UIState{CameraZoom: 275.5}); err != nil {
		t.Fatalf("SaveUIState: %v", err)
	}

	got := LoadUIState()
	if got.CameraZoom != 275.5 {
		t.Errorf("CameraZoom = %v, want 275.5", got.CameraZoom)
	}
}

// TestUIStateCreatesItsDirectory: the state lives under data/, which may not
// exist on a fresh checkout.
func TestUIStateCreatesItsDirectory(t *testing.T) {
	chdirTemp(t)

	if err := SaveUIState(UIState{CameraZoom: 200}); err != nil {
		t.Fatalf("SaveUIState: %v", err)
	}
	if _, err := os.Stat(filepath.Dir(UIStatePath)); err != nil {
		t.Errorf("state directory was not created: %v", err)
	}
}

// TestLoadUIStateMissingFile: nothing remembered yet is the normal first-run
// case, not an error. Callers fall back to their own defaults.
func TestLoadUIStateMissingFile(t *testing.T) {
	chdirTemp(t)

	if got := LoadUIState(); got.CameraZoom != 0 {
		t.Errorf("CameraZoom = %v for a missing file, want the zero value", got.CameraZoom)
	}
}

// TestLoadUIStateCorruptFile: a half-written or hand-mangled file must not
// take the client down with it.
func TestLoadUIStateCorruptFile(t *testing.T) {
	chdirTemp(t)

	if err := os.MkdirAll(filepath.Dir(UIStatePath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(UIStatePath, []byte("{not json"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	if got := LoadUIState(); got.CameraZoom != 0 {
		t.Errorf("CameraZoom = %v from a corrupt file, want the zero value", got.CameraZoom)
	}
}

// TestUpdateUIStateKeepsOtherFields is the guard for the clobber: the camera
// records its zoom on the way out, and a whole-struct save that only knew
// about the zoom would write zeroes over the chat's remembered place.
func TestUpdateUIStateKeepsOtherFields(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	if err := UpdateUIState(func(s *UIState) {
		s.ChatX, s.ChatY, s.ChatW, s.ChatH = 12, 34, 500, 200
		s.ChatLocked = true
	}); err != nil {
		t.Fatalf("saving chat placement: %v", err)
	}

	if err := UpdateUIState(func(s *UIState) { s.CameraZoom = 42 }); err != nil {
		t.Fatalf("saving zoom: %v", err)
	}

	got := LoadUIState()
	if got.CameraZoom != 42 {
		t.Errorf("CameraZoom = %v, want 42", got.CameraZoom)
	}
	if got.ChatX != 12 || got.ChatY != 34 || got.ChatW != 500 || got.ChatH != 200 {
		t.Errorf("chat placement = %v/%v %vx%v, want 12/34 500x200",
			got.ChatX, got.ChatY, got.ChatW, got.ChatH)
	}
	if !got.ChatLocked {
		t.Error("ChatLocked was lost")
	}
}
