// Package ui provides game user interface components.
package ui

import (
	"fmt"

	"github.com/AllenDang/cimgui-go/imgui"

	"github.com/Faultbox/midgard-ro/internal/game/states"
	"github.com/Faultbox/midgard-ro/internal/network/packets"
)

// CharSelectUI renders the character selection UI.
type CharSelectUI struct {
	state *states.CharSelectState

	// Selected character index in the UI
	selectedIndex int
}

// NewCharSelectUI creates a new character selection UI.
func NewCharSelectUI(state *states.CharSelectState) *CharSelectUI {
	return &CharSelectUI{
		state:         state,
		selectedIndex: -1,
	}
}

// Render renders the character selection UI.
func (ui *CharSelectUI) Render(viewportWidth, viewportHeight float32) {
	// Main window takes most of the screen
	windowWidth := float32(700)
	windowHeight := float32(500)
	windowX := (viewportWidth - windowWidth) / 2
	windowY := (viewportHeight - windowHeight) / 2

	imgui.SetNextWindowPos(imgui.NewVec2(windowX, windowY))
	imgui.SetNextWindowSize(imgui.NewVec2(windowWidth, windowHeight))

	flags := imgui.WindowFlagsNoResize | imgui.WindowFlagsNoMove | imgui.WindowFlagsNoCollapse
	if imgui.BeginV("Character Selection", nil, flags) {
		ui.renderContent()
	}
	imgui.End()
}

func (ui *CharSelectUI) renderContent() {
	// Status message
	if statusMsg := ui.state.GetStatusMessage(); statusMsg != "" {
		imgui.Text(statusMsg)
	}

	// Error message
	if errMsg := ui.state.GetErrorMessage(); errMsg != "" {
		imgui.TextColored(imgui.NewVec4(1, 0.3, 0.3, 1), errMsg)
	}

	imgui.Separator()
	imgui.Spacing()

	if !ui.state.IsCharListReady() {
		// Loading state
		centerText("Loading character list...")
		return
	}

	characters := ui.state.GetCharacters()
	if len(characters) == 0 {
		// No characters
		imgui.Spacing()
		centerText("No characters found.")
		imgui.Spacing()
		centerText("Create a new character on the server.")
		return
	}

	// Character list on the left, details on the right
	if imgui.BeginTable("charLayout", 2) {
		imgui.TableSetupColumnV("List", imgui.TableColumnFlagsWidthFixed, 300, 0)
		imgui.TableSetupColumnV("Details", imgui.TableColumnFlagsWidthStretch, 0, 0)

		imgui.TableNextRow()
		imgui.TableNextColumn()

		// Character list
		ui.renderCharacterList(characters)

		imgui.TableNextColumn()

		// Character details
		ui.renderCharacterDetails()

		imgui.EndTable()
	}

	imgui.Spacing()
	imgui.Separator()
	imgui.Spacing()

	// Action buttons
	ui.renderActionButtons()
}

func (ui *CharSelectUI) renderCharacterList(characters []*packets.CharInfo) {
	imgui.Text("Characters:")
	imgui.Spacing()

	// List of characters
	if imgui.BeginListBoxV("##charlist", imgui.NewVec2(-1, 300)) {
		for i, char := range characters {
			label := fmt.Sprintf("%s (Lv %d %s)", char.GetName(), char.BaseLevel, getJobName(char.Class))
			isSelected := ui.selectedIndex == i
			if imgui.SelectableBoolV(label, isSelected, 0, imgui.NewVec2(0, 0)) {
				ui.selectedIndex = i
			}
		}
		imgui.EndListBox()
	}
}

func (ui *CharSelectUI) renderCharacterDetails() {
	imgui.Text("Character Info:")
	imgui.Spacing()

	if ui.selectedIndex < 0 || ui.selectedIndex >= len(ui.state.GetCharacters()) {
		imgui.TextDisabled("Select a character to view details")
		return
	}

	char := ui.state.GetCharacters()[ui.selectedIndex]

	// Character info table
	if imgui.BeginTable("charinfo", 2) {
		imgui.TableSetupColumnV("Label", imgui.TableColumnFlagsWidthFixed, 100, 0)
		imgui.TableSetupColumnV("Value", imgui.TableColumnFlagsWidthStretch, 0, 0)

		addInfoRow("Name:", char.GetName())
		addInfoRow("Job:", getJobName(char.Class))
		addInfoRow("Base Level:", fmt.Sprintf("%d", char.BaseLevel))
		addInfoRow("Job Level:", fmt.Sprintf("%d", char.JobLevel))
		addInfoRow("HP:", fmt.Sprintf("%d / %d", char.HP, char.MaxHP))
		addInfoRow("SP:", fmt.Sprintf("%d / %d", char.SP, char.MaxSP))
		addInfoRow("Zeny:", fmt.Sprintf("%d", char.Zeny))
		addInfoRow("Map:", char.GetMapName())

		imgui.EndTable()
	}

	imgui.Spacing()
	imgui.Separator()
	imgui.Spacing()

	// Stats
	imgui.Text("Stats:")
	if imgui.BeginTable("charstats", 2) {
		imgui.TableSetupColumnV("Stat", imgui.TableColumnFlagsWidthFixed, 60, 0)
		imgui.TableSetupColumnV("Value", imgui.TableColumnFlagsWidthStretch, 0, 0)

		addStatRow("STR", int(char.Str))
		addStatRow("AGI", int(char.Agi))
		addStatRow("VIT", int(char.Vit))
		addStatRow("INT", int(char.Int))
		addStatRow("DEX", int(char.Dex))
		addStatRow("LUK", int(char.Luk))

		imgui.EndTable()
	}
}

func (ui *CharSelectUI) renderActionButtons() {
	// Select button
	imgui.BeginDisabledV(ui.selectedIndex < 0 || ui.state.IsLoadingState())
	if imgui.ButtonV("Enter Game", imgui.NewVec2(150, 30)) {
		ui.state.SelectCharacter(ui.selectedIndex)
	}
	imgui.EndDisabled()

	imgui.SameLine()

	// Create character button (placeholder)
	imgui.BeginDisabledV(true)
	if imgui.ButtonV("Create Character", imgui.NewVec2(150, 0)) {
		// TODO: Character creation
	}
	imgui.EndDisabled()

	imgui.SameLine()

	// Delete character button (placeholder)
	imgui.BeginDisabledV(true)
	if imgui.ButtonV("Delete Character", imgui.NewVec2(150, 0)) {
		// TODO: Character deletion
	}
	imgui.EndDisabled()
}

func addInfoRow(label, value string) {
	imgui.TableNextRow()
	imgui.TableNextColumn()
	imgui.Text(label)
	imgui.TableNextColumn()
	imgui.Text(value)
}

func addStatRow(name string, value int) {
	imgui.TableNextRow()
	imgui.TableNextColumn()
	imgui.Text(name)
	imgui.TableNextColumn()
	imgui.Text(fmt.Sprintf("%d", value))
}

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
		4073: "Royal Guard (Gryphon)",
	}

	if name, ok := jobs[jobID]; ok {
		return name
	}
	return fmt.Sprintf("Unknown (%d)", jobID)
}
