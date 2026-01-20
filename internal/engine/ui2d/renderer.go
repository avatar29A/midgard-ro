// Package ui2d provides a simple 2D UI rendering layer using OpenGL.
// This replaces ImGui to avoid viewport/window separation issues.
package ui2d

import (
	"fmt"
	"strings"
	"unsafe"

	"github.com/go-gl/gl/v4.1-core/gl"
)

// Renderer handles 2D UI rendering with OpenGL.
type Renderer struct {
	screenWidth  int
	screenHeight int

	// Shader program for solid color quads
	solidShader uint32

	// Shader program for textured quads
	textShader uint32

	// Shader program for scene texture (full RGBA)
	sceneShader uint32

	// VAO/VBO for solid quad rendering
	solidVAO uint32
	solidVBO uint32

	// VAO/VBO for textured quad rendering (text)
	textVAO uint32
	textVBO uint32

	// VAO/VBO for scene texture rendering
	sceneVAO uint32
	sceneVBO uint32

	// Current draw lists
	solidVertices []float32
	textVertices  []float32

	// Font for text rendering
	font *Font

	// Current text texture (for batching)
	currentTextTex uint32
}

// New creates a new 2D UI renderer.
func New(width, height int) (*Renderer, error) {
	r := &Renderer{
		screenWidth:   width,
		screenHeight:  height,
		solidVertices: make([]float32, 0, 4096),
		textVertices:  make([]float32, 0, 4096),
	}

	// Create solid color shader
	var err error
	r.solidShader, err = r.createSolidShader()
	if err != nil {
		return nil, fmt.Errorf("create solid shader: %w", err)
	}

	// Create text shader
	r.textShader, err = r.createTextShader()
	if err != nil {
		return nil, fmt.Errorf("create text shader: %w", err)
	}

	// Create VAO/VBO for solid quads
	if err := r.createSolidBuffers(); err != nil {
		return nil, fmt.Errorf("create solid buffers: %w", err)
	}

	// Create VAO/VBO for textured quads
	if err := r.createTextBuffers(); err != nil {
		return nil, fmt.Errorf("create text buffers: %w", err)
	}

	// Create scene shader (full RGBA sampling for 3D scene texture)
	r.sceneShader, err = r.createSceneShader()
	if err != nil {
		return nil, fmt.Errorf("create scene shader: %w", err)
	}

	// Create VAO/VBO for scene texture rendering
	if err := r.createSceneBuffers(); err != nil {
		return nil, fmt.Errorf("create scene buffers: %w", err)
	}

	// Create font
	r.font = NewFont()

	return r, nil
}

// Resize updates the screen dimensions.
func (r *Renderer) Resize(width, height int) {
	r.screenWidth = width
	r.screenHeight = height
}

// GetScreenSize returns the current screen dimensions.
func (r *Renderer) GetScreenSize() (int, int) {
	return r.screenWidth, r.screenHeight
}

// Begin starts a new UI frame.
func (r *Renderer) Begin() {
	r.solidVertices = r.solidVertices[:0]
	r.textVertices = r.textVertices[:0]
}

