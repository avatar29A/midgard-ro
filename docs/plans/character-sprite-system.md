# RO Character Sprite System - Complete Technical Specification

## Overview

This document describes exactly how Ragnarok Online character sprites work, including billboard rendering, direction selection, texture compositing, and movement. This is the authoritative reference for implementing the sprite system correctly.

---

## 1. Coordinate Systems

### 1.1 World Coordinates
```
        +Z (North)
           ^
           |
   +X <----+----> -X
 (West)    |     (East)
           v
        -Z (South)
       [CAMERA default position]
```

### 1.2 Direction Indices (0-7)
```
         N (4)
    NW (3)   NE (5)
   W (2)  [P]  E (6)
    SW (1)   SE (7)
         S (0)
```

**Direction 0 (South)** = Character faces TOWARD the default camera (toward the viewer).

---

## 2. ACT/SPR File Organization

### 2.1 Action Index Formula
```
actionIndex = actionType * 8 + direction
```

| Action Type | Index Range | Description |
|-------------|-------------|-------------|
| Idle        | 0-7         | Standing still |
| Walk        | 8-15        | Walking animation |
| Sit         | 16-23       | Sitting |
| Pick Up     | 24-31       | Picking up items |
| Attack      | 32-39+      | Attack animations |

**Example**: Walk animation facing East (direction 6) = action index `1*8 + 6 = 14`

### 2.2 Layer Mirroring
Each layer in an ACT frame has a `Flags` field:
- Bit 0 = horizontal mirror flag
- `layer.IsMirrored()` returns true if the layer should be flipped horizontally

---

## 3. Billboard Rendering - THE CRITICAL DECISION

### 3.1 Two Approaches

| Approach | Billboard Behavior | Direction Selection | Result |
|----------|-------------------|---------------------|--------|
| **Camera-Facing** | Sprite quad rotates to always face camera | Select sprite based on camera+player angle | Sprite always fully visible, direction changes create 3D illusion |
| **World-Aligned** | Sprite quad fixed in world space | Select sprite based on player facing only | Sprite can become thin/invisible from side angles |

### 3.2 CORRECT APPROACH: Camera-Facing Billboard

**The billboard MUST rotate to face the camera.** This ensures:
1. The sprite is always fully visible (never edge-on)
2. The 3D illusion comes from changing which sprite direction is displayed
3. When camera rotates 360°, we see all 8 sides of the character

```
Camera at South:     Camera at East:      Camera at North:
   [Sprite]              [Sprite]            [Sprite]
      |                     |                    |
   [Camera]            [Camera]              [Camera]

Shows: Front         Shows: Left side      Shows: Back
(direction 0)        (direction 2)         (direction 4)
```

### 3.3 Billboard Calculation (Y-Axis Aligned)

```go
// Direction from player to camera
dirX := cameraX - playerX
dirZ := cameraZ - playerZ
length := sqrt(dirX*dirX + dirZ*dirZ)
dirX /= length
dirZ /= length

// Y-axis aligned billboard vectors
camRight := [3]float32{-dirZ, 0, dirX}  // Perpendicular in XZ plane
camUp := [3]float32{0, 1, 0}            // World up (keeps sprite upright)
```

This creates a billboard that:
- Rotates around the Y axis to face the camera
- Stays perfectly upright (no tilting)
- Never becomes edge-on to the camera

---

## 4. Visual Direction Selection Algorithm

### 4.1 The Problem
When camera is at a different angle than the default, we need to show a different sprite direction to maintain the illusion that we're seeing a different side of the character.

### 4.2 The Algorithm

```go
// STEP 1: Calculate angle from player to camera
cameraAngle := atan2(dirX, dirZ)  // dirX, dirZ normalized

// STEP 2: Player's facing angle in radians
// Direction 0 = South = 0 radians
// Each direction is 45° = π/4 radians
playerAngle := float32(player.Direction) * (π / 4)

// STEP 3: Combine angles
combinedAngle := cameraAngle + playerAngle

// STEP 4: Normalize to 0-2π
while combinedAngle < 0:
    combinedAngle += 2π
while combinedAngle >= 2π:
    combinedAngle -= 2π

// STEP 5: Map to 8 sectors (45° each, with 22.5° offset for centering)
sector := int((combinedAngle + π/8) / (π/4))
if sector >= 8:
    sector = 0

// STEP 6: Map sector to RO direction
// Sectors go counter-clockwise from +Z, RO directions need mapping
directionMap := [8]int{0, 7, 6, 5, 4, 3, 2, 1}
visualDir := directionMap[sector]
```

