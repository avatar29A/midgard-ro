// Package config handles game configuration loading and management.
package config

import "time"

// Config holds all game settings.
type Config struct {
	Graphics GraphicsConfig `yaml:"graphics"`
	Audio    AudioConfig    `yaml:"audio"`
	Network  NetworkConfig  `yaml:"network"`
	Game     GameConfig     `yaml:"game"`
	Data     DataConfig     `yaml:"data"`
	Logging  LoggingConfig  `yaml:"logging"`
}

// DataConfig holds game data file paths.
type DataConfig struct {
	GRFPaths []string `yaml:"grf_paths"` // Paths to GRF archives
}

// GraphicsConfig holds display and rendering settings.
type GraphicsConfig struct {
	Width      int  `yaml:"width"`
	Height     int  `yaml:"height"`
	Fullscreen bool `yaml:"fullscreen"`
	VSync      bool `yaml:"vsync"`
	FPSLimit   int  `yaml:"fps_limit"`
}

// AudioConfig holds audio settings.
type AudioConfig struct {
	MasterVolume float32 `yaml:"master_volume"`
	MusicVolume  float32 `yaml:"music_volume"`
	SFXVolume    float32 `yaml:"sfx_volume"`
	Muted        bool    `yaml:"muted"`
}

// NetworkConfig holds server connection settings.
type NetworkConfig struct {
	LoginServer    string        `yaml:"login_server"`
	ConnectTimeout time.Duration `yaml:"connect_timeout"`
	Username       string        `yaml:"username"`
	Password       string        `yaml:"password"`
}

// GameConfig holds gameplay settings.
type GameConfig struct {
	Language string `yaml:"language"`
	ShowFPS  bool   `yaml:"show_fps"`
	ShowPing bool   `yaml:"show_ping"`
}

// LoggingConfig holds logging settings.
type LoggingConfig struct {
	Level   string `yaml:"level"`
	LogFile string `yaml:"log_file"`
}

// Default returns a Config with sensible default values.
func Default() *Config {
	return &Config{
		Graphics: GraphicsConfig{
			Width:      1280,
			Height:     720,
			Fullscreen: false,
			VSync:      true,
			FPSLimit:   0,
		},
		Audio: AudioConfig{
			MasterVolume: 0.8,
			MusicVolume:  0.7,
			SFXVolume:    0.8,
			Muted:        false,
		},
		Network: NetworkConfig{
			LoginServer:    "127.0.0.1:6900",
			ConnectTimeout: 10 * time.Second,
		},
		Game: GameConfig{
			Language: "en",
			ShowFPS:  false,
			ShowPing: false,
		},
		Data: DataConfig{
			GRFPaths: []string{"data.grf"},
		},
		Logging: LoggingConfig{
			Level:   "info",
			LogFile: "",
		},
	}
}
