package ui

import "strconv"

// Job names by class id, from the character list and the Basic Info panel.
//
// Moved here when the ImGui widgets were deleted: the table was the only
// thing in charselect_ui.go that was not ImGui, and both native panels read
// it.

// getJobName returns the job class name from the job ID.
func getJobName(jobID uint16) string {
	jobs := map[uint16]string{
		0:    "Novice",
		1:    "Swordman",
		2:    "Mage",
		3:    "Archer",
		4:    "Acolyte",
		5:    "Merchant",
		6:    "Thief",
		7:    "Knight",
		8:    "Priest",
		9:    "Wizard",
		10:   "Blacksmith",
		11:   "Hunter",
		12:   "Assassin",
		13:   "Knight (Peco)",
		14:   "Crusader",
		15:   "Monk",
		16:   "Sage",
		17:   "Rogue",
		18:   "Alchemist",
		19:   "Bard",
		20:   "Dancer",
		21:   "Crusader (Peco)",
		23:   "Super Novice",
		24:   "Gunslinger",
		25:   "Ninja",
		4001: "High Novice",
		4002: "High Swordman",
		4003: "High Mage",
		4004: "High Archer",
		4005: "High Acolyte",
		4006: "High Merchant",
		4007: "High Thief",
		4008: "Lord Knight",
		4009: "High Priest",
		4010: "High Wizard",
		4011: "Whitesmith",
		4012: "Sniper",
		4013: "Assassin Cross",
		4014: "Lord Knight (Peco)",
		4015: "Paladin",
		4016: "Champion",
		4017: "Professor",
		4018: "Stalker",
		4019: "Creator",
		4020: "Clown",
		4021: "Gypsy",
		4022: "Paladin (Peco)",
		4023: "Baby",
		4024: "Baby Swordman",
		4025: "Baby Mage",
		4026: "Baby Archer",
		4027: "Baby Acolyte",
		4028: "Baby Merchant",
		4029: "Baby Thief",
		4030: "Baby Knight",
		4031: "Baby Priest",
		4032: "Baby Wizard",
		4033: "Baby Blacksmith",
		4034: "Baby Hunter",
		4035: "Baby Assassin",
		4045: "Super Baby",
		4046: "Taekwon",
		4047: "Star Gladiator",
		4049: "Soul Linker",
		4054: "Rune Knight",
		4055: "Warlock",
		4056: "Ranger",
		4057: "Arch Bishop",
		4058: "Mechanic",
		4059: "Guillotine Cross",
		4060: "Rune Knight (Dragon)",
		4066: "Royal Guard",
		4067: "Sorcerer",
		4068: "Minstrel",
		4069: "Wanderer",
		4070: "Sura",
		4071: "Genetic",
		4072: "Shadow Chaser",
		4073: "Royal Guard (Gryphon)", //nolint:misspell // "Gryphon" is the RO Royal Guard mount name

		// Doram. Creatable on our server alongside Novice, so this is the one
		// gap in the table a player can reach without the server ever having
		// been upgraded — it showed as "Unknown (4218)" on the status panel of
		// a character the client had just made. Named as the server names it
		// (map_msg.conf:729).
		4218: "Summoner",
	}

	if name, ok := jobs[jobID]; ok {
		return name
	}

	// Named rather than blank: a job the table does not know is a newer class
	// than the table, and its number is the only useful thing to show.
	return "Unknown (" + strconv.Itoa(int(jobID)) + ")"
}
