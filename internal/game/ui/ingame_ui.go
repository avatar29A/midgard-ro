// Package ui provides game user interface components.
package ui

import (
	"fmt"

	"github.com/AllenDang/cimgui-go/imgui"

	"github.com/Faultbox/midgard-ro/internal/game/entity"
	"github.com/Faultbox/midgard-ro/internal/game/states"
)

// InGameUI renders the in-game HUD.
type InGameUI struct {
	state *states.InGameState

	// UI Components
	statusBar    *StatusBar
	minimap      *Minimap
	chatBox      *ChatBox
	debugOverlay *DebugOverlay
	entityHPBar  *EntityHPBar

	// Settings
	ShowDebugInfo    bool
	ShowMinimap      bool
	ShowChat         bool
	ShowStatusBar    bool
	ShowEntityBars   bool
}

// NewInGameUI creates a new in-game UI.
func NewInGameUI(state *states.InGameState) *InGameUI {
	return &InGameUI{
		state:            state,
		statusBar:        NewStatusBar(),
		minimap:          NewMinimap(),
		chatBox:          NewChatBox(),
		debugOverlay:     NewDebugOverlay(),
		entityHPBar:      NewEntityHPBar(),
		ShowDebugInfo:    true, // Show debug info by default during development
		ShowMinimap:      true,
		ShowChat:         true,
		ShowStatusBar:    true,
		ShowEntityBars:   true,
	}
}

// GetStatusBar returns the status bar component.
func (ui *InGameUI) GetStatusBar() *StatusBar {
	return ui.statusBar
}

// GetMinimap returns the minimap component.
func (ui *InGameUI) GetMinimap() *Minimap {
	return ui.minimap
}

// GetChatBox returns the chat box component.
func (ui *InGameUI) GetChatBox() *ChatBox {
	return ui.chatBox
}

// GetDebugOverlay returns the debug overlay component.
func (ui *InGameUI) GetDebugOverlay() *DebugOverlay {
	return ui.debugOverlay
}

// Update updates the UI state.
func (ui *InGameUI) Update(deltaMs float64) {
	// Update debug overlay
	ui.debugOverlay.Update(deltaMs)

	// Update debug overlay with current state
	player := ui.state.GetPlayer()
	if player != nil {
		x, y, z := player.RenderPosition()
		ui.debugOverlay.PlayerX = x
		ui.debugOverlay.PlayerY = y
		ui.debugOverlay.PlayerZ = z

		tileX, tileY := ui.state.GetPlayerTilePosition()
		ui.debugOverlay.PlayerTileX = tileX
		ui.debugOverlay.PlayerTileY = tileY
		ui.debugOverlay.PlayerDirection = uint8(player.Direction)

		// Update minimap player position
		ui.minimap.SetPlayerPosition(tileX, tileY)
	}

	ui.debugOverlay.MapName = ui.state.GetMapName()

	// Update entity counts
	entityMgr := ui.state.GetEntityManager()
	if entityMgr != nil {
		ui.debugOverlay.EntityCount = entityMgr.Count()
		ui.debugOverlay.PlayerCount = entityMgr.CountByType(entity.TypePlayer)
		ui.debugOverlay.MonsterCount = entityMgr.CountByType(entity.TypeMonster)
		ui.debugOverlay.NPCCount = entityMgr.CountByType(entity.TypeNPC)
		ui.debugOverlay.ItemCount = entityMgr.CountByType(entity.TypeItem)
	}
}

// Render renders the in-game UI.
func (ui *InGameUI) Render(viewportWidth, viewportHeight float32) {
	// Render the 3D scene as background
	ui.renderSceneBackground(viewportWidth, viewportHeight)

	// Debug overlay (top-left)
	if ui.ShowDebugInfo {
		ui.debugOverlay.Render()
	}

	// Minimap (top-right)
	if ui.ShowMinimap {
		ui.minimap.Render(viewportWidth-170, 10)
	}

	// Status bar with HP/SP (top-left, below debug)
	if ui.ShowStatusBar {
		player := ui.state.GetPlayer()
		if player != nil {
			// Convert Character to Entity for status bar
			playerEntity := ui.state.GetPlayerEntity()
			if playerEntity != nil {
				ui.statusBar.SetEntity(playerEntity)
				ui.statusBar.Render(10, 200)
			}
		}
	}

	// Chat box (bottom-left)
	if ui.ShowChat {
		chatHeight := float32(200)
		chatWidth := float32(400)
		ui.chatBox.Render(10, viewportHeight-chatHeight-35, chatWidth, chatHeight)
	}

	// Simple status bar at very bottom
	ui.renderBottomStatusBar(viewportWidth, viewportHeight)

	// Error message overlay if any
	if errMsg := ui.state.GetErrorMessage(); errMsg != "" {
		ui.renderErrorOverlay(errMsg, viewportWidth, viewportHeight)
	}
}

