# Product Requirements Document: Midgard RO Client

**Version**: 1.0  
**Date**: January 9, 2025  
**Authors**: Boris (CEO/Game Designer), Ilon (CTO)

---

## 1. Executive Summary

### 1.1 Purpose
Build a modern Ragnarok Online client from scratch using Go, SDL2, and OpenGL. This project serves dual purposes:
1. **Educational**: Demonstrate AI-assisted game development and teach gamedev fundamentals
2. **Showcase**: Create a portfolio piece showing modern reimplementation of classic game

### 1.2 Success Criteria (Q1 2025)
By end of March 2025, the client must:
- [ ] Successfully connect to a Hercules server
- [ ] Complete registration and login flow
- [ ] Create and select characters
- [ ] Render Prontera and 3 surrounding maps
- [ ] Display and animate player characters
- [ ] Display and animate monsters with AI movement
- [ ] Match or exceed original client's visual quality

---

## 2. Scope

### 2.1 In Scope (Q1 MVP)

| Category | Features |
|----------|----------|
| **Authentication** | Account registration, login, character selection |
| **Character** | Creation (6 base classes), basic stats display |
| **World** | Prontera city, prt_fild01-03 (surrounding fields) |
| **Rendering** | 2.5D isometric maps, sprites, basic lighting |
| **Entities** | Player movement, NPC display, mob spawns |
| **Network** | Hercules protocol, state synchronization |

### 2.2 Out of Scope (Post-Q1)
- Combat system
- Skill system
- Party/Guild systems
- Chat system (beyond basic)
- Inventory management
- Quest system
- Other cities/dungeons

---

## 3. Technical Requirements

### 3.1 Technology Stack

| Component | Technology | Rationale |
|-----------|------------|-----------|
| Language | Go 1.22+ | Memory safety, concurrency, fast compilation |
| Graphics API | OpenGL 4.1+ | Cross-platform, extensive documentation |
| Windowing | SDL2 | Input, audio, window management, mature |
| Go-SDL2 Binding | veandco/go-sdl2 | Well-maintained, extensive docs |
| Server | Hercules | Active community, documented protocol |

### 3.2 Supported Platforms (MVP)
- macOS (Apple Silicon + Intel)
- Linux (x64)
- Windows (x64) - stretch goal

### 3.3 Performance Targets
- 60 FPS minimum at 1080p
- < 500MB RAM usage
- < 5 second initial load time

---

## 4. Feature Specifications

### 4.1 Authentication System

#### 4.1.1 Registration
```
Flow:
1. User enters username, password, email
2. Client sends registration packet to login server
3. Server validates and creates account
4. Client receives success/failure response
5. Redirect to login screen
```

**Packets Required**:
- `CA_REQ_NEW_ACCOUNT` (or equivalent Hercules packet)

#### 4.1.2 Login
```
Flow:
1. User enters username, password
2. Client sends login request
3. Server authenticates, returns session ID + char server list
4. Client connects to character server
5. Server sends character list
6. User selects character
7. Client receives map server info
8. Connect to map server, enter game
```

**Packets Required**:
- `CA_LOGIN` - Login request
- `AC_ACCEPT_LOGIN` - Login success with server list
- `CH_ENTER` - Enter character server
- `HC_ACCEPT_ENTER` - Character list
- `CH_SELECT_CHAR` - Select character
- `HC_NOTIFY_ZONESVR` - Map server info
- `CZ_ENTER` - Enter map server
- `ZC_ACCEPT_ENTER` - Enter success

### 4.2 Character System

#### 4.2.1 Character Creation
- **Classes**: Novice (spawns as), Swordman, Mage, Archer, Thief, Merchant, Acolyte
- **Appearance**: Hair style (1-23), Hair color (1-8), Gender
- **Stats**: Initial stat point allocation (pre-Renewal style)

#### 4.2.2 Character Display
- Show name, class, level
- Display equipment sprites
- Show HP/SP bars
- Current map indicator

### 4.3 World Rendering

#### 4.3.1 Maps Required
| Map ID | Name | Type |
|--------|------|------|
| prontera | Prontera City | Town |
| prt_fild01 | Prontera Field 1 | Field |
| prt_fild02 | Prontera Field 2 | Field |
| prt_fild03 | Prontera Field 3 | Field |

