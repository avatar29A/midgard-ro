// Package renderer provides OpenGL rendering functionality.
package renderer

import (
	"fmt"
	"strings"
	"unsafe"

	"go.uber.org/zap"

	"github.com/Faultbox/midgard-ro/internal/logger"
	"github.com/go-gl/gl/v4.1-core/gl"
)

// Config holds renderer configuration.
type Config struct {
	Width  int
	Height int
	VSync  bool
}

// Renderer handles all OpenGL rendering.
type Renderer struct {
	config Config

	// Shader program for basic rendering
	shaderProgram uint32

	// Triangle VAO/VBO for testing
	triangleVAO uint32
	triangleVBO uint32
}

// New creates a new renderer.
// IMPORTANT: Must be called AFTER OpenGL context is created!
func New(cfg Config) (*Renderer, error) {
	r := &Renderer{
		config: cfg,
	}

	// Initialize OpenGL
	if err := gl.Init(); err != nil {
		return nil, fmt.Errorf("failed to initialize OpenGL: %w", err)
	}

	// Log OpenGL info
	version := gl.GoStr(gl.GetString(gl.VERSION))
	rendererName := gl.GoStr(gl.GetString(gl.RENDERER))
	logger.Info("OpenGL initialized",
		zap.String("version", version),
		zap.String("renderer", rendererName),
	)

	// Setup default OpenGL state
	gl.Enable(gl.DEPTH_TEST)
	gl.DepthFunc(gl.LESS)
	gl.ClearColor(0.1, 0.1, 0.15, 1.0) // Dark blue-gray background

	// Create shader program
	var err error
	r.shaderProgram, err = r.createShaderProgram()
	if err != nil {
		return nil, fmt.Errorf("failed to create shader program: %w", err)
	}

	// Create test triangle
	if err := r.createTriangle(); err != nil {
		return nil, fmt.Errorf("failed to create triangle: %w", err)
	}

	return r, nil
}

// Close cleans up renderer resources.
func (r *Renderer) Close() {
	logger.Info("closing renderer")
	if r.triangleVAO != 0 {
		gl.DeleteVertexArrays(1, &r.triangleVAO)
	}
	if r.triangleVBO != 0 {
		gl.DeleteBuffers(1, &r.triangleVBO)
	}
	if r.shaderProgram != 0 {
		gl.DeleteProgram(r.shaderProgram)
	}
}

// Resize handles window resize.
func (r *Renderer) Resize(width, height int) {
	r.config.Width = width
	r.config.Height = height
	gl.Viewport(0, 0, int32(width), int32(height))
	logger.Debug("renderer resized",
		zap.Int("width", width),
		zap.Int("height", height),
	)
}

// Begin starts a new frame.
func (r *Renderer) Begin() {
	gl.Clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT)
}

// End finishes the current frame.
func (r *Renderer) End() {
	// Nothing to do for now - batched draws would be flushed here
}

// DrawTriangle draws a test triangle.
func (r *Renderer) DrawTriangle() {
	gl.UseProgram(r.shaderProgram)
	gl.BindVertexArray(r.triangleVAO)
	gl.DrawArrays(gl.TRIANGLES, 0, 3)
	gl.BindVertexArray(0)
}

// createShaderProgram creates the basic shader program.
func (r *Renderer) createShaderProgram() (uint32, error) {
	// Vertex shader - transforms vertices
	vertexShaderSource := `
		#version 410 core

		layout (location = 0) in vec3 aPos;
		layout (location = 1) in vec3 aColor;

		out vec3 vertexColor;

		void main() {
			gl_Position = vec4(aPos, 1.0);
			vertexColor = aColor;
		}
	` + "\x00"

	// Fragment shader - colors pixels
	fragmentShaderSource := `
		#version 410 core

		in vec3 vertexColor;
		out vec4 FragColor;

		void main() {
			FragColor = vec4(vertexColor, 1.0);
		}
	` + "\x00"

	// Compile vertex shader
	vertexShader, err := compileShader(vertexShaderSource, gl.VERTEX_SHADER)
	if err != nil {
		return 0, fmt.Errorf("vertex shader: %w", err)
	}
	defer gl.DeleteShader(vertexShader)

	// Compile fragment shader
	fragmentShader, err := compileShader(fragmentShaderSource, gl.FRAGMENT_SHADER)
	if err != nil {
		return 0, fmt.Errorf("fragment shader: %w", err)
	}
	defer gl.DeleteShader(fragmentShader)

	// Link program
	program := gl.CreateProgram()
	gl.AttachShader(program, vertexShader)
	gl.AttachShader(program, fragmentShader)
	gl.LinkProgram(program)

	// Check link status
	var status int32
	gl.GetProgramiv(program, gl.LINK_STATUS, &status)
	if status == gl.FALSE {
		var logLength int32
		gl.GetProgramiv(program, gl.INFO_LOG_LENGTH, &logLength)
		log := strings.Repeat("\x00", int(logLength+1))
		gl.GetProgramInfoLog(program, logLength, nil, gl.Str(log))
		return 0, fmt.Errorf("link failed: %s", log)
	}

	logger.Debug("shader program created", zap.Uint32("program", program))
	return program, nil
}

// compileShader compiles a shader from source.
func compileShader(source string, shaderType uint32) (uint32, error) {
	shader := gl.CreateShader(shaderType)

	csources, free := gl.Strs(source)
	gl.ShaderSource(shader, 1, csources, nil)
	free()
	gl.CompileShader(shader)

	// Check compile status
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

// createTriangle creates the test triangle geometry.
func (r *Renderer) createTriangle() error {
	// Triangle vertices: position (x, y, z) + color (r, g, b)
	vertices := []float32{
		// Position          // Color (RGB)
		0.0, 0.5, 0.0, 1.0, 0.0, 0.0, // Top - Red
		-0.5, -0.5, 0.0, 0.0, 1.0, 0.0, // Bottom Left - Green
		0.5, -0.5, 0.0, 0.0, 0.0, 1.0, // Bottom Right - Blue
	}

	// Create VAO (Vertex Array Object)
	gl.GenVertexArrays(1, &r.triangleVAO)
	gl.BindVertexArray(r.triangleVAO)

	// Create VBO (Vertex Buffer Object)
	gl.GenBuffers(1, &r.triangleVBO)
	gl.BindBuffer(gl.ARRAY_BUFFER, r.triangleVBO)
	gl.BufferData(gl.ARRAY_BUFFER, len(vertices)*4, unsafe.Pointer(&vertices[0]), gl.STATIC_DRAW)

	// Position attribute (location = 0)
	gl.VertexAttribPointer(0, 3, gl.FLOAT, false, 6*4, nil)
	gl.EnableVertexAttribArray(0)

	// Color attribute (location = 1)
	gl.VertexAttribPointer(1, 3, gl.FLOAT, false, 6*4, unsafe.Pointer(uintptr(3*4)))
	gl.EnableVertexAttribArray(1)

	// Unbind
	gl.BindBuffer(gl.ARRAY_BUFFER, 0)
	gl.BindVertexArray(0)

	logger.Debug("triangle created",
		zap.Uint32("vao", r.triangleVAO),
		zap.Uint32("vbo", r.triangleVBO),
	)
	return nil
}
