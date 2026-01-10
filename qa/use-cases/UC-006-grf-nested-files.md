# UC-006: GRF Nested Directory Files

## Description
Tests reading files from nested subdirectories within the GRF archive. Verifies that deep directory structures are handled correctly.

## Preconditions
- Valid GRF archive is open
- Archive contains files in nested directories

## Test Steps
1. Open GRF archive
2. Call `archive.Contains("data/subfolder/nested/file.txt")` to verify file exists
3. Call `archive.Read("data/subfolder/nested/file.txt")` to read the nested file
4. Verify content matches expected: "Nested file content"

## Expected Results
- Nested file is found via `Contains()`
- `Read()` successfully retrieves nested file
- Content matches expected data
- No issues with path depth

## Priority
High

## Related
- PRD Section: 5.1 GRF Extraction
- ADR: ADR-006-grf-archive-reader.md
- Code: `/Users/borisglebov/git/Faultbox/midgard-ro/pkg/grf/grf.go`
- Test: `/Users/borisglebov/git/Faultbox/midgard-ro/pkg/grf/grf_test.go::TestReadNestedFile`

## Edge Cases to Test
- Very deep nesting (e.g., 10+ levels)
- Mixed forward/backward slashes in path
- Directory names with special characters
