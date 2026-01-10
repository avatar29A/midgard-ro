# UC-010: Vec2 Basic Operations

## Description
Tests 2D vector mathematical operations (Add, Sub, Scale, Dot, Length, Normalize, Distance). These are fundamental for 2D positioning, UI layout, and map coordinates.

## Preconditions
- None (pure math operations)

## Test Steps

### Addition
1. Create `a := Vec2{1, 2}` and `b := Vec2{3, 4}`
2. Call `result := a.Add(b)`
3. Verify `result == Vec2{4, 6}`

### Subtraction
1. Create `a := Vec2{5, 7}` and `b := Vec2{2, 3}`
2. Call `result := a.Sub(b)`
3. Verify `result == Vec2{3, 4}`

### Scaling
1. Create `v := Vec2{2, 3}`
2. Call `result := v.Scale(2.0)`
3. Verify `result == Vec2{4, 6}`

### Dot Product
1. Create `a := Vec2{1, 2}` and `b := Vec2{3, 4}`
2. Call `result := a.Dot(b)`
3. Verify `result == 11` (1*3 + 2*4)

### Length
1. Create `v := Vec2{3, 4}`
2. Call `length := v.Length()`
3. Verify `length == 5.0` (Pythagorean theorem)

### Normalize
1. Create `v := Vec2{3, 4}`
2. Call `normalized := v.Normalize()`
3. Call `length := normalized.Length()`
4. Verify `length` is approximately 1.0 (within 0.001)

### Normalize Zero Vector
1. Create `v := Vec2{0, 0}`
2. Call `normalized := v.Normalize()`
3. Verify `normalized == Vec2{0, 0}` (edge case)

### Distance
1. Create `a := Vec2{0, 0}` and `b := Vec2{3, 4}`
2. Call `dist := a.Distance(b)`
3. Verify `dist == 5.0`

## Expected Results
- All operations produce mathematically correct results
- No panics or errors
- Zero-length vector normalization handled safely
- Floating point precision is acceptable (within epsilon)

## Priority
High

## Related
- PRD Section: 4.4.1 Player Character (movement), 6.2 Main Game Loop
- Code: `/Users/borisglebov/git/Faultbox/midgard-ro/pkg/math/vec2.go`
- Test: `/Users/borisglebov/git/Faultbox/midgard-ro/pkg/math/vec_test.go::TestVec2*`

## Use Cases
- Player position on map
- UI element positioning
- Mouse cursor position
- 2D movement vectors
