package math

import (
	"math"
	"testing"
)

func TestQuatIdentity(t *testing.T) {
	q := QuatIdentity()
	if q.X != 0 || q.Y != 0 || q.Z != 0 || q.W != 1 {
		t.Errorf("Identity quaternion should be (0,0,0,1), got (%v,%v,%v,%v)", q.X, q.Y, q.Z, q.W)
	}
}

func TestQuatNormalize(t *testing.T) {
	q := Quat{X: 1, Y: 2, Z: 3, W: 4}
	n := q.Normalize()

	length := float32(math.Sqrt(float64(n.X*n.X + n.Y*n.Y + n.Z*n.Z + n.W*n.W)))
	if math.Abs(float64(length-1.0)) > 0.0001 {
		t.Errorf("Normalized quaternion length should be 1, got %v", length)
	}
}

func TestQuatSlerp(t *testing.T) {
	// Test endpoints
	q1 := QuatIdentity()
	q2 := QuatFromAxisAngle(Vec3{X: 0, Y: 1, Z: 0}, float32(math.Pi/2))

	// At t=0, should equal q1
	result0 := q1.Slerp(q2, 0)
	if math.Abs(float64(result0.W-q1.W)) > 0.001 {
		t.Errorf("Slerp at t=0 should equal q1")
	}

	// At t=1, should equal q2
	result1 := q1.Slerp(q2, 1)
	if math.Abs(float64(result1.W-q2.W)) > 0.001 {
		t.Errorf("Slerp at t=1 should equal q2")
	}

	// At t=0.5, should be halfway
	result5 := q1.Slerp(q2, 0.5)
	// For 90 degree rotation, halfway should be 45 degrees
	expectedW := float32(math.Cos(float64(math.Pi / 8))) // cos(45/2 degrees)
	if math.Abs(float64(result5.W-expectedW)) > 0.01 {
		t.Errorf("Slerp at t=0.5: expected W ~%v, got %v", expectedW, result5.W)
	}
}

func TestQuatToMat4(t *testing.T) {
	// Identity quaternion should produce identity matrix
	q := QuatIdentity()
	m := q.ToMat4()

	identity := Identity()
	for i := 0; i < 16; i++ {
		if math.Abs(float64(m[i]-identity[i])) > 0.0001 {
			t.Errorf("Identity quat should produce identity matrix, element %d: got %v, want %v", i, m[i], identity[i])
		}
	}
}

func TestQuatFromAxisAngle(t *testing.T) {
	// 90 degrees around Y axis
	q := QuatFromAxisAngle(Vec3{X: 0, Y: 1, Z: 0}, float32(math.Pi/2))

	// Should have Y component and W = cos(45deg)
	expectedW := float32(math.Cos(math.Pi / 4))
	expectedY := float32(math.Sin(math.Pi / 4))

	if math.Abs(float64(q.W-expectedW)) > 0.001 {
		t.Errorf("QuatFromAxisAngle W: expected %v, got %v", expectedW, q.W)
	}
	if math.Abs(float64(q.Y-expectedY)) > 0.001 {
		t.Errorf("QuatFromAxisAngle Y: expected %v, got %v", expectedY, q.Y)
	}
}

func TestLerpVec3(t *testing.T) {
	a := [3]float32{0, 0, 0}
	b := [3]float32{10, 20, 30}

	result := LerpVec3(a, b, 0.5)
	expected := [3]float32{5, 10, 15}

	for i := 0; i < 3; i++ {
		if math.Abs(float64(result[i]-expected[i])) > 0.001 {
			t.Errorf("LerpVec3 component %d: expected %v, got %v", i, expected[i], result[i])
		}
	}
}
