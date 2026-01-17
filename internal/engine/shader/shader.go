// Package shader provides OpenGL shader compilation utilities.
package shader

import (
	"fmt"

	"github.com/go-gl/gl/v4.1-core/gl"
)

// CompileProgram compiles vertex and fragment shaders and links them into a program.
// Returns the program ID or an error if compilation/linking fails.
func CompileProgram(vertexSrc, fragmentSrc string) (uint32, error) {
	// Compile vertex shader
	vertShader, err := compileShader(vertexSrc, gl.VERTEX_SHADER, "vertex")
	if err != nil {
		return 0, err
	}
	defer gl.DeleteShader(vertShader)

	// Compile fragment shader
	fragShader, err := compileShader(fragmentSrc, gl.FRAGMENT_SHADER, "fragment")
	if err != nil {
		return 0, err
	}
	defer gl.DeleteShader(fragShader)

	// Link program
	program := gl.CreateProgram()
	gl.AttachShader(program, vertShader)
	gl.AttachShader(program, fragShader)
	gl.LinkProgram(program)

	var status int32
	gl.GetProgramiv(program, gl.LINK_STATUS, &status)
	if status == gl.FALSE {
		var logLen int32
		gl.GetProgramiv(program, gl.INFO_LOG_LENGTH, &logLen)
		log := make([]byte, logLen)
		gl.GetProgramInfoLog(program, logLen, nil, &log[0])
		gl.DeleteProgram(program)
		return 0, fmt.Errorf("link: %s", string(log))
	}

	return program, nil
}

// compileShader compiles a single shader of the given type.
func compileShader(source string, shaderType uint32, name string) (uint32, error) {
	shader := gl.CreateShader(shaderType)
	csource, free := gl.Strs(source + "\x00")
	gl.ShaderSource(shader, 1, csource, nil)
	free()
	gl.CompileShader(shader)

	var status int32
	gl.GetShaderiv(shader, gl.COMPILE_STATUS, &status)
	if status == gl.FALSE {
		var logLen int32
		gl.GetShaderiv(shader, gl.INFO_LOG_LENGTH, &logLen)
		log := make([]byte, logLen)
		gl.GetShaderInfoLog(shader, logLen, nil, &log[0])
		gl.DeleteShader(shader)
		return 0, fmt.Errorf("%s shader: %s", name, string(log))
	}

	return shader, nil
}

// GetUniform returns the uniform location for the given name.
// Panics if the uniform is not found (useful for required uniforms).
func GetUniform(program uint32, name string) int32 {
	return gl.GetUniformLocation(program, gl.Str(name+"\x00"))
}

// MustGetUniform returns the uniform location for the given name.
// Returns -1 if the uniform is not found or inactive.
func MustGetUniform(program uint32, name string) int32 {
	loc := gl.GetUniformLocation(program, gl.Str(name+"\x00"))
	if loc < 0 {
		panic(fmt.Sprintf("uniform %q not found in program %d", name, program))
	}
	return loc
}
