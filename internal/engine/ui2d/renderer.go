// Package ui2d provides a simple 2D UI rendering layer using OpenGL.
// This replaces ImGui to avoid viewport/window separation issues.
package ui2d

import (
	"fmt"
	"strings"
	"unsafe"

	"github.com/go-gl/gl/v4.1-core/gl"
)

// imageDrawCall represents a batched image draw call.
type imageDrawCall struct {
	textureID uint32
	vertStart int
	vertCount int
}

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

	// Shader program for image quads (full RGBA with tint)
	imageShader uint32

	// VAO/VBO for solid quad rendering
	solidVAO uint32
	solidVBO uint32

	// VAO/VBO for textured quad rendering (text)
	textVAO uint32
	textVBO uint32

	// VAO/VBO for scene texture rendering
	sceneVAO uint32
	sceneVBO uint32

	// VAO/VBO for image rendering
	imageVAO uint32
	imageVBO uint32

	// Current draw lists
	solidVertices []float32

	// commands is every draw in the order it was asked for.
	//
	// The vertices still live in one buffer per shader — that is what the GPU
	// wants — but which buffer a draw came from no longer decides when it is
	// drawn. Each command names its kind, its texture and its span in that
	// kind's buffer, and the flush walks them in order. Consecutive commands
	// that share a kind and texture are merged as they are added, so a window
	// drawing twenty quads of skin costs one call, as it did before.
	commands []drawCmd

	textVertices  []float32
	imageVertices []float32

	// Batched image draw calls (grouped by texture)
	imageDrawCalls []imageDrawCall

	// The overlay layer: textured quads that must sit above everything,
	// solids and text included. The mouse cursor is the reason it exists —
	// it is an image, and images are the first thing drawn, so anything
	// rectangular painted afterwards was covering it.
	overlayVertices  []float32
	overlayDrawCalls []imageDrawCall

	// Font for text rendering
	font *Font

	// pixelDensity is framebuffer pixels per layout point (2.0 on retina).
	// The glyph atlas is rasterized at this density so text is drawn at
	// native resolution rather than magnified.
	pixelDensity float32
}

