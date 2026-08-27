// Package ui provides game user interface components.
package ui

import (
	"github.com/Faultbox/midgard-ro/internal/engine/cursor"
	"github.com/Faultbox/midgard-ro/internal/engine/ui2d"
	"github.com/Faultbox/midgard-ro/internal/game/states"
	"github.com/Faultbox/midgard-ro/internal/network/packets"
)

// UIBackend defines the interface for UI rendering backends.
// This abstraction allows switching between different UI implementations
// (e.g., ImGui, custom ui2d) without changing game logic.
type UIBackend interface {
	// TakeChatMessage returns a line the player has entered and clears it,
	// so the game layer can send it — the interface has no client of its own.
	TakeChatMessage() (target, message string)

	// Begin starts a new UI frame.
	Begin()

	// End finishes the UI frame and presents.
	End()

	// Close releases backend resources.
	Close()

	// GetScreenSize returns the current screen dimensions.
	GetScreenSize() (width, height float32)

	// MouseCaptured reports whether the pointer is over the interface, so a
	// click on a panel is not also a click on the world behind it.
	MouseCaptured() bool

	// SetCursorState switches which of the original's cursors is drawn.
	SetCursorState(state cursor.State)

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
	OnExit           func()
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

	// ImageIndex is which of the archive's loading screens to show, 1-based;
	// zero falls back to the title backdrop.
	ImageIndex int
}

// InGameUIState contains the data needed to render the in-game HUD.
type InGameUIState struct {
	// Map info
	MapName string

	// MapCellsX/Y are the map's size in cells, which the minimap needs to put
	// the player marker in the right place.
	MapCellsX, MapCellsY int

	// ChatLines is the chat scrollback, oldest first.
	ChatLines []states.ChatLine

	// EntityBars are the HP/SP bars to draw under units, already projected
	// into viewport pixels.
	EntityBars []states.EntityBar

	// Player position
	PlayerX, PlayerY, PlayerZ float32
	PlayerTileX, PlayerTileY  int
	PlayerDirection           uint8
	PlayerHasDest             bool
	PlayerDestX, PlayerDestZ  float32
	PlayerIsMoving            bool

	// Camera (debug)
	CamX, CamY, CamZ float32
	CamDistance      float32
	CamYaw, CamPitch float32

	// Scene framebuffer + GL diagnostics (debug)
	SceneFBWidth  int32
	SceneFBHeight int32
	SceneTexID    uint32
	LastGLError   uint32 // 0 = NO_ERROR
	TerrainY      float32
	HasGAT        bool

	// Player identity and stats, as shown on the Basic Info panel.
	PlayerName            string
	PlayerClass           int
	PlayerHP, PlayerMaxHP int
	PlayerSP, PlayerMaxSP int
	PlayerLevel           int
	PlayerJobLevel        int

	PlayerBaseExp, PlayerNextBaseExp int64
	PlayerJobExp, PlayerNextJobExp   int64
	PlayerZeny                       int64
	PlayerWeight, PlayerMaxWeight    int

	// The last map load: how long, and where the time went (debug).
	MapLoadMs     float64
	MapLoadPhases string

	// The map's camera rules and water, as applied (debug).
	Indoor           bool
	CameraYawLocked  bool
	CameraZoomLocked bool
	CameraArc        bool
	WaterCells       int

	// Entity counts
	EntityCount  int
	PlayerCount  int
	MonsterCount int
	NPCCount     int
	ItemCount    int

	// Network telemetry (debug)
	PacketsSent     uint64
	PacketsReceived uint64
	BytesSent       uint64
	BytesReceived   uint64
	LastSentID      uint16
	LastSentLen     int
	LastSentAgoMs   int64
	LastRecvID      uint16
	LastRecvLen     int
	LastRecvAgoMs   int64

	// Scene info
	SceneReady    bool
	SceneTexture  uint32
	StatusMessage string
	ErrorMessage  string

	// What the NPC said, as the script wrote it — color codes and all.
	DialogMessage string

	// DialogShowNext and DialogShowClose are which button the server has
	// asked for. Never both.
	DialogShowNext  bool
	DialogShowClose bool

	// OnDialogNext and OnDialogClose are what the buttons do.
	OnDialogNext  func()
	OnDialogClose func()

	// The conversation in progress, for the debug overlay.
	DialogPhase   string
	DialogNPCID   uint32
	DialogNPCName string

	// DialogMenu is the choices on offer, and the callbacks answer them.
	// OnDialogChoose takes a one-based index, which is what the wire wants.
	DialogMenu     []string
	OnDialogChoose func(choice int)
	OnDialogCancel func()

	// ToggleBasicInfo folds the Basic Info panel to its reduced form, or back.
	// It is an event rather than a setting — set for the one frame the key was
	// pressed — because the panel owns which form it is in.
	ToggleBasicInfo bool

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
