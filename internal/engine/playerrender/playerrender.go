// Package playerrender renders the local player character as a billboard
// inside the 3D scene framebuffer.
//
// The sprite pipeline is shared with cmd/grfbrowser: internal/engine/charsprite
// resolves the archive paths and bakes head+body composites, and this package
// owns nothing but the GL state needed to put them on screen. When no sprite
// can be loaded we fall back to a procedural marker so the player is still
// visible on the map.
//
// Frame selection follows RO's two-part rule (docs/plans/sprite-direction-system.md):
// the quad always turns to face the camera so the sprite is never edge-on, and
// which of the 8 facings we draw is picked from the camera angle combined with
// the character's own facing. That combination is what sells the 3D illusion —
// orbiting a standing character walks through all 8 sides of them.
package playerrender

import (
	"fmt"
	"unsafe"

	"github.com/go-gl/gl/v4.1-core/gl"

	"github.com/Faultbox/midgard-ro/internal/engine/character"
	"github.com/Faultbox/midgard-ro/internal/engine/charsprite"
	"github.com/Faultbox/midgard-ro/internal/engine/scene/shaders"
	"github.com/Faultbox/midgard-ro/internal/engine/shader"
	"github.com/Faultbox/midgard-ro/internal/engine/sprite"
	"github.com/Faultbox/midgard-ro/internal/game/entity"
	"github.com/Faultbox/midgard-ro/pkg/math"
)

// SpriteScale converts sprite pixels to world units.
//
// RO sprite art is authored to be drawn at its native pixel size, so the right
// scale is whatever makes one sprite pixel cover one screen pixel. That falls
// out of the camera: with a 45-degree vertical field of view, the visible
// height at the subject's depth is 2*d*tan(22.5) = 0.828*d world units, and at
// the default distance of 145 that is 120 units spread over a 720-point
// viewport — very close to six points per world unit.
//
// One sprite pixel per screen pixel is therefore 1/6 of a world unit. That
// also lands where RO itself does: a cell comes out around 30 points wide
// against the original client's 32, and a character stands about three cells
// tall, which is the proportion RO has always had.
//
// It was 0.25, carried over from grfbrowser, which drew the character half
// again as large as the art intends.
const SpriteScale = 1.0 / 6.0

// The character stands on a soft shadow, as it does on the select screen and
// in the original client.
const (
	// shadowWidthRatio sizes the shadow against the sprite it belongs to, so
	// it stays right whatever the character's dimensions are. It is half the
	// quad's width: a character's sheet is 50x94px, which at SpriteScale is a
	// quad 8.3 units wide, so this puts the shadow at 5 units — one tile,
	// about the width of the character's stance.
	shadowWidthRatio = 0.30

	// shadowOpacity is darker than the shared default, which was chosen for
	// the map viewer's lighting. On a bright stone street the softer value
	// disappeared.
	shadowOpacity = 0.4

	// shadowLift keeps the shadow off the ground plane it is drawn on;
	// coplanar quads z-fight.
	shadowLift = 0.1
)

// Renderer owns the GL state for drawing the local player as a billboard.
type Renderer struct {
	// The shadow the character stands on: its own texture and flat quad.
	shadowTex uint32
	shadowVAO uint32
	shadowVBO uint32

	// Shader program + uniform locations (mirror scene.SpriteRenderer's setup,
	// kept independent so we can render with our own VAO/draw pattern).
	program       uint32
	locViewProj   int32
	locWorldPos   int32
	locSpriteSize int32
	locCamRight   int32
	locCamUp      int32
	locTexture    int32
	locTint       int32

	// Billboard quad — 4 verts, drawn as TRIANGLE_STRIP.
	vao uint32
	vbo uint32

	// Baked character sheet: action*8+direction -> one texture per frame.
	// Empty until LoadCharacter succeeds.
	frames  map[int][]uint32
	sheetW  int
	sheetH  int
	loaded  bool
	scale   float32
	sprPath string

	// Procedural fallback, used until (or unless) a sheet loads.
	fallbackTex          uint32
	fallbackW, fallbackH int
	fallbackScale        float32
}

