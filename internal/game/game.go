// Package game implements the main game loop and state management.
package game

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/AllenDang/cimgui-go/backend"
	"github.com/AllenDang/cimgui-go/backend/sdlbackend"
	"github.com/AllenDang/cimgui-go/imgui"
	"github.com/go-gl/gl/v4.1-core/gl"
	"go.uber.org/zap"

	"github.com/Faultbox/midgard-ro/internal/assets"
	"github.com/Faultbox/midgard-ro/internal/config"
	"github.com/Faultbox/midgard-ro/internal/game/states"
	"github.com/Faultbox/midgard-ro/internal/game/ui"
	"github.com/Faultbox/midgard-ro/internal/logger"
	"github.com/Faultbox/midgard-ro/internal/network"
	"github.com/Faultbox/midgard-ro/internal/trace"
)

// koreanGlyphRanges defines the Unicode ranges for Korean text rendering.
var koreanGlyphRanges = []imgui.Wchar{
	0x0020, 0x00FF, // Basic Latin + Latin Supplement
	0x3000, 0x30FF, // CJK Symbols and Punctuation, Hiragana, Katakana
	0x3130, 0x318F, // Hangul Compatibility Jamo
	0xAC00, 0xD7AF, // Hangul Syllables
	0, // Terminator
}

// Game is the main game instance.
type Game struct {
	config  *config.Config
	running bool

	// ImGui backend (for windowing and OpenGL context)
	imguiBackend backend.Backend[sdlbackend.SDLWindowFlags]

	// UI backend abstraction (for rendering UI)
	uiBackend ui.UIBackend

	// State management
	stateManager *states.Manager
	client       *network.Client

	// Assets
	assetManager *assets.Manager

	// Timing
	lastTime   time.Time
	frameCount int
	fps        float64
	fpsTimer   time.Time
	dt         float64 // Delta time in seconds

	// Screenshot support
	screenshotDir       string
	screenshotRequested bool
	screenshotMsg       string
	screenshotMsgTime   time.Time

	// Input tracking
	lastMouseX float32
	lastMouseY float32

	// Deferred actions (execute next frame for visual feedback)
	pendingAction func()

	// Debug overlay toggle (F3). Default off so the HUD isn't cluttered;
	// turn on to inspect player/camera/scene/network telemetry live.
	showDebug bool

	// Unattended screenshot capture (--screenshot-after / --screenshot-every),
	// so the UI can be inspected without someone sitting at the keyboard to
	// press F12.
	shotAfter time.Duration
	shotEvery time.Duration
	shotOnce  bool
	shotLast  time.Time
	startedAt time.Time

	// Per-phase frame cost, accumulated for the render trace.
	costTimer   time.Time
	costSamples int
	costUpdate  float64
	costScene   float64
	costUI      float64
	costTotal   float64
	costWorst   float64
}

// New creates a new game instance with ImGui windowing (backward compatible).
// For external windowing (e.g., SDL2), use NewHeadless() instead.
func New(cfg *config.Config) (*Game, error) {
	runtime.LockOSThread()

	logger.Info("initializing game",
		zap.Int("width", cfg.Graphics.Width),
		zap.Int("height", cfg.Graphics.Height),
		zap.Bool("fullscreen", cfg.Graphics.Fullscreen),
	)

	g := &Game{
		config:        cfg,
		running:       false,
		stateManager:  states.NewManager(),
		client:        network.New(),
		assetManager:  assets.NewManager(),
		screenshotDir: "data/Screenshots",
	}

	// Load GRF archives
	for _, grfPath := range cfg.Data.GRFPaths {
		if err := g.assetManager.AddArchive(grfPath); err != nil {
			logger.Warn("failed to load GRF archive", zap.String("path", grfPath), zap.Error(err))
		} else {
			logger.Info("loaded GRF archive", zap.String("path", grfPath))
		}
	}

	// Create ImGui backend (for windowing)
	var err error
	g.imguiBackend, err = backend.CreateBackend(sdlbackend.NewSDLBackend())
	if err != nil {
		return nil, fmt.Errorf("create backend: %w", err)
	}

	// Set up font loading hook before creating window
	g.imguiBackend.SetAfterCreateContextHook(func() {
		// CRITICAL: Disable viewports to prevent separate OS windows
		io := imgui.CurrentIO()
		flags := io.ConfigFlags()
		flags &^= imgui.ConfigFlagsViewportsEnable // Clear viewport flag
		io.SetConfigFlags(flags)

		g.loadKoreanFont()
	})

	g.imguiBackend.SetBgColor(imgui.NewVec4(0.05, 0.05, 0.08, 1.0))
	g.imguiBackend.CreateWindow("Midgard RO", cfg.Graphics.Width, cfg.Graphics.Height)

	// Initialize OpenGL
	if err := gl.Init(); err != nil {
		return nil, fmt.Errorf("init opengl: %w", err)
	}

	version := gl.GoStr(gl.GetString(gl.VERSION))
	renderer := gl.GoStr(gl.GetString(gl.RENDERER))
	logger.Info("OpenGL initialized",
		zap.String("version", version),
		zap.String("renderer", renderer),
	)

	// Initialize game state
	if err := g.initGameState(cfg); err != nil {
		return nil, err
	}

	// Create UI backend. We swapped from ImGui to the custom ui2d renderer
	// (see RFC #67) — ImGui stays only as the SDL/GL windowing host.
	ui2dBackend, err := ui.NewUI2DBackend(cfg.Graphics.Width, cfg.Graphics.Height)
	if err != nil {
		return nil, fmt.Errorf("create ui2d backend: %w", err)
	}
	ui2dBackend.SetAssetLoader(g.assetManager.Load)
	g.uiBackend = ui2dBackend

	logger.Info("game initialized successfully")
	return g, nil
}