// New creates a new 2D UI renderer.
func New(width, height int) (*Renderer, error) {
	r := &Renderer{
		screenWidth:   width,
		screenHeight:  height,
		solidVertices: make([]float32, 0, 4096),
		textVertices:  make([]float32, 0, 4096),
		imageVertices: make([]float32, 0, 4096),
		pixelDensity:  1,
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

	// Create image shader (full RGBA sampling with tint for UI textures)
	r.imageShader, err = r.createImageShader()
	if err != nil {
		return nil, fmt.Errorf("create image shader: %w", err)
	}

	// Create VAO/VBO for image rendering
	if err := r.createImageBuffers(); err != nil {
		return nil, fmt.Errorf("create image buffers: %w", err)
	}

	// Create font
	r.font = NewFont(r.pixelDensity)

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
	r.imageVertices = r.imageVertices[:0]
	r.imageDrawCalls = r.imageDrawCalls[:0]
	r.overlayVertices = r.overlayVertices[:0]
	r.overlayDrawCalls = r.overlayDrawCalls[:0]
	r.commands = r.commands[:0]
}

// withDrawState sets up the 2D drawing state, runs the body, and puts the
// state back as it found it.
func (r *Renderer) withDrawState(body func(proj [16]float32)) {
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

	body(proj)

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

// drawKind is which shader and vertex buffer a command draws from.
type drawKind uint8

const (
	drawImage drawKind = iota
	drawSolid
	drawText
)

// drawCmd is one run of vertices from one buffer.
type drawCmd struct {
	kind    drawKind
	texture uint32
	start   int
	count   int

	// additive draws the run by adding to what is behind it instead of
	// covering it. RO's effects are drawn this way — a flash is light, not
	// paint — and one drawn with ordinary alpha reads as a grey rectangle
	// laid over the scene.
	additive bool
}

// pushCmd records count vertices of a kind, extending the previous command
// when it can rather than adding another.
func (r *Renderer) pushCmd(kind drawKind, texture uint32, start int) {
	r.pushBlendCmd(kind, texture, start, false)
}

// quadVertices is what every command draws: two triangles. Nothing in this
// renderer submits anything else, so the count is not worth passing round.
const quadVertices = 6

// pushBlendCmd is pushCmd for a run that names its own blend mode. Runs merge
// only when that matches too, or an additive quad would be drawn with the
// blend of whatever happened to precede it.
func (r *Renderer) pushBlendCmd(kind drawKind, texture uint32, start int, additive bool) {
	if n := len(r.commands); n > 0 {
		last := &r.commands[n-1]
		if last.kind == kind && last.texture == texture &&
			last.additive == additive && last.start+last.count == start {
			last.count += quadVertices

			return
		}
	}

	r.commands = append(r.commands, drawCmd{
		kind: kind, texture: texture, start: start, count: quadVertices, additive: additive,
	})
}

// End finishes the UI frame, drawing whatever is still queued and the overlay
// above it.
func (r *Renderer) End() {
	r.withDrawState(func(proj [16]float32) {
		r.drawQueuedBatches(proj)
		r.drawOverlay(proj)
	})
}

// drawQueuedBatches draws every command in the order it was asked for.
//
// The vertices go up once per shader, then the commands are walked and each
// run drawn from its own buffer. Which buffer a run came from no longer
// decides when it is drawn, so "drawn later is on top" holds for images,
// solids and text alike.
func (r *Renderer) drawQueuedBatches(proj [16]float32) {
	if len(r.commands) == 0 {
		return
	}

	r.uploadBuffers()

	var (
		bound    drawKind = 255
		boundTex uint32
		additive bool
	)

	// Restored at the end so the rest of the frame is unaffected by an effect
	// having been drawn: the blend state is global, and leaving it additive
	// makes every window drawn afterwards glow.
	defer func() {
		if additive {
			gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)
		}
	}()

	for _, cmd := range r.commands {
		if cmd.kind != bound {
			r.bindKind(cmd.kind, proj)
			bound = cmd.kind
			boundTex = 0
		}

		if cmd.additive != additive {
			if cmd.additive {
				gl.BlendFunc(gl.SRC_ALPHA, gl.ONE)
			} else {
				gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)
			}

			additive = cmd.additive
		}

		// Text draws from one atlas, bound with its shader; images name
		// their own texture and change it only when it actually changes.
		if cmd.kind == drawImage && cmd.texture != boundTex {
			gl.BindTexture(gl.TEXTURE_2D, cmd.texture)
			boundTex = cmd.texture
		}

		gl.DrawArrays(gl.TRIANGLES, int32(cmd.start), int32(cmd.count))
	}
}

// uploadBuffers sends each shader's vertices to the GPU once.
func (r *Renderer) uploadBuffers() {
	if len(r.imageVertices) > 0 {
		gl.BindVertexArray(r.imageVAO)
		gl.BindBuffer(gl.ARRAY_BUFFER, r.imageVBO)
		gl.BufferData(gl.ARRAY_BUFFER, len(r.imageVertices)*4, unsafe.Pointer(&r.imageVertices[0]), gl.STREAM_DRAW)
	}

	if len(r.solidVertices) > 0 {
		gl.BindVertexArray(r.solidVAO)
		gl.BindBuffer(gl.ARRAY_BUFFER, r.solidVBO)
		gl.BufferData(gl.ARRAY_BUFFER, len(r.solidVertices)*4, unsafe.Pointer(&r.solidVertices[0]), gl.STREAM_DRAW)
	}

	if len(r.textVertices) > 0 {
		gl.BindVertexArray(r.textVAO)
		gl.BindBuffer(gl.ARRAY_BUFFER, r.textVBO)
		gl.BufferData(gl.ARRAY_BUFFER, len(r.textVertices)*4, unsafe.Pointer(&r.textVertices[0]), gl.STREAM_DRAW)
	}
}