#### 4.3.2 Rendering Features
- **Ground**: Textured tiles with height variation
- **Objects**: 3D models (RSM format)
- **Lighting**: Ambient + directional light
- **Water**: Animated water surfaces
- **Effects**: Basic particle effects

### 4.4 Entity System

#### 4.4.1 Player Character
- 8-directional movement
- Idle, walking, sitting animations
- Smooth interpolated movement

#### 4.4.2 NPCs
- Static display with idle animation
- Name display
- Click highlighting

#### 4.4.3 Monsters
- Spawn from server data
- Random movement AI
- Idle, walk, attack animations
- HP bar display

---

## 5. Asset Requirements

### 5.1 GRF Extraction
Client must read from official GRF files:
- `data.grf` - Main game assets
- `rdata.grf` - Additional assets

### 5.2 File Formats to Support

| Format | Extension | Purpose |
|--------|-----------|---------|
| GRF | .grf | Archive container |
| Sprite | .spr | 2D sprite sheets |
| Action | .act | Animation data |
| Ground | .gnd | Map ground mesh |
| Altitude | .gat | Walkability data |
| Resource | .rsw | Map objects/lights |
| Model | .rsm | 3D object models |
| Texture | .bmp, .tga | Textures |
| Palette | .pal | Color palettes |

---

## 6. Architecture Overview

### 6.1 High-Level Components

```
┌─────────────────────────────────────────────────────────────┐
│                        GAME CLIENT                          │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐ │
│  │   Assets    │  │    Game     │  │      Network        │ │
│  │   Loader    │  │    Logic    │  │      Client         │ │
│  │  (GRF/SPR)  │  │  (Entities) │  │    (Hercules)       │ │
│  └──────┬──────┘  └──────┬──────┘  └──────────┬──────────┘ │
│         │                │                     │            │
│  ┌──────▼────────────────▼─────────────────────▼──────────┐ │
│  │                    ENGINE CORE                          │ │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌────────┐  │ │
│  │  │ Renderer │  │  Input   │  │  Audio   │  │  Time  │  │ │
│  │  │ (OpenGL) │  │  (SDL2)  │  │  (SDL2)  │  │        │  │ │
│  │  └──────────┘  └──────────┘  └──────────┘  └────────┘  │ │
│  └─────────────────────────────────────────────────────────┘ │
├─────────────────────────────────────────────────────────────┤
│                     SDL2 / OpenGL                           │
└─────────────────────────────────────────────────────────────┘
```

### 6.2 Main Game Loop

```go
func (g *Game) Run() {
    for g.running {
        // 1. Process input
        g.input.Update()
        
        // 2. Process network
        g.network.ProcessPackets()
        
        // 3. Update game state
        dt := g.time.Delta()
        g.world.Update(dt)
        g.entities.Update(dt)
        g.ui.Update(dt)
        
        // 4. Render
        g.renderer.Begin()
        g.world.Render()
        g.entities.Render()
        g.ui.Render()
        g.renderer.End()
        
        // 5. Present
        g.window.Swap()
    }
}
```

---

## 7. Milestones

### Milestone 1: Window & Triangle (Week 1)
- [x] SDL2 window creation
- [x] OpenGL context initialization
- [ ] Render a colored triangle
- [ ] Basic game loop with timing

### Milestone 2: Textured Rendering (Week 2)
- [ ] Texture loading (BMP/TGA)
- [ ] Sprite rendering with transparency
- [ ] Camera system (orthographic)

### Milestone 3: GRF & Sprites (Week 3-4)
- [ ] GRF archive reader
- [ ] SPR/ACT parser
- [ ] Animated sprite playback
- [ ] Test with character sprites

### Milestone 4: Map Rendering (Week 5-6)
- [ ] GAT file parser (walkability)
- [ ] GND file parser (ground mesh)
- [ ] Basic map rendering
- [ ] Camera controls

### Milestone 5: Network Foundation (Week 7-8)
- [ ] TCP client
- [ ] Packet serialization
- [ ] Login flow implementation
- [ ] Character selection

