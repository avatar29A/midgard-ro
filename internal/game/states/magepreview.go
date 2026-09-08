package states

import (
	"fmt"
	"sort"

	"github.com/Faultbox/midgard-ro/internal/game/skills"
	"github.com/Faultbox/midgard-ro/pkg/formats"
)

// targetVisualCall is also used by playSkillUseEffects, keeping deferred shots,
// STR/burst combinations and sound timing aligned with the game.
type targetVisualCall struct {
	effect              string
	delayMs             float32
	hits                int
	deferred, burstOnly bool
	sounds              []float32
}

func targetVisualPlan(effects []string, hits int) []targetVisualCall {
	var out []targetVisualCall
	bolts, onImpact := splitBolts(effects)
	if len(bolts) > 0 {
		for _, name := range bolts {
			out = append(out, targetVisualCall{effect: name, hits: hits, burstOnly: true, sounds: blowTimes(name, hits)})
		}
		for i := 0; i < min(max(hits, 1), boltMax); i++ {
			for _, name := range onImpact {
				at := boltImpactMs(i)
				out = append(out, targetVisualCall{effect: name, hits: 1, delayMs: at, deferred: true, sounds: []float32{at + strikeLeadMs([]string{name})}})
			}
		}
	} else if volleyed(effects) {
		for i := 0; i < min(max(hits, 1), boltMax); i++ {
			for _, name := range effects {
				at := strikeMs(i)
				out = append(out, targetVisualCall{effect: name, hits: 1, delayMs: at, deferred: true, sounds: []float32{at + strikeLeadMs([]string{name})}})
			}
		}
	} else {
		for _, name := range effects {
			times := blowTimes(name, hits)
			if len(times) == 0 {
				times = []float32{0}
			}
			out = append(out, targetVisualCall{effect: name, hits: hits, sounds: times})
		}
	}
	return out
}

type MageCue struct {
	ID, Kind, Owner, Sound string
	AtMS                   float32
}
type mageCall struct {
	targetVisualCall
	owner string
}
type MagePreview struct {
	calls           []mageCall
	arts            map[string]*formats.STR
	bursts          map[string]burstSpec
	Cues            []MageCue
	DurationMS      float32
	ProjectileCount int
	Warnings        []string
}

