# CLAUDE.md - Instructions for Claude Code

This file provides context for Claude Code when working on the Midgard RO project.

## ⚠️ Session Continuity

**When starting a new session:**
1. Read the latest session log in `docs/sessions/`
2. Check the current milestone status in `docs/WORKFLOW.md`
3. Ask Boris for any context from the previous session

**Before ending a session:**
1. Summarize what was accomplished
2. Create/update the session log
3. Note what should be done next

## Project Overview

**Midgard RO** is a modern Ragnarok Online client written in Go. This is an educational project demonstrating AI-assisted game development.

### Tech Stack
- **Language**: Go 1.22+
- **Graphics**: OpenGL 4.1 (Core Profile)
- **Windowing**: SDL2 via go-sdl2 (requires CGO + system SDL2)
- **Server**: Hercules (RO server emulator)

### Target Platforms
- macOS (Apple Silicon + Intel) - PRIMARY
- Linux x64
- Windows x64 (stretch goal)

---

## Architecture

We use a **layered architecture**. See `docs/adr/ADR-002-architecture.md` for details.

### Dependency Rules (IMPORTANT!)

```
cmd/           → imports internal/, pkg/
internal/game/ → imports internal/engine/, internal/assets/, internal/network/, pkg/
internal/engine/ → imports pkg/ ONLY
internal/assets/ → imports pkg/ ONLY
internal/network/ → imports pkg/ ONLY
pkg/           → NO internal imports (standard lib + external only)
```

### Package Purposes

| Package | Purpose |
|---------|---------|
| `cmd/client` | Main entry point |
| `internal/engine/renderer` | OpenGL rendering abstraction |
| `internal/engine/input` | Input handling via SDL2 |
| `internal/engine/audio` | Audio playback via SDL2 |
| `internal/game` | Game loop, state management |
| `internal/game/world` | Map loading and rendering |
| `internal/game/entity` | Players, mobs, NPCs |
| `internal/game/ui` | User interface |
| `internal/game/states` | Game states (login, char select, etc.) |
| `internal/assets` | Asset loading and caching |
| `internal/network` | Hercules protocol client |
| `pkg/grf` | GRF archive reader (reusable library) |
| `pkg/formats` | RO file format parsers (SPR, ACT, GAT, etc.) |
| `pkg/math` | Game math utilities (vectors, matrices) |
| `pkg/events` | Event bus and event definitions |
| `internal/debug` | Debug API and QA tooling |
| `qa/runner` | QA test runner and validation |

---

## Code Style

### Go Conventions
- Use `gofmt` / `goimports`
- Follow standard Go naming (MixedCaps, not snake_case)
- Keep functions small and focused
- Write table-driven tests

### Project Conventions
- **Errors**: Wrap with context: `fmt.Errorf("loading sprite %s: %w", name, err)`
- **Logging**: Use structured logging (slog)
- **Comments**: Document all exported functions
- **Tests**: Place `_test.go` files next to source

### File Organization
```go
// file.go
package mypackage

// Constants first
const (...)

// Types
type MyStruct struct {...}

// Constructors
func NewMyStruct() *MyStruct {...}

// Methods
func (m *MyStruct) Method() {...}

// Private helpers
func helper() {...}
```

---

## Working with Claude Code

### Before Starting a Task

1. **Read relevant docs** in `docs/` folder
2. **Check existing code** for patterns
3. **Understand the layer** you're working in
4. **Create tests** for new functionality

### Task Workflow

1. **Understand**: Read PRD section for context
2. **Plan**: Break down into small steps
3. **Implement**: Write code + tests
4. **Test**: Run `go test ./...`
5. **Document**: Update docs if needed

### Common Tasks

#### Adding a new file format parser (pkg/formats/):
```bash
# 1. Create the parser file
touch pkg/formats/newformat.go

# 2. Create test file with example data
touch pkg/formats/newformat_test.go

# 3. Run tests
go test ./pkg/formats/...
```

#### Adding a new game feature (internal/game/):
```bash
# 1. Check if engine support exists
# 2. Create feature in appropriate subpackage
# 3. Wire up in game/game.go
# 4. Add state handling if needed
```

#### Adding network packets (internal/network/packets/):
```bash
# 1. Reference Hercules packet documentation
# 2. Create packet struct with proper byte layout
# 3. Implement Read/Write methods
# 4. Add tests with known packet data
```

---

## Key Files

| File | Purpose |
|------|---------|
| `docs/WORKFLOW.md` | How to work with Claude Code |
| `docs/sessions/` | Session logs for continuity |
| `cmd/client/main.go` | Application entry point |
| `internal/game/game.go` | Main game struct and loop |
| `internal/engine/renderer/renderer.go` | Rendering interface |
| `internal/network/client.go` | Network connection handling |
| `pkg/grf/grf.go` | GRF archive reading |
| `docs/prd/PRD.md` | Full requirements |
| `docs/adr/` | Architecture decisions |

---

## Build & Run

```bash
# Install dependencies
go mod tidy

# Run the client
go run ./cmd/client

# Run all tests
go test ./...

# Run with verbose output
go test -v ./pkg/...

# Build binary
go build -o midgard ./cmd/client
```

---

## Current Milestone

**Phase 1: Foundation (Weeks 1-2)**
- [x] SDL2 window creation
- [x] OpenGL context initialization
- [ ] Basic rendering pipeline
- [ ] Game loop with timing

See `docs/prd/PRD.md` Section 7 for full milestone breakdown.

---

## External Resources

- [OpenGL Tutorial](https://learnopengl.com)
- [go-sdl2 Docs](https://github.com/veandco/go-sdl2)
- [SDL2 Wiki](https://wiki.libsdl.org/SDL2)
- [Hercules Wiki](https://herc.ws/wiki/)
- [RO File Formats](https://ratemyserver.net/dev/)

---

## Notes for Claude

### DO:
- Follow the layer dependency rules strictly
- Write tests for all new functionality
- Use descriptive error messages
- Keep functions under 50 lines when possible
- Explain complex algorithms with comments

### DON'T:
- Import `internal/` packages from `pkg/`
- Skip error handling
- Create circular dependencies
- Add external dependencies without discussing first

### When Stuck:
1. Check existing code for patterns
2. Read the relevant ADR
3. Ask Boris for clarification on requirements
4. Search for RO-specific documentation