// NewHeadless creates a new game instance without creating a window.
// The caller is responsible for:
// - Creating the OpenGL context (via SDL2 or other)
// - Calling SetUIBackend() to set the UI renderer
// - Calling InitTiming() before the main loop
// - Calling Update() and RenderUI() each frame
func NewHeadless(cfg *config.Config) (*Game, error) {
	logger.Info("initializing headless game",
		zap.Int("width", cfg.Graphics.Width),
		zap.Int("height", cfg.Graphics.Height),
	)

	g := &Game{
		config:        cfg,
		running:       false,
		stateManager:  states.NewManager(),
		client:        network.New(),
		assetManager:  assets.NewManager(),
		screenshotDir: "data/Screenshots",
	}

	// Load GRF archives
	for _, grfPath := range cfg.Data.GRFPaths {
		if err := g.assetManager.AddArchive(grfPath); err != nil {
			logger.Warn("failed to load GRF archive", zap.String("path", grfPath), zap.Error(err))
		} else {
			logger.Info("loaded GRF archive", zap.String("path", grfPath))
		}
	}

	// Initialize game state
	if err := g.initGameState(cfg); err != nil {
		return nil, err
	}

	logger.Info("headless game initialized successfully")
	return g, nil
}

// initGameState initializes the game state machine with login state.
func (g *Game) initGameState(cfg *config.Config) error {
	// Initialize with login state
	loginCfg := states.LoginStateConfig{
		ServerHost:    cfg.Network.LoginServer,
		ServerPort:    6900, // Default RO login port
		ClientVersion: 55,   // rAthena compatible version
		Username:      cfg.Network.Username,
		Password:      cfg.Network.Password,
	}

	// Parse server address
	if host, port := parseHostPort(cfg.Network.LoginServer); host != "" {
		loginCfg.ServerHost = host
		loginCfg.ServerPort = port
	}

	// Set texture loader for states
	g.stateManager.SetTexLoader(g.assetManager.Load)

	loginState := states.NewLoginState(loginCfg, g.client, g.stateManager)
	g.stateManager.Change(loginState)

	return nil
}

// loadKoreanFont loads a font with Korean glyph support.
func (g *Game) loadKoreanFont() {
	io := imgui.CurrentIO()
	fonts := io.Fonts()

	// Try different font paths (cross-platform support)
	fontPaths := []string{
		"/Library/Fonts/Arial Unicode.ttf",
		"/System/Library/Fonts/Supplemental/Arial Unicode.ttf",
		"/System/Library/Fonts/AppleSDGothicNeo.ttc",
		"C:\\Windows\\Fonts\\malgun.ttf",
		"C:\\Windows\\Fonts\\gulim.ttc",
		"/usr/share/fonts/truetype/noto/NotoSansCJK-Regular.ttc",
		"/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc",
	}

	var fontPath string
	for _, path := range fontPaths {
		if fileExists(path) {
			fontPath = path
			break
		}
	}

	if fontPath == "" {
		logger.Debug("no Korean font found, using default")
		return
	}

	fontCfg := imgui.NewFontConfig()
	defer fontCfg.Destroy()

	fonts.AddFontFromFileTTFV(fontPath, 16.0, fontCfg, &koreanGlyphRanges[0])
	logger.Debug("loaded Korean font", zap.String("path", fontPath))
}

