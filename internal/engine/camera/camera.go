// Package camera provides camera implementations for 3D rendering.
package camera

import (
	gomath "math"

	"github.com/Faultbox/midgard-ro/pkg/math"
)

// OrbitCamera orbits around a center point.
type OrbitCamera struct {
	// Center point to orbit around
	CenterX, CenterY, CenterZ float32

	// Spherical coordinates
	Distance  float32 // Distance from center
	RotationX float32 // Pitch (vertical angle, radians)
	RotationY float32 // Yaw (horizontal angle, radians)

	// Constraints
	MinDistance float32
	MaxDistance float32
	MinPitch    float32
	MaxPitch    float32

	// Sensitivity
	DragSensitivity float32
	ZoomSensitivity float32
}

// NewOrbitCamera creates a new orbit camera with default settings.
func NewOrbitCamera() *OrbitCamera {
	return &OrbitCamera{
		Distance:        200.0,
		RotationX:       0.5,
		RotationY:       0.0,
		MinDistance:     50.0,
		MaxDistance:     5000.0,
		MinPitch:        0.1,
		MaxPitch:        1.5,
		DragSensitivity: 0.005,
		ZoomSensitivity: 0.1,
	}
}

// Position returns the camera position in world space.
func (c *OrbitCamera) Position() math.Vec3 {
	x := c.Distance * float32(gomath.Cos(float64(c.RotationX))*gomath.Sin(float64(c.RotationY)))
	y := c.Distance * float32(gomath.Sin(float64(c.RotationX)))
	z := c.Distance * float32(gomath.Cos(float64(c.RotationX))*gomath.Cos(float64(c.RotationY)))

	return math.Vec3{
		X: c.CenterX + x,
		Y: c.CenterY + y,
		Z: c.CenterZ + z,
	}
}

// ViewMatrix returns the view matrix for this camera.
func (c *OrbitCamera) ViewMatrix() math.Mat4 {
	pos := c.Position()
	center := math.Vec3{X: c.CenterX, Y: c.CenterY, Z: c.CenterZ}
	up := math.Vec3{X: 0, Y: 1, Z: 0}
	return math.LookAt(pos, center, up)
}

// HandleDrag updates rotation based on mouse drag delta.
func (c *OrbitCamera) HandleDrag(deltaX, deltaY float32) {
	c.RotationY -= deltaX * c.DragSensitivity
	c.RotationX += deltaY * c.DragSensitivity

	// Clamp pitch
	if c.RotationX < c.MinPitch {
		c.RotationX = c.MinPitch
	}
	if c.RotationX > c.MaxPitch {
		c.RotationX = c.MaxPitch
	}
}

// HandleZoom updates distance based on scroll wheel delta.
func (c *OrbitCamera) HandleZoom(delta float32) {
	c.Distance -= delta * c.Distance * c.ZoomSensitivity
	if c.Distance < c.MinDistance {
		c.Distance = c.MinDistance
	}
	if c.Distance > c.MaxDistance {
		c.Distance = c.MaxDistance
	}
}

// HandleMovement pans the camera center point based on keyboard input.
func (c *OrbitCamera) HandleMovement(forward, right, up float32) {
	// Speed scales with distance for consistent feel
	speed := c.Distance * 0.01

	// Calculate movement direction based on current camera rotation
	dirX := float32(gomath.Sin(float64(c.RotationY)))
	dirZ := float32(gomath.Cos(float64(c.RotationY)))

	// Right direction (perpendicular to forward)
	rightX := float32(gomath.Cos(float64(c.RotationY)))
	rightZ := float32(-gomath.Sin(float64(c.RotationY)))

	// Apply movement to center point (negate forward so W moves "into" the scene)
	c.CenterX += (-dirX*forward + rightX*right) * speed
	c.CenterZ += (-dirZ*forward + rightZ*right) * speed
	c.CenterY += up * speed
}

// SetCenter sets the camera's center point.
func (c *OrbitCamera) SetCenter(x, y, z float32) {
	c.CenterX = x
	c.CenterY = y
	c.CenterZ = z
}

