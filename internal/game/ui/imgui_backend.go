// Package ui provides game user interface components.
package ui

import (
	"fmt"

	"github.com/AllenDang/cimgui-go/imgui"

	"github.com/Faultbox/midgard-ro/internal/engine/ui2d"
	"github.com/Faultbox/midgard-ro/internal/network/packets"
)

// ImGuiBackend implements UIBackend using Dear ImGui.
// This provides backward compatibility with the existing ImGui-based UI.
type ImGuiBackend struct {
	// Input state adapter (converts ImGui input to ui2d format)
	input *ui2d.InputState

	// Cached UI component instances
	loginUI      *ImGuiLoginUI
	connectingUI *ImGuiConnectingUI
	charSelectUI *ImGuiCharSelectUI
	loadingUI    *ImGuiLoadingUI
	inGameUI     *ImGuiInGameUI
}

// NewImGuiBackend creates a new ImGui UI backend.
func NewImGuiBackend() *ImGuiBackend {
	return &ImGuiBackend{
		input: &ui2d.InputState{},
	}
}

// Begin starts a new UI frame.
func (b *ImGuiBackend) Begin() {
	// ImGui frame is handled by the backend (sdlbackend)
	// Update input state from ImGui
	b.updateInputFromImGui()
}

// End finishes the UI frame.
func (b *ImGuiBackend) End() {
	// ImGui rendering is handled by the backend
}

// Close releases backend resources.
func (b *ImGuiBackend) Close() {
	// Nothing to clean up for ImGui backend
}

// GetScreenSize returns the current screen dimensions.
func (b *ImGuiBackend) GetScreenSize() (width, height float32) {
	viewport := imgui.MainViewport()
	size := viewport.WorkSize()
	return size.X, size.Y
}

// Input returns the input state.
func (b *ImGuiBackend) Input() *ui2d.InputState {
	return b.input
}

// DrawSceneTexture draws a 3D scene texture.
func (b *ImGuiBackend) DrawSceneTexture(x, y, w, h float32, textureID uint32) {
	if textureID == 0 {
		return
	}

	imgui.SetNextWindowPos(imgui.NewVec2(x, y))
	imgui.SetNextWindowSize(imgui.NewVec2(w, h))

	flags := imgui.WindowFlagsNoTitleBar | imgui.WindowFlagsNoResize |
		imgui.WindowFlagsNoMove | imgui.WindowFlagsNoScrollbar |
		imgui.WindowFlagsNoScrollWithMouse | imgui.WindowFlagsNoBringToFrontOnFocus |
		imgui.WindowFlagsNoInputs

	imgui.PushStyleVarVec2(imgui.StyleVarWindowPadding, imgui.NewVec2(0, 0))
	if imgui.BeginV("##SceneBackground", nil, flags) {
		texRef := imgui.NewTextureRefTextureID(imgui.TextureID(textureID))
		imgui.ImageV(*texRef,
			imgui.NewVec2(w, h),
			imgui.NewVec2(0, 1),
			imgui.NewVec2(1, 0))
	}
	imgui.End()
	imgui.PopStyleVar()
}

// RenderLoginUI renders the login screen.
func (b *ImGuiBackend) RenderLoginUI(state LoginUIState, width, height float32) {
	if b.loginUI == nil {
		b.loginUI = NewImGuiLoginUI()
	}
	b.loginUI.Render(state, width, height)
}

// RenderConnectingUI renders the connecting screen.
func (b *ImGuiBackend) RenderConnectingUI(state ConnectingUIState, width, height float32) {
	if b.connectingUI == nil {
		b.connectingUI = NewImGuiConnectingUI()
	}
	b.connectingUI.Render(state, width, height)
}

// RenderCharSelectUI renders the character selection screen.
func (b *ImGuiBackend) RenderCharSelectUI(state CharSelectUIState, width, height float32) {
	if b.charSelectUI == nil {
		b.charSelectUI = NewImGuiCharSelectUI()
	}
	b.charSelectUI.Render(state, width, height)
}

