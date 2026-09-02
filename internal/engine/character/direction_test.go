package character

import (
	gomath "math"
	"testing"

	"github.com/Faultbox/midgard-ro/internal/game/entity"
)

// Bearings from the character to the camera, in the atan2(x, z) convention
// CameraAngleToPlayer uses: 0 is north (+Z), increasing toward east (+X).
const (
	camNorth = float32(0)
	camEast  = float32(gomath.Pi / 2)
	camSouth = float32(gomath.Pi)
	camWest  = float32(3 * gomath.Pi / 2)
)

// TestVisualDirectionCanonicalCamera pins the defining case: with the camera
// due south — the view RO sprite sheets are authored for, and where
// ThirdPersonCamera sits at Yaw 0 — the sprite index is just the character's
// compass facing.
//
// Concretely: walking north (away from the camera) shows their back, and
// walking south (toward it) shows their face. The old formula mirrored the
// index and produced exactly the opposite.
func TestVisualDirectionCanonicalCamera(t *testing.T) {
	tests := []struct {
		name   string
		facing int
		want   int
		sees   string
	}{
		{"facing south, toward camera", entity.DirS, entity.DirS, "face"},
		{"facing north, away from camera", entity.DirN, entity.DirN, "back"},
		{"facing east", entity.DirE, entity.DirE, "right profile"},
		{"facing west", entity.DirW, entity.DirW, "left profile"},
		{"facing southwest", entity.DirSW, entity.DirSW, "front-left"},
		{"facing southeast", entity.DirSE, entity.DirSE, "front-right"},
		{"facing northwest", entity.DirNW, entity.DirNW, "back-left"},
		{"facing northeast", entity.DirNE, entity.DirNE, "back-right"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := CalculateVisualDirection(camSouth, tt.facing, -1)
			if got != tt.want {
				t.Errorf("facing %d under the canonical camera drew sprite %d, want %d (%s)",
					tt.facing, got, tt.want, tt.sees)
			}
		})
	}
}

// TestVisualDirectionFacingCameraAlwaysShowsFront is the invariant that makes
// the whole thing legible: whichever way the camera is moved, a character
// facing it is drawn front-on (sprite 0), and one facing directly away is
// drawn from behind (sprite 4).
func TestVisualDirectionFacingCameraAlwaysShowsFront(t *testing.T) {
	tests := []struct {
		name        string
		camAngle    float32
		facesCamera int
		facesAway   int
	}{
		{"camera south", camSouth, entity.DirS, entity.DirN},
		{"camera north", camNorth, entity.DirN, entity.DirS},
		{"camera east", camEast, entity.DirE, entity.DirW},
		{"camera west", camWest, entity.DirW, entity.DirE},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, _ := CalculateVisualDirection(tt.camAngle, tt.facesCamera, -1); got != entity.DirS {
				t.Errorf("facing the camera drew sprite %d, want 0 (front view)", got)
			}
			if got, _ := CalculateVisualDirection(tt.camAngle, tt.facesAway, -1); got != entity.DirN {
				t.Errorf("facing away drew sprite %d, want 4 (back view)", got)
			}
		})
	}
}

// TestVisualDirectionOrbitCyclesAllEight checks the 3D illusion: parking a
// character and walking the camera the whole way round must show all eight
// sides exactly once.
func TestVisualDirectionOrbitCyclesAllEight(t *testing.T) {
	seen := map[int]int{}
	for sector := 0; sector < 8; sector++ {
		angle := float32(sector) * SectorSize
		got, _ := CalculateVisualDirection(angle, entity.DirS, -1)
		seen[got]++
	}

	if len(seen) != 8 {
		t.Errorf("orbiting the camera showed %d distinct sprites, want all 8: %v", len(seen), seen)
	}
	for dir, count := range seen {
		if count != 1 {
			t.Errorf("sprite %d appeared %d times during one orbit, want once", dir, count)
		}
	}
}

// TestVisualDirectionRotatingCharacterCyclesAllEight is the same check from the
// other side: a character turning in place under a fixed camera.
func TestVisualDirectionRotatingCharacterCyclesAllEight(t *testing.T) {
	seen := map[int]bool{}
	for facing := 0; facing < 8; facing++ {
		got, _ := CalculateVisualDirection(camSouth, facing, -1)
		seen[got] = true
	}
	if len(seen) != 8 {
		t.Errorf("turning in place showed %d distinct sprites, want all 8", len(seen))
	}
}

// TestCameraAngleToPlayerBearings pins the coordinate convention the formula
// depends on: +X east, +Z north, angle measured as atan2(x, z).
func TestCameraAngleToPlayerBearings(t *testing.T) {
	const px, pz = float32(100), float32(100)

	tests := []struct {
		name         string
		camX, camZ   float32
		wantSectorID int
	}{
		{"camera north of player", px, pz + 50, 0},
		{"camera east of player", px + 50, pz, 2},
		{"camera south of player", px, pz - 50, 4},
		{"camera west of player", px - 50, pz, 6},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			angle := CameraAngleToPlayer(tt.camX, tt.camZ, px, pz)
			got := cameraSectorFromAngle(angle, -1)
			if got != tt.wantSectorID {
				t.Errorf("camera sector = %d, want %d (angle %.3f)", got, tt.wantSectorID, angle)
			}
		})
	}
}

