// Package skillcatalog exposes the client's own skill tables without a server,
// graphics context or GRF archive. A binding does not prove working visuals.
package skillcatalog

import (
	"fmt"

	"github.com/Faultbox/midgard-ro/internal/engine/skillvisual"
	"github.com/Faultbox/midgard-ro/internal/game/jobs"
	"github.com/Faultbox/midgard-ro/internal/game/skills"
)

type Job struct {
	ID     int      `json:"id"`
	Name   string   `json:"name"`
	Skills []uint16 `json:"skills"`
}
type Binding struct {
	Phase   string   `json:"phase"`
	Effects []string `json:"effects"`
}
type Skill struct {
	ID       uint16    `json:"id"`
	Name     string    `json:"name"`
	Icon     string    `json:"icon"`
	Jobs     []int     `json:"jobs"`
	HasInfo  bool      `json:"hasInfo"`
	Kind     string    `json:"kind"`
	Target   string    `json:"target"`
	Elements []string  `json:"elements"`
	Hits     []int     `json:"hits"`
	CastMS   []int     `json:"castMs"`
	FixedMS  []int     `json:"fixedMs"`
	Bindings []Binding `json:"bindings"`
	Studio   string    `json:"studio"`
	Sources  []string  `json:"sources"`
}
type Catalog struct {
	Jobs   []Job   `json:"jobs"`
	Skills []Skill `json:"skills"`
	Source string  `json:"source"`
}

func ints(a []int) []int { return append([]int{}, a...) }
func Build() Catalog {
	out := Catalog{Jobs: []Job{}, Skills: []Skill{}, Source: "midgard-ro: skills.Tree / Name / Sprite / InfoOf / EffectsOf"}
	owners := map[uint16][]int{}
	for _, id := range skills.JobIDs() {
		job := Job{ID: id, Name: jobs.Name(uint16(id)), Skills: []uint16{}}
		seen := map[uint16]bool{}
		for _, slot := range skills.Tree(id) {
			if !seen[slot.Skill] {
				seen[slot.Skill] = true
				job.Skills = append(job.Skills, slot.Skill)
				owners[slot.Skill] = append(owners[slot.Skill], id)
			}
		}
		out.Jobs = append(out.Jobs, job)
	}
	for _, id := range skills.IDs() {
		info, hasInfo := skills.InfoOf(id)
		row := Skill{ID: id, Name: skills.Name(id), Icon: skills.Sprite(id), Jobs: ints(owners[id]), HasInfo: hasInfo, Kind: info.Kind, Target: info.Target, Elements: append([]string{}, info.Element...), Hits: ints(info.Hits), CastMS: ints(info.CastMs), FixedMS: ints(info.FixedMs), Bindings: []Binding{}, Studio: "unmapped", Sources: []string{"internal/game/skills/names.go", "internal/game/skills/tables.go"}}
		if row.Name == "" {
			row.Name = fmt.Sprintf("Skill %d", id)
		}
		if hasInfo {
			row.Sources = append(row.Sources, "internal/game/skills/info.go")
		}
		if effects, known := skills.EffectsOf(id); known {
			for _, phase := range []struct {
				name    string
				effects []string
			}{{"cast", effects.BeginCast}, {"caster", effects.OnCaster}, {"target", effects.OnTarget}, {"ground", effects.OnGround}} {
				if len(phase.effects) > 0 {
					row.Bindings = append(row.Bindings, Binding{phase.name, append([]string{}, phase.effects...)})
				}
			}
			if len(row.Bindings) > 0 {
				row.Studio = "mapped"
			}
			row.Sources = append(row.Sources, "internal/game/skills/skilleffects.go")
		}
		// Only registered client adapters can be played in the Studio viewport.
		if adapter, ok := skillvisual.PreviewOf(id); ok {
			row.Studio = "preview"
			if id == 16 {
				row.Studio = "partial"
			}
			row.Sources = append(row.Sources, adapter.Source)
		}
		out.Skills = append(out.Skills, row)
	}
	return out
}