// RenderLoadingUI renders the loading screen.
func (b *ImGuiBackend) RenderLoadingUI(state LoadingUIState, width, height float32) {
	if b.loadingUI == nil {
		b.loadingUI = NewImGuiLoadingUI()
	}
	b.loadingUI.Render(state, width, height)
}

// RenderInGameUI renders the in-game HUD.
func (b *ImGuiBackend) RenderInGameUI(state InGameUIState, dt float64, width, height float32) {
	if b.inGameUI == nil {
		b.inGameUI = NewImGuiInGameUI()
	}
	b.inGameUI.Render(state, dt, width, height)
}

// RenderFPSOverlay renders an FPS counter.
func (b *ImGuiBackend) RenderFPSOverlay(fps float64, width, height float32) {
	imgui.SetNextWindowPos(imgui.NewVec2(width-100, 5))
	imgui.SetNextWindowBgAlpha(0.5)
	flags := imgui.WindowFlagsNoTitleBar | imgui.WindowFlagsNoResize |
		imgui.WindowFlagsNoMove | imgui.WindowFlagsNoInputs |
		imgui.WindowFlagsAlwaysAutoResize
	if imgui.BeginV("##FPS", nil, flags) {
		imgui.Text(fmt.Sprintf("FPS: %.0f", fps))
	}
	imgui.End()
}

// RenderScreenshotMessage renders a screenshot notification.
func (b *ImGuiBackend) RenderScreenshotMessage(msg string, width, height float32) {
	msgWidth := float32(300)
	imgui.SetNextWindowPos(imgui.NewVec2((width-msgWidth)/2, height-60))
	imgui.SetNextWindowSize(imgui.NewVec2(msgWidth, 0))
	imgui.SetNextWindowBgAlpha(0.8)
	flags := imgui.WindowFlagsNoTitleBar | imgui.WindowFlagsNoResize |
		imgui.WindowFlagsNoMove | imgui.WindowFlagsNoInputs |
		imgui.WindowFlagsAlwaysAutoResize
	if imgui.BeginV("##Screenshot", nil, flags) {
		imgui.TextColored(imgui.NewVec4(0.2, 1.0, 0.2, 1.0), msg)
	}
	imgui.End()
}

// updateInputFromImGui updates the ui2d InputState from ImGui.
func (b *ImGuiBackend) updateInputFromImGui() {
	io := imgui.CurrentIO()
	mousePos := imgui.MousePos()

	b.input.MouseX = mousePos.X
	b.input.MouseY = mousePos.Y
	b.input.MouseLeftDown = imgui.IsMouseDown(imgui.MouseButtonLeft)
	b.input.MouseRightDown = imgui.IsMouseDown(imgui.MouseButtonRight)
	b.input.MouseMiddleDown = imgui.IsMouseDown(imgui.MouseButtonMiddle)
	b.input.ScrollX = io.MouseWheelH()
	b.input.ScrollY = io.MouseWheel()

	// Update edge detection
	b.input.Update()
}

// -----------------------------------------------------------------------------
// ImGui UI Component Wrappers
// -----------------------------------------------------------------------------

// ImGuiLoginUI renders the login UI using ImGui.
type ImGuiLoginUI struct {
	username string
	password string
}

// NewImGuiLoginUI creates a new ImGui login UI.
func NewImGuiLoginUI() *ImGuiLoginUI {
	return &ImGuiLoginUI{}
}

