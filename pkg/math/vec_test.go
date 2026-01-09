package math

import (
	"testing"
)

func TestVec2Add(t *testing.T) {
	a := Vec2{1, 2}
	b := Vec2{3, 4}
	got := a.Add(b)
	want := Vec2{4, 6}
	if got != want {
		t.Errorf("Vec2.Add() = %v, want %v", got, want)
	}
}

func TestVec2Length(t *testing.T) {
	v := Vec2{3, 4}
	got := v.Length()
	want := float32(5)
	if got != want {
		t.Errorf("Vec2.Length() = %v, want %v", got, want)
	}
}

func TestVec2Normalize(t *testing.T) {
	v := Vec2{3, 4}
	n := v.Normalize()
	l := n.Length()
	if l < 0.999 || l > 1.001 {
		t.Errorf("Vec2.Normalize().Length() = %v, want ~1", l)
	}
}

func TestVec3Cross(t *testing.T) {
	x := Vec3{1, 0, 0}
	y := Vec3{0, 1, 0}
	got := x.Cross(y)
	want := Vec3{0, 0, 1}
	if got != want {
		t.Errorf("Vec3.Cross() = %v, want %v", got, want)
	}
}