// TestVisualDirectionHysteresis checks that a camera sitting just past a sector
// boundary keeps the previous sprite rather than flickering between two.
func TestVisualDirectionHysteresis(t *testing.T) {
	// Just past the boundary between sector 4 and 5.
	boundary := 4*SectorSize + SectorSize/2 + HysteresisAngle/2

	fresh := cameraSectorFromAngle(boundary, -1)
	sticky := cameraSectorFromAngle(boundary, 4)

	if sticky != 4 {
		t.Errorf("with sector 4 held, angle just past the boundary moved to %d; "+
			"the dead zone should have kept it at 4", sticky)
	}
	if fresh == 4 {
		t.Error("without a previous sector the same angle should have rounded past 4, " +
			"otherwise the test proves nothing")
	}
}

// TestVisualDirectionDefaultCameraMatchesGameplay documents the end-to-end
// expectation in the terms you see in game, with the camera at Yaw 0.
func TestVisualDirectionDefaultCameraMatchesGameplay(t *testing.T) {
	// ThirdPersonCamera.Position subtracts the yaw offset, so Yaw 0 puts the
	// camera at -Z from the player: due south.
	tests := []struct {
		walking string
		facing  int
		want    string
	}{
		{"north", entity.DirN, "back"},
		{"south", entity.DirS, "face"},
		{"east", entity.DirE, "right side"},
		{"west", entity.DirW, "left side"},
	}

	views := map[int]string{
		entity.DirN: "back",
		entity.DirS: "face",
		entity.DirE: "right side",
		entity.DirW: "left side",
	}

	for _, tt := range tests {
		t.Run("walking "+tt.walking, func(t *testing.T) {
			got, _ := CalculateVisualDirection(camSouth, tt.facing, -1)
			if views[got] != tt.want {
				t.Errorf("walking %s showed %q, want %q", tt.walking, views[got], tt.want)
			}
		})
	}
}

// TestBillboardFacesTheCameraSquarely: the quad's axes have to be
// perpendicular to the line of sight, or the sprite is foreshortened by
// however far the camera is tilted — which at RO's own angle is a third of
// the character's height.
func TestBillboardFacesTheCameraSquarely(t *testing.T) {
	// A camera at RO's pitch of 0.85 radians, 300 units out.
	const pitch = 0.85

	camX := float32(0)
	camY := float32(300 * gomath.Sin(pitch))
	camZ := float32(300 * gomath.Cos(pitch))

	right, up := BillboardVectors(camX, camY, camZ, 0, 0, 0)

	view := [3]float32{camX, camY, camZ}
	length := float32(gomath.Sqrt(float64(view[0]*view[0] + view[1]*view[1] + view[2]*view[2])))
	for i := range view {
		view[i] /= length
	}

	if d := dot(up, view); absf(d) > 1e-4 {
		t.Errorf("up is not square to the view: dot = %v", d)
	}
	if d := dot(right, view); absf(d) > 1e-4 {
		t.Errorf("right is not square to the view: dot = %v", d)
	}
	if d := dot(right, up); absf(d) > 1e-4 {
		t.Errorf("the axes are not square to each other: dot = %v", d)
	}

	if l := length3(up); absf(l-1) > 1e-4 {
		t.Errorf("up is %v long, want 1 — a quad scaled by it would be the wrong size", l)
	}
	if l := length3(right); absf(l-1) > 1e-4 {
		t.Errorf("right is %v long, want 1", l)
	}

	// Leaned back, not upright: an upright quad is what the squashing was.
	if up[1] > 0.99 {
		t.Errorf("up is %v, which is still upright — the sprite would stay foreshortened", up)
	}
}

// TestBillboardIsUprightForALevelCamera: with nothing to lean away from, the
// quad stands as it always did.
func TestBillboardIsUprightForALevelCamera(t *testing.T) {
	right, up := BillboardVectors(0, 0, 100, 0, 0, 0)

	if up != [3]float32{0, 1, 0} {
		t.Errorf("up = %v, want straight up for a camera at eye level", up)
	}
	// Across the view, in the sense this has always used: the other way round
	// mirrors every sprite.
	if right != [3]float32{-1, 0, 0} {
		t.Errorf("right = %v, want across the view", right)
	}
}

// TestBillboardNeverRolls: the horizontal axis stays level however the camera
// is placed, so a sprite is never drawn tilted sideways.
func TestBillboardNeverRolls(t *testing.T) {
	for _, cam := range [][3]float32{
		{100, 200, 100}, {-50, 400, 30}, {0, 10, -300}, {7, 0, 0},
	} {
		right, _ := BillboardVectors(cam[0], cam[1], cam[2], 0, 0, 0)
		if right[1] != 0 {
			t.Errorf("camera %v gave right = %v, which rolls the sprite", cam, right)
		}
	}
}

func dot(a, b [3]float32) float32 { return a[0]*b[0] + a[1]*b[1] + a[2]*b[2] }

func length3(v [3]float32) float32 { return float32(gomath.Sqrt(float64(dot(v, v)))) }

func absf(v float32) float32 {
	if v < 0 {
		return -v
	}

	return v
}
