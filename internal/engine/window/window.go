// Package window handles SDL2 window and OpenGL context creation.
package window

import (
	"fmt"
	"log/slog"
	"runtime"

	"github.com/veandco/go-sdl2/sdl"
)

func init() {
	// OpenGL calls must be made from the main thread
	runtime.LockOSThread()
}

// Config holds window configuration.
type Config struct {
	Title      string
	Width      int
	Height     int
	Fullscreen bool
	VSync      bool
}

// Window wraps SDL2 window and OpenGL context.
type Window struct {
	config    Config
	sdlWindow *sdl.Window
	glContext sdl.GLContext
}

// New creates a new window with OpenGL context.
func New(cfg Config) (*Window, error) {
	w := &Window{
		config: cfg,
	}

	// Initialize SDL2
	slog.Info("initializing SDL2")
	if err := sdl.Init(sdl.INIT_VIDEO | sdl.INIT_EVENTS); err != nil {
		return nil, fmt.Errorf("SDL_Init failed: %w", err)
	}

	// Set OpenGL attributes BEFORE creating window
	// We want OpenGL 4.1 Core Profile (max supported on macOS)
	sdl.GLSetAttribute(sdl.GL_CONTEXT_MAJOR_VERSION, 4)
	sdl.GLSetAttribute(sdl.GL_CONTEXT_MINOR_VERSION, 1)
	sdl.GLSetAttribute(sdl.GL_CONTEXT_PROFILE_MASK, sdl.GL_CONTEXT_PROFILE_CORE)

	// Double buffering
	sdl.GLSetAttribute(sdl.GL_DOUBLEBUFFER, 1)

	// Depth buffer
	sdl.GLSetAttribute(sdl.GL_DEPTH_SIZE, 24)

	// Create window with OpenGL flag
	flags := uint32(sdl.WINDOW_OPENGL | sdl.WINDOW_RESIZABLE)
	if cfg.Fullscreen {
		flags |= sdl.WINDOW_FULLSCREEN
	}

	var err error
	w.sdlWindow, err = sdl.CreateWindow(
		cfg.Title,
		sdl.WINDOWPOS_CENTERED,
		sdl.WINDOWPOS_CENTERED,
		int32(cfg.Width),
		int32(cfg.Height),
		flags,
	)
	if err != nil {
		sdl.Quit()
		return nil, fmt.Errorf("SDL_CreateWindow failed: %w", err)
	}

	// Create OpenGL context
	w.glContext, err = w.sdlWindow.GLCreateContext()
	if err != nil {
		w.sdlWindow.Destroy()
		sdl.Quit()
		return nil, fmt.Errorf("SDL_GL_CreateContext failed: %w", err)
	}

	// Enable VSync
	if cfg.VSync {
		if err := sdl.GLSetSwapInterval(1); err != nil {
			slog.Warn("failed to enable VSync", "error", err)
		}
	} else {
		sdl.GLSetSwapInterval(0)
	}

	slog.Info("window created",
		"title", cfg.Title,
		"width", cfg.Width,
		"height", cfg.Height,
		"fullscreen", cfg.Fullscreen,
		"vsync", cfg.VSync,
	)

	return w, nil
}

// Close destroys the window and cleans up SDL2.
func (w *Window) Close() {
	slog.Info("closing window")

	if w.glContext != nil {
		sdl.GLDeleteContext(w.glContext)
	}
	if w.sdlWindow != nil {
		w.sdlWindow.Destroy()
	}

	sdl.Quit()
}

// SwapBuffers swaps the OpenGL buffers.
func (w *Window) SwapBuffers() {
	w.sdlWindow.GLSwap()
}

// GetSize returns the current window size.
func (w *Window) GetSize() (int, int) {
	width, height := w.sdlWindow.GetSize()
	return int(width), int(height)
}

// SetTitle sets the window title.
func (w *Window) SetTitle(title string) {
	w.sdlWindow.SetTitle(title)
}
