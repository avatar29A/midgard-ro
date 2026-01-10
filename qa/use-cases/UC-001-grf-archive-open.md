# UC-001: GRF Archive Open and Header Validation

## Description
Tests the ability to open a GRF archive file, validate its header, and extract basic metadata. This is the foundational operation for all GRF-based asset loading.

## Preconditions
- Valid GRF archive file exists (e.g., `pkg/grf/testdata/test.grf`)
- Go 1.22+ installed
- No file permissions issues

## Test Steps
1. Call `grf.Open()` with path to valid GRF file
2. Verify no error is returned
3. Check that `archive.header.Magic` equals "Master of Magic"
4. Check that `archive.header.Version` equals `0x200`
5. Verify `archive.fileList` is populated (non-empty)
6. Call `archive.Close()` and verify no error

## Expected Results
- Archive opens successfully without errors
- Header magic matches expected value
- Version is 0x200 (GRF v2.0)
- File list contains expected number of entries
- Close operation succeeds without errors

## Priority
Critical

## Related
- PRD Section: 5.1 GRF Extraction
- ADR: ADR-006-grf-archive-reader.md
- Code: `/Users/borisglebov/git/Faultbox/midgard-ro/pkg/grf/grf.go`
- Test: `/Users/borisglebov/git/Faultbox/midgard-ro/pkg/grf/grf_test.go::TestOpen`

## Edge Cases to Test
- Opening non-existent file (should error)
- Opening file with invalid magic (should error)
- Opening file with unsupported version (should error)
- Opening file without read permissions (should error)
