// Package effectcatalog joins editorial labels with live client bindings and GRF identities.
package effectcatalog

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Faultbox/midgard-ro/internal/assetworkbench"
	"github.com/Faultbox/midgard-ro/internal/engine/skillvisual"
	"github.com/Faultbox/midgard-ro/internal/skillcatalog"
)

const MetadataPath = "tools/skill-studio/catalog/effects.json"

type Label struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	English     string   `json:"english"`
	Description string   `json:"description"`
	Kind        string   `json:"kind"`
	Tags        []string `json:"tags"`
	SkillID     uint16   `json:"skillId"`
	Paths       []string `json:"paths"`
}
type Resource struct {
	Path      string `json:"path"`
	ID        string `json:"id"`
	Type      string `json:"type"`
	Available bool   `json:"available"`
}
type Usage struct {
	ID    uint16 `json:"id"`
	Name  string `json:"name"`
	Phase string `json:"phase"`
}
type Entry struct {
	ID           string     `json:"id"`
	Name         string     `json:"name"`
	English      string     `json:"english"`
	Description  string     `json:"description"`
	Kind         string     `json:"kind"`
	Tags         []string   `json:"tags"`
	Effects      []string   `json:"effects"`
	Skills       []Usage    `json:"skills"`
	Resources    []Resource `json:"resources"`
	PreviewSkill uint16     `json:"previewSkill"`
	Curated      bool       `json:"curated"`
	Sources      []string   `json:"sources"`
}
type Result struct {
	Entries     []Entry `json:"entries"`
	Total       int     `json:"total"`
	Matched     int     `json:"matched"`
	Next        int     `json:"next"`
	Revision    string  `json:"revision"`
	Fingerprint string  `json:"fingerprint"`
}

