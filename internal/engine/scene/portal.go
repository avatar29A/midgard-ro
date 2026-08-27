package scene

import (
	"fmt"
	gomath "math"
	"unsafe"

	"github.com/go-gl/gl/v4.1-core/gl"

	"github.com/Faultbox/midgard-ro/internal/engine/scene/shaders"
	"github.com/Faultbox/midgard-ro/internal/engine/shader"
	"github.com/Faultbox/midgard-ro/pkg/math"
)

// PortalRenderer draws the original client's warp portal — EF_WARPZONE2, the
// "Warp NPC" effect it shows for every NPC of class 45.
//
// No file in the archive describes the effect; the client draws it itself,
// and so do we. It lies flat on the ground rather than standing up: rings of
// pale blue light around a dark middle, a handful of bright sparkles set
// around them, the whole turning slowly and faint enough to read the floor
// through. The texture is generated rather than loaded, because the archive
// has no picture of it — what it does ship under that name (ring_blue.tga
// and its siblings) is the vertical spikes of other effects.
const (
	// PortalRadius is how far the light reaches from the warp's cell, in
	// world units — ten to a cell, so the effect is a little over four cells
	// across, which is how it measures against the characters standing beside
	// one in the reference captures.
	PortalRadius = float32(22.0)

	// PortalHeight is nothing that is drawn: the effect is flat. It is how
	// tall the warp's hit box stands, so a portal can be clicked without
	// having to aim at the floor.
	PortalHeight = float32(12.0)

	// portalLift keeps the disc off the terrain it lies on, out of the depth
	// buffer's way.
	portalLift = float32(2.5)

	// portalSpinDegPerMs turns the rings, slowly: a little over three seconds
	// for a full turn.
	portalSpinDegPerMs = 0.1

	// portalTexSize is the generated texture's side.
	portalTexSize = 256
)

// portalTint scales the generated colors. The texture already carries the
// blue, so this only holds the whole effect short of full strength.
var portalTint = [4]float32{1, 1, 1, 0.9}

// PortalRenderer holds the shader, the quad and the texture. One serves every
// portal on the map.
type PortalRenderer struct {
	program uint32

	locViewProj   int32
	locPosition   int32
	locBottomSize int32
	locTopSize    int32
	locHeight     int32
	locSpin       int32
	locTexture    int32
	locTint       int32

	discVAO, discVBO uint32
	discVerts        int32

	discTex uint32
}

// NewPortalRenderer compiles the shader, builds the quad and generates the
// texture. Nothing is read from the archive.
func NewPortalRenderer() (*PortalRenderer, error) {
	pr := &PortalRenderer{}

	program, err := shader.CompileProgram(shaders.PortalVertexShader, shaders.PortalFragmentShader)
	if err != nil {
		return nil, fmt.Errorf("portal shader: %w", err)
	}
	pr.program = program
	pr.locViewProj = shader.GetUniform(program, "uViewProj")
	pr.locPosition = shader.GetUniform(program, "uPosition")
	pr.locBottomSize = shader.GetUniform(program, "uBottomSize")
	pr.locTopSize = shader.GetUniform(program, "uTopSize")
	pr.locHeight = shader.GetUniform(program, "uHeight")
	pr.locSpin = shader.GetUniform(program, "uSpin")
	pr.locTexture = shader.GetUniform(program, "uTexture")
	pr.locTint = shader.GetUniform(program, "uTint")

	pr.discVAO, pr.discVBO, pr.discVerts = uploadMesh5(discVertices())
	pr.discTex = uploadRGBA(portalPixels(portalTexSize), portalTexSize, portalTexSize, gl.CLAMP_TO_EDGE)

	return pr, nil
}

// discVertices is a unit quad on the ground, textured corner to corner.
func discVertices() []float32 {
	return []float32{
		-1, 0, -1, 0, 0,
		1, 0, -1, 1, 0,
		1, 0, 1, 1, 1,
		-1, 0, -1, 0, 0,
		1, 0, 1, 1, 1,
		-1, 0, 1, 0, 1,
	}
}

