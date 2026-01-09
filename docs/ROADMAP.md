# Midgard RO - Development Roadmap

## Month 1: Asset Pipeline + Basic Rendering

**Strategy:** Asset Tools First - build tooling foundation, then rendering.

---

### Week 1: GRF & Format Foundation
**Output:** CLI tool to list/extract GRF contents

| Task | Output |
|------|--------|
| Implement GRF reader (`pkg/grf`) | Read file index, decompress entries |
| Basic format parsers | SPR (sprites), ACT (animations) |
| CLI tool `cmd/grftool` | `grftool list data.grf`, `grftool extract sprite.spr` |

**Technical Details:**
- GRF is a custom archive format (similar to ZIP)
- Uses ZLIB compression for entries
- File table at end of archive
- Version 0x200 is most common

---

### Week 2: Sprite Viewer Tool
**Output:** GUI window displaying RO sprites with animation

| Task | Output |
|------|--------|
| SPR/ACT renderer | Load sprite sheets, play animation frames |
| Simple viewer UI | `cmd/sprviewer` - open .spr, see animation |
| Palette support | RO uses indexed colors with .pal files |

**Technical Details:**
- SPR contains indexed/RGBA images
- ACT defines animation frames, timing, anchor points
- Palette files (.pal) for color mapping

---

### Week 3: Map Formats & Ground Rendering
**Output:** Render walkable ground mesh from GAT/GND

| Task | Output |
|------|--------|
| GAT parser | Ground altitude + cell types (walkable/water/etc) |
| GND parser | Ground mesh with texture coords |
| Basic 3D camera | WASD movement, mouse look |
| Ground renderer | Textured ground tiles from GND |

**Technical Details:**
- GAT: 2D grid of altitude values + cell flags
- GND: 3D mesh with texture coordinates, lightmaps
- Maps are typically 256x256 to 512x512 cells

---

### Week 4: Map Viewer Tool
**Output:** Walk around RO maps with basic rendering

| Task | Output |
|------|--------|
| RSW parser | Map metadata, object placement |
| RSM parser (basic) | 3D model format for props |
| Map viewer `cmd/mapviewer` | Load map, render ground + props |
| Water rendering | Animated water planes |

**Technical Details:**
- RSW: "Resource World" - ties together GAT, GND, models, lights
- RSM: Hierarchical 3D models with textures
- Water uses animated texture scrolling

---

## Deliverables Summary

| Week | Tool/Feature | User Can... |
|------|--------------|-------------|
| 1 | `grftool` | Browse/extract game archives |
| 2 | `sprviewer` | View sprites + animations |
| 3 | Ground renderer | See textured terrain |
| 4 | `mapviewer` | Explore RO maps in 3D |

---

## Month 2: Game Client Foundation (Planned)

- Character rendering (sprites on map)
- Basic UI framework
- Network protocol (login server)
- Input handling for movement

---

## Month 3: Playable Prototype (Planned)

- Connect to Hercules server
- Character selection
- Walk around maps
- See other players/NPCs

---

## File Format Reference

| Format | Extension | Purpose |
|--------|-----------|---------|
| GRF | .grf | Archive container |
| SPR | .spr | Sprite images |
| ACT | .act | Animation data |
| PAL | .pal | Color palettes |
| GAT | .gat | Ground altitude/walkability |
| GND | .gnd | Ground mesh/textures |
| RSW | .rsw | Map resource file |
| RSM | .rsm | 3D models |
| STR | .str | Effects |
| IMF | .imf | Input mapping |

---

## Resources

- [Hercules Wiki](https://herc.ws/wiki/)
- [OpenKore GRF Documentation](https://openkore.com/wiki/)
- [rAthena File Formats](https://rathena.org/)
