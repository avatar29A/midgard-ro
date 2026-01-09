// Package main is the entry point for the Midgard RO client.
package main

import (
	"fmt"
	"os"

	"go.uber.org/zap"

	"github.com/Faultbox/midgard-ro/internal/config"
	"github.com/Faultbox/midgard-ro/internal/game"
	"github.com/Faultbox/midgard-ro/internal/logger"
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

	logger.Info("=== Midgard RO Client ===")
	logger.Sugar.Debugf("Config: %+v", cfg)

	// Create and run game
	g, err := game.New(cfg)
	if err != nil {
		logger.Error("failed to create game", zap.Error(err))
		os.Exit(1)
	}
	defer g.Close()

	// Run the game loop
	if err := g.Run(); err != nil {
		logger.Error("game error", zap.Error(err))
		os.Exit(1)
	}

	logger.Info("game closed normally")
}