// New creates a renderer with the procedural fallback texture ready.
// Must be called on the GL thread (creates shader program + VAO + texture).
func New() (*Renderer, error) {
	r := &Renderer{
		frames:        make(map[int][]uint32),
		scale:         SpriteScale,
		fallbackScale: sprite.DefaultProceduralScale,
	}

	// Compile sprite shader (same source scene.SpriteRenderer uses).
	prog, err := shader.CompileProgram(shaders.SpriteVertexShader, shaders.SpriteFragmentShader)
	if err != nil {
		return nil, fmt.Errorf("sprite shader: %w", err)
	}
	r.program = prog
	r.locViewProj = shader.GetUniform(prog, "uViewProj")
	r.locWorldPos = shader.GetUniform(prog, "uWorldPos")
	r.locSpriteSize = shader.GetUniform(prog, "uSpriteSize")
	r.locCamRight = shader.GetUniform(prog, "uCamRight")
	r.locCamUp = shader.GetUniform(prog, "uCamUp")
	r.locTexture = shader.GetUniform(prog, "uTexture")
	r.locTint = shader.GetUniform(prog, "uTint")

	// VAO/VBO: foot-anchored quad (Y=0 at feet, Y=1 at head), TRIANGLE_STRIP.
	if err := r.createShadow(); err != nil {
		return nil, err
	}

	gl.GenVertexArrays(1, &r.vao)
	gl.GenBuffers(1, &r.vbo)
	gl.BindVertexArray(r.vao)
	gl.BindBuffer(gl.ARRAY_BUFFER, r.vbo)

	verts := sprite.GenerateBillboardQuadVertices() // [TL, TR, BL, BR] × (pos, uv)
	gl.BufferData(gl.ARRAY_BUFFER, len(verts)*4, unsafe.Pointer(&verts[0]), gl.STATIC_DRAW)

	// position (location 0): vec2
	gl.VertexAttribPointerWithOffset(0, 2, gl.FLOAT, false, 4*4, 0)
	gl.EnableVertexAttribArray(0)
	// texcoord (location 1): vec2
	gl.VertexAttribPointerWithOffset(1, 2, gl.FLOAT, false, 4*4, 2*4)
	gl.EnableVertexAttribArray(1)

	gl.BindVertexArray(0)

	r.fallbackW = sprite.DefaultProceduralWidth
	r.fallbackH = sprite.DefaultProceduralHeight
	r.fallbackTex = uploadRGBA(
		sprite.GenerateProceduralPlayer(r.fallbackW, r.fallbackH), r.fallbackW, r.fallbackH)

	return r, nil
}

// LoadCharacter resolves and bakes the sprites for spec and uploads every
// frame to the GPU. On failure the renderer keeps drawing the procedural
// marker and the error is returned for the caller to log.
//
// Must be called on the GL thread.
func (r *Renderer) LoadCharacter(load charsprite.Loader, spec charsprite.Spec) error {
	assets, err := charsprite.Load(load, spec)
	if err != nil {
		return err
	}

	r.releaseFrames()
	r.frames = make(map[int][]uint32, len(assets.Sheet.Frames))
	r.sheetW = assets.Sheet.Width
	r.sheetH = assets.Sheet.Height
	r.sprPath = assets.BodyPath

	for key, frames := range assets.Sheet.Frames {
		textures := make([]uint32, len(frames))
		for i, f := range frames {
			textures[i] = uploadRGBA(f.Pixels, r.sheetW, r.sheetH)
		}
		r.frames[key] = textures
	}
	r.loaded = len(r.frames) > 0

	if !r.loaded {
		return fmt.Errorf("sprite sheet for %q produced no frames", assets.BodyPath)
	}
	return nil
}

// HasSprites reports whether real character sprites are loaded (as opposed to
// the procedural fallback).
func (r *Renderer) HasSprites() bool {
	return r != nil && r.loaded
}

// SpritePath returns the archive path the loaded body sprite came from.
func (r *Renderer) SpritePath() string {
	if r == nil {
		return ""
	}
	return r.sprPath
}

// FrameCount returns how many animation frames the given action/direction has,
// or 0 when no sheet is loaded.
func (r *Renderer) FrameCount(action, direction int) int {
	if r == nil || !r.loaded {
		return 0
	}
	return len(r.frames[action*charsprite.Directions+direction])
}

