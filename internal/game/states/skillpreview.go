package states

import "github.com/Faultbox/midgard-ro/internal/engine/skillvisual"

// SoulStrikePreview is the first Studio adapter. It deliberately wraps the
// game's existing generator/evaluator; no parallel implementation lives in bb.
// This boundary can move to skillvisual when the other burst families migrate.
type SoulStrikePreview struct{ burst activeBurst }

func NewSoulStrikePreview(hits int, from, at [3]float32, definition skillvisual.SoulDefinition) *SoulStrikePreview {
	spec := soulStrikePartsWithDefinition(hits, definition)
	return &SoulStrikePreview{activeBurst{parts: spec.parts, runMs: spec.runMs,
		x: at[0], y: at[1], z: at[2], otherX: from[0] - at[0], otherY: from[1] - at[1], otherZ: from[2] - at[2]}}
}
func (p *SoulStrikePreview) DurationMS() float32 { return p.burst.runMs }
func (p *SoulStrikePreview) ParticleCount() int  { return len(p.burst.parts) }
func (p *SoulStrikePreview) QuadsAt(tick int, project func(x, y, z float32) (float32, float32, float32, bool)) []EffectQuad {
	b := p.burst // absolute evaluation, independent of prior seeks or camera changes
	b.ageMs = float32(tick) * 1000 / 60
	return b.quadsAt(project)
}
func SoulStrikeImpactTicks(hits int) []int {
	out := make([]int, min(max(hits, 1), soulBoltsMax))
	for i := range out {
		out[i] = soulFlightFrames + i*soulSpawnFrames
	}
	return out
}

// CastAuraShape is shared by the live client and the offline timeline.
type CastAuraShape struct{ Bottom, Top, Height, Alpha float32 }

func CastAuraAt(elapsedMs, totalMs float32) CastAuraShape {
	if totalMs <= 0 || elapsedMs < 0 || elapsedMs >= totalMs {
		return CastAuraShape{}
	}
	done := elapsedMs / totalMs
	alpha := float32(1)
	if done > 1-castAuraFade {
		alpha = (1 - done) / castAuraFade
	}
	return CastAuraShape{castAuraRadius, castAuraRadius + (castAuraFlare-1)*castAuraRadius*done, castAuraHeightMax * done, alpha}
}
func AttackHitDelayMS(frame, frames int, durationMs float32) float32 {
	if frames <= 0 || frame <= 0 || durationMs <= 0 {
		return 0
	}
	return durationMs * float32(frame) / float32(frames)
}
func SoulStrikeSoundPaths() (cast, impact string) {
	return effectSoundFor(castAuraEffect), effectSoundFor("EF_SOULSTRIKE")
}
