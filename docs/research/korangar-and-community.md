# Research: Korangar & RO Community Resources

**Date**: January 9, 2025  
**Purpose**: Reference for Midgard RO development

---

## 1. Korangar - Next-Gen RO Client

**Repository**: https://github.com/vE5li/korangar  
**Language**: Rust  
**Graphics**: Vulkan  
**License**: MIT

### 1.1 What is it?

Korangar is a modern RO client that's further along than us. Key features:
- Real-time lighting with drop shadows
- Day/night cycle
- Customizable UI
- Removes original client limitations (fixed aspect ratio)
- Cross-platform: Linux, Windows, macOS

### 1.2 Project Structure (Important!)

```
korangar/
├── korangar/               # Main client application
├── korangar-audio/         # Audio system
├── korangar-collision/     # Collision detection
├── korangar-container/     # Container types
├── korangar-debug/         # Debug utilities
├── korangar-interface/     # UI system
├── korangar-loaders/       # Asset loaders (uses ragnarok-formats)
├── korangar-networking/    # Network client
├── korangar-video/         # Video rendering
│
│   # REUSABLE CRATES (No Korangar dependencies!)
├── ragnarok-bytes/         # Byte serialization utilities
├── ragnarok-formats/       # RO file format parsers (SPR, ACT, GAT, etc.)
├── ragnarok-macros/        # Rust macros
├── ragnarok-packets/       # Network packet definitions
│
└── wiki/                   # Documentation
```

### 1.3 What We Can Learn

**Architecture Decisions:**
1. **Separation of reusable code**: `ragnarok-*` crates have zero dependency on Korangar
2. **This is exactly what we planned with `pkg/`** - validation of our approach!
3. **Modular crates** for different concerns (audio, collision, UI, etc.)

**Technical References:**
1. Their `ragnarok-formats` crate has all file parsers - we can study the logic
2. Their `ragnarok-packets` has Hercules packet definitions
3. Their `ragnarok-bytes` shows binary parsing patterns

### 1.4 Key Differences from Our Approach

| Aspect | Korangar | Midgard RO |
|--------|----------|------------|
| Language | Rust | Go |
| Graphics | Vulkan | OpenGL 4.1 |
| Complexity | High (production-grade) | Medium (educational) |
| Learning curve | Steeper | Gentler |

**Why we're still doing our own:**
1. **Educational value** - Building from scratch teaches more
2. **Go ecosystem** - Different strengths than Rust
3. **Simpler graphics** - OpenGL is more accessible than Vulkan
4. **AI showcase** - Demonstrating Claude Code capabilities

---

## 2. Ragnarok Research Lab

**Website**: https://ragnarokresearchlab.github.io/  
**Discord**: https://discord.gg/7RFdMNrySy

### 2.1 What is it?

A community documentation project for RO file formats, rendering, and game mechanics.

### 2.2 Available Documentation

| Format | Description | Status |
|--------|-------------|--------|
| ACT | Animation data | Documented |
| SPR | Sprite images | Documented |
| GAT | Map walkability | Documented |
| GND | Ground mesh | Documented |
| RSW | Map resources/objects | Documented |
| GRF | Archive format | Documented |
| PAL | Color palettes | Documented |
| RSM | 3D models | Placeholder |
| EBM | Unknown | Documented |
| GR2 | Granny 3D models | Documented |

### 2.3 SPR Format Summary

```
SPR files contain:
- Indexed-color bitmaps (BMP segment)
- Truecolor images with alpha (TGA segment)
- Color palettes (256 colors)

Versions:
- 1.1: Arcturus only (no TGA)
- 2.0: BMP + TGA segments
- 2.1: RLE compression for BMP segment (current)

Key points:
- Palette index 0 = transparent background
- RLE compresses runs of zero bytes
- TGA pixels are ABGR format
```

---

## 3. Other Useful Projects

### 3.1 rust-ro (Server Implementation)

**Repository**: https://github.com/nmeylan/rust-ro

A Rust-based RO server (not client). Useful for understanding:
- Packet structure and versioning
- Server architecture
- Database schema

### 3.2 rpatchur (GRF Tools)

**Repository**: https://github.com/L1nkZ/rpatchur

Contains `gruf` - a GRF/THOR archive library in Rust. Useful reference for:
- GRF parsing
- THOR patch format
- Archive building

### 3.3 RagnarokFileFormats Archive

**Repository**: https://github.com/rdw-archive/RagnarokFileFormats

Detailed documentation on RO file formats. Note: Now redirects to Ragnarok Research Lab.

---

## 4. Recommendations for Midgard RO

### 4.1 Use as Reference (Don't Copy!)

1. **Study Korangar's format parsers** for logic, implement in Go yourself
2. **Use Research Lab docs** as authoritative source for file formats
3. **Check rust-ro** for packet definitions and server behavior

### 4.2 Our Advantages

1. **Go's simplicity** - Faster iteration than Rust
2. **AI-assisted development** - Claude Code handles boilerplate
3. **Educational focus** - Can document learning journey
4. **OpenGL** - More tutorials and simpler than Vulkan

### 4.3 Priority References

For each milestone, reference these in order:

**Milestone 3 (GRF & Sprites):**
1. Ragnarok Research Lab - GRF format
2. Ragnarok Research Lab - SPR format
3. Korangar's `ragnarok-formats` - Implementation reference

**Milestone 4 (Maps):**
1. Ragnarok Research Lab - GAT, GND, RSW
2. Korangar's `ragnarok-formats` - Implementation reference

**Milestone 5 (Networking):**
1. Korangar's `ragnarok-packets` - Packet definitions
2. rust-ro's packet handling
3. Hercules source code

---

## 5. Links

### Documentation
- https://ragnarokresearchlab.github.io/file-formats/

### Source Code References
- https://github.com/vE5li/korangar (Rust client)
- https://github.com/nmeylan/rust-ro (Rust server)
- https://github.com/L1nkZ/rpatchur (GRF tools)

### Community
- Korangar Discord: https://discord.gg/2CqRZsvKja
- Research Lab Discord: https://discord.gg/7RFdMNrySy
- Hercules Board: https://board.herc.ws/

---

*This document should be updated as we discover new resources.*
