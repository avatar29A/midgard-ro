# UC-005: GRF File Read - Compressed (zlib)

## Description
Tests reading compressed files from a GRF archive. Most files are stored with zlib compression to reduce archive size.

## Preconditions
- Valid GRF archive is open
- Archive contains compressed test files (e.g., SPR, BMP files)

## Test Steps
1. Open GRF archive
2. Call `archive.Read("data/sprite/test.spr")` to read compressed sprite file
3. Verify no error is returned
4. Verify returned data is properly decompressed
5. Check that decompressed data has valid SPR magic ("SP")

## Expected Results
- `Read()` successfully decompresses file
- No decompression errors
- Decompressed data is valid (SPR magic bytes present)
- Data size matches expected uncompressed size

## Priority
Critical

## Related
- PRD Section: 5.1 GRF Extraction, 5.2 File Formats (SPR)
- ADR: ADR-006-grf-archive-reader.md
- Code: `/Users/borisglebov/git/Faultbox/midgard-ro/pkg/grf/grf.go::Read()`
- Test: `/Users/borisglebov/git/Faultbox/midgard-ro/pkg/grf/grf_test.go::TestReadSprite`

## Edge Cases to Test
- Corrupted compressed data (should error)
- File with invalid compression flag
- Very large compressed file (performance)