// Run starts the main game loop.
func (g *Game) Run() error {
	g.running = true
	g.lastTime = time.Now()
	g.fpsTimer = time.Now()

	logger.Info("starting game loop")

	// Run with ImGui backend
	g.imguiBackend.Run(func() {
		g.frame()
	})

	return nil
}

// frame processes a single frame.
func (g *Game) frame() {
	// Run any pending UI action from the previous frame (login, char-select, etc).
	// Deferred one frame so the click visibly highlights before the action fires.
	if g.pendingAction != nil {
		action := g.pendingAction
		g.pendingAction = nil
		action()
	}

	// Calculate delta time
	now := time.Now()
	g.dt = now.Sub(g.lastTime).Seconds()
	g.lastTime = now

	// Update FPS counter
	g.frameCount++
	if time.Since(g.fpsTimer) >= time.Second {
		g.fps = float64(g.frameCount)
		g.frameCount = 0
		g.fpsTimer = time.Now()

		if g.config.Game.ShowFPS {
			logger.Debug("fps", zap.Float64("count", g.fps))
		}
	}

	// Handle ESC to quit
	if imgui.IsKeyPressedBoolV(imgui.KeyEscape, false) {
		g.running = false
		g.imguiBackend.SetShouldClose(true)
	}

	// Handle F12 for screenshot (will capture at start of NEXT frame)
	if imgui.IsKeyChordPressed(imgui.KeyChord(imgui.KeyF12)) {
		g.screenshotRequested = true
	}

	g.checkTimedScreenshot()

	// F3 toggles the in-game debug overlay (player/camera/scene/network).
	if imgui.IsKeyPressedBoolV(imgui.KeyF3, false) {
		g.showDebug = !g.showDebug
	}

	// F4 hides map objects, leaving bare terrain. Anything still wrong on
	// screen with the models gone belongs to the terrain mesh — otherwise the
	// two are hard to tell apart where objects sit flush against the ground.
	if imgui.IsKeyPressedBoolV(imgui.KeyF4, false) {
		if inGame, ok := g.stateManager.Current().(*states.InGameState); ok {
			if sc := inGame.GetScene(); sc != nil {
				sc.HideModels = !sc.HideModels
				logger.Info("models toggled", zap.Bool("hidden", sc.HideModels))
			}
		}
	}

	// Handle camera controls when in InGameState
	if inGameState, ok := g.stateManager.Current().(*states.InGameState); ok {
		g.handleInGameInput(inGameState)
	}

	// Update state machine
	updateStart := time.Now()
	if err := g.stateManager.Update(g.dt); err != nil {
		logger.Error("state update error", zap.Error(err))
	}
	updateMs := msSince(updateStart)

	// Render 3D scene (if applicable)
	sceneStart := time.Now()
	if err := g.stateManager.Render(); err != nil {
		logger.Error("state render error", zap.Error(err))
	}
	sceneMs := msSince(sceneStart)

	// Render UI based on current state
	uiStart := time.Now()
	g.renderUI()
	uiMs := msSince(uiStart)

	g.recordFrameCost(updateMs, sceneMs, uiMs)

	// Capture screenshot AFTER rendering (from back buffer before swap)
	if g.screenshotRequested {
		g.screenshotRequested = false
		g.captureScreenshot()
	}
}

// SetScreenshotTimers arms the unattended capture. A zero duration disables
// that timer.
func (g *Game) SetScreenshotTimers(after, every time.Duration) {
	g.shotAfter = after
	g.shotEvery = every
	g.startedAt = time.Now()
	g.shotLast = time.Now()
}

