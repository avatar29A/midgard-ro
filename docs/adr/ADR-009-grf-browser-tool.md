# ADR-009: GRF Browser Tool

## Status
Accepted

## Context

We need a comprehensive tool to browse, view, and eventually modify GRF archive contents. This tool will:
1. Validate our file format parsers (SPR, ACT, and future formats)
2. Provide a development/debugging tool for asset inspection
3. Serve as a foundation for modding tools

### Inspiration from Existing Solutions

| Tool | Key Features |
|------|--------------|
| Unity Project Window | Dual-pane, search with filters, preview panel, favorites |
| Unreal Content Browser | Breadcrumbs, thumbnails, property inspector, collections |
| Godot FileSystem | Tree + inspector, quick filter, drag-drop |
| GRF Editor (RO) | Tree view, extract/add files, sprite preview |
| actOR2 (RO) | SPR/ACT editing, frame-by-frame preview |

## Decision

### 1. Technology Stack

| Component | Choice | Rationale |
|-----------|--------|-----------|
| GUI | [Dear ImGui](https://github.com/ocornut/imgui) via [cimgui-go](https://github.com/AllenDang/cimgui-go) | Industry standard, immediate mode, highly customizable |
| Window/Input | SDL2 | Already in use, cross-platform |
| Rendering | OpenGL 4.1 | Already in use, texture support |

### 2. Application Layout

```
┌─────────────────────────────────────────────────────────────────────────┐
│  GRF Browser - data.grf                                         [─][□][×]│
├────────────────────────┬────────────────────────────────────────────────┤
│ ┌────────────────────┐ │ ┌────────────────────────────────────────────┐ │
│ │ 🔍 Search...       │ │ │              Preview Panel                 │ │
│ ├────────────────────┤ │ │                                            │ │
│ │ Filter: [All ▼]    │ │ │     ┌──────────────────────────┐          │ │
│ │ ☑ Sprites (.spr)   │ │ │     │                          │          │ │
│ │ ☑ Animations (.act)│ │ │     │    [Animated Sprite]     │          │ │
│ │ ☑ Textures (.bmp)  │ │ │     │                          │          │ │
│ │ ☑ Models (.rsm)    │ │ │     └──────────────────────────┘          │ │
│ │ ☑ Maps (.rsw)      │ │ │                                            │ │
│ │ ☐ Audio (.wav)     │ │ │  ◀ Action 5/56 ▶   ◀ Frame 2/4 ▶          │ │
│ │ ☐ Other            │ │ │  [▶ Play] [⏸ Pause]  Speed: [1.0x ▼]      │ │
│ ├────────────────────┤ │ ├────────────────────────────────────────────┤
│ │ 📁 data            │ │ │              Properties Panel              │ │
│ │  ├─📁 sprite       │ │ │ ┌──────────────────────────────────────┐  │ │
│ │  │  ├─📁 npc       │ │ │ │ File: duckling.spr                   │  │ │
│ │  │  │  ├─🖼 duck...│ │ │ │ Size: 34,098 bytes                   │  │ │
│ │  │  │  └─🎬 duck...│ │ │ │ Version: 2.1                         │  │ │
│ │  │  └─📁 monster   │ │ │ │ Images: 47                           │  │ │
│ │  ├─📁 texture      │ │ │ │ Palette: Yes (256 colors)            │  │ │
│ │  └─📁 wav          │ │ │ └──────────────────────────────────────┘  │ │
│ └────────────────────┘ │ └────────────────────────────────────────────┘ │
├────────────────────────┴────────────────────────────────────────────────┤
│ 18,841 files │ Filter: 2,456 sprites │ Selected: duckling.spr          │
└─────────────────────────────────────────────────────────────────────────┘
```

### 3. Core Features

#### 3.1 File Tree Panel (Left)
- **Hierarchical tree view** of GRF contents
- **Virtual folders** from file paths (data/sprite/npc/...)
- **Icons** by file type (folder, sprite, animation, texture, etc.)
- **Lazy loading** for large archives (18k+ files)
- **Keyboard navigation**: Arrow keys, Enter to expand/select
- **Mouse support**: Click to select, double-click to expand/preview

#### 3.2 Search & Filter
- **Live search** with `%like%` pattern matching
- **Type filters** as checkboxes:
  - Sprites (.spr)
  - Animations (.act)
  - Textures (.bmp, .tga, .jpg)
  - Models (.rsm)
  - Maps (.rsw, .gat, .gnd)
  - Audio (.wav, .mp3)
  - Other
- **Results highlighting** in tree
- **Search history** (last 10 searches)

#### 3.3 Preview Panel (Right Top)
File-type specific viewers:

| Type | Viewer |
|------|--------|
| .spr | Sprite viewer with frame navigation |
| .spr + .act | Animated sprite with playback controls |
| .bmp/.tga | Image viewer with zoom |
| .wav | Audio player (play/stop) |
| .txt/.xml | Text viewer |
| Other | Hex dump preview |

#### 3.4 Properties Panel (Right Bottom)
Metadata display:
- File path, size, compression ratio
- Format-specific info (SPR version, image count, etc.)
- Timestamps (if available)

#### 3.5 Status Bar
- Total file count
- Filtered file count
- Current selection
- Loading progress

### 4. Controls

#### Keyboard
| Key | Action |
|-----|--------|
| Ctrl+O | Open GRF |
| Ctrl+F | Focus search |
| Ctrl+E | Extract selected |
| ↑/↓ | Navigate tree |
| ←/→ | Collapse/Expand folder |
| Enter | Select/Preview |
| Space | Play/Pause animation |
| +/- | Zoom preview |
| Escape | Clear search / Close dialog |

#### Mouse
- Click: Select
- Double-click: Expand folder / Open viewer
- Right-click: Context menu (Extract, Copy path, etc.)
- Scroll: Navigate tree / Zoom preview
- Drag splitter: Resize panels

### 5. Development Stages

#### Stage 1: Foundation (MVP)
**Goal**: Load GRF, display tree, basic navigation

- [ ] ImGui + SDL2 + OpenGL integration
- [ ] Open GRF dialog
- [ ] Tree view with virtual folders
- [ ] Basic file list (no icons yet)
- [ ] Keyboard navigation (↑↓←→)
- [ ] Status bar with file count

#### Stage 2: Search & Filter
**Goal**: Find files efficiently

- [ ] Search input with live filtering
- [ ] Type filter checkboxes
- [ ] Result count display
- [ ] Search history dropdown
- [ ] Highlight matching items

#### Stage 3: Sprite Viewer
**Goal**: Preview SPR/ACT files

- [ ] SPR loading and texture creation
- [ ] Single frame display
- [ ] Frame navigation (←→)
- [ ] ACT loading
- [ ] Animation playback with timing
- [ ] Action navigation
- [ ] Play/Pause/Speed controls

#### Stage 4: Extended Viewers
**Goal**: Preview more file types

- [ ] Image viewer (.bmp, .tga)
- [ ] Text viewer (.txt, .xml, .lua)
- [ ] Hex viewer (fallback)
- [ ] Audio player (.wav)
- [ ] Properties panel for all types

#### Stage 5: Polish
**Goal**: Production-ready UX

- [ ] File type icons
- [ ] Recent files list
- [ ] Favorites/Bookmarks
- [ ] Drag splitters for panel resize
- [ ] Keyboard shortcuts overlay (?)
- [ ] Preferences (theme, default zoom, etc.)

#### Stage 6: Modification (Future)
**Goal**: Edit and save GRF contents

- [ ] Extract single file
- [ ] Extract with folder structure
- [ ] Extract filtered results
- [ ] Add new files to GRF
- [ ] Replace existing files
- [ ] Delete files
- [ ] Create new GRF
- [ ] Save modified GRF

### 6. Package Structure

```
cmd/grfbrowser/
├── main.go              # Entry point, argument parsing
├── app.go               # Application state and main loop
├── ui/
│   ├── ui.go            # ImGui setup and main layout
│   ├── tree.go          # File tree panel
│   ├── search.go        # Search and filter panel
│   ├── preview.go       # Preview panel router
│   ├── properties.go    # Properties panel
│   └── dialogs.go       # Open file, extract, etc.
└── viewers/
    ├── sprite.go        # SPR/ACT viewer
    ├── image.go         # BMP/TGA viewer
    ├── text.go          # Text file viewer
    ├── hex.go           # Hex dump viewer
    └── audio.go         # Audio player
```

### 7. Technical Considerations

#### Performance
- **Lazy tree building**: Only expand visible nodes
- **Texture caching**: LRU cache for sprite textures
- **Background loading**: Load previews in goroutine
- **Debounced search**: 100ms delay before filtering

#### Memory
- **Stream large files**: Don't load entire file for preview
- **Unload hidden textures**: Free textures not in view
- **Limit preview size**: Max 2048x2048 for images

#### Cross-Platform
- cimgui-go provides pre-built binaries for:
  - Windows (x64)
  - macOS (x64, arm64)
  - Linux (x64)

## Consequences

### Positive
- Comprehensive asset browser for development
- Validates all file format parsers
- Foundation for modding tools
- Professional UX inspired by game engines

### Negative
- Adds cimgui-go dependency (CGO required)
- More complex than simple CLI tool
- Longer development time

### Mitigations
- Stage-based development allows early usable versions
- ImGui reduces UI boilerplate significantly
- Existing window/renderer code reusable

## References

- [Dear ImGui](https://github.com/ocornut/imgui)
- [cimgui-go](https://github.com/AllenDang/cimgui-go) - Go bindings
- [ImGui Demo](https://github.com/ocornut/imgui/blob/master/imgui_demo.cpp) - UI patterns
- [Unity Project Window](https://docs.unity3d.com/Manual/ProjectView.html)
- [GRF Format - ADR-006](./ADR-006-grf-archive-reader.md)
- [SPR Format - ADR-007](./ADR-007-spr-format-parser.md)
- [ACT Format - ADR-008](./ADR-008-act-format-parser.md)