func LoadMagePreview(id uint16, hits int, load func(string) ([]byte, error)) (*MagePreview, error) {
	p := &MagePreview{arts: map[string]*formats.STR{}, bursts: map[string]burstSpec{}, Cues: []MageCue{}, Warnings: []string{}}
	fx, _ := skills.EffectsOf(id)
	for _, name := range fx.OnCaster {
		p.calls = append(p.calls, mageCall{targetVisualCall: targetVisualCall{effect: name, hits: hits, sounds: []float32{0}}, owner: "caster"})
	}
	if id == 21 {
		for _, name := range fx.OnGround {
			p.calls = append(p.calls, mageCall{targetVisualCall: targetVisualCall{effect: name, hits: hits, sounds: []float32{0}}, owner: "ground"})
		}
	} else {
		for _, c := range targetVisualPlan(fx.OnTarget, hits) {
			p.calls = append(p.calls, mageCall{c, "target"})
		}
	}
	if id == 16 {
		p.Warnings = append(p.Warnings, "Stone Curse: the client renders cast and target STR effects, but BodyStone has no petrified-body renderer.")
		p.DurationMS = 500
	}
	loaded := map[string]bool{}
	for i, c := range p.calls {
		if !loaded[c.effect] {
			loaded[c.effect] = true
			burst, hasBurst := burstFor(c.effect, c.hits)
			if hasBurst {
				p.bursts[c.effect] = burst
			}
			if !c.burstOnly {
				data, err := load(effectPath + effectFileFor(c.effect))
				if err == nil {
					art, err := formats.ParseSTR(data)
					if err != nil {
						return nil, err
					}
					p.arts[c.effect] = art
				} else if !hasBurst {
					p.Warnings = append(p.Warnings, fmt.Sprintf("%s: %v; skipped as in the client", c.effect, err))
				}
			}
		}
		run := float32(0)
		if b, ok := p.bursts[c.effect]; ok {
			run = b.runMs
		}
		if a := p.arts[c.effect]; a != nil {
			run = max(run, a.DurationMs())
		}
		p.DurationMS = max(p.DurationMS, c.delayMs+run)
		p.Cues = append(p.Cues, MageCue{ID: fmt.Sprintf("effect-%d", i), Kind: "effect.started", Owner: c.owner, AtMS: c.delayMs})
		for n, at := range c.sounds {
			p.Cues = append(p.Cues, MageCue{ID: fmt.Sprintf("sound-%d-%d", i, n), Kind: "sound", Owner: c.owner, Sound: effectSoundFor(c.effect), AtMS: at})
		}
	}
	// Only known projectile/strike policies yield impact markers. Ground rain's
	// internal STR flashes are not invented server damage events.
	if id == 14 || id == 19 {
		p.ProjectileCount = min(max(hits, 1), boltMax)
		for i := 0; i < p.ProjectileCount; i++ {
			p.Cues = append(p.Cues, MageCue{ID: fmt.Sprintf("impact-%d", i), Kind: "visual.impact", Owner: "target", AtMS: boltImpactMs(i)})
		}
	}
	if id == 20 {
		for i := 0; i < min(max(hits, 1), boltMax); i++ {
			p.Cues = append(p.Cues, MageCue{ID: fmt.Sprintf("impact-%d", i), Kind: "visual.impact", Owner: "target", AtMS: strikeMs(i) + strikeLeadMs(fx.OnTarget)})
		}
	}
	if id == 21 {
		p.Warnings = append(p.Warnings, "Thunder Storm shows the client ground STR. Separate server damage packets and damage numbers are not simulated.")
	}
	sort.SliceStable(p.Cues, func(i, j int) bool { return p.Cues[i].AtMS < p.Cues[j].AtMS })
	return p, nil
}
func (p *MagePreview) Textures() []string {
	var out []string
	for _, b := range p.bursts {
		for _, part := range b.parts {
			out = append(out, part.texture)
		}
	}
	for _, a := range p.arts {
		for _, l := range a.Layers {
			for _, t := range l.Textures {
				if t != "" {
					out = append(out, effectTexturePath+t)
				}
			}
		}
	}
	return out
}
func (p *MagePreview) QuadsAt(ageMs float32, from, target, ground [3]float32, project func(x, y, z float32) (float32, float32, float32, bool)) []EffectQuad {
	var out []EffectQuad
	for _, c := range p.calls {
		age := ageMs - c.delayMs
		if age < 0 {
			continue
		}
		at := target
		if c.owner == "caster" {
			at = from
		}
		if c.owner == "ground" {
			at = ground
		}
		if a := p.arts[c.effect]; a != nil && !c.burstOnly && age <= a.DurationMs() {
			x, y, _, _ := project(at[0], at[1], at[2])
			if x >= 0 {
				out = append(out, strEffectQuads(a, age, x, y)...)
			}
		}
		if b, ok := p.bursts[c.effect]; ok && age < b.runMs {
			start := from
			anchor := at
			if b.onGround {
				start[1] = 0
				anchor[1] = 0
			}
			other := [3]float32{b.otherX, b.otherY, b.otherZ}
			if b.fromCaster {
				other = [3]float32{start[0] - anchor[0], start[1] - anchor[1], start[2] - anchor[2]}
			}
			active := activeBurst{parts: b.parts, x: anchor[0], y: anchor[1], z: anchor[2], otherX: other[0], otherY: other[1], otherZ: other[2], ageMs: age, runMs: b.runMs}
			out = append(out, active.quadsAt(project)...)
		}
	}
	return out
}
func MageReactionDelay(skill uint16, swingDelay float32) float32 {
	fx, _ := skills.EffectsOf(skill)
	return max(swingDelay, strikeLeadMs(fx.OnTarget))
}
func FrozenPreviewTexture() string { return spriteFrameKey(frozenSprite, frozenBlock) }
func frozenPreviewBurst(at [3]float32, height float32) *activeBurst {
	height = max(height*frozenBlockOver, frozenBlockLeast)
	block := frozenPart(frozenBlock, height)
	block.y = height / 2
	block.lifeMs = 1
	return &activeBurst{parts: []burstParticle{block}, x: at[0], y: at[1], z: at[2]}
}
func FrozenPreviewQuads(at [3]float32, height float32, project func(x, y, z float32) (float32, float32, float32, bool)) []EffectQuad {
	return frozenPreviewBurst(at, height).quadsAt(project)
}
