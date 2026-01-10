# UC-007: GRF Error Handling

## Description
Tests error handling for various failure scenarios when working with GRF archives. Ensures the library fails gracefully with informative error messages.

## Preconditions
- Access to filesystem for creating test scenarios

## Test Steps

### Scenario 1: Non-existent File
1. Call `grf.Open("nonexistent.grf")`
2. Verify error is returned
3. Verify error message indicates file not found

### Scenario 2: Invalid GRF Magic
1. Create file with invalid magic bytes
2. Attempt to open with `grf.Open()`
3. Verify error indicates "invalid GRF magic"

### Scenario 3: Unsupported Version
1. (If test file available) Open GRF with version != 0x200
2. Verify error indicates unsupported version

### Scenario 4: Read Non-existent File
1. Open valid GRF
2. Call `archive.Read("nonexistent/file.txt")`
3. Verify error indicates "file not found"

### Scenario 5: Encrypted Files
1. (If test file available) Attempt to read encrypted file
2. Verify error indicates "encrypted files not yet supported"

## Expected Results
- All error conditions return non-nil error
- Error messages are descriptive and actionable
- No panics or crashes
- Resources are cleaned up properly on errors

## Priority
High

## Related
- PRD Section: 5.1 GRF Extraction
- ADR: ADR-006-grf-archive-reader.md
- Code: `/Users/borisglebov/git/Faultbox/midgard-ro/pkg/grf/grf.go`
- Test: `/Users/borisglebov/git/Faultbox/midgard-ro/pkg/grf/grf_test.go::TestOpenInvalidFile`, `TestReadNonExistent`

## Known Limitations
- Encrypted files are not supported (returns error)
- Only GRF version 0x200 is supported