// bindKind switches to the shader and vertex array one kind of command draws
// from.
func (r *Renderer) bindKind(kind drawKind, proj [16]float32) {
	switch kind {
	case drawImage:
		gl.UseProgram(r.imageShader)
		gl.UniformMatrix4fv(gl.GetUniformLocation(r.imageShader, gl.Str("uProjection\x00")), 1, false, &proj[0])
		gl.Uniform1i(gl.GetUniformLocation(r.imageShader, gl.Str("uTexture\x00")), 0)
		gl.ActiveTexture(gl.TEXTURE0)
		gl.BindVertexArray(r.imageVAO)
		gl.BindBuffer(gl.ARRAY_BUFFER, r.imageVBO)

	case drawSolid:
		gl.UseProgram(r.solidShader)
		gl.UniformMatrix4fv(gl.GetUniformLocation(r.solidShader, gl.Str("uProjection\x00")), 1, false, &proj[0])
		gl.BindVertexArray(r.solidVAO)
		gl.BindBuffer(gl.ARRAY_BUFFER, r.solidVBO)

	case drawText:
		if r.font == nil {
			return
		}

		gl.UseProgram(r.textShader)
		gl.UniformMatrix4fv(gl.GetUniformLocation(r.textShader, gl.Str("uProjection\x00")), 1, false, &proj[0])
		gl.Uniform1i(gl.GetUniformLocation(r.textShader, gl.Str("uTexture\x00")), 0)
		gl.ActiveTexture(gl.TEXTURE0)
		gl.BindTexture(gl.TEXTURE_2D, r.font.TextureID())
		gl.BindVertexArray(r.textVAO)
		gl.BindBuffer(gl.ARRAY_BUFFER, r.textVBO)
	}
}

