// Package game implements the main game loop and state management.
package game

import (
	"fmt"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/AllenDang/cimgui-go/backend"
	"github.com/AllenDang/cimgui-go/backend/sdlbackend"
	"github.com/AllenDang/cimgui-go/imgui"
	"github.com/go-gl/gl/v4.1-core/gl"
	"github.com/veandco/go-sdl2/sdl"
	"go.uber.org/zap"

	"github.com/Faultbox/midgard-ro/internal/assets"
	"github.com/Faultbox/midgard-ro/internal/config"
	"github.com/Faultbox/midgard-ro/internal/engine/audio"
	"github.com/Faultbox/midgard-ro/internal/engine/cursor"
	"github.com/Faultbox/midgard-ro/internal/game/entity"
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

	// Audio
	audioManager *audio.Manager
	bgm          *audio.LocationPlayer
	uiClickWarn  sync.Once

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

	// screenshots writes captured frames off the render thread.
	screenshots screenshotWriter

	// A cell to walk to once the map is up, from --walk-to. Fired once.
	walkToX, walkToY int
	walkToPending    bool

	// Lines to say once in game, from --say, drained in order.
	sayQueue []string
	sayNext  time.Time

	// Where to put the pointer once the map is up, from --mouse-at, in window
	// points. Fired once.
	mouseAtX, mouseAtY int
	mouseAtPending     bool

	// toggleBasicInfo is set for the frame Ctrl+V was pressed.
	toggleBasicInfo bool

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

		// The game draws RO's cursor itself. ImGui otherwise sets the system
		// cursor every frame, which puts it back however often it is hidden.
		flags |= imgui.ConfigFlagsNoMouseCursorChange
		io.SetConfigFlags(flags)

		g.loadKoreanFont()
	})

	g.imguiBackend.SetBgColor(imgui.NewVec4(0.05, 0.05, 0.08, 1.0))
	g.imguiBackend.CreateWindow("Midgard RO", cfg.Graphics.Width, cfg.Graphics.Height)

	// The game draws RO's own cursor, so the system one would be a second
	// pointer on screen. SDL owns it — the windowing backend runs on the same
	// library — and a failure here is cosmetic, not worth refusing to start.
	if _, err := sdl.ShowCursor(sdl.DISABLE); err != nil {
		logger.Warn("could not hide the system cursor", zap.Error(err))
	}

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
	g.attachUISounds(ui2dBackend)
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

	g.initAudio(cfg)

	g.stateManager.AutoPlay = config.AutoLogin()
	loginState := states.NewLoginState(loginCfg, g.client, g.stateManager)
	g.stateManager.Change(loginState)

	return nil
}

// uiClickSound is the sound the client plays when a button is pressed. The
// archives name it in Korean; the asset manager falls back to EUC-KR, so the
// UTF-8 literal resolves.
const uiClickSound = "data/wav/버튼소리.wav"

// clickSounder is the part of a UI backend that can be given a press sound.
type clickSounder interface {
	SetClickSound(play func())
}

// attachUISounds gives the UI backend its press sound, when both the backend
// and the audio support one.
func (g *Game) attachUISounds(backend ui.UIBackend) {
	sounder, ok := backend.(clickSounder)
	if !ok || g.audioManager == nil {
		return
	}

	sounder.SetClickSound(g.playUIClick)
}

// playUIClick plays the button press sound. A click is not worth failing or
// spamming the log over, so a missing sound is reported once and then ignored.
func (g *Game) playUIClick() {
	if g.audioManager == nil {
		return
	}

	data, err := g.assetManager.Load(uiClickSound)
	if err == nil {
		err = g.audioManager.PlaySFX(data)
	}

	if err != nil {
		g.uiClickWarn.Do(func() {
			logger.Warn("no button press sound",
				zap.String("path", uiClickSound), zap.Error(err))
		})
	}
}