// End finishes the UI frame and renders all queued elements.
func (r *Renderer) End() {
	// Save OpenGL state
	var prevBlend int32
	var prevDepth int32
	var prevCull int32
	gl.GetIntegerv(gl.BLEND, &prevBlend)
	gl.GetIntegerv(gl.DEPTH_TEST, &prevDepth)
	gl.GetIntegerv(gl.CULL_FACE, &prevCull)

	// Setup state for 2D rendering
	gl.Enable(gl.BLEND)
	gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)
	gl.Disable(gl.DEPTH_TEST)
	gl.Disable(gl.CULL_FACE)

	proj := r.orthoMatrix(0, float32(r.screenWidth), float32(r.screenHeight), 0, -1, 1)

	// Render solid quads first
	if len(r.solidVertices) > 0 {
		gl.UseProgram(r.solidShader)
		projLoc := gl.GetUniformLocation(r.solidShader, gl.Str("uProjection\x00"))
		gl.UniformMatrix4fv(projLoc, 1, false, &proj[0])

		gl.BindVertexArray(r.solidVAO)
		gl.BindBuffer(gl.ARRAY_BUFFER, r.solidVBO)
		gl.BufferData(gl.ARRAY_BUFFER, len(r.solidVertices)*4, unsafe.Pointer(&r.solidVertices[0]), gl.STREAM_DRAW)
		gl.DrawArrays(gl.TRIANGLES, 0, int32(len(r.solidVertices)/7)) // 7 floats per vertex
	}

	// Render textured quads (text) on top
	if len(r.textVertices) > 0 && r.font != nil {
		gl.UseProgram(r.textShader)
		projLoc := gl.GetUniformLocation(r.textShader, gl.Str("uProjection\x00"))
		gl.UniformMatrix4fv(projLoc, 1, false, &proj[0])

		texLoc := gl.GetUniformLocation(r.textShader, gl.Str("uTexture\x00"))
		gl.Uniform1i(texLoc, 0)

		gl.ActiveTexture(gl.TEXTURE0)
		gl.BindTexture(gl.TEXTURE_2D, r.font.TextureID())

		gl.BindVertexArray(r.textVAO)
		gl.BindBuffer(gl.ARRAY_BUFFER, r.textVBO)
		gl.BufferData(gl.ARRAY_BUFFER, len(r.textVertices)*4, unsafe.Pointer(&r.textVertices[0]), gl.STREAM_DRAW)
		gl.DrawArrays(gl.TRIANGLES, 0, int32(len(r.textVertices)/9)) // 9 floats per vertex (pos3 + uv2 + color4)
	}

	// Restore state
	gl.BindVertexArray(0)
	gl.BindTexture(gl.TEXTURE_2D, 0)
	gl.UseProgram(0)

	if prevBlend == gl.FALSE {
		gl.Disable(gl.BLEND)
	}
	if prevDepth == gl.TRUE {
		gl.Enable(gl.DEPTH_TEST)
	}
	if prevCull == gl.TRUE {
		gl.Enable(gl.CULL_FACE)
	}
}

// Close releases renderer resources.
func (r *Renderer) Close() {
	if r.font != nil {
		r.font.Close()
	}
	if r.solidVAO != 0 {
		gl.DeleteVertexArrays(1, &r.solidVAO)
	}
	if r.solidVBO != 0 {
		gl.DeleteBuffers(1, &r.solidVBO)
	}
	if r.textVAO != 0 {
		gl.DeleteVertexArrays(1, &r.textVAO)
	}
	if r.textVBO != 0 {
		gl.DeleteBuffers(1, &r.textVBO)
	}
	if r.sceneVAO != 0 {
		gl.DeleteVertexArrays(1, &r.sceneVAO)
	}
	if r.sceneVBO != 0 {
		gl.DeleteBuffers(1, &r.sceneVBO)
	}
	if r.solidShader != 0 {
		gl.DeleteProgram(r.solidShader)
	}
	if r.textShader != 0 {
		gl.DeleteProgram(r.textShader)
	}
	if r.sceneShader != 0 {
		gl.DeleteProgram(r.sceneShader)
	}
}

// DrawRect draws a filled rectangle.
func (r *Renderer) DrawRect(x, y, width, height float32, color Color) {
	r.addQuad(x, y, width, height, color)
}

// DrawRectOutline draws a rectangle outline.
func (r *Renderer) DrawRectOutline(x, y, width, height, thickness float32, color Color) {
	// Top
	r.addQuad(x, y, width, thickness, color)
	// Bottom
	r.addQuad(x, y+height-thickness, width, thickness, color)
	// Left
	r.addQuad(x, y+thickness, thickness, height-thickness*2, color)
	// Right
	r.addQuad(x+width-thickness, y+thickness, thickness, height-thickness*2, color)
}

// DrawPanel draws a panel with border.
func (r *Renderer) DrawPanel(x, y, width, height float32, bg, border Color) {
	// Background
	r.DrawRect(x, y, width, height, bg)
	// Border
	r.DrawRectOutline(x, y, width, height, 1, border)
}

// addQuad adds a solid color quad to the vertex buffer.
func (r *Renderer) addQuad(x, y, w, h float32, c Color) {
	// Two triangles forming a quad
	// Vertex format: x, y, z, r, g, b, a (7 floats)

	// Triangle 1
	r.solidVertices = append(r.solidVertices,
		x, y, 0, c.R, c.G, c.B, c.A,
		x+w, y, 0, c.R, c.G, c.B, c.A,
		x+w, y+h, 0, c.R, c.G, c.B, c.A,
	)
	// Triangle 2
	r.solidVertices = append(r.solidVertices,
		x, y, 0, c.R, c.G, c.B, c.A,
		x+w, y+h, 0, c.R, c.G, c.B, c.A,
		x, y+h, 0, c.R, c.G, c.B, c.A,
	)
}

