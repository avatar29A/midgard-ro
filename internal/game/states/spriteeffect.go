package states

import (
	"go.uber.org/zap"

	"github.com/Faultbox/midgard-ro/internal/engine/playerrender"
	"github.com/Faultbox/midgard-ro/internal/logger"
	"github.com/Faultbox/midgard-ro/internal/trace"
	"github.com/Faultbox/midgard-ro/pkg/formats"
)

// Effects the archive draws frame by frame.
//
// A third kind, beside the STR animations and the particle bursts. Fire Wall,
// Safety Wall and Sight are sprites: an SPR of frames and an ACT that says
// which to show when and where to put it, exactly like a monster. Nothing here
// could play one, so three of the Mage's skills drew nothing at all.
//
// The frames come through the same texture cache the ice does — a frame of a
// sprite is nameable the way a file is — and come out as the same quads
// everything else in this file ends as, so no new renderer is involved.

// effectScales is how big an effect is drawn against its own art, where one
// is the size a character's sheet is drawn at.
//
// The frame is the art rather than the size. A Fire Wall's frames are 64
// pixels square where a Mage's body is 61 by 88, so drawn at the same scale
// the wall comes out shorter than the caster — and a wall a monster steps
// over is not a wall. The original draws it taller than a character, which is
// what this is for.
//
// Anything not named here is drawn at its own size.
var effectScales = map[string]float32{
	"firewall": 1.6,
}

// effectScaleOf is that, or one for an effect with nothing said about it.
func effectScaleOf(name string) float32 {
	if scale, set := effectScales[name]; set {
		return scale
	}

	return 1
}

