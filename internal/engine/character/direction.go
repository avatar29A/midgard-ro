package character

import (
	gomath "math"
)

// CalculateVisualDirection returns which of the eight sprite facings to draw.
//
// RO sprite sheets are authored for one canonical camera, sitting due south of
// the character. Under that camera the sprite index is simply the character's
// compass direction: a character facing south (0) is drawn facing the viewer,
// one facing north (4) is drawn from behind. Move the camera and the same
// character has to be drawn from a different side, shifted by however many 45°
// sectors the camera has swung away from south:
//
//	sprite = (facing + 4 - cameraSector) mod 8
//
// cameraSector is the bearing from character to camera in 45° units, north
// being 0 and south 4. Substituting the canonical camera (sector 4) leaves
// sprite = facing, as it should.
//
// This previously computed (8 - (cameraSector + facing)) mod 8, which negates
// the facing instead of offsetting it — a mirror image. Under the default
// camera it made a character walking north show their face and one walking
// south show their back, i.e. exactly inverted.
//
// cameraAngle is the bearing from the player to the camera, as returned by
// CameraAngleToPlayer. lastCameraSector is the previous frame's sector, or -1
// on the first frame; pass the returned sector back in to keep the hysteresis
// working.
func CalculateVisualDirection(cameraAngle float32, playerDirection, lastCameraSector int) (visualDir, cameraSector int) {
	cameraSector = cameraSectorFromAngle(cameraAngle, lastCameraSector)
	visualDir = mod8(playerDirection + 4 - cameraSector)
	return visualDir, cameraSector
}

// cameraSectorFromAngle rounds a bearing to a 45° sector, keeping the previous
// sector while the angle stays within it plus a dead zone. A camera parked on a
// sector boundary would otherwise flip between two sprites every frame.
func cameraSectorFromAngle(angle float32, last int) int {
	const twoPi = 2 * gomath.Pi

	for angle < 0 {
		angle += twoPi
	}
	for angle >= twoPi {
		angle -= twoPi
	}

	sector := int((angle+SectorSize/2)/SectorSize) % 8

	if last >= 0 && last < 8 && sector != last {
		// How far the bearing sits from the previous sector's center, wrapped
		// to ±π so the comparison works across the 0/2π seam.
		diff := angle - float32(last)*SectorSize
		for diff > gomath.Pi {
			diff -= twoPi
		}
		for diff < -gomath.Pi {
			diff += twoPi
		}
		if diff > -(SectorSize/2+HysteresisAngle) && diff < SectorSize/2+HysteresisAngle {
			sector = last
		}
	}

	return sector
}

func mod8(v int) int {
	v %= 8
	if v < 0 {
		v += 8
	}
	return v
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

// HysteresisAngle is the dead zone angle (~11°) to prevent flickering at boundaries.
const HysteresisAngle = float32(gomath.Pi / 16)

// SectorSize is the angular size of each direction sector (45°).
const SectorSize = float32(gomath.Pi / 4)
