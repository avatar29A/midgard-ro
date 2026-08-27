package config

import (
	"flag"
	"fmt"
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
	flagTrace      = flag.String("trace", "", "Comma-separated trace channels (move, pick, net, render, status, npc, map, or all)")
	flagShotAfter  = flag.Duration("screenshot-after", 0, "Capture a screenshot this long after startup, then keep running (QA aid)")
	flagShotEvery  = flag.Duration("screenshot-every", 0, "Capture a screenshot on this interval (QA aid)")
	flagAutoLogin  = flag.Bool("autologin", false, "Log in and enter the first character without input (QA aid)")
	flagDebugHUD   = flag.Bool("debug-overlay", false, "Start with the F3 debug overlay open (QA aid)")
	flagNoBGM      = flag.Bool("no-bgm", false, "Run without background music, keeping sound effects")
	flagWalkTo     = flag.String("walk-to", "", "Once in game, walk to this cell, e.g. 156,22 (QA aid)")
	flagMouseAt    = flag.String("mouse-at", "", "Once in game, put the pointer at this window position, e.g. 640,360 (QA aid)")
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

// DebugOverlay reports whether the F3 overlay should start open.
//
// The overlay is the cheapest proof that a value reached the client, but F3 is
// a keypress, so an unattended --screenshot-after run could never capture it.
// This makes the same readout reachable from a command line.
func DebugOverlay() bool {
	return *flagDebugHUD
}

// NoBGM reports whether background music should be left off.
//
// The music loops for as long as the client runs, which is exactly what you
// do not want while reading a log or listening for a sound effect. Sound
// effects are unaffected.
func NoBGM() bool {
	return *flagNoBGM
}

// WalkTo returns the cell --walk-to asked for, and whether one was given.
//
// Stepping onto a warp is the one thing an unattended run could not do by
// itself: the trigger is a cell, and reaching it takes a click. This issues
// that click once the map is up, through the same path a real one takes, so
// a map change can be captured with nobody at the keyboard.
func WalkTo() (x, y int, ok bool) {
	if *flagWalkTo == "" {
		return 0, 0, false
	}
	if _, err := fmt.Sscanf(*flagWalkTo, "%d,%d", &x, &y); err != nil {
		return 0, 0, false
	}
	return x, y, true
}

// WalkToSpec returns the raw --walk-to value, for reporting one that did not
// parse.
func WalkToSpec() string {
	return *flagWalkTo
}

// MouseAt returns the window position --mouse-at asked for, and whether one
// was given. Which cursor is drawn depends on what the pointer is over, and
// an unattended capture has no hand to put it there.
func MouseAt() (x, y int, ok bool) {
	if *flagMouseAt == "" {
		return 0, 0, false
	}
	if _, err := fmt.Sscanf(*flagMouseAt, "%d,%d", &x, &y); err != nil {
		return 0, 0, false
	}
	return x, y, true
}

// MouseAtSpec returns the raw --mouse-at value, for reporting one that did
// not parse.
func MouseAtSpec() string {
	return *flagMouseAt
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
