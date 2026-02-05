package ui2d

// NineSlice describes a texture split into 9 regions for scalable UI frames.
// Corners stay fixed size, edges stretch along one axis, center stretches both.
type NineSlice struct {
	TextureID uint32
	TexWidth  int
	TexHeight int
	// Border insets in pixels from texture edges
	Left   int
	Right  int
	Top    int
	Bottom int
}

// Draw renders the nine-slice at the given screen position and size.
// All 9 quads share the same texture so the renderer batches them into one GL draw call.
func (ns *NineSlice) Draw(r *Renderer, x, y, w, h float32, tint Color) {
	if ns.TextureID == 0 {
		return
	}

	tw := float32(ns.TexWidth)
	th := float32(ns.TexHeight)
	l := float32(ns.Left)
	ri := float32(ns.Right)
	t := float32(ns.Top)
	b := float32(ns.Bottom)

	// UV coordinates for the border cuts
	uL := l / tw
	uR := (tw - ri) / tw
	vT := t / th
	vB := (th - b) / th

	// Screen coordinates for the inner region
	midW := w - l - ri
	midH := h - t - b

	// Top-left corner
	r.DrawImageUV(ns.TextureID, x, y, l, t, 0, 0, uL, vT, tint)
	// Top edge
	r.DrawImageUV(ns.TextureID, x+l, y, midW, t, uL, 0, uR, vT, tint)
	// Top-right corner
	r.DrawImageUV(ns.TextureID, x+l+midW, y, ri, t, uR, 0, 1, vT, tint)

	// Left edge
	r.DrawImageUV(ns.TextureID, x, y+t, l, midH, 0, vT, uL, vB, tint)
	// Center
	r.DrawImageUV(ns.TextureID, x+l, y+t, midW, midH, uL, vT, uR, vB, tint)
	// Right edge
	r.DrawImageUV(ns.TextureID, x+l+midW, y+t, ri, midH, uR, vT, 1, vB, tint)

	// Bottom-left corner
	r.DrawImageUV(ns.TextureID, x, y+t+midH, l, b, 0, vB, uL, 1, tint)
	// Bottom edge
	r.DrawImageUV(ns.TextureID, x+l, y+t+midH, midW, b, uL, vB, uR, 1, tint)
	// Bottom-right corner
	r.DrawImageUV(ns.TextureID, x+l+midW, y+t+midH, ri, b, uR, vB, 1, 1, tint)
}
