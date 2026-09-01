// Package effect plays RO's STR effect animations — what the game draws for a
// level-up, a warp, or a skill going off.
//
// The file is a stack of layers, each a textured quad with its own key frames.
// Playing one means working out, for a moment in time, where each layer's quad
// is and what color it is drawn in.
package effect

import (
	"github.com/Faultbox/midgard-ro/pkg/formats"
)

// Quad is one layer of an effect at one instant, ready to draw.
//
// The corners are offsets from the effect's origin in the original client's
// screen pixels, so the caller anchors the whole thing by projecting whatever
// the effect belongs to and adding these.
type Quad struct {
	// Texture is the file name of the layer's art, relative to the effect
	// texture directory.
	Texture string

	// Corners go clockwise from the top left, and UV matches them corner for
	// corner.
	Corners [4][2]float32
	UV      [4][2]float32

	// Color is RGBA in 0..1.
	Color [4]float32

	// Additive says to add to what is behind rather than cover it.
	Additive bool
}

// Origin is where the file's coordinates are measured from: the middle of the
// original client's 640x480 window, horizontally, with the vertical picked to
// put an effect's feet on the character's.
//
// The horizontal half is not a guess — every key frame in the level-up effect
// sits at x 320 exactly, which is that window's center line.
const (
	OriginX = 320
	OriginY = 290
)

// Frames is every quad of an effect at a moment in its playback.
//
// A layer that has not started or has already finished contributes nothing,
// which is what makes an effect build and fade rather than appear whole.
func Frames(str *formats.STR, elapsedMs float32) []Quad {
	if str == nil || str.FPS <= 0 {
		return nil
	}

	frame := elapsedMs * float32(str.FPS) / 1000

	quads := make([]Quad, 0, len(str.Layers))
	for i := range str.Layers {
		if quad, ok := layerQuad(&str.Layers[i], frame); ok {
			quads = append(quads, quad)
		}
	}

	return quads
}

// layerQuad works out one layer's quad at a frame.
//
// Key frames come in pairs: a basic one states the values, and the morph one
// after it gives how much each changes per frame until the next basic one.
// That is not a reading of the format taken on trust — in the level-up effect
// a layer's basic key sits at y 81 on frame 10, its morph key carries 32.25,
// and the next basic key at frame 14 is at 210, which is 81 plus four steps of
// 32.25 exactly. Color and corners check out the same way.
func layerQuad(layer *formats.STRLayer, frame float32) (Quad, bool) {
	key, morph, ok := keyAt(layer, frame)
	if !ok {
		return Quad{}, false
	}

	steps := frame - float32(key.Frame)

	x := key.X + morph.X*steps
	y := key.Y + morph.Y*steps

	var quad Quad
	for i := range quad.Corners {
		quad.Corners[i] = [2]float32{
			x - OriginX + key.XY[i] + morph.XY[i]*steps,
			y - OriginY + key.XY[4+i] + morph.XY[4+i]*steps,
		}

	}

	quad.UV = texCorners(key, morph, steps)

	for i := range quad.Color {
		quad.Color[i] = clamp01((key.Color[i] + morph.Color[i]*steps) / 255)
	}

	// Fully transparent is not worth a draw call, and the last key of a layer
	// is usually exactly that — the fade has finished.
	if quad.Color[3] <= 0 {
		return Quad{}, false
	}

	quad.Texture = textureAt(layer, key.AnimFrame+morph.AnimFrame*steps)
	if quad.Texture == "" {
		return Quad{}, false
	}

	quad.Additive = key.Additive()

	return quad, true
}

// texCorners maps the texture onto the quad's four corners.
//
// The eight floats are a source rectangle — u0, v0, u1, v1 — written twice
// rather than a coordinate per corner. Every effect in the archive stores the
// same value, (0, 0, 1, 1) twice, so the two readings cannot be told apart
// from the data; what settles it is that the per-corner reading gives two
// corners the same coordinate and smears the texture along a diagonal, which
// is exactly what the level-up flash looked like before this.
func texCorners(key, morph formats.STRKey, steps float32) [4][2]float32 {
	u0 := key.U[0] + morph.U[0]*steps
	v0 := key.U[1] + morph.U[1]*steps
	u1 := key.U[2] + morph.U[2]*steps
	v1 := key.U[3] + morph.U[3]*steps

	// Clockwise from the top left, matching the corner order the quad's own
	// coordinates come in.
	return [4][2]float32{{u0, v0}, {u1, v0}, {u1, v1}, {u0, v1}}
}

// keyAt finds the basic key in force at a frame, and the morph key that
// follows it.
//
// The morph is returned zeroed when there is none, so the caller can add its
// deltas unconditionally rather than branching on every field.
func keyAt(layer *formats.STRLayer, frame float32) (key, morph formats.STRKey, ok bool) {
	found := -1

	for i := range layer.Keys {
		if layer.Keys[i].Kind != formats.STRKeyBasic {
			continue
		}

		if float32(layer.Keys[i].Frame) > frame {
			break
		}

		found = i
	}

	if found < 0 {
		return key, morph, false
	}

	key = layer.Keys[found]

	// Past the last basic key the layer is over. Without this the final key's
	// values would hold on screen forever, which for a level-up means a flash
	// that never goes out.
	if found == lastBasic(layer) && frame > float32(key.Frame) {
		return key, morph, false
	}

	if found+1 < len(layer.Keys) && layer.Keys[found+1].Kind == formats.STRKeyMorph {
		morph = layer.Keys[found+1]
	}

	return key, morph, true
}

// lastBasic is the index of the layer's final basic key.
func lastBasic(layer *formats.STRLayer) int {
	for i := len(layer.Keys) - 1; i >= 0; i-- {
		if layer.Keys[i].Kind == formats.STRKeyBasic {
			return i
		}
	}

	return -1
}

// textureAt picks which of a layer's textures to draw.
//
// Most layers have exactly one. The ones that have several step through them
// as the animation frame advances, and a step past the end holds the last
// rather than wrapping — an effect that looped its art would keep flickering
// after everything else had faded.
func textureAt(layer *formats.STRLayer, animFrame float32) string {
	if len(layer.Textures) == 0 {
		return ""
	}

	index := int(animFrame)
	if index < 0 {
		index = 0
	}
	if index >= len(layer.Textures) {
		index = len(layer.Textures) - 1
	}

	return layer.Textures[index]
}

func clamp01(v float32) float32 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}

	return v
}
