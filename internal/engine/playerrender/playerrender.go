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
	"go.uber.org/zap"

	"github.com/Faultbox/midgard-ro/internal/engine/character"
	"github.com/Faultbox/midgard-ro/internal/engine/charsprite"
	"github.com/Faultbox/midgard-ro/internal/engine/scene/shaders"
	"github.com/Faultbox/midgard-ro/internal/engine/shader"
	"github.com/Faultbox/midgard-ro/internal/engine/sprite"
	"github.com/Faultbox/midgard-ro/internal/game/entity"
	"github.com/Faultbox/midgard-ro/internal/trace"
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

// Renderer owns the GL state for drawing the local player as a billboard.
type Renderer struct {
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

	// The local player's baked sheet, nil until LoadCharacter succeeds.
	player *sheet

	// Sheets for the other units on the map, keyed by appearance. Many units
	// look alike — a town full of Novices needs one sheet between them — so
	// they are cached rather than baked per unit.
	units map[charsprite.Spec]*sheet

	scale float32

	// Procedural fallback, used until (or unless) a sheet loads.
	fallbackTex          uint32
	fallbackW, fallbackH int
	fallbackScale        float32
}

// New creates a renderer with the procedural fallback texture ready.
// Must be called on the GL thread (creates shader program + VAO + texture).
func New() (*Renderer, error) {
	r := &Renderer{
		units:         make(map[charsprite.Spec]*sheet),
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

	baked := newSheet(assets)
	if baked == nil {
		return fmt.Errorf("sprite sheet for %q produced no frames", assets.BodyPath)
	}

	r.player.release()
	r.player = baked
	return nil
}

// HasSprites reports whether real character sprites are loaded (as opposed to
// the procedural fallback).
func (r *Renderer) HasSprites() bool {
	return r != nil && r.player != nil
}

// SpritePath returns the archive path the loaded body sprite came from.
func (r *Renderer) SpritePath() string {
	if r == nil || r.player == nil {
		return ""
	}
	return r.player.path
}

// FrameCount returns how many animation frames the given action/direction has,
// or 0 when no sheet is loaded.
func (r *Renderer) FrameCount(action, direction int) int {
	if r == nil {
		return 0
	}
	return r.player.frameCount(action, direction)
}

// Render draws the player billboard at the character's render position.
// camPosX/Z are the camera world XZ — used both to orient the billboard and to
// choose which of the 8 sprite facings to show.
func (r *Renderer) Render(viewProj math.Mat4, char *entity.Character, camPosX, camPosZ float32) {
	r.draw(viewProj, char, camPosX, camPosZ, r.player)
}

// RenderUnit draws another unit on the map, baking and caching the sheet for
// spec the first time that appearance is seen.
//
// Loading happens here rather than when the unit is first reported because
// uploading textures has to be on the GL thread, and this is the only place
// that is guaranteed to be. The cost is a hitch the first time a new
// appearance walks into view; every unit that looks the same after that is
// free.
func (r *Renderer) RenderUnit(viewProj math.Mat4, char *entity.Character, camPosX, camPosZ float32,
	load charsprite.Loader, spec charsprite.Spec) {

	if r == nil || char == nil {
		return
	}

	// No fallback marker here, unlike the player. The marker exists so you can
	// always see yourself; drawing one per unit whose sprite will not resolve
	// litters the map with boxes standing in for things that are not there.
	sh := r.unitSheet(load, spec)
	if sh == nil {
		return
	}
	r.draw(viewProj, char, camPosX, camPosZ, sh)
}

// UnitFrameCount reports the animation length for an appearance, or zero when
// its sheet has not been baked yet.
func (r *Renderer) UnitFrameCount(spec charsprite.Spec, action, direction int) int {
	if r == nil {
		return 0
	}
	return r.units[spec].frameCount(action, direction)
}

// CachedUnitSheets reports how many distinct appearances are held in memory.
func (r *Renderer) CachedUnitSheets() int {
	if r == nil {
		return 0
	}
	return len(r.units)
}

// unitSheet returns the cached sheet for an appearance, baking it on first
// use. A failure is cached as a nil sheet so a sprite that cannot be resolved
// is not retried every frame; the unit falls back to the procedural marker.
func (r *Renderer) unitSheet(load charsprite.Loader, spec charsprite.Spec) *sheet {
	if cached, ok := r.units[spec]; ok {
		return cached
	}

	var baked *sheet
	assets, err := charsprite.Load(load, spec)
	if err == nil {
		baked = newSheet(assets)
	}
	r.units[spec] = baked

	// Reported once per appearance, not per frame, because this is where a
	// unit silently stops being drawn: the packet arrived, the sprite did not
	// resolve, and nothing else says so.
	if baked != nil {
		frames, bytes := baked.cost()
		trace.Emit(trace.Render, "sheet",
			zap.Int("job", spec.Job),
			zap.Uint8("kind", uint8(spec.Kind)),
			zap.String("sprite", baked.path),
			zap.Int("frames", frames),
			zap.Int("kb", bytes/1024),
			zap.Int("droppedFrames", baked.dropped),
			zap.Int("cached", len(r.units)))
	} else {
		trace.Emit(trace.Render, "sheet-failed",
			zap.Int("job", spec.Job),
			zap.Uint8("kind", uint8(spec.Kind)),
			zap.Error(err),
			zap.Int("cached", len(r.units)))
	}
	return baked
}

// draw puts one character on screen using the given sheet, falling back to the
// procedural marker when there is none.
func (r *Renderer) draw(viewProj math.Mat4, char *entity.Character, camPosX, camPosZ float32, sh *sheet) {
	if r == nil || char == nil || r.program == 0 || r.vao == 0 {
		return
	}

	// The quad turns to face the camera so the sprite is never seen edge-on.
	right, up := character.BillboardVectors(camPosX, camPosZ, char.RenderX, char.RenderZ)

	texture, spriteW, spriteH := r.selectFrame(char, camPosX, camPosZ, sh)
	if texture == 0 {
		return
	}

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

// selectFrame picks the texture for the character's current action, camera-
// relative facing and animation frame, along with the quad size to draw it at.
func (r *Renderer) selectFrame(char *entity.Character, camPosX, camPosZ float32, sh *sheet) (texture uint32, w, h float32) {
	if sh == nil {
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

	frames := sh.frames[char.CurrentAction*charsprite.Directions+visualDir]
	if len(frames) == 0 {
		// Fall back to idle for this facing rather than dropping the draw.
		frames = sh.frames[visualDir]
		if len(frames) == 0 {
			return 0, 0, 0
		}
	}

	return frames[char.CurrentFrame%len(frames)],
		float32(sh.width) * r.scale,
		float32(sh.height) * r.scale
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

// sheet is one baked appearance: every frame of every action and facing,
// already on the GPU. Several units can share one.
type sheet struct {
	// frames is keyed by action*Directions+direction, as charsprite bakes it.
	frames map[int][]uint32
	width  int
	height int
	path   string

	// dropped is how many animation frames the bake left out, from
	// charsprite.MaxAnimationFrames.
	dropped int
}

// newSheet uploads every frame of a baked appearance, returning nil when there
// was nothing to upload.
//
// Must be called on the GL thread.
func newSheet(assets *charsprite.Assets) *sheet {
	if assets == nil || len(assets.Sheet.Frames) == 0 {
		return nil
	}

	sh := &sheet{
		frames:  make(map[int][]uint32, len(assets.Sheet.Frames)),
		width:   assets.Sheet.Width,
		height:  assets.Sheet.Height,
		path:    assets.BodyPath,
		dropped: assets.Sheet.Dropped,
	}
	for key, frames := range assets.Sheet.Frames {
		textures := make([]uint32, len(frames))
		for i, f := range frames {
			textures[i] = uploadRGBA(f.Pixels, sh.width, sh.height)
		}
		sh.frames[key] = textures
	}

	if len(sh.frames) == 0 {
		return nil
	}
	return sh
}

// frameCount is nil-safe so callers can ask about an appearance that has not
// been baked, which parks the animation on frame 0 rather than failing.
func (sh *sheet) frameCount(action, direction int) int {
	if sh == nil {
		return 0
	}
	return len(sh.frames[action*charsprite.Directions+direction])
}

// cost reports how many frames the sheet holds and roughly how much GPU memory
// they take. Idle animations vary enormously — most NPCs stand on a single
// frame, a Kafra has ninety-nine — so this is worth being able to see rather
// than assume.
func (sh *sheet) cost() (frames, bytes int) {
	if sh == nil {
		return 0, 0
	}
	for _, textures := range sh.frames {
		frames += len(textures)
	}
	return frames, frames * sh.width * sh.height * 4
}

func (sh *sheet) release() {
	if sh == nil {
		return
	}
	for _, textures := range sh.frames {
		for i := range textures {
			if textures[i] != 0 {
				gl.DeleteTextures(1, &textures[i])
			}
		}
	}
	sh.frames = nil
}

// Destroy releases all GL resources owned by the renderer.
func (r *Renderer) Destroy() {
	if r == nil {
		return
	}
	r.player.release()
	r.player = nil
	for spec, sh := range r.units {
		sh.release()
		delete(r.units, spec)
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
