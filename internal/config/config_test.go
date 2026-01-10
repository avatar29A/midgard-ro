package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefault(t *testing.T) {
	cfg := Default()

	// Test graphics defaults
	if cfg.Graphics.Width != 1280 {
		t.Errorf("expected width 1280, got %d", cfg.Graphics.Width)
	}
	if cfg.Graphics.Height != 720 {
		t.Errorf("expected height 720, got %d", cfg.Graphics.Height)
	}
	if cfg.Graphics.Fullscreen {
		t.Error("expected fullscreen to be false by default")
	}
	if !cfg.Graphics.VSync {
		t.Error("expected vsync to be true by default")
	}

	// Test audio defaults
	if cfg.Audio.MasterVolume != 0.8 {
		t.Errorf("expected master volume 0.8, got %f", cfg.Audio.MasterVolume)
	}
	if cfg.Audio.MusicVolume != 0.7 {
		t.Errorf("expected music volume 0.7, got %f", cfg.Audio.MusicVolume)
	}

	// Test network defaults
	if cfg.Network.LoginServer != "127.0.0.1:6900" {
		t.Errorf("expected login server 127.0.0.1:6900, got %s", cfg.Network.LoginServer)
	}
	if cfg.Network.ConnectTimeout != 10*time.Second {
		t.Errorf("expected timeout 10s, got %v", cfg.Network.ConnectTimeout)
	}

	// Test game defaults
	if cfg.Game.Language != "en" {
		t.Errorf("expected language 'en', got %s", cfg.Game.Language)
	}
	if cfg.Game.ShowFPS {
		t.Error("expected show_fps to be false by default")
	}

	// Test logging defaults
	if cfg.Logging.Level != "info" {
		t.Errorf("expected log level 'info', got %s", cfg.Logging.Level)
	}
	if cfg.Logging.LogFile != "" {
		t.Errorf("expected empty log file, got %s", cfg.Logging.LogFile)
	}
}