// checkTimedScreenshot fires the unattended capture timers. Both are off
// unless asked for on the command line.
func (g *Game) checkTimedScreenshot() {
	if g.startedAt.IsZero() {
		g.startedAt = time.Now()
	}

	if g.shotAfter > 0 && !g.shotOnce && time.Since(g.startedAt) >= g.shotAfter {
		g.shotOnce = true
		g.screenshotRequested = true
		return
	}

	if g.shotEvery > 0 && time.Since(g.shotLast) >= g.shotEvery {
		g.shotLast = time.Now()
		g.screenshotRequested = true
	}
}

// msSince returns elapsed milliseconds as a float.
func msSince(t time.Time) float64 {
	return float64(time.Since(t).Microseconds()) / 1000
}

// recordFrameCost accumulates per-phase timings and reports them once a
// second on the render trace channel.
//
// The phases are measured separately because they fail differently: a slow
// scene means draw calls or fill rate, a slow UI means our 2D batching, and a
// frame that is slow while every phase is fast means we are blocked in the
// buffer swap waiting on vsync — which caps at exact divisors of the refresh
// rate, so being a hair over budget drops you straight from 60 to 30.
func (g *Game) recordFrameCost(updateMs, sceneMs, uiMs float64) {
	if !trace.On(trace.Render) {
		return
	}

	g.costSamples++
	g.costUpdate += updateMs
	g.costScene += sceneMs
	g.costUI += uiMs
	total := updateMs + sceneMs + uiMs
	g.costTotal += total
	if total > g.costWorst {
		g.costWorst = total
	}

	if time.Since(g.costTimer) < time.Second {
		return
	}

	n := float64(g.costSamples)
	if n > 0 {
		trace.Emit(trace.Render, "frame",
			zap.Float64("fps", g.fps),
			zap.Float64("avgWorkMs", g.costTotal/n),
			zap.Float64("worstWorkMs", g.costWorst),
			zap.Float64("updateMs", g.costUpdate/n),
			zap.Float64("sceneMs", g.costScene/n),
			zap.Float64("uiMs", g.costUI/n),
			// Whatever is left of the frame budget is spent blocked in the
			// swap. A large value here with small phase times means vsync.
			zap.Float64("blockedMs", frameBudgetMs(g.fps)-g.costTotal/n))
	}

	g.costSamples = 0
	g.costUpdate, g.costScene, g.costUI, g.costTotal, g.costWorst = 0, 0, 0, 0, 0
	g.costTimer = time.Now()
}

// frameBudgetMs is how long each frame actually took, derived from the
// measured frame rate.
func frameBudgetMs(fps float64) float64 {
	if fps <= 0 {
		return 0
	}
	return 1000 / fps
}

