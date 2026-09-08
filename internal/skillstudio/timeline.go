package skillstudio

import (
	"fmt"
	"math"
	"sort"

	"github.com/Faultbox/midgard-ro/internal/game/skills"
	"github.com/Faultbox/midgard-ro/internal/game/states"
)

// Sequence is a saved fixture, not a server simulation. Nil preserves v0.3
// volley-only snapshots. A negative cast duration selects generated metadata.
type Sequence struct {
	CastMS     int  `json:"castMs"`
	CancelTick *int `json:"cancelTick"`
	Reaction   bool `json:"reaction"`
}
type Event struct {
	ID    string `json:"id"`
	Kind  string `json:"kind"`
	Tick  int    `json:"tick"`
	Owner string `json:"owner"`
	Sound string `json:"sound,omitempty"`
}
type Timeline struct {
	CastMS        int     `json:"castMs"`
	ReleaseTick   int     `json:"releaseTick"`
	DurationTicks int     `json:"durationTicks"`
	EndTick       int     `json:"endTick"`
	ReactionTicks []int   `json:"reactionTicks"`
	ImpactTicks   []int   `json:"impactTicks"`
	Events        []Event `json:"events"`
	CancelTick    *int    `json:"cancelTick"`
}

func ticks(ms float32) int { return int(math.Ceil(float64(ms)*60/1000 - 0.00001)) }
func castDuration(sequence *Sequence) int {
	if sequence.CastMS >= 0 {
		return sequence.CastMS
	}
	info, _ := skills.InfoOf(13)
	return info.CastMs[0] + info.FixedMs[0]
}
func makeTimeline(sequence *Sequence, hits, volleyTicks int, reactionDelayMs float32) Timeline {
	t := Timeline{DurationTicks: volleyTicks, ReactionTicks: []int{}, ImpactTicks: []int{}, Events: []Event{}}
	castSound, impactSound := states.SoulStrikeSoundPaths()
	add := func(id, kind, owner string, tick int, sound string) {
		t.Events = append(t.Events, Event{id, kind, tick, owner, sound})
	}
	if sequence != nil {
		t.CastMS = castDuration(sequence)
		t.ReleaseTick = ticks(float32(t.CastMS))
		t.CancelTick = sequence.CancelTick
		if t.CastMS > 0 {
			add("cast", "cast.started", "caster", 0, "")
			add("cast-audio", "sound", "caster", 0, castSound)
		}
		if t.CancelTick != nil && *t.CancelTick < t.ReleaseTick {
			add("cancel", "cast.canceled", "caster", *t.CancelTick, "")
			t.EndTick = *t.CancelTick
			t.DurationTicks = *t.CancelTick + 30
			return t
		}
		t.CancelTick = nil
		t.DurationTicks += t.ReleaseTick + 30
		if sequence.Reaction {
			reaction := t.ReleaseTick + ticks(reactionDelayMs)
			t.ReactionTicks = append(t.ReactionTicks, reaction)
			add("reaction", "target.hurt", "target", reaction, "")
		}
	}
	add("release", "cast.released", "caster", t.ReleaseTick, "")
	for i, n := range states.SoulStrikeImpactTicks(hits) {
		n += t.ReleaseTick
		t.ImpactTicks = append(t.ImpactTicks, n)
		add(fmt.Sprintf("impact-%d", i), "visual.impact", "target", n, "")
		if sequence != nil {
			add(fmt.Sprintf("impact-audio-%d", i), "sound", "target", n, impactSound)
		}
	}
	t.EndTick = t.ReleaseTick + volleyTicks
	add("end", "effect.finished", "effect", t.EndTick, "")
	sort.SliceStable(t.Events, func(i, j int) bool { return t.Events[i].Tick < t.Events[j].Tick })
	return t
}
func (t Timeline) phase(tick int) string {
	if t.CancelTick != nil && tick >= *t.CancelTick {
		return "canceled"
	}
	if tick < t.ReleaseTick {
		return "cast"
	}
	if len(t.ImpactTicks) > 0 && tick >= t.ImpactTicks[len(t.ImpactTicks)-1] {
		if tick < t.EndTick {
			return "tail"
		}
		return "finished"
	}
	if len(t.ImpactTicks) == 0 {
		if tick >= t.EndTick {
			return "finished"
		}
		return "active"
	}
	return "volley"
}

