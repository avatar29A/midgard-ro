# Plan: RO-Style Sprite Direction System (v2 - Research-Based)

## Problem Statement

Two issues:
1. **Movement**: Sprite direction doesn't match movement direction
2. **Camera rotation**: Sprite looks flat from all angles, no 3D simulation

---

## Research Findings

### Sources
- [thiscodeworks - Complete algorithm for 2D sprites based on camera angle](https://www.thiscodeworks.com/function-to-rotate-change-animation-of-2d-sprites-based-on-camera-angle/622c95642a5de40015e7f37c)
- [MonoGame - Sprite facing relative to isometric camera](https://community.monogame.net/t/determine-facing-for-a-2d-sprite-relative-to-3d-isometric-camera/11674)
- [Unity Shadergraph sprite direction](https://www.noveltech.dev/sprite-change-camera-direction)
- [Ragnarok Research Lab](https://ragnarokresearchlab.github.io/rendering/camera-controls/)

### Key Discovery: The Working Algorithm

From the thiscodeworks reference, here's a **proven working implementation**:

```csharp
// 1. Calculate angle from CAMERA to UNIT
float angle = atan2(cameraX - unitX, cameraZ - unitZ) * 180 / PI;

// 2. ADD unit's facing direction offset
if (unit.direction.x == 1)  angle += 90;   // Moving right
if (unit.direction.x == -1) angle -= 90;   // Moving left
if (unit.direction.z == 1)  angle += 180;  // Moving forward

// 3. Normalize to 0-360
if (angle < 0) angle += 360;
if (angle > 360) angle -= 360;

// 4. Map to 8 directions (45° sectors with 22.5° tolerance)
// 5. Billboard faces camera using LookAt()
unit.transform.LookAt(camera);
```

### Critical Insight: TWO Systems Working Together

The 3D illusion requires **BOTH**:

| Component | What it does | Why needed |
|-----------|--------------|------------|
| **Billboard (LookAt)** | Sprite quad ALWAYS faces camera | Makes sprite visible from any angle |
| **Sprite Frame Selection** | Choose which of 8 frames to show | Creates illusion of seeing different sides |

**The billboard ensures visibility. The sprite frame creates the 3D illusion.**

---

## The Algorithm Explained

### Step 1: Calculate Camera-to-Unit Angle

```
cameraAngle = atan2(camX - playerX, camZ - playerZ)
```

This gives the angle FROM the camera TO the player (where camera is looking from).

### Step 2: Combine with Player Facing Direction

```
combinedAngle = cameraAngle + playerFacingAngle
```

The player's facing direction rotates the base angle.

### Step 3: Map to 8 Directions

Divide 360° into 8 sectors of 45° each:

```
        0° (Up/N)
   315°         45°
270° (Left)    90° (Right)
   225°        135°
       180° (Down/S)
```

### Step 4: Apply Mirroring

Left-facing directions (225°-315°) use mirrored right-facing sprites.

---

## Visual Explanation

```
CAMERA POSITIONS (looking at player in center):

         North (180°)
            ↓
   West ← [PLAYER] → East
  (270°)     ↑      (90°)
         South (0°)
        (default)


WHAT HAPPENS WHEN CAMERA MOVES:

Camera at South (default):
- Player faces North → We see BACK → Show back sprite
- Player faces South → We see FRONT → Show front sprite

Camera moves to East (90° rotation):
- Player faces North → We see LEFT SIDE → Show left sprite
- Player faces South → We see RIGHT SIDE → Show right sprite

Camera moves to North (180° rotation):
- Player faces North → We see FRONT → Show front sprite
- Player faces South → We see BACK → Show back sprite
```

---

## Implementation Plan

### Part 1: Billboard Always Faces Camera

```go
// Calculate direction from player to camera
toCamera := normalize(cameraPos - playerPos)

// Billboard right vector = perpendicular to toCamera in XZ plane
camRight := [3]float32{-toCamera.Z, 0, toCamera.X}
camUp := [3]float32{0, 1, 0}  // World up
```

This ensures sprite is always fully visible (not edge-on).

### Part 2: Calculate Sprite Direction

```go
// Step 1: Camera angle (from camera to player, in XZ plane)
dx := playerX - camX
dz := playerZ - camZ
cameraAngle := atan2(dx, dz)  // Radians

// Step 2: Player facing angle (convert direction 0-7 to radians)
// RO directions: 0=S, 1=SW, 2=W, 3=NW, 4=N, 5=NE, 6=E, 7=SE
playerAngle := float32(player.Direction) * (PI / 4)  // 45° per direction

// Step 3: Combine angles
combinedAngle := cameraAngle + playerAngle

// Step 4: Normalize to 0-2π
for combinedAngle < 0 { combinedAngle += 2*PI }
for combinedAngle >= 2*PI { combinedAngle -= 2*PI }

// Step 5: Map to direction index (0-7)
sector := int((combinedAngle + PI/8) / (PI/4))  // 45° sectors with 22.5° offset
if sector >= 8 { sector = 0 }
visualDir := sector
```

### Part 3: Sprite Mirroring

```go
// Left-facing directions use mirrored right sprites
if visualDir == DirSW || visualDir == DirW || visualDir == DirNW {
    spriteWidth = -spriteWidth  // Flip horizontally
}
```

---

## Why Previous Attempts Failed

| Attempt | Problem |
|---------|---------|
| Fixed billboard + sprite direction | Sprite looked flat from side angles |
| Y-axis billboard + (camDir + playerDir) | Wrong formula, sprite spun with camera |
| Y-axis billboard + (playerDir - camDir) | Still wrong, sprite rotated incorrectly |

**The fix**: Use BOTH camera-facing billboard AND camera+player angle combination for sprite selection.

---

## Expected Behavior

### Movement Test
| Player moves | Camera at default | Sprite shows |
|--------------|-------------------|--------------|
| North (away) | South | Back |
| South (toward) | South | Front |
| East (right) | South | Right profile |
| West (left) | South | Left profile (mirrored) |

### Camera Rotation Test
| Player faces | Camera position | Sprite shows |
|--------------|-----------------|--------------|
| North | South | Back |
| North | East | Left side |
| North | North | Front |
| North | West | Right side |

---

## Files to Modify

`cmd/grfbrowser/map_viewer.go`:
- `renderPlayerCharacter()` - implement new algorithm
- Keep Y-axis billboard (camera-facing)
- New sprite direction calculation based on atan2

---

## Verification Checklist

1. [ ] Walk North → see back sprite
2. [ ] Walk South → see front sprite
3. [ ] Walk East → see right profile
4. [ ] Walk West → see left profile (mirrored)
5. [ ] Stand still, rotate camera 360° → see all 8 sides sequentially
6. [ ] Walk any direction + rotate camera → consistent view of appropriate side
