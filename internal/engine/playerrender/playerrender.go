// Package playerrender renders the local player character as a billboard
// inside the 3D scene framebuffer.
//
// This is a faithful port of cmd/grfbrowser/map_viewer.go's PlayMode
// rendering — the working reference Boris confirmed visually before. We
// own our own GL state (program, VAO, texture) rather than going through
// scene.SpriteRenderer. The shader source is shared with scene's sprite
// renderer (same vertex layout, same uniforms) so behavior matches.
//
// Today the texture is procedural (a small humanoid colored quad). The
// next PR replaces it with real Novice SPR/ACT composites loaded from the
// GRF.
package playerrender

import (
	"fmt"
	"unsafe"

	"github.com/go-gl/gl/v4.1-core/gl"

	"github.com/Faultbox/midgard-ro/internal/engine/character"
	"github.com/Faultbox/midgard-ro/internal/engine/scene/shaders"
	"github.com/Faultbox/midgard-ro/internal/engine/shader"
	"github.com/Faultbox/midgard-ro/internal/engine/sprite"
	"github.com/Faultbox/midgard-ro/internal/game/entity"
	"github.com/Faultbox/midgard-ro/pkg/math"
)

// Renderer owns the GL state for drawing the local player as a billboard.
type Renderer struct {
	// Shader program + uniform locations (mirror scene.SpriteRenderer's setup,
	// kept independent so we can render with our own VAO/draw pattern).
	program       uint32
	locViewProj   int32
	locWorldPos   int32
	locSpriteSize int32
	locCamRight   int32
	locCamUp      int32
	locTexture    int32
	locTint       int32

	// Billboard quad — 4 verts, drawn as TRIANGLE_STRIP (matches grfbrowser).
	vao uint32
	vbo uint32

	// Procedural humanoid texture.
	texture       uint32
	width, height int

	// Scale applied to (texturePixelsW, texturePixelsH) to get world units.
	scale float32
}

// New creates a renderer with a procedural humanoid texture.
// Must be called on the GL thread (creates shader program + VAO + texture).
func New() (*Renderer, error) {
	r := &Renderer{
		scale: sprite.DefaultProceduralScale,
	}

	// Compile sprite shader (same source scene.SpriteRenderer uses).
	prog, err := shader.CompileProgram(shaders.SpriteVertexShader, shaders.SpriteFragmentShader)
	if err != nil {
		return nil, fmt.Errorf("sprite shader: %w", err)
	}
	r.program = prog
	r.locViewProj = shader.GetUniform(prog, "uViewProj")
	r.locWorldPos = shader.GetUniform(prog, "uWorldPos")
	r.locSpriteSize = shader.GetUniform(prog, "uSpriteSize")
	r.locCamRight = shader.GetUniform(prog, "uCamRight")
	r.locCamUp = shader.GetUniform(prog, "uCamUp")
	r.locTexture = shader.GetUniform(prog, "uTexture")
	r.locTint = shader.GetUniform(prog, "uTint")

	// VAO/VBO. Vertex layout matches grfbrowser exactly:
	// foot-anchored quad (Y=0 at feet, Y=1 at head), TRIANGLE_STRIP order.
	gl.GenVertexArrays(1, &r.vao)
	gl.GenBuffers(1, &r.vbo)
	gl.BindVertexArray(r.vao)
	gl.BindBuffer(gl.ARRAY_BUFFER, r.vbo)

	verts := sprite.GenerateBillboardQuadVertices() // [TL, TR, BL, BR] × (pos, uv)
	gl.BufferData(gl.ARRAY_BUFFER, len(verts)*4, unsafe.Pointer(&verts[0]), gl.STATIC_DRAW)

	// position (location 0): vec2
	gl.VertexAttribPointerWithOffset(0, 2, gl.FLOAT, false, 4*4, 0)
	gl.EnableVertexAttribArray(0)
	// texcoord (location 1): vec2
	gl.VertexAttribPointerWithOffset(1, 2, gl.FLOAT, false, 4*4, 2*4)
	gl.EnableVertexAttribArray(1)

	gl.BindVertexArray(0)

	// Procedural texture.
	r.width = sprite.DefaultProceduralWidth
	r.height = sprite.DefaultProceduralHeight
	pixels := sprite.GenerateProceduralPlayer(r.width, r.height)

	gl.GenTextures(1, &r.texture)
	gl.BindTexture(gl.TEXTURE_2D, r.texture)
	gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA8, int32(r.width), int32(r.height), 0,
		gl.RGBA, gl.UNSIGNED_BYTE, gl.Ptr(pixels))
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.NEAREST)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.NEAREST)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
	gl.BindTexture(gl.TEXTURE_2D, 0)

	return r, nil
}

// Render draws the player billboard at the character's render position.
// camPosX/Z are the camera world XZ — used to orient the billboard.
//
// Mirrors cmd/grfbrowser/map_viewer.go renderPlayerCharacter (procedural
// path) including draw mode + state transitions.
func (r *Renderer) Render(viewProj math.Mat4, char *entity.Character, camPosX, camPosZ float32) {
	if r == nil || char == nil || r.program == 0 || r.vao == 0 || r.texture == 0 {
		return
	}

	right, up := character.BillboardVectors(camPosX, camPosZ, char.RenderX, char.RenderZ)

	spriteW := float32(r.width) * r.scale
	spriteH := float32(r.height) * r.scale

	gl.Enable(gl.BLEND)
	gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)
	gl.UseProgram(r.program)

	gl.UniformMatrix4fv(r.locViewProj, 1, false, &viewProj[0])
	gl.Uniform3f(r.locWorldPos, char.RenderX, char.RenderY, char.RenderZ)
	gl.Uniform2f(r.locSpriteSize, spriteW, spriteH)
	gl.Uniform4f(r.locTint, 1.0, 1.0, 1.0, 1.0)
	gl.Uniform3f(r.locCamRight, right[0], right[1], right[2])
	gl.Uniform3f(r.locCamUp, up[0], up[1], up[2])

	gl.ActiveTexture(gl.TEXTURE0)
	gl.BindTexture(gl.TEXTURE_2D, r.texture)
	gl.Uniform1i(r.locTexture, 0)

	gl.BindVertexArray(r.vao)
	gl.DrawArrays(gl.TRIANGLE_STRIP, 0, 4)
	gl.BindVertexArray(0)

	gl.Disable(gl.BLEND)
}

// Destroy releases all GL resources owned by the renderer.
func (r *Renderer) Destroy() {
	if r == nil {
		return
	}
	if r.texture != 0 {
		gl.DeleteTextures(1, &r.texture)
		r.texture = 0
	}
	if r.vbo != 0 {
		gl.DeleteBuffers(1, &r.vbo)
		r.vbo = 0
	}
	if r.vao != 0 {
		gl.DeleteVertexArrays(1, &r.vao)
		r.vao = 0
	}
	if r.program != 0 {
		gl.DeleteProgram(r.program)
		r.program = 0
	}
}
