// Package ui provides game user interface components.
package ui

import (
	"fmt"

	"github.com/AllenDang/cimgui-go/imgui"
	"github.com/go-gl/gl/v4.1-core/gl"

	"github.com/Faultbox/midgard-ro/internal/engine/ui2d"
)

// UI2DBackend implements UIBackend using the custom ui2d rendering system.
type UI2DBackend struct {
	ctx *ui2d.Context

	// Texture cache for GRF-based UI textures
	texCache *TextureCache

	// Login screen textures (lazy-loaded)
	loginBgTex    *TextureInfo
	logoTex       *TextureInfo
	loginTexTried bool // avoid repeated load attempts

	// Cached widget states
	loginUsername string
	loginPassword string
	charSelectIdx int
}

// NewUI2DBackend creates a new ui2d UI backend.
func NewUI2DBackend(width, height int) (*UI2DBackend, error) {
	ctx, err := ui2d.NewContext(width, height)
	if err != nil {
		return nil, fmt.Errorf("create ui2d context: %w", err)
	}

	return &UI2DBackend{
		ctx:           ctx,
		charSelectIdx: -1,
	}, nil
}

// Begin starts a new UI frame.
//
// We piggyback on cimgui-go's SDL backend for windowing and input. ImGui has
// already pumped SDL events into its IO by this point, so we read the mouse
// and key state straight off ImGui's IO rather than installing a parallel SDL
// event handler. Same trick the ImGuiBackend uses (see updateInputFromImGui).
func (b *UI2DBackend) Begin() {
	b.syncInputFromImGui()
	b.syncViewportSize()
	b.fixHiDPIViewport()
	b.ctx.Begin()
}

// fixHiDPIViewport overrides the glViewport that cimgui-go's SDL backend set
// to (0, 0, DisplaySize.x, DisplaySize.y). Those numbers are in logical
// points, but glViewport interprets them as framebuffer pixels — on a 2x
// retina display that confines our drawing to the bottom-left quadrant of the
// real framebuffer. Setting the viewport to the drawable size
// (points × DisplayFramebufferScale) makes our point-space rendering land
// 1:1 under the OS cursor.
func (b *UI2DBackend) fixHiDPIViewport() {
	io := imgui.CurrentIO()
	disp := io.DisplaySize()
	scale := io.DisplayFramebufferScale()
	if scale.X <= 0 {
		scale.X = 1
	}
	if scale.Y <= 0 {
		scale.Y = 1
	}
	gl.Viewport(0, 0, int32(disp.X*scale.X), int32(disp.Y*scale.Y))
}

// syncInputFromImGui copies the current frame's mouse + key state from ImGui's
// IO into the ui2d InputState. Must run before ctx.Begin() (which calls
// InputState.Update for edge detection).
//
// Coordinate space: empirically (via the click-corner test) cimgui-go's SDL
// backend reports mouse position in *global screen* points, not relative to
// the SDL window. Click deltas across known widget widths matched our render
// units 1:1, so the only correction needed is subtracting the SDL window's
// screen position — given to us by MainViewport().Pos(). After that the
// mouse lives in the same logical 0..DisplaySize space we render into,
// which fixHiDPIViewport stretches across the full retina framebuffer.
func (b *UI2DBackend) syncInputFromImGui() {
	in := b.ctx.Input()
	io := imgui.CurrentIO()

	winPos := imgui.MainViewport().Pos()
	mp := imgui.MousePos()
	in.MouseX = mp.X - winPos.X
	in.MouseY = mp.Y - winPos.Y
	in.MouseLeftDown = imgui.IsMouseDown(imgui.MouseButtonLeft)
	in.MouseRightDown = imgui.IsMouseDown(imgui.MouseButtonRight)
	in.MouseMiddleDown = imgui.IsMouseDown(imgui.MouseButtonMiddle)
	in.ScrollX = io.MouseWheelH()
	in.ScrollY = io.MouseWheel()

	in.KeyBackspace = imgui.IsKeyDown(imgui.KeyBackspace)
	in.KeyEnter = imgui.IsKeyDown(imgui.KeyEnter)
	in.KeyEscape = imgui.IsKeyDown(imgui.KeyEscape)
	in.KeyTab = imgui.IsKeyDown(imgui.KeyTab)
}

