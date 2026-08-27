package scene

import (
	"fmt"
	gomath "math"
	"unsafe"

	"github.com/go-gl/gl/v4.1-core/gl"

	"github.com/Faultbox/midgard-ro/internal/engine/scene/shaders"
	"github.com/Faultbox/midgard-ro/internal/engine/shader"
	"github.com/Faultbox/midgard-ro/internal/engine/texture"
	"github.com/Faultbox/midgard-ro/pkg/math"
)

// PortalRenderer draws the original client's warp portal — EF_WARPZONE2, the
// "Warp NPC" effect it shows for every NPC of class 45.
//
// No file in the archive describes the effect; the client draws it itself,
// and so do we. The geometry is the one roBrowser carries for it
// (Cylinder.js): a twenty-sided tube with separate top and bottom radii,
// wrapped in ring_blue.tga — pale blue spikes of light rising from the base —
// and spun about its axis at a quarter degree per millisecond. Under it a
// soft disc of the same blue, which the real-client frames show and the
// texture alone would not give.
//
// Sizes are in world units, ten to a cell. The tube stands a little wider
// than a character and about a character and a half tall, which is how it
// looks beside a Prontera door in the reference frames.
const (
	PortalRadius    = float32(5.5)  // at the base
	PortalTopRadius = float32(4.0)  // at the top; the tube leans in
	PortalHeight    = float32(16.0) // world units
	PortalSegments  = 20

	portalDiscRadius   = float32(7.5)
	portalDiscLift     = float32(0.4) // above the ground, out of the terrain's depth
	portalSpinDegPerMs = 0.25
)

// portalTint colors the light: the original's blue, at nine tenths.
var portalTint = [4]float32{0.55, 0.8, 1.0, 0.9}

// ringTexturePath is the tube's texture; the archive has it in one place.
const ringTexturePath = "data/texture/effect/ring_blue.tga"

// PortalRenderer holds the shader, the meshes and the textures. One serves
// every portal on the map.
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

	tubeVAO, tubeVBO uint32
	tubeVerts        int32
	discVAO, discVBO uint32
	discVerts        int32

	ringTex uint32
	discTex uint32
}

// NewPortalRenderer compiles the shader and builds the meshes. The ring
// texture comes later, from LoadTextures; the disc's is generated here.
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

	pr.tubeVAO, pr.tubeVBO, pr.tubeVerts = uploadMesh5(tubeVertices())
	pr.discVAO, pr.discVBO, pr.discVerts = uploadMesh5(discVertices())
	pr.discTex = uploadRGBA(discPixels(64), 64, 64, gl.CLAMP_TO_EDGE)

	return pr, nil
}

