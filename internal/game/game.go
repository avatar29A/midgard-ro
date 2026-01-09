// Package game implements the main game loop and state management.
package game

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/Faultbox/midgard-ro/internal/engine/input"
	"github.com/Faultbox/midgard-ro/internal/engine/renderer"
	"github.com/Faultbox/midgard-ro/internal/engine/window"
)

// Config holds game configuration.
type Config struct {
	Title      string
	Width      int
	Height     int
	Fullscreen bool
}

// Game is the main game instance.
type Game struct {
	config   Config
	running  bool
	window   *window.Window
	renderer *renderer.Renderer
	input    *input.Input
}

// New creates a new game instance.
func New(cfg Config) (*Game, error) {
	slog.Info("initializing game",
		"title", cfg.Title,
		"width", cfg.Width,
		"height", cfg.Height,
	)

	g := &Game{
		config:  cfg,
		running: false,
	}

	// Create window (this also creates OpenGL context)
	var err error
	g.window, err = window.New(window.Config{
		Title:      cfg.Title,
		Width:      cfg.Width,
		Height:     cfg.Height,
		Fullscreen: cfg.Fullscreen,
		VSync:      true, // Enable VSync by default
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create window: %w", err)
	}

	// Create renderer (AFTER window, since OpenGL context must exist)
	g.renderer, err = renderer.New(renderer.Config{
		Width:  cfg.Width,
		Height: cfg.Height,
		VSync:  true,
	})
	if err != nil {
		g.window.Close()
		return nil, fmt.Errorf("failed to create renderer: %w", err)
	}

	// Create input handler
	g.input = input.New()

	slog.Info("game initialized successfully")
	return g, nil
}

// Run starts the main game loop.
func (g *Game) Run() error {
	g.running = true

	// Timing
	lastTime := time.Now()
	frameCount := 0
	fpsTimer := time.Now()

	slog.Info("starting game loop")

	for g.running {
		// Calculate delta time
		now := time.Now()
		dt := now.Sub(lastTime).Seconds()
		lastTime = now

		// 1. Process input
		if g.input.Update() {
			// Quit event received
			g.running = false
			break
		}

		// Handle events
		for _, event := range g.input.Events() {
			switch event.Type {
			case input.EventWindowResize:
				g.renderer.Resize(event.Width, event.Height)
			case input.EventKeyDown:
				// ESC to quit
				if event.Key == 41 { // SDL_SCANCODE_ESCAPE
					g.running = false
				}
			}
		}

		// 2. Update game state
		if err := g.update(dt); err != nil {
			return fmt.Errorf("update error: %w", err)
		}

		// 3. Render
		if err := g.render(); err != nil {
			return fmt.Errorf("render error: %w", err)
		}

		// 4. Present (swap buffers)
		g.window.SwapBuffers()

		// FPS counter
		frameCount++
		if time.Since(fpsTimer) >= time.Second {
			slog.Debug("fps", "count", frameCount, "dt", fmt.Sprintf("%.2fms", dt*1000))
			frameCount = 0
			fpsTimer = time.Now()
		}
	}

	return nil
}

// Close cleans up game resources.
func (g *Game) Close() {
	slog.Info("closing game")

	if g.renderer != nil {
		g.renderer.Close()
	}
	if g.window != nil {
		g.window.Close()
	}
}

// update updates game state.
func (g *Game) update(dt float64) error {
	// TODO: Update current game state
	// TODO: Update entities
	// TODO: Update UI
	_ = dt // suppress unused warning
	return nil
}

// render draws the current frame.
func (g *Game) render() error {
	// Begin frame
	g.renderer.Begin()

	// Draw test triangle
	g.renderer.DrawTriangle()

	// End frame
	g.renderer.End()

	return nil
}