// addTexturedQuad adds a textured quad to the text vertex buffer.
func (r *Renderer) addTexturedQuad(x, y, w, h float32, u0, v0, u1, v1 float32, c Color) {
	// Two triangles forming a quad
	// Vertex format: x, y, z, u, v, r, g, b, a (9 floats)

	// Triangle 1
	r.textVertices = append(r.textVertices,
		x, y, 0, u0, v0, c.R, c.G, c.B, c.A,
		x+w, y, 0, u1, v0, c.R, c.G, c.B, c.A,
		x+w, y+h, 0, u1, v1, c.R, c.G, c.B, c.A,
	)
	// Triangle 2
	r.textVertices = append(r.textVertices,
		x, y, 0, u0, v0, c.R, c.G, c.B, c.A,
		x+w, y+h, 0, u1, v1, c.R, c.G, c.B, c.A,
		x, y+h, 0, u0, v1, c.R, c.G, c.B, c.A,
	)
}

// DrawText draws text at the given position.
func (r *Renderer) DrawText(x, y float32, text string, scale float32, color Color) {
	if r.font == nil {
		return
	}

	gw, gh := r.font.GlyphSize()
	charW := float32(gw) * scale
	charH := float32(gh) * scale

	curX := x
	for _, char := range text {
		if char == '\n' {
			curX = x
			y += charH
			continue
		}

		u0, v0, u1, v1 := r.font.GetGlyphUV(char)
		r.addTexturedQuad(curX, y, charW, charH, u0, v0, u1, v1, color)
		curX += charW
	}
}

// MeasureText returns the width and height of rendered text.
func (r *Renderer) MeasureText(text string, scale float32) (float32, float32) {
	if r.font == nil {
		return 0, 0
	}
	return r.font.MeasureText(text, scale)
}

// DrawSceneTexture draws a scene texture as a fullscreen or positioned quad.
// Call this BEFORE Begin() to draw the 3D scene background, or it will be drawn on top.
func (r *Renderer) DrawSceneTexture(x, y, w, h float32, textureID uint32) {
	if textureID == 0 {
		return
	}

	// Save state
	var prevBlend, prevDepth int32
	gl.GetIntegerv(gl.BLEND, &prevBlend)
	gl.GetIntegerv(gl.DEPTH_TEST, &prevDepth)

	// Setup state for 2D rendering
	gl.Disable(gl.BLEND) // No blending for scene - it's opaque
	gl.Disable(gl.DEPTH_TEST)

	// Use scene shader (full RGBA sampling)
	gl.UseProgram(r.sceneShader)
	proj := r.orthoMatrix(0, float32(r.screenWidth), float32(r.screenHeight), 0, -1, 1)
	projLoc := gl.GetUniformLocation(r.sceneShader, gl.Str("uProjection\x00"))
	gl.UniformMatrix4fv(projLoc, 1, false, &proj[0])

	texLoc := gl.GetUniformLocation(r.sceneShader, gl.Str("uTexture\x00"))
	gl.Uniform1i(texLoc, 0)

	gl.ActiveTexture(gl.TEXTURE0)
	gl.BindTexture(gl.TEXTURE_2D, textureID)

	// Ensure proper texture filtering for quality
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)

	// Create vertices for the quad (simpler format: pos + uv)
	// UV coordinates: flip V for OpenGL texture orientation
	vertices := []float32{
		// Triangle 1: pos(x,y,z) + uv(u,v)
		x, y, 0, 0, 1,
		x + w, y, 0, 1, 1,
		x + w, y + h, 0, 1, 0,
		// Triangle 2
		x, y, 0, 0, 1,
		x + w, y + h, 0, 1, 0,
		x, y + h, 0, 0, 0,
	}

	gl.BindVertexArray(r.sceneVAO)
	gl.BindBuffer(gl.ARRAY_BUFFER, r.sceneVBO)
	gl.BufferData(gl.ARRAY_BUFFER, len(vertices)*4, unsafe.Pointer(&vertices[0]), gl.STREAM_DRAW)
	gl.DrawArrays(gl.TRIANGLES, 0, 6)

	// Restore state
	gl.BindVertexArray(0)
	gl.BindTexture(gl.TEXTURE_2D, 0)
	gl.UseProgram(0)

	if prevBlend == gl.TRUE {
		gl.Enable(gl.BLEND)
	}
	if prevDepth == gl.TRUE {
		gl.Enable(gl.DEPTH_TEST)
	}
}

