// Package main is the entry point for the Midgard RO client.
package main

import (
	"fmt"
	_ "image/jpeg" // JPEG decoder registration
	_ "image/png"  // PNG decoder registration
	"os"

	"go.uber.org/zap"
	_ "golang.org/x/image/bmp" // BMP decoder registration

	"github.com/Faultbox/midgard-ro/internal/config"
	"github.com/Faultbox/midgard-ro/internal/game"
	"github.com/Faultbox/midgard-ro/internal/logger"
	"github.com/Faultbox/midgard-ro/internal/trace"
)

func main() {
	// Parse CLI flags first
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

	// Opt-in tracing (--trace=move,pick,net or --trace=all). Off by default.
	trace.Enable(config.TraceSpec())

	logger.Info("=== Midgard RO Client ===")
	logger.Sugar.Debugf("Config: %+v", cfg)

	// Create and run game
	g, err := game.New(cfg)
	if err != nil {
		logger.Error("failed to create game", zap.Error(err))
		os.Exit(1)
	}
	defer g.Close()

	// Unattended screenshot capture, for inspecting the UI without someone at
	// the keyboard.
	g.SetScreenshotTimers(config.ScreenshotAfter(), config.ScreenshotEvery())
	g.ShowDebugOverlay(config.DebugOverlay())
	if x, y, ok := config.WalkTo(); ok {
		g.SetWalkTo(x, y)
	} else if spec := config.WalkToSpec(); spec != "" {
		logger.Warn("--walk-to not understood, want x,y", zap.String("value", spec))
	}
	if x, y, ok := config.MouseAt(); ok {
		g.SetMouseAt(x, y)
	} else if spec := config.MouseAtSpec(); spec != "" {
		logger.Warn("--mouse-at not understood, want x,y", zap.String("value", spec))
	}
	if lines := config.Say(); len(lines) > 0 {
		g.SetSay(lines)
	}
	if windows := config.OpenWindows(); len(windows) > 0 {
		g.SetOpenWindows(windows)
	}
	if config.AttackNearest() {
		g.SetAttackNearest(true)
	}

	if slots := config.EquipSlots(); len(slots) > 0 {
		g.SetEquipSlots(slots)
	}

	// Run the game loop
	if err := g.Run(); err != nil {
		logger.Error("game error", zap.Error(err))
		os.Exit(1)
	}

	logger.Info("game closed normally")
}