// portalPixels draws the warp: concentric rings of pale light, a swirl
// through them, sparkles set around the widest ring, and a point at the
// middle. Everything else is transparent, so the floor shows through.
//
// It is generated rather than painted because the original generates it too,
// and because there is no picture of it in the archive to copy. The numbers
// are read off the reference captures: four rings, the third of them the
// widest and brightest, six sparkles a little inside it.
func portalPixels(size int) []byte {
	pix := make([]byte, size*size*4)
	half := float64(size) / 2

	// radius, half-width, strength — as fractions of the disc's own radius.
	// Three broad rings rather than four thin ones: on screen a portal is
	// sixty pixels across, and a ring any finer than this disappears into
	// the gap between two of them.
	rings := [...][3]float64{
		{0.34, 0.090, 0.55},
		{0.62, 0.075, 1.00},
		{0.88, 0.055, 0.70},
	}
	const (
		sparkles      = 6
		sparkleRadius = 0.74
		sparkleArm    = 0.13 // how far a star's points reach
		sparkleCore   = 0.025
	)

	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			dx := (float64(x) + 0.5 - half) / half
			dy := (float64(y) + 0.5 - half) / half
			d := gomath.Sqrt(dx*dx + dy*dy)
			if d > 1 {
				continue
			}
			angle := gomath.Atan2(dy, dx)

			var light float64
			for _, r := range rings {
				t := (d - r[0]) / r[1]
				light += r[2] * gomath.Exp(-t*t)
			}

			// The swirl. The rings are not even all the way round, which is
			// what makes the effect look like it is turning rather than
			// pulsing.
			light *= 0.50 + 0.50*gomath.Cos(angle*2+d*7)

			// A soft wash inside the widest ring, so the middle reads as
			// light rather than as a hole in the floor.
			light += 0.10 * gomath.Exp(-(d/0.55)*(d/0.55))

			// Sparkles: four-pointed stars, brightest at their core.
			var spark float64
			for i := 0; i < sparkles; i++ {
				a := 2 * gomath.Pi * float64(i) / sparkles
				ax := gomath.Abs(dx - sparkleRadius*gomath.Cos(a))
				ay := gomath.Abs(dy - sparkleRadius*gomath.Sin(a))
				arm := gomath.Exp(-(ax/sparkleCore)*(ax/sparkleCore)-(ay/sparkleArm)*(ay/sparkleArm)) +
					gomath.Exp(-(ax/sparkleArm)*(ax/sparkleArm)-(ay/sparkleCore)*(ay/sparkleCore))
				if arm > spark {
					spark = arm
				}
			}

			// The point at the middle.
			spark += gomath.Exp(-(d / 0.03) * (d / 0.03))

			// Fade out at the rim rather than cutting it off.
			edge := 1 - gomath.Max(0, (d-0.86)/0.14)
			alpha := (light*0.85 + spark) * edge
			if alpha <= 0 {
				continue
			}
			if alpha > 1 {
				alpha = 1
			}

			// The rings are blue; a sparkle's core burns out to white.
			white := gomath.Min(1, spark)
			i := (y*size + x) * 4
			pix[i] = byte(150 + 105*white)
			pix[i+1] = byte(205 + 50*white)
			pix[i+2] = 255
			pix[i+3] = byte(alpha * 255)
		}
	}
	return pix
}

// uploadMesh5 uploads position (3) + texcoord (2) vertices.
func uploadMesh5(vertices []float32) (vao, vbo uint32, count int32) {
	gl.GenVertexArrays(1, &vao)
	gl.BindVertexArray(vao)
	gl.GenBuffers(1, &vbo)
	gl.BindBuffer(gl.ARRAY_BUFFER, vbo)
	gl.BufferData(gl.ARRAY_BUFFER, len(vertices)*4, unsafe.Pointer(&vertices[0]), gl.STATIC_DRAW)
	gl.VertexAttribPointerWithOffset(0, 3, gl.FLOAT, false, 5*4, 0)
	gl.EnableVertexAttribArray(0)
	gl.VertexAttribPointerWithOffset(1, 2, gl.FLOAT, false, 5*4, 3*4)
	gl.EnableVertexAttribArray(1)
	gl.BindVertexArray(0)
	return vao, vbo, int32(len(vertices) / 5)
}