// drawOverlay draws the topmost layer — the cursor.
//
// It shares the image shader and buffers; re-uploading replaces what the
// image pass put there, which has already been drawn.
func (r *Renderer) drawOverlay(proj [16]float32) {
	if len(r.overlayDrawCalls) > 0 {
		gl.UseProgram(r.imageShader)
		projLoc := gl.GetUniformLocation(r.imageShader, gl.Str("uProjection\x00"))
		gl.UniformMatrix4fv(projLoc, 1, false, &proj[0])

		texLoc := gl.GetUniformLocation(r.imageShader, gl.Str("uTexture\x00"))
		gl.Uniform1i(texLoc, 0)

		gl.ActiveTexture(gl.TEXTURE0)
		gl.BindVertexArray(r.imageVAO)
		gl.BindBuffer(gl.ARRAY_BUFFER, r.imageVBO)
		gl.BufferData(gl.ARRAY_BUFFER, len(r.overlayVertices)*4, unsafe.Pointer(&r.overlayVertices[0]), gl.STREAM_DRAW)

		for _, dc := range r.overlayDrawCalls {
			gl.BindTexture(gl.TEXTURE_2D, dc.textureID)
			gl.DrawArrays(gl.TRIANGLES, int32(dc.vertStart), int32(dc.vertCount))
		}
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
	if r.imageVAO != 0 {
		gl.DeleteVertexArrays(1, &r.imageVAO)
	}
	if r.imageVBO != 0 {
		gl.DeleteBuffers(1, &r.imageVBO)
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
	if r.imageShader != 0 {
		gl.DeleteProgram(r.imageShader)
	}
}

// DrawRect draws a filled rectangle.
func (r *Renderer) DrawRect(x, y, width, height float32, color Color) {
	r.addQuad(x, y, width, height, color)
}

// DrawRectGradient draws a rectangle with a vertical gradient — top vertices
// get topColor, bottom vertices get bottomColor. The solid shader already
// reads per-vertex color, so this is a free upgrade over DrawRect.
func (r *Renderer) DrawRectGradient(x, y, width, height float32, topColor, bottomColor Color) {
	r.addQuadGradient(x, y, width, height, topColor, bottomColor)
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
	r.addQuadTo(&r.solidVertices, x, y, w, h, c)
}

// addQuadTo adds a solid color quad to one of the solid layers.
func (r *Renderer) addQuadTo(into *[]float32, x, y, w, h float32, c Color) {
	// Two triangles forming a quad
	// Vertex format: x, y, z, r, g, b, a (7 floats)
	r.pushCmd(drawSolid, 0, len(*into)/7)

	// Triangle 1
	*into = append(*into,
		x, y, 0, c.R, c.G, c.B, c.A,
		x+w, y, 0, c.R, c.G, c.B, c.A,
		x+w, y+h, 0, c.R, c.G, c.B, c.A,
	)
	// Triangle 2
	*into = append(*into,
		x, y, 0, c.R, c.G, c.B, c.A,
		x+w, y+h, 0, c.R, c.G, c.B, c.A,
		x, y+h, 0, c.R, c.G, c.B, c.A,
	)
}

// addQuadGradient adds a vertically-gradient quad: top vertices get t,
// bottom vertices get b.
func (r *Renderer) addQuadGradient(x, y, w, h float32, t, b Color) {
	// Triangle 1
	r.solidVertices = append(r.solidVertices,
		x, y, 0, t.R, t.G, t.B, t.A,
		x+w, y, 0, t.R, t.G, t.B, t.A,
		x+w, y+h, 0, b.R, b.G, b.B, b.A,
	)
	// Triangle 2
	r.solidVertices = append(r.solidVertices,
		x, y, 0, t.R, t.G, t.B, t.A,
		x+w, y+h, 0, b.R, b.G, b.B, b.A,
		x, y+h, 0, b.R, b.G, b.B, b.A,
	)
}

// addTexturedQuad adds a textured quad to the text vertex buffer.
func (r *Renderer) addTexturedQuad(x, y, w, h float32, u0, v0, u1, v1 float32, c Color) {
	// Two triangles forming a quad
	// Vertex format: x, y, z, u, v, r, g, b, a (9 floats)
	r.pushCmd(drawText, 0, len(r.textVertices)/9)

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

// SetPixelDensity tells the renderer how many framebuffer pixels there are
// per layout point, rebuilding the glyph atlas when it changes. Call it when
// the window moves between displays or the backing scale changes; it is a
// no-op when the density is unchanged.
func (r *Renderer) SetPixelDensity(density float32) {
	if density <= 0 {
		density = 1
	}
	if density == r.pixelDensity && r.font != nil {
		return
	}
	r.pixelDensity = density
	if r.font != nil {
		r.font.Close()
	}
	r.font = NewFont(density)
}

// DrawText draws text starting at the given top-left position. Y is the
// top of the line; the renderer adds the font's ascent internally to land
// glyphs on the baseline. Variable-width glyphs are positioned by their
// per-glyph bearing + advance.
func (r *Renderer) DrawText(x, y float32, text string, scale float32, color Color) {
	if r.font == nil {
		return
	}
	f := r.font
	lineH := f.LineHeight() * scale
	ascent := f.Ascent() * scale

	// Glyph metrics are in atlas pixels. On a 2x display the atlas is
	// rasterized at 2x, so each glyph must be drawn at half its pixel size to
	// land one texel per physical pixel — that is what keeps text sharp
	// instead of magnified.
	px := scale / f.Density()

	cursorX := x
	yLine := y
	for _, char := range text {
		if char == '\n' {
			cursorX = x
			yLine += lineH
			continue
		}
		g := f.Glyph(char)
		if g == nil {
			continue
		}
		if g.Width > 0 && g.Height > 0 {
			gx := cursorX + g.BearingX*px
			gy := yLine + ascent + g.BearingY*px
			r.addTexturedQuad(
				gx, gy,
				float32(g.Width)*px, float32(g.Height)*px,
				g.U0, g.V0, g.U1, g.V1, color,
			)
		}
		cursorX += g.Advance * px
	}
}

// DrawTextVertical draws a line turned a quarter turn, reading bottom to top.
//
// Which is how RO labels a vertical tab: the tab strips in the archive carry
// their words that way, and stacking upright letters instead reads as a column
// of letters rather than as a word.
//
// The origin is the corner the unrotated line would start at, and the line
// turns about it — so the text runs upward from y and rightward from x by its
// own line height. Placing it is therefore the same arithmetic as placing
// ordinary text, with the measured width and height swapped.
func (r *Renderer) DrawTextVertical(x, y float32, text string, scale float32, color Color) {
	if r.font == nil {
		return
	}

	f := r.font
	ascent := f.Ascent() * scale
	px := scale / f.Density()

	cursorX := float32(0)

	for _, char := range text {
		g := f.Glyph(char)
		if g == nil {
			continue
		}

		if g.Width > 0 && g.Height > 0 {
			lx := cursorX + g.BearingX*px
			ly := ascent + g.BearingY*px
			gw := float32(g.Width) * px
			gh := float32(g.Height) * px

			// A quarter turn counter-clockwise on a screen whose y runs down:
			// (a, b) goes to (b, -a). What was the line's rightward run
			// becomes upward, and each letter lies on its side with its top
			// toward the left, which is the way the archive's tabs read.
			corner := func(a, b float32) [2]float32 {
				return [2]float32{x + b, y - a}
			}

			r.addTexturedQuadCorners(
				corner(lx, ly), corner(lx+gw, ly),
				corner(lx+gw, ly+gh), corner(lx, ly+gh),
				g.U0, g.V0, g.U1, g.V1, color,
			)
		}

		cursorX += g.Advance * px
	}
}

// addTexturedQuadCorners is addTexturedQuad for a quad that is not
// axis-aligned, corners clockwise from the one the texture's origin sits on.
func (r *Renderer) addTexturedQuadCorners(p0, p1, p2, p3 [2]float32,
	u0, v0, u1, v1 float32, c Color,
) {
	r.pushCmd(drawText, 0, len(r.textVertices)/9)

	r.textVertices = append(r.textVertices,
		p0[0], p0[1], 0, u0, v0, c.R, c.G, c.B, c.A,
		p1[0], p1[1], 0, u1, v0, c.R, c.G, c.B, c.A,
		p2[0], p2[1], 0, u1, v1, c.R, c.G, c.B, c.A,
	)
	r.textVertices = append(r.textVertices,
		p0[0], p0[1], 0, u0, v0, c.R, c.G, c.B, c.A,
		p2[0], p2[1], 0, u1, v1, c.R, c.G, c.B, c.A,
		p3[0], p3[1], 0, u0, v1, c.R, c.G, c.B, c.A,
	)
}

// MeasureText returns the width and height of rendered text.
func (r *Renderer) MeasureText(text string, scale float32) (float32, float32) {
	if r.font == nil {
		return 0, 0
	}
	return r.font.MeasureText(text, scale)
}

// FontLineHeight returns the line advance of the body font at the given scale:
// ascent, descent and leading together. It is what one line of text occupies,
// so anything stacking lines should space them by this rather than by a
// constant that only suits one font at one pixel density.
func (r *Renderer) FontLineHeight(scale float32) float32 {
	if r.font == nil {
		return 0
	}

	return r.font.LineHeight() * scale
}

// FontAscent returns the cap-height of the body font at the given scale —
// useful for vertically centering text in a box where line-height would
// look top-heavy because of leading + descender padding.
func (r *Renderer) FontAscent(scale float32) float32 {
	if r.font == nil {
		return 0
	}
	return r.font.Ascent() * scale
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
			// RO textures are already in sRGB, pass through directly
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

// CreateTextureNearest uploads RGBA pixels with nearest-neighbor filtering.
//
// RO's interface art is pixel art authored for 1:1 display. We lay the UI out
// in points and stretch it across the full framebuffer, so on a 2x display
// every one of these is magnified — linear filtering turns the window frame's
// one-pixel bevels into grey smears. Nearest keeps the edges hard.
func (r *Renderer) CreateTextureNearest(width, height int, pixels []byte) uint32 {
	return r.createTexture(width, height, pixels, gl.NEAREST)
}

// CreateTexture uploads RGBA pixel data to the GPU and returns a texture ID.
func (r *Renderer) CreateTexture(width, height int, pixels []byte) uint32 {
	return r.createTexture(width, height, pixels, gl.LINEAR)
}

func (r *Renderer) createTexture(width, height int, pixels []byte, filter int32) uint32 {
	var texID uint32
	gl.GenTextures(1, &texID)
	gl.BindTexture(gl.TEXTURE_2D, texID)

	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, filter)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, filter)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)

	gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA, int32(width), int32(height), 0,
		gl.RGBA, gl.UNSIGNED_BYTE, unsafe.Pointer(&pixels[0]))

	gl.BindTexture(gl.TEXTURE_2D, 0)
	return texID
}