func castDurationFor(id uint16, level int, s *Sequence) int {
	if s == nil {
		return 0
	}
	if s.CastMS >= 0 {
		return s.CastMS
	}
	info, _ := skills.InfoOf(id)
	variable, _ := skills.At(info.CastMs, level)
	fixed, _ := skills.At(info.FixedMs, level)
	return variable + fixed
}
func persistentTimeline(req Request) Timeline {
	castMS := castDurationFor(req.skill(), req.Level, req.Sequence)
	t := Timeline{CastMS: castMS, ReleaseTick: ticks(float32(castMS)), Events: []Event{}, ImpactTicks: []int{}, ReactionTicks: []int{}}
	add := func(id, kind, owner string, at int, sound string) {
		t.Events = append(t.Events, Event{id, kind, at, owner, sound})
	}
	if castMS > 0 {
		add("cast", "cast.started", "caster", 0, "")
		paths := states.PreviewSoundPaths(req.skill())
		if len(paths) > 0 {
			add("cast-audio", "sound", "caster", 0, paths[0])
		}
	}
	if req.Sequence != nil && req.Sequence.CancelTick != nil {
		t.CancelTick = req.Sequence.CancelTick
		t.EndTick = *t.CancelTick
		t.DurationTicks = t.EndTick + 30
		add("cancel", "cast.canceled", "caster", t.EndTick, "")
		return t
	}
	t.EndTick = t.ReleaseTick + ticks(float32(req.lifetime()))
	t.DurationTicks = t.EndTick + 30
	add("release", "cast.released", "caster", t.ReleaseTick, "")
	if req.skill() == 10 {
		add("status-on", "status.applied", "caster", t.ReleaseTick, "")
		add("status-off", "status.removed", "caster", t.EndTick, "")
	} else {
		for i := 0; i < req.cells(); i++ {
			add(fmt.Sprintf("unit-%d-on", i), "unit.created", "ground", t.ReleaseTick, "")
			add(fmt.Sprintf("unit-%d-off", i), "unit.removed", "ground", t.EndTick, "")
		}
	}
	add("end", "effect.finished", "effect", t.EndTick, "")
	sort.SliceStable(t.Events, func(i, j int) bool { return t.Events[i].Tick < t.Events[j].Tick })
	return t
}

func mageTimeline(req Request, p *states.MagePreview, reactionDelay float32) Timeline {
	castMS := castDurationFor(req.skill(), req.Level, req.Sequence)
	t := Timeline{CastMS: castMS, ReleaseTick: ticks(float32(castMS)), Events: []Event{}, ImpactTicks: []int{}, ReactionTicks: []int{}}
	if castMS > 0 {
		paths := states.PreviewSoundPaths(req.skill())
		t.Events = append(t.Events, Event{ID: "cast", Kind: "cast.started", Tick: 0, Owner: "caster"})
		if len(paths) > 0 {
			t.Events = append(t.Events, Event{ID: "cast-audio", Kind: "sound", Tick: 0, Owner: "caster", Sound: paths[0]})
		}
	}
	if req.Sequence != nil && req.Sequence.CancelTick != nil {
		t.CancelTick = req.Sequence.CancelTick
		t.EndTick = *t.CancelTick
		t.DurationTicks = t.EndTick + 30
		t.Events = append(t.Events, Event{ID: "cancel", Kind: "cast.canceled", Tick: t.EndTick, Owner: "caster"})
		return t
	}
	t.Events = append(t.Events, Event{ID: "release", Kind: "cast.released", Tick: t.ReleaseTick, Owner: "caster"})
	t.EndTick = t.ReleaseTick + ticks(p.DurationMS)
	for _, cue := range p.Cues {
		tick := t.ReleaseTick + ticks(cue.AtMS)
		t.Events = append(t.Events, Event{ID: cue.ID, Kind: cue.Kind, Tick: tick, Owner: cue.Owner, Sound: cue.Sound})
		if cue.Kind == "visual.impact" {
			t.ImpactTicks = append(t.ImpactTicks, tick)
		}
	}
	if req.Sequence != nil && req.Sequence.Reaction && req.skill() != 16 && req.skill() != 21 {
		at := t.ReleaseTick + ticks(reactionDelay)
		t.ReactionTicks = append(t.ReactionTicks, at)
		t.Events = append(t.Events, Event{ID: "reaction", Kind: "target.hurt", Tick: at, Owner: "target"})
		t.EndTick = max(t.EndTick, at+30)
	}
	if req.TargetStatus != "" && req.TargetStatus != "none" {
		start := t.ReleaseTick + ticks(float32(req.StatusDelayMS))
		end := start + ticks(float32(req.lifetime()))
		t.Events = append(t.Events, Event{ID: "body-on", Kind: "status.applied", Tick: start, Owner: "target"}, Event{ID: "body-off", Kind: "status.removed", Tick: end, Owner: "target"})
		t.EndTick = max(t.EndTick, end)
	}
	t.DurationTicks = t.EndTick + 30
	t.Events = append(t.Events, Event{ID: "end", Kind: "effect.finished", Tick: t.EndTick, Owner: "effect"})
	sort.SliceStable(t.Events, func(i, j int) bool { return t.Events[i].Tick < t.Events[j].Tick })
	return t
}
