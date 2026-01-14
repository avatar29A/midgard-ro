# ADR-016: Autonomous Visual QA Pipeline

## Status
Proposed

## Date
2026-01-14

## Context

During ADR-014 development, significant time was spent on manual visual debugging:
- Taking screenshots and comparing visually
- Iterating on rendering code with trial-and-error
- Difficulty identifying specific issues (e.g., missing faces vs wrong textures)

We need an autonomous approach to:
1. Detect rendering issues automatically
2. Compare against reference implementations
3. Provide actionable diagnostics
4. Enable faster iteration on visual fixes

## Problem Statement

Current workflow:
```
1. Make code change
2. Run application
3. Navigate to test map
4. Take screenshot manually
5. Compare visually with expected result
6. Identify issue (often unclear)
7. Repeat
```

This is:
- **Slow**: Each iteration takes several minutes
- **Error-prone**: Visual comparison misses subtle issues
- **Non-reproducible**: Different camera angles, lighting
- **Hard to automate**: No programmatic validation

## Research: How Other Projects Handle This

### korangar (Rust)
- Uses comprehensive logging with debug flags
- Has debug rendering modes (wireframe, normals, bounding boxes)
- No automated visual testing found

### roBrowser (JavaScript)
- Manual testing with browser devtools
- No automated visual testing

### Industry Best Practices
1. **Reference Image Comparison** - Pixel-diff against known-good renders
2. **Structured Validation** - Check vertex counts, face counts programmatically
3. **Debug Overlays** - Visual indicators for different data types
4. **Automated Screenshot Pipeline** - CI/CD screenshot generation

## Decision

Implement a multi-layer autonomous visual QA system:

### Layer 1: Structural Validation (No Rendering Required)

Validate RSM/GND/RSW data integrity before rendering:

```go
type ModelValidation struct {
    TotalFaces      int
    TwoSidedFaces   int
    DegenerateTriangles int
    MissingTextures []string
    NodeCount       int
    AnimationFrames int
}

func ValidateRSM(rsm *formats.RSM) ModelValidation {
    var v ModelValidation
    for _, node := range rsm.Nodes {
        v.NodeCount++
        for _, face := range node.Faces {
            v.TotalFaces++
            if face.TwoSide != 0 { v.TwoSidedFaces++ }
            if isDegenerate(face) { v.DegenerateTriangles++ }
        }
    }
    return v
}
```

**Use case**: Before debugging rendering, know if the model data itself is valid.

### Layer 2: Debug Rendering Modes

Add toggleable debug overlays to grfbrowser:

| Mode | Visualization | Purpose |
|------|---------------|---------|
| Normals | RGB arrows per vertex | Verify normal direction |
| Wireframe | Lines only | See mesh structure |
| UVs | Color-coded UVs | Check texture mapping |
| TwoSide | Highlight two-sided faces | Debug TwoSide handling |
| Missing | Red for missing textures | Identify texture issues |
| Depth | Grayscale depth buffer | Debug Z-fighting |

Implementation:
```go
type DebugMode int
const (
    DebugNone DebugMode = iota
    DebugNormals
    DebugWireframe
    DebugUVs
    DebugTwoSide
    DebugMissingTex
    DebugDepth
)

func (mv *MapViewer) SetDebugMode(mode DebugMode)
```

### Layer 3: Automated Screenshot Comparison

```go
// qa/visual/screenshot_test.go

func TestMapRendering(t *testing.T) {
    maps := []string{"alberta", "prontera", "geffen"}

    for _, mapName := range maps {
        t.Run(mapName, func(t *testing.T) {
            // Load map
            mv := NewMapViewer(...)
            mv.LoadMap(mapName)

            // Render from fixed camera positions
            positions := GetStandardCameraPositions()
            for i, pos := range positions {
                mv.SetCamera(pos)
                screenshot := mv.RenderToImage()

                // Compare with reference
                refPath := fmt.Sprintf("testdata/reference/%s_%d.png", mapName, i)
                diff := CompareImages(screenshot, refPath)

                if diff > threshold {
                    SaveDiffImage(screenshot, refPath, diff)
                    t.Errorf("Visual difference %f%% exceeds threshold", diff*100)
                }
            }
        })
    }
}
```