// tubeVertices is the unit tube as roBrowser builds it: for each segment a
// bottom vertex at angle i/n and a top vertex half a segment further round,
// so the spikes lean; u runs once around, v from 1 at the base to 0 at the
// top, which is the way up the texture is painted.
func tubeVertices() []float32 {
	n := PortalSegments
	bottom := make([][5]float32, n+1)
	top := make([][5]float32, n+1)
	for i := 0; i <= n; i++ {
		a := float64(i) / float64(n)
		b := (float64(i) + 0.5) / float64(n)
		bottom[i] = [5]float32{float32(gomath.Sin(a * 2 * gomath.Pi)), 0, float32(gomath.Cos(a * 2 * gomath.Pi)), float32(a), 1}
		top[i] = [5]float32{float32(gomath.Sin(b * 2 * gomath.Pi)), 1, float32(gomath.Cos(b * 2 * gomath.Pi)), float32(b), 0}
	}

	mesh := make([]float32, 0, n*6*5)
	push := func(v [5]float32) { mesh = append(mesh, v[:]...) }
	for i := 0; i < n; i++ {
		push(bottom[i])
		push(top[i])
		push(bottom[i+1])
		push(top[i])
		push(bottom[i+1])
		push(top[i+1])
	}
	return mesh
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

// discPixels is a soft disc: white-blue at the center, transparent at the
// rim, falling off with the square of the distance so the edge is a glow
// rather than a line.
func discPixels(size int) []byte {
	pix := make([]byte, size*size*4)
	half := float64(size) / 2
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			dx := (float64(x) + 0.5 - half) / half
			dy := (float64(y) + 0.5 - half) / half
			d := gomath.Sqrt(dx*dx + dy*dy)
			a := 1 - d
			if a < 0 {
				a = 0
			}
			a *= a
			i := (y*size + x) * 4
			pix[i], pix[i+1], pix[i+2] = 170, 215, 255
			pix[i+3] = byte(a * 220)
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

// uploadRGBA uploads a texture with linear filtering and the given
// horizontal wrap. The tube repeats around; the disc does not.
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

// LoadTextures reads the ring texture from the archives. Without it the
// portal is not drawn at all — a blank tube would be worse than nothing —
// and the error says which file was wanted.
func (pr *PortalRenderer) LoadTextures(load func(string) ([]byte, error)) error {
	data, err := load(ringTexturePath)
	if err != nil {
		return fmt.Errorf("loading %s: %w", ringTexturePath, err)
	}
	img, err := texture.DecodeTGA(data)
	if err != nil {
		return fmt.Errorf("decoding %s: %w", ringTexturePath, err)
	}
	rgba := texture.ImageToRGBA(img, false)
	b := rgba.Bounds()
	pr.ringTex = uploadRGBA(rgba.Pix, b.Dx(), b.Dy(), gl.REPEAT)
	return nil
}

// Ready reports whether the portal can be drawn.
func (pr *PortalRenderer) Ready() bool {
	return pr != nil && pr.ringTex != 0
}

// Render draws one portal standing on the ground at a world position, with
// its base on the ground. timeMs drives the spin; alpha fades it with the
// unit it belongs to.
//
// The light is added to what is behind it rather than painted over it, which
// is what makes it read as a glow, and it writes no depth so the units
// walking through it are not clipped by an invisible tube.
func (pr *PortalRenderer) Render(viewProj math.Mat4, x, y, z, timeMs, alpha float32) {
	if !pr.Ready() {
		return
	}

	gl.UseProgram(pr.program)
	gl.Enable(gl.BLEND)
	gl.BlendFunc(gl.SRC_ALPHA, gl.ONE)
	gl.DepthMask(false)
	gl.Disable(gl.CULL_FACE)

	gl.UniformMatrix4fv(pr.locViewProj, 1, false, &viewProj[0])
	// Added light over a sunlit doorway comes out white; the original's
	// portal is unmistakably blue, so the texture is tinted towards it and
	// held short of full strength.
	gl.Uniform4f(pr.locTint, portalTint[0], portalTint[1], portalTint[2], alpha*portalTint[3])
	gl.ActiveTexture(gl.TEXTURE0)
	gl.Uniform1i(pr.locTexture, 0)

	// The disc, flat on the ground.
	gl.BindTexture(gl.TEXTURE_2D, pr.discTex)
	gl.Uniform3f(pr.locPosition, x, y+portalDiscLift, z)
	gl.Uniform1f(pr.locBottomSize, portalDiscRadius)
	gl.Uniform1f(pr.locTopSize, portalDiscRadius)
	gl.Uniform1f(pr.locHeight, 0)
	gl.Uniform1f(pr.locSpin, 0)
	gl.BindVertexArray(pr.discVAO)
	gl.DrawArrays(gl.TRIANGLES, 0, pr.discVerts)

	// The tube, spinning.
	spin := float32(float64(timeMs) * portalSpinDegPerMs * gomath.Pi / 180)
	gl.BindTexture(gl.TEXTURE_2D, pr.ringTex)
	gl.Uniform3f(pr.locPosition, x, y, z)
	gl.Uniform1f(pr.locBottomSize, PortalRadius)
	gl.Uniform1f(pr.locTopSize, PortalTopRadius)
	gl.Uniform1f(pr.locHeight, PortalHeight)
	gl.Uniform1f(pr.locSpin, spin)
	gl.BindVertexArray(pr.tubeVAO)
	gl.DrawArrays(gl.TRIANGLES, 0, pr.tubeVerts)

	gl.BindVertexArray(0)
	gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)
	gl.DepthMask(true)
}

// Destroy releases the GPU resources.
func (pr *PortalRenderer) Destroy() {
	if pr == nil {
		return
	}
	for _, vao := range []*uint32{&pr.tubeVAO, &pr.discVAO} {
		if *vao != 0 {
			gl.DeleteVertexArrays(1, vao)
			*vao = 0
		}
	}
	for _, vbo := range []*uint32{&pr.tubeVBO, &pr.discVBO} {
		if *vbo != 0 {
			gl.DeleteBuffers(1, vbo)
			*vbo = 0
		}
	}
	for _, tex := range []*uint32{&pr.ringTex, &pr.discTex} {
		if *tex != 0 {
			gl.DeleteTextures(1, tex)
			*tex = 0
		}
	}
	if pr.program != 0 {
		gl.DeleteProgram(pr.program)
		pr.program = 0
	}
}