// FitToBounds adjusts camera to view the given bounding box.
func (c *OrbitCamera) FitToBounds(minX, minY, minZ, maxX, maxY, maxZ float32) {
	// Set center to bounds center
	c.CenterX = (minX + maxX) / 2
	c.CenterY = (minY + maxY) / 2
	c.CenterZ = (minZ + maxZ) / 2

	// Calculate distance based on size
	sizeX := maxX - minX
	sizeZ := maxZ - minZ
	maxSize := sizeX
	if sizeZ > maxSize {
		maxSize = sizeZ
	}

	c.Distance = maxSize * 0.3
	if c.Distance < 200 {
		c.Distance = 200
	}

	c.RotationX = 0.6 // Look down at ~35 degrees
	c.RotationY = 0.0
}

// ThirdPersonCamera follows a target from behind.
type ThirdPersonCamera struct {
	// Camera orientation
	Yaw   float32 // Horizontal rotation around target (radians)
	Pitch float32 // Vertical angle (radians), fixed for RO-style

	// Distance from target
	Distance    float32
	MinDistance float32
	MaxDistance float32

	// Sensitivity
	YawSensitivity  float32
	ZoomSensitivity float32

	// Cached position for external access
	PosX, PosY, PosZ float32
}

// NewThirdPersonCamera creates a new third-person camera with RO-style defaults.
func NewThirdPersonCamera() *ThirdPersonCamera {
	return &ThirdPersonCamera{
		Yaw:             0.0,
		Pitch:           0.85, // ~48 degrees - RO-style top-down
		Distance:        300.0,
		MinDistance:     100.0,
		MaxDistance:     800.0,
		YawSensitivity:  0.005,
		ZoomSensitivity: 0.1,
	}
}

// Position calculates camera position based on target position.
func (c *ThirdPersonCamera) Position(targetX, targetY, targetZ float32) math.Vec3 {
	// Calculate camera offset from target using yaw for rotation
	offsetY := c.Distance * float32(gomath.Sin(float64(c.Pitch)))
	horizDist := c.Distance * float32(gomath.Cos(float64(c.Pitch)))
	offsetX := horizDist * float32(gomath.Sin(float64(c.Yaw)))
	offsetZ := horizDist * float32(gomath.Cos(float64(c.Yaw)))

	// Camera position: behind and above target
	pos := math.Vec3{
		X: targetX - offsetX,
		Y: targetY + offsetY,
		Z: targetZ - offsetZ,
	}

	// Cache for external access
	c.PosX = pos.X
	c.PosY = pos.Y
	c.PosZ = pos.Z

	return pos
}

// ViewMatrix returns the view matrix for this camera looking at target.
func (c *ThirdPersonCamera) ViewMatrix(targetX, targetY, targetZ float32) math.Mat4 {
	pos := c.Position(targetX, targetY, targetZ)

	// Look at target position (slightly above for character center)
	target := math.Vec3{
		X: targetX,
		Y: targetY + 30, // Look at character center, not feet
		Z: targetZ,
	}

	up := math.Vec3{X: 0, Y: 1, Z: 0}
	return math.LookAt(pos, target, up)
}

// HandleYaw rotates camera horizontally around target.
func (c *ThirdPersonCamera) HandleYaw(deltaX float32) {
	c.Yaw -= deltaX * c.YawSensitivity
}

// HandleZoom updates distance from target.
func (c *ThirdPersonCamera) HandleZoom(delta float32) {
	c.Distance -= delta * c.Distance * c.ZoomSensitivity
	if c.Distance < c.MinDistance {
		c.Distance = c.MinDistance
	}
	if c.Distance > c.MaxDistance {
		c.Distance = c.MaxDistance
	}
}

// ForwardDirection returns the camera's forward direction on the XZ plane.
func (c *ThirdPersonCamera) ForwardDirection() (x, z float32) {
	return float32(gomath.Sin(float64(c.Yaw))), float32(gomath.Cos(float64(c.Yaw)))
}

// RightDirection returns the camera's right direction on the XZ plane.
func (c *ThirdPersonCamera) RightDirection() (x, z float32) {
	return float32(-gomath.Cos(float64(c.Yaw))), float32(gomath.Sin(float64(c.Yaw)))
}
