package math

import (
	"testing"
)

// ============================================================================
// Vec2 Tests
// ============================================================================

func TestVec2Add(t *testing.T) {
	tests := []struct {
		name string
		a, b Vec2
		want Vec2
	}{
		{"positive", Vec2{1, 2}, Vec2{3, 4}, Vec2{4, 6}},
		{"negative", Vec2{-1, -2}, Vec2{-3, -4}, Vec2{-4, -6}},
		{"mixed", Vec2{1, -2}, Vec2{-3, 4}, Vec2{-2, 2}},
		{"zero", Vec2{0, 0}, Vec2{0, 0}, Vec2{0, 0}},
		{"identity", Vec2{5, 7}, Vec2{0, 0}, Vec2{5, 7}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.a.Add(tt.b)
			if got != tt.want {
				t.Errorf("Vec2.Add() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestVec2Sub(t *testing.T) {
	tests := []struct {
		name string
		a, b Vec2
		want Vec2
	}{
		{"positive", Vec2{5, 7}, Vec2{2, 3}, Vec2{3, 4}},
		{"negative", Vec2{-1, -2}, Vec2{-3, -4}, Vec2{2, 2}},
		{"self", Vec2{3, 4}, Vec2{3, 4}, Vec2{0, 0}},
		{"zero", Vec2{5, 7}, Vec2{0, 0}, Vec2{5, 7}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.a.Sub(tt.b)
			if got != tt.want {
				t.Errorf("Vec2.Sub() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestVec2Scale(t *testing.T) {
	tests := []struct {
		name   string
		v      Vec2
		scalar float32
		want   Vec2
	}{
		{"positive", Vec2{2, 3}, 2, Vec2{4, 6}},
		{"negative", Vec2{2, 3}, -2, Vec2{-4, -6}},
		{"zero", Vec2{2, 3}, 0, Vec2{0, 0}},
		{"one", Vec2{2, 3}, 1, Vec2{2, 3}},
		{"fraction", Vec2{4, 6}, 0.5, Vec2{2, 3}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.v.Scale(tt.scalar)
			if got != tt.want {
				t.Errorf("Vec2.Scale() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestVec2Dot(t *testing.T) {
	tests := []struct {
		name string
		a, b Vec2
		want float32
	}{
		{"positive", Vec2{1, 2}, Vec2{3, 4}, 11},
		{"perpendicular", Vec2{1, 0}, Vec2{0, 1}, 0},
		{"parallel", Vec2{2, 0}, Vec2{3, 0}, 6},
		{"negative", Vec2{1, 2}, Vec2{-1, -2}, -5},
		{"zero", Vec2{0, 0}, Vec2{1, 1}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.a.Dot(tt.b)
			if got != tt.want {
				t.Errorf("Vec2.Dot() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestVec2Length(t *testing.T) {
	tests := []struct {
		name string
		v    Vec2
		want float32
	}{
		{"3-4-5 triangle", Vec2{3, 4}, 5},
		{"unit x", Vec2{1, 0}, 1},
		{"unit y", Vec2{0, 1}, 1},
		{"zero", Vec2{0, 0}, 0},
		{"negative", Vec2{-3, -4}, 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.v.Length()
			if got != tt.want {
				t.Errorf("Vec2.Length() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestVec2Normalize(t *testing.T) {
	tests := []struct {
		name       string
		v          Vec2
		wantLength float32
	}{
		{"standard", Vec2{3, 4}, 1},
		{"large", Vec2{100, 200}, 1},
		{"small", Vec2{0.001, 0.002}, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := tt.v.Normalize()
			l := n.Length()
			if l < 0.999 || l > 1.001 {
				t.Errorf("Vec2.Normalize().Length() = %v, want ~%v", l, tt.wantLength)
			}
		})
	}
}

func TestVec2NormalizeZero(t *testing.T) {
	v := Vec2{0, 0}
	n := v.Normalize()
	if n != (Vec2{}) {
		t.Errorf("Vec2.Normalize() on zero vector = %v, want zero vector", n)
	}
}

func TestVec2Distance(t *testing.T) {
	tests := []struct {
		name string
		a, b Vec2
		want float32
	}{
		{"3-4-5 triangle", Vec2{0, 0}, Vec2{3, 4}, 5},
		{"same point", Vec2{5, 5}, Vec2{5, 5}, 0},
		{"horizontal", Vec2{0, 0}, Vec2{10, 0}, 10},
		{"vertical", Vec2{0, 0}, Vec2{0, 10}, 10},
		{"negative coords", Vec2{-3, 0}, Vec2{0, 4}, 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.a.Distance(tt.b)
			if got != tt.want {
				t.Errorf("Vec2.Distance() = %v, want %v", got, tt.want)
			}
		})
	}
}

// ============================================================================
// Vec3 Tests
// ============================================================================

func TestVec3Add(t *testing.T) {
	tests := []struct {
		name string
		a, b Vec3
		want Vec3
	}{
		{"positive", Vec3{1, 2, 3}, Vec3{4, 5, 6}, Vec3{5, 7, 9}},
		{"negative", Vec3{-1, -2, -3}, Vec3{-4, -5, -6}, Vec3{-5, -7, -9}},
		{"mixed", Vec3{1, -2, 3}, Vec3{-4, 5, -6}, Vec3{-3, 3, -3}},
		{"zero", Vec3{0, 0, 0}, Vec3{0, 0, 0}, Vec3{0, 0, 0}},
		{"identity", Vec3{5, 7, 9}, Vec3{0, 0, 0}, Vec3{5, 7, 9}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.a.Add(tt.b)
			if got != tt.want {
				t.Errorf("Vec3.Add() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestVec3Sub(t *testing.T) {
	tests := []struct {
		name string
		a, b Vec3
		want Vec3
	}{
		{"positive", Vec3{5, 7, 9}, Vec3{1, 2, 3}, Vec3{4, 5, 6}},
		{"negative", Vec3{-1, -2, -3}, Vec3{-4, -5, -6}, Vec3{3, 3, 3}},
		{"self", Vec3{3, 4, 5}, Vec3{3, 4, 5}, Vec3{0, 0, 0}},
		{"zero", Vec3{5, 7, 9}, Vec3{0, 0, 0}, Vec3{5, 7, 9}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.a.Sub(tt.b)
			if got != tt.want {
				t.Errorf("Vec3.Sub() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestVec3Scale(t *testing.T) {
	tests := []struct {
		name   string
		v      Vec3
		scalar float32
		want   Vec3
	}{
		{"positive", Vec3{1, 2, 3}, 2, Vec3{2, 4, 6}},
		{"negative", Vec3{1, 2, 3}, -2, Vec3{-2, -4, -6}},
		{"zero", Vec3{1, 2, 3}, 0, Vec3{0, 0, 0}},
		{"one", Vec3{1, 2, 3}, 1, Vec3{1, 2, 3}},
		{"fraction", Vec3{2, 4, 6}, 0.5, Vec3{1, 2, 3}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.v.Scale(tt.scalar)
			if got != tt.want {
				t.Errorf("Vec3.Scale() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestVec3Dot(t *testing.T) {
	tests := []struct {
		name string
		a, b Vec3
		want float32
	}{
		{"positive", Vec3{1, 2, 3}, Vec3{4, 5, 6}, 32},
		{"perpendicular", Vec3{1, 0, 0}, Vec3{0, 1, 0}, 0},
		{"parallel", Vec3{1, 0, 0}, Vec3{2, 0, 0}, 2},
		{"negative", Vec3{1, 2, 3}, Vec3{-1, -2, -3}, -14},
		{"zero", Vec3{0, 0, 0}, Vec3{1, 2, 3}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.a.Dot(tt.b)
			if got != tt.want {
				t.Errorf("Vec3.Dot() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestVec3Cross(t *testing.T) {
	tests := []struct {
		name string
		a, b Vec3
		want Vec3
	}{
		{"x cross y = z", Vec3{1, 0, 0}, Vec3{0, 1, 0}, Vec3{0, 0, 1}},
		{"y cross z = x", Vec3{0, 1, 0}, Vec3{0, 0, 1}, Vec3{1, 0, 0}},
		{"z cross x = y", Vec3{0, 0, 1}, Vec3{1, 0, 0}, Vec3{0, 1, 0}},
		{"y cross x = -z", Vec3{0, 1, 0}, Vec3{1, 0, 0}, Vec3{0, 0, -1}},
		{"parallel zero", Vec3{1, 0, 0}, Vec3{2, 0, 0}, Vec3{0, 0, 0}},
		{"self zero", Vec3{1, 2, 3}, Vec3{1, 2, 3}, Vec3{0, 0, 0}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.a.Cross(tt.b)
			if got != tt.want {
				t.Errorf("Vec3.Cross() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestVec3Length(t *testing.T) {
	tests := []struct {
		name string
		v    Vec3
		want float32
	}{
		{"unit x", Vec3{1, 0, 0}, 1},
		{"unit y", Vec3{0, 1, 0}, 1},
		{"unit z", Vec3{0, 0, 1}, 1},
		{"zero", Vec3{0, 0, 0}, 0},
		{"3-4-0 triangle", Vec3{3, 4, 0}, 5},
		{"negative", Vec3{-1, -2, -2}, 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.v.Length()
			if got != tt.want {
				t.Errorf("Vec3.Length() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestVec3Normalize(t *testing.T) {
	tests := []struct {
		name       string
		v          Vec3
		wantLength float32
	}{
		{"standard", Vec3{3, 4, 0}, 1},
		{"all axes", Vec3{1, 2, 2}, 1},
		{"large", Vec3{100, 200, 300}, 1},
		{"small", Vec3{0.001, 0.002, 0.002}, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := tt.v.Normalize()
			l := n.Length()
			if l < 0.999 || l > 1.001 {
				t.Errorf("Vec3.Normalize().Length() = %v, want ~%v", l, tt.wantLength)
			}
		})
	}
}

func TestVec3NormalizeZero(t *testing.T) {
	v := Vec3{0, 0, 0}
	n := v.Normalize()
	if n != (Vec3{}) {
		t.Errorf("Vec3.Normalize() on zero vector = %v, want zero vector", n)
	}
}

func TestVec3Distance(t *testing.T) {
	tests := []struct {
		name string
		a, b Vec3
		want float32
	}{
		{"origin to point", Vec3{0, 0, 0}, Vec3{3, 4, 0}, 5},
		{"same point", Vec3{5, 5, 5}, Vec3{5, 5, 5}, 0},
		{"x axis", Vec3{0, 0, 0}, Vec3{10, 0, 0}, 10},
		{"y axis", Vec3{0, 0, 0}, Vec3{0, 10, 0}, 10},
		{"z axis", Vec3{0, 0, 0}, Vec3{0, 0, 10}, 10},
		{"diagonal", Vec3{0, 0, 0}, Vec3{1, 2, 2}, 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.a.Distance(tt.b)
			if got != tt.want {
				t.Errorf("Vec3.Distance() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestVec3XZ(t *testing.T) {
	tests := []struct {
		name string
		v    Vec3
		want Vec2
	}{
		{"positive", Vec3{1, 2, 3}, Vec2{1, 3}},
		{"zero y", Vec3{5, 0, 7}, Vec2{5, 7}},
		{"negative", Vec3{-1, 99, -3}, Vec2{-1, -3}},
		{"zero", Vec3{0, 0, 0}, Vec2{0, 0}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.v.XZ()
			if got != tt.want {
				t.Errorf("Vec3.XZ() = %v, want %v", got, tt.want)
			}
		})
	}
}
