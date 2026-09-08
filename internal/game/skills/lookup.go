package skills

import "sort"

// Reading the generated tables.
//
// What the server sends about a skill is what the character has of it: the id,
// the level, the range, whether it can be raised. What it never sends is any
// of what is here — where the skill sits in the window, what it has to be
// learned after, what it looks like when it goes off. That is the client's own
// data, and these are the ways in.

// Needs is what a skill has to be learned after, and to what level. Empty for
// a skill that stands on its own.
//
// The server refuses a point spent on something out of reach and says nothing
// about why, so this is what the interface needs in order to explain it before
// the request goes out.
func Needs(skill uint16) []Need {
	return needs[skill]
}

// Tree is the grid a job's skill window lays out, in slot order.
//
// Empty for a job the client has no tree for, which is every job that cannot
// learn anything of its own.
func Tree(job int) []TreeSlot {
	return tree[job]
}

// EffectOf is what a skill plays when it is cast, and whether it has anything
// beyond the default.
func EffectOf(skill uint16) (Effect, bool) {
	effect, ok := effects[skill]

	return effect, ok
}

// Jobs is how many jobs have a skill tree, for reporting.
func Jobs() int {
	return len(tree)
}

// EffectsOf is what a skill plays and where, and whether anything is known for
// it.
//
// Four moments, because the original draws four: the circle under the caster
// while it casts, what the caster does when it goes off, what appears on
// whoever it hit, and what stays on a cell for a placed skill. A skill with no
// entry is one nobody has identified yet, not one that plays nothing.
func EffectsOf(skill uint16) (SkillEffects, bool) {
	effects, ok := skillEffects[skill]

	return effects, ok
}

// JobIDs returns the actual pages in the generated client tree.
func JobIDs() []int {
	ids := make([]int, 0, len(tree))
	for id := range tree {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	return ids
}

// IDs is the union of known names, metadata, trees and effect mappings.
func IDs() []uint16 {
	all := map[uint16]bool{}
	for id := range names {
		all[id] = true
	}
	for id := range info {
		all[id] = true
	}
	for id := range skillEffects {
		all[id] = true
	}
	for id := range effects {
		all[id] = true
	}
	for _, slots := range tree {
		for _, slot := range slots {
			all[slot.Skill] = true
		}
	}
	ids := make([]uint16, 0, len(all))
	for id := range all {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}
