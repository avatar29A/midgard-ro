package config

// UI state that outlives a session but does not belong in config.yaml.
//
// config.yaml is written by hand and carries comments; rewriting it to record
// a camera position would lose them and blur the line between what the user
// configured and what the client remembered. This is the client's own scratch
// state — small, disposable, and safe to delete.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// UIStatePath is where remembered UI state is kept, relative to the working
// directory. It sits beside the screenshots for the same reason: it is
// generated, not authored.
const UIStatePath = "data/client_state.json"

// UIState is what the client remembers between runs.
type UIState struct {
	// CameraZoom is the third-person camera distance. Zero means "unset", in
	// which case the state's default applies.
	CameraZoom float32 `json:"camera_zoom,omitempty"`
}

// LoadUIState reads remembered UI state. A missing or unreadable file is not
// an error — it just means nothing has been remembered yet, and the caller
// falls back to its defaults.
func LoadUIState() UIState {
	var state UIState

	data, err := os.ReadFile(UIStatePath)
	if err != nil {
		return state
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return UIState{}
	}
	return state
}

// SaveUIState writes remembered UI state, creating the directory if needed.
func SaveUIState(state UIState) error {
	if err := os.MkdirAll(filepath.Dir(UIStatePath), 0o755); err != nil {
		return fmt.Errorf("creating state directory: %w", err)
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding ui state: %w", err)
	}

	if err := os.WriteFile(UIStatePath, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", UIStatePath, err)
	}
	return nil
}