// orthoMatrix creates an orthographic projection matrix.
func (r *Renderer) orthoMatrix(left, right, bottom, top, near, far float32) [16]float32 {
	return [16]float32{
		2 / (right - left), 0, 0, 0,
		0, 2 / (top - bottom), 0, 0,
		0, 0, -2 / (far - near), 0,
		-(right + left) / (right - left), -(top + bottom) / (top - bottom), -(far + near) / (far - near), 1,
	}
}

// createSolidShader creates the shader for solid color quads.
func (r *Renderer) createSolidShader() (uint32, error) {
	vertexShaderSource := `
		#version 410 core

		layout (location = 0) in vec3 aPos;
		layout (location = 1) in vec4 aColor;

		uniform mat4 uProjection;

		out vec4 vColor;

		void main() {
			gl_Position = uProjection * vec4(aPos, 1.0);
			vColor = aColor;
		}
	` + "\x00"

	fragmentShaderSource := `
		#version 410 core

		in vec4 vColor;
		out vec4 FragColor;

		void main() {
			FragColor = vColor;
		}
	` + "\x00"

	return r.linkShaderProgram(vertexShaderSource, fragmentShaderSource)
}

// createTextShader creates the shader for textured text quads.
func (r *Renderer) createTextShader() (uint32, error) {
	vertexShaderSource := `
		#version 410 core

		layout (location = 0) in vec3 aPos;
		layout (location = 1) in vec2 aTexCoord;
		layout (location = 2) in vec4 aColor;

		uniform mat4 uProjection;

		out vec2 vTexCoord;
		out vec4 vColor;

		void main() {
			gl_Position = uProjection * vec4(aPos, 1.0);
			vTexCoord = aTexCoord;
			vColor = aColor;
		}
	` + "\x00"

	fragmentShaderSource := `
		#version 410 core

		uniform sampler2D uTexture;

		in vec2 vTexCoord;
		in vec4 vColor;
		out vec4 FragColor;

		void main() {
			float alpha = texture(uTexture, vTexCoord).a;
			FragColor = vec4(vColor.rgb, vColor.a * alpha);
		}
	` + "\x00"

	return r.linkShaderProgram(vertexShaderSource, fragmentShaderSource)
}

// linkShaderProgram compiles and links a shader program.
func (r *Renderer) linkShaderProgram(vertexSrc, fragmentSrc string) (uint32, error) {
	vertexShader, err := compileShader(vertexSrc, gl.VERTEX_SHADER)
	if err != nil {
		return 0, fmt.Errorf("vertex shader: %w", err)
	}
	defer gl.DeleteShader(vertexShader)

	fragmentShader, err := compileShader(fragmentSrc, gl.FRAGMENT_SHADER)
	if err != nil {
		return 0, fmt.Errorf("fragment shader: %w", err)
	}
	defer gl.DeleteShader(fragmentShader)

	program := gl.CreateProgram()
	gl.AttachShader(program, vertexShader)
	gl.AttachShader(program, fragmentShader)
	gl.LinkProgram(program)

	var status int32
	gl.GetProgramiv(program, gl.LINK_STATUS, &status)
	if status == gl.FALSE {
		var logLength int32
		gl.GetProgramiv(program, gl.INFO_LOG_LENGTH, &logLength)
		log := strings.Repeat("\x00", int(logLength+1))
		gl.GetProgramInfoLog(program, logLength, nil, gl.Str(log))
		return 0, fmt.Errorf("link failed: %s", log)
	}

	return program, nil
}

// createSolidBuffers creates VAO/VBO for solid color quad rendering.
func (r *Renderer) createSolidBuffers() error {
	gl.GenVertexArrays(1, &r.solidVAO)
	gl.BindVertexArray(r.solidVAO)

	gl.GenBuffers(1, &r.solidVBO)
	gl.BindBuffer(gl.ARRAY_BUFFER, r.solidVBO)

	// Vertex format: pos(3) + color(4) = 7 floats, 28 bytes
	stride := int32(7 * 4)

	// Position attribute (location = 0): 3 floats
	gl.VertexAttribPointerWithOffset(0, 3, gl.FLOAT, false, stride, 0)
	gl.EnableVertexAttribArray(0)

	// Color attribute (location = 1): 4 floats
	gl.VertexAttribPointerWithOffset(1, 4, gl.FLOAT, false, stride, 3*4)
	gl.EnableVertexAttribArray(1)

	gl.BindVertexArray(0)
	gl.BindBuffer(gl.ARRAY_BUFFER, 0)

	return nil
}

