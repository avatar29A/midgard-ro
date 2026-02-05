// Package ui provides game user interface components.
package ui

import (
	"github.com/AllenDang/cimgui-go/imgui"

	"github.com/Faultbox/midgard-ro/internal/game/states"
)

// LoginUI renders the login screen UI.
type LoginUI struct {
	state *states.LoginState

	// Input buffers
	username string
	password string

	// Remember credentials
	rememberMe bool
}

// NewLoginUI creates a new login UI.
func NewLoginUI(state *states.LoginState) *LoginUI {
	return &LoginUI{
		state:    state,
		username: state.GetUsername(),
		password: state.GetPassword(),
	}
}

// Render renders the login UI.
func (ui *LoginUI) Render(viewportWidth, viewportHeight float32) {
	// Center the login window
	windowWidth := float32(350)
	windowHeight := float32(250)
	windowX := (viewportWidth - windowWidth) / 2
	windowY := (viewportHeight - windowHeight) / 2

	imgui.SetNextWindowPos(imgui.NewVec2(windowX, windowY))
	imgui.SetNextWindowSize(imgui.NewVec2(windowWidth, windowHeight))

	flags := imgui.WindowFlagsNoResize | imgui.WindowFlagsNoMove | imgui.WindowFlagsNoCollapse
	if imgui.BeginV("Login to Ragnarok Online", nil, flags) {
		ui.renderContent()
	}
	imgui.End()
}

func (ui *LoginUI) renderContent() {
	// Title
	imgui.Spacing()
	centerText("Welcome to Midgard")
	imgui.Spacing()
	imgui.Separator()
	imgui.Spacing()

	// Username input
	imgui.Text("Username:")
	imgui.SetNextItemWidth(-1)
	if imgui.InputTextWithHint("##username", "Enter username", &ui.username, 0, nil) {
		ui.state.SetUsername(ui.username)
	}

	imgui.Spacing()

	// Password input
	imgui.Text("Password:")
	imgui.SetNextItemWidth(-1)
	if imgui.InputTextWithHint("##password", "Enter password", &ui.password, imgui.InputTextFlagsPassword, nil) {
		ui.state.SetPassword(ui.password)
	}

	imgui.Spacing()

	// Remember me checkbox
	imgui.Checkbox("Remember me", &ui.rememberMe)

	imgui.Spacing()
	imgui.Spacing()

	// Error message
	if errMsg := ui.state.GetErrorMessage(); errMsg != "" {
		imgui.TextColored(imgui.NewVec4(1, 0.3, 0.3, 1), errMsg)
		imgui.Spacing()
	}

	// Login button (full width)
	imgui.BeginDisabledV(ui.state.IsLoadingState())
	if imgui.ButtonV("Login", imgui.NewVec2(-1, 30)) {
		ui.state.AttemptLogin()
	}
	imgui.EndDisabled()

	// Loading indicator
	if ui.state.IsLoadingState() {
		imgui.Spacing()
		centerText("Connecting...")
	}

	imgui.Spacing()
	imgui.Separator()
	imgui.Spacing()

	// Server info
	imgui.TextDisabled("Server: Korangar Test Server")
}

// centerText renders centered text.
func centerText(text string) {
	textSize := imgui.CalcTextSize(text)
	windowWidth := imgui.ContentRegionAvail().X
	cursorX := (windowWidth - textSize.X) / 2
	if cursorX > 0 {
		imgui.SetCursorPosX(imgui.CursorPosX() + cursorX)
	}
	imgui.Text(text)
}