// DeleteTexture releases a GPU texture.
func (r *Renderer) DeleteTexture(texID uint32) {
	if texID != 0 {
		gl.DeleteTextures(1, &texID)
	}
}

// DrawImage draws a textured quad with full UV range (0,0)→(1,1).
func (r *Renderer) DrawImage(texID uint32, x, y, w, h float32, tint Color) {
	r.DrawImageUV(texID, x, y, w, h, 0, 0, 1, 1, tint)
}

// DrawImageUV draws a textured quad with custom UV coordinates.
func (r *Renderer) DrawImageUV(texID uint32, x, y, w, h, u0, v0, u1, v1 float32, tint Color) {
	if texID == 0 {
		return
	}

	// Recorded here rather than in appendImageQuad: the overlay shares that
	// helper and is drawn on its own terms, above the command list entirely.
	r.pushCmd(drawImage, texID, len(r.imageVertices)/9)
	appendImageQuad(&r.imageVertices, &r.imageDrawCalls, texID, x, y, w, h, u0, v0, u1, v1, tint)
}

// DrawImageQuad draws a textured quad from its four corners.
//
// Corners rather than a rect and an angle because that is how RO's effect
// files store them: a quad that is rotated, sheared or squashed has its four
// points written out, and rebuilding an angle from them only to turn it back
// into points would lose the shear.
//
// Corners go clockwise from the top left, and uv matches them corner for
// corner.
func (r *Renderer) DrawImageQuad(texID uint32, corners [4][2]float32, uv [4][2]float32,
	tint Color, additive bool,
) {
	if texID == 0 {
		return
	}

	r.pushBlendCmd(drawImage, texID, len(r.imageVertices)/9, additive)
	appendImageQuadCorners(&r.imageVertices, &r.imageDrawCalls, texID, corners, uv, tint)
}