func TestLoadFromFile(t *testing.T) {
	// Create temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	yamlContent := `
graphics:
  width: 1920
  height: 1080
  fullscreen: true
  vsync: false
  fps_limit: 144

audio:
  master_volume: 0.5
  music_volume: 0.6
  sfx_volume: 0.7
  muted: true

network:
  login_server: "game.server.com:6900"
  connect_timeout: 5s

game:
  language: "ja"
  show_fps: true
  show_ping: true

logging:
  level: "debug"
  log_file: "game.log"
`

	if err := os.WriteFile(configPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	// Load config
	cfg := Default()
	if err := loadFromFile(cfg, configPath); err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Verify values were loaded
	if cfg.Graphics.Width != 1920 {
		t.Errorf("expected width 1920, got %d", cfg.Graphics.Width)
	}
	if cfg.Graphics.Height != 1080 {
		t.Errorf("expected height 1080, got %d", cfg.Graphics.Height)
	}
	if !cfg.Graphics.Fullscreen {
		t.Error("expected fullscreen to be true")
	}
	if cfg.Graphics.VSync {
		t.Error("expected vsync to be false")
	}
	if cfg.Graphics.FPSLimit != 144 {
		t.Errorf("expected fps limit 144, got %d", cfg.Graphics.FPSLimit)
	}

	if cfg.Audio.MasterVolume != 0.5 {
		t.Errorf("expected master volume 0.5, got %f", cfg.Audio.MasterVolume)
	}
	if !cfg.Audio.Muted {
		t.Error("expected muted to be true")
	}

	if cfg.Network.LoginServer != "game.server.com:6900" {
		t.Errorf("expected server game.server.com:6900, got %s", cfg.Network.LoginServer)
	}

	if cfg.Game.Language != "ja" {
		t.Errorf("expected language 'ja', got %s", cfg.Game.Language)
	}
	if !cfg.Game.ShowFPS {
		t.Error("expected show_fps to be true")
	}

	if cfg.Logging.Level != "debug" {
		t.Errorf("expected log level 'debug', got %s", cfg.Logging.Level)
	}
	if cfg.Logging.LogFile != "game.log" {
		t.Errorf("expected log file 'game.log', got %s", cfg.Logging.LogFile)
	}
}

func TestLoadFromFileInvalid(t *testing.T) {
	// Create temporary config file with invalid YAML
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "invalid.yaml")

	invalidYAML := `
graphics:
  width: not a number
  invalid syntax here
`

	if err := os.WriteFile(configPath, []byte(invalidYAML), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	// Try to load - should error
	cfg := Default()
	err := loadFromFile(cfg, configPath)
	if err == nil {
		t.Error("expected error loading invalid YAML, got nil")
	}
}

func TestLoadFromFileMissing(t *testing.T) {
	cfg := Default()
	err := loadFromFile(cfg, "/nonexistent/path/config.yaml")
	if err == nil {
		t.Error("expected error loading missing file, got nil")
	}
}

func TestConfigDir(t *testing.T) {
	dir := ConfigDir()

	// Just verify it returns a non-empty path
	// Actual path depends on OS
	if dir == "" {
		t.Error("ConfigDir returned empty string")
	}

	// Verify path is absolute
	if !filepath.IsAbs(dir) {
		t.Errorf("ConfigDir should return absolute path, got %s", dir)
	}
}

func TestFindConfigFile(t *testing.T) {
	// Save current directory
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)

	// Create temp directory and change to it
	tmpDir := t.TempDir()
	os.Chdir(tmpDir)

	// No config file exists - should return empty
	path := findConfigFile()
	if path != "" {
		t.Errorf("expected empty path when no config exists, got %s", path)
	}

	// Create config.yaml in current directory
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("graphics:\n  width: 800\n"), 0644); err != nil {
		t.Fatalf("failed to create test config: %v", err)
	}

	// Should find it now
	path = findConfigFile()
	if path == "" {
		t.Error("expected to find config.yaml in current directory")
	}
}

func TestApplyFlags(t *testing.T) {
	tests := []struct {
		name     string
		setup    func()
		verify   func(*Config) error
		teardown func()
	}{
		{
			name: "debug flag",
			setup: func() {
				*flagDebug = true
			},
			verify: func(cfg *Config) error {
				if cfg.Logging.Level != "debug" {
					t.Errorf("expected log level 'debug', got %s", cfg.Logging.Level)
				}
				if !cfg.Game.ShowFPS {
					t.Error("expected show_fps to be enabled with debug flag")
				}
				return nil
			},
			teardown: func() {
				*flagDebug = false
			},
		},
		{
			name: "server flag",
			setup: func() {
				*flagServer = "custom.server.com:7000"
			},
			verify: func(cfg *Config) error {
				if cfg.Network.LoginServer != "custom.server.com:7000" {
					t.Errorf("expected server custom.server.com:7000, got %s", cfg.Network.LoginServer)
				}
				return nil
			},
			teardown: func() {
				*flagServer = ""
			},
		},
		{
			name: "windowed flag",
			setup: func() {
				*flagWindowed = true
			},
			verify: func(cfg *Config) error {
				if cfg.Graphics.Fullscreen {
					t.Error("expected fullscreen to be false with windowed flag")
				}
				return nil
			},
			teardown: func() {
				*flagWindowed = false
			},
		},
		{
			name: "fullscreen flag",
			setup: func() {
				*flagFullscreen = true
			},
			verify: func(cfg *Config) error {
				if !cfg.Graphics.Fullscreen {
					t.Error("expected fullscreen to be true with fullscreen flag")
				}
				return nil
			},
			teardown: func() {
				*flagFullscreen = false
			},
		},
		{
			name: "width and height flags",
			setup: func() {
				*flagWidth = 2560
				*flagHeight = 1440
			},
			verify: func(cfg *Config) error {
				if cfg.Graphics.Width != 2560 {
					t.Errorf("expected width 2560, got %d", cfg.Graphics.Width)
				}
				if cfg.Graphics.Height != 1440 {
					t.Errorf("expected height 1440, got %d", cfg.Graphics.Height)
				}
				return nil
			},
			teardown: func() {
				*flagWidth = 0
				*flagHeight = 0
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			tt.setup()
			defer tt.teardown()

			// Apply flags to default config
			cfg := Default()
			applyFlags(cfg)

			// Verify
			tt.verify(cfg)
		})
	}
}

func TestLoadPriority(t *testing.T) {
	// Create temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	yamlContent := `
graphics:
  width: 1600
  height: 900
`

	if err := os.WriteFile(configPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	// Set flag to override config file
	*flagConfig = configPath
	*flagWidth = 1920
	defer func() {
		*flagConfig = ""
		*flagWidth = 0
	}()

	// Load config
	cfg, err := Load()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Width should be from flag (1920), not file (1600)
	if cfg.Graphics.Width != 1920 {
		t.Errorf("expected width 1920 from flag, got %d", cfg.Graphics.Width)
	}

	// Height should be from file (900) since no flag override
	if cfg.Graphics.Height != 900 {
		t.Errorf("expected height 900 from file, got %d", cfg.Graphics.Height)
	}
}