// effectSpriteDir is where the archive keeps them. A handful live beside the
// items instead, which is why the path is per effect rather than fixed.
const effectSpriteDir = `data\sprite\이팩트\`

// spriteEffect is one playing.
type spriteEffect struct {
	// name is the sprite's path without its extension, and act what says how
	// to play it.
	name string
	act  *formats.ACT

	// on is the unit it follows, or zero for one that stands where it was
	// put. A Sight orbits its caster; a Fire Wall stays on its cell.
	on uint32

	x, y, z float32

	ageMs float32

	// runMs is how long it plays for. Zero means until something takes it
	// away — a ground skill's wall stands until its unit is gone.
	runMs float32

	// unit is the ground skill this belongs to, for the ones that are taken
	// away rather than running out.
	unit uint32
}

// spriteEffectFrame is which frame of an effect sprite to draw now, and where
// the art sits relative to the point it is played at.
type spriteEffectFrame struct {
	frame int

	// offX and offY are the frame's own offset, in the sprite's pixels. An
	// effect is drawn around a point rather than standing on one, and the ACT
	// is what says how far off that point each frame goes.
	offX, offY float32
}

// frameAt is the frame an effect shows at an age, and whether it is still
// playing.
//
// The ACT's own interval rather than a rate chosen here: a Fire Wall burns at
// its own pace and a Sight turns at another, and both are written down.
func (e *spriteEffect) frameAt(ageMs float32) (spriteEffectFrame, bool) {
	if e.act == nil || len(e.act.Actions) == 0 {
		return spriteEffectFrame{}, false
	}

	action := e.act.Actions[0]
	if len(action.Frames) == 0 {
		return spriteEffectFrame{}, false
	}

	interval := float32(0)
	if len(e.act.Intervals) > 0 {
		interval = e.act.Intervals[0] * actIntervalMs
	}

	if interval <= 0 {
		interval = actIntervalMs
	}

	at := int(ageMs / interval)

	// One that runs out loops rather than stopping on its last frame: these
	// are effects that stand for as long as something else lasts, and a wall
	// that froze on a frame would read as one that had gone out.
	if e.runMs <= 0 {
		at %= len(action.Frames)
	} else if at >= len(action.Frames) {
		at = len(action.Frames) - 1
	}

	layers := action.Frames[at].Layers
	if len(layers) == 0 || layers[0].SpriteID < 0 {
		return spriteEffectFrame{}, false
	}

	return spriteEffectFrame{
		frame: int(layers[0].SpriteID),
		offX:  float32(layers[0].X),
		offY:  float32(layers[0].Y),
	}, true
}

// actIntervalMs is what one of an ACT's interval ticks is worth. The same
// twenty-five milliseconds a character's animation counts in.
const actIntervalMs = float32(25)

// playSpriteEffect starts one, loading its ACT the first time that effect is
// seen.
func (s *InGameState) playSpriteEffect(name string, on uint32, x, y, z, runMs float32) {
	act := s.effectACT(name)
	if act == nil {
		return
	}

	trace.Emit(trace.HUD, "sprite-effect",
		zap.String("name", name), zap.Uint32("on", on), zap.Float32("runMs", runMs))

	s.spriteEffects = append(s.spriteEffects, &spriteEffect{
		name: name, act: act, on: on,
		x: x, y: y, z: z,
		runMs: runMs,
	})
}

// effectACT reads an effect sprite's ACT, remembering it.
func (s *InGameState) effectACT(name string) *formats.ACT {
	if act, known := s.effectACTs[name]; known {
		return act
	}

	if s.effectACTs == nil {
		s.effectACTs = map[string]*formats.ACT{}
	}

	var act *formats.ACT

	if s.manager != nil && s.manager.TexLoader != nil {
		data, err := s.manager.TexLoader(effectSpriteDir + name + ".act")
		if err != nil {
			logger.Warn("effect sprite has no act", zap.String("name", name), zap.Error(err))
		} else if parsed, err := formats.ParseACT(data); err != nil {
			logger.Warn("effect sprite act would not parse",
				zap.String("name", name), zap.Error(err))
		} else {
			act = parsed
		}
	}

	// Remembered either way, so a missing one is complained about once.
	s.effectACTs[name] = act

	return act
}

// playUnitSprite draws what a ground skill left standing, for as long as the
// server says it stands there.
func (s *InGameState) playUnitSprite(name string, unit uint32, x, y, z float32) {
	s.playSpriteEffect(name, 0, x, y, z, 0)

	if len(s.spriteEffects) > 0 {
		s.spriteEffects[len(s.spriteEffects)-1].unit = unit
	}
}

// hideSkillUnit takes away what a ground skill was drawing, because the
// server has taken the unit away.
func (s *InGameState) hideSkillUnit(unit uint32) {
	kept := s.spriteEffects[:0]
	for _, effect := range s.spriteEffects {
		if effect.unit != unit {
			kept = append(kept, effect)
		}
	}

	s.spriteEffects = kept
}

// advanceSpriteEffects ages them and drops what has finished or lost what it
// was following.
func (s *InGameState) advanceSpriteEffects(deltaMs float32) {
	if len(s.spriteEffects) == 0 {
		return
	}

	kept := s.spriteEffects[:0]
	for _, effect := range s.spriteEffects {
		effect.ageMs += deltaMs

		if effect.runMs > 0 && effect.ageMs >= effect.runMs {
			continue
		}

		if effect.on != 0 && s.bodyOf(effect.on) == nil {
			continue
		}

		kept = append(kept, effect)
	}

	s.spriteEffects = kept
}

// spriteEffectQuads is what they draw this frame.
func (s *InGameState) spriteEffectQuads(viewportW, viewportH float32) []EffectQuad {
	if len(s.spriteEffects) == 0 || s.scene == nil || !s.SceneReady {
		return nil
	}

	out := make([]EffectQuad, 0, len(s.spriteEffects))

	for _, effect := range s.spriteEffects {
		at, ok := effect.frameAt(effect.ageMs)
		if !ok {
			continue
		}

		x, y, z := effect.x, effect.y, effect.z
		if effect.on != 0 {
			if body := s.bodyOf(effect.on); body != nil {
				x, y, z = body.RenderX, body.RenderY, body.RenderZ
			}
		}

		screenX, screenY := s.projectToScreen(x, y, z, viewportW, viewportH)
		if screenX < 0 {
			continue
		}

		size, known := s.effectFrameSize(effect.name, at.frame)
		if !known {
			continue
		}

		// The sprite's pixels are a size in the world, not on the screen, and
		// the world is what decides how big they come out. Drawn a pixel to a
		// pixel a Fire Wall stood a third the height of the mage who cast it,
		// and it stayed that size however near the camera came.
		//
		// The same scale a character's own sheet is drawn at, so an effect
		// and the units standing in it agree, and the same measure of what a
		// world unit is worth here that the particle bursts use, so it
		// shrinks with distance along with everything else.
		perUnit := s.pixelsPerUnit(x, y, z, screenX, screenY, viewportW, viewportH)
		if perUnit <= 0 {
			continue
		}

		scale := playerrender.SpriteScale * effectScaleOf(effect.name) * perUnit

		w, h := size[0]*scale, size[1]*scale

		// The frame's own offset is where its middle goes, which is what the
		// ACT means by it: an effect is drawn around a point rather than
		// standing on one.
		left := screenX + at.offX*scale - w/2
		top := screenY + at.offY*scale - h/2

		out = append(out, EffectQuad{
			Texture: spriteFrameKey(effectSpriteDir+effect.name+".spr", at.frame),
			Corners: [4][2]float32{
				{left, top},
				{left + w, top},
				{left + w, top + h},
				{left, top + h},
			},
			UV:       [4][2]float32{{0, 0}, {1, 0}, {1, 1}, {0, 1}},
			Color:    [4]float32{1, 1, 1, 1},
			Additive: true,
		})
	}

	return out
}

// effectFrameSize is how big one frame of an effect sprite is, in its own
// pixels.
//
// Read from the sprite rather than assumed: the frames of one effect are
// different sizes — a flame that grows, a ball that shrinks — and drawn at a
// common size they pulse.
func (s *InGameState) effectFrameSize(name string, frame int) ([2]float32, bool) {
	spr := s.effectSPR(name)
	if spr == nil || frame < 0 || frame >= len(spr.Images) {
		return [2]float32{}, false
	}

	img := spr.Images[frame]

	return [2]float32{float32(img.Width), float32(img.Height)}, true
}

// effectSPR reads an effect sprite, remembering it.
func (s *InGameState) effectSPR(name string) *formats.SPR {
	if spr, known := s.effectSPRs[name]; known {
		return spr
	}

	if s.effectSPRs == nil {
		s.effectSPRs = map[string]*formats.SPR{}
	}

	var spr *formats.SPR

	if s.manager != nil && s.manager.TexLoader != nil {
		data, err := s.manager.TexLoader(effectSpriteDir + name + ".spr")
		if err != nil {
			logger.Warn("effect sprite is not in the archive",
				zap.String("name", name), zap.Error(err))
		} else if parsed, err := formats.ParseSPR(data); err != nil {
			logger.Warn("effect sprite would not parse",
				zap.String("name", name), zap.Error(err))
		} else {
			spr = parsed
		}
	}

	s.effectSPRs[name] = spr

	return spr
}