// Render renders the login UI.
func (ui *ImGuiLoginUI) Render(state LoginUIState, viewportWidth, viewportHeight float32) {
	// Sync state
	if ui.username == "" && state.Username != "" {
		ui.username = state.Username
	}
	if ui.password == "" && state.Password != "" {
		ui.password = state.Password
	}

	windowWidth := float32(350)
	windowHeight := float32(250)
	windowX := (viewportWidth - windowWidth) / 2
	windowY := (viewportHeight - windowHeight) / 2

	imgui.SetNextWindowPos(imgui.NewVec2(windowX, windowY))
	imgui.SetNextWindowSize(imgui.NewVec2(windowWidth, windowHeight))

	flags := imgui.WindowFlagsNoResize | imgui.WindowFlagsNoMove | imgui.WindowFlagsNoCollapse
	if imgui.BeginV("Login to Ragnarok Online", nil, flags) {
		imgui.Spacing()
		imguiCenterText("Welcome to Midgard")
		imgui.Spacing()
		imgui.Separator()
		imgui.Spacing()

		// Username
		imgui.Text("Username:")
		imgui.SetNextItemWidth(-1)
		if imgui.InputTextWithHint("##username", "Enter username", &ui.username, 0, nil) {
			if state.OnUsernameChange != nil {
				state.OnUsernameChange(ui.username)
			}
		}
		imgui.Spacing()

		// Password
		imgui.Text("Password:")
		imgui.SetNextItemWidth(-1)
		if imgui.InputTextWithHint("##password", "Enter password", &ui.password, imgui.InputTextFlagsPassword, nil) {
			if state.OnPasswordChange != nil {
				state.OnPasswordChange(ui.password)
			}
		}
		imgui.Spacing()
		imgui.Spacing()

		// Error message
		if state.ErrorMessage != "" {
			imgui.TextColored(imgui.NewVec4(1, 0.3, 0.3, 1), state.ErrorMessage)
			imgui.Spacing()
		}

		// Login button
		imgui.BeginDisabledV(state.IsLoading)
		if imgui.ButtonV("Login", imgui.NewVec2(-1, 30)) {
			if state.OnLogin != nil {
				state.OnLogin()
			}
		}
		imgui.EndDisabled()

		if state.IsLoading {
			imgui.Spacing()
			imguiCenterText("Connecting...")
		}

		imgui.Spacing()
		imgui.Separator()
		imgui.Spacing()
		imgui.TextDisabled("Server: " + state.ServerName)
	}
	imgui.End()
}

// ImGuiConnectingUI renders the connecting UI using ImGui.
type ImGuiConnectingUI struct{}

// NewImGuiConnectingUI creates a new ImGui connecting UI.
func NewImGuiConnectingUI() *ImGuiConnectingUI {
	return &ImGuiConnectingUI{}
}

// Render renders the connecting UI.
func (ui *ImGuiConnectingUI) Render(state ConnectingUIState, viewportWidth, viewportHeight float32) {
	windowWidth := float32(300)
	windowHeight := float32(120)
	windowX := (viewportWidth - windowWidth) / 2
	windowY := (viewportHeight - windowHeight) / 2

	imgui.SetNextWindowPos(imgui.NewVec2(windowX, windowY))
	imgui.SetNextWindowSize(imgui.NewVec2(windowWidth, windowHeight))

	flags := imgui.WindowFlagsNoResize | imgui.WindowFlagsNoMove | imgui.WindowFlagsNoCollapse
	if imgui.BeginV("Connecting", nil, flags) {
		imgui.Spacing()
		imgui.Spacing()

		if state.StatusMessage != "" {
			imguiCenterText(state.StatusMessage)
		}
		imgui.Spacing()

		if state.ErrorMessage != "" {
			imgui.TextColored(imgui.NewVec4(1, 0.3, 0.3, 1), state.ErrorMessage)
		}

		imgui.Spacing()
		imgui.Spacing()
		imguiCenterText("Please wait...")
	}
	imgui.End()
}

// ImGuiCharSelectUI renders the character selection UI using ImGui.
type ImGuiCharSelectUI struct {
	selectedIndex int
}

// NewImGuiCharSelectUI creates a new ImGui character selection UI.
func NewImGuiCharSelectUI() *ImGuiCharSelectUI {
	return &ImGuiCharSelectUI{selectedIndex: -1}
}

