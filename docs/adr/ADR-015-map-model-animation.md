# ADR-015: Map Model Animation System

## Status
Proposed

## Context

The grfbrowser 3D map viewer currently renders RSM models statically, using only keyframe 0 for animated models. Many RO maps contain animated objects (windmills, flags, fountains, hanging signs, etc.) that should animate in real-time.

### Current State
- **model_viewer.go**: Full animation support with mesh rebuild per frame
- **map_viewer.go**: Static rendering, uses `node.RotKeys[0]` for animated nodes
- Maps can contain 1000+ models, many with animation keyframes

### Requirements
1. Enable animation for models on the map
2. Default: animation enabled
3. Per-model animation control (enable/disable, manual frame scrubbing)
4. Performant with hundreds of animated models
5. Future: Play mode with character sprite walking

## Decision

### Stage 1: Map Animation (Mesh Rebuild Approach)

Use selective mesh rebuild for animated models, similar to model_viewer but optimized for maps.

#### Architecture

```
MapViewer
├── globalAnimTime float32        // Global animation clock (ms)
├── animationEnabled bool         // Master animation toggle
├── animatedModels []*MapModel    // Subset of models with animation
│
MapModel (extended)
├── hasAnimation bool             // Cached flag for animation presence
├── animLength int32              // From RSM.AnimLength
├── animEnabled bool              // Per-model toggle (default: true)
├── animTime float32              // Per-model time (for manual control)
├── useGlobalTime bool            // Use global vs per-model time
├── rsm *formats.RSM              // Reference to RSM data for rebuild
└── textureLoader func            // Cached texture loader
```

#### Animation Update Flow

```
Per Frame:
1. Advance globalAnimTime by deltaMs
2. For each animated model where animEnabled:
   a. If useGlobalTime: time = globalAnimTime % animLength
   b. Else: time = animTime
   c. Rebuild mesh with interpolated keyframes
   d. Re-upload to GPU
```

#### Transform Function Changes

Modify `buildNodeMatrixRecursive` to accept `animTimeMs float32`:

```go
func (mv *MapViewer) buildNodeMatrixRecursive(
    node *formats.RSMNode,
    rsm *formats.RSM,
    animTimeMs float32,  // NEW: animation time
    visited map[string]bool,
) math.Mat4
```

Add interpolation functions (copy from model_viewer):
- `interpolateRotKeys(keys, timeMs) Quat`
- `interpolatePosKeys(keys, timeMs) [3]float32`
- `interpolateScaleKeys(keys, timeMs) [3]float32`

### Stage 2: Play Mode

Rename "FPS Camera" to "Play" mode with RO-style features:

#### Camera System
- Fixed isometric angle (RO default ~45 degrees)
- Camera follows character sprite
- Optional: zoom in/out, but fixed angle

#### Character System
- Load player sprite (SPR/ACT format)
- Click-to-move: pathfind to clicked position
- Walking animation with direction detection
- Movement speed matching RO (~150 units/second)

#### Implementation
```
PlayMode
├── enabled bool
├── characterSprite *Sprite       // SPR/ACT loaded sprite
├── characterPos Vec3             // Current position
├── targetPos Vec3                // Click destination
├── moveSpeed float32             // Units per second
├── facing int                    // Direction (0-7 for 8 directions)
└── walkingAnim bool              // Currently moving
```

## Consequences

### Positive
- Animated maps look alive (windmills spin, flags wave)
- Debug tools for investigating animation issues
- Foundation for full game client Play mode

### Negative
- Mesh rebuild is CPU-intensive for many animated models
- Increased memory usage (store RSM reference per animated model)
- GPU upload overhead per animated model per frame

### Performance Considerations

| Approach | Pros | Cons |
|----------|------|------|
| Mesh Rebuild | Simple, matches model_viewer | CPU-bound for many models |
| GPU Skinning | Efficient, single upload | Complex shader, bone limit |
| Instancing | Good for identical models | Doesn't help unique anims |

**Decision**: Start with mesh rebuild (simpler), optimize later if needed. Most maps have <50 animated models, which is manageable.

### Optimization Opportunities (Future)

1. **LOD-based animation**: Only animate visible/close models
2. **Frame skipping**: Animate at 30fps even if render at 60fps
3. **Batched uploads**: Group animated models, single draw call
4. **GPU compute**: Move interpolation to compute shader

## UI Design

### Controls Panel Additions

```
Animation
├── [x] Enable Animation (master toggle)
├── Speed: [====|====] 1.0x
├── Time: 1234 / 5000 ms
└── Animated Models: 47 / 1304

Selected Model Properties
├── [x] Animate
├── Time: [====|====] (manual scrub)
├── [ ] Use Global Time
└── Keyframes: Rot(24) Pos(0) Scale(0)
```

## Implementation Plan

### Stage 1 Tasks
1. Add animation state to MapModel struct
2. Identify and cache animated models during load
3. Add animation time tracking to MapViewer
4. Port interpolation functions from model_viewer
5. Modify buildNodeMatrixRecursive for animation time
6. Add rebuildAnimatedMesh() function
7. Update render loop to advance animation and rebuild
8. Add UI controls for animation

### Stage 2 Tasks
1. Create PlayMode struct and state management
2. Implement RO-style camera (fixed angle, follow character)
3. Load character sprite (SPR/ACT)
4. Implement click-to-move with pathfinding
5. Add walking animation with direction detection
6. Integrate with GAT walkability data

## Testing

### Animated Models to Test
- Windmills (prontera, geffen)
- Flags (various towns)
- Fountains (prontera)
- Hanging signs (aldebaran)
- Water wheels
- Smoke/steam effects

### Test Cases
1. Animation plays smoothly (no stuttering)
2. Animation speed control works
3. Per-model enable/disable works
4. Manual time scrubbing works
5. Animation loops correctly at AnimLength boundary
6. Multiple animated models don't conflict

## References

- ADR-014: RSM Model Transform Order
- model_viewer.go animation implementation
- korangar RSM animation
- RO client animation behavior

## Revision History

| Date | Change |
|------|--------|
| 2026-01-15 | Initial proposal |
