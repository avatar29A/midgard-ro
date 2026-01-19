// Package framebuffer provides OpenGL framebuffer utilities for offscreen rendering.
package framebuffer

import (
	"fmt"

	"github.com/go-gl/gl/v4.1-core/gl"
)

// Framebuffer manages an offscreen render target with color and depth attachments.
type Framebuffer struct {
	fbo          uint32
	colorTexture uint32
	depthRBO     uint32
	width        int32
	height       int32
}

// New creates a new framebuffer with the specified dimensions.
func New(width, height int32) (*Framebuffer, error) {
	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = 1
	}

	fb := &Framebuffer{
		width:  width,
		height: height,
	}

	if err := fb.create(); err != nil {
		return nil, fmt.Errorf("creating framebuffer: %w", err)
	}

	return fb, nil
}

func (fb *Framebuffer) create() error {
	// Create framebuffer object
	gl.GenFramebuffers(1, &fb.fbo)
	gl.BindFramebuffer(gl.FRAMEBUFFER, fb.fbo)

	// Create color texture attachment
	gl.GenTextures(1, &fb.colorTexture)
	gl.BindTexture(gl.TEXTURE_2D, fb.colorTexture)
	gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA8, fb.width, fb.height, 0, gl.RGBA, gl.UNSIGNED_BYTE, nil)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)
	gl.FramebufferTexture2D(gl.FRAMEBUFFER, gl.COLOR_ATTACHMENT0, gl.TEXTURE_2D, fb.colorTexture, 0)

	// Create depth renderbuffer attachment
	gl.GenRenderbuffers(1, &fb.depthRBO)
	gl.BindRenderbuffer(gl.RENDERBUFFER, fb.depthRBO)
	gl.RenderbufferStorage(gl.RENDERBUFFER, gl.DEPTH_COMPONENT24, fb.width, fb.height)
	gl.FramebufferRenderbuffer(gl.FRAMEBUFFER, gl.DEPTH_ATTACHMENT, gl.RENDERBUFFER, fb.depthRBO)

	// Check framebuffer completeness
	status := gl.CheckFramebufferStatus(gl.FRAMEBUFFER)
	if status != gl.FRAMEBUFFER_COMPLETE {
		fb.Destroy()
		return fmt.Errorf("framebuffer incomplete: 0x%x", status)
	}

	gl.BindFramebuffer(gl.FRAMEBUFFER, 0)
	return nil
}

// Bind makes this framebuffer the current render target.
func (fb *Framebuffer) Bind() {
	gl.BindFramebuffer(gl.FRAMEBUFFER, fb.fbo)
	gl.Viewport(0, 0, fb.width, fb.height)
}

// Unbind restores the default framebuffer.
func (fb *Framebuffer) Unbind() {
	gl.BindFramebuffer(gl.FRAMEBUFFER, 0)
}

// BindWithViewport binds and sets viewport, saving previous state.
// Returns a restore function to restore the previous framebuffer and viewport.
func (fb *Framebuffer) BindWithViewport() func() {
	var prevFBO int32
	var prevViewport [4]int32
	gl.GetIntegerv(gl.FRAMEBUFFER_BINDING, &prevFBO)
	gl.GetIntegerv(gl.VIEWPORT, &prevViewport[0])

	gl.BindFramebuffer(gl.FRAMEBUFFER, fb.fbo)
	gl.Viewport(0, 0, fb.width, fb.height)

	return func() {
		gl.BindFramebuffer(gl.FRAMEBUFFER, uint32(prevFBO))
		gl.Viewport(prevViewport[0], prevViewport[1], prevViewport[2], prevViewport[3])
	}
}

// Clear clears color and depth buffers with the specified color.
func (fb *Framebuffer) Clear(r, g, b, a float32) {
	gl.ClearColor(r, g, b, a)
	gl.Clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT)
}

// ColorTexture returns the color attachment texture ID.
func (fb *Framebuffer) ColorTexture() uint32 {
	return fb.colorTexture
}

// FBO returns the underlying framebuffer object ID.
func (fb *Framebuffer) FBO() uint32 {
	return fb.fbo
}

// Size returns the framebuffer dimensions.
func (fb *Framebuffer) Size() (width, height int32) {
	return fb.width, fb.height
}

// Resize updates the framebuffer dimensions if they have changed.
func (fb *Framebuffer) Resize(width, height int32) {
	if width == fb.width && height == fb.height {
		return
	}
	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = 1
	}

	fb.width = width
	fb.height = height

	// Resize color texture
	gl.BindTexture(gl.TEXTURE_2D, fb.colorTexture)
	gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA8, fb.width, fb.height, 0, gl.RGBA, gl.UNSIGNED_BYTE, nil)

	// Resize depth renderbuffer
	gl.BindRenderbuffer(gl.RENDERBUFFER, fb.depthRBO)
	gl.RenderbufferStorage(gl.RENDERBUFFER, gl.DEPTH_COMPONENT24, fb.width, fb.height)
}

// ReadPixels reads the framebuffer color attachment into a byte slice.
// Returns RGBA data with the image flipped vertically (OpenGL has origin at bottom-left).
func (fb *Framebuffer) ReadPixels() []byte {
	pixels := make([]byte, fb.width*fb.height*4)

	// Bind our framebuffer to read from it
	var prevFBO int32
	gl.GetIntegerv(gl.FRAMEBUFFER_BINDING, &prevFBO)
	gl.BindFramebuffer(gl.FRAMEBUFFER, fb.fbo)

	gl.ReadPixels(0, 0, fb.width, fb.height, gl.RGBA, gl.UNSIGNED_BYTE, gl.Ptr(pixels))

	// Restore previous framebuffer
	gl.BindFramebuffer(gl.FRAMEBUFFER, uint32(prevFBO))

	return pixels
}

// Destroy releases all OpenGL resources.
func (fb *Framebuffer) Destroy() {
	if fb.fbo != 0 {
		gl.DeleteFramebuffers(1, &fb.fbo)
		fb.fbo = 0
	}
	if fb.colorTexture != 0 {
		gl.DeleteTextures(1, &fb.colorTexture)
		fb.colorTexture = 0
	}
	if fb.depthRBO != 0 {
		gl.DeleteRenderbuffers(1, &fb.depthRBO)
		fb.depthRBO = 0
	}
}
