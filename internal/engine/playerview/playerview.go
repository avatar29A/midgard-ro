// Package playerview renders the local player as a billboard sprite in
// the 3D scene.
//
// Today this is a procedural placeholder (a small humanoid colored quad)
// so the player can verify position + click-to-move + camera-facing
// orientation without the full SPR/ACT load path. The next PR on RFC #49
// Track B (#51) replaces the procedural texture with real Novice composite
// frames (body + head merged via internal/engine/sprite/composite.go).
package playerview

import (
	"github.com/go-gl/gl/v4.1-core/gl"

	"github.com/Faultbox/midgard-ro/internal/engine/character"
	"github.com/Faultbox/midgard-ro/internal/engine/scene"
	"github.com/Faultbox/midgard-ro/internal/engine/sprite"
	"github.com/Faultbox/midgard-ro/internal/game/entity"
	"github.com/Faultbox/midgard-ro/pkg/math"
)

// View renders a single player character as a billboard.
type View struct {
	char *entity.Character

	// Procedural sprite texture (placeholder).
	texture       uint32
	width, height int
	scale         float32
}

// New creates a player view backed by a procedural humanoid texture.
// Must be called on the GL thread (creates a texture).
func New(char *entity.Character) *View {
	const w, h = sprite.DefaultProceduralWidth, sprite.DefaultProceduralHeight

	pixels := sprite.GenerateProceduralPlayer(w, h)

	var tex uint32
	gl.GenTextures(1, &tex)
	gl.BindTexture(gl.TEXTURE_2D, tex)
	gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA, int32(w), int32(h), 0, gl.RGBA, gl.UNSIGNED_BYTE, gl.Ptr(pixels))
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.NEAREST)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.NEAREST)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
	gl.BindTexture(gl.TEXTURE_2D, 0)

	return &View{
		char:    char,
		texture: tex,
		width:   w,
		height:  h,
		scale:   sprite.DefaultProceduralScale,
	}
}

// SetCharacter swaps the underlying character (e.g. after a respawn).
func (v *View) SetCharacter(char *entity.Character) {
	v.char = char
}

// Render draws the player billboard at its render position. The billboard
// faces (camX, camZ) — pass the camera's world XZ.
func (v *View) Render(s *scene.Scene, viewProj math.Mat4, camX, camZ float32) {
	if v == nil || v.char == nil || v.texture == 0 || s == nil {
		return
	}
	px, py, pz := v.char.RenderPosition()
	right, up := character.BillboardVectors(camX, camZ, px, pz)

	spriteW := float32(v.width) * v.scale
	spriteH := float32(v.height) * v.scale

	s.RenderSprite(viewProj,
		math.Vec3{X: right[0], Y: right[1], Z: right[2]},
		math.Vec3{X: up[0], Y: up[1], Z: up[2]},
		[3]float32{px, py, pz},
		spriteW, spriteH,
		v.texture,
		[4]float32{1, 1, 1, 1},
	)
}

// Destroy releases the procedural texture.
func (v *View) Destroy() {
	if v == nil || v.texture == 0 {
		return
	}
	gl.DeleteTextures(1, &v.texture)
	v.texture = 0
}
