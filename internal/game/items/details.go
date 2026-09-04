package items

import (
	_ "embed"
	"strconv"
	"strings"
	"sync"
)

// What the server's database says about an item beyond its name.
//
// The inventory packet carries an id, a count and whether the thing is worn.
// That is enough for a grid of icons and nothing else: a player looking at a
// Guard cannot see that it is armour, that it weighs thirty, that it goes in
// the left hand or that it can be refined.
//
// Its own file rather than more columns on names.txt, and parsed only when
// somebody asks: the names are read every time the inventory is drawn, and
// this is read when one item is looked at.

//go:embed details.txt
var detailsData string

// Detail is one item's figures.
//
// Every field is the server's own, in the server's own units. Weight in
// particular is tenths, as rAthena stores it — a Red Potion is 70 here and
// seven on screen.
type Detail struct {
	// Type is Weapon, Armor, Healing, Card, Etc and the rest, and SubType
	// narrows a weapon to what it is swung as. SubType is empty for
	// everything that is not a weapon.
	Type    string
	SubType string

	Weight int

	Attack  int
	Defense int
	Range   int

	// Slots is how many cards it takes.
	Slots int

	// Level is the weapon's or the armour's own level, and MinLevel the base
	// level a character has to reach before they may wear it.
	Level    int
	MinLevel int

	Refineable bool

	// Locations is where it is worn and Jobs who may wear it. Jobs is empty
	// when anybody may, which is how the database says it: no list at all.
	Locations []string
	Jobs      []string

	// Buy is what a shop charges for it.
	Buy int
}

// Worn reports whether this is something equipped rather than used or kept.
func (d Detail) Worn() bool {
	return len(d.Locations) > 0
}

var (
	detailsOnce sync.Once
	details     map[uint32]Detail
)

// The columns of details.txt, after the id.
const (
	colType = iota
	colSubType
	colWeight
	colAttack
	colDefense
	colRange
	colSlots
	colLevel
	colMinLevel
	colRefineable
	colLocations
	colJobs
	colBuy

	detailColumns
)

// loadDetails parses the table on first use.
func loadDetails() {
	details = make(map[uint32]Detail, 30000)

	for line := range strings.Lines(detailsData) {
		fields := strings.Split(strings.TrimSuffix(line, "\n"), "\t")
		if len(fields) < 1+detailColumns {
			continue
		}

		id, err := strconv.ParseUint(fields[0], 10, 32)
		if err != nil {
			continue
		}

		at := fields[1:]

		details[uint32(id)] = Detail{
			Type:       at[colType],
			SubType:    at[colSubType],
			Weight:     number(at[colWeight]),
			Attack:     number(at[colAttack]),
			Defense:    number(at[colDefense]),
			Range:      number(at[colRange]),
			Slots:      number(at[colSlots]),
			Level:      number(at[colLevel]),
			MinLevel:   number(at[colMinLevel]),
			Refineable: at[colRefineable] != "",
			Locations:  words(at[colLocations]),
			Jobs:       words(at[colJobs]),
			Buy:        number(at[colBuy]),
		}
	}
}

// number reads a column, with an empty one meaning nought — which is how the
// generator writes every figure an item does not have.
func number(field string) int {
	if field == "" {
		return 0
	}

	n, err := strconv.Atoi(field)
	if err != nil {
		return 0
	}

	return n
}

// words splits a comma-separated column, and nil for an empty one.
func words(field string) []string {
	if field == "" {
		return nil
	}

	return strings.Split(field, ",")
}

// DetailOf is what the database says about an item, and whether it says
// anything: an item newer than the tree this was generated from has no entry.
func DetailOf(id uint32) (Detail, bool) {
	detailsOnce.Do(loadDetails)

	detail, ok := details[id]

	return detail, ok
}

// DetailCount is how many items the table holds, for tests that check the
// generator produced something rather than an empty file.
func DetailCount() int {
	detailsOnce.Do(loadDetails)

	return len(details)
}
