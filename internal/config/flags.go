package config

import (
	"flag"
	"time"
)

var (
	flagConfig     = flag.String("config", "", "Path to config file")
	flagDebug      = flag.Bool("debug", false, "Enable debug logging")
	flagServer     = flag.String("server", "", "Login server address")
	flagWindowed   = flag.Bool("windowed", false, "Run in windowed mode")
	flagFullscreen = flag.Bool("fullscreen", false, "Run in fullscreen mode")
	flagWidth      = flag.Int("width", 0, "Window width")
	flagHeight     = flag.Int("height", 0, "Window height")
	flagTrace      = flag.String("trace", "", "Comma-separated trace channels (move, pick, net, render, or all)")
	flagShotAfter  = flag.Duration("screenshot-after", 0, "Capture a screenshot this long after startup, then keep running (QA aid)")
	flagShotEvery  = flag.Duration("screenshot-every", 0, "Capture a screenshot on this interval (QA aid)")
	flagAutoLogin  = flag.Bool("autologin", false, "Log in and enter the first character without input (QA aid)")
)

// ParseFlags parses command-line flags. Call this early in main().
func ParseFlags() {
	flag.Parse()
}

// TraceSpec returns the --trace channel list, empty when tracing is off.
func TraceSpec() string {
	return *flagTrace
}

// ScreenshotAfter returns the one-shot screenshot delay, zero when unset.
func ScreenshotAfter() time.Duration {
	return *flagShotAfter
}

// ScreenshotEvery returns the repeating screenshot interval, zero when unset.
func ScreenshotEvery() time.Duration {
	return *flagShotEvery
}

// AutoLogin reports whether the client should walk the login and character
// select screens by itself.
//
// Getting in game otherwise takes two clicks, which is enough to make every
// check of anything past the login screen need a person at the keyboard. The
// credentials still come from the config, so this grants no access that
// running the client would not.
func AutoLogin() bool {
	return *flagAutoLogin
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
