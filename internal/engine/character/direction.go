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

// BillboardVectors are the axes of a quad turned to face the camera.
//
// Both axes, not only the horizontal one. A quad that stands upright in the
// world and turns about Y alone is foreshortened by however far the camera is
// tilted: at RO's own angle of 0.85 radians that is cos(48.7°), so a character
// is drawn at two thirds of its height and everything reads slightly squat.
//
// The original never does that. Its sprites are drawn against the screen, at
// the size the art was painted, whatever the camera is doing — so the quad is
// leaned back to match, which keeps its full height on screen.
//
// The unit's own Y is not needed: what decides the tilt is the direction from
// the sprite to the camera, and taking it from the ground the sprite stands on
// is what keeps every sprite on a map leaning the same way rather than each
// one leaning by its own height.
func BillboardVectors(cameraX, cameraY, cameraZ, playerX, playerY, playerZ float32) (right, up [3]float32) {
	viewX := cameraX - playerX
	viewY := cameraY - playerY
	viewZ := cameraZ - playerZ

	length := float32(gomath.Sqrt(float64(viewX*viewX + viewY*viewY + viewZ*viewZ)))
	if length <= 0.001 {
		return [3]float32{1, 0, 0}, [3]float32{0, 1, 0}
	}

	viewX /= length
	viewY /= length
	viewZ /= length

	// Across the view, level with the world: a sprite never rolls, however
	// the camera is turned. The sign is the one this has always used — the
	// other way round mirrors every sprite.
	rightX, rightZ := -viewZ, viewX

	flat := float32(gomath.Sqrt(float64(rightX*rightX + rightZ*rightZ)))
	if flat <= 0.001 {
		// Straight overhead, where "across" has no meaning left. Nothing in
		// the game reaches this, but a camera that did would otherwise put a
		// zero-width quad on screen.
		return [3]float32{1, 0, 0}, [3]float32{0, 1, 0}
	}

	rightX /= flat
	rightZ /= flat

	right = [3]float32{rightX, 0, rightZ}

	// Perpendicular to both, which is the quad's own up once it has been
	// leaned back to face the camera: right crossed into the view, in that
	// order, so it comes out pointing up rather than down.
	up = [3]float32{
		-rightZ * viewY,
		rightZ*viewX - rightX*viewZ,
		rightX * viewY,
	}

	return right, up
}

// HysteresisAngle is the dead zone angle (~11°) to prevent flickering at boundaries.
const HysteresisAngle = float32(gomath.Pi / 16)

// SectorSize is the angular size of each direction sector (45°).
const SectorSize = float32(gomath.Pi / 4)