// appendImageQuadCorners adds a free-form quad as two triangles.
func appendImageQuadCorners(vertices *[]float32, calls *[]imageDrawCall, texID uint32,
	corners [4][2]float32, uv [4][2]float32, c Color,
) {
	if texID == 0 {
		return
	}

	vertStart := len(*vertices) / 9

	add := func(i int) {
		*vertices = append(*vertices,
			corners[i][0], corners[i][1], 0, uv[i][0], uv[i][1], c.R, c.G, c.B, c.A)
	}

	for _, i := range [6]int{0, 1, 2, 0, 2, 3} {
		add(i)
	}

	if n := len(*calls); n > 0 {
		if last := &(*calls)[n-1]; last.textureID == texID {
			last.vertCount += 6

			return
		}
	}

	*calls = append(*calls, imageDrawCall{
		textureID: texID,
		vertStart: vertStart,
		vertCount: 6,
	})
}

// DrawImageTop draws a textured quad above everything else, text included.
//
// Reserved for things that are not part of the interface's stacking order at
// all — the mouse cursor. Ordinary images go through DrawImage, or they would
// paint over the windows they belong to.
func (r *Renderer) DrawImageTop(texID uint32, x, y, w, h float32, tint Color) {
	r.DrawImageUVTop(texID, x, y, w, h, 0, 0, 1, 1, tint)
}