### Milestone 6: Game World (Week 9-10)
- [ ] RSW parser (map objects)
- [ ] Entity spawning from server
- [ ] Player movement
- [ ] Basic mob display

### Milestone 7: Polish (Week 11-12)
- [ ] UI framework
- [ ] Loading screens
- [ ] Error handling
- [ ] Performance optimization

---

## 8. Risks & Mitigations

| Risk | Probability | Impact | Mitigation |
|------|-------------|--------|------------|
| Hercules protocol complexity | High | High | Reference rust-ro and existing clients |
| GRF encryption issues | Medium | Medium | Use documented format, test early |
| OpenGL learning curve | Medium | Medium | Start simple, iterate |
| Time constraints | High | High | Strict MVP scope, weekly reviews |

---

## 9. Success Metrics

### 9.1 Functional
- All login states work without errors
- 4 maps load and render correctly
- Player can walk around maps
- Mobs spawn and move

### 9.2 Performance
- 60 FPS on M1 MacBook Air
- Memory usage under 500MB
- No memory leaks after 1 hour

### 9.3 Code Quality
- 80%+ test coverage for parsers
- All exported functions documented
- No lint warnings

---

## 10. Appendices

### A. Hercules Server Setup
See [docs/research/hercules-setup.md](../research/hercules-setup.md)

### B. GRF Format Specification
See [docs/research/grf-format.md](../research/grf-format.md)

### C. Packet Documentation
See [docs/research/packets.md](../research/packets.md)

---

## 11. External References

### Primary Documentation

| Resource | URL | Purpose |
|----------|-----|--------|
| Ragnarok Research Lab | https://ragnarokresearchlab.github.io/ | Authoritative file format docs |
| Korangar (Rust client) | https://github.com/vE5li/korangar | Reference implementation |
| Hercules Wiki | https://herc.ws/wiki/ | Server protocol docs |

### File Format References

| Format | Documentation | Reference Code |
|--------|--------------|----------------|
| SPR (Sprites) | [Research Lab - SPR](https://ragnarokresearchlab.github.io/file-formats/spr/) | Korangar `ragnarok-formats` |
| ACT (Animations) | [Research Lab - ACT](https://ragnarokresearchlab.github.io/file-formats/act/) | Korangar `ragnarok-formats` |
| GRF (Archives) | [Research Lab - GRF](https://ragnarokresearchlab.github.io/file-formats/grf/) | Korangar `ragnarok-formats` |
| GAT (Walkability) | [Research Lab - GAT](https://ragnarokresearchlab.github.io/file-formats/gat/) | Korangar `ragnarok-formats` |
| GND (Ground) | [Research Lab - GND](https://ragnarokresearchlab.github.io/file-formats/gnd/) | Korangar `ragnarok-formats` |
| RSW (Map Resources) | [Research Lab - RSW](https://ragnarokresearchlab.github.io/file-formats/rsw/) | Korangar `ragnarok-formats` |
| PAL (Palettes) | [Research Lab - PAL](https://ragnarokresearchlab.github.io/file-formats/pal/) | Korangar `ragnarok-formats` |

### Network Protocol References

| Resource | URL | Notes |
|----------|-----|-------|
| Korangar Packets | https://github.com/vE5li/korangar/tree/main/ragnarok-packets | Rust packet definitions |
| rust-ro Server | https://github.com/nmeylan/rust-ro | Server-side packet handling |
| Hercules Source | https://github.com/HerculesWS/Hercules | Authoritative packet source |

### Community

| Resource | URL |
|----------|-----|
| Korangar Discord | https://discord.gg/2CqRZsvKja |
| Research Lab Discord | https://discord.gg/7RFdMNrySy |
| Hercules Board | https://board.herc.ws/ |

### Graphics & Engine Learning

| Resource | URL | Purpose |
|----------|-----|--------|
| LearnOpenGL | https://learnopengl.com | OpenGL tutorials |
| go-sdl2 | https://github.com/veandco/go-sdl2 | SDL2 Go bindings |
| go-gl | https://github.com/go-gl/gl | OpenGL Go bindings |

---

**Note**: See [docs/research/korangar-and-community.md](../research/korangar-and-community.md) for detailed analysis of these resources.
