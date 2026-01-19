// Package scene provides a reusable 3D scene rendering system.
package scene

import (
	"bytes"
	"fmt"
	"image"
	"strings"
	"unsafe"

	"github.com/go-gl/gl/v4.1-core/gl"

	"github.com/Faultbox/midgard-ro/internal/engine/scene/shaders"
	"github.com/Faultbox/midgard-ro/internal/engine/shader"
	"github.com/Faultbox/midgard-ro/internal/engine/texture"
	"github.com/Faultbox/midgard-ro/pkg/math"
)

// WaterRenderer handles water plane rendering.
type WaterRenderer struct {
	// Shader
	program uint32

	// Uniform locations
	locMVP       int32
	locWaterColor int32
	locTime      int32
	locWaterTex  int32
	locUseTexture int32

	// Mesh
	vao uint32
	vbo uint32

	// Water properties
	waterLevel    float32
	hasWater      bool
	waterTime     float32
	waterTextures []uint32
	waterFrame    int
	useWaterTex   bool
	waterAnimSpeed float32
}

// NewWaterRenderer creates a new water renderer.
func NewWaterRenderer() (*WaterRenderer, error) {
	wr := &WaterRenderer{
		waterAnimSpeed: 30.0,
	}

	program, err := shader.CompileProgram(shaders.WaterVertexShader, shaders.WaterFragmentShader)
	if err != nil {
		return nil, fmt.Errorf("water shader: %w", err)
	}
	wr.program = program

	// Get uniform locations
	wr.locMVP = shader.GetUniform(program, "uMVP")
	wr.locWaterColor = shader.GetUniform(program, "uWaterColor")
	wr.locTime = shader.GetUniform(program, "uTime")
	wr.locWaterTex = shader.GetUniform(program, "uWaterTex")
	wr.locUseTexture = shader.GetUniform(program, "uUseTexture")

	return wr, nil
}

// SetupWater creates a water plane at the specified level.
func (wr *WaterRenderer) SetupWater(level float32, minBounds, maxBounds [3]float32, texLoader func(string) ([]byte, error)) {
	wr.waterLevel = level
	wr.hasWater = true

	// Create water plane mesh
	wr.createWaterPlane(minBounds, maxBounds)

	// Load water textures
	wr.loadWaterTextures(texLoader)
}

func (wr *WaterRenderer) createWaterPlane(minBounds, maxBounds [3]float32) {
	// Extend water plane slightly beyond terrain bounds
	padding := float32(50.0)
	minX := minBounds[0] - padding
	maxX := maxBounds[0] + padding
	minZ := minBounds[2] - padding
	maxZ := maxBounds[2] + padding
	y := -wr.waterLevel

	// Simple quad vertices (position only)
	vertices := []float32{
		minX, y, minZ,
		maxX, y, minZ,
		maxX, y, maxZ,
		minX, y, maxZ,
		minX, y, minZ,
		maxX, y, maxZ,
	}

	// Create VAO/VBO
	gl.GenVertexArrays(1, &wr.vao)
	gl.BindVertexArray(wr.vao)

	gl.GenBuffers(1, &wr.vbo)
	gl.BindBuffer(gl.ARRAY_BUFFER, wr.vbo)
	gl.BufferData(gl.ARRAY_BUFFER, len(vertices)*4, unsafe.Pointer(&vertices[0]), gl.STATIC_DRAW)

	// Position attribute
	gl.VertexAttribPointerWithOffset(0, 3, gl.FLOAT, false, 3*4, 0)
	gl.EnableVertexAttribArray(0)

	gl.BindVertexArray(0)
}

