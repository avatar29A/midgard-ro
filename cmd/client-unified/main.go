// Package main is the unified game client using custom ui2d rendering.
// This replaces ImGui to avoid viewport/window separation issues while
// integrating with the full game architecture (states, network, entities).
package main

import (
	"fmt"
	"os"
	"runtime"

	"github.com/go-gl/gl/v4.1-core/gl"
	"github.com/veandco/go-sdl2/sdl"
	"go.uber.org/zap"
	_ "golang.org/x/image/bmp" // BMP decoder registration

	"github.com/Faultbox/midgard-ro/internal/config"
	"github.com/Faultbox/midgard-ro/internal/engine/ui2d"
	"github.com/Faultbox/midgard-ro/internal/game"
	"github.com/Faultbox/midgard-ro/internal/game/states"
	"github.com/Faultbox/midgard-ro/internal/game/ui"
	"github.com/Faultbox/midgard-ro/internal/logger"
)

const (
	defaultWidth  = 1280
	defaultHeight = 720
	windowTitle   = "Midgard RO"
)

func init() {
	runtime.LockOSThread()
}

func main() {
	// Parse CLI flags
	config.ParseFlags()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Config error: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	if err := logger.Init(cfg.Logging.Level, cfg.Logging.LogFile); err != nil {
		fmt.Fprintf(os.Stderr, "Logger error: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	logger.Info("=== Midgard RO Client (Unified) ===")

	// Determine window size
	width := cfg.Graphics.Width
	height := cfg.Graphics.Height
	if width <= 0 {
		width = defaultWidth
	}
	if height <= 0 {
		height = defaultHeight
	}

	// Initialize SDL2
	if err := sdl.Init(sdl.INIT_VIDEO | sdl.INIT_EVENTS); err != nil {
		logger.Error("SDL init failed", zap.Error(err))
		os.Exit(1)
	}
	defer sdl.Quit()

	// Set OpenGL attributes
	sdl.GLSetAttribute(sdl.GL_CONTEXT_MAJOR_VERSION, 4)
	sdl.GLSetAttribute(sdl.GL_CONTEXT_MINOR_VERSION, 1)
	sdl.GLSetAttribute(sdl.GL_CONTEXT_PROFILE_MASK, sdl.GL_CONTEXT_PROFILE_CORE)
	sdl.GLSetAttribute(sdl.GL_DOUBLEBUFFER, 1)
	sdl.GLSetAttribute(sdl.GL_DEPTH_SIZE, 24)

	// Create window with HiDPI support
	window, err := sdl.CreateWindow(
		windowTitle,
		sdl.WINDOWPOS_CENTERED, sdl.WINDOWPOS_CENTERED,
		int32(width), int32(height),
		sdl.WINDOW_OPENGL|sdl.WINDOW_SHOWN|sdl.WINDOW_RESIZABLE|sdl.WINDOW_ALLOW_HIGHDPI,
	)
	if err != nil {
		logger.Error("Window creation failed", zap.Error(err))
		os.Exit(1)
	}
	defer window.Destroy()

	// Create OpenGL context
	glContext, err := window.GLCreateContext()
	if err != nil {
		logger.Error("OpenGL context creation failed", zap.Error(err))
		os.Exit(1)
	}
	defer sdl.GLDeleteContext(glContext)

	// Initialize OpenGL
	if err := gl.Init(); err != nil {
		logger.Error("OpenGL init failed", zap.Error(err))
		os.Exit(1)
	}

	version := gl.GoStr(gl.GetString(gl.VERSION))
	renderer := gl.GoStr(gl.GetString(gl.RENDERER))
	logger.Info("OpenGL initialized",
		zap.String("version", version),
		zap.String("renderer", renderer),
	)

	// Enable VSync
	sdl.GLSetSwapInterval(1)

	// Set initial viewport using actual drawable size (for HiDPI/Retina displays)
	drawableW, drawableH := window.GLGetDrawableSize()
	if drawableW != int32(width) || drawableH != int32(height) {
		logger.Info("HiDPI detected",
			zap.Int32("window", int32(width)),
			zap.Int32("drawable", drawableW),
			zap.Float32("scale", float32(drawableW)/float32(width)),
		)
	}
	gl.Viewport(0, 0, drawableW, drawableH)

	// Create game instance (headless - no ImGui window)
	g, err := game.NewHeadless(cfg)
	if err != nil {
		logger.Error("failed to create game", zap.Error(err))
		os.Exit(1)
	}
	defer g.Close()

	// Replace the UI backend with ui2d
	ui2dBackend, err := ui.NewUI2DBackend(width, height)
	if err != nil {
		logger.Error("failed to create ui2d backend", zap.Error(err))
		os.Exit(1)
	}
	g.SetUIBackend(ui2dBackend)

	logger.Info("UI2D backend initialized")

	// Initialize timing
	g.InitTiming()

	// Input state tracking
	var rightMouseDown bool
	var lastMouseX float32

	// Main loop
	running := true
	for running {
		// Handle SDL events
		for event := sdl.PollEvent(); event != nil; event = sdl.PollEvent() {
			switch e := event.(type) {
			case *sdl.QuitEvent:
				running = false

			case *sdl.WindowEvent:
				if e.Event == sdl.WINDOWEVENT_RESIZED || e.Event == sdl.WINDOWEVENT_SIZE_CHANGED {
					// Use drawable size for scene (actual pixels)
					dw, dh := window.GLGetDrawableSize()
					gl.Viewport(0, 0, dw, dh)
					// UI uses window size (points) for layout
					ww, wh := window.GetSize()
					ui2dBackend.Resize(int(ww), int(wh))
					// Scene framebuffer needs actual pixel size
					if inGameState, ok := g.StateManager().Current().(*states.InGameState); ok {
						inGameState.ResizeScene(dw, dh)
					}
				}

			case *sdl.MouseMotionEvent:
				input := ui2dBackend.Input()
				input.MouseX = float32(e.X)
				input.MouseY = float32(e.Y)

				// Camera rotation with right mouse button
				if rightMouseDown {
					deltaX := float32(e.X) - lastMouseX
					g.HandleInGameCameraInput(0, deltaX, true)
				}
				lastMouseX = float32(e.X)

			case *sdl.MouseButtonEvent:
				input := ui2dBackend.Input()
				pressed := e.State == sdl.PRESSED
				switch e.Button {
				case sdl.BUTTON_LEFT:
					input.MouseLeftDown = pressed
					if pressed {
						input.MouseLeftClicked = true // Event-based click detection
					}
				case sdl.BUTTON_RIGHT:
					input.MouseRightDown = pressed
					rightMouseDown = pressed
					if pressed {
						input.MouseRightClicked = true
					}
				case sdl.BUTTON_MIDDLE:
					input.MouseMiddleDown = pressed
				}

			case *sdl.MouseWheelEvent:
				input := ui2dBackend.Input()
				input.ScrollX = float32(e.X)
				input.ScrollY = float32(e.Y)
				// Camera zoom
				g.HandleInGameCameraInput(float32(e.Y), 0, false)

			case *sdl.TextInputEvent:
				input := ui2dBackend.Input()
				input.TextInput += e.GetText()

			case *sdl.KeyboardEvent:
				handleKeyEvent(e, ui2dBackend.Input(), &running, g)
			}
		}

		// Clear screen
		gl.ClearColor(0.1, 0.1, 0.15, 1.0)
		gl.Clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT)

		// Update game state
		if err := g.Update(); err != nil {
			logger.Error("game update error", zap.Error(err))
		}

		// Ensure scene framebuffer matches drawable size when in-game
		if inGameState, ok := g.StateManager().Current().(*states.InGameState); ok {
			dw, dh := window.GLGetDrawableSize()
			inGameState.ResizeScene(dw, dh)
		}

		// Render UI
		g.RenderUI()

		// Process screenshot if requested
		g.ProcessScreenshot()

		// Swap buffers
		window.GLSwap()
	}

	logger.Info("game closed normally")
}

func handleKeyEvent(e *sdl.KeyboardEvent, input *ui2d.InputState, running *bool, g *game.Game) {
	pressed := e.State == sdl.PRESSED
	mod := sdl.GetModState()
	ctrl := mod&sdl.KMOD_CTRL != 0

	switch e.Keysym.Sym {
	case sdl.K_ESCAPE:
		input.KeyEscape = pressed
		if pressed {
			*running = false
		}
	case sdl.K_BACKSPACE:
		input.KeyBackspace = pressed
	case sdl.K_DELETE:
		input.KeyDelete = pressed
	case sdl.K_RETURN, sdl.K_KP_ENTER:
		input.KeyEnter = pressed
	case sdl.K_TAB:
		input.KeyTab = pressed

	// Arrow keys
	case sdl.K_LEFT:
		input.KeyLeft = pressed
	case sdl.K_RIGHT:
		input.KeyRight = pressed
	case sdl.K_UP:
		input.KeyUp = pressed
	case sdl.K_DOWN:
		input.KeyDown = pressed

	// Function keys
	case sdl.K_F12:
		if pressed {
			g.HandleScreenshot()
		}

	// Ctrl shortcuts
	case sdl.K_a:
		if ctrl && pressed {
			input.KeySelectAll = true
		}
	case sdl.K_c:
		if ctrl && pressed {
			input.KeyCopy = true
		}
	case sdl.K_v:
		if ctrl && pressed {
			input.KeyPaste = true
		}
	case sdl.K_x:
		if ctrl && pressed {
			input.KeyCut = true
		}
	case sdl.K_z:
		if ctrl && pressed {
			input.KeyUndo = true
		}
	}

	input.KeyCtrl = mod&sdl.KMOD_CTRL != 0
	input.KeyShift = mod&sdl.KMOD_SHIFT != 0
	input.KeyAlt = mod&sdl.KMOD_ALT != 0
}
