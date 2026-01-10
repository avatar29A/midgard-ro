# UC-011: Vec3 Basic Operations

## Description
Tests 3D vector mathematical operations (Add, Sub, Scale, Dot, Cross, Length, Normalize, Distance, XZ). Essential for 3D positioning, camera math, and lighting calculations.

## Preconditions
- None (pure math operations)

## Test Steps

### Cross Product
1. Create `x := Vec3{1, 0, 0}` (X axis)
2. Create `y := Vec3{0, 1, 0}` (Y axis)
3. Call `z := x.Cross(y)`
4. Verify `z == Vec3{0, 0, 1}` (Z axis)

### Cross Product (Anticommutative)
1. Create `x := Vec3{1, 0, 0}` and `y := Vec3{0, 1, 0}`
2. Call `a := x.Cross(y)` and `b := y.Cross(x)`
3. Verify `a == Vec3{0, 0, 1}` and `b == Vec3{0, 0, -1}`

### XZ Projection
1. Create `v := Vec3{5, 10, 15}`
2. Call `xz := v.XZ()`
3. Verify `xz == Vec2{5, 15}` (Y component dropped)

### 3D Length
1. Create `v := Vec3{2, 3, 6}`
2. Call `length := v.Length()`
3. Verify `length == 7.0` (sqrt(4+9+36))

### 3D Normalize
1. Create `v := Vec3{2, 3, 6}`
2. Call `normalized := v.Normalize()`
3. Call `length := normalized.Length()`
4. Verify `length` is approximately 1.0 (within 0.001)

### 3D Distance
1. Create `a := Vec3{0, 0, 0}` and `b := Vec3{2, 3, 6}`
2. Call `dist := a.Distance(b)`
3. Verify `dist == 7.0`

## Expected Results
- All operations produce mathematically correct results
- Cross product follows right-hand rule
- XZ projection correctly drops Y component
- No panics or errors
- Zero-length vector normalization handled safely

## Priority
High

## Related
- PRD Section: 4.3 World Rendering, 4.3.2 Rendering Features (lighting)
- Code: `/Users/borisglebov/git/Faultbox/midgard-ro/pkg/math/vec3.go`
- Test: `/Users/borisglebov/git/Faultbox/midgard-ro/pkg/math/vec_test.go::TestVec3*`

## Use Cases
- 3D object positions
- Camera position and direction
- Light direction vectors
- Normal vectors for surfaces
- Map coordinate conversion (XZ ground plane)
