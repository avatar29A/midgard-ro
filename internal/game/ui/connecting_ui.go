// Package ui provides game user interface components.
package ui

import (
	"github.com/AllenDang/cimgui-go/imgui"

	"github.com/Faultbox/midgard-ro/internal/game/states"
)

// ConnectingUI renders the connecting screen UI.
type ConnectingUI struct {
	state *states.ConnectingState
}

// NewConnectingUI creates a new connecting UI.
func NewConnectingUI(state *states.ConnectingState) *ConnectingUI {
	return &ConnectingUI{
		state: state,
	}
}

// Render renders the connecting UI.
func (ui *ConnectingUI) Render(viewportWidth, viewportHeight float32) {
	// Center the connecting window
	windowWidth := float32(300)
	windowHeight := float32(120)
	windowX := (viewportWidth - windowWidth) / 2
	windowY := (viewportHeight - windowHeight) / 2

	imgui.SetNextWindowPos(imgui.NewVec2(windowX, windowY))
	imgui.SetNextWindowSize(imgui.NewVec2(windowWidth, windowHeight))

	flags := imgui.WindowFlagsNoResize | imgui.WindowFlagsNoMove | imgui.WindowFlagsNoCollapse
	if imgui.BeginV("Connecting", nil, flags) {
		ui.renderContent()
	}
	imgui.End()
}

func (ui *ConnectingUI) renderContent() {
	imgui.Spacing()
	imgui.Spacing()

	// Status message
	if statusMsg := ui.state.GetStatusMessage(); statusMsg != "" {
		centerText(statusMsg)
	}

	imgui.Spacing()

	// Error message
	if errMsg := ui.state.GetErrorMessage(); errMsg != "" {
		imgui.TextColored(imgui.NewVec4(1, 0.3, 0.3, 1), errMsg)
	}

	imgui.Spacing()
	imgui.Spacing()

	// Simple progress indicator (dots animation would require frame counting)
	centerText("Please wait...")
}