// createTextBuffers creates VAO/VBO for textured text quad rendering.
func (r *Renderer) createTextBuffers() error {
	gl.GenVertexArrays(1, &r.textVAO)
	gl.BindVertexArray(r.textVAO)

	gl.GenBuffers(1, &r.textVBO)
	gl.BindBuffer(gl.ARRAY_BUFFER, r.textVBO)

	// Vertex format: pos(3) + texcoord(2) + color(4) = 9 floats, 36 bytes
	stride := int32(9 * 4)

	// Position attribute (location = 0): 3 floats
	gl.VertexAttribPointerWithOffset(0, 3, gl.FLOAT, false, stride, 0)
	gl.EnableVertexAttribArray(0)

	// TexCoord attribute (location = 1): 2 floats
	gl.VertexAttribPointerWithOffset(1, 2, gl.FLOAT, false, stride, 3*4)
	gl.EnableVertexAttribArray(1)

	// Color attribute (location = 2): 4 floats
	gl.VertexAttribPointerWithOffset(2, 4, gl.FLOAT, false, stride, 5*4)
	gl.EnableVertexAttribArray(2)

	gl.BindVertexArray(0)
	gl.BindBuffer(gl.ARRAY_BUFFER, 0)

	return nil
}

// createSceneShader creates the shader for scene texture rendering (full RGBA).
func (r *Renderer) createSceneShader() (uint32, error) {
	vertexShaderSource := `
		#version 410 core

		layout (location = 0) in vec3 aPos;
		layout (location = 1) in vec2 aTexCoord;

		uniform mat4 uProjection;

		out vec2 vTexCoord;

		void main() {
			gl_Position = uProjection * vec4(aPos, 1.0);
			vTexCoord = aTexCoord;
		}
	` + "\x00"

	fragmentShaderSource := `
		#version 410 core

		uniform sampler2D uTexture;

		in vec2 vTexCoord;
		out vec4 FragColor;

		void main() {
			vec4 color = texture(uTexture, vTexCoord);
			// Gamma correction (sRGB output)
			color.rgb = pow(color.rgb, vec3(1.0 / 2.2));
			// Warm tint (Korangar-style)
			color.rgb *= vec3(1.08, 1.02, 0.92);
			// Clamp to valid range
			color.rgb = clamp(color.rgb, 0.0, 1.0);
			FragColor = color;
		}
	` + "\x00"

	return r.linkShaderProgram(vertexShaderSource, fragmentShaderSource)
}

// createSceneBuffers creates VAO/VBO for scene texture rendering.
func (r *Renderer) createSceneBuffers() error {
	gl.GenVertexArrays(1, &r.sceneVAO)
	gl.BindVertexArray(r.sceneVAO)

	gl.GenBuffers(1, &r.sceneVBO)
	gl.BindBuffer(gl.ARRAY_BUFFER, r.sceneVBO)

	// Vertex format: pos(3) + texcoord(2) = 5 floats, 20 bytes
	stride := int32(5 * 4)

	// Position attribute (location = 0): 3 floats
	gl.VertexAttribPointerWithOffset(0, 3, gl.FLOAT, false, stride, 0)
	gl.EnableVertexAttribArray(0)

	// TexCoord attribute (location = 1): 2 floats
	gl.VertexAttribPointerWithOffset(1, 2, gl.FLOAT, false, stride, 3*4)
	gl.EnableVertexAttribArray(1)

	gl.BindVertexArray(0)
	gl.BindBuffer(gl.ARRAY_BUFFER, 0)

	return nil
}

// compileShader compiles a shader from source.
func compileShader(source string, shaderType uint32) (uint32, error) {
	shader := gl.CreateShader(shaderType)

	csources, free := gl.Strs(source)
	gl.ShaderSource(shader, 1, csources, nil)
	free()
	gl.CompileShader(shader)

	var status int32
	gl.GetShaderiv(shader, gl.COMPILE_STATUS, &status)
	if status == gl.FALSE {
		var logLength int32
		gl.GetShaderiv(shader, gl.INFO_LOG_LENGTH, &logLength)
		log := strings.Repeat("\x00", int(logLength+1))
		gl.GetShaderInfoLog(shader, logLength, nil, gl.Str(log))
		return 0, fmt.Errorf("compile failed: %s", log)
	}

	return shader, nil
}
