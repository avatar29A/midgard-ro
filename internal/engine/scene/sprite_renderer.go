// Package scene provides a reusable 3D scene rendering system.
package scene

import (
	"fmt"
	"unsafe"

	"github.com/go-gl/gl/v4.1-core/gl"

	"github.com/Faultbox/midgard-ro/internal/engine/scene/shaders"
	"github.com/Faultbox/midgard-ro/internal/engine/shader"
	"github.com/Faultbox/midgard-ro/pkg/math"
)

// SpriteRenderer handles billboard sprite rendering for characters and effects.
type SpriteRenderer struct {
	// Shader
	program uint32

	// Uniform locations
	locViewProj  int32
	locWorldPos  int32
	locSpriteSize int32
	locCamRight  int32
	locCamUp     int32
	locTexture   int32
	locTint      int32

	// Billboard quad mesh
	vao uint32
	vbo uint32
}

// NewSpriteRenderer creates a new sprite renderer.
func NewSpriteRenderer() (*SpriteRenderer, error) {
	sr := &SpriteRenderer{}

	program, err := shader.CompileProgram(shaders.SpriteVertexShader, shaders.SpriteFragmentShader)
	if err != nil {
		return nil, fmt.Errorf("sprite shader: %w", err)
	}
	sr.program = program

	// Get uniform locations
	sr.locViewProj = shader.GetUniform(program, "uViewProj")
	sr.locWorldPos = shader.GetUniform(program, "uWorldPos")
	sr.locSpriteSize = shader.GetUniform(program, "uSpriteSize")
	sr.locCamRight = shader.GetUniform(program, "uCamRight")
	sr.locCamUp = shader.GetUniform(program, "uCamUp")
	sr.locTexture = shader.GetUniform(program, "uTexture")
	sr.locTint = shader.GetUniform(program, "uTint")

	// Create billboard quad
	sr.createQuad()

	return sr, nil
}

func (sr *SpriteRenderer) createQuad() {
	// Billboard quad vertices: position (2D) + texcoord
	// The quad is centered at origin, shader expands it based on camera vectors
	vertices := []float32{
		// Position (XY), TexCoord (UV)
		-0.5, 0.0, 0.0, 1.0, // Bottom-left
		0.5, 0.0, 1.0, 1.0,  // Bottom-right
		0.5, 1.0, 1.0, 0.0,  // Top-right
		-0.5, 0.0, 0.0, 1.0, // Bottom-left
		0.5, 1.0, 1.0, 0.0,  // Top-right
		-0.5, 1.0, 0.0, 0.0, // Top-left
	}

	gl.GenVertexArrays(1, &sr.vao)
	gl.BindVertexArray(sr.vao)

	gl.GenBuffers(1, &sr.vbo)
	gl.BindBuffer(gl.ARRAY_BUFFER, sr.vbo)
	gl.BufferData(gl.ARRAY_BUFFER, len(vertices)*4, unsafe.Pointer(&vertices[0]), gl.STATIC_DRAW)

	// Position attribute (location 0)
	gl.VertexAttribPointerWithOffset(0, 2, gl.FLOAT, false, 4*4, 0)
	gl.EnableVertexAttribArray(0)

	// TexCoord attribute (location 1)
	gl.VertexAttribPointerWithOffset(1, 2, gl.FLOAT, false, 4*4, 2*4)
	gl.EnableVertexAttribArray(1)

	gl.BindVertexArray(0)
}

// Render renders a sprite at the given world position.
func (sr *SpriteRenderer) Render(viewProj math.Mat4, camRight, camUp math.Vec3, worldPos [3]float32, width, height float32, textureID uint32, tint [4]float32) {
	if sr.vao == 0 {
		return
	}

	gl.UseProgram(sr.program)

	// Enable blending
	gl.Enable(gl.BLEND)
	gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)

	// Disable depth writing for sprites (but keep depth testing)
	gl.DepthMask(false)

	// Set uniforms
	gl.UniformMatrix4fv(sr.locViewProj, 1, false, &viewProj[0])
	gl.Uniform3f(sr.locWorldPos, worldPos[0], worldPos[1], worldPos[2])
	gl.Uniform2f(sr.locSpriteSize, width, height)
	gl.Uniform3f(sr.locCamRight, camRight.X, camRight.Y, camRight.Z)
	gl.Uniform3f(sr.locCamUp, camUp.X, camUp.Y, camUp.Z)
	gl.Uniform4f(sr.locTint, tint[0], tint[1], tint[2], tint[3])

	// Bind texture
	gl.ActiveTexture(gl.TEXTURE0)
	gl.BindTexture(gl.TEXTURE_2D, textureID)
	gl.Uniform1i(sr.locTexture, 0)

	// Draw
	gl.BindVertexArray(sr.vao)
	gl.DrawArrays(gl.TRIANGLES, 0, 6)
	gl.BindVertexArray(0)

	// Restore depth writing
	gl.DepthMask(true)
}

// Destroy releases all resources.
func (sr *SpriteRenderer) Destroy() {
	if sr.vao != 0 {
		gl.DeleteVertexArrays(1, &sr.vao)
		sr.vao = 0
	}
	if sr.vbo != 0 {
		gl.DeleteBuffers(1, &sr.vbo)
		sr.vbo = 0
	}
	if sr.program != 0 {
		gl.DeleteProgram(sr.program)
		sr.program = 0
	}
}
