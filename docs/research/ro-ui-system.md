# Ragnarok Online UI System Research

This document contains research on the original Ragnarok Online UI system, asset locations in GRF files, and a proposed implementation plan for creating an authentic RO-style interface in Midgard.

## Table of Contents

1. [UI Elements Overview](#1-ui-elements-overview)
2. [GRF Asset Structure](#2-grf-asset-structure)
3. [File Format Requirements](#3-file-format-requirements)
4. [Visual Design Principles](#4-visual-design-principles)
5. [Implementation Stages](#5-implementation-stages)

---

## 1. UI Elements Overview

### 1.1 Core Windows

| Window | Shortcut | Purpose |
|--------|----------|---------|
| **Basic Info** | ALT+V | Character name, class, level, HP/SP, weight, Zeny |
| **Status** | ALT+A | Character stats (STR, AGI, VIT, INT, DEX, LUK) |
| **Equipment** | ALT+Q | Equipped items display with paper doll |
| **Inventory** | ALT+E | Item management with tabs (Usable, Equip, Misc, Favorites) |
| **Skill Tree** | ALT+S | Skill points allocation and skill icons |
| **Quest Log** | ALT+U | Active quests and progress tracking |
| **Options** | ALT+O | Game settings (audio, video, controls) |

### 1.2 Social Windows

| Window | Shortcut | Purpose |
|--------|----------|---------|
| **Party** | ALT+Z | Party member list with HP bars |
| **Guild** | ALT+G | Guild info, members, emblem |
| **Friends** | ALT+H | Friend list with online status |
| **Chat** | F10/ALT+F10 | Chat input and message display |
| **Chat Room** | ALT+C | Create/join chat rooms |

### 1.3 Special Windows

| Window | Shortcut | Purpose |
|--------|----------|---------|
| **Pet** | ALT+J | Pet stats and intimacy (pet owners only) |
| **Homunculus** | ALT+R | Homunculus management (Alchemist only) |
| **Mercenary** | CTRL+R | Mercenary companion info |
| **Cart** | ALT+W | Cart inventory (Merchants only) |
| **Kafra Storage** | NPC | Extended storage (240 slots) |
| **Vending** | - | Shop setup interface (Merchants) |

### 1.4 HUD Elements (Always Visible)

| Element | Description |
|---------|-------------|
| **HP/SP Bars** | Health and skill points display |
| **Minimap** | Toggle with CTRL+Tab, shows terrain and entities |
| **Shortcut Bar** | F1-F9 quick slots for skills/items (F12 to configure) |
| **Chat Box** | Message history and input field |
| **Compass** | Cardinal directions indicator |
| **Clock** | In-game time display |
| **Entity HP Bars** | Floating bars above monsters/players |

### 1.5 Special Interfaces

| Interface | Trigger | Description |
|-----------|---------|-------------|
| **Login Screen** | Game start | Server selection, username/password |
| **Character Select** | After login | Character slots (3-12), create/delete |
| **Character Creation** | New char | Class, appearance, stats, name |
| **Loading Screen** | Map change | Progress bar, map image/tips |
| **NPC Dialog** | NPC click | Dialog boxes with choices |
| **Shop Window** | NPC/Vendor | Buy/sell item interface |
| **Trade Window** | Player trade | Item exchange interface |
| **Dead/Respawn** | On death | Respawn options |
| **Warp Portal Select** | Skill use | Destination selection |

### 1.6 Effects and Feedback

| Element | Description |
|---------|-------------|
| **Damage Numbers** | Floating damage/heal values |
| **Level Up Effect** | Visual celebration on level gain |
| **Skill Casting Bar** | Progress indicator for cast times |
| **Status Icons** | Buff/debuff indicators |
| **Emotes** | Character emotion animations |
| **System Messages** | Server announcements, errors |

---

## 2. GRF Asset Structure

### 2.1 Main UI Texture Directory

```
data/texture/유저인터페이스/
├── basic_interface/     # Core UI window frames
├── cardbmp/            # Card illustrations
├── collection/         # Collection book images
├── item/               # Item icons (24x24 and 75x100)
├── illust/             # Character illustrations
├── map/                # Minimap images
├── login_interface/    # Login screen elements
└── ...
```

**Note:** Korean path `유저인터페이스` appears as `À¯ÀúÀÎÅÍÆäÀÌ½º` in EUC-KR encoding.

### 2.2 Detailed Directory Structure

#### `basic_interface/` - Window Frames and Common UI
```
basic_interface/
├── dialframe.bmp       # Dialog window frame
├── dialback.bmp        # Dialog window background
├── btn_ok.bmp          # OK button
├── btn_cancel.bmp      # Cancel button
├── btn_close.bmp       # Close button (X)
├── scrollbar.bmp       # Scrollbar elements
├── checkbox.bmp        # Checkbox states
├── radio.bmp           # Radio button states
├── tab.bmp             # Tab elements
├── progressbar.bmp     # Progress bar
├── hpbar.bmp           # HP bar texture
├── spbar.bmp           # SP bar texture
├── expbar.bmp          # Experience bar
└── window_*.bmp        # Various window frames
```

#### `item/` - Item Icons
```
item/
├── apple.bmp           # Consumable icon (24x24)
├── sword.bmp           # Equipment icon
├── potion_red.bmp      # Potion icon
└── ...                 # Thousands of item icons
```

#### `illust/` - Character Illustrations
```
illust/
├── swordman.bmp        # Job change illustrations
├── mage.bmp
├── archer.bmp
├── merchant.bmp
├── thief.bmp
├── acolyte.bmp
└── ...
```

#### `map/` - Minimap Images
```
map/
├── prontera.bmp        # City minimaps
├── prt_fild01.bmp      # Field minimaps
├── prt_maze01.bmp      # Dungeon minimaps
└── ...
```

#### `login_interface/` - Login Screen
```
login_interface/
├── ro_logo.bmp         # Game logo
├── login_window.bmp    # Login form background
├── btn_connect.bmp     # Connect button
├── btn_exit.bmp        # Exit button
├── server_select.bmp   # Server list frame
└── ...
```

### 2.3 Sprite Locations (Effects, Cursors)

```
data/sprite/
├── cursors/            # Mouse cursor animations
│   ├── cursor.spr
│   └── cursor.act
├── 이팩트/              # Effects (Korean: 이팩트)
│   ├── levelup.spr     # Level up effect
│   ├── damage.spr      # Damage numbers font
│   └── ...
└── npc/                # NPC sprites for dialogs
```

### 2.4 Configuration Files

```
data/
├── lua files/
│   └── datainfo/
│       ├── iteminfo.lub        # Item descriptions
│       ├── skillinfo.lub       # Skill descriptions
│       ├── npcidentity.lub     # NPC names
│       └── ...
└── texture/
    └── scr_logo.bmp            # Screenshot logo overlay
```

---

## 3. File Format Requirements

### 3.1 BMP Format Specifications

| Property | Requirement |
|----------|-------------|
| **Format** | BMP (Windows Bitmap) |
| **Color Depth** | 8-bit indexed (palette) |
| **Colors** | 255 max (NOT 256!) |
| **Compression** | None (uncompressed) |
| **Alpha** | No alpha channel |
| **Transparency** | First palette color = transparent (typically magenta #FF00FF) |

**ImageMagick conversion command:**
```bash
convert input.png -type palette -colors 255 -depth 8 -compress none BMP3:output.bmp
```

### 3.2 TGA Format (for 32-bit with Alpha)

| Property | Requirement |
|----------|-------------|
| **Format** | TGA (Targa) |
| **Color Depth** | 32-bit RGBA |
| **Compression** | RLE or uncompressed |
| **Use Case** | UI elements requiring smooth transparency |

### 3.3 Standard Texture Sizes

| Asset Type | Dimensions |
|------------|------------|
| Item Icons (small) | 24 × 24 px |
| Item Icons (large) | 75 × 100 px |
| Card Images | 75 × 100 px |
| Character Illustrations | 256 × 512 px (varies) |
| Window Frames | Variable, 9-slice compatible |
| Minimap | Matches map dimensions |
| Buttons | Variable, typically 80 × 20 px |

### 3.4 Transparency Color

The standard transparency color is **Magenta (#FF00FF)** or the **first color in the palette**. This color will not be rendered in-game.

---

## 4. Visual Design Principles

### 4.1 RO UI Aesthetic Characteristics

1. **Frame Style**: Dark brown/gold ornate borders with fantasy motifs
2. **Background**: Semi-transparent dark panels (50-70% opacity)
3. **Text**: White/yellow primary, gray secondary, red for warnings
4. **Buttons**: 3D beveled appearance with hover/press states
5. **Icons**: Pixel art style, limited palette, clear silhouettes
6. **Color Palette**: Earth tones (brown, gold, tan) with accent colors

### 4.2 Window Composition (9-Slice)

RO windows use 9-slice scaling for flexible sizing:

```
┌─────┬───────────┬─────┐
│ TL  │    TOP    │ TR  │  <- Corners don't stretch
├─────┼───────────┼─────┤
│     │           │     │
│LEFT │  CENTER   │RIGHT│  <- Sides stretch in one direction
│     │           │     │
├─────┼───────────┼─────┤
│ BL  │  BOTTOM   │ BR  │  <- Center stretches both ways
└─────┴───────────┴─────┘
```

### 4.3 Interactive Elements

**Button States:**
- Normal: Default appearance
- Hover: Slightly brighter/highlighted
- Pressed: Darker, shifted down 1-2px
- Disabled: Grayed out, no interaction

**Scrollbar:**
- Track (background)
- Thumb (draggable element)
- Up/Down arrows

### 4.4 Font Requirements

- **Primary Font**: Arial/MS Gothic style (bitmap)
- **Damage Numbers**: Custom sprite-based numerals
- **Chat Font**: System font or embedded bitmap font
- **Size**: 10-12px for most UI text

---

## 5. Implementation Stages

### Stage 1: Foundation (Week 1-2)
**Goal:** Load and display GRF UI textures

**Tasks:**
- [ ] Create UI texture loader in `internal/assets/`
- [ ] Add support for 8-bit BMP with palette transparency
- [ ] Implement texture atlas generation for UI elements
- [ ] Create basic UI texture cache
- [ ] Test loading from `data/texture/유저인터페이스/`

**Deliverables:**
- UI texture loading from GRF works
- Transparency rendering works correctly
- Basic atlas system for batching

### Stage 2: Window System (Week 3-4)
**Goal:** Implement skinnable window framework

**Tasks:**
- [ ] Design 9-slice window renderer
- [ ] Create base `Window` struct with:
  - Title bar
  - Close/minimize buttons
  - Dragging support
  - Resizing (optional per window)
- [ ] Implement window manager (z-order, focus)
- [ ] Add window open/close animations
- [ ] Create `Skin` system to load different UI themes

**Deliverables:**
- Draggable windows with RO-style frames
- Multiple windows can be open simultaneously
- Windows can be minimized/closed

### Stage 3: Core Widgets (Week 5-6)
**Goal:** Implement essential UI widgets

**Tasks:**
- [ ] **Button**: Normal, hover, pressed, disabled states
- [ ] **Label**: Text rendering with alignment
- [ ] **TextInput**: Editable text field with cursor
- [ ] **Scrollbar**: Vertical/horizontal with drag
- [ ] **ListView**: Scrollable item list
- [ ] **ProgressBar**: HP/SP/EXP bars
- [ ] **Checkbox/Radio**: Toggle controls
- [ ] **TabControl**: Tabbed panels
- [ ] **Tooltip**: Hover information

**Deliverables:**
- All core widgets functional
- Widgets use GRF textures
- Consistent look and feel

### Stage 4: Login & Character Select (Week 7-8)
**Goal:** Complete pre-game UI flow

**Tasks:**
- [ ] **Login Screen**:
  - RO logo and background
  - Username/password fields
  - Server selection list
  - Connect/Exit buttons
- [ ] **Character Select**:
  - Character slot display (sprites)
  - Character info panel
  - Create/Delete/Select buttons
- [ ] **Character Creation**:
  - Class selection
  - Appearance customization (hair style/color)
  - Stat point allocation
  - Name input

**Deliverables:**
- Full login flow with authentic visuals
- Character creation wizard
- Smooth transitions between screens

### Stage 5: In-Game HUD (Week 9-10)
**Goal:** Essential gameplay interface

**Tasks:**
- [ ] **Basic Info Window** (ALT+V): HP/SP/Level/Zeny
- [ ] **Shortcut Bar**: F1-F9 skill/item slots
- [ ] **Chat Box**: Message display and input
- [ ] **Minimap**: Toggleable map overlay
- [ ] **Entity HP Bars**: Floating bars above entities
- [ ] **Damage Numbers**: Floating damage/heal values
- [ ] **Casting Bar**: Skill cast progress

**Deliverables:**
- Functional gameplay HUD
- All HUD elements positioned correctly
- Keyboard shortcuts work

### Stage 6: Major Windows (Week 11-14)
**Goal:** Complete primary game windows

**Tasks:**
- [ ] **Inventory** (ALT+E):
  - Tabbed view (Usable, Equip, Misc, Favorites)
  - Item icons with quantity
  - Item tooltip on hover
  - Drag-drop support
- [ ] **Equipment** (ALT+Q):
  - Paper doll display
  - Equipment slots
  - Stats summary
- [ ] **Skill Tree** (ALT+S):
  - Skill icons with levels
  - Point allocation
  - Drag to shortcut bar
- [ ] **Status** (ALT+A):
  - Base/job level
  - Stat points display
  - Stat allocation buttons

**Deliverables:**
- Four major windows fully functional
- Item drag-drop works
- Skill system integrated

### Stage 7: NPC & Interaction (Week 15-16)
**Goal:** NPC dialogs and interactions

**Tasks:**
- [ ] **NPC Dialog Box**:
  - Portrait display
  - Text with word wrapping
  - Next/Close buttons
  - Choice selection
- [ ] **Shop Window**:
  - Item list with prices
  - Buy/Sell tabs
  - Quantity selection
- [ ] **Trade Window**:
  - Two-panel item exchange
  - Zeny input
  - Confirm/Cancel

**Deliverables:**
- NPC interaction complete
- Shop buying/selling works
- Player trading works

### Stage 8: Social Features (Week 17-18)
**Goal:** Multiplayer social UI

**Tasks:**
- [ ] **Party Window** (ALT+Z): Member list with HP
- [ ] **Guild Window** (ALT+G): Guild info, emblem
- [ ] **Friends List** (ALT+H): Online status
- [ ] **Chat Rooms** (ALT+C): Room creation/joining
- [ ] **Whisper System**: Private messages

**Deliverables:**
- All social windows functional
- Real-time updates from server

### Stage 9: Polish & Effects (Week 19-20)
**Goal:** Visual polish and effects

**Tasks:**
- [ ] **Level Up Effect**: Particle celebration
- [ ] **Status Icons**: Buff/debuff display
- [ ] **Emotes**: /emote command support
- [ ] **System Messages**: Announcement display
- [ ] **Sound Effects**: UI click/open/close sounds
- [ ] **Cursor**: Custom cursor from GRF
- [ ] **Loading Screens**: Map loading with tips

**Deliverables:**
- Polished, authentic RO feel
- All visual feedback complete
- Audio feedback integrated

---

## 6. Current Codebase Status

### Existing UI Infrastructure

The project already has two UI backends:

1. **ImGui Backend** (`internal/game/ui/imgui_backend.go`)
   - Development/debug UI
   - Quick prototyping

2. **UI2D Backend** (`internal/game/ui/ui2d_backend.go`)
   - Custom OpenGL renderer
   - More control for authentic styling

### Existing UI Components

- `login_ui.go` - Basic login form
- `charselect_ui.go` - Character selection
- `ingame_ui.go` - In-game HUD elements
- `debug_overlay.go` - Performance metrics

### Recommended Approach

**Option A: Extend UI2D Backend**
- Add GRF texture loading to existing system
- Implement 9-slice rendering
- Gradually replace procedural UI with textured UI

**Option B: Create Dedicated RO UI System**
- New package: `internal/engine/roui/`
- Purpose-built for RO's specific needs
- Clean separation from debug UI

**Recommendation:** Option A for faster iteration, with gradual migration to purpose-built components.

---

## 7. References

### Official Resources
- [iRO Wiki - Basic Game Control](https://irowiki.org/wiki/Basic_Game_Control)
- [iRO Wiki - Skins](https://irowiki.org/wiki/Skins)
- [StrategyWiki - RO Interface](https://strategywiki.org/wiki/Ragnarok_Online/Interface)

### Community Resources
- [rAthena Wiki - GRF](https://github.com/rathena/rathena/wiki/GRF)
- [rAthena - Basics of Ragnarok Arting](https://rathena.org/board/topic/140768-tutorial-basics-of-ragnarok-arting-adding-custom-items/)
- [GRF Editor by Tokeiburu](https://github.com/Tokeiburu/GRFEditor)

### Design References
- [Dribbble - RO UI Elements](https://dribbble.com/shots/20537412-Ragnarok-Online-UI-Elements)
- [Steam Guide - RO Interface](https://steamcommunity.com/sharedfiles/filedetails/?id=190870754)

---

## 8. Appendix: Korean Path Reference

| Korean | Romanization | English |
|--------|--------------|---------|
| 유저인터페이스 | yujeointeopeiseu | User Interface |
| 이팩트 | ipaekteu | Effect |
| 인간족 | ingangjok | Human Race |
| 몬스터 | monseuteo | Monster |
| 워터 | woteo | Water |
| 몸통 | momtong | Body/Torso |
| 머리통 | meoritong | Head |

---

*Document created: 2026-01-20*
*Author: Claude Code (AI Assistant)*