// syncViewportSize keeps the ui2d renderer matched to ImGui's viewport size,
// so the UI scales correctly when the SDL window is resized.
func (b *UI2DBackend) syncViewportSize() {
	size := imgui.MainViewport().Size()
	curW, curH := b.ctx.GetScreenSize()
	if int(size.X) != int(curW) || int(size.Y) != int(curH) {
		b.ctx.Resize(int(size.X), int(size.Y))
	}
}

// End finishes the UI frame.
func (b *UI2DBackend) End() {
	b.ctx.End()
}

// SetAssetLoader wires the GRF asset loader into the UI backend.
// This enables loading RO textures for window skins and login screen.
func (b *UI2DBackend) SetAssetLoader(loadFunc func(string) ([]byte, error)) {
	b.texCache = NewTextureCache(b.ctx.Renderer(), loadFunc)

	// Try to load window skin
	skin, err := LoadWindowSkin(b.texCache)
	if err == nil && skin.Frame != nil {
		b.ctx.SetDefaultWindowSkin(skin.Frame)
	}
}

// Close releases backend resources.
func (b *UI2DBackend) Close() {
	if b.texCache != nil {
		b.texCache.Close()
	}
	if b.ctx != nil {
		b.ctx.Close()
	}
}

// Resize updates the screen size.
func (b *UI2DBackend) Resize(width, height int) {
	b.ctx.Resize(width, height)
}

// GetScreenSize returns the current screen dimensions.
func (b *UI2DBackend) GetScreenSize() (width, height float32) {
	return b.ctx.GetScreenSize()
}

// Input returns the input state.
func (b *UI2DBackend) Input() *ui2d.InputState {
	return b.ctx.Input()
}

// DrawSceneTexture draws a 3D scene texture.
func (b *UI2DBackend) DrawSceneTexture(x, y, w, h float32, textureID uint32) {
	b.ctx.Renderer().DrawSceneTexture(x, y, w, h, textureID)
}

