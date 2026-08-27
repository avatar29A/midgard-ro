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

	// Where the chat box was left, and whether it was pinned there. A zero
	// width or height means unset, and the box takes its default corner.
	ChatX      float32 `json:"chat_x,omitempty"`
	ChatY      float32 `json:"chat_y,omitempty"`
	ChatW      float32 `json:"chat_w,omitempty"`
	ChatH      float32 `json:"chat_h,omitempty"`
	ChatLocked bool    `json:"chat_locked,omitempty"`

	// Where the hotkey bar was left and how many of its rows were open. Zero
	// rows means unset, and the bar opens with one beside the info panel.
	HotkeyX    float32 `json:"hotkey_x,omitempty"`
	HotkeyY    float32 `json:"hotkey_y,omitempty"`
	HotkeyRows int     `json:"hotkey_rows,omitempty"`

	// The sound dialog's levels and switches.
	//
	// The switches are written even when false — no omitempty — so that "off"
	// survives a reload. Omitted, it would be indistinguishable from never
	// having been set, and every start would turn the sound back on. Sound
	// being set at all is what SoundSet records.
	SoundSet  bool    `json:"sound_set,omitempty"`
	BGMVolume float32 `json:"bgm_volume,omitempty"`
	SFXVolume float32 `json:"sfx_volume,omitempty"`
	BGMOn     bool    `json:"bgm_on"`
	SFXOn     bool    `json:"sfx_on"`
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

// UpdateUIState reads what is remembered, applies a change, and writes it
// back.
//
// Whole-struct saves are how the chat's position would get erased: the camera
// records its zoom on the way out, and a save that only knew about the zoom
// would write zeroes over everything else. Callers change the field they own
// and leave the rest alone.
func UpdateUIState(apply func(*UIState)) error {
	state := LoadUIState()
	apply(&state)

	return SaveUIState(state)
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
