# RSM Model Positioning Investigation Plan

## Background

During ADR-013 Stage 4 implementation, we discovered edge cases where some RSM models appear:
1. **Floating** - positioned above the terrain
2. **Upside down** - rotated 180 degrees incorrectly
3. **Underground** - positioned below terrain surface

## Current Implementation

The current approach uses bounding box centering:
- Calculate model bounding box from transformed vertices
- Subtract centerX/centerZ for horizontal centering
- Subtract minY to put model base at ground level

This works for ~90% of models but has edge cases.

## Known Issues

### 1. Floating Objects
**Symptoms**: Lamp posts, signs, decorative elements float in air
**Possible causes**:
- Multi-node models with child nodes having large position offsets
- Models designed to attach to other objects (e.g., signs on buildings)
- NodeName field in RSW not being used (specifies attachment node)

### 2. Upside-Down Models
**Symptoms**: Some trees/objects rendered upside down
**Possible causes**:
- Rotation matrix determinant issues (negative = flip)
- Y-negation combined with certain rotations
- Node rotation axis interpretation differences

### 3. Underground Objects
**Symptoms**: Walls/structures partially below ground
**Possible causes**:
- Models with anchor points not at base
- Altitude calculation differences
- Multi-part models with separate positioning

## Investigation Tasks

### Phase 1: Data Analysis
- [ ] Log problematic model names and their transforms
- [ ] Compare RSW position/rotation/scale with final world position
- [ ] Identify patterns in problematic models (file names, node counts, etc.)

### Phase 2: Reference Implementation Study
- [ ] Study RoBrowser RSM rendering code
- [ ] Study RagnarokRebuild model loader (Unity implementation)
- [ ] Document differences with our implementation

### Phase 3: NodeName Investigation
- [ ] Implement NodeName field usage from RSW
- [ ] Test if NodeName specifies anchor point for positioning
- [ ] Test models with and without NodeName

### Phase 4: Rotation Analysis
- [ ] Check rotation matrix determinants for problem models
- [ ] Test different rotation application orders (XYZ vs ZYX, etc.)
- [ ] Compare with model_viewer.go rotation handling

### Phase 5: Multi-Node Handling
- [ ] Analyze parent-child hierarchy for problem models
- [ ] Check if orphan nodes should be excluded
- [ ] Test rendering only root-connected nodes

## Resources

- [RagnarokFileFormats RSM.MD](https://github.com/Duckwhale/RagnarokFileFormats/blob/master/RSM.MD)
- [RoBrowser Source Code](https://github.com/nicop83/roBrowserLegacy)
- [RagnarokRebuild Model Loader](https://github.com/Doddler/RagnarokRebuild)

## Test Maps

- **Prontera** (prontera.rsw) - Has floating archway decorations
- **Alberta** (alberta.rsw) - Has upside-down trees, floating objects

## Success Criteria

- All visible models positioned correctly on terrain
- No floating or underground objects
- Correct orientation for all model types
