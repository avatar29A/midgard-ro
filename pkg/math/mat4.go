package math

import "math"

// Mat4 is a 4x4 matrix in column-major order (OpenGL compatible).
// Layout: [m0 m4 m8  m12]
//
//	[m1 m5 m9  m13]
//	[m2 m6 m10 m14]
//	[m3 m7 m11 m15]
type Mat4 [16]float32

// Identity returns an identity matrix.
func Identity() Mat4 {
	return Mat4{
		1, 0, 0, 0,
		0, 1, 0, 0,
		0, 0, 1, 0,
		0, 0, 0, 1,
	}
}

// Perspective returns a perspective projection matrix.
// fovY is in radians, aspect is width/height.
func Perspective(fovY, aspect, near, far float32) Mat4 {
	f := float32(1.0 / math.Tan(float64(fovY)/2.0))
	nf := 1.0 / (near - far)

	return Mat4{
		f / aspect, 0, 0, 0,
		0, f, 0, 0,
		0, 0, (far + near) * nf, -1,
		0, 0, 2 * far * near * nf, 0,
	}
}

// LookAt returns a view matrix looking from eye to center with up direction.
func LookAt(eye, center, up Vec3) Mat4 {
	f := center.Sub(eye).Normalize()
	s := f.Cross(up).Normalize()
	u := s.Cross(f)

	return Mat4{
		s.X, u.X, -f.X, 0,
		s.Y, u.Y, -f.Y, 0,
		s.Z, u.Z, -f.Z, 0,
		-s.Dot(eye), -u.Dot(eye), f.Dot(eye), 1,
	}
}

// Translate returns a translation matrix.
func Translate(x, y, z float32) Mat4 {
	return Mat4{
		1, 0, 0, 0,
		0, 1, 0, 0,
		0, 0, 1, 0,
		x, y, z, 1,
	}
}

// Scale returns a scale matrix.
func Scale(x, y, z float32) Mat4 {
	return Mat4{
		x, 0, 0, 0,
		0, y, 0, 0,
		0, 0, z, 0,
		0, 0, 0, 1,
	}
}

// RotateX returns a rotation matrix around the X axis.
// angle is in radians.
func RotateX(angle float32) Mat4 {
	c := float32(math.Cos(float64(angle)))
	s := float32(math.Sin(float64(angle)))

	return Mat4{
		1, 0, 0, 0,
		0, c, s, 0,
		0, -s, c, 0,
		0, 0, 0, 1,
	}
}

// RotateY returns a rotation matrix around the Y axis.
// angle is in radians.
func RotateY(angle float32) Mat4 {
	c := float32(math.Cos(float64(angle)))
	s := float32(math.Sin(float64(angle)))

	return Mat4{
		c, 0, -s, 0,
		0, 1, 0, 0,
		s, 0, c, 0,
		0, 0, 0, 1,
	}
}

// RotateZ returns a rotation matrix around the Z axis.
// angle is in radians.
func RotateZ(angle float32) Mat4 {
	c := float32(math.Cos(float64(angle)))
	s := float32(math.Sin(float64(angle)))

	return Mat4{
		c, s, 0, 0,
		-s, c, 0, 0,
		0, 0, 1, 0,
		0, 0, 0, 1,
	}
}

// Mul multiplies this matrix by another (m * other).
func (m Mat4) Mul(other Mat4) Mat4 {
	var result Mat4
	for col := 0; col < 4; col++ {
		for row := 0; row < 4; row++ {
			result[col*4+row] =
				m[0*4+row]*other[col*4+0] +
					m[1*4+row]*other[col*4+1] +
					m[2*4+row]*other[col*4+2] +
					m[3*4+row]*other[col*4+3]
		}
	}
	return result
}

// TransformPoint transforms a 3D point by this matrix (assumes w=1).
func (m Mat4) TransformPoint(p [3]float32) [3]float32 {
	x := m[0]*p[0] + m[4]*p[1] + m[8]*p[2] + m[12]
	y := m[1]*p[0] + m[5]*p[1] + m[9]*p[2] + m[13]
	z := m[2]*p[0] + m[6]*p[1] + m[10]*p[2] + m[14]
	w := m[3]*p[0] + m[7]*p[1] + m[11]*p[2] + m[15]
	if w != 0 && w != 1 {
		return [3]float32{x / w, y / w, z / w}
	}
	return [3]float32{x, y, z}
}

// TransformVec3 transforms a Vec3 point by this matrix.
func (m Mat4) TransformVec3(v Vec3) Vec3 {
	p := m.TransformPoint([3]float32{v.X, v.Y, v.Z})
	return Vec3{p[0], p[1], p[2]}
}

// TransformDirection transforms a direction vector (ignores translation).
func (m Mat4) TransformDirection(d [3]float32) [3]float32 {
	return [3]float32{
		m[0]*d[0] + m[4]*d[1] + m[8]*d[2],
		m[1]*d[0] + m[5]*d[1] + m[9]*d[2],
		m[2]*d[0] + m[6]*d[1] + m[10]*d[2],
	}
}

// Mat3x3 returns the upper-left 3x3 portion of the matrix.
func (m Mat4) Mat3x3() [9]float32 {
	return [9]float32{
		m[0], m[1], m[2],
		m[4], m[5], m[6],
		m[8], m[9], m[10],
	}
}

// FromMat3x3 creates a Mat4 from a 3x3 rotation matrix.
func FromMat3x3(m3 [9]float32) Mat4 {
	return Mat4{
		m3[0], m3[1], m3[2], 0,
		m3[3], m3[4], m3[5], 0,
		m3[6], m3[7], m3[8], 0,
		0, 0, 0, 1,
	}
}

// Ptr returns a pointer to the first element (for OpenGL uniform calls).
func (m *Mat4) Ptr() *float32 {
	return &m[0]
}
