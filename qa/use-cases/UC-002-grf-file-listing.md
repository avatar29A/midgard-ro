# UC-002: GRF File Listing

## Description
Tests the ability to list all files contained in a GRF archive. This operation is essential for discovering available assets and verifying archive contents.

## Preconditions
- Valid GRF archive is open
- Archive contains known test files

## Test Steps
1. Open GRF archive using `grf.Open()`
2. Call `archive.List()` to get all file paths
3. Verify the returned slice is non-empty
4. Check that expected files are present in the list
5. Verify file paths are normalized (forward slashes, lowercase)

## Expected Results
- `List()` returns a slice of strings
- All files in the archive are included
- Paths are normalized to forward slashes
- Paths are lowercase
- No duplicate entries

## Priority
High

## Related
- PRD Section: 5.1 GRF Extraction
- ADR: ADR-006-grf-archive-reader.md
- Code: `/Users/borisglebov/git/Faultbox/midgard-ro/pkg/grf/grf.go::List()`
- Test: `/Users/borisglebov/git/Faultbox/midgard-ro/pkg/grf/grf_test.go::TestList`

## Test Data (from test.grf)
Expected files:
- `data/test.txt`
- `data/sprite/test.spr`
- `data/texture/test.bmp`
- `data/subfolder/nested/file.txt`

## Edge Cases to Test
- Empty GRF archive (should return empty slice)
- Archive with only directories, no files (should return empty slice)
- Very large archive (performance check)