// Render renders the character selection UI.
func (ui *ImGuiCharSelectUI) Render(state CharSelectUIState, viewportWidth, viewportHeight float32) {
	windowWidth := float32(700)
	windowHeight := float32(500)
	windowX := (viewportWidth - windowWidth) / 2
	windowY := (viewportHeight - windowHeight) / 2

	imgui.SetNextWindowPos(imgui.NewVec2(windowX, windowY))
	imgui.SetNextWindowSize(imgui.NewVec2(windowWidth, windowHeight))

	flags := imgui.WindowFlagsNoResize | imgui.WindowFlagsNoMove | imgui.WindowFlagsNoCollapse
	if imgui.BeginV("Character Selection", nil, flags) {
		if state.StatusMessage != "" {
			imgui.Text(state.StatusMessage)
		}
		if state.ErrorMessage != "" {
			imgui.TextColored(imgui.NewVec4(1, 0.3, 0.3, 1), state.ErrorMessage)
		}

		imgui.Separator()
		imgui.Spacing()

		if !state.IsReady {
			imguiCenterText("Loading character list...")
		} else if len(state.Characters) == 0 {
			imgui.Spacing()
			imguiCenterText("No characters found.")
			imgui.Spacing()
			imguiCenterText("Create a new character on the server.")
		} else {
			ui.renderCharacterList(state.Characters)
			ui.renderActionButtons(state)
		}
	}
	imgui.End()
}

func (ui *ImGuiCharSelectUI) renderCharacterList(characters []*packets.CharInfo) {
	if imgui.BeginTable("charLayout", 2) {
		imgui.TableSetupColumnV("List", imgui.TableColumnFlagsWidthFixed, 300, 0)
		imgui.TableSetupColumnV("Details", imgui.TableColumnFlagsWidthStretch, 0, 0)

		imgui.TableNextRow()
		imgui.TableNextColumn()

		imgui.Text("Characters:")
		imgui.Spacing()

		if imgui.BeginListBoxV("##charlist", imgui.NewVec2(-1, 300)) {
			for i, char := range characters {
				label := fmt.Sprintf("%s (Lv %d %s)", char.GetName(), char.BaseLevel, imguiGetJobName(char.Class))
				isSelected := ui.selectedIndex == i
				if imgui.SelectableBoolV(label, isSelected, 0, imgui.NewVec2(0, 0)) {
					ui.selectedIndex = i
				}
			}
			imgui.EndListBox()
		}

		imgui.TableNextColumn()
		ui.renderCharacterDetails(characters)

		imgui.EndTable()
	}
}

func (ui *ImGuiCharSelectUI) renderCharacterDetails(characters []*packets.CharInfo) {
	imgui.Text("Character Info:")
	imgui.Spacing()

	if ui.selectedIndex < 0 || ui.selectedIndex >= len(characters) {
		imgui.TextDisabled("Select a character to view details")
		return
	}

	char := characters[ui.selectedIndex]

	if imgui.BeginTable("charinfo", 2) {
		imgui.TableSetupColumnV("Label", imgui.TableColumnFlagsWidthFixed, 100, 0)
		imgui.TableSetupColumnV("Value", imgui.TableColumnFlagsWidthStretch, 0, 0)

		imguiAddInfoRow("Name:", char.GetName())
		imguiAddInfoRow("Job:", imguiGetJobName(char.Class))
		imguiAddInfoRow("Base Level:", fmt.Sprintf("%d", char.BaseLevel))
		imguiAddInfoRow("Job Level:", fmt.Sprintf("%d", char.JobLevel))
		imguiAddInfoRow("HP:", fmt.Sprintf("%d / %d", char.HP, char.MaxHP))
		imguiAddInfoRow("SP:", fmt.Sprintf("%d / %d", char.SP, char.MaxSP))
		imguiAddInfoRow("Zeny:", fmt.Sprintf("%d", char.Zeny))
		imguiAddInfoRow("Map:", char.GetMapName())

		imgui.EndTable()
	}
}

func (ui *ImGuiCharSelectUI) renderActionButtons(state CharSelectUIState) {
	imgui.Spacing()
	imgui.Separator()
	imgui.Spacing()

	imgui.BeginDisabledV(ui.selectedIndex < 0 || state.IsLoading)
	if imgui.ButtonV("Enter Game", imgui.NewVec2(150, 30)) {
		if state.OnSelect != nil {
			state.OnSelect(ui.selectedIndex)
		}
	}
	imgui.EndDisabled()

	imgui.SameLine()
	imgui.BeginDisabledV(true)
	imgui.ButtonV("Create Character", imgui.NewVec2(150, 0))
	imgui.EndDisabled()

	imgui.SameLine()
	imgui.BeginDisabledV(true)
	imgui.ButtonV("Delete Character", imgui.NewVec2(150, 0))
	imgui.EndDisabled()
}

