// Package ui provides game user interface components.
package ui

import (
	"fmt"

	"github.com/AllenDang/cimgui-go/imgui"

	"github.com/Faultbox/midgard-ro/internal/game/states"
)

// LoadingUI renders the loading screen UI.
type LoadingUI struct {
	state *states.LoadingState
}

// NewLoadingUI creates a new loading UI.
func NewLoadingUI(state *states.LoadingState) *LoadingUI {
	return &LoadingUI{
		state: state,
	}
}

// Render renders the loading UI.
func (ui *LoadingUI) Render(viewportWidth, viewportHeight float32) {
	// Center the loading window
	windowWidth := float32(400)
	windowHeight := float32(150)
	windowX := (viewportWidth - windowWidth) / 2
	windowY := (viewportHeight - windowHeight) / 2

	imgui.SetNextWindowPos(imgui.NewVec2(windowX, windowY))
	imgui.SetNextWindowSize(imgui.NewVec2(windowWidth, windowHeight))

	flags := imgui.WindowFlagsNoResize | imgui.WindowFlagsNoMove | imgui.WindowFlagsNoCollapse
	if imgui.BeginV("Loading", nil, flags) {
		ui.renderContent()
	}
	imgui.End()
}

func (ui *LoadingUI) renderContent() {
	imgui.Spacing()

	// Map name
	mapName := ui.state.GetMapName()
	centerText(fmt.Sprintf("Loading: %s", mapName))

	imgui.Spacing()
	imgui.Spacing()

	// Status message
	if statusMsg := ui.state.GetStatusMessage(); statusMsg != "" {
		centerText(statusMsg)
	}

	imgui.Spacing()

	// Progress bar
	progress := ui.state.GetProgress()
	imgui.ProgressBarV(progress, imgui.NewVec2(-1, 20), fmt.Sprintf("%.0f%%", progress*100))

	imgui.Spacing()

	// Error message
	if errMsg := ui.state.GetErrorMessage(); errMsg != "" {
		imgui.TextColored(imgui.NewVec4(1, 0.3, 0.3, 1), errMsg)
	}

	// Loading phase indicator
	phase := ui.state.GetLoadingPhase()
	imgui.TextDisabled(fmt.Sprintf("Phase: %s", phase))
}
