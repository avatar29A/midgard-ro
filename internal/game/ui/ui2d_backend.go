// Package ui provides game user interface components.
package ui

import (
	"fmt"

	"github.com/Faultbox/midgard-ro/internal/engine/ui2d"
)

// UI2DBackend implements UIBackend using the custom ui2d rendering system.
type UI2DBackend struct {
	ctx *ui2d.Context

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
func (b *UI2DBackend) Begin() {
	b.ctx.Begin()
}

// End finishes the UI frame.
func (b *UI2DBackend) End() {
	b.ctx.End()
}

// Close releases backend resources.
func (b *UI2DBackend) Close() {
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

// RenderLoginUI renders the login screen.
func (b *UI2DBackend) RenderLoginUI(state LoginUIState, width, height float32) {
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
		if b.ctx.BeginWindow("debug", 10, 10, 220, 180, "Debug") {
			b.ctx.Label(fmt.Sprintf("Map: %s", state.MapName))
			b.ctx.Spacer(4)
			b.ctx.Label(fmt.Sprintf("Tile: (%d, %d)", state.PlayerTileX, state.PlayerTileY))
			b.ctx.Spacer(4)
			b.ctx.Label(fmt.Sprintf("World: (%.1f, %.1f, %.1f)", state.PlayerX, state.PlayerY, state.PlayerZ))
			b.ctx.Spacer(4)
			b.ctx.Label(fmt.Sprintf("Dir: %d", state.PlayerDirection))
			b.ctx.Separator()
			b.ctx.Label(fmt.Sprintf("Entities: %d", state.EntityCount))
			b.ctx.Spacer(4)
			b.ctx.Label(fmt.Sprintf("FPS: %.0f", state.FPS))
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
	b.ctx.Renderer().DrawText(10, barY+4, statusText, scale, ui2d.ColorText)

	posText := fmt.Sprintf("(%d, %d)", state.PlayerTileX, state.PlayerTileY)
	posW, _ := b.ctx.Renderer().MeasureText(posText, scale)
	b.ctx.Renderer().DrawText(width-posW-10, barY+4, posText, scale, ui2d.ColorText)
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
	b.ctx.Renderer().DrawText(x, y, text, scale, ui2d.ColorText)
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
