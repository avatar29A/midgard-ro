# ADR-007: SPR Format Parser

## Status
Accepted

## Context

Ragnarok Online uses SPR (Sprite) files to store all character, monster, NPC, and item sprites. To render entities in our client, we need to parse these files and extract the image data.

### SPR Format Overview

SPR files are binary sprite sheets containing:
- **Indexed-color images** (BMP segment) - use a 256-color palette
- **True-color images** (TGA segment) - ABGR format with alpha channel
- **Color palette** - 256 RGBA colors at end of file

SPR files are tightly coupled with ACT (animation) files. SPR provides the images, ACT defines how to animate them.

## Decision

### 1. Package Structure

```
pkg/formats/
├── spr.go          # SPR parser and types
├── spr_test.go     # Tests with fixture files
└── act.go          # ACT parser (future, separate ADR)
```

### 2. SPR File Format

#### Header (All Versions)
```
Offset  Size  Field
------  ----  -----
0       2     Magic ("SP")
2       1     Version Minor
3       1     Version Major
4       2     Indexed Image Count
6       2     True-Color Image Count (v2.0+)
```

Note: Version bytes are reversed - `01 02` means version 2.1, not 1.2.

#### Version Differences

| Version | Features |
|---------|----------|
| 1.0 | Indexed images only, system palette |
| 1.1 | Indexed images + embedded palette |
| 2.0 | Adds true-color (RGBA) images |
| 2.1 | RLE compression for indexed images |

We will support versions 1.1, 2.0, and 2.1 (most common).

#### Indexed Image Structure (v1.1/2.0)
```
Offset  Size       Field
------  ----       -----
0       2          Width
2       2          Height
4       W*H        Pixel indices (palette lookup)
```

#### Indexed Image Structure (v2.1 - RLE Compressed)
```
Offset  Size       Field
------  ----       -----
0       2          Width
2       2          Height
4       2          Compressed Size
6       var        RLE-compressed pixel data
```

RLE Encoding:
- `0x00 0xNN` = NN transparent pixels
- `0x00 0x00` = single `0x00` byte
- Other bytes = palette index as-is

#### True-Color Image Structure
```
Offset  Size       Field
------  ----       -----
0       2          Width
2       2          Height
4       W*H*4      ABGR pixel data
```

#### Palette (1024 bytes at EOF)
```
256 entries × 4 bytes = 1024 bytes
Each entry: R, G, B, A (but A is often ignored)
Index 0 = transparent (regardless of palette value)
```

### 3. API Design

```go
package formats

// SPR represents a parsed sprite file.
type SPR struct {
    Version     Version
    Images      []Image  // All images (indexed converted to RGBA)
    Palette     *Palette // Original palette (nil for pure TGA sprites)
}

// Version represents SPR file version.
type Version struct {
    Major uint8
    Minor uint8
}

// Image represents a single sprite image.
type Image struct {
    Width  uint16
    Height uint16
    Pixels []byte  // RGBA format, 4 bytes per pixel
}

// Palette represents a 256-color palette.
type Palette struct {
    Colors [256]Color
}

// Color represents an RGBA color.
type Color struct {
    R, G, B, A uint8
}

// ParseSPR parses an SPR file from raw bytes.
func ParseSPR(data []byte) (*SPR, error)

// ParseSPRFile parses an SPR file from disk.
func ParseSPRFile(path string) (*SPR, error)
```

### 4. Design Decisions

#### Convert to RGBA on Parse
Instead of storing indexed and true-color images separately, we convert all images to RGBA format during parsing:
- Simplifies renderer (one format to handle)
- Palette lookup done once at load time
- Matches OpenGL texture format

#### Palette Index 0 = Transparent
Per RO convention, palette index 0 is always treated as transparent, regardless of the palette's actual color value at index 0.

#### Invalid Images
Images with dimensions (-1, -1) or (0, 0) are marked as blank and should be discarded or replaced with a 1x1 transparent pixel.

### 5. Error Handling

```go
var (
    ErrInvalidMagic    = errors.New("invalid SPR magic: expected 'SP'")
    ErrUnsupportedVersion = errors.New("unsupported SPR version")
    ErrTruncatedData   = errors.New("truncated SPR data")
    ErrInvalidImageSize = errors.New("invalid image dimensions")
)
```

### 6. Performance Considerations

- Parse lazily if sprite files are large? No - RO sprites are typically small (< 1MB)
- RLE decompression is simple and fast
- Memory: ~4 bytes per pixel (RGBA) after conversion

## Implementation Plan

### Phase 1: Core Parser
1. Header parsing with version detection
2. Palette parsing (last 1024 bytes)
3. Indexed image parsing (v1.1/2.0)
4. RLE decompression (v2.1)
5. True-color image parsing

### Phase 2: Testing
1. Unit tests with synthetic data
2. Integration tests with real SPR files from GRF
3. Test all supported versions

### Phase 3: Integration
1. Load SPR via GRF reader
2. Create OpenGL textures from images
3. Wire up to entity rendering

## Consequences

### Positive
- Can render all game entities (characters, monsters, NPCs)
- Clean API that hides format complexity
- Unified RGBA output simplifies rendering

### Negative
- Memory overhead from palette-to-RGBA conversion
- Version 1.0 not supported (rare, uses system palette)

## References

- [Ragnarok Research Lab - SPR Format](https://ragnarokresearchlab.github.io/file-formats/spr/)
- [RagnarokFileFormats - SPR.MD](https://github.com/rdw-archive/RagnarokFileFormats/blob/master/SPR.MD)
- [rAthena Wiki - Spriting](https://github.com/rathena/rathena/wiki/Spriting)
