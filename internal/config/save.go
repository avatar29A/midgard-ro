package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Save writes the config to the user's config directory.
func (c *Config) Save() error {
	dir := ConfigDir()

	// Create directory if needed
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	path := filepath.Join(dir, "config.yaml")
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// SaveTo writes the config to a specific path.
func (c *Config) SaveTo(path string) error {
	// Create parent directory if needed
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}
