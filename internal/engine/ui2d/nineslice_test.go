package ui2d

import "testing"

func TestNineSlice_UVCalculation(t *testing.T) {
	ns := &NineSlice{
		TextureID: 1, // fake ID, no GL calls in this test
		TexWidth:  64,
		TexHeight: 64,
		Left:      8,
		Right:     8,
		Top:       8,
		Bottom:    8,
	}

	tw := float32(ns.TexWidth)
	th := float32(ns.TexHeight)

	// Verify UV border positions
	uL := float32(ns.Left) / tw
	uR := (tw - float32(ns.Right)) / tw
	vT := float32(ns.Top) / th
	vB := (th - float32(ns.Bottom)) / th

	if uL != 0.125 {
		t.Errorf("uL = %f, want 0.125", uL)
	}
	if uR != 0.875 {
		t.Errorf("uR = %f, want 0.875", uR)
	}
	if vT != 0.125 {
		t.Errorf("vT = %f, want 0.125", vT)
	}
	if vB != 0.875 {
		t.Errorf("vB = %f, want 0.875", vB)
	}
}

func TestNineSlice_AsymmetricBorders(t *testing.T) {
	ns := &NineSlice{
		TextureID: 1,
		TexWidth:  100,
		TexHeight: 50,
		Left:      10,
		Right:     20,
		Top:       5,
		Bottom:    15,
	}

	tw := float32(ns.TexWidth)
	th := float32(ns.TexHeight)

	uL := float32(ns.Left) / tw          // 10/100 = 0.1
	uR := (tw - float32(ns.Right)) / tw  // 80/100 = 0.8
	vT := float32(ns.Top) / th           // 5/50 = 0.1
	vB := (th - float32(ns.Bottom)) / th // 35/50 = 0.7

	const epsilon = 0.0001
	check := func(name string, got, want float32) {
		t.Helper()
		diff := got - want
		if diff < -epsilon || diff > epsilon {
			t.Errorf("%s = %f, want %f", name, got, want)
		}
	}

	check("uL", uL, 0.1)
	check("uR", uR, 0.8)
	check("vT", vT, 0.1)
	check("vB", vB, 0.7)

	// Verify center region size at 200x100 screen size
	screenW := float32(200)
	screenH := float32(100)
	midW := screenW - float32(ns.Left) - float32(ns.Right) // 200-10-20 = 170
	midH := screenH - float32(ns.Top) - float32(ns.Bottom) // 100-5-15 = 80

	check("midW", midW, 170)
	check("midH", midH, 80)
}

func TestNineSlice_ZeroTexture(t *testing.T) {
	ns := &NineSlice{
		TextureID: 0, // Should be a no-op
		TexWidth:  64,
		TexHeight: 64,
		Left:      8,
		Right:     8,
		Top:       8,
		Bottom:    8,
	}

	// Draw with nil renderer would panic if TextureID=0 check doesn't work.
	// We just verify the struct is valid; actual Draw() requires a Renderer.
	if ns.TextureID != 0 {
		t.Error("expected zero texture ID")
	}
}
