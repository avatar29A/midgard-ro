# ADR-006: GRF Archive Reader

## Status
Accepted

## Context

Ragnarok Online stores all game assets (sprites, textures, maps, sounds) in GRF archive files. To build a client, we need to read these archives efficiently.

### GRF Format Overview

GRF (Gravity Resource File) is a custom archive format similar to ZIP:
- Contains compressed files using zlib
- Optional DES encryption for some files
- File table stored at end of archive
- Multiple versions exist (0x102, 0x103, 0x200)

### Existing Implementation

We have a basic implementation in `pkg/grf/grf.go` that:
- Reads GRF version 0x200
- Decompresses zlib-compressed files
- Lists and extracts files
- Does NOT support encrypted files

## Decision

### 1. Package Structure

```
pkg/grf/
├── grf.go          # Main Archive type and Open/Close
├── header.go       # Header parsing
├── entry.go        # File entry type and parsing
├── decrypt.go      # DES decryption (when needed)
├── reader.go       # io.Reader implementation for streaming
└── grf_test.go     # Tests with real GRF files
```

### 2. GRF File Format (Version 0x200)

#### Header (46 bytes)
```
Offset  Size  Field
------  ----  -----
0       15    Magic ("Master of Magic")
15      15    Encryption Key (usually zeros)
30      4     File Table Offset
34      4     Scrambling Seed
38      4     Scrambled File Count
42      4     Version (0x200)
```

#### File Table
Located at `TableOffset + 46`:
```
Offset  Size  Field
------  ----  -----
0       4     Compressed Size
4       4     Uncompressed Size
8       var   zlib-compressed entry records
```

#### Entry Record (in decompressed table)
```
Offset  Size  Field
------  ----  -----
0       var   Filename (null-terminated)
var     4     Compressed Size
var+4   4     Aligned Size (8-byte boundary)
var+8   4     Uncompressed Size
var+12  1     Flags
var+13  4     Data Offset
```

#### Entry Flags
| Flag | Value | Meaning |
|------|-------|---------|
| FILE | 0x01 | Entry is a file (not directory) |
| ENCRYPTED | 0x02 | DES encrypted |
| ENCRYPTED_HEADER | 0x04 | Mixed DES + byte mangling |

### 3. API Design

```go
// Open an archive
archive, err := grf.Open("data.grf")
defer archive.Close()

// List all files
files := archive.List()

// Check if file exists
if archive.Contains("data/sprite/npc/npc.spr") {
    data, err := archive.Read("data/sprite/npc/npc.spr")
}

// Get entry metadata
entry, ok := archive.Entry("data/sprite/npc/npc.spr")
fmt.Printf("Size: %d bytes\n", entry.UncompressedSize)

// Stream large files (future)
reader, err := archive.OpenReader("data/wav/bgm.wav")
defer reader.Close()
io.Copy(audioPlayer, reader)
```

### 4. Path Handling

- RO uses Windows-style paths (`data\sprite\npc.spr`)
- Normalize to forward slashes internally
- Case-insensitive lookups (RO filesystem is case-insensitive)

### 5. Encryption Support (Deferred)

DES decryption will be added when needed:
- Most user-accessible GRFs are not encrypted
- Official kRO GRFs may have encrypted entries
- Implementation uses standard DES with known keys

### 6. Version Support

| Version | Support | Notes |
|---------|---------|-------|
| 0x102 | Deferred | Oldest format, rare |
| 0x103 | Deferred | With encryption |
| 0x200 | **Yes** | Most common |
| 0x300 | Deferred | 4GB+ archives (2024+) |

## Implementation Plan

### Phase 1: Improve Existing (Current)
1. Add comprehensive tests
2. Better error handling with wrapped errors
3. Add `Entry()` method for metadata access
4. Add file count and archive info methods

### Phase 2: CLI Tool
1. Create `cmd/grftool` with subcommands:
   - `list` - List archive contents
   - `extract` - Extract files
   - `info` - Show archive metadata

### Phase 3: Streaming (Future)
1. Implement `io.Reader` interface for large files
2. Memory-efficient extraction

### Phase 4: Encryption (When Needed)
1. DES decryption for flag 0x02
2. Mixed encryption for flag 0x04

## Consequences

### Positive
- Can read game assets from original GRF files
- Reusable library for tools and game client
- Well-documented format understanding

### Negative
- No write support (not needed for client)
- Encrypted files not supported initially

## References

- [Ragnarok Research Lab - GRF Format](https://ragnarokresearchlab.github.io/file-formats/grf/)
- [rAthena Wiki - GRF](https://github.com/rathena/rathena/wiki/GRF)
- [OpenKore Documentation](https://openkore.com/wiki/)