// Render draws the player billboard at the character's render position.
// camPosX/Z are the camera world XZ — used both to orient the billboard and to
// choose which of the 8 sprite facings to show.
func (r *Renderer) Render(viewProj math.Mat4, char *entity.Character, camPosX, camPosZ float32) {
	if r == nil || char == nil || r.program == 0 || r.vao == 0 {
		return
	}

	// The quad turns to face the camera so the sprite is never seen edge-on.
	right, up := character.BillboardVectors(camPosX, camPosZ, char.RenderX, char.RenderZ)

	texture, spriteW, spriteH := r.selectFrame(char, camPosX, camPosZ)
	if texture == 0 {
		return
	}

	r.renderShadow(viewProj, char, spriteW)

	gl.Enable(gl.BLEND)
	gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)
	gl.UseProgram(r.program)

	gl.UniformMatrix4fv(r.locViewProj, 1, false, &viewProj[0])
	gl.Uniform3f(r.locWorldPos, char.RenderX, char.RenderY, char.RenderZ)
	gl.Uniform2f(r.locSpriteSize, spriteW, spriteH)
	gl.Uniform4f(r.locTint, 1.0, 1.0, 1.0, 1.0)
	gl.Uniform3f(r.locCamRight, right[0], right[1], right[2])
	gl.Uniform3f(r.locCamUp, up[0], up[1], up[2])

	gl.ActiveTexture(gl.TEXTURE0)
	gl.BindTexture(gl.TEXTURE_2D, texture)
	gl.Uniform1i(r.locTexture, 0)

	gl.BindVertexArray(r.vao)
	gl.DrawArrays(gl.TRIANGLE_STRIP, 0, 4)
	gl.BindVertexArray(0)

	gl.Disable(gl.BLEND)
}

// createShadow builds the soft circle the character stands on and the flat
// quad it is drawn with.
func (r *Renderer) createShadow() error {
	pixels := sprite.GenerateCircularShadow(sprite.DefaultShadowSize, shadowOpacity)

	gl.GenTextures(1, &r.shadowTex)
	gl.BindTexture(gl.TEXTURE_2D, r.shadowTex)
	gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA8,
		int32(sprite.DefaultShadowSize), int32(sprite.DefaultShadowSize), 0,
		gl.RGBA, gl.UNSIGNED_BYTE, gl.Ptr(pixels))

	// The shadow is a soft gradient rather than pixel art, so it takes linear
	// filtering where the character sprite takes nearest.
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
	gl.BindTexture(gl.TEXTURE_2D, 0)

	gl.GenVertexArrays(1, &r.shadowVAO)
	gl.GenBuffers(1, &r.shadowVBO)
	gl.BindVertexArray(r.shadowVAO)
	gl.BindBuffer(gl.ARRAY_BUFFER, r.shadowVBO)

	// A unit quad: the size comes from uSpriteSize at draw time, so one buffer
	// serves characters of any size.
	verts := sprite.GenerateShadowQuadVertices(1)
	gl.BufferData(gl.ARRAY_BUFFER, len(verts)*4, unsafe.Pointer(&verts[0]), gl.STATIC_DRAW)
	gl.VertexAttribPointerWithOffset(0, 2, gl.FLOAT, false, 4*4, 0)
	gl.EnableVertexAttribArray(0)
	gl.VertexAttribPointerWithOffset(1, 2, gl.FLOAT, false, 4*4, 2*4)
	gl.EnableVertexAttribArray(1)
	gl.BindVertexArray(0)

	return nil
}