// loginTexBasePath is the GRF path for login screen textures.
const loginTexBasePath = `data\texture\유저인터페이스\login_interface\`

// loadLoginTextures lazy-loads the login background and logo.
func (b *UI2DBackend) loadLoginTextures() {
	if b.loginTexTried || b.texCache == nil {
		return
	}
	b.loginTexTried = true

	bg, err := b.texCache.Load(loginTexBasePath + `login_bg.bmp`)
	if err == nil {
		b.loginBgTex = bg
	}

	logo, err := b.texCache.Load(loginTexBasePath + `login_logo.bmp`)
	if err == nil {
		b.logoTex = logo
	}
}

// RenderLoginUI renders the login screen.
func (b *UI2DBackend) RenderLoginUI(state LoginUIState, width, height float32) {
	// Lazy-load login textures on first render
	b.loadLoginTextures()

	// Draw login background fullscreen
	if b.loginBgTex != nil {
		b.ctx.Renderer().DrawImage(b.loginBgTex.ID, 0, 0, width, height, ui2d.ColorWhite)
	}

	// Sync state to local cache
	if b.loginUsername == "" && state.Username != "" {
		b.loginUsername = state.Username
	}
	if b.loginPassword == "" && state.Password != "" {
		b.loginPassword = state.Password
	}

	// Center the login window
	windowWidth := float32(400)
	windowHeight := float32(340)
	windowX := (width - windowWidth) / 2
	windowY := (height - windowHeight) / 2

	// Draw logo centered above the login window
	if b.logoTex != nil {
		logoW := float32(b.logoTex.Width)
		logoH := float32(b.logoTex.Height)
		logoX := (width - logoW) / 2
		logoY := windowY - logoH - 16
		if logoY < 0 {
			logoY = 0
		}
		b.ctx.Renderer().DrawImage(b.logoTex.ID, logoX, logoY, logoW, logoH, ui2d.ColorWhite)
	}

	if b.ctx.BeginWindow("login", windowX, windowY, windowWidth, windowHeight, "Login to Ragnarok Online") {
		b.ctx.Spacer(12)
		b.ctx.LabelCentered("Welcome to Midgard")
		b.ctx.Spacer(12)
		b.ctx.Separator()
		b.ctx.Spacer(12)

		// Username
		b.ctx.Row(20)
		b.ctx.Label("Username:")
		b.ctx.Row(32)
		newUsername, changed, _ := b.ctx.TextInput("username", 0, b.loginUsername)
		if changed {
			b.loginUsername = newUsername
			if state.OnUsernameChange != nil {
				state.OnUsernameChange(newUsername)
			}
		}
		b.ctx.Spacer(12)

		// Password
		b.ctx.Row(20)
		b.ctx.Label("Password:")
		b.ctx.Row(32)
		newPassword, changed, submitted := b.ctx.PasswordInput("password", 0, b.loginPassword)
		if changed {
			b.loginPassword = newPassword
			if state.OnPasswordChange != nil {
				state.OnPasswordChange(newPassword)
			}
		}
		b.ctx.Spacer(16)

		// Error message
		if state.ErrorMessage != "" {
			b.ctx.LabelColored(state.ErrorMessage, ui2d.Color{R: 1, G: 0.3, B: 0.3, A: 1})
			b.ctx.Spacer(8)
		}

		// Login button - larger for easier clicking
		b.ctx.Row(40)
		if state.IsLoading {
			b.ctx.ButtonDisabled("login", 0, "Login")
		} else {
			btnClicked := b.ctx.Button("login", 0, "Login")
			if btnClicked || submitted {
				if state.OnLogin != nil {
					state.OnLogin()
				}
			}
		}

		if state.IsLoading {
			b.ctx.Spacer(8)
			b.ctx.LabelCentered("Connecting...")
		}

		b.ctx.Spacer(12)
		b.ctx.Separator()
		b.ctx.Spacer(8)
		b.ctx.LabelColored("Server: "+state.ServerName, ui2d.ColorTextDim)

		b.ctx.EndWindow()
	}
}

// RenderConnectingUI renders the connecting screen.
func (b *UI2DBackend) RenderConnectingUI(state ConnectingUIState, width, height float32) {
	windowWidth := float32(300)
	windowHeight := float32(120)
	windowX := (width - windowWidth) / 2
	windowY := (height - windowHeight) / 2

	if b.ctx.BeginWindow("connecting", windowX, windowY, windowWidth, windowHeight, "Connecting") {
		b.ctx.Spacer(16)

		if state.StatusMessage != "" {
			b.ctx.LabelCentered(state.StatusMessage)
		}
		b.ctx.Spacer(8)

		if state.ErrorMessage != "" {
			b.ctx.LabelColored(state.ErrorMessage, ui2d.Color{R: 1, G: 0.3, B: 0.3, A: 1})
		}

		b.ctx.Spacer(16)
		b.ctx.LabelCentered("Please wait...")

		b.ctx.EndWindow()
	}
}

// RenderCharSelectUI renders the character selection screen.
func (b *UI2DBackend) RenderCharSelectUI(state CharSelectUIState, width, height float32) {
	windowWidth := float32(500)
	windowHeight := float32(400)
	windowX := (width - windowWidth) / 2
	windowY := (height - windowHeight) / 2

	if b.ctx.BeginWindow("charselect", windowX, windowY, windowWidth, windowHeight, "Character Selection") {
		if state.StatusMessage != "" {
			b.ctx.Label(state.StatusMessage)
			b.ctx.Spacer(4)
		}
		if state.ErrorMessage != "" {
			b.ctx.LabelColored(state.ErrorMessage, ui2d.Color{R: 1, G: 0.3, B: 0.3, A: 1})
			b.ctx.Spacer(4)
		}

		b.ctx.Separator()
		b.ctx.Spacer(8)

		if !state.IsReady {
			b.ctx.LabelCentered("Loading character list...")
		} else if len(state.Characters) == 0 {
			b.ctx.Spacer(16)
			b.ctx.LabelCentered("No characters found.")
			b.ctx.Spacer(8)
			b.ctx.LabelCentered("Create a new character on the server.")
		} else {
			// Auto-select first character if none selected
			if b.charSelectIdx < 0 && len(state.Characters) > 0 {
				b.charSelectIdx = 0
				if state.OnSelectIndex != nil {
					state.OnSelectIndex(0)
				}
			}

			// Character list
			b.ctx.Row(20)
			b.ctx.Label("Characters:")
			b.ctx.Spacer(8)
			b.ctx.BeginListBox("charlist", 0, 150)

			for i, char := range state.Characters {
				label := fmt.Sprintf("%s (Lv %d)", char.GetName(), char.BaseLevel)
				if b.ctx.Selectable(fmt.Sprintf("char_%d", i), label, b.charSelectIdx == i) {
					b.charSelectIdx = i
					if state.OnSelectIndex != nil {
						state.OnSelectIndex(i)
					}
				}
			}

			b.ctx.EndListBox()
			b.ctx.Spacer(8)

			// Show selected character details
			if b.charSelectIdx >= 0 && b.charSelectIdx < len(state.Characters) {
				char := state.Characters[b.charSelectIdx]
				b.ctx.Row(20)
				b.ctx.Label(fmt.Sprintf("HP: %d/%d   SP: %d/%d", char.HP, char.MaxHP, char.SP, char.MaxSP))
				b.ctx.Row(20)
				b.ctx.Label(fmt.Sprintf("Map: %s", char.GetMapName()))
			}

			b.ctx.Spacer(8)
			b.ctx.Separator()
			b.ctx.Spacer(8)

			// Action buttons
			b.ctx.Row(40)
			if state.IsLoading || b.charSelectIdx < 0 {
				b.ctx.ButtonDisabled("enter", 0, "Enter Game")
			} else {
				btnClicked := b.ctx.Button("enter", 0, "Enter Game")
				if btnClicked {
					if state.OnSelect != nil {
						state.OnSelect(b.charSelectIdx)
					}
				}
			}
		}

		b.ctx.EndWindow()
	}
}

// RenderLoadingUI renders the loading screen.
func (b *UI2DBackend) RenderLoadingUI(state LoadingUIState, width, height float32) {
	windowWidth := float32(400)
	windowHeight := float32(150)
	windowX := (width - windowWidth) / 2
	windowY := (height - windowHeight) / 2

	if b.ctx.BeginWindow("loading", windowX, windowY, windowWidth, windowHeight, "Loading") {
		b.ctx.Spacer(8)

		b.ctx.LabelCentered(fmt.Sprintf("Loading: %s", state.MapName))
		b.ctx.Spacer(8)

		if state.StatusMessage != "" {
			b.ctx.LabelCentered(state.StatusMessage)
		}
		b.ctx.Spacer(8)

		// Progress bar
		b.ctx.ProgressBar(state.Progress, 0, 20, fmt.Sprintf("%.0f%%", state.Progress*100))
		b.ctx.Spacer(4)

		if state.ErrorMessage != "" {
			b.ctx.LabelColored(state.ErrorMessage, ui2d.Color{R: 1, G: 0.3, B: 0.3, A: 1})
		}

		b.ctx.LabelColored(fmt.Sprintf("Phase: %s", state.Phase), ui2d.ColorTextDim)

		b.ctx.EndWindow()
	}
}

// RenderInGameUI renders the in-game HUD.
func (b *UI2DBackend) RenderInGameUI(state InGameUIState, dt float64, width, height float32) {
	// Draw scene texture as background
	if state.SceneReady && state.SceneTexture != 0 {
		b.ctx.Renderer().DrawSceneTexture(0, 0, width, height, state.SceneTexture)
	}

	// Debug overlay (top-left)
	if state.ShowDebugInfo {
		if b.ctx.BeginWindow("debug", 10, 10, 320, 105, "Debug") {
			b.ctx.Row(16)
			b.ctx.Label(fmt.Sprintf("Map: %s", state.MapName))
			b.ctx.Row(16)
			b.ctx.Label(fmt.Sprintf("Tile: (%d, %d)", state.PlayerTileX, state.PlayerTileY))
			b.ctx.Row(16)
			b.ctx.Label(fmt.Sprintf("Pos: (%.0f, %.0f, %.0f)", state.PlayerX, state.PlayerY, state.PlayerZ))
			b.ctx.Separator()
			b.ctx.Row(16)
			b.ctx.Label(fmt.Sprintf("Dir: %d  Entities: %d", state.PlayerDirection, state.EntityCount))
			b.ctx.EndWindow()
		}
	}

	// Error overlay
	if state.ErrorMessage != "" {
		windowWidth := float32(300)
		windowHeight := float32(80)
		windowX := (width - windowWidth) / 2
		windowY := (height - windowHeight) / 2

		if b.ctx.BeginWindow("error", windowX, windowY, windowWidth, windowHeight, "Error") {
			b.ctx.Spacer(4)
			b.ctx.LabelColored(state.ErrorMessage, ui2d.Color{R: 1, G: 0.3, B: 0.3, A: 1})
			b.ctx.EndWindow()
		}
	}

	// Bottom status bar (drawn as simple text, not a window)
	statusText := state.MapName
	if state.StatusMessage != "" {
		statusText = state.StatusMessage
	}
	scale := float32(2.0)
	barY := height - 25
	b.ctx.Renderer().DrawRect(0, barY, width, 25, ui2d.ColorPanelBg)
	b.ctx.Renderer().DrawText(10, barY+4, statusText, scale, ui2d.ColorTextOnDark)

	posText := fmt.Sprintf("(%d, %d)", state.PlayerTileX, state.PlayerTileY)
	posW, _ := b.ctx.Renderer().MeasureText(posText, scale)
	b.ctx.Renderer().DrawText(width-posW-10, barY+4, posText, scale, ui2d.ColorTextOnDark)
}

// RenderFPSOverlay renders an FPS counter.
func (b *UI2DBackend) RenderFPSOverlay(fps float64, width, height float32) {
	scale := float32(2.0)
	text := fmt.Sprintf("FPS: %.0f", fps)
	textW, _ := b.ctx.Renderer().MeasureText(text, scale)

	x := width - textW - 10
	y := float32(5)

	// Semi-transparent background
	b.ctx.Renderer().DrawRect(x-5, y-2, textW+10, 20, ui2d.ColorPanelBg.WithAlpha(0.5))
	b.ctx.Renderer().DrawText(x, y, text, scale, ui2d.ColorTextOnDark)
}

// RenderScreenshotMessage renders a screenshot notification.
func (b *UI2DBackend) RenderScreenshotMessage(msg string, width, height float32) {
	scale := float32(2.0)
	textW, textH := b.ctx.Renderer().MeasureText(msg, scale)

	msgWidth := textW + 20
	x := (width - msgWidth) / 2
	y := height - 60

	// Semi-transparent background
	b.ctx.Renderer().DrawRect(x, y, msgWidth, textH+10, ui2d.ColorPanelBg.WithAlpha(0.8))
	b.ctx.Renderer().DrawText(x+10, y+5, msg, scale, ui2d.Color{R: 0.2, G: 1.0, B: 0.2, A: 1.0})
}