### Layer 4: Diagnostic Reports

Generate structured reports for debugging:

```go
type RenderDiagnostics struct {
    MapName         string
    TotalModels     int
    LoadedModels    int
    FailedModels    []string
    TotalFaces      int
    RenderedFaces   int
    SkippedFaces    int  // degenerate or invalid
    TexturesMissing []string
    Warnings        []string
}

func (mv *MapViewer) GetDiagnostics() RenderDiagnostics
```

### Layer 5: Reference Comparison Tool

Tool to compare our rendering against known-good client:

```bash
# Generate comparison report
./grfbrowser compare --map alberta \
    --reference ~/screenshots/original_client/ \
    --output report.html
```

Output includes:
- Side-by-side screenshots
- Pixel difference heatmap
- Structural comparison (model counts, face counts)
- List of detected issues

## Implementation Phases

### Phase 1: Structural Validation (1-2 hours)
- Add `ValidateRSM()`, `ValidateGND()` functions
- Log validation results on map load
- Print warnings for suspicious data

### Phase 2: Debug Modes (2-3 hours)
- Add debug mode toggle (keyboard shortcut)
- Implement wireframe mode first
- Add normal visualization
- Add TwoSide face highlighting

### Phase 3: Automated Testing (4-6 hours)
- Create `qa/visual/` package
- Implement image comparison
- Generate reference screenshots
- Add to CI pipeline

### Phase 4: Diagnostic CLI (2-3 hours)
- Add `--diagnose` flag to grfbrowser
- Generate JSON/HTML reports
- Include model loading statistics

## File Structure

```
qa/
├── visual/
│   ├── screenshot_test.go    # Automated tests
│   ├── compare.go            # Image comparison
│   ├── report.go             # Report generation
│   └── testdata/
│       └── reference/        # Known-good screenshots
│           ├── alberta_0.png
│           ├── alberta_1.png
│           └── ...
├── validation/
│   ├── rsm.go                # RSM validation
│   ├── gnd.go                # GND validation
│   └── rsw.go                # RSW validation
└── reports/
    └── ...                   # Generated reports
```

## Debug Mode UI

```
┌─────────────────────────────────────────────┐
│ Debug: [F1] Normal  [F2] Wire  [F3] UV      │
│        [F4] TwoSide [F5] Missing [F6] Depth │
│                                             │
│ Stats: Models: 245/250 | Faces: 12450       │
│        TwoSided: 234   | Degenerate: 5      │
└─────────────────────────────────────────────┘
```

## Consequences

### Positive
- Faster debugging iteration (minutes -> seconds)
- Reproducible test cases
- Automatic regression detection
- Clear diagnostic output
- Self-documenting rendering issues

### Negative
- Initial implementation overhead
- Need to generate/maintain reference images
- May have false positives in comparison

## Success Metrics

1. **Time to identify issue**: < 30 seconds (vs 5+ minutes currently)
2. **Regression detection**: Automated in CI
3. **Issue categorization**: Automatically identify type of problem

## Alternative Approaches Considered

### Option A: Pure Manual Testing
- **Rejected**: Too slow, not reproducible

### Option B: Unit Tests Only
- **Rejected**: Can't catch visual issues

### Option C: External Screenshot Tool
- **Partially adopted**: Combined with internal debug modes

## References

- ADR-005: QA Automation (existing framework)
- ADR-014: Visual Quality (current work)
- ADR-015: RSM Rendering Improvements (fixes to implement)

## Next Steps

1. Implement Phase 1 (structural validation) immediately
2. Add basic debug mode toggle
3. Create reference screenshots for key maps
4. Integrate into CI after manual validation
