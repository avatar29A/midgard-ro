# UC-004: GRF File Read - Uncompressed

## Description
Tests reading uncompressed files from a GRF archive. Some files in GRF archives are stored without compression.

## Preconditions
- Valid GRF archive is open
- Archive contains uncompressed test file

## Test Steps
1. Open GRF archive
2. Call `archive.Read("data/test.txt")` to read uncompressed text file
3. Verify no error is returned
4. Verify returned byte slice matches expected content
5. Verify file size matches expected size

## Expected Results
- `Read()` returns byte slice without error
- Content matches expected data: "Hello, GRF!"
- No decompression errors (file is uncompressed)

## Priority
Critical

## Related
- PRD Section: 5.1 GRF Extraction
- ADR: ADR-006-grf-archive-reader.md
- Code: `/Users/borisglebov/git/Faultbox/midgard-ro/pkg/grf/grf.go::Read()`
- Test: `/Users/borisglebov/git/Faultbox/midgard-ro/pkg/grf/grf_test.go::TestRead`

## Edge Cases to Test
- Empty file (0 bytes)
- Large uncompressed file
- Binary file (non-text)
