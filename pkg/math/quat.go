package math

import "math"

// Quat represents a quaternion for 3D rotations.
// Components are stored as X, Y, Z, W where W is the scalar part.
type Quat struct {
	X, Y, Z, W float32
}

// QuatIdentity returns an identity quaternion (no rotation).
func QuatIdentity() Quat {
	return Quat{X: 0, Y: 0, Z: 0, W: 1}
}

// QuatFromAxisAngle creates a quaternion from axis-angle rotation.
// axis should be normalized, angle is in radians.
func QuatFromAxisAngle(axis Vec3, angle float32) Quat {
	halfAngle := angle / 2
	s := float32(math.Sin(float64(halfAngle)))
	return Quat{
		X: axis.X * s,
		Y: axis.Y * s,
		Z: axis.Z * s,
		W: float32(math.Cos(float64(halfAngle))),
	}
}

// Normalize returns a normalized quaternion.
func (q Quat) Normalize() Quat {
	length := float32(math.Sqrt(float64(q.X*q.X + q.Y*q.Y + q.Z*q.Z + q.W*q.W)))
	if length < 0.0001 {
		return QuatIdentity()
	}
	invLen := 1.0 / length
	return Quat{
		X: q.X * invLen,
		Y: q.Y * invLen,
		Z: q.Z * invLen,
		W: q.W * invLen,
	}
}

// Dot returns the dot product of two quaternions.
func (q Quat) Dot(other Quat) float32 {
	return q.X*other.X + q.Y*other.Y + q.Z*other.Z + q.W*other.W
}

// Slerp performs spherical linear interpolation between two quaternions.
// t should be in range [0, 1].
func (q Quat) Slerp(other Quat, t float32) Quat {
	// Compute cos of angle between quaternions
	dot := q.Dot(other)

	// If dot is negative, negate one quaternion to take the shorter path
	if dot < 0 {
		other = Quat{X: -other.X, Y: -other.Y, Z: -other.Z, W: -other.W}
		dot = -dot
	}

	// If quaternions are very close, use linear interpolation to avoid division by zero
	if dot > 0.9995 {
		return Quat{
			X: q.X + t*(other.X-q.X),
			Y: q.Y + t*(other.Y-q.Y),
			Z: q.Z + t*(other.Z-q.Z),
			W: q.W + t*(other.W-q.W),
		}.Normalize()
	}

	// Standard slerp
	theta0 := float32(math.Acos(float64(dot)))
	theta := theta0 * t
	sinTheta := float32(math.Sin(float64(theta)))
	sinTheta0 := float32(math.Sin(float64(theta0)))

	s0 := float32(math.Cos(float64(theta))) - dot*sinTheta/sinTheta0
	s1 := sinTheta / sinTheta0

	return Quat{
		X: q.X*s0 + other.X*s1,
		Y: q.Y*s0 + other.Y*s1,
		Z: q.Z*s0 + other.Z*s1,
		W: q.W*s0 + other.W*s1,
	}
}

// ToMat4 converts the quaternion to a 4x4 rotation matrix.
func (q Quat) ToMat4() Mat4 {
	// Normalize first
	q = q.Normalize()

	xx := q.X * q.X
	xy := q.X * q.Y
	xz := q.X * q.Z
	xw := q.X * q.W
	yy := q.Y * q.Y
	yz := q.Y * q.Z
	yw := q.Y * q.W
	zz := q.Z * q.Z
	zw := q.Z * q.W

	return Mat4{
		1 - 2*(yy+zz), 2 * (xy + zw), 2 * (xz - yw), 0,
		2 * (xy - zw), 1 - 2*(xx+zz), 2 * (yz + xw), 0,
		2 * (xz + yw), 2 * (yz - xw), 1 - 2*(xx+yy), 0,
		0, 0, 0, 1,
	}
}

// Lerp performs linear interpolation between two quaternions.
// Use Slerp for rotation interpolation; this is for simple blending.
func (q Quat) Lerp(other Quat, t float32) Quat {
	return Quat{
		X: q.X + t*(other.X-q.X),
		Y: q.Y + t*(other.Y-q.Y),
		Z: q.Z + t*(other.Z-q.Z),
		W: q.W + t*(other.W-q.W),
	}.Normalize()
}

// Mul multiplies two quaternions (combines rotations).
func (q Quat) Mul(other Quat) Quat {
	return Quat{
		X: q.W*other.X + q.X*other.W + q.Y*other.Z - q.Z*other.Y,
		Y: q.W*other.Y - q.X*other.Z + q.Y*other.W + q.Z*other.X,
		Z: q.W*other.Z + q.X*other.Y - q.Y*other.X + q.Z*other.W,
		W: q.W*other.W - q.X*other.X - q.Y*other.Y - q.Z*other.Z,
	}
}

// LerpVec3 performs linear interpolation between two 3D vectors.
func LerpVec3(a, b [3]float32, t float32) [3]float32 {
	return [3]float32{
		a[0] + t*(b[0]-a[0]),
		a[1] + t*(b[1]-a[1]),
		a[2] + t*(b[2]-a[2]),
	}
}