// DrawImageUVTop is DrawImageTop with custom UV coordinates.
func (r *Renderer) DrawImageUVTop(texID uint32, x, y, w, h, u0, v0, u1, v1 float32, tint Color) {
	appendImageQuad(&r.overlayVertices, &r.overlayDrawCalls, texID, x, y, w, h, u0, v0, u1, v1, tint)
}

// appendImageQuad adds a textured quad to a layer, extending the layer's last
// draw call when it uses the same texture so a run of quads costs one call.
func appendImageQuad(vertices *[]float32, calls *[]imageDrawCall, texID uint32, x, y, w, h, u0, v0, u1, v1 float32, tint Color) {
	if texID == 0 {
		return
	}

	vertStart := len(*vertices) / 9 // 9 floats per vertex

	if len(*calls) > 0 {
		last := &(*calls)[len(*calls)-1]
		if last.textureID == texID {
			addImageQuad(vertices, x, y, w, h, u0, v0, u1, v1, tint)
			last.vertCount += 6

			return
		}
	}

	addImageQuad(vertices, x, y, w, h, u0, v0, u1, v1, tint)
	*calls = append(*calls, imageDrawCall{
		textureID: texID,
		vertStart: vertStart,
		vertCount: 6,
	})
}

// addImageQuad appends one quad's vertices.
func addImageQuad(vertices *[]float32, x, y, w, h, u0, v0, u1, v1 float32, c Color) {
	// Same vertex format as text: pos(3) + uv(2) + color(4) = 9 floats
	*vertices = append(*vertices,
		x, y, 0, u0, v0, c.R, c.G, c.B, c.A,
		x+w, y, 0, u1, v0, c.R, c.G, c.B, c.A,
		x+w, y+h, 0, u1, v1, c.R, c.G, c.B, c.A,
	)
	*vertices = append(*vertices,
		x, y, 0, u0, v0, c.R, c.G, c.B, c.A,
		x+w, y+h, 0, u1, v1, c.R, c.G, c.B, c.A,
		x, y+h, 0, u0, v1, c.R, c.G, c.B, c.A,
	)
}

// createImageShader creates the shader for image quads (full RGBA with tint).
func (r *Renderer) createImageShader() (uint32, error) {
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
			FragColor = texture(uTexture, vTexCoord) * vColor;
		}
	` + "\x00"

	return r.linkShaderProgram(vertexShaderSource, fragmentShaderSource)
}

// createImageBuffers creates VAO/VBO for image quad rendering.
func (r *Renderer) createImageBuffers() error {
	gl.GenVertexArrays(1, &r.imageVAO)
	gl.BindVertexArray(r.imageVAO)

	gl.GenBuffers(1, &r.imageVBO)
	gl.BindBuffer(gl.ARRAY_BUFFER, r.imageVBO)

	// Same vertex format as text: pos(3) + texcoord(2) + color(4) = 9 floats, 36 bytes
	stride := int32(9 * 4)

	gl.VertexAttribPointerWithOffset(0, 3, gl.FLOAT, false, stride, 0)
	gl.EnableVertexAttribArray(0)

	gl.VertexAttribPointerWithOffset(1, 2, gl.FLOAT, false, stride, 3*4)
	gl.EnableVertexAttribArray(1)

	gl.VertexAttribPointerWithOffset(2, 4, gl.FLOAT, false, stride, 5*4)
	gl.EnableVertexAttribArray(2)

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