// initAudio brings up audio playback and the location background music.
// Sound is a nicety: when the device or the music is unavailable the game runs
// on in silence.
func (g *Game) initAudio(cfg *config.Config) {
	manager := audio.New()
	if err := manager.Init(); err != nil {
		logger.Warn("audio unavailable, continuing without sound", zap.Error(err))

		return
	}

	master := float64(cfg.Audio.MasterVolume)
	if cfg.Audio.Muted {
		master = 0
	}

	manager.SetMasterVolume(master)
	manager.SetBGMVolume(float64(cfg.Audio.MusicVolume))
	manager.SetSFXVolume(float64(cfg.Audio.SFXVolume))

	// What the sound dialog was last left at wins over the configured
	// levels: config.yaml is what the file says, this is what the player did.
	if saved := config.LoadUIState(); saved.SoundSet {
		bgm, sfx := saved.BGMVolume, saved.SFXVolume
		if !saved.BGMOn {
			bgm = 0
		}
		if !saved.SFXOn {
			sfx = 0
		}

		manager.SetBGMVolume(float64(bgm))
		manager.SetSFXVolume(float64(sfx))
	}

	g.audioManager = manager

	if config.NoBGM() {
		logger.Info("background music disabled (--no-bgm)")

		return
	}

	// The name table says which track belongs to which map. Without it every
	// location falls back to the title theme, which is still better than
	// silence.
	table, err := audio.LoadNameTable(g.assetManager)
	if err != nil {
		logger.Warn("no background music name table, every map will use the fallback track",
			zap.Error(err))
	}

	bgmDir := cfg.Audio.BGMDir
	if bgmDir == "" && len(cfg.Data.GRFPaths) > 0 {
		bgmDir = audio.DefaultBGMDir(cfg.Data.GRFPaths[0])
	}

	logger.Info("audio initialized",
		zap.String("bgmDir", bgmDir),
		zap.Int("bgmTracks", len(table)))

	g.bgm = audio.NewLocationPlayer(manager, table, bgmDir)
	g.stateManager.BGM = g.bgm
	// The / commands that touch sound reach it through here.
	g.stateManager.CommandHost = g
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

	// Escape opens the menu rather than quitting outright. Leaving used to
	// call os.Exit from here, which told the server nothing and left rAthena
	// holding the session until it timed out.
	if imgui.IsKeyPressedBoolV(imgui.KeyEscape, false) && g.uiBackend != nil {
		g.uiBackend.ToggleEscMenu()
	}

	// Handle F12 for screenshot (will capture at start of NEXT frame)
	if imgui.IsKeyChordPressed(imgui.KeyChord(imgui.KeyF12)) {
		g.screenshotRequested = true
	}

	g.checkTimedScreenshot()

	g.checkHotkeys()

	// F3 toggles the in-game debug overlay (player/camera/scene/network).
	if imgui.IsKeyPressedBoolV(imgui.KeyF3, false) {
		g.showDebug = !g.showDebug
	}

	// Ctrl+V folds the Basic Info panel to its reduced form, as in the
	// original. Read here and passed to the UI as a one-frame event: which
	// form the panel is in belongs to the panel.
	g.toggleBasicInfo = imgui.IsKeyChordPressed(imgui.KeyChord(imgui.ModCtrl | imgui.KeyV))

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

	g.runWalkTo()
	g.runMouseAt()
	g.runSay()

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

// SetWalkTo asks for one click-to-move to a cell as soon as the first map is
// up, from --walk-to. It goes through the same path a real click takes.
func (g *Game) SetWalkTo(x, y int) {
	g.walkToX, g.walkToY = x, y
	g.walkToPending = true
}

// SetMouseAt asks for the pointer to be placed at a window position once the
// first map is up, from --mouse-at. What the pointer is over decides which
// cursor is drawn, and that is the one thing an unattended capture could not
// otherwise show.
func (g *Game) SetMouseAt(x, y int) {
	g.mouseAtX, g.mouseAtY = x, y
	g.mouseAtPending = true
}

// runMouseAt moves the pointer for --mouse-at once the map is up. It goes
// through the OS, so the same motion event a hand would cause reaches the
// input layer.
func (g *Game) runMouseAt() {
	if !g.mouseAtPending {
		return
	}
	state, ok := g.stateManager.Current().(*states.InGameState)
	if !ok || !state.MapReady() {
		return
	}
	g.mouseAtPending = false

	winPos := imgui.MainViewport().Pos()
	x := int32(winPos.X) + int32(g.mouseAtX)
	y := int32(winPos.Y) + int32(g.mouseAtY)
	if err := sdl.WarpMouseGlobal(x, y); err != nil {
		logger.Warn("--mouse-at could not move the pointer", zap.Error(err))
		return
	}
	logger.Info("pointer placed for --mouse-at", zap.Int("x", g.mouseAtX), zap.Int("y", g.mouseAtY))
}

// runWalkTo fires the --walk-to click once the map can take it.
func (g *Game) runWalkTo() {
	if !g.walkToPending {
		return
	}
	state, ok := g.stateManager.Current().(*states.InGameState)
	if !ok || !state.MapReady() {
		return
	}
	g.walkToPending = false

	logger.Info("walking to the cell asked for on the command line",
		zap.Int("x", g.walkToX), zap.Int("y", g.walkToY))
	if err := state.RequestMove(g.walkToX, g.walkToY); err != nil {
		logger.Warn("--walk-to request failed", zap.Error(err))
	}
}

// SetSay queues lines to type into the chat box once in game, from --say.
func (g *Game) SetSay(lines []string) {
	g.sayQueue = append(g.sayQueue, lines...)
}

// sayInterval is the gap between one --say line and the next.
//
// Not politeness: a command's answer has to arrive before the next line goes
// out, or the trace reads as one burst with the replies interleaved and it is
// no longer clear which answer belongs to which command. A second is longer
// than any local reply takes and short enough that a sequence still fits
// inside a --screenshot-after window.
const sayInterval = 1000 * time.Millisecond

// runSay types the next --say line once the map can take it.
//
// Gated on MapReady as well as the interval, so a command that changes maps —
// @go, /mm — lets the new map finish loading before the line after it is sent
// rather than being swallowed during the load.
func (g *Game) runSay() {
	if len(g.sayQueue) == 0 {
		return
	}
	state, ok := g.stateManager.Current().(*states.InGameState)
	if !ok || !state.MapReady() {
		return
	}
	now := time.Now()
	if now.Before(g.sayNext) {
		return
	}

	line := g.sayQueue[0]
	if !g.uiBackend.QueueChatMessage(line) {
		// The line before this one has not been drained yet. Leave it in the
		// queue and try again next frame rather than overwriting it.
		return
	}

	g.sayNext = now.Add(sayInterval)
	g.sayQueue = g.sayQueue[1:]

	logger.Info("saying the line asked for on the command line",
		zap.String("line", line),
		zap.Int("remaining", len(g.sayQueue)))
}

// SetScreenshotTimers arms the unattended capture. A zero duration disables
// that timer.
func (g *Game) SetScreenshotTimers(after, every time.Duration) {
	g.shotAfter = after
	g.shotEvery = every
	g.startedAt = time.Now()
	g.shotLast = time.Now()
}

// ShowDebugOverlay opens the F3 overlay from the start, so an unattended
// screenshot can capture what it reads. F3 still toggles it afterwards.
func (g *Game) ShowDebugOverlay(show bool) {
	g.showDebug = show
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
			OnExit: func() {
				// SDLBackend.SetShouldClose is a no-op TODO upstream
				// (cimgui-go), so we terminate the process directly.
				// Deferred a frame via pendingAction so the button
				// visibly registers its pressed state first.
				g.pendingAction = func() {
					logger.Info("exit button clicked, exiting")
					os.Exit(0)
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
			ImageIndex:    state.GetLoadingImage(),
		}, viewportWidth, viewportHeight)

	case *states.InGameState:
		// While a map loads there is nothing to draw but the loading screen;
		// the HUD comes back with the map.
		if state.IsLoadingMap() {
			g.uiBackend.RenderLoadingUI(ui.LoadingUIState{
				MapName:       state.GetMapName(),
				StatusMessage: state.GetStatusMessage(),
				ErrorMessage:  state.GetErrorMessage(),
				Progress:      state.MapLoadProgress(),
				Phase:         state.MapLoadPhase(),
				ImageIndex:    state.MapLoadImage(),
			}, viewportWidth, viewportHeight)
			break
		}

		var playerX, playerY, playerZ float32
		var playerTileX, playerTileY int
		var playerDirection uint8

		if player := state.GetPlayer(); player != nil {
			playerX, playerY, playerZ = player.RenderPosition()
			playerDirection = uint8(player.Direction)
		}
		playerTileX, playerTileY = state.GetPlayerTilePosition()

		stats := state.Stats()
		dialog := state.Dialog()

		var itemLabel *states.HoverLabel
		if label, ok := state.HoverItemLabel(viewportWidth, viewportHeight); ok {
			itemLabel = &label
		}

		mapCellsX, mapCellsY := 0, 0
		if gat := state.GetGAT(); gat != nil {
			mapCellsX, mapCellsY = int(gat.Width), int(gat.Height)
		}

		uiState := ui.InGameUIState{
			MapName:         state.GetMapName(),
			MapCellsX:       mapCellsX,
			MapCellsY:       mapCellsY,
			ChatLines:       state.ChatLines(),
			EntityBars:      state.EntityBars(viewportWidth, viewportHeight),
			ItemLabel:       itemLabel,
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
			ToggleBasicInfo: g.toggleBasicInfo,
			FPS:             g.fps,
			DialogMessage:   dialog.Message,
			DialogShowNext:  dialog.Phase == states.DialogWaitingNext,
			DialogShowClose: dialog.Phase == states.DialogWaitingClose,
			OnDialogNext: func() {
				g.pendingAction = state.AdvanceDialog
			},
			OnDialogClose: func() {
				g.pendingAction = state.EndDialog
			},
			DialogPhase:   dialog.Phase.String(),
			DialogNPCID:   dialog.NPCID,
			DialogNPCName: dialog.Name,
			DialogMenu:    dialog.Menu,
			OnDialogChoose: func(choice int) {
				g.pendingAction = func() { state.ChooseMenuItem(choice) }
			},
			OnDialogCancel: func() {
				g.pendingAction = state.CancelMenuChoice
			},
			PlayerName:     ui.GetCharName(state.CharInfo()),
			PlayerClass:    stats.Class,
			PlayerHP:       stats.HP,
			PlayerMaxHP:    stats.MaxHP,
			PlayerSP:       stats.SP,
			PlayerMaxSP:    stats.MaxSP,
			PlayerLevel:    stats.BaseLevel,
			PlayerJobLevel: stats.JobLevel,

			PlayerBaseExp:     stats.BaseExp,
			PlayerNextBaseExp: stats.NextBaseExp,
			PlayerJobExp:      stats.JobExp,
			PlayerNextJobExp:  stats.NextJobExp,
			PlayerZeny:        stats.Zeny,
			PlayerWeight:      stats.Weight,
			PlayerMaxWeight:   stats.MaxWeight,

			MapMarkers: state.MapMarkers(),

			Skills:    state.Skills(),
			Inventory: state.Inventory(),

			PrimaryStats: stats.Primary,
			PrimaryBonus: stats.PrimaryBonus,
			PrimaryCost:  stats.PrimaryCost,
			StatusPoints: stats.StatusPoints,
			SkillPoints:  stats.SkillPoints,

			Atk: stats.Atk, AtkBonus: stats.AtkBonus,
			MatkMin: stats.MatkMin, MatkMax: stats.MatkMax,
			Def: stats.Def, DefBonus: stats.DefBonus,
			Mdef: stats.Mdef, MdefBonus: stats.MdefBonus,
			Flee: stats.Flee, FleeBonus: stats.FleeBonus,
			Hit: stats.Hit, Critical: stats.Critical, Aspd: stats.Aspd,
		}
		populateDebugFields(&uiState, state, g.client)
		g.uiBackend.RenderInGameUI(uiState, g.dt, viewportWidth, viewportHeight)

		g.applySoundSettings()

		// A double click in the inventory goes out here, for the same reason
		// the chat line does: the interface has no connection of its own.
		if action, ok := g.uiBackend.TakeItemAction(); ok {
			var err error
			if action.Equip {
				err = state.EquipItem(action.Index)
			} else {
				err = state.UseItem(action.Index)
			}
			if err != nil {
				logger.Warn("could not act on item", zap.Error(err))
			}
		}

		// An item dragged out of the inventory window and let go over the
		// world, which is how the original drops one.
		if drop, ok := g.uiBackend.TakeDropAction(); ok {
			if err := state.DropItem(drop.Index, drop.Amount); err != nil {
				logger.Warn("could not drop item", zap.Error(err))
			}
		}

		// What the ESC menu was asked for goes out here for the same reason
		// the chat line below does: the interface has no connection.
		//
		// Neither request ends anything by itself. The server answers both,
		// and can refuse both, so the state acts on the answer.
		switch g.uiBackend.TakeEscAction() {
		case ui.EscCharSelect:
			if err := state.RequestCharSelect(); err != nil {
				logger.Warn("could not ask for character select", zap.Error(err))
			}
		case ui.EscQuit:
			state.SetQuitFunc(func() {
				g.pendingAction = func() {
					logger.Info("server released the session, exiting")
					os.Exit(0)
				}
			})

			if err := state.RequestQuit(); err != nil {
				logger.Warn("could not ask to quit", zap.Error(err))
			}
		case ui.EscNone:
		}

		// A line the player typed goes out here rather than from the widget:
		// the interface has no client to send with, and whether a line is
		// even sent depends on what kind of command it turns out to be.
		if to, msg := g.uiBackend.TakeChatMessage(); msg != "" {
			if err := state.SubmitLine(to, msg); err != nil {
				logger.Warn("could not send chat", zap.Error(err))
			}
		}

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

	// A capture asked for just before exit is still worth having.
	g.screenshots.flush(5 * time.Second)

	if g.uiBackend != nil {
		g.uiBackend.Close()
	}

	if g.client != nil {
		g.client.Disconnect()
	}

	if g.audioManager != nil {
		g.audioManager.Close()
	}

	if g.assetManager != nil {
		g.assetManager.Close()
	}
}

// captureScreenshot reads the current frame back and hands it to the
// screenshot writer. Only the readback and the flip happen here; the PNG
// encoding is the expensive part and runs off the render thread.
func (g *Game) captureScreenshot() {
	// Get actual viewport size from OpenGL (handles HiDPI correctly)
	var viewport [4]int32
	gl.GetIntegerv(gl.VIEWPORT, &viewport[0])
	width := int(viewport[2])
	height := int(viewport[3])

	if width <= 0 || height <= 0 {
		logger.Warn("screenshot failed: invalid viewport")
		return
	}

	pixels := make([]byte, width*height*4)
	gl.ReadPixels(0, 0, int32(width), int32(height), gl.RGBA, gl.UNSIGNED_BYTE, gl.Ptr(pixels))

	// Flip vertically: OpenGL reads the bottom row first.
	rowSize := width * 4
	flipped := make([]byte, len(pixels))
	for y := 0; y < height; y++ {
		srcRow := (height - 1 - y) * rowSize
		dstRow := y * rowSize
		copy(flipped[dstRow:dstRow+rowSize], pixels[srcRow:srcRow+rowSize])
	}

	name := screenshotName(time.Now())
	if !g.screenshots.enqueue(screenshotJob{pixels: flipped, width: width, height: height, dir: g.screenshotDir, name: name}) {
		return
	}

	g.screenshotMsg = fmt.Sprintf("Saved: %s", name)
	g.screenshotMsgTime = time.Now()
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
	//
	// The interface gets the wheel first. A dialog long enough to scroll is
	// exactly when the player reaches for it, and zooming the camera instead
	// makes the scrollbar look broken.
	scroll := io.MouseWheel()
	if scroll != 0 && !io.WantCaptureMouse() && !g.uiBackend.MouseCaptured() {
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

	// No keyboard walking. RO moves by mouse, and the letters belong to chat:
	// with WASD bound to the character, typing "was" into the chat box walked
	// you across the map. The state's StepToward went with it.

	// The pointer changes over an NPC, the way the original tells you a thing
	// can be talked to before you click it. Runs every frame, so it uses the
	// pick that does not trace.
	g.updateCursor(state, io, mouseX, mouseY)

	// Left click for click-to-move. Skip if any imgui window (HUD, minimap,
	// chat, etc) is consuming the click; otherwise ray-cast to ground plane
	// and dispatch a server move request.
	// A click on the interface is not a click on the world behind it. ImGui
	// answers for its own windows; the 2D UI has to be asked separately,
	// since the HUD is not an ImGui window and ImGui does not know it is
	// there.
	if imgui.IsMouseClickedBool(imgui.MouseButtonLeft) && !io.WantCaptureMouse() &&
		!g.uiBackend.MouseCaptured() {
		viewportW, viewportH := g.uiBackend.GetScreenSize()

		trace.Emit(trace.Pick, "click",
			zap.Float32("rawX", rawMouse.X), zap.Float32("rawY", rawMouse.Y),
			zap.Float32("windowX", winPos.X), zap.Float32("windowY", winPos.Y),
			zap.Float32("mouseX", mouseX), zap.Float32("mouseY", mouseY),
			zap.Float32("viewportW", viewportW), zap.Float32("viewportH", viewportH))

		state.ClickWorld(mouseX, mouseY, viewportW, viewportH)
	}

	// The marker follows the cursor whether or not anything is clicked, so
	// the cell you are about to walk to is shown before you commit to it.
	if !io.WantCaptureMouse() && !g.uiBackend.MouseCaptured() {
		viewportW, viewportH := g.uiBackend.GetScreenSize()
		tileX, tileY, ok := state.ScreenToTile(mouseX, mouseY, viewportW, viewportH)
		state.SetHoverCell(tileX, tileY, ok)
	} else {
		// Over the interface: no cell is being pointed at.
		state.SetHoverCell(0, 0, false)
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

	g.attachUISounds(backend)
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

// updateCursor picks the cursor for whatever the pointer is over.
//
// The original signals what a click would do before you click: talk, for an
// NPC, is the only way to tell scenery from something with a script behind
// it. Which cursor each kind of unit gets is cursorFor's decision.
func (g *Game) updateCursor(state *states.InGameState, io *imgui.IO, mouseX, mouseY float32) {
	if g.uiBackend == nil {
		return
	}

	want := cursor.StateDefault

	hudCursor, hudAsked := g.uiBackend.WantCursor()

	switch {
	// Rotating outranks everything. The camera is swinging, so whatever the
	// pointer was over a moment ago is not where it is now, and the cursor
	// should say what the drag is doing rather than what it started on.
	//
	// Dragging rather than merely holding the button: a right click that does
	// not move is not a rotation, and showing the rotate cursor for it would
	// promise a camera swing that never comes.
	case imgui.IsMouseDragging(imgui.MouseButtonRight):
		want = cursor.StateRotate
		state.SetHoverEntity(nil)

	// An interface element under the pointer has first say: a resize corner
	// asking for the hand is the only thing that knows it is one.
	case hudAsked:
		want = hudCursor
		state.SetHoverEntity(nil)

	// Otherwise the pointer belongs to whatever is under it in the world.
	case !io.WantCaptureMouse() && !g.uiBackend.MouseCaptured():
		viewportW, viewportH := g.uiBackend.GetScreenSize()
		hovered := state.HoverEntity(mouseX, mouseY, viewportW, viewportH)
		state.SetHoverEntity(hovered)
		want = cursorFor(hovered)

	default:
		state.SetHoverEntity(nil)
	}

	g.uiBackend.SetCursorState(want)
}

// ToggleBGM turns background music on or off for /bgm, reporting the new
// state.
//
// Goes through the same sound settings the options dialog writes, so the two
// stay in agreement and the choice is persisted — a command that turned the
// music off behind the dialog's back would leave the checkbox lying.
func (g *Game) ToggleBGM() bool {
	return g.toggleChannel(true)
}

// ToggleSFX does the same for sound effects.
func (g *Game) ToggleSFX() bool {
	return g.toggleChannel(false)
}

// toggleChannel flips one audio channel and saves it. Reports the new state.
//
// A channel that is off has volume zero, which is also how the dialog stores
// it, so "on" means a level above zero. Turning one back on restores the level
// the settings file remembers rather than a guess — except when that level is
// itself zero, which would turn the channel "on" inaudibly.
func (g *Game) toggleChannel(bgm bool) bool {
	if g.audioManager == nil {
		return false
	}

	const defaultLevel = 0.5

	current := g.audioManager.GetSFXVolume()
	if bgm {
		current = g.audioManager.GetBGMVolume()
	}

	on := current <= 0
	level := 0.0
	if on {
		level = defaultLevel

		saved := config.LoadUIState()
		remembered := saved.SFXVolume
		if bgm {
			remembered = saved.BGMVolume
		}
		if remembered > 0 {
			level = float64(remembered)
		}
	}

	if bgm {
		g.audioManager.SetBGMVolume(level)
	} else {
		g.audioManager.SetSFXVolume(level)
	}

	err := config.UpdateUIState(func(state *config.UIState) {
		state.SoundSet = true
		if bgm {
			state.BGMOn = on
			if on {
				state.BGMVolume = float32(level)
			}

			return
		}
		state.SFXOn = on
		if on {
			state.SFXVolume = float32(level)
		}
	})
	if err != nil {
		logger.Warn("could not save the sound setting", zap.Error(err))
	}

	return on
}

// applySoundSettings seeds the sound dialog from the audio manager and puts
// back whatever the player changed.
//
// A channel switched off is played at zero rather than having its level
// zeroed, so the slider keeps its position and switching back on returns to
// the level that was there.
func (g *Game) applySoundSettings() {
	if g.audioManager == nil || g.uiBackend == nil {
		return
	}

	bgm := float32(g.audioManager.GetBGMVolume())
	sfx := float32(g.audioManager.GetSFXVolume())

	g.uiBackend.SetSoundSettings(ui.SoundSettings{
		BGMVolume: bgm,
		SFXVolume: sfx,
		BGMOn:     bgm > 0,
		SFXOn:     sfx > 0,
	})

	settings, changed := g.uiBackend.TakeSoundSettings()
	if !changed {
		return
	}

	bgmVol, sfxVol := settings.BGMVolume, settings.SFXVolume
	if !settings.BGMOn {
		bgmVol = 0
	}
	if !settings.SFXOn {
		sfxVol = 0
	}

	g.audioManager.SetBGMVolume(float64(bgmVol))
	g.audioManager.SetSFXVolume(float64(sfxVol))

	err := config.UpdateUIState(func(state *config.UIState) {
		state.SoundSet = true
		state.BGMVolume, state.SFXVolume = settings.BGMVolume, settings.SFXVolume
		state.BGMOn, state.SFXOn = settings.BGMOn, settings.SFXOn
	})
	if err != nil {
		logger.Warn("could not save sound settings", zap.Error(err))
	}
}

// cursorFor is the cursor the original shows over a unit of each kind, or
// the default over nothing.
func cursorFor(e *entity.Entity) cursor.State {
	if e == nil {
		return cursor.StateDefault
	}
	switch e.Type {
	case entity.TypeNPC:
		return cursor.StateTalk
	case entity.TypeWarp:
		return cursor.StateWarp
	case entity.TypeItem:
		return cursor.StatePick
	default:
		return cursor.StateDefault
	}
}

// hotkeyRowKeys is which key opens each row of the quick panel.
//
// The function keys take the top row, the plain numbers the second, and the
// two below them the same numbers held with Ctrl and with Alt.
type hotkeyRowKeys struct {
	row int

	// fn selects F1..F9 rather than 1..9.
	fn bool

	// ctrl and alt are the modifiers that must be held, and the ones that
	// must not: a bare 1 and Alt+1 are different rows, so the plain row has
	// to check that neither is down.
	ctrl bool
	alt  bool
}

var hotkeyRows = []hotkeyRowKeys{
	{row: 0, fn: true},
	{row: 1},
	{row: 2, ctrl: true},
	{row: 3, alt: true},
}

// checkHotkeys uses the quick panel from the keyboard.
//
// Nothing fires while a field has focus. The chat box takes the number keys
// as text, and a potion drunk in the middle of typing a message would be the
// interface acting on something that was never meant for it.
func (g *Game) checkHotkeys() {
	if g.uiBackend == nil || g.uiBackend.TextEntryFocused() {
		return
	}

	io := imgui.CurrentIO()
	ctrl, alt := io.KeyCtrl(), io.KeyAlt()

	for _, binding := range hotkeyRows {
		if binding.ctrl != ctrl || binding.alt != alt {
			continue
		}

		for col := 0; col < 9; col++ {
			key := imgui.Key1 + imgui.Key(col)
			if binding.fn {
				key = imgui.KeyF1 + imgui.Key(col)
			}

			if imgui.IsKeyPressedBoolV(key, false) {
				g.uiBackend.PressHotkey(binding.row, col)
			}
		}
	}
}
