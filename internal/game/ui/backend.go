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
	// ToggleEscMenu opens the menu Escape shows, or closes it.
	ToggleEscMenu()

	// EscMenuOpen reports whether that menu is showing.
	EscMenuOpen() bool

	// TakeEscAction returns what the player picked in it and clears it.
	TakeEscAction() EscAction

	// TakeItemAction returns a double click on an inventory item.
	TakeItemAction() (ItemAction, bool)

	// TakeDropAction returns an item dragged out of the inventory window.
	TakeDropAction() (DropAction, bool)

	// TakeStatAction returns a stat the player asked to raise.
	TakeStatAction() (StatAction, bool)

	// TakeSkillAction returns a skill the player asked to raise.
	TakeSkillAction() (SkillAction, bool)

	// TakeSkillCast returns a skill the player asked to use.
	TakeSkillCast() (SkillCast, bool)

	// TakeShowEquipAction returns a click on the equipment switch.
	TakeShowEquipAction() (bool, bool)

	// TakeLevelUpAction returns a level-up button the player pressed.
	TakeLevelUpAction() (LevelUpAction, bool)

	// OpenWindow opens one of the HUD windows.
	OpenWindow(window HUDWindow)

	// PressHotkey asks for the item in a quick-panel cell to be used.
	PressHotkey(row, col int)

	// TextEntryFocused reports whether typing is going into a field, so a
	// shortcut key does not fire while a message is being written.
	TextEntryFocused() bool

	// SetSoundSettings seeds the sound dialog from what is actually playing.
	SetSoundSettings(s SoundSettings)

	// TakeSoundSettings returns the sound settings when they have changed.
	TakeSoundSettings() (SoundSettings, bool)

	// TakeChatMessage returns a line the player has entered and clears it,
	// so the game layer can send it — the interface has no client of its own.
	TakeChatMessage() (target, message string)

	// QueueChatMessage puts a line in as though it had been typed and
	// submitted, for --say. It goes to the same field TakeChatMessage drains,
	// so an unattended run exercises the real path rather than a shortcut
	// past the interface. Reports false when a line is already waiting.
	QueueChatMessage(text string) bool

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

	// WantCursor is the cursor an interface element under the pointer asked
	// for while drawing, and whether anything asked at all.
	WantCursor() (cursor.State, bool)

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

	// WorldLabels are the names drawn over the world — what the pointer is
	// on, and the target being fought — already projected.
	WorldLabels []states.HoverLabel

	// WorldEffects are the STR effect quads playing over the map, already
	// projected into the viewport.
	WorldEffects []states.EffectQuad

	// TargetMarker is the mark over the unit being fought, already projected,
	// or nil when nothing is being fought.
	TargetMarker *states.TargetMarker

	// DamageNumbers are the figures floating up from recent blows, already
	// projected.
	DamageNumbers []states.DamageNumber

	// LevelUpButtons says which corners have a level waiting to be spent.
	LevelUpButtons LevelUpButtons

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

	// The six primary stats, what equipment and buffs add to them, and the
	// points left to spend — indexed the same way the model holds them.
	PrimaryStats [packets.PrimaryStatCount]int
	PrimaryBonus [packets.PrimaryStatCount]int
	PrimaryCost  [packets.PrimaryStatCount]int
	StatusPoints int
	SkillPoints  int

	// MapMarkers is who is around, for the map window.
	MapMarkers []states.MapMarker

	// Skills is what the character can do, as the server listed it, and
	// Inventory what it is carrying.
	Skills    []packets.Skill
	Inventory []packets.InventoryItem

	// Equipment is what is worn, keyed by the place on the body it is worn
	// in, which is the question the equipment window's ten slots ask.
	Equipment map[uint32]packets.InventoryItem

	// Portrait is the character's own sprite, for the equipment window to
	// show what it is dressing, with the size of the baked frame.
	Portrait             uint32
	PortraitW, PortraitH float32

	// ShowEquipment is the server's word on whether other players may look
	// at what this character is wearing.
	ShowEquipment bool

	// The numbers derived from those six, down the right of the window.
	Atk, AtkBonus    int
	MatkMin, MatkMax int
	Def, DefBonus    int
	Mdef, MdefBonus  int
	Flee, FleeBonus  int
	Hit              int
	Critical         int
	Aspd             int

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

	// The last command typed and what became of it (debug). A command that
	// did nothing and one that was never recognized look identical on
	// screen, and want opposite fixes.
	LastCommand        string
	LastCommandOutcome string

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
