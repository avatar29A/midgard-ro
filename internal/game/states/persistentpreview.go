package states

import (
	"fmt"

	"github.com/Faultbox/midgard-ro/internal/game/skills"

	"github.com/Faultbox/midgard-ro/pkg/formats"
)

// PersistentPreview wraps actual client evaluators. Lifetime and unit placement
// come from the Studio event fixture, just as they come from the server in game.
type PersistentPreview struct {
	skill uint16
	act   *formats.ACT
	spr   *formats.SPR
	str   *formats.STR
}

func LoadPersistentPreview(skill uint16, load func(string) ([]byte, error)) (*PersistentPreview, error) {
	p := &PersistentPreview{skill: skill}
	switch skill {
	case 10, 18:
		name := sightSprite
		if skill == 18 {
			name = effectSpriteDir + "firewall.spr"
		}
		data, err := load(name)
		if err != nil {
			return nil, err
		}
		p.spr, err = formats.ParseSPR(data)
		if err != nil {
			return nil, err
		}
		if skill == 18 {
			data, err = load(effectSpriteDir + "firewall.act")
			if err != nil {
				return nil, err
			}
			p.act, err = formats.ParseACT(data)
			if err != nil {
				return nil, err
			}
			if len(p.act.Actions) == 0 || len(p.act.Actions[0].Frames) == 0 {
				return nil, fmt.Errorf("firewall has no action frames")
			}
		}
		if len(p.spr.Images) == 0 || len(p.spr.Images) > 512 {
			return nil, fmt.Errorf("invalid effect sprite image count")
		}
	case 12:
		data, err := load(effectPath + "safetywall.str")
		if err != nil {
			return nil, err
		}
		p.str, err = formats.ParseSTR(data)
		if err != nil {
			return nil, err
		}
		if p.str.DurationMs() <= 0 {
			return nil, fmt.Errorf("invalid safety wall duration")
		}
	default:
		return nil, fmt.Errorf("unsupported persistent preview %d", skill)
	}
	return p, nil
}
func (p *PersistentPreview) Textures() []string {
	var out []string
	switch p.skill {
	case 10:
		out = append(out, spriteFrameKey(sightSprite, 0))
	case 18:
		for _, frame := range p.act.Actions[0].Frames {
			if len(frame.Layers) > 0 && frame.Layers[0].SpriteID >= 0 {
				out = append(out, spriteFrameKey(effectSpriteDir+"firewall.spr", int(frame.Layers[0].SpriteID)))
			}
		}
	case 12:
		for _, layer := range p.str.Layers {
			for _, name := range layer.Textures {
				if name != "" {
					out = append(out, effectTexturePath+name)
				}
			}
		}
	}
	return out
}
func (p *PersistentPreview) Parameters() map[string]any {
	switch p.skill {
	case 10:
		return map[string]any{"radiusWorld": sightRadius, "heightWorld": sightHeight, "trailParts": sightTrail, "halfSizeWorld": sightHalf, "turnDegreesPerFrame": sightTurnPerFrame, "dropFrames": sightDropFrames, "alpha": sightAlpha, "shrink": sightShrink, "space": "world billboard", "clock": "single caster Sight flag"}
	case 18:
		interval := actIntervalMs
		if len(p.act.Intervals) > 0 && p.act.Intervals[0] > 0 {
			interval = p.act.Intervals[0] * actIntervalMs
		}
		scale := effectScaleOf("firewall")
		return map[string]any{"widthScale": scale[0], "heightScale": scale[1], "groundPivotV": fireWallGroundV, "anchor": "fixed flame base; independent of target size", "intervalMs": interval, "actFrames": len(p.act.Actions[0].Frames), "space": "world billboard"}
	default:
		return map[string]any{"fps": p.str.FPS, "loopMs": p.str.DurationMs(), "space": "screen pixels anchored in world", "loop": true}
	}
}
func (p *PersistentPreview) QuadsAt(ageMs float32, at [3]float32, to func(x, y, z float32) (float32, float32, float32, bool)) []EffectQuad {
	if ageMs < 0 {
		return nil
	}
	if p.skill == 10 {
		b := activeBurst{parts: sightTrailParts(ageMs), x: at[0], y: at[1], z: at[2]}
		return b.quadsAt(to)
	}
	x, y, per, ok := to(at[0], at[1], at[2])
	if p.skill == 12 {
		if x < 0 {
			return nil
		}
		duration := p.str.DurationMs()
		ageMs -= duration * float32(int(ageMs/duration))
		return strEffectQuads(p.str, ageMs, x, y)
	}
	if !ok {
		return nil
	}
	e := spriteEffect{act: p.act}
	frame, ok := e.frameAt(ageMs)
	if !ok || frame.frame >= len(p.spr.Images) {
		return nil
	}
	img := p.spr.Images[frame.frame]
	return []EffectQuad{spriteEffectQuad("firewall", frame, [2]float32{float32(img.Width), float32(img.Height)}, x, y, per)}
}

// PreviewSoundPaths follows the effect table, including the fact that the
// persistent status/ground units themselves have no extra sound binding.
func PreviewSoundPaths(skill uint16) []string {
	effects, ok := skills.EffectsOf(skill)
	if !ok {
		return nil
	}
	var paths []string
	seen := map[string]bool{}
	for _, list := range [][]string{effects.BeginCast, effects.OnCaster, effects.OnTarget, effects.OnGround} {
		for _, name := range list {
			path := effectSoundFor(name)
			if path != "" && !seen[path] {
				seen[path] = true
				paths = append(paths, path)
			}
		}
	}
	return paths
}