// renderUI renders the appropriate UI for the current state.
func (g *Game) renderUI() {
	viewportWidth, viewportHeight := g.uiBackend.GetScreenSize()

	// Begin UI frame
	g.uiBackend.Begin()

	// Render based on current state type
	switch state := g.stateManager.Current().(type) {
	case *states.LoginState:
		g.uiBackend.RenderLoginUI(ui.LoginUIState{
			Username:     state.GetUsername(),
			Password:     state.GetPassword(),
			ErrorMessage: state.GetErrorMessage(),
			IsLoading:    state.IsLoadingState(),
			ServerName:   g.config.Network.LoginServer,
			OnUsernameChange: func(s string) {
				state.SetUsername(s)
			},
			OnPasswordChange: func(s string) {
				state.SetPassword(s)
			},
			OnLogin: func() {
				g.pendingAction = func() {
					_ = state.AttemptLogin()
				}
			},
		}, viewportWidth, viewportHeight)

	case *states.ConnectingState:
		g.uiBackend.RenderConnectingUI(ui.ConnectingUIState{
			StatusMessage: state.GetStatusMessage(),
			ErrorMessage:  state.GetErrorMessage(),
		}, viewportWidth, viewportHeight)

	case *states.CharSelectState:
		g.uiBackend.RenderCharSelectUI(ui.CharSelectUIState{
			Characters:    state.GetCharacters(),
			SelectedIndex: -1, // Managed by the backend
			StatusMessage: state.GetStatusMessage(),
			ErrorMessage:  state.GetErrorMessage(),
			IsLoading:     state.IsLoadingState(),
			IsReady:       state.IsCharListReady(),
			OnSelect: func(index int) {
				g.pendingAction = func() {
					_ = state.SelectCharacter(index)
				}
			},
		}, viewportWidth, viewportHeight)

	case *states.LoadingState:
		g.uiBackend.RenderLoadingUI(ui.LoadingUIState{
			MapName:       state.GetMapName(),
			StatusMessage: state.GetStatusMessage(),
			ErrorMessage:  state.GetErrorMessage(),
			Progress:      state.GetProgress(),
			Phase:         state.GetLoadingPhase(),
		}, viewportWidth, viewportHeight)

	case *states.InGameState:
		var playerX, playerY, playerZ float32
		var playerTileX, playerTileY int
		var playerDirection uint8

		if player := state.GetPlayer(); player != nil {
			playerX, playerY, playerZ = player.RenderPosition()
			playerDirection = uint8(player.Direction)
		}
		playerTileX, playerTileY = state.GetPlayerTilePosition()

		uiState := ui.InGameUIState{
			MapName:         state.GetMapName(),
			PlayerX:         playerX,
			PlayerY:         playerY,
			PlayerZ:         playerZ,
			PlayerTileX:     playerTileX,
			PlayerTileY:     playerTileY,
			PlayerDirection: playerDirection,
			SceneReady:      state.IsSceneReady(),
			SceneTexture:    state.GetSceneTexture(),
			StatusMessage:   state.GetStatusMessage(),
			ErrorMessage:    state.GetErrorMessage(),
			ShowDebugInfo:   g.showDebug,
			FPS:             g.fps,
		}
		populateDebugFields(&uiState, state, g.client)
		g.uiBackend.RenderInGameUI(uiState, g.dt, viewportWidth, viewportHeight)

	default:
		// Show placeholder for unknown state (using ImGui directly for simplicity)
		imgui.SetNextWindowPos(imgui.NewVec2(viewportWidth/2-100, viewportHeight/2-20))
		if imgui.BeginV("##Loading", nil, imgui.WindowFlagsNoTitleBar|imgui.WindowFlagsNoResize|imgui.WindowFlagsAlwaysAutoResize) {
			imgui.Text("Loading...")
		}
		imgui.End()
	}

	// Debug: Show FPS overlay
	if g.config.Game.ShowFPS {
		g.uiBackend.RenderFPSOverlay(g.fps, viewportWidth, viewportHeight)
	}

	// Screenshot notification (show for 3 seconds)
	if g.screenshotMsg != "" && time.Since(g.screenshotMsgTime) < 3*time.Second {
		g.uiBackend.RenderScreenshotMessage(g.screenshotMsg, viewportWidth, viewportHeight)
	}

	// End UI frame
	g.uiBackend.End()
}

// Close cleans up game resources.
func (g *Game) Close() {
	logger.Info("closing game")

	if g.uiBackend != nil {
		g.uiBackend.Close()
	}

	if g.client != nil {
		g.client.Disconnect()
	}

	if g.assetManager != nil {
		g.assetManager.Close()
	}
}

// captureScreenshot captures the current frame to a PNG file.
func (g *Game) captureScreenshot() {
	var pixels []byte
	var width, height int

	// Get actual viewport size from OpenGL (handles HiDPI correctly)
	var viewport [4]int32
	gl.GetIntegerv(gl.VIEWPORT, &viewport[0])
	width = int(viewport[2])
	height = int(viewport[3])

	if width <= 0 || height <= 0 {
		logger.Warn("screenshot failed: invalid viewport")
		return
	}

	pixels = make([]byte, width*height*4)
	gl.ReadPixels(0, 0, int32(width), int32(height), gl.RGBA, gl.UNSIGNED_BYTE, gl.Ptr(pixels))

	// Flip vertically for default framebuffer
	rowSize := width * 4
	flipped := make([]byte, len(pixels))
	for y := 0; y < height; y++ {
		srcRow := (height - 1 - y) * rowSize
		dstRow := y * rowSize
		copy(flipped[dstRow:dstRow+rowSize], pixels[srcRow:srcRow+rowSize])
	}
	pixels = flipped

	// Create screenshot directory if needed
	if err := os.MkdirAll(g.screenshotDir, 0755); err != nil {
		logger.Warn("failed to create screenshot dir", zap.Error(err))
		return
	}

	// Create image (pixels are already in correct orientation from CaptureScene or flipped above)
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	copy(img.Pix, pixels)

	// Generate filename with timestamp
	timestamp := time.Now().Format("20060102-150405")
	filename := fmt.Sprintf("screenshot-%s.png", timestamp)
	savePath := filepath.Join(g.screenshotDir, filename)

	// Save to file
	file, err := os.Create(savePath)
	if err != nil {
		logger.Warn("failed to create screenshot file", zap.Error(err))
		return
	}
	defer file.Close()

	if err := png.Encode(file, img); err != nil {
		logger.Warn("failed to encode screenshot", zap.Error(err))
		return
	}

	// Also save as "latest.png" for easy access
	latestPath := filepath.Join(g.screenshotDir, "latest.png")
	if latestFile, err := os.Create(latestPath); err == nil {
		_ = png.Encode(latestFile, img)
		latestFile.Close()
	}

	g.screenshotMsg = fmt.Sprintf("Saved: %s", filename)
	g.screenshotMsgTime = time.Now()
	logger.Info("screenshot saved", zap.String("path", savePath))
}