func blank(id, name, kind string) Entry {
	return Entry{ID: id, Name: name, Kind: kind, Tags: []string{}, Effects: []string{}, Skills: []Usage{}, Resources: []Resource{}, Sources: []string{}}
}
func Load(root string, c *assetworkbench.Catalog, query, kind, mode string, skill uint16, offset, limit int) (Result, error) {
	out := Result{Entries: []Entry{}, Next: -1, Fingerprint: c.Fingerprint}
	data, err := os.ReadFile(filepath.Join(root, MetadataPath))
	if err != nil {
		return out, err
	}
	if len(data) > 1<<20 {
		return out, fmt.Errorf("catalog metadata exceeds 1 MiB")
	}
	var labels []Label
	if err = json.Unmarshal(data, &labels); err != nil {
		return out, err
	}
	sum := sha256.Sum256(data)
	out.Revision = hex.EncodeToString(sum[:])
	resolve := func(path string) Resource {
		r := Resource{Path: path, Type: strings.TrimPrefix(filepath.Ext(path), ".")}
		if e, err := c.ResolvePath(path); err == nil {
			r.ID = e.ID
			r.Type = e.Type
			r.Available = true
		}
		return r
	}
	byEF := map[string]*Entry{}
	allSkills := skillcatalog.Build().Skills
	for _, s := range allSkills {
		for _, b := range s.Bindings {
			for _, ef := range b.Effects {
				row := byEF[ef]
				if row == nil {
					v := blank(ef, ef, "binding")
					v.Effects = []string{ef}
					v.Description = "Привязка клиента. STR — кандидат из таблицы имён; эффект может также требовать процедурного кода."
					v.Sources = []string{"internal/game/skills/skilleffects.go", "internal/engine/skillvisual/registry.go"}
					v.Resources = []Resource{resolve(`data\texture\effect\` + skillvisual.EffectFileFor(ef))}
					row = &v
					byEF[ef] = row
				}
				row.Skills = append(row.Skills, Usage{s.ID, s.Name, b.Phase})
			}
		}
	}
	rows := []Entry{}
	seen := map[string]bool{}
	for _, l := range labels {
		if l.ID == "" || l.Name == "" || seen[l.ID] {
			return out, fmt.Errorf("invalid/duplicate editorial ID %q", l.ID)
		}
		seen[l.ID] = true
		adapter, ok := skillvisual.PreviewOf(l.SkillID)
		if !ok || adapter.ID != l.ID {
			return out, fmt.Errorf("catalog %s does not match a client adapter", l.ID)
		}
		e := blank(l.ID, l.Name, l.Kind)
		e.English = l.English
		e.Description = l.Description
		e.Tags = l.Tags
		e.PreviewSkill = l.SkillID
		e.Curated = true
		e.Sources = []string{MetadataPath, adapter.Source}
		paths := map[string]bool{}
		for _, path := range l.Paths {
			paths[path] = true
			e.Resources = append(e.Resources, resolve(path))
		}
		for _, s := range allSkills {
			if s.ID != l.SkillID {
				continue
			}
			for _, b := range s.Bindings {
				for _, ef := range b.Effects {
					if b.Phase == "cast" {
						continue
					}
					e.Effects = append(e.Effects, ef)

					r := byEF[ef].Resources[0]
					if r.Available && !paths[r.Path] {
						paths[r.Path] = true
						e.Resources = append(e.Resources, r)
					}
				}
			}
		}
		// Persistent visuals are bound by status/unit code, not always EF tables.
		own := false
		for _, u := range e.Skills {
			if u.ID == l.SkillID {
				own = true
			}
		}
		if !own {
			for _, s := range allSkills {
				if s.ID == l.SkillID {
					e.Skills = append(e.Skills, Usage{s.ID, s.Name, "runtime"})
				}
			}
		}
		rows = append(rows, e)
	}
	keys := []string{}
	for ef := range byEF {
		keys = append(keys, ef)
	}
	sort.Strings(keys)
	for _, ef := range keys {
		rows = append(rows, *byEF[ef])
	}
	// All animations remain discoverable without guessed translations or skill associations.
	uses := map[string][]Usage{}
	effects := map[string][]string{}
	aliases := map[string][]string{}
	for _, r := range rows {
		for _, asset := range r.Resources {
			if asset.ID != "" {
				if r.Curated {
					aliases[asset.ID] = append(aliases[asset.ID], append([]string{r.Name, r.English}, r.Tags...)...)
				}
				for _, u := range r.Skills {
					if !containsUsage(uses[asset.ID], u) {
						uses[asset.ID] = append(uses[asset.ID], u)
					}
				}
				effects[asset.ID] = append(effects[asset.ID], r.Effects...)
			}
		}
	}
	for _, t := range []string{"str", "act", "spr"} {
		found := c.Search("", t, -1, 0, c.Total)
		for _, a := range found.Entries {
			e := blank("resource:"+a.ID, a.Path, t)
			e.Description = "Исходная анимация GRF; назначение и полнота эффекта требуют исследования."
			e.Resources = []Resource{{a.Path, a.ID, a.Type, true}}
			if tags := aliases[a.ID]; tags != nil {
				e.Tags = tags
			}
			if u := uses[a.ID]; u != nil {
				e.Skills = u
			}
			if f := effects[a.ID]; f != nil {
				e.Effects = f
			}
			rows = append(rows, e)
		}
	}
	out.Total = len(rows)
	words := strings.Fields(strings.ToLower(query))
	for _, e := range rows {
		raw := strings.HasPrefix(e.ID, "resource:")
		if mode == "curated" && !e.Curated || mode == "bindings" && e.Kind != "binding" || mode == "resources" && !raw {
			continue
		}
		if kind != "" && kind != e.Kind {
			continue
		}
		if skill != 0 {
			found := false
			for _, u := range e.Skills {
				if u.ID == skill {
					found = true
				}
			}
			if !found {
				continue
			}
		}
		hay := e.ID + " " + e.Name + " " + e.English + " " + e.Description + " " + strings.Join(e.Tags, " ") + " " + strings.Join(e.Effects, " ")
		for _, r := range e.Resources {
			hay += " " + r.Path
		}
		for _, u := range e.Skills {
			hay += fmt.Sprintf(" %s %d", u.Name, u.ID)
		}
		hay = strings.ToLower(hay)
		match := true
		for _, w := range words {
			if !strings.Contains(hay, w) {
				match = false
				break
			}
		}
		if !match {
			continue
		}
		if out.Matched >= offset && len(out.Entries) < limit {
			out.Entries = append(out.Entries, e)
		}
		out.Matched++
	}
	if offset+len(out.Entries) < out.Matched {
		out.Next = offset + len(out.Entries)
	}
	return out, nil
}
func containsUsage(a []Usage, b Usage) bool {
	for _, v := range a {
		if v == b {
			return true
		}
	}
	return false
}
