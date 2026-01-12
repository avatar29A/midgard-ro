package math

import (
	"math"
	"testing"
)

func TestIdentity(t *testing.T) {
	m := Identity()
	// Diagonal should be 1
	if m[0] != 1 || m[5] != 1 || m[10] != 1 || m[15] != 1 {
		t.Error("Identity diagonal should be 1")
	}
	// Off-diagonal should be 0
	if m[1] != 0 || m[4] != 0 {
		t.Error("Identity off-diagonal should be 0")
	}
}

func TestMulIdentity(t *testing.T) {
	m := Translate(1, 2, 3)
	id := Identity()
	result := m.Mul(id)

	for i := 0; i < 16; i++ {
		if result[i] != m[i] {
			t.Errorf("M * I should equal M, element %d: got %f, want %f", i, result[i], m[i])
		}
	}
}

func TestTranslate(t *testing.T) {
	m := Translate(5, 10, 15)

	// Translation should be in column 4 (indices 12, 13, 14)
	if m[12] != 5 || m[13] != 10 || m[14] != 15 {
		t.Errorf("Translate: got (%f, %f, %f), want (5, 10, 15)", m[12], m[13], m[14])
	}
}

func TestScale(t *testing.T) {
	m := Scale(2, 3, 4)

	if m[0] != 2 || m[5] != 3 || m[10] != 4 {
		t.Errorf("Scale diagonal: got (%f, %f, %f), want (2, 3, 4)", m[0], m[5], m[10])
	}
}

func TestTransformPoint(t *testing.T) {
	// Translate by (10, 20, 30)
	m := Translate(10, 20, 30)
	p := [3]float32{1, 2, 3}
	result := m.TransformPoint(p)

	expected := [3]float32{11, 22, 33}
	if result != expected {
		t.Errorf("TransformPoint: got %v, want %v", result, expected)
	}
}

func TestTransformPointScale(t *testing.T) {
	m := Scale(2, 2, 2)
	p := [3]float32{1, 2, 3}
	result := m.TransformPoint(p)

	expected := [3]float32{2, 4, 6}
	if result != expected {
		t.Errorf("TransformPoint with scale: got %v, want %v", result, expected)
	}
}

func TestRotateY90(t *testing.T) {
	m := RotateY(float32(math.Pi / 2)) // 90 degrees
	p := [3]float32{1, 0, 0}           // Point on X axis
	result := m.TransformPoint(p)

	// After 90 degree Y rotation, (1,0,0) should become approximately (0,0,-1)
	if abs(result[0]) > 0.001 || abs(result[1]) > 0.001 || abs(result[2]+1) > 0.001 {
		t.Errorf("RotateY 90: got %v, want (0, 0, -1)", result)
	}
}

func TestPerspective(t *testing.T) {
	fov := float32(math.Pi / 4) // 45 degrees
	aspect := float32(1.0)
	near := float32(0.1)
	far := float32(100.0)

	m := Perspective(fov, aspect, near, far)

	// Should be a valid projection matrix (not identity)
	if m[0] == 0 || m[5] == 0 {
		t.Error("Perspective should have non-zero elements")
	}
	// Element [15] should be 0 for perspective projection
	if m[15] != 0 {
		t.Errorf("Perspective [15] should be 0, got %f", m[15])
	}
	// Element [11] should be -1 for perspective projection
	if m[11] != -1 {
		t.Errorf("Perspective [11] should be -1, got %f", m[11])
	}
}

func TestLookAt(t *testing.T) {
	eye := Vec3{0, 0, 5}
	center := Vec3{0, 0, 0}
	up := Vec3{0, 1, 0}

	m := LookAt(eye, center, up)

	// Transform eye position - should result in origin (or close to it)
	// This is a simple sanity check
	if m[15] != 1 {
		t.Errorf("LookAt [15] should be 1, got %f", m[15])
	}
}

func TestFromMat3x3(t *testing.T) {
	m3 := [9]float32{
		1, 2, 3,
		4, 5, 6,
		7, 8, 9,
	}
	m4 := FromMat3x3(m3)

	// Check that 3x3 portion is preserved
	if m4[0] != 1 || m4[1] != 2 || m4[2] != 3 {
		t.Error("FromMat3x3 column 0 incorrect")
	}
	if m4[4] != 4 || m4[5] != 5 || m4[6] != 6 {
		t.Error("FromMat3x3 column 1 incorrect")
	}
	// Element [15] should be 1
	if m4[15] != 1 {
		t.Errorf("FromMat3x3 [15] should be 1, got %f", m4[15])
	}
}

func abs(x float32) float32 {
	if x < 0 {
		return -x
	}
	return x
}