// renderSceneBackground renders the 3D scene as the window background.
func (ui *InGameUI) renderSceneBackground(viewportWidth, viewportHeight float32) {
	if !ui.state.IsSceneReady() {
		return
	}

	texID := ui.state.GetSceneTexture()
	if texID == 0 {
		return
	}

	// Create a fullscreen window for the scene
	imgui.SetNextWindowPos(imgui.NewVec2(0, 0))
	imgui.SetNextWindowSize(imgui.NewVec2(viewportWidth, viewportHeight))

	flags := imgui.WindowFlagsNoTitleBar | imgui.WindowFlagsNoResize |
		imgui.WindowFlagsNoMove | imgui.WindowFlagsNoScrollbar |
		imgui.WindowFlagsNoScrollWithMouse | imgui.WindowFlagsNoBringToFrontOnFocus |
		imgui.WindowFlagsNoInputs

	imgui.PushStyleVarVec2(imgui.StyleVarWindowPadding, imgui.NewVec2(0, 0))
	if imgui.BeginV("##SceneBackground", nil, flags) {
		// Display the scene texture - using TextureRef for cimgui-go
		texRef := imgui.NewTextureRefTextureID(imgui.TextureID(texID))
		imgui.ImageV(*texRef,
			imgui.NewVec2(viewportWidth, viewportHeight),
			imgui.NewVec2(0, 1), // UV0 - flip vertically
			imgui.NewVec2(1, 0)) // UV1
	}
	imgui.End()
	imgui.PopStyleVar()
}

// RenderEntityBars renders HP bars above visible entities.
// screenPositions maps entity ID to screen coordinates.
func (ui *InGameUI) RenderEntityBars(entities []*entity.Entity, getScreenPos func(e *entity.Entity) (float32, float32, bool)) {
	if !ui.ShowEntityBars {
		return
	}

	for _, e := range entities {
		screenX, screenY, visible := getScreenPos(e)
		if visible {
			ui.entityHPBar.RenderForEntity(e, screenX, screenY)
		}
	}
}

func (ui *InGameUI) renderBottomStatusBar(viewportWidth, viewportHeight float32) {
	barHeight := float32(25)
	imgui.SetNextWindowPos(imgui.NewVec2(0, viewportHeight-barHeight))
	imgui.SetNextWindowSize(imgui.NewVec2(viewportWidth, barHeight))

	flags := imgui.WindowFlagsNoTitleBar | imgui.WindowFlagsNoResize |
		imgui.WindowFlagsNoMove | imgui.WindowFlagsNoScrollbar

	imgui.PushStyleVarVec2(imgui.StyleVarWindowPadding, imgui.NewVec2(10, 3))
	if imgui.BeginV("##StatusBar", nil, flags) {
		// Status message
		if statusMsg := ui.state.GetStatusMessage(); statusMsg != "" {
			imgui.Text(statusMsg)
		} else {
			imgui.Text(fmt.Sprintf("Map: %s", ui.state.GetMapName()))
		}

		// Position info on the right side
		imgui.SameLine()
		tileX, tileY := ui.state.GetPlayerTilePosition()
		posText := fmt.Sprintf("(%d, %d)", tileX, tileY)
		textWidth := imgui.CalcTextSize(posText).X
		imgui.SetCursorPosX(viewportWidth - textWidth - 20)
		imgui.Text(posText)
	}
	imgui.End()
	imgui.PopStyleVar()
}

func (ui *InGameUI) renderErrorOverlay(errMsg string, viewportWidth, viewportHeight float32) {
	// Center error message
	windowWidth := float32(300)
	windowHeight := float32(80)
	windowX := (viewportWidth - windowWidth) / 2
	windowY := (viewportHeight - windowHeight) / 2

	imgui.SetNextWindowPos(imgui.NewVec2(windowX, windowY))
	imgui.SetNextWindowSize(imgui.NewVec2(windowWidth, windowHeight))
	imgui.SetNextWindowBgAlpha(0.9)

	flags := imgui.WindowFlagsNoResize | imgui.WindowFlagsNoMove |
		imgui.WindowFlagsNoCollapse | imgui.WindowFlagsNoTitleBar

	if imgui.BeginV("##ErrorOverlay", nil, flags) {
		imgui.Spacing()
		imgui.TextColored(imgui.NewVec4(1, 0.3, 0.3, 1), "Error")
		imgui.Separator()
		imgui.TextWrapped(errMsg)
	}
	imgui.End()
}

// ToggleDebugInfo toggles the debug info panel.
func (ui *InGameUI) ToggleDebugInfo() {
	ui.ShowDebugInfo = !ui.ShowDebugInfo
}

// ToggleMinimap toggles the minimap.
func (ui *InGameUI) ToggleMinimap() {
	ui.ShowMinimap = !ui.ShowMinimap
}
