# ADR-014: RSM Model Transform Order and Map Rendering

## Status
Accepted

## Context

During implementation of the 3D map viewer (ADR-013), we encountered significant issues with RSM (Ragnarok Online Model) rendering:
1. Buildings appearing underground or floating
2. Models with "sweep" artifacts (geometry stretched incorrectly)
3. Floating decorative elements
4. Animated models rendering differently from static ones

This ADR documents the correct transform order discovered through investigation of the korangar reference implementation and extensive debugging.

## Decision

### RSM Node Transform Order

RSM models have two distinct transform pipelines depending on whether the node has animation keyframes:

#### Static Nodes (No Animation Keyframes)

For nodes without `RotKeys`, `PosKeys`, or `ScaleKeys`, use the **korangar-style** transform:

```go
// Main transform: Offset + Rotation Matrix
main := Translate(node.Offset[0], node.Offset[1], node.Offset[2])
main = main.Mul(FromMat3x3(node.Matrix))

// Instance transform: Position + AxisAngle + Scale
transform := Translate(node.Position[0], node.Position[1], node.Position[2])
if node.RotAngle != 0 {
    transform = transform.Mul(RotateAxis(node.RotAxis, node.RotAngle))
}
transform = transform.Mul(Scale(node.Scale[0], node.Scale[1], node.Scale[2]))

// Final: transform * main
result = transform.Mul(main)
```

Key points:
- `Offset` is applied as **positive** translation (not negated)
- The 3x3 `Matrix` contains the node's base rotation
- `RotAngle`/`RotAxis` provide additional axis-angle rotation
- Order: Position → AxisAngle → Scale → Offset → Matrix

#### Animated Nodes (Has Keyframes)

For nodes with `RotKeys`, `PosKeys`, or `ScaleKeys`, use a **simpler direct order**:

```go
result := Identity()

// 1. Apply scale (from keyframe or default)
scale := node.Scale
if len(node.ScaleKeys) > 0 {
    scale = node.ScaleKeys[0].Scale  // Or interpolated value
}
result = result.Mul(Scale(scale[0], scale[1], scale[2]))

// 2. Apply rotation (from keyframe quaternion or matrix)
if len(node.RotKeys) > 0 {
    quat := node.RotKeys[0].Quaternion  // Or interpolated
    result = result.Mul(QuatToMat4(quat))
} else {
    result = result.Mul(FromMat3x3(node.Matrix))
}

// 3. Apply position (from keyframe or default)
position := node.Position
if len(node.PosKeys) > 0 {
    position = node.PosKeys[0].Position  // Or interpolated
}
result = result.Mul(Translate(position[0], position[1], position[2]))
```

Key points:
- Order: Scale → Rotation → Position
- Rotation keyframes use **quaternions** (XYZW format)
- The node's `Offset` and `Matrix` are **not used** when keyframes exist
- This matches how RO animates models at runtime

### Parent Chain

Both static and animated nodes multiply by their parent's transform:

```go
if node.Parent != "" && node.Parent != node.Name {
    parentNode := rsm.GetNodeByName(node.Parent)
    if parentNode != nil {
        parentMatrix := buildNodeMatrixRecursive(parentNode, rsm, visited)
        result = parentMatrix.Mul(result)
    }
}
```

Use a `visited` map to prevent infinite recursion from malformed data.

### Model Centering for Map Placement

When placing RSM models on maps:
- **Center X and Z** to align model origin with RSW position
- **Do NOT center Y** - preserve vertical offset from RSM data

```go
centerX := (minX + maxX) / 2
centerZ := (minZ + maxZ) / 2
for i := range vertices {
    vertices[i].Position[0] -= centerX
    // Y is NOT centered - preserves original vertical placement
    vertices[i].Position[2] -= centerZ
}
```

Centering Y causes issues with models that have intentional vertical offsets (decorative elements, elevated platforms).

### RSW Instance Transform

After RSM model transform, apply RSW instance transform:

```go
// RSW provides position, rotation (degrees), scale
instanceMatrix := Identity()
instanceMatrix = instanceMatrix.Mul(Translate(rsw.Position[0], rsw.Position[1], rsw.Position[2]))
instanceMatrix = instanceMatrix.Mul(RotateZ(DegreesToRadians(rsw.Rotation[2])))
instanceMatrix = instanceMatrix.Mul(RotateX(DegreesToRadians(rsw.Rotation[0])))
instanceMatrix = instanceMatrix.Mul(RotateY(DegreesToRadians(rsw.Rotation[1])))
instanceMatrix = instanceMatrix.Mul(Scale(rsw.Scale[0], rsw.Scale[1], rsw.Scale[2]))
```

## Consequences

### Positive
- Models render correctly in their intended positions
- Animated models (windmills, flags) work properly
- Decorative elements maintain correct vertical placement
- Consistent with korangar reference implementation

### Negative
- Two code paths for static vs animated nodes adds complexity
- Must detect animation presence before choosing transform path

## Debug Information

The Properties panel shows useful debug info for each model:
- RSM version, node count
- Per-node: Offset, Position, Scale, Matrix, RotAngle/RotAxis
- Animation flags: HasRotKeys, HasPosKeys, HasScaleKeys
- First rotation quaternion for animated nodes

## References

- korangar RSM implementation: https://github.com/korangar/korangar
- roBrowser RSM loader: https://github.com/nicklaus/roBrowser
- RO file format documentation: Various community resources

## Revision History

| Date | Change |
|------|--------|
| 2026-01-15 | Initial version based on debugging session |
