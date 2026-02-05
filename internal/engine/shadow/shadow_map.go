// Package shadow provides real-time shadow mapping for 3D rendering.
package shadow

import (
	"github.com/go-gl/gl/v4.1-core/gl"
)

// Map represents a shadow map framebuffer for directional light shadows.
// Uses a depth-only texture for shadow comparison sampling.
type Map struct {
	FBO          uint32   // Framebuffer object
	DepthTexture uint32   // Depth texture for shadow sampling
	Resolution   int32    // Shadow map resolution (width = height)
	prevViewport [4]int32 // Saved viewport for restore
}

// DefaultResolution is the default shadow map resolution.
const DefaultResolution = 2048

// NewMap creates a new shadow map with the specified resolution.
// Resolution should be a power of 2 (e.g., 1024, 2048, 4096).
func NewMap(resolution int32) *Map {
	if resolution <= 0 {
		resolution = DefaultResolution
	}

	sm := &Map{
		Resolution: resolution,
	}

	// Generate framebuffer
	gl.GenFramebuffers(1, &sm.FBO)
	gl.BindFramebuffer(gl.FRAMEBUFFER, sm.FBO)

	// Generate depth texture
	gl.GenTextures(1, &sm.DepthTexture)
	gl.BindTexture(gl.TEXTURE_2D, sm.DepthTexture)

	// Allocate depth texture storage
	gl.TexImage2D(
		gl.TEXTURE_2D,
		0,
		gl.DEPTH_COMPONENT24,
		resolution,
		resolution,
		0,
		gl.DEPTH_COMPONENT,
		gl.FLOAT,
		nil,
	)

	// Texture parameters for shadow mapping
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)

	// Clamp to border with white (1.0) to avoid shadow outside frustum
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_BORDER)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_BORDER)
	borderColor := []float32{1.0, 1.0, 1.0, 1.0}
	gl.TexParameterfv(gl.TEXTURE_2D, gl.TEXTURE_BORDER_COLOR, &borderColor[0])

	// Enable shadow comparison mode for sampler2DShadow
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_COMPARE_MODE, gl.COMPARE_REF_TO_TEXTURE)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_COMPARE_FUNC, gl.LEQUAL)

	// Attach depth texture to framebuffer
	gl.FramebufferTexture2D(
		gl.FRAMEBUFFER,
		gl.DEPTH_ATTACHMENT,
		gl.TEXTURE_2D,
		sm.DepthTexture,
		0,
	)

	// No color buffer for shadow pass
	gl.DrawBuffer(gl.NONE)
	gl.ReadBuffer(gl.NONE)

	// Check framebuffer completeness
	if status := gl.CheckFramebufferStatus(gl.FRAMEBUFFER); status != gl.FRAMEBUFFER_COMPLETE {
		// Clean up on failure
		gl.DeleteFramebuffers(1, &sm.FBO)
		gl.DeleteTextures(1, &sm.DepthTexture)
		return nil
	}

	gl.BindFramebuffer(gl.FRAMEBUFFER, 0)
	gl.BindTexture(gl.TEXTURE_2D, 0)

	return sm
}

// Bind binds the shadow map framebuffer for rendering the depth pass.
// Sets the viewport to match the shadow map resolution.
func (sm *Map) Bind() {
	// Save current viewport for restore
	gl.GetIntegerv(gl.VIEWPORT, &sm.prevViewport[0])

	gl.BindFramebuffer(gl.FRAMEBUFFER, sm.FBO)
	gl.Viewport(0, 0, sm.Resolution, sm.Resolution)
	gl.Clear(gl.DEPTH_BUFFER_BIT)

	// Enable depth testing
	gl.Enable(gl.DEPTH_TEST)
	gl.DepthFunc(gl.LESS)

	// Enable front-face culling to reduce shadow acne
	gl.Enable(gl.CULL_FACE)
	gl.CullFace(gl.FRONT)
}

// Unbind unbinds the shadow map framebuffer.
// Restores viewport and back-face culling.
func (sm *Map) Unbind() {
	gl.BindFramebuffer(gl.FRAMEBUFFER, 0)

	// Restore previous viewport
	gl.Viewport(sm.prevViewport[0], sm.prevViewport[1], sm.prevViewport[2], sm.prevViewport[3])

	// Restore normal culling
	gl.CullFace(gl.BACK)
}

// BindTexture binds the shadow map depth texture to the specified texture unit.
// Use this when sampling the shadow map in the main render pass.
func (sm *Map) BindTexture(textureUnit uint32) {
	gl.ActiveTexture(textureUnit)
	gl.BindTexture(gl.TEXTURE_2D, sm.DepthTexture)
}

// Destroy releases all GPU resources associated with this shadow map.
func (sm *Map) Destroy() {
	if sm.FBO != 0 {
		gl.DeleteFramebuffers(1, &sm.FBO)
		sm.FBO = 0
	}
	if sm.DepthTexture != 0 {
		gl.DeleteTextures(1, &sm.DepthTexture)
		sm.DepthTexture = 0
	}
}

// IsValid returns true if the shadow map was created successfully.
func (sm *Map) IsValid() bool {
	return sm != nil && sm.FBO != 0 && sm.DepthTexture != 0
}
