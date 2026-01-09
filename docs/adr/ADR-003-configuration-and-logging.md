# ADR-003: Configuration and Logging

**Status**: Proposed
**Date**: 2025-01-09
**Decision Makers**: Boris (CEO), Ilon (CTO)

## Context

We need a configuration and logging system appropriate for a **desktop game client** (not a cloud service).

### Requirements

**Configuration:**
- Simple config file users can edit (graphics, audio, keybinds)
- Command-line flags for development/testing
- Sensible defaults that work out-of-the-box
- Settings persist between sessions

**Logging:**
- Debug logging for development
- Log file for troubleshooting crashes
- Minimal overhead in release builds

### Non-Requirements (Cloud/Service patterns we DON'T need)

- ~~HashiCorp Vault~~ - User enters credentials in login screen
- ~~Environment variables~~ - Desktop users don't set these
- ~~Multiple environment configs~~ - Only one environment: user's machine
- ~~JSON structured logs~~ - No log aggregation systems

---

## Decision

### Configuration: Simple Hierarchy

```
┌─────────────────────────────────────────────────────────────┐
│                    CONFIGURATION LAYERS                      │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│   Priority 3 (Highest): Command-line flags                  │
│   └── Developer overrides: --debug, --server, --windowed    │
│                                                              │
│   Priority 2: User config file                               │
│   └── ~/.config/midgard-ro/config.yaml (or ./config.yaml)   │
│                                                              │
│   Priority 1 (Lowest): Compiled defaults                    │
│   └── Sensible defaults embedded in binary                  │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

**Libraries:**
- `gopkg.in/yaml.v3` - YAML parsing (lightweight)
- `flag` (stdlib) - Command-line flags

### Logging: Zap with Simple File Output

**Libraries:**
- `go.uber.org/zap` - Fast structured logging
- `gopkg.in/natefinch/lumberjack.v2` - Log rotation (prevent huge log files)

---

## Implementation Plan

### Phase 1: Configuration Package

#### 1.1 Directory Structure

```
internal/
└── config/
    ├── config.go       # Config struct and defaults
    ├── load.go         # Load from file
    ├── save.go         # Save to file (for settings UI)
    └── flags.go        # CLI flag parsing
```

#### 1.2 Configuration Structure

```go
// internal/config/config.go
package config

import "time"

// Config holds all game settings
type Config struct {
    // Graphics settings
    Graphics GraphicsConfig `yaml:"graphics"`

    // Audio settings
    Audio AudioConfig `yaml:"audio"`

    // Network settings (servers)
    Network NetworkConfig `yaml:"network"`

    // Game settings
    Game GameConfig `yaml:"game"`

    // Logging settings
    Logging LoggingConfig `yaml:"logging"`
}

type GraphicsConfig struct {
    Width      int  `yaml:"width"`
    Height     int  `yaml:"height"`
    Fullscreen bool `yaml:"fullscreen"`
    VSync      bool `yaml:"vsync"`
    FPSLimit   int  `yaml:"fps_limit"` // 0 = unlimited
}

type AudioConfig struct {
    MasterVolume float32 `yaml:"master_volume"` // 0.0 - 1.0
    MusicVolume  float32 `yaml:"music_volume"`
    SFXVolume    float32 `yaml:"sfx_volume"`
    Muted        bool    `yaml:"muted"`
}

type NetworkConfig struct {
    LoginServer    string        `yaml:"login_server"`
    ConnectTimeout time.Duration `yaml:"connect_timeout"`
}

type GameConfig struct {
    Language     string `yaml:"language"`      // en, ko, etc.
    ShowFPS      bool   `yaml:"show_fps"`
    ShowPing     bool   `yaml:"show_ping"`
}

type LoggingConfig struct {
    Level   string `yaml:"level"`    // debug, info, warn, error
    LogFile string `yaml:"log_file"` // empty = no file logging
}

// Default returns config with sensible defaults
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
        Logging: LoggingConfig{
            Level:   "info",
            LogFile: "", // No file by default
        },
    }
}
```

#### 1.3 Config Loader

```go
// internal/config/load.go
package config

