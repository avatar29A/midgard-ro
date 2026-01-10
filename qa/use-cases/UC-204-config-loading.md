# UC-204: Configuration Loading

## Description
Tests loading configuration from YAML file with fallback to defaults. Configuration drives many aspects of the application.

## Preconditions
- None (config system is self-contained)

## Test Steps

### Load from File
1. Create valid `config.yaml` with custom settings
2. Call `config.Load()`
3. Verify config is loaded from file
4. Verify values match file contents
5. Verify no errors

### Load with Missing File
1. Delete or rename `config.yaml`
2. Call `config.Load()`
3. Verify default config is returned
4. Verify no error (graceful fallback)

### Load with Invalid YAML
1. Create `config.yaml` with syntax errors
2. Call `config.Load()`
3. Verify error is returned
4. Verify error message indicates YAML parse failure

### Default Values
1. Call `config.Default()`
2. Verify all fields have sensible defaults:
   - Graphics: 1280x720, windowed, VSync on
   - Audio: 80% master volume
   - Network: localhost:6900
   - Logging: info level

### CLI Flag Override
1. Run with `--width 1920 --height 1080`
2. Verify CLI flags override config file values
3. Verify other config values are preserved

## Expected Results
- Valid config files load correctly
- Missing files fall back to defaults gracefully
- Invalid YAML returns error with helpful message
- CLI flags take precedence over file
- Default values are reasonable for development

## Priority
High

## Related
- PRD Section: 6.1 High-Level Components
- ADR: ADR-003-configuration-and-logging.md
- Code: `/Users/borisglebov/git/Faultbox/midgard-ro/internal/config/load.go`
- Code: `/Users/borisglebov/git/Faultbox/midgard-ro/internal/config/flags.go`
- Test: None (integration test)

## Config Structure
```yaml
graphics:
  width: 1280
  height: 720
  fullscreen: false
  vsync: true
  fps_limit: 0

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
  show_fps: false
  show_ping: false

logging:
  level: "info"
  log_file: ""
```