// handleInGameInput handles camera and movement input when in game.
func (g *Game) handleInGameInput(state *states.InGameState) {
	camera := state.GetCamera()
	if camera == nil {
		return
	}

	io := imgui.CurrentIO()

	// Scroll wheel for zoom - use small multiplier for smooth zooming
	// Each scroll tick changes distance by ~20% (sensitivity 0.1 * delta 2 = 20%)
	scroll := io.MouseWheel()
	if scroll != 0 {
		camera.HandleZoom(scroll * 2)
	}

	// Get current mouse position, in the same point space we render into.
	//
	// cimgui-go's SDL backend reports the cursor in *global screen* points,
	// not relative to the window, so the window's screen position has to come
	// off first — exactly the correction UI2DBackend.syncInputFromImGui makes
	// for widget hit-testing. Click-to-move was missing it, so every ray was
	// cast from a point offset by wherever the window happened to sit on the
	// desktop; the further from the top-left corner, the further the character
	// walked from where you clicked.
	winPos := imgui.MainViewport().Pos()
	rawMouse := imgui.MousePos()
	mouseX := rawMouse.X - winPos.X
	mouseY := rawMouse.Y - winPos.Y

	// Right mouse button drag for camera rotation
	if imgui.IsMouseDragging(imgui.MouseButtonRight) {
		deltaX := mouseX - g.lastMouseX
		camera.HandleYaw(deltaX)
	}

	// Update last mouse position
	g.lastMouseX = mouseX
	g.lastMouseY = mouseY

	// WASD walking. Movement is relative to where the camera is looking, so
	// W always walks away from the viewer no matter how the camera is turned.
	// Each press asks the server for one cell; StepToward ignores us while a
	// walk is already in flight, so holding a key walks continuously.
	if !io.WantCaptureKeyboard() {
		var forward, strafe float32
		if imgui.IsKeyDown(imgui.KeyW) {
			forward++
		}
		if imgui.IsKeyDown(imgui.KeyS) {
			forward--
		}
		if imgui.IsKeyDown(imgui.KeyD) {
			strafe++
		}
		if imgui.IsKeyDown(imgui.KeyA) {
			strafe--
		}

		if forward != 0 || strafe != 0 {
			camDirX, camDirZ := camera.ForwardDirection()
			camRightX, camRightZ := camera.RightDirection()
			moveX := camDirX*forward + camRightX*strafe
			moveZ := camDirZ*forward + camRightZ*strafe
			if err := state.StepToward(moveX, moveZ); err != nil {
				logger.Warn("keyboard step failed", zap.Error(err))
			}
		}
	}

	// Left click for click-to-move. Skip if any imgui window (HUD, minimap,
	// chat, etc) is consuming the click; otherwise ray-cast to ground plane
	// and dispatch a server move request.
	if imgui.IsMouseClickedBool(imgui.MouseButtonLeft) && !io.WantCaptureMouse() {
		viewportW, viewportH := g.uiBackend.GetScreenSize()

		trace.Emit(trace.Pick, "click",
			zap.Float32("rawX", rawMouse.X), zap.Float32("rawY", rawMouse.Y),
			zap.Float32("windowX", winPos.X), zap.Float32("windowY", winPos.Y),
			zap.Float32("mouseX", mouseX), zap.Float32("mouseY", mouseY),
			zap.Float32("viewportW", viewportW), zap.Float32("viewportH", viewportH))

		if tileX, tileY, ok := state.ScreenToTile(mouseX, mouseY, viewportW, viewportH); ok {
			if err := state.RequestMove(tileX, tileY); err != nil {
				logger.Warn("click-to-move RequestMove failed", zap.Error(err))
			}
		} else {
			trace.Emit(trace.Pick, "miss")
		}
	}
}