// ImGuiLoadingUI renders the loading UI using ImGui.
type ImGuiLoadingUI struct{}

// NewImGuiLoadingUI creates a new ImGui loading UI.
func NewImGuiLoadingUI() *ImGuiLoadingUI {
	return &ImGuiLoadingUI{}
}

// Render renders the loading UI.
func (ui *ImGuiLoadingUI) Render(state LoadingUIState, viewportWidth, viewportHeight float32) {
	windowWidth := float32(400)
	windowHeight := float32(150)
	windowX := (viewportWidth - windowWidth) / 2
	windowY := (viewportHeight - windowHeight) / 2

	imgui.SetNextWindowPos(imgui.NewVec2(windowX, windowY))
	imgui.SetNextWindowSize(imgui.NewVec2(windowWidth, windowHeight))

	flags := imgui.WindowFlagsNoResize | imgui.WindowFlagsNoMove | imgui.WindowFlagsNoCollapse
	if imgui.BeginV("Loading", nil, flags) {
		imgui.Spacing()

		imguiCenterText(fmt.Sprintf("Loading: %s", state.MapName))
		imgui.Spacing()
		imgui.Spacing()

		if state.StatusMessage != "" {
			imguiCenterText(state.StatusMessage)
		}
		imgui.Spacing()

		imgui.ProgressBarV(state.Progress, imgui.NewVec2(-1, 20), fmt.Sprintf("%.0f%%", state.Progress*100))
		imgui.Spacing()

		if state.ErrorMessage != "" {
			imgui.TextColored(imgui.NewVec4(1, 0.3, 0.3, 1), state.ErrorMessage)
		}

		imgui.TextDisabled(fmt.Sprintf("Phase: %s", state.Phase))
	}
	imgui.End()
}

// ImGuiInGameUI renders the in-game HUD using ImGui.
type ImGuiInGameUI struct{}

// NewImGuiInGameUI creates a new ImGui in-game UI.
func NewImGuiInGameUI() *ImGuiInGameUI {
	return &ImGuiInGameUI{}
}

// Render renders the in-game HUD.
func (ui *ImGuiInGameUI) Render(state InGameUIState, dt float64, viewportWidth, viewportHeight float32) {
	// Scene background
	if state.SceneReady && state.SceneTexture != 0 {
		imgui.SetNextWindowPos(imgui.NewVec2(0, 0))
		imgui.SetNextWindowSize(imgui.NewVec2(viewportWidth, viewportHeight))

		flags := imgui.WindowFlagsNoTitleBar | imgui.WindowFlagsNoResize |
			imgui.WindowFlagsNoMove | imgui.WindowFlagsNoScrollbar |
			imgui.WindowFlagsNoScrollWithMouse | imgui.WindowFlagsNoBringToFrontOnFocus |
			imgui.WindowFlagsNoInputs

		imgui.PushStyleVarVec2(imgui.StyleVarWindowPadding, imgui.NewVec2(0, 0))
		if imgui.BeginV("##SceneBackground", nil, flags) {
			texRef := imgui.NewTextureRefTextureID(imgui.TextureID(state.SceneTexture))
			imgui.ImageV(*texRef,
				imgui.NewVec2(viewportWidth, viewportHeight),
				imgui.NewVec2(0, 1),
				imgui.NewVec2(1, 0))
		}
		imgui.End()
		imgui.PopStyleVar()
	}

	// Debug overlay (top-left)
	if state.ShowDebugInfo {
		ui.renderDebugOverlay(state)
	}

	// Bottom status bar
	ui.renderBottomStatusBar(state, viewportWidth, viewportHeight)

	// Error overlay
	if state.ErrorMessage != "" {
		ui.renderErrorOverlay(state.ErrorMessage, viewportWidth, viewportHeight)
	}
}

