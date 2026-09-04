package states

import (
	"math"

	"go.uber.org/zap"

	"github.com/Faultbox/midgard-ro/internal/engine/effect"
	"github.com/Faultbox/midgard-ro/internal/logger"
	"github.com/Faultbox/midgard-ro/internal/trace"
	"github.com/Faultbox/midgard-ro/pkg/formats"
)

// World effects — the STR animations the game plays over a character, of which
// the level-up flash is the first.
//
// They are held here rather than in the interface because they belong to
// something in the world and have to be projected with it, the same way damage
// numbers are. The interface is handed quads that have already been placed.

// Where the effect files and the textures they name live in the archive.
const (
	effectPath        = `data\texture\effect\`
	effectTexturePath = `data\texture\effect\`

	// levelUpEffect is the flash a level-up plays. There is no sprite version
	// of it in the archive any more — the only level-up art kRO ships is this
	// STR — so this is the effect or nothing.
	levelUpEffect = "h_levelup.str"

	// worldSoundDir is where the archive keeps the sounds the world plays —
	// the level-up chime, and the sound a sprite names on the frame its blow
	// lands.
	worldSoundDir = `data\wav\`

	// levelUpSound is the sound that goes with it.
	levelUpSound = worldSoundDir + `levelup.wav`
)

// activeEffect is one effect playing over a world position.
type activeEffect struct {
	str *formats.STR

	// Where it plays. Held as a position rather than as the unit it belongs
	// to: an effect that outlived its owner should finish where it started
	// rather than vanish or follow a corpse.
	x, y, z float32

	ageMs float32
}

// EffectQuad is one piece of an effect, projected and ready to draw.
type EffectQuad struct {
	Texture string

	// Corners are viewport pixels, clockwise from the top left, and UV
	// matches them corner for corner.
	Corners [4][2]float32
	UV      [4][2]float32

	Color    [4]float32
	Additive bool
}

// PlayEffectAt starts an effect over a world position.
func (s *InGameState) PlayEffectAt(name string, x, y, z float32) {
	str := s.loadEffect(name)
	if str == nil {
		return
	}

	trace.Emit(trace.HUD, "effect-play", zap.String("effect", name))

	s.effects = append(s.effects, &activeEffect{str: str, x: x, y: y, z: z})
}

// loadEffect parses an effect file, once, and remembers it.
//
// Cached because an effect is played again and again — every level-up is the
// same thirty-eight layers — and parsing is the expensive half.
func (s *InGameState) loadEffect(name string) *formats.STR {
	if str, ok := s.effectCache[name]; ok {
		return str
	}

	if s.effectCache == nil {
		s.effectCache = make(map[string]*formats.STR)
	}

	// A miss is remembered too, as a nil: an archive without the file will not
	// grow one, and retrying every level-up would parse nothing repeatedly.
	var str *formats.STR

	if s.manager == nil || s.manager.TexLoader == nil {
		s.effectCache[name] = nil

		return nil
	}

	data, err := s.manager.TexLoader(effectPath + name)
	if err != nil {
		logger.Warn("effect art unavailable", zap.String("effect", name), zap.Error(err))
		s.effectCache[name] = nil

		return nil
	}

	if str, err = formats.ParseSTR(data); err != nil {
		logger.Warn("effect art unreadable", zap.String("effect", name), zap.Error(err))
		str = nil
	}

	s.effectCache[name] = str

	return str
}

// effectDurationMs is how long an effect runs, for sequencing one behind
// another. Zero when the effect could not be loaded.
func (s *InGameState) effectDurationMs(name string) float32 {
	return s.loadEffect(name).DurationMs()
}

// updateEffects ages what is playing and drops what has finished.
func (s *InGameState) updateEffects(deltaMs float32) {
	if len(s.effects) == 0 {
		return
	}

	kept := s.effects[:0]
	for _, e := range s.effects {
		e.ageMs += deltaMs
		if e.ageMs <= e.str.DurationMs() {
			kept = append(kept, e)
		}
	}

	s.effects = kept
}

// EffectQuads is everything to draw for the effects playing, projected into
// the viewport.
func (s *InGameState) EffectQuads(viewportW, viewportH float32) []EffectQuad {
	if s.scene == nil || !s.SceneReady {
		return nil
	}

	// The bursts drawn in code and the sprites drawn frame by frame go
	// through the same path as the ones read from a file: all three end as
	// quads around a projected point.
	out := s.burstQuads(viewportW, viewportH)
	out = append(out, s.spriteEffectQuads(viewportW, viewportH)...)

	if len(s.effects) == 0 {
		return out
	}

	for _, e := range s.effects {
		originX, originY := s.projectToScreen(e.x, e.y, e.z, viewportW, viewportH)
		if originX < 0 {
			continue
		}

		for _, quad := range effect.Frames(e.str, e.ageMs) {
			placed := EffectQuad{
				// The whole path. A quad may draw a file out of the effect
				// texture directory or a frame of a sprite, and the two are
				// not in the same place.
				Texture:  effectTexturePath + quad.Texture,
				UV:       quad.UV,
				Color:    quad.Color,
				Additive: quad.Additive,
			}

			for i := range placed.Corners {
				placed.Corners[i] = [2]float32{
					originX + quad.Corners[i][0],
					originY + quad.Corners[i][1],
				}
			}

			out = append(out, placed)
		}
	}

	return out
}

// Celebrating a level, which is an effect and a sound together.
//
// Reaching a base and a job level at the same moment is ordinary — a Novice
// does it repeatedly — and the two arrive as two packets in the same instant.
// Played on top of each other they are one louder flash; queued, they read as
// two things happening, which is what they are.

// queueLevelUpCelebration adds one to the queue, to play when the one before
// it has finished.
func (s *InGameState) queueLevelUpCelebration() {
	s.celebrations++
}

// updateCelebrations starts the next celebration once the last has run its
// course.
//
// The gap is the effect's own length, read out of the file rather than chosen
// here: an effect that is sequenced behind another has to wait exactly as long
// as the first one lasts, and the first one knows.
func (s *InGameState) updateCelebrations(deltaMs float32) {
	if s.celebrationWaitMs > 0 {
		s.celebrationWaitMs -= deltaMs
	}

	if s.celebrations <= 0 || s.celebrationWaitMs > 0 || s.player == nil {
		return
	}

	s.celebrations--
	s.celebrationWaitMs = s.effectDurationMs(levelUpEffect)

	s.PlayEffectAt(levelUpEffect, s.player.RenderX, s.player.RenderY, s.player.RenderZ)
	s.playSound(levelUpSound)
}

// TakeSounds returns every sound the world wants played this frame, and clears
// them.
//
// A list rather than one: a skill going off asks for its own sound at the same
// moment as the blow that carried it, and a single slot loses whichever came
// second. The state has no audio device, so playing them is the caller's job —
// the same split the interface has for the packets it cannot send.
func (s *InGameState) TakeSounds() []Sound {
	if len(s.sounds) == 0 {
		return nil
	}

	out := s.sounds
	s.sounds = nil

	return out
}

// Sound is one the world wants played, and how loud.
type Sound struct {
	Path string

	// Gain is a fraction of the effects volume: one for something that
	// happened to the character, and less for something that happened at a
	// distance from them.
	Gain float32
}

// soundRange is how far away a sound is still heard at all, in world units.
// Twelve cells, which is a little past the edge of the screen at the default
// camera — far enough that a monster is heard before it is seen, and near
// enough that the far side of the map is silent.
const soundRange = float32(60)

// soundGainAt is how loud something at a place should be.
//
// Straight-line falloff rather than the square law the physics has: the map is
// a stage rather than a room, and a quarter of the way out an inverse square
// is already nearly inaudible, which makes a busy field sound empty.
func (s *InGameState) soundGainAt(x, z float32) float32 {
	if s.player == nil {
		return 1
	}

	px, _, pz := s.player.RenderPosition()
	dx, dz := x-px, z-pz

	away := float32(math.Sqrt(float64(dx*dx + dz*dz)))
	if away >= soundRange {
		return 0
	}

	return 1 - away/soundRange
}

// playSoundAt asks for one that comes from somewhere on the map.
func (s *InGameState) playSoundAt(path string, x, z float32) {
	if gain := s.soundGainAt(x, z); gain > 0 {
		s.playSoundGain(path, gain)
	}
}

// playSound asks for one, unless it is already asked for this frame — a skill
// that hits three times should not play its sound three times over itself.
func (s *InGameState) playSound(path string) {
	s.playSoundGain(path, 1)
}

// playSoundGain is the same at a chosen loudness. The loudest ask wins when
// the same sound is asked for twice: two monsters of a kind walking together
// are one noise, and it is the near one that decides how loud.
func (s *InGameState) playSoundGain(path string, gain float32) {
	if path == "" || gain <= 0 {
		return
	}

	for i, already := range s.sounds {
		if already.Path == path {
			if gain > already.Gain {
				s.sounds[i].Gain = gain
			}

			return
		}
	}

	s.sounds = append(s.sounds, Sound{Path: path, Gain: gain})
}
