# ADR-008: ACT Format Parser

## Status
Accepted

## Context

Ragnarok Online uses ACT (Action) files to define animation metadata for all game entities. ACT files work in conjunction with SPR files - SPR provides the sprite images, ACT defines how to animate them.

### ACT Format Overview

ACT files contain:
- **Actions**: Animation sequences (idle, walk, attack, etc.)
- **Frames**: Individual animation frames within each action
- **Layers**: Sprite layers composited per frame (for equipment, effects)
- **Anchor Points**: Attachment points for equipment sprites
- **Events**: Sound effects and damage triggers
- **Intervals**: Animation timing per action

## Decision

### 1. Package Structure

```
pkg/formats/
├── act.go          # ACT parser and types
├── act_test.go     # Tests
├── spr.go          # SPR parser (existing)
└── testdata/
    ├── generate_act.go  # Test data generator
    └── test.act         # Generated test file
```

### 2. ACT File Format

#### Header (16 bytes)
```
Offset  Size  Field
------  ----  -----
0       2     Magic ("AC")
2       2     Version (0x200 - 0x205)
4       2     Action Count
6       10    Reserved (unused)
```

Note: Version bytes are reversed - `05 02` means version 2.5, not 5.2.

#### Version History

| Version | Features Added |
|---------|---------------|
| 0x200 | Base format with actions, frames, layers |
| 0x201 | Event support (sound effects) |
| 0x202 | Animation intervals per action |
| 0x203 | Anchor points for equipment |
| 0x204 | Separate X/Y scaling per layer |
| 0x205 | Layer width/height fields |

We will support all versions 0x200 - 0x205.

#### Action Structure
```
Offset  Size  Field
------  ----  -----
0       4     Frame Count
4       var   Frames[]
```

#### Frame Structure
```
Offset  Size  Field
------  ----  -----
0       16    Range1 (attack range, unused)
16      16    Range2 (fit range, unused)
32      4     Layer Count
36      var   Layers[]
var     4     Event ID (-1 = none) [v0x200+]
var     4     Anchor Point Count [v0x203+]
var     var   Anchor Points[] [v0x203+]
```

#### Layer Structure (version dependent size)
```
Offset  Size  Field
------  ----  -----
0       4     X offset
4       4     Y offset
8       4     Sprite Index (-1 = invalid)
12      4     Flags (bit 0 = mirror Y)
16      4     Color RGBA tint
20      4     X Scale (float)
24      4     Y Scale (float) [v0x204+, else same as X]
28      4     Rotation (degrees, float)
32      4     Sprite Type (0=indexed, 1=RGBA)
36      4     Width [v0x205+]
40      4     Height [v0x205+]
```

#### Anchor Point Structure (16 bytes)
```
Offset  Size  Field
------  ----  -----
0       4     Unknown/padding
4       4     X offset
8       4     Y offset
12      4     Attribute
```

#### Event Structure (40 bytes)
```
Offset  Size  Field
------  ----  -----
0       40    Name (null-terminated string)
```

#### Intervals (at end of file)
```
For v0x202+: 4 bytes (float) per action
```

### 3. API Design

```go
package formats

// ACT represents a parsed animation file.
type ACT struct {
    Version   ACTVersion
    Actions   []Action
    Events    []string   // Event names (sound files, "atk", etc.)
    Intervals []float32  // Animation delay per action (ms)
}

// ACTVersion represents the ACT file version.
type ACTVersion uint16

// Action represents an animation sequence.
type Action struct {
    Frames []Frame
}

// Frame represents a single animation frame.
type Frame struct {
    Layers       []Layer
    EventID      int32        // -1 = no event
    AnchorPoints []AnchorPoint
}

// Layer represents a sprite layer in a frame.
type Layer struct {
    X          int32
    Y          int32
    SpriteID   int32   // -1 = invalid/skip
    Flags      uint32  // bit 0 = mirror Y
    Color      [4]uint8 // RGBA tint
    ScaleX     float32
    ScaleY     float32
    Rotation   float32 // degrees
    SpriteType int32   // 0=indexed, 1=RGBA
    Width      int32   // v0x205+
    Height     int32   // v0x205+
}

// AnchorPoint represents an attachment point for equipment.
type AnchorPoint struct {
    X         int32
    Y         int32
    Attribute int32
}

// ParseACT parses an ACT file from raw bytes.
func ParseACT(data []byte) (*ACT, error)

// ParseACTFile parses an ACT file from disk.
func ParseACTFile(path string) (*ACT, error)

// IsMirrored returns true if the layer should be Y-mirrored.
func (l *Layer) IsMirrored() bool
```

### 4. Design Decisions

#### Version-Aware Parsing
Parse according to the file version, handling missing fields gracefully:
- Pre-v0x204: Use X scale for both X and Y
- Pre-v0x205: Set width/height to 0 (use sprite dimensions)
- Pre-v0x203: Empty anchor points
- Pre-v0x202: Empty intervals (use default timing)
- Pre-v0x201: Empty events

#### Sprite Type Mapping
Layer's SpriteType field indicates which SPR image array to use:
- Type 0: Indexed-color images (from SPR BMP segment)
- Type 1: True-color images (from SPR TGA segment)

Since our SPR parser converts all to RGBA, we just use SpriteID as index.

#### Coordinate System
RO uses screen coordinates (Y increases downward). Keep as-is for consistency with game logic.

### 5. Error Handling

```go
var (
    ErrInvalidACTMagic       = errors.New("invalid ACT magic: expected 'AC'")
    ErrUnsupportedACTVersion = errors.New("unsupported ACT version")
    ErrTruncatedACTData      = errors.New("truncated ACT data")
)
```

## Implementation Plan

### Phase 1: Core Parser
1. Header parsing with version detection
2. Action/Frame/Layer parsing
3. Version-specific field handling

### Phase 2: Extended Features
1. Anchor point parsing (v0x203+)
2. Event parsing (v0x201+)
3. Interval parsing (v0x202+)

### Phase 3: Testing
1. Synthetic test data for each version
2. Test data generator
3. Integration tests with real ACT files

## Consequences

### Positive
- Can animate all game entities
- Supports all known ACT versions
- Clean integration with SPR parser

### Negative
- Complex version handling
- Some fields (Range1, Range2) unused but must be parsed

## References

- [RagnarokFileFormats - ACT.MD](https://github.com/rdw-archive/RagnarokFileFormats/blob/master/ACT.MD)
- [z0q - ACT Format](https://z0q.neocities.org/ragnarok-online-formats/act/)
- [rAthena Wiki - Acts](https://github.com/rathena/rathena/wiki/Acts)
