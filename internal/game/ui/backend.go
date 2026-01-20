// Package ui provides game user interface components.
package ui

import (
	"github.com/Faultbox/midgard-ro/internal/engine/ui2d"
	"github.com/Faultbox/midgard-ro/internal/network/packets"
)

// UIBackend defines the interface for UI rendering backends.
// This abstraction allows switching between different UI implementations
// (e.g., ImGui, custom ui2d) without changing game logic.
type UIBackend interface {
	// Begin starts a new UI frame.
	Begin()

	// End finishes the UI frame and presents.
	End()

	// Close releases backend resources.
	Close()

	// GetScreenSize returns the current screen dimensions.
	GetScreenSize() (width, height float32)

	// Input returns the input state for the current frame.
	// Note: This returns the ui2d InputState; ImGui backends should provide
	// a compatible adapter or translation layer.
	Input() *ui2d.InputState

	// DrawSceneTexture draws a 3D scene texture at the specified position.
	DrawSceneTexture(x, y, w, h float32, textureID uint32)

	// RenderLoginUI renders the login screen.
	RenderLoginUI(state LoginUIState, width, height float32)

	// RenderConnectingUI renders the connecting/loading indicator.
	RenderConnectingUI(state ConnectingUIState, width, height float32)

	// RenderCharSelectUI renders the character selection screen.
	RenderCharSelectUI(state CharSelectUIState, width, height float32)

	// RenderLoadingUI renders the map loading screen.
	RenderLoadingUI(state LoadingUIState, width, height float32)

	// RenderInGameUI renders the in-game HUD.
	RenderInGameUI(state InGameUIState, dt float64, width, height float32)

	// RenderFPSOverlay renders an FPS counter (if enabled).
	RenderFPSOverlay(fps float64, width, height float32)

	// RenderScreenshotMessage renders a screenshot notification.
	RenderScreenshotMessage(msg string, width, height float32)
}

// LoginUIState contains the data needed to render the login UI.
type LoginUIState struct {
	Username     string
	Password     string
	ErrorMessage string
	IsLoading    bool
	ServerName   string

	// Callbacks
	OnUsernameChange func(string)
	OnPasswordChange func(string)
	OnLogin          func()
}

// ConnectingUIState contains the data needed to render the connecting UI.
type ConnectingUIState struct {
	StatusMessage string
	ErrorMessage  string
}

// CharSelectUIState contains the data needed to render the character select UI.
type CharSelectUIState struct {
	Characters    []*packets.CharInfo
	SelectedIndex int
	StatusMessage string
	ErrorMessage  string
	IsLoading     bool
	IsReady       bool

	// Callbacks
	OnSelect      func(index int)
	OnSelectIndex func(index int)
}

// LoadingUIState contains the data needed to render the loading UI.
type LoadingUIState struct {
	MapName       string
	StatusMessage string
	ErrorMessage  string
	Progress      float32
	Phase         string
}

// InGameUIState contains the data needed to render the in-game HUD.
type InGameUIState struct {
	// Map info
	MapName string

	// Player position
	PlayerX, PlayerY, PlayerZ float32
	PlayerTileX, PlayerTileY  int
	PlayerDirection           uint8

	// Player stats
	PlayerHP, PlayerMaxHP int
	PlayerSP, PlayerMaxSP int
	PlayerLevel           int
	PlayerJobLevel        int

	// Entity counts
	EntityCount  int
	PlayerCount  int
	MonsterCount int
	NPCCount     int
	ItemCount    int

	// Scene info
	SceneReady    bool
	SceneTexture  uint32
	StatusMessage string
	ErrorMessage  string

	// UI visibility settings
	ShowDebugInfo  bool
	ShowMinimap    bool
	ShowChat       bool
	ShowStatusBar  bool
	ShowEntityBars bool

	// FPS
	FPS float64
}

// GetCharName safely gets a character name from CharInfo.
func GetCharName(char *packets.CharInfo) string {
	if char == nil {
		return ""
	}
	return char.GetName()
}

// GetCharMapName safely gets a character's map name from CharInfo.
func GetCharMapName(char *packets.CharInfo) string {
	if char == nil {
		return ""
	}
	return char.GetMapName()
}
