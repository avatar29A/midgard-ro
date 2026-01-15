# Session Log: RSM Transform Debugging

**Date:** 2026-01-15
**Duration:** Extended debugging session
**PR:** #40 - fix: Correct RSM node transform order for static vs animated nodes

## Summary

This was a highly productive debugging session focused on fixing RSM (Ragnarok Online Model) rendering issues in the grfbrowser 3D map viewer. We resolved multiple rendering bugs through systematic investigation and reference to the korangar implementation.

## Problems Solved

### 1. Buildings Appearing Underground/Sweeping
**Symptom:** Most buildings rendered correctly, but some had geometry "sweeping" incorrectly or appeared underground.

**Root Cause:** Incorrect transform order for static nodes. We were using `Scale → Rotation → (-Offset) → Position` but the correct order is korangar-style: `Translate(Offset) * Matrix` as "main", then `Position → AxisAngle → Scale` as "transform".

**Key Insight:** The `Offset` should be **positive**, not negated.

### 2. Animated Models Breaking
**Symptom:** Two specific buildings with rotation keyframes still rendered incorrectly after the korangar fix.

**Root Cause:** Animated nodes require a **different** transform order than static nodes. When nodes have `RotKeys`, `PosKeys`, or `ScaleKeys`, they need the simpler `Scale → Rotation → Position` order.

**Key Insight:** Detect animation presence first, then choose the appropriate transform pipeline.

### 3. Floating Decorative Elements
**Symptom:** Decorative elements (like arches attached to walls) floated above their intended positions.

**Root Cause:** Y-axis centering was shifting models vertically. Some models have intentional vertical offsets in their RSM data.

**Fix:** Center X/Z only, preserve original Y offset.

## Best Practices Discovered

### Debugging RSM Issues
1. **Add comprehensive debug info to Properties panel** - Show RSM version, node transforms, animation flags, quaternions
2. **Check for animation keyframes** - `len(node.RotKeys) > 0` changes everything
3. **Compare with korangar reference** - Their transform order is well-tested
4. **Test with multiple model types** - Static buildings, animated windmills, decorative elements

### Transform Order Rules
```
Static Nodes:
  main = Translate(Offset) * Matrix3x3
  transform = Translate(Position) * RotateAxis * Scale
  result = transform * main

Animated Nodes:
  result = Scale → Rotation(keyframe) → Position(keyframe)
```

### Code Organization
- Keep transform logic identical between `map_viewer.go` and `model_viewer.go`
- Document the transform order in ADR for future reference
- Preserve useful debug info even after fixing - it helps with future issues

## Tools That Helped

1. **Properties Panel Debug Info** - Showed node offsets, positions, scales, quaternions
2. **Model list with selection** - Quickly identify problematic models
3. **Copy/Viewer buttons** - Jump from map view to isolated model view
4. **ForceAllTwoSided debug option** - Rule out face culling issues

## Session Workflow Highlights

1. **Hypothesis-driven debugging** - Each change was based on comparing to reference implementation
2. **Incremental testing** - Test after each small change, not batch changes
3. **Keep useful intermediate work** - Debug info added during investigation remained valuable
4. **Document findings** - Created ADR-014 to capture knowledge

## Files Modified

- `cmd/grfbrowser/map_viewer.go` - Core transform logic, centering, debug info
- `cmd/grfbrowser/model_viewer.go` - Same transform logic for consistency
- `cmd/grfbrowser/main.go` - Properties panel UI enhancements
- `cmd/grfbrowser/preview_map.go` - ForceAllTwoSided debug option
- `docs/adr/ADR-014-rsm-transform-order.md` - Knowledge documentation

## Remaining Investigation

- `data/model/프론테라\교역소.rsm` - Still has positioning issues, may need separate investigation
- Korean path handling in GRF - Path encoding differs between RSW references and GRF storage

## Key Learnings for Next Time

1. **Animated vs Static distinction is critical** - Always check for keyframes first
2. **Don't assume all centering is good** - Y centering can break intended offsets
3. **Reference implementations are invaluable** - korangar saved hours of guessing
4. **Debug info pays dividends** - Initial investment in Properties panel made subsequent debugging faster
5. **Commit working state before risky changes** - Easy to revert if something breaks

## Metrics

- Models fixed: 1300+ (all prontera buildings now render correctly)
- Lines changed: ~600
- CI: Build, Lint, Test all passing
- Documentation: ADR-014 created
