# UC-003: GRF File Existence Check

## Description
Tests the `Contains()` method for checking if a file exists in the archive without reading it. Essential for conditional asset loading and error prevention.

## Preconditions
- Valid GRF archive is open
- Archive contains known test files

## Test Steps
1. Open GRF archive
2. Call `archive.Contains("data/test.txt")` with known file (should return true)
3. Call `archive.Contains("data\\test.txt")` with backslash path (should return true - normalized)
4. Call `archive.Contains("DATA/TEST.TXT")` with uppercase path (should return true - case insensitive)
5. Call `archive.Contains("nonexistent/file.txt")` with non-existent file (should return false)

## Expected Results
- Existing files return `true`
- Non-existent files return `false`
- Path normalization works (backslash to forward slash)
- Case insensitivity works (uppercase/lowercase treated the same)

## Priority
High

## Related
- PRD Section: 5.1 GRF Extraction
- ADR: ADR-006-grf-archive-reader.md
- Code: `/Users/borisglebov/git/Faultbox/midgard-ro/pkg/grf/grf.go::Contains()`
- Test: `/Users/borisglebov/git/Faultbox/midgard-ro/pkg/grf/grf_test.go::TestContains`

## Edge Cases to Test
- Empty string path
- Path with multiple consecutive slashes
- Path with ./ or ../ components
- Very long file paths
