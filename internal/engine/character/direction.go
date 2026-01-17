package character

import (
	gomath "math"
)

// CalculateVisualDirection determines the sprite direction to display based on
// camera angle, player facing direction, and applies hysteresis to prevent flickering.
//
// cameraAngle: angle from player to camera (radians, from atan2)
// playerDirection: player's facing direction (0-7, RO direction index)
// lastVisualSector: previous frame's sector (-1 if none)
//
// Returns the visual direction (0-7) and the new sector for hysteresis.
func CalculateVisualDirection(cameraAngle float32, playerDirection int, lastVisualSector int) (visualDir int, newSector int) {
	// Player facing angle
	playerAngle := float32(playerDirection) * (gomath.Pi / 4.0)

	// Combine camera and player angles
	combinedAngle := cameraAngle + playerAngle

	// Normalize to 0-2π
	for combinedAngle < 0 {
		combinedAngle += 2 * gomath.Pi
	}
	for combinedAngle >= 2*gomath.Pi {
		combinedAngle -= 2 * gomath.Pi
	}

	// Calculate new sector with standard boundaries
	sectorSize := float32(gomath.Pi / 4)   // 45° per sector
	sectorOffset := float32(gomath.Pi / 8) // 22.5° offset
	newSector = int((combinedAngle + sectorOffset) / sectorSize)
	if newSector >= 8 {
		newSector = 0
	}

	// Hysteresis: only change direction if we're past the dead zone
	// This prevents flickering at sector boundaries
	hysteresis := float32(gomath.Pi / 16) // ~11° dead zone on each side of boundary

	// Check if we should keep the previous direction (within dead zone)
	if lastVisualSector >= 0 {
		// Calculate the center angle of the current visual direction's sector
		currentSectorCenter := float32(lastVisualSector) * sectorSize

		// Distance from current sector center
		angleDiff := combinedAngle - currentSectorCenter
		// Normalize to -π to π
		for angleDiff > gomath.Pi {
			angleDiff -= 2 * gomath.Pi
		}
		for angleDiff < -gomath.Pi {
			angleDiff += 2 * gomath.Pi
		}

		// If within the extended range (half sector + hysteresis), keep current direction
		if angleDiff > -(sectorSize/2+hysteresis) && angleDiff < (sectorSize/2+hysteresis) {
			newSector = lastVisualSector
		}
	}

	// Map sector to RO direction index
	visualDir = SectorToDirection[newSector]
	return visualDir, newSector
}

// CameraAngleToPlayer calculates the angle from player to camera.
// Returns the angle in radians suitable for CalculateVisualDirection.
func CameraAngleToPlayer(cameraX, cameraZ, playerX, playerZ float32) float32 {
	dirX := cameraX - playerX
	dirZ := cameraZ - playerZ
	length := float32(gomath.Sqrt(float64(dirX*dirX + dirZ*dirZ)))
	if length > 0.001 {
		dirX /= length
		dirZ /= length
	} else {
		dirX = 0
		dirZ = 1
	}
	return float32(gomath.Atan2(float64(dirX), float64(dirZ)))
}

// BillboardVectors calculates camera-facing billboard vectors for sprite rendering.
// Returns right and up vectors for Y-axis aligned billboard.
func BillboardVectors(cameraX, cameraZ, playerX, playerZ float32) (right, up [3]float32) {
	dirX := cameraX - playerX
	dirZ := cameraZ - playerZ
	length := float32(gomath.Sqrt(float64(dirX*dirX + dirZ*dirZ)))
	if length > 0.001 {
		dirX /= length
		dirZ /= length
	} else {
		dirX = 0
		dirZ = 1
	}
	// Camera-facing billboard vectors (Y-axis aligned)
	right = [3]float32{-dirZ, 0, dirX}
	up = [3]float32{0, 1, 0}
	return right, up
}

// SectorToDirection maps sector index to RO direction index.
var SectorToDirection = [8]int{0, 7, 6, 5, 4, 3, 2, 1}

// DirectionToSector maps RO direction index to sector index.
var DirectionToSector = [8]int{0, 7, 6, 5, 4, 3, 2, 1}

// HysteresisAngle is the dead zone angle (~11°) to prevent flickering at boundaries.
const HysteresisAngle = float32(gomath.Pi / 16)

// SectorSize is the angular size of each direction sector (45°).
const SectorSize = float32(gomath.Pi / 4)