### 4.3 Why the Direction Map?

The atan2 function returns angles counter-clockwise from +Z axis:
- Sector 0: 0° (pointing +Z/North)
- Sector 1: 45° (NE direction)
- Sector 2: 90° (E direction)
- etc.

But RO directions are ordered differently:
- Direction 0: South
- Direction 1: Southwest
- Direction 2: West
- etc.

The mapping `{0, 7, 6, 5, 4, 3, 2, 1}` converts sector indices to correct RO direction indices.

### 4.4 Expected Behavior Table

| Player Faces | Camera At | Combined Angle | Sector | Visual Dir | Shows |
|--------------|-----------|----------------|--------|------------|-------|
| S (0)        | South     | 0°             | 0      | S (0)      | Front |
| S (0)        | East      | 90°            | 2      | E (6)      | Left side |
| S (0)        | North     | 180°           | 4      | N (4)      | Back |
| S (0)        | West      | 270°           | 6      | W (2)      | Right side |
| N (4)        | South     | 180°           | 4      | N (4)      | Back |
| N (4)        | East      | 270°           | 6      | W (2)      | Right side |
| E (6)        | South     | 270°           | 6      | W (2)      | Right profile |

---

## 5. Mirroring for Left-Facing Directions

### 5.1 Which Directions Need Mirroring
Directions SW (1), W (2), NW (3) should be rendered as horizontally mirrored versions.

### 5.2 Implementation
```go
spriteWidth := float32(texture.Width) * scale

// Flip for left-facing directions
if visualDir == DirSW || visualDir == DirW || visualDir == DirNW {
    spriteWidth = -spriteWidth  // Negative width flips the quad
}
```

This works because:
- The sprite quad uses `camRight * spriteWidth` for horizontal positioning
- Negative width reverses the direction, flipping the sprite horizontally

---

## 6. Composite Sprites (Head + Body)

### 6.1 Why Compositing?
- Head and body are separate SPR/ACT files
- They must be aligned using anchor points
- Pre-compositing creates a single "monolith" texture that moves as one unit

### 6.2 Anchor Point Alignment
```go
// Body frame has anchor point where head attaches
bodyAnchorX := bodyFrame.AnchorPoints[0].X
bodyAnchorY := bodyFrame.AnchorPoints[0].Y

// Head frame has anchor point for attachment
headAnchorX := headFrame.AnchorPoints[0].X
headAnchorY := headFrame.AnchorPoints[0].Y

// Head offset = difference between anchors
headOffsetX := bodyAnchorX - headAnchorX
headOffsetY := bodyAnchorY - headAnchorY
```

### 6.3 Compositing Process
1. Calculate bounding box for body sprite
2. Calculate bounding box for head sprite (with offset applied)
3. Create canvas that fits both
4. Draw body layers (bottom)
5. Draw head layers on top (with offset)
6. Store as single texture

### 6.4 Head Frame Selection
**IMPORTANT**: Always use head frame 0 for compositing, regardless of body animation frame.
- Head frame 0 has the correct anchor point positions
- Other head frames (1, 2) are "doridori" animation (head tilting) which use different anchors
- Using non-zero head frames causes head to appear offset

---

## 7. Movement System

### 7.1 Movement State
```go
type PlayerCharacter struct {
    WorldX, WorldY, WorldZ float32  // Current position
    TargetX, TargetZ       float32  // Destination
    Direction              int      // Current facing (0-7)
    CurrentAction          int      // 0=idle, 1=walk
    MoveSpeed              float32  // Units per second
}
```

### 7.2 Click-to-Move
1. Ray cast from mouse position to ground plane
2. Set target position
3. Calculate direction from current to target
4. Set CurrentAction to Walk (1)
5. Set Direction based on movement angle

