// Package main is the entry point for the Midgard RO client.
package main

import (
	"log/slog"
	"os"

	"github.com/Faultbox/midgard-ro/internal/game"
)

func main() {
	// Setup structured logging
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	slog.SetDefault(logger)

	slog.Info("=== Midgard RO Client ===")
	slog.Info("Milestone 1: Window & Triangle")

	// Create and run game
	g, err := game.New(game.Config{
		Title:      "Midgard RO - Milestone 1",
		Width:      1280,
		Height:     720,
		Fullscreen: false,
	})
	if err != nil {
		slog.Error("failed to create game", "error", err)
		os.Exit(1)
	}
	defer g.Close()

	// Run the game loop
	if err := g.Run(); err != nil {
		slog.Error("game error", "error", err)
		os.Exit(1)
	}

	slog.Info("game closed normally")
}