// uploadRGBA uploads a texture with linear filtering and the given horizontal
// wrap.
func uploadRGBA(pix []byte, width, height int, wrapS int32) uint32 {
	var tex uint32
	gl.GenTextures(1, &tex)
	gl.BindTexture(gl.TEXTURE_2D, tex)
	gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA, int32(width), int32(height), 0, gl.RGBA, gl.UNSIGNED_BYTE, gl.Ptr(pix))
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, wrapS)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
	return tex
}

// LoadTextures is kept for the caller's sake: the effect needs nothing from
// the archive, so there is nothing to load and nothing to fail.
func (pr *PortalRenderer) LoadTextures(func(string) ([]byte, error)) error {
	return nil
}

// Ready reports whether the portal can be drawn.
func (pr *PortalRenderer) Ready() bool {
	return pr != nil && pr.discTex != 0
}

// Render draws one portal, flat on the ground at a world position. timeMs
// turns it; alpha fades it with the unit it belongs to.
//
// It is blended over the floor rather than added to it. Added light looked
// right on the dark stone of a dungeon and destroyed the effect on Prontera's
// pale pavement, where everything above about a third of full brightness
// clipped to white and the rings disappeared into one smear. Blending keeps
// the pattern on any floor, which is what the reference captures show on
// both. Depth is not written, so a character standing on a portal is not
// clipped by it.
func (pr *PortalRenderer) Render(viewProj math.Mat4, x, y, z, timeMs, alpha float32) {
	if !pr.Ready() {
		return
	}

	gl.UseProgram(pr.program)
	gl.Enable(gl.BLEND)
	// Color blends over the floor; the destination alpha is left alone.
	//
	// The scene is drawn into an offscreen buffer that the interface later
	// composites by its alpha, and every other thing in it writes alpha 1.
	// Blending alpha the ordinary way punched the portal's own shape out of
	// that buffer, and what showed through the hole was the interface behind
	// it: hard bands of cyan and black in the middle of the street. Additive
	// blending had hidden the fault by saturating alpha to 1.
	gl.BlendFuncSeparate(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA, gl.ZERO, gl.ONE)
	gl.DepthMask(false)
	gl.Disable(gl.CULL_FACE)

	spin := float32(float64(timeMs) * portalSpinDegPerMs * gomath.Pi / 180)

	gl.UniformMatrix4fv(pr.locViewProj, 1, false, &viewProj[0])
	gl.Uniform4f(pr.locTint, portalTint[0], portalTint[1], portalTint[2], alpha*portalTint[3])
	gl.ActiveTexture(gl.TEXTURE0)
	gl.Uniform1i(pr.locTexture, 0)

	gl.BindTexture(gl.TEXTURE_2D, pr.discTex)
	gl.Uniform3f(pr.locPosition, x, y+portalLift, z)
	gl.Uniform1f(pr.locBottomSize, PortalRadius)
	gl.Uniform1f(pr.locTopSize, PortalRadius)
	gl.Uniform1f(pr.locHeight, 0)
	gl.Uniform1f(pr.locSpin, spin)
	gl.BindVertexArray(pr.discVAO)
	gl.DrawArrays(gl.TRIANGLES, 0, pr.discVerts)

	gl.BindVertexArray(0)
	gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)
	gl.DepthMask(true)
}

// Destroy releases the GPU resources.
func (pr *PortalRenderer) Destroy() {
	if pr == nil {
		return
	}
	if pr.discVAO != 0 {
		gl.DeleteVertexArrays(1, &pr.discVAO)
		pr.discVAO = 0
	}
	if pr.discVBO != 0 {
		gl.DeleteBuffers(1, &pr.discVBO)
		pr.discVBO = 0
	}
	if pr.discTex != 0 {
		gl.DeleteTextures(1, &pr.discTex)
		pr.discTex = 0
	}
	if pr.program != 0 {
		gl.DeleteProgram(pr.program)
		pr.program = 0
	}
}