// renderShadow lays the shadow flat on the ground under the character.
//
// It reuses the sprite shader: pointing its billboard vectors along X and Z
// instead of at the camera turns the same quad from upright to flat, so the
// shadow needs no shader of its own.
func (r *Renderer) renderShadow(viewProj math.Mat4, char *entity.Character, spriteW float32) {
	if r.shadowVAO == 0 || r.shadowTex == 0 {
		return
	}

	size := spriteW * shadowWidthRatio

	gl.Enable(gl.BLEND)
	gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)

	// The shadow darkens the ground rather than hiding what is behind it, so
	// it blends without writing depth.
	gl.DepthMask(false)

	gl.UseProgram(r.program)
	gl.UniformMatrix4fv(r.locViewProj, 1, false, &viewProj[0])
	gl.Uniform3f(r.locWorldPos, char.RenderX, char.RenderY+shadowLift, char.RenderZ)
	gl.Uniform2f(r.locSpriteSize, size, size)
	gl.Uniform4f(r.locTint, 1, 1, 1, 1)
	gl.Uniform3f(r.locCamRight, 1, 0, 0)
	gl.Uniform3f(r.locCamUp, 0, 0, 1)

	gl.ActiveTexture(gl.TEXTURE0)
	gl.BindTexture(gl.TEXTURE_2D, r.shadowTex)
	gl.Uniform1i(r.locTexture, 0)

	gl.BindVertexArray(r.shadowVAO)
	gl.DrawArrays(gl.TRIANGLE_STRIP, 0, 4)
	gl.BindVertexArray(0)

	gl.DepthMask(true)
}

// selectFrame picks the texture for the character's current action, camera-
// relative facing and animation frame, along with the quad size to draw it at.
func (r *Renderer) selectFrame(char *entity.Character, camPosX, camPosZ float32) (texture uint32, w, h float32) {
	if !r.loaded {
		return r.fallbackTex,
			float32(r.fallbackW) * r.fallbackScale,
			float32(r.fallbackH) * r.fallbackScale
	}

	// Which side of the character the camera is looking at, combined with
	// where they're facing. Hysteresis keeps the sprite from flickering when
	// the angle sits on a sector boundary.
	camAngle := character.CameraAngleToPlayer(camPosX, camPosZ, char.RenderX, char.RenderZ)
	visualDir, camSector := character.CalculateVisualDirection(camAngle, char.Direction, char.LastCameraSector)
	char.LastCameraSector = camSector

	frames := r.frames[char.CurrentAction*charsprite.Directions+visualDir]
	if len(frames) == 0 {
		// Fall back to idle for this facing rather than dropping the draw.
		frames = r.frames[visualDir]
		if len(frames) == 0 {
			return 0, 0, 0
		}
	}

	return frames[char.CurrentFrame%len(frames)],
		float32(r.sheetW) * r.scale,
		float32(r.sheetH) * r.scale
}

// uploadRGBA creates a nearest-filtered texture from RGBA pixels. RO art is
// pixel art — linear filtering just blurs it.
func uploadRGBA(pixels []byte, w, h int) uint32 {
	var tex uint32
	gl.GenTextures(1, &tex)
	gl.BindTexture(gl.TEXTURE_2D, tex)
	gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA8, int32(w), int32(h), 0,
		gl.RGBA, gl.UNSIGNED_BYTE, gl.Ptr(pixels))
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.NEAREST)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.NEAREST)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
	gl.BindTexture(gl.TEXTURE_2D, 0)
	return tex
}

func (r *Renderer) releaseFrames() {
	for _, textures := range r.frames {
		for i := range textures {
			if textures[i] != 0 {
				gl.DeleteTextures(1, &textures[i])
			}
		}
	}
	r.frames = make(map[int][]uint32)
	r.loaded = false
}

// Destroy releases all GL resources owned by the renderer.
func (r *Renderer) Destroy() {
	if r == nil {
		return
	}
	r.releaseFrames()

	if r.shadowTex != 0 {
		gl.DeleteTextures(1, &r.shadowTex)
		r.shadowTex = 0
	}

	if r.shadowVBO != 0 {
		gl.DeleteBuffers(1, &r.shadowVBO)
		r.shadowVBO = 0
	}

	if r.shadowVAO != 0 {
		gl.DeleteVertexArrays(1, &r.shadowVAO)
		r.shadowVAO = 0
	}

	if r.fallbackTex != 0 {
		gl.DeleteTextures(1, &r.fallbackTex)
		r.fallbackTex = 0
	}
	if r.vbo != 0 {
		gl.DeleteBuffers(1, &r.vbo)
		r.vbo = 0
	}
	if r.vao != 0 {
		gl.DeleteVertexArrays(1, &r.vao)
		r.vao = 0
	}
	if r.program != 0 {
		gl.DeleteProgram(r.program)
		r.program = 0
	}
}
