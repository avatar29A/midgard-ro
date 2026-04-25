# Midgard RO Client

**A modern Ragnarok Online client built from scratch in Go**

> Educational project demonstrating how modern AI tools can accelerate game development

## 🎯 Project Vision

Build a fully functional Ragnarok Online client that connects to Hercules servers, featuring:
- Enhanced graphics using modern OpenGL
- Clean, maintainable Go codebase
- Deep understanding of game development fundamentals

## 🚀 Q1 2025 Goals

| Feature | Status |
|---------|--------|
| Registration process | 🔴 Not Started |
| Character creation | 🔴 Not Started |
| Prontera city + 3 locations | 🔴 Not Started |
| Mob movement | 🔴 Not Started |
| Player movement | 🔴 Not Started |
| Same or enhanced graphics | 🔴 Not Started |

## 🛠 Tech Stack

- **Language**: Go 1.22+
- **Graphics**: OpenGL 4.1+ (modern, programmable pipeline)
- **Windowing**: SDL2 via [go-sdl2](https://github.com/veandco/go-sdl2)
- **Server**: Hercules (existing RO server emulator)

## 📁 Project Structure

```
midgard-ro/
├── cmd/
│   └── client/           # Main application entry point
├── internal/
│   ├── assets/           # Asset loading (GRF, sprites, textures)
│   ├── engine/           # Core game engine
│   │   ├── renderer/     # OpenGL rendering
│   │   ├── input/        # Input handling
│   │   └── audio/        # Sound system
│   ├── game/             # Game logic
│   │   ├── world/        # World/map management
│   │   ├── entity/       # Players, mobs, NPCs
│   │   └── ui/           # User interface
│   └── network/          # Hercules protocol
│       ├── packets/      # Packet definitions
│       └── client/       # Network client
├── pkg/
│   ├── grf/              # GRF file parser (reusable library)
│   ├── formats/          # RO file formats (SPR, ACT, RSM, etc.)
│   └── math/             # Game math utilities
├── assets/               # Game assets (gitignored, user provides)
├── docs/                 # Documentation
│   ├── prd/              # Product Requirements
│   ├── adr/              # Architecture Decision Records
│   └── research/         # Technical research notes
├── tools/                # Development tools
└── scripts/              # Build and utility scripts
```

## 🎓 Learning Journey

This project is designed as a learning experience. Each phase teaches specific concepts:

### Phase 1: Foundation (Weeks 1-2)
- SDL2 window creation and OpenGL context
- Basic rendering pipeline (vertices, shaders, textures)
- Game loop architecture

### Phase 2: Asset Pipeline (Weeks 3-4)
- GRF archive extraction
- Sprite (SPR/ACT) loading and animation
- Map (GAT/GND/RSW) loading

### Phase 3: Networking (Weeks 5-6)
- TCP socket handling in Go
- Hercules packet protocol
- Login/character flow

### Phase 4: Game World (Weeks 7-10)
- 2.5D isometric rendering
- Entity management
- Basic game mechanics

### Phase 5: Polish (Weeks 11-12)
- UI system
- Sound integration
- Performance optimization

## 🚦 Getting Started

End-to-end setup (client + self-hosted rAthena server) is documented in
**[docs/QUICKSTART.md](docs/QUICKSTART.md)**. The short version:

```bash
make env-install-macos    # Go, SDL2, colima, docker, docker-compose
colima start --memory 8 --cpu 4
make config               # creates config.yaml — edit GRF paths
make play                 # starts the local rAthena server and launches the client
```

You also need a legitimate copy of `data.grf` and `rdata.grf` from a
Ragnarok Online installation. Run `make help` for all targets.

## 📚 Documentation

- [Product Requirements Document](docs/prd/PRD.md)
- [Architecture Decisions](docs/adr/)
- [Technical Research](docs/research/)

---

*This is an educational and fan project. Ragnarok Online is a trademark of Gravity Co., Ltd.*