import (
    "errors"
    "os"
    "path/filepath"
    "runtime"

    "gopkg.in/yaml.v3"
)

// Load loads config with priority: defaults < file < flags
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
            return nil, err
        }
    }

    // Apply CLI flags (highest priority)
    applyFlags(cfg)

    return cfg, nil
}

// findConfigFile looks for config in standard locations
func findConfigFile() string {
    candidates := []string{
        "./config.yaml",                    // Current directory
        configDir() + "/config.yaml",       // User config dir
    }

    for _, path := range candidates {
        if _, err := os.Stat(path); err == nil {
            return path
        }
    }
    return ""
}

// configDir returns OS-appropriate config directory
func configDir() string {
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

func loadFromFile(cfg *Config, path string) error {
    data, err := os.ReadFile(path)
    if err != nil {
        return err
    }
    return yaml.Unmarshal(data, cfg)
}
```

#### 1.4 CLI Flags

```go
// internal/config/flags.go
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

// ConfigPath returns explicit config path if provided
func ConfigPath() string {
    return *flagConfig
}

func init() {
    flag.Parse()
}

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
```

#### 1.5 Save Config (for Settings UI)

```go
// internal/config/save.go
package config

import (
    "os"
    "path/filepath"

    "gopkg.in/yaml.v3"
)

// Save writes config to user's config directory
func (c *Config) Save() error {
    dir := configDir()

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
```

#### 1.6 Example Config File

```yaml
# Midgard RO Configuration
# Location: ~/.config/midgard-ro/config.yaml (Linux)
#           ~/Library/Application Support/MidgardRO/config.yaml (macOS)
#           %APPDATA%\MidgardRO\config.yaml (Windows)

graphics:
  width: 1920
  height: 1080
  fullscreen: false
  vsync: true
  fps_limit: 0  # 0 = unlimited

audio:
  master_volume: 0.8
  music_volume: 0.7
  sfx_volume: 0.8
  muted: false

network:
  login_server: "127.0.0.1:6900"
  connect_timeout: 10s

game:
  language: "en"
  show_fps: true
  show_ping: true

logging:
  level: "info"     # debug, info, warn, error
  log_file: ""      # empty = console only, or path like "midgard.log"
```

### Phase 2: Logging Package

#### 2.1 Directory Structure

```
internal/
└── logger/
    └── logger.go       # Simple logger setup
```

#### 2.2 Logger Implementation

```go
// internal/logger/logger.go
package logger

import (
    "os"

    "go.uber.org/zap"
    "go.uber.org/zap/zapcore"
    "gopkg.in/natefinch/lumberjack.v2"
)

var Log *zap.Logger
var Sugar *zap.SugaredLogger

// Init initializes the logger
func Init(level string, logFile string) error {
    lvl := parseLevel(level)

    // Console encoder (human-readable, colored)
    consoleEncoder := zapcore.NewConsoleEncoder(zapcore.EncoderConfig{
        TimeKey:        "time",
        LevelKey:       "level",
        MessageKey:     "msg",
        CallerKey:      "caller",
        EncodeTime:     zapcore.TimeEncoderOfLayout("15:04:05"),
        EncodeLevel:    zapcore.CapitalColorLevelEncoder,
        EncodeCaller:   zapcore.ShortCallerEncoder,
    })

    // Console output
    consoleCore := zapcore.NewCore(
        consoleEncoder,
        zapcore.AddSync(os.Stdout),
        lvl,
    )

    cores := []zapcore.Core{consoleCore}

    // File output (if configured)
    if logFile != "" {
        fileWriter := &lumberjack.Logger{
            Filename:   logFile,
            MaxSize:    50, // MB
            MaxBackups: 3,
            MaxAge:     7,  // days
            Compress:   true,
        }

        fileEncoder := zapcore.NewConsoleEncoder(zapcore.EncoderConfig{
            TimeKey:        "time",
            LevelKey:       "level",
            MessageKey:     "msg",
            CallerKey:      "caller",
            EncodeTime:     zapcore.ISO8601TimeEncoder,
            EncodeLevel:    zapcore.CapitalLevelEncoder,
            EncodeCaller:   zapcore.ShortCallerEncoder,
        })

        fileCore := zapcore.NewCore(
            fileEncoder,
            zapcore.AddSync(fileWriter),
            lvl,
        )
        cores = append(cores, fileCore)
    }

    Log = zap.New(zapcore.NewTee(cores...), zap.AddCaller())
    Sugar = Log.Sugar()

    return nil
}

func parseLevel(level string) zapcore.Level {
    switch level {
    case "debug":
        return zapcore.DebugLevel
    case "warn":
        return zapcore.WarnLevel
    case "error":
        return zapcore.ErrorLevel
    default:
        return zapcore.InfoLevel
    }
}

// Sync flushes any buffered log entries
func Sync() {
    if Log != nil {
        _ = Log.Sync()
    }
}
```

#### 2.3 Usage

```go
// Simple logging
logger.Sugar.Info("Game started")
logger.Sugar.Debugf("Loading map: %s", mapName)
logger.Sugar.Warnw("Connection slow", "ping", pingMs)
logger.Sugar.Error("Failed to load sprite", zap.Error(err))
```

### Phase 3: Integration

#### 3.1 Updated main.go

```go
// cmd/client/main.go
package main

import (
    "fmt"
    "os"

    "github.com/Faultbox/midgard-ro/internal/config"
    "github.com/Faultbox/midgard-ro/internal/logger"
    "github.com/Faultbox/midgard-ro/internal/game"
)

func main() {
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

    logger.Sugar.Info("Midgard RO starting")
    logger.Sugar.Debugf("Config loaded: %+v", cfg)

    // Run game
    if err := game.Run(cfg); err != nil {
        logger.Sugar.Fatalf("Game error: %v", err)
    }
}
```

---

## CLI Usage Examples

```bash
# Normal launch (auto-finds config)
./midgard

# Use specific config file
./midgard --config config.yaml

# Development mode (debug logs + FPS counter)
./midgard --config config.yaml --debug

# Connect to specific server
./midgard --server play.example.com:6900

# Windowed mode with specific resolution
./midgard --windowed --width 1920 --height 1080

# Fullscreen
./midgard --fullscreen
```

---

## Config File Locations

| OS | Path |
|----|------|
| Linux | `~/.config/midgard-ro/config.yaml` |
| macOS | `~/Library/Application Support/MidgardRO/config.yaml` |
| Windows | `%APPDATA%\MidgardRO\config.yaml` |

Fallback: `./config.yaml` in current directory (useful for development)

---

## Consequences

### Positive
- Simple, game-appropriate approach
- Users can easily edit YAML config
- CLI flags for quick testing
- Works offline (no external dependencies)
- Config can be saved by future Settings UI

### Negative
- No hot-reload (requires restart for config changes)
- Manual YAML editing until Settings UI is built

---

## Execution Checklist

- [ ] **Phase 1: Configuration**
  - [ ] Add `gopkg.in/yaml.v3` dependency
  - [ ] Create `internal/config/` package
  - [ ] Implement defaults, load, save, flags
  - [ ] Write unit tests

- [ ] **Phase 2: Logging**
  - [ ] Add `go.uber.org/zap` dependency
  - [ ] Add `gopkg.in/natefinch/lumberjack.v2` dependency
  - [ ] Create `internal/logger/` package
  - [ ] Test console and file output

- [ ] **Phase 3: Integration**
  - [ ] Update `cmd/client/main.go`
  - [ ] Update `internal/game/game.go` to accept config
  - [ ] Migrate existing `slog` calls to zap

---

## Future Enhancements

- **Settings UI**: In-game menu to modify config and call `config.Save()`
- **Keybinds**: Add `keybinds` section for customizable controls
- **Profiles**: Multiple config profiles for different servers

---

## References

- [YAML v3 for Go](https://github.com/go-yaml/yaml)
- [Uber Zap Logger](https://github.com/uber-go/zap)
- [Lumberjack Log Rotation](https://github.com/natefinch/lumberjack)
- [XDG Base Directory Spec](https://specifications.freedesktop.org/basedir-spec/basedir-spec-latest.html)