### 7.3 Direction from Movement
```go
func calculateDirection(fromX, fromZ, toX, toZ float32) int {
    dx := toX - fromX
    dz := toZ - fromZ
    angle := atan2(dx, dz)

    // Normalize to 0-2π
    if angle < 0:
        angle += 2π

    // Map to 8 sectors
    sector := int((angle + π/8) / (π/4))
    if sector >= 8:
        sector = 0

    // Map sector to RO direction
    directionMap := [8]int{0, 7, 6, 5, 4, 3, 2, 1}
    return directionMap[sector]
}
```

### 7.4 Animation Update
```go
func updateAnimation(deltaTime float32) {
    if player.CurrentAction == ActionIdle:
        return  // No animation for idle

    actionIdx := player.CurrentAction*8 + player.Direction
    interval := act.Intervals[actionIdx]  // ms per frame

    player.AnimTimer += deltaTime * 1000  // Convert to ms
    if player.AnimTimer >= interval:
        player.AnimTimer -= interval
        player.CurrentFrame++

        numFrames := len(act.Actions[actionIdx].Frames)
        if player.CurrentFrame >= numFrames:
            player.CurrentFrame = 0  // Loop
}
```

---

## 8. Rendering Pipeline

### 8.1 Render Order
1. Render shadow (below character)
2. Calculate billboard vectors (camera-facing)
3. Calculate visual direction (camera + player angle)
4. Select composite texture for current action/direction/frame
5. Apply mirroring for left-facing directions
6. Draw billboard quad with texture

### 8.2 Vertex Shader
```glsl
uniform mat4 uViewProj;
uniform vec3 uWorldPos;      // Character position
uniform vec2 uSpriteSize;    // Width, height (width can be negative for mirroring)
uniform vec3 uCamRight;      // Billboard right vector
uniform vec3 uCamUp;         // Billboard up vector

void main() {
    vec3 pos = uWorldPos;
    pos += uCamRight * aPosition.x * uSpriteSize.x;
    pos += uCamUp * aPosition.y * uSpriteSize.y;
    gl_Position = uViewProj * vec4(pos, 1.0);
}
```

### 8.3 Quad Vertices
```
(-0.5, 1.0) --- (0.5, 1.0)   // Top edge (head)
     |              |
     |   [sprite]   |
     |              |
(-0.5, 0.0) --- (0.5, 0.0)   // Bottom edge (feet at origin)
```

---

## 9. Common Bugs and Solutions

### 9.1 Bug: Character Spins with Camera
**Cause**: Using world-aligned billboard instead of camera-facing
**Solution**: Billboard must rotate to face camera (Y-axis aligned)

### 9.2 Bug: Wrong Sprite Direction Shown
**Cause**: Missing direction map in visual direction calculation
**Solution**: Apply `directionMap := {0, 7, 6, 5, 4, 3, 2, 1}` after calculating sector

### 9.3 Bug: Head Offset from Body
**Cause**: Using wrong head frame (not frame 0) for compositing
**Solution**: Always use head action's frame 0 for anchor alignment

### 9.4 Bug: Sprite Looks Flat from Side
**Cause**: Billboard not rotating to face camera
**Solution**: Use camera-facing billboard vectors

### 9.5 Bug: Pick-up Animation Shown Instead of Idle
**Cause**: Wrong action index calculation
**Solution**: Ensure `actionIdx = actionType * 8 + direction` is correct

---

## 10. Verification Checklist

### 10.1 Direction Test
- [ ] Face South, camera at default → see front (dir 0)
- [ ] Face North, camera at default → see back (dir 4)
- [ ] Face East, camera at default → see right profile (dir 6)
- [ ] Face West, camera at default → see left profile mirrored (dir 2)

### 10.2 Camera Rotation Test
- [ ] Player faces North, rotate camera 360° → see all 8 directions in sequence
- [ ] Sprite never becomes edge-on or invisible
- [ ] Sprite does NOT spin/rotate with camera movement

### 10.3 Movement Test
- [ ] Click north → character walks showing back
- [ ] Click south → character walks showing front
- [ ] Click east → character walks showing right side
- [ ] Smooth animation during walk

### 10.4 Composite Test
- [ ] Head stays attached to body in all directions
- [ ] Head stays attached during walk animation
- [ ] No visible seam between head and body
