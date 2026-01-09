package config

import "flag"

var (
	flagConfig     = flag.String("config", "", "Path to config file")
	flagDebug      = flag.Bool("debug", false, "Enable debug logging")
	flagServer     = flag.String("server", "", "Login server address")
	flagWindowed   = flag.Bool("windowed", false, "Run in windowed mode")
	flagFullscreen = flag.Bool("fullscreen", false, "Run in fullscreen mode")
	flagWidth      = flag.Int("width", 0, "Window width")
	flagHeight     = flag.Int("height", 0, "Window height")
)

// ParseFlags parses command-line flags. Call this early in main().
func ParseFlags() {
	flag.Parse()
}

// ConfigPath returns the explicit config path if provided via --config flag.
func ConfigPath() string {
	return *flagConfig
}

// applyFlags applies CLI flag overrides to the config.
func applyFlags(cfg *Config) {
	if *flagDebug {
		cfg.Logging.Level = "debug"
		cfg.Game.ShowFPS = true
	}
	if *flagServer != "" {
		cfg.Network.LoginServer = *flagServer
	}
	if *flagWindowed {
		cfg.Graphics.Fullscreen = false
	}
	if *flagFullscreen {
		cfg.Graphics.Fullscreen = true
	}
	if *flagWidth > 0 {
		cfg.Graphics.Width = *flagWidth
	}
	if *flagHeight > 0 {
		cfg.Graphics.Height = *flagHeight
	}
}
