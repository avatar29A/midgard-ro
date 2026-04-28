// Package ui provides game user interface components.
package ui

import (
	"fmt"
	"strings"

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
	loginBgTex    *TextureInfo // t_login.jpg — fullscreen title backdrop
	loginTexTried bool         // avoid repeated load attempts

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


	// Bridge ImGui's per-frame character input queue into ui2d's TextInput
	// so users can type into our text fields. ImGui already translates
	// SDL2 SDL_TEXTINPUT events into Wchars on its IO; we just consume the
	// queue. Non-ASCII Wchars become multi-byte UTF-8 via rune conversion.
	if chars := io.InputQueueCharacters().Slice(); len(chars) > 0 {
		var buf strings.Builder
		for _, ch := range chars {
			if ch >= 32 { // skip control chars; backspace/enter handled via key flags
				buf.WriteRune(rune(ch))
			}
		}
		in.TextInput += buf.String()
	}
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

	// Try to load input skin (RO `name-edit.bmp`). If the asset is missing
	// the inputs fall back to drawSunkenInput's procedural bevel.
	if inputSkin, err := LoadInputSkin(b.texCache); err == nil && inputSkin != nil {
		b.ctx.SetDefaultInputSkin(inputSkin)
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

// loginUIBasePath is the GRF path under which RO stores all login-screen
// textures (the Korean folder name means "user interface").
const loginUIBasePath = `data\texture\유저인터페이스\`

// loadLoginTextures lazy-loads the login-screen backdrop. We use
// `t_login.jpg`, the Korean RO client's title-screen art, drawn fullscreen
// behind the dialog. Dialog chrome itself is rendered from the generic
// `win_msgbox` skin so labels and buttons stay text-driven (translatable);
// the per-screen `win_login.bmp` is *not* used because its Korean labels
// are baked into the artwork.
func (b *UI2DBackend) loadLoginTextures() {
	if b.loginTexTried || b.texCache == nil {
		return
	}
	b.loginTexTried = true

	if bg, err := b.texCache.Load(loginUIBasePath + `t_login.jpg`); err == nil {
		b.loginBgTex = bg
	}
}

// RenderLoginUI renders the login screen.
//
// Layout: t_login.jpg fills the screen as the title backdrop; the dialog
// itself is a generic themed window centered on top. Labels and the Login
// button are drawn from Go strings (not from baked-in BMP artwork) so they
// remain translatable.
func (b *UI2DBackend) RenderLoginUI(state LoginUIState, width, height float32) {
	b.loadLoginTextures()

	if b.loginBgTex != nil {
		b.ctx.Renderer().DrawImage(b.loginBgTex.ID, 0, 0, width, height, ui2d.ColorWhite)
	}

	if b.loginUsername == "" && state.Username != "" {
		b.loginUsername = state.Username
	}
	if b.loginPassword == "" && state.Password != "" {
		b.loginPassword = state.Password
	}

	// Compact dialog modeled on the original RO "Log On" — labels sit
	// LEFT of inputs (not above), and login/exit buttons live in the
	// bottom-right corner. HStack flex sizes the inputs to fill the
	// remaining width after the fixed-width label column.
	windowWidth := float32(420)
	windowHeight := float32(190)
	windowX := (width - windowWidth) / 2
	windowY := (height - windowHeight) / 2

	if b.ctx.BeginWindow("login", windowX, windowY, windowWidth, windowHeight, "Log On") {
		doLogin := func() {
			if !state.IsLoading && state.OnLogin != nil {
				state.OnLogin()
			}
		}

		labelW := float32(80) // wide enough for "Password" without wrap

		idRow := ui2d.HStack(8,
			ui2d.Sized(labelW, 0, ui2d.Label("ID")),
			ui2d.Sized(0, 22, ui2d.TextInput("username", &b.loginUsername, nil)),
		)
		passRow := ui2d.HStack(8,
			ui2d.Sized(labelW, 0, ui2d.Label("Password")),
			ui2d.Sized(0, 22, ui2d.PasswordInput("password", &b.loginPassword, doLogin)),
		)

		// Bottom action row: Filler pushes the buttons to the right edge.
		// 28px tall gives the radius-6 corners visible vertical space
		// (h - 2*r = 16px straight middle) without the button looking
		// chunky.
		btnW := float32(80)
		btnH := float32(28)
		btnRow := ui2d.HStack(6,
			ui2d.Filler(),
			ui2d.Sized(btnW, btnH, ui2d.Button("login", "login", doLogin)),
			ui2d.Sized(btnW, btnH, ui2d.Button("exit", "exit", func() {
				if state.OnExit != nil {
					state.OnExit()
				}
			})),
		)

		var rows []ui2d.Element
		rows = append(rows, idRow, passRow)
		if state.ErrorMessage != "" {
			rows = append(rows, ui2d.LabelColor(state.ErrorMessage, ui2d.Color{R: 1, G: 0.4, B: 0.4, A: 1}))
		}
		if state.IsLoading {
			rows = append(rows, ui2d.LabelCenteredEl("Connecting..."))
		}
		rows = append(rows,
			ui2d.LabelColor("Server: "+state.ServerName, ui2d.ColorTextDim),
			ui2d.Filler(), // pushes button row to bottom
			btnRow,
		)

		// Notify owners if the tree mutated the editable values.
		prevUser := state.Username
		prevPass := state.Password
		b.ctx.RenderTree(ui2d.VStack(8, rows...), b.ctx.CurrentWindowContentRect())
		if b.loginUsername != prevUser && state.OnUsernameChange != nil {
			state.OnUsernameChange(b.loginUsername)
		}
		if b.loginPassword != prevPass && state.OnPasswordChange != nil {
			state.OnPasswordChange(b.loginPassword)
		}

		b.ctx.EndWindow()
	}
}

// RenderConnectingUI renders the connecting screen — same backdrop as
// login/charselect, themed window with status messages.
func (b *UI2DBackend) RenderConnectingUI(state ConnectingUIState, width, height float32) {
	b.loadLoginTextures()
	if b.loginBgTex != nil {
		b.ctx.Renderer().DrawImage(b.loginBgTex.ID, 0, 0, width, height, ui2d.ColorWhite)
	}

	windowWidth := float32(320)
	windowHeight := float32(150)
	windowX := (width - windowWidth) / 2
	windowY := (height - windowHeight) / 2

	if b.ctx.BeginWindow("connecting", windowX, windowY, windowWidth, windowHeight, "Connecting") {
		var rows []ui2d.Element
		if state.StatusMessage != "" {
			rows = append(rows, ui2d.LabelCenteredEl(state.StatusMessage))
		}
		if state.ErrorMessage != "" {
			rows = append(rows, ui2d.LabelColor(state.ErrorMessage, ui2d.Color{R: 1, G: 0.4, B: 0.4, A: 1}))
		}
		rows = append(rows, ui2d.Spacer(8), ui2d.LabelCenteredEl("Please wait..."))

		b.ctx.RenderTree(ui2d.VStack(8, rows...), b.ctx.CurrentWindowContentRect())
		b.ctx.EndWindow()
	}
}

// RenderCharSelectUI renders the character selection screen.
//
// Layout: t_login.jpg backdrop, centered themed window. The body is a
// declarative tree — VStack of status/error rows, character Selectables,
// detail labels, and the Enter Game button — so positions are deterministic
// and every row gets the same vertical rhythm.
func (b *UI2DBackend) RenderCharSelectUI(state CharSelectUIState, width, height float32) {
	// Reuse the title-screen backdrop: it doubles as the char-select
	// scenery in vanilla RO and saves loading another big asset.
	b.loadLoginTextures()
	if b.loginBgTex != nil {
		b.ctx.Renderer().DrawImage(b.loginBgTex.ID, 0, 0, width, height, ui2d.ColorWhite)
	}

	windowWidth := float32(420)
	windowHeight := float32(420)
	windowX := (width - windowWidth) / 2
	windowY := (height - windowHeight) / 2

	if b.ctx.BeginWindow("charselect", windowX, windowY, windowWidth, windowHeight, "Character Selection") {
		// Auto-select first row when characters arrive.
		if state.IsReady && b.charSelectIdx < 0 && len(state.Characters) > 0 {
			b.charSelectIdx = 0
			if state.OnSelectIndex != nil {
				state.OnSelectIndex(0)
			}
		}

		var rows []ui2d.Element
		if state.StatusMessage != "" {
			rows = append(rows, ui2d.Label(state.StatusMessage))
		}
		if state.ErrorMessage != "" {
			rows = append(rows, ui2d.LabelColor(state.ErrorMessage, ui2d.Color{R: 1, G: 0.4, B: 0.4, A: 1}))
		}

		switch {
		case !state.IsReady:
			rows = append(rows, ui2d.Spacer(8), ui2d.LabelCenteredEl("Loading character list..."))
		case len(state.Characters) == 0:
			rows = append(rows,
				ui2d.Spacer(8),
				ui2d.LabelCenteredEl("No characters found."),
				ui2d.Spacer(4),
				ui2d.LabelCenteredEl("Create a new character on the server."),
			)
		default:
			rows = append(rows, ui2d.Label("Characters:"))
			for i, char := range state.Characters {
				idx := i // capture for closure
				rows = append(rows, ui2d.Sized(0, 24, ui2d.Selectable(
					fmt.Sprintf("char_%d", i),
					fmt.Sprintf("%s  (Lv %d)", char.GetName(), char.BaseLevel),
					b.charSelectIdx == i,
					func() {
						b.charSelectIdx = idx
						if state.OnSelectIndex != nil {
							state.OnSelectIndex(idx)
						}
					},
				)))
			}

			if b.charSelectIdx >= 0 && b.charSelectIdx < len(state.Characters) {
				char := state.Characters[b.charSelectIdx]
				rows = append(rows,
					ui2d.Spacer(8),
					ui2d.Label(fmt.Sprintf("HP: %d/%d    SP: %d/%d", char.HP, char.MaxHP, char.SP, char.MaxSP)),
					ui2d.Label(fmt.Sprintf("Map: %s", char.GetMapName())),
				)
			}

			rows = append(rows, ui2d.Spacer(8))
			canEnter := !state.IsLoading && b.charSelectIdx >= 0
			enterLabel := "Enter Game"
			rows = append(rows, ui2d.Sized(0, 36, ui2d.Button("enter", enterLabel, func() {
				if canEnter && state.OnSelect != nil {
					state.OnSelect(b.charSelectIdx)
				}
			})))
		}

		b.ctx.RenderTree(ui2d.VStack(6, rows...), b.ctx.CurrentWindowContentRect())
		b.ctx.EndWindow()
	}
}

// RenderLoadingUI renders the loading screen the same way the original
// RO client does: just the title backdrop fills the screen and a single
// short progress bar sits centered on the X-axis near the bottom — no
// dialog chrome, no labels, no percentage caption.
func (b *UI2DBackend) RenderLoadingUI(state LoadingUIState, width, height float32) {
	b.loadLoginTextures()
	if b.loginBgTex != nil {
		b.ctx.Renderer().DrawImage(b.loginBgTex.ID, 0, 0, width, height, ui2d.ColorWhite)
	}

	barW := float32(280)
	barH := float32(16)
	barX := (width - barW) / 2
	barY := height - 60

	b.ctx.ProgressBarAt(barX, barY, barW, barH, state.Progress, fmt.Sprintf("%.0f%%", state.Progress*100))

	// Debug gate hint — visible once loading is done and the state is
	// waiting on Enter. Sits just above the progress bar in light text.
	if state.ReadyForInput {
		hint := "Press Enter to continue"
		tw, _ := b.ctx.Renderer().MeasureText(hint, 1.0)
		b.ctx.LabelAtColored((width-tw)/2, barY-26, hint, ui2d.ColorTextOnDark)
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
	scale := float32(1.0)
	barY := height - 25
	b.ctx.Renderer().DrawRect(0, barY, width, 25, ui2d.ColorPanelBg)
	b.ctx.Renderer().DrawText(10, barY+4, statusText, scale, ui2d.ColorTextOnDark)

	posText := fmt.Sprintf("(%d, %d)", state.PlayerTileX, state.PlayerTileY)
	posW, _ := b.ctx.Renderer().MeasureText(posText, scale)
	b.ctx.Renderer().DrawText(width-posW-10, barY+4, posText, scale, ui2d.ColorTextOnDark)
}

// RenderFPSOverlay renders an FPS counter.
func (b *UI2DBackend) RenderFPSOverlay(fps float64, width, height float32) {
	scale := float32(1.0)
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
	scale := float32(1.0)
	textW, textH := b.ctx.Renderer().MeasureText(msg, scale)

	msgWidth := textW + 20
	x := (width - msgWidth) / 2
	y := height - 60

	// Semi-transparent background
	b.ctx.Renderer().DrawRect(x, y, msgWidth, textH+10, ui2d.ColorPanelBg.WithAlpha(0.8))
	b.ctx.Renderer().DrawText(x+10, y+5, msg, scale, ui2d.Color{R: 0.2, G: 1.0, B: 0.2, A: 1.0})
}