func (wr *WaterRenderer) loadWaterTextures(texLoader func(string) ([]byte, error)) {
	var textures []uint32

	// Load 32 frames of water animation
	for frame := 0; frame < 32; frame++ {
		// RO water textures are in Korean folder name
		path := fmt.Sprintf("data/texture/워터/water%03d.jpg", frame)

		data, err := texLoader(path)
		if err != nil {
			continue
		}

		img, err := wr.decodeTexture(data, path)
		if err != nil {
			continue
		}

		texID := wr.uploadTexture(img)
		textures = append(textures, texID)
	}

	if len(textures) > 0 {
		wr.waterTextures = textures
		wr.useWaterTex = true
	}
}

func (wr *WaterRenderer) decodeTexture(data []byte, path string) (*image.RGBA, error) {
	lowerPath := strings.ToLower(path)
	var img image.Image
	var err error
	if strings.HasSuffix(lowerPath, ".tga") {
		img, err = texture.DecodeTGA(data)
	} else {
		img, _, err = image.Decode(bytes.NewReader(data))
	}
	if err != nil {
		return nil, err
	}
	return texture.ImageToRGBA(img, false), nil
}

func (wr *WaterRenderer) uploadTexture(img *image.RGBA) uint32 {
	var texID uint32
	gl.GenTextures(1, &texID)
	gl.BindTexture(gl.TEXTURE_2D, texID)
	gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA, int32(img.Bounds().Dx()), int32(img.Bounds().Dy()), 0, gl.RGBA, gl.UNSIGNED_BYTE, unsafe.Pointer(&img.Pix[0]))
	gl.GenerateMipmap(gl.TEXTURE_2D)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR_MIPMAP_LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.REPEAT)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.REPEAT)
	return texID
}

// HasWater returns whether water is enabled.
func (wr *WaterRenderer) HasWater() bool {
	return wr.hasWater
}

// Update updates water animation.
func (wr *WaterRenderer) Update(deltaTime float32) {
	if !wr.hasWater {
		return
	}

	wr.waterTime += deltaTime

	// Update animation frame
	if len(wr.waterTextures) > 0 {
		frameTime := 1000.0 / wr.waterAnimSpeed
		wr.waterFrame = int(wr.waterTime/frameTime) % len(wr.waterTextures)
	}
}

// Render renders the water plane.
func (wr *WaterRenderer) Render(viewProj math.Mat4) {
	if !wr.hasWater || wr.vao == 0 {
		return
	}

	gl.UseProgram(wr.program)

	// Enable blending for transparency
	gl.Enable(gl.BLEND)
	gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)

	// Set uniforms
	gl.UniformMatrix4fv(wr.locMVP, 1, false, &viewProj[0])
	gl.Uniform4f(wr.locWaterColor, 0.2, 0.4, 0.6, 0.7) // Blue-ish water color
	gl.Uniform1f(wr.locTime, wr.waterTime/1000.0)

	// Bind water texture if available
	if wr.useWaterTex && len(wr.waterTextures) > 0 {
		gl.Uniform1i(wr.locUseTexture, 1)
		gl.ActiveTexture(gl.TEXTURE0)
		gl.BindTexture(gl.TEXTURE_2D, wr.waterTextures[wr.waterFrame])
		gl.Uniform1i(wr.locWaterTex, 0)
	} else {
		gl.Uniform1i(wr.locUseTexture, 0)
	}

	gl.BindVertexArray(wr.vao)
	gl.DrawArrays(gl.TRIANGLES, 0, 6)
	gl.BindVertexArray(0)
}

// Destroy releases all resources.
func (wr *WaterRenderer) Destroy() {
	if wr.vao != 0 {
		gl.DeleteVertexArrays(1, &wr.vao)
		wr.vao = 0
	}
	if wr.vbo != 0 {
		gl.DeleteBuffers(1, &wr.vbo)
		wr.vbo = 0
	}
	for _, tex := range wr.waterTextures {
		if tex != 0 {
			gl.DeleteTextures(1, &tex)
		}
	}
	wr.waterTextures = nil
	if wr.program != 0 {
		gl.DeleteProgram(wr.program)
		wr.program = 0
	}
}
