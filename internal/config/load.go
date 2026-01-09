package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"gopkg.in/yaml.v3"
)

// Load loads configuration with priority: defaults < file < flags.
func Load() (*Config, error) {
	// Start with defaults
	cfg := Default()

	// Try to load from file (explicit path takes priority)
	configPath := ConfigPath()
	if configPath == "" {
		configPath = findConfigFile()
	}

	if configPath != "" {
		if err := loadFromFile(cfg, configPath); err != nil {
			return nil, fmt.Errorf("loading config from %s: %w", configPath, err)
		}
	}

	// Apply CLI flags (highest priority)
	applyFlags(cfg)

	return cfg, nil
}

// findConfigFile looks for config in standard locations.
func findConfigFile() string {
	candidates := []string{
		"./config.yaml",
		filepath.Join(ConfigDir(), "config.yaml"),
	}

	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

// ConfigDir returns the OS-appropriate config directory.
func ConfigDir() string {
	switch runtime.GOOS {
	case "darwin":
		home, _ := os.UserHomeDir()
		return filepath.Join(home, "Library", "Application Support", "MidgardRO")
	case "windows":
		return filepath.Join(os.Getenv("APPDATA"), "MidgardRO")
	default: // Linux and others
		if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
			return filepath.Join(xdg, "midgard-ro")
		}
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".config", "midgard-ro")
	}
}

// loadFromFile loads config from a YAML file, merging with existing values.
func loadFromFile(cfg *Config, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, cfg)
}
