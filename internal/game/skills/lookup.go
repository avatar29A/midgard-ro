package skills

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
