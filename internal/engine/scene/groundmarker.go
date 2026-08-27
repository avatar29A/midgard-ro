package scene

import (
	"fmt"
	"image"
	"image/draw"

	"github.com/go-gl/gl/v4.1-core/gl"

	"github.com/Faultbox/midgard-ro/internal/engine/scene/shaders"
	"github.com/Faultbox/midgard-ro/internal/engine/shader"
	"github.com/Faultbox/midgard-ro/internal/engine/texture"
	"github.com/Faultbox/midgard-ro/pkg/math"
)

// GroundMarkerTexture is the green square RO puts under the cursor.
//
// It is a texture on the ground rather than a cursor state: the cursor has its
// own sprite with its own states, and this lies flat on the terrain and follows
// the cell, not the pointer.
const GroundMarkerTexture = "data/texture/grid.tga"

// The marker covers exactly one cell, lifted clear of the terrain it lies on.
const (
	// markerLift keeps the quad out of the depth buffer's way, the same
	// problem the warp portal has and for the same reason.
	markerLift = float32(1.5)

	// MarkerPulseMs is how long the click flourish runs.
	MarkerPulseMs = float32(320)

	// markerPulseScale is how much bigger the marker gets at the peak of that
	// flourish before settling back.
	markerPulseScale = float32(0.45)
)

// markerTint is the green the marker draws in.
//
// grid.tga is a white mask — 224 opaque pixels of corner bracket in a 32x32
// square, and the rest fully transparent — so the color is the client's to
// supply. Drawn untinted it is white on pale pavement at about a cell wide,
// which is to say invisible: the first version of this drew correctly and
// could not be seen at all.
var markerTint = [4]float32{0.35, 1.0, 0.4, 0.9}

// GroundMarker draws the cell marker under the cursor.
//
// It shares the warp portal's shader, which places a unit mesh at a world
// position with a size and a spin — with no height and no spin that is exactly
// a flat square on the ground.
type GroundMarker struct {
	program uint32

	locViewProj   int32
	locPosition   int32
	locBottomSize int32
	locTopSize    int32
	locHeight     int32
	locSpin       int32
	locTexture    int32
	locTint       int32

	vao, vbo uint32
	verts    int32

	tex uint32
}

// NewGroundMarker compiles the shader and builds the quad. The texture is
// loaded separately, once the archives are available.
//
// Must be called on the GL thread.
func NewGroundMarker() (*GroundMarker, error) {
	m := &GroundMarker{}

	program, err := shader.CompileProgram(shaders.PortalVertexShader, shaders.PortalFragmentShader)
	if err != nil {
		return nil, fmt.Errorf("ground marker shader: %w", err)
	}
	m.program = program

	m.locViewProj = shader.GetUniform(program, "uViewProj")
	m.locPosition = shader.GetUniform(program, "uPosition")
	m.locBottomSize = shader.GetUniform(program, "uBottomSize")
	m.locTopSize = shader.GetUniform(program, "uTopSize")
	m.locHeight = shader.GetUniform(program, "uHeight")
	m.locSpin = shader.GetUniform(program, "uSpin")
	m.locTexture = shader.GetUniform(program, "uTexture")
	m.locTint = shader.GetUniform(program, "uTint")

	// The same unit quad the portal's disc uses: flat on the ground, textured
	// corner to corner.
	m.vao, m.vbo, m.verts = uploadMesh5(discVertices())

	return m, nil
}

// LoadTexture reads the marker's texture from the archives.
//
// A miss is reported rather than swallowed: without it the marker silently
// stops appearing, and a click with no feedback is exactly what this step is
// meant to fix.
func (m *GroundMarker) LoadTexture(load func(string) ([]byte, error)) error {
	if m == nil {
		return nil
	}

	data, err := load(GroundMarkerTexture)
	if err != nil {
		return fmt.Errorf("ground marker texture %q: %w", GroundMarkerTexture, err)
	}

	img, err := texture.DecodeTGA(data)
	if err != nil {
		return fmt.Errorf("decode %q: %w", GroundMarkerTexture, err)
	}

	bounds := img.Bounds()
	rgba, ok := img.(*image.RGBA)
	if !ok {
		rgba = image.NewRGBA(bounds)
		draw.Draw(rgba, bounds, img, bounds.Min, draw.Src)
	}

	m.tex = uploadRGBA(rgba.Pix, bounds.Dx(), bounds.Dy(), gl.CLAMP_TO_EDGE)

	return nil
}

// Ready reports whether the marker has everything it needs to draw.
func (m *GroundMarker) Ready() bool {
	return m != nil && m.program != 0 && m.tex != 0
}

// MarkerScale is how large the marker draws for a given pulse progress.
//
// Progress runs 0 to 1 across the flourish that follows a click. The marker
// swells and settles rather than growing steadily, so the movement reads as a
// response to the click instead of a resize.
func MarkerScale(progress float32) float32 {
	if progress <= 0 || progress >= 1 {
		return 1
	}

	// A single arch: zero at both ends, one in the middle.
	arch := 4 * progress * (1 - progress)

	return 1 + markerPulseScale*arch
}

// Render draws the marker flat on the ground at a world position.
//
// size is the cell's width in world units; pulse runs 0 to 1 across the click
// flourish, or 1 for a marker that is only following the cursor.
func (m *GroundMarker) Render(viewProj math.Mat4, x, y, z, size, pulse, alpha float32) {
	if !m.Ready() || alpha <= 0 {
		return
	}

	half := size / 2 * MarkerScale(pulse)

	gl.UseProgram(m.program)
	gl.Enable(gl.BLEND)

	// The scene is drawn into an offscreen buffer the interface composites by
	// its alpha, and everything else in it writes alpha 1. Blending alpha the
	// ordinary way punches this quad's shape out of that buffer and shows the
	// interface through the hole — the fault the warp portal hit first.
	gl.BlendFuncSeparate(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA, gl.ZERO, gl.ONE)

	// Not written, so a character standing on the marker is not clipped by it.
	gl.DepthMask(false)
	gl.Disable(gl.CULL_FACE)

	gl.UniformMatrix4fv(m.locViewProj, 1, false, &viewProj[0])
	gl.Uniform3f(m.locPosition, x, y+markerLift, z)
	gl.Uniform1f(m.locBottomSize, half)
	gl.Uniform1f(m.locTopSize, half)
	gl.Uniform1f(m.locHeight, 0)
	gl.Uniform1f(m.locSpin, 0)
	gl.Uniform4f(m.locTint, markerTint[0], markerTint[1], markerTint[2], markerTint[3]*alpha)

	gl.ActiveTexture(gl.TEXTURE0)
	gl.BindTexture(gl.TEXTURE_2D, m.tex)
	gl.Uniform1i(m.locTexture, 0)

	gl.BindVertexArray(m.vao)
	gl.DrawArrays(gl.TRIANGLES, 0, m.verts)
	gl.BindVertexArray(0)

	gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)
	gl.DepthMask(true)
}

// Destroy releases the GPU resources.
func (m *GroundMarker) Destroy() {
	if m == nil {
		return
	}
	if m.tex != 0 {
		gl.DeleteTextures(1, &m.tex)
		m.tex = 0
	}
	if m.vbo != 0 {
		gl.DeleteBuffers(1, &m.vbo)
		m.vbo = 0
	}
	if m.vao != 0 {
		gl.DeleteVertexArrays(1, &m.vao)
		m.vao = 0
	}
	if m.program != 0 {
		gl.DeleteProgram(m.program)
		m.program = 0
	}
}