func (ui *ImGuiInGameUI) renderDebugOverlay(state InGameUIState) {
	imgui.SetNextWindowPos(imgui.NewVec2(10, 10))
	imgui.SetNextWindowBgAlpha(0.7)
	flags := imgui.WindowFlagsNoTitleBar | imgui.WindowFlagsNoResize |
		imgui.WindowFlagsNoMove | imgui.WindowFlagsAlwaysAutoResize
	if imgui.BeginV("##Debug", nil, flags) {
		imgui.Text(fmt.Sprintf("Map: %s", state.MapName))
		imgui.Text(fmt.Sprintf("Tile: (%d, %d)", state.PlayerTileX, state.PlayerTileY))
		imgui.Text(fmt.Sprintf("World: (%.1f, %.1f, %.1f)", state.PlayerX, state.PlayerY, state.PlayerZ))
		imgui.Text(fmt.Sprintf("Dir: %d", state.PlayerDirection))
		imgui.Separator()
		imgui.Text(fmt.Sprintf("Entities: %d", state.EntityCount))
		imgui.Text(fmt.Sprintf("FPS: %.0f", state.FPS))
	}
	imgui.End()
}

func (ui *ImGuiInGameUI) renderBottomStatusBar(state InGameUIState, viewportWidth, viewportHeight float32) {
	barHeight := float32(25)
	imgui.SetNextWindowPos(imgui.NewVec2(0, viewportHeight-barHeight))
	imgui.SetNextWindowSize(imgui.NewVec2(viewportWidth, barHeight))

	flags := imgui.WindowFlagsNoTitleBar | imgui.WindowFlagsNoResize |
		imgui.WindowFlagsNoMove | imgui.WindowFlagsNoScrollbar

	imgui.PushStyleVarVec2(imgui.StyleVarWindowPadding, imgui.NewVec2(10, 3))
	if imgui.BeginV("##StatusBar", nil, flags) {
		if state.StatusMessage != "" {
			imgui.Text(state.StatusMessage)
		} else {
			imgui.Text(fmt.Sprintf("Map: %s", state.MapName))
		}

		imgui.SameLine()
		posText := fmt.Sprintf("(%d, %d)", state.PlayerTileX, state.PlayerTileY)
		textWidth := imgui.CalcTextSize(posText).X
		imgui.SetCursorPosX(viewportWidth - textWidth - 20)
		imgui.Text(posText)
	}
	imgui.End()
	imgui.PopStyleVar()
}

func (ui *ImGuiInGameUI) renderErrorOverlay(errMsg string, viewportWidth, viewportHeight float32) {
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

// -----------------------------------------------------------------------------
// Helper functions for ImGui backend
// -----------------------------------------------------------------------------

// imguiCenterText renders centered text (ImGui helper).
func imguiCenterText(text string) {
	textSize := imgui.CalcTextSize(text)
	windowWidth := imgui.ContentRegionAvail().X
	cursorX := (windowWidth - textSize.X) / 2
	if cursorX > 0 {
		imgui.SetCursorPosX(imgui.CursorPosX() + cursorX)
	}
	imgui.Text(text)
}

// imguiAddInfoRow adds a row to a two-column table.
func imguiAddInfoRow(label, value string) {
	imgui.TableNextRow()
	imgui.TableNextColumn()
	imgui.Text(label)
	imgui.TableNextColumn()
	imgui.Text(value)
}

// imguiGetJobName returns the job class name from the job ID.
func imguiGetJobName(jobID uint16) string {
	jobs := map[uint16]string{
		0:    "Novice",
		1:    "Swordman",
		2:    "Mage",
		3:    "Archer",
		4:    "Acolyte",
		5:    "Merchant",
		6:    "Thief",
		7:    "Knight",
		8:    "Priest",
		9:    "Wizard",
		10:   "Blacksmith",
		11:   "Hunter",
		12:   "Assassin",
		4008: "Lord Knight",
		4009: "High Priest",
		4010: "High Wizard",
		4054: "Rune Knight",
		4055: "Warlock",
		4057: "Arch Bishop",
	}
	if name, ok := jobs[jobID]; ok {
		return name
	}
	return fmt.Sprintf("Job %d", jobID)
}
