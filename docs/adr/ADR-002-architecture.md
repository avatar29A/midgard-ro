# ADR-002: Project Architecture

**Status**: Accepted  
**Date**: 2025-01-09  
**Decision Makers**: Boris (CEO), Ilon (CTO)

## Context

We need to establish a clean, maintainable architecture for the game client that supports:
- Clear separation of concerns
- Testability
- Educational value
- Future extensibility

## Decision

We adopt a **layered architecture** with clear dependency rules.

### Layer Structure

```
┌─────────────────────────────────────────────────────────────┐
│                       cmd/ (Entry Points)                    │
│                    Depends on: internal/, pkg/               │
└─────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────▼───────────────────────────────┐
│                    internal/game/ (Game Logic)               │
│              Depends on: internal/engine/, pkg/              │
│  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────────────┐ │
│  │  world  │  │ entity  │  │   ui    │  │     states      │ │
│  └─────────┘  └─────────┘  └─────────┘  └─────────────────┘ │
└─────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────▼───────────────────────────────┐
│                 internal/engine/ (Engine Core)               │
│                      Depends on: pkg/                        │
│  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────────────┐ │
│  │renderer │  │  input  │  │  audio  │  │      time       │ │
│  └─────────┘  └─────────┘  └─────────┘  └─────────────────┘ │
└─────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────▼───────────────────────────────┐
│             internal/assets/ (Asset Management)              │
│                      Depends on: pkg/                        │
│  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────────────┐ │
│  │  cache  │  │  loader │  │converter│  │    registry     │ │
│  └─────────┘  └─────────┘  └─────────┘  └─────────────────┘ │
└─────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────▼───────────────────────────────┐
│                internal/network/ (Networking)                │
│                      Depends on: pkg/                        │
│  ┌─────────────────┐  ┌─────────────────────────────────┐   │
│  │     client      │  │            packets              │   │
│  └─────────────────┘  └─────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────▼───────────────────────────────┐
│                    pkg/ (Reusable Libraries)                 │
│                      No internal dependencies                │
│  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────────────┐ │
│  │   grf   │  │ formats │  │  math   │  │      types      │ │
│  └─────────┘  └─────────┘  └─────────┘  └─────────────────┘ │
└─────────────────────────────────────────────────────────────┘
```

### Dependency Rules

1. **cmd/** → Can import from `internal/` and `pkg/`
2. **internal/game/** → Can import from `internal/engine/`, `internal/assets/`, `internal/network/`, `pkg/`
3. **internal/engine/** → Can only import from `pkg/`
4. **internal/assets/** → Can only import from `pkg/`
5. **internal/network/** → Can only import from `pkg/`
6. **pkg/** → No internal imports, only standard library and external packages

### Package Responsibilities

#### pkg/grf
Standalone GRF archive reader:
```go
package grf

type Archive struct { ... }
func Open(path string) (*Archive, error)
func (a *Archive) List() []string
func (a *Archive) Read(path string) ([]byte, error)
```

#### pkg/formats
RO file format parsers:
```go
package formats

// Sprites
type SPR struct { Frames []Frame }
type ACT struct { Actions []Action }

// Maps
type GAT struct { Cells [][]Cell }
type GND struct { Tiles []Tile, Surfaces []Surface }
type RSW struct { Objects []Object, Lights []Light }

// Models
type RSM struct { Meshes []Mesh }
```

#### internal/engine/renderer
OpenGL abstraction:
```go
package renderer

type Renderer interface {
    Begin()
    End()
    DrawSprite(sprite *Sprite, pos Vec2, opts DrawOptions)
    DrawMesh(mesh *Mesh, transform Mat4)
}
```

#### internal/game/entity
Entity Component System (simple):
```go
package entity

type Entity struct {
    ID       uint32
    Type     EntityType
    Position Vec3
    Sprite   *Sprite
    // ... components
}

type Manager struct {
    entities map[uint32]*Entity
}
```

## Rationale

1. **Separation of Concerns**: Each layer has a single responsibility
2. **Testability**: Lower layers (pkg/) are easily unit tested
3. **Reusability**: pkg/grf can be used by other projects
4. **Learning**: Clear structure helps understand game architecture

## Consequences

### Positive
- Clean import graph
- Easy to test components in isolation
- Clear boundaries for future contributors

### Negative
- More boilerplate for simple features
- Need discipline to maintain layer boundaries

## File Structure

```
midgard-ro/
├── cmd/
│   └── client/
│       └── main.go              # Entry point
├── internal/
│   ├── assets/
│   │   ├── cache.go             # Asset caching
│   │   ├── loader.go            # Asset loading
│   │   └── registry.go          # Asset registry
│   ├── engine/
│   │   ├── renderer/
│   │   │   ├── renderer.go      # Main renderer
│   │   │   ├── shader.go        # Shader management
│   │   │   ├── texture.go       # Texture management
│   │   │   └── sprite.go        # Sprite rendering
│   │   ├── input/
│   │   │   └── input.go         # Input handling
│   │   └── audio/
│   │       └── audio.go         # Audio system
│   ├── game/
│   │   ├── game.go              # Main game struct
│   │   ├── world/
│   │   │   └── map.go           # Map management
│   │   ├── entity/
│   │   │   ├── entity.go        # Entity base
│   │   │   ├── player.go        # Player entity
│   │   │   └── monster.go       # Monster entity
│   │   ├── ui/
│   │   │   └── ui.go            # UI system
│   │   └── states/
│   │       ├── state.go         # State interface
│   │       ├── login.go         # Login state
│   │       ├── charselect.go    # Character selection
│   │       └── ingame.go        # In-game state
│   └── network/
│       ├── client.go            # Network client
│       └── packets/
│           ├── login.go         # Login packets
│           ├── char.go          # Character packets
│           └── map.go           # Map packets
└── pkg/
    ├── grf/
    │   ├── grf.go               # GRF reader
    │   ├── grf_test.go          # Tests
    │   └── des.go               # DES decryption
    ├── formats/
    │   ├── spr.go               # SPR parser
    │   ├── act.go               # ACT parser
    │   ├── gat.go               # GAT parser
    │   ├── gnd.go               # GND parser
    │   ├── rsw.go               # RSW parser
    │   └── rsm.go               # RSM parser
    └── math/
        ├── vec2.go              # 2D vector
        ├── vec3.go              # 3D vector
        └── mat4.go              # 4x4 matrix
```

## References
- [Standard Go Project Layout](https://github.com/golang-standards/project-layout)
- [Game Programming Patterns](https://gameprogrammingpatterns.com/)