// LoadAsset loads an asset from GRF archives.
func (g *Game) LoadAsset(path string) ([]byte, error) {
	return g.assetManager.Load(path)
}

// SetUIBackend allows setting a custom UI backend.
// This must be called before Run().
func (g *Game) SetUIBackend(backend ui.UIBackend) {
	if g.uiBackend != nil {
		g.uiBackend.Close()
	}
	g.uiBackend = backend
}

// StateManager returns the state manager.
func (g *Game) StateManager() *states.Manager {
	return g.stateManager
}

// NetworkClient returns the network client.
func (g *Game) NetworkClient() *network.Client {
	return g.client
}

// AssetManager returns the asset manager.
func (g *Game) AssetManager() *assets.Manager {
	return g.assetManager
}

// UIBackend returns the current UI backend.
func (g *Game) UIBackend() ui.UIBackend {
	return g.uiBackend
}

// FPS returns the current frames per second.
func (g *Game) FPS() float64 {
	return g.fps
}

// DeltaTime returns the time since the last frame in seconds.
func (g *Game) DeltaTime() float64 {
	return g.dt
}

// Update processes a single frame update.
// This can be called externally when using a custom event loop.
func (g *Game) Update() error {
	// Execute any pending action from previous frame (deferred for visual feedback)
	if g.pendingAction != nil {
		action := g.pendingAction
		g.pendingAction = nil
		action()
	}

	// Calculate delta time
	now := time.Now()
	g.dt = now.Sub(g.lastTime).Seconds()
	g.lastTime = now

	// Update FPS counter
	g.frameCount++
	if time.Since(g.fpsTimer) >= time.Second {
		g.fps = float64(g.frameCount)
		g.frameCount = 0
		g.fpsTimer = time.Now()
	}

	// Update state machine
	if err := g.stateManager.Update(g.dt); err != nil {
		logger.Error("state update error", zap.Error(err))
		return err
	}

	// Render 3D scene (if applicable)
	if err := g.stateManager.Render(); err != nil {
		logger.Error("state render error", zap.Error(err))
		return err
	}

	return nil
}

// RenderUI renders the UI for the current state.
// This can be called externally when using a custom event loop.
func (g *Game) RenderUI() {
	g.renderUI()
}

// HandleScreenshot requests a screenshot capture.
func (g *Game) HandleScreenshot() {
	g.screenshotRequested = true
}

// ProcessScreenshot processes any pending screenshot request.
func (g *Game) ProcessScreenshot() {
	if g.screenshotRequested {
		g.screenshotRequested = false
		g.captureScreenshot()
	}
}

// HandleInGameCameraInput handles camera controls when in InGameState.
func (g *Game) HandleInGameCameraInput(scrollDelta float32, mouseDeltaX float32, rightButtonDown bool) {
	inGameState, ok := g.stateManager.Current().(*states.InGameState)
	if !ok {
		return
	}

	camera := inGameState.GetCamera()
	if camera == nil {
		return
	}

	// Scroll wheel for zoom
	if scrollDelta != 0 {
		camera.HandleZoom(scrollDelta * 2)
	}

	// Right mouse button drag for camera rotation
	if rightButtonDown && mouseDeltaX != 0 {
		camera.HandleYaw(mouseDeltaX)
	}
}

// InitTiming initializes timing for the game loop.
func (g *Game) InitTiming() {
	g.lastTime = time.Now()
	g.fpsTimer = time.Now()
}

// parseHostPort extracts host and port from "host:port" string.
func parseHostPort(addr string) (string, int) {
	var host string
	var port int

	n, err := fmt.Sscanf(addr, "%s:%d", &host, &port)
	if err != nil || n != 2 {
		// Try with colons allowed in format
		for i := len(addr) - 1; i >= 0; i-- {
			if addr[i] == ':' {
				host = addr[:i]
				_, _ = fmt.Sscanf(addr[i+1:], "%d", &port)
				break
			}
		}
	}

	if port == 0 {
		port = 6900 // Default
	}

	return host, port
}

// fileExists checks if a file exists.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
