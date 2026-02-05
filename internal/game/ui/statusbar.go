// Package ui provides game user interface components.
package ui

import (
	"fmt"

	"github.com/AllenDang/cimgui-go/imgui"

	"github.com/Faultbox/midgard-ro/internal/game/entity"
)

// StatusBar renders the player status bars (HP, SP, EXP).
type StatusBar struct {
	// Entity to display stats for
	entity *entity.Entity

	// Display settings
	ShowNumeric bool // Show HP/SP as numbers
	Compact     bool // Compact mode (smaller bars)
}

// NewStatusBar creates a new status bar.
func NewStatusBar() *StatusBar {
	return &StatusBar{
		ShowNumeric: true,
		Compact:     false,
	}
}

// SetEntity sets the entity to display stats for.
func (sb *StatusBar) SetEntity(e *entity.Entity) {
	sb.entity = e
}

// Render renders the status bar at the specified position.
func (sb *StatusBar) Render(x, y float32) {
	if sb.entity == nil {
		return
	}

	var width, height float32
	if sb.Compact {
		width = 200
		height = 60
	} else {
		width = 250
		height = 80
	}

	imgui.SetNextWindowPos(imgui.NewVec2(x, y))
	imgui.SetNextWindowSize(imgui.NewVec2(width, height))

	flags := imgui.WindowFlagsNoTitleBar | imgui.WindowFlagsNoResize |
		imgui.WindowFlagsNoMove | imgui.WindowFlagsNoScrollbar |
		imgui.WindowFlagsNoCollapse

	imgui.PushStyleVarVec2(imgui.StyleVarWindowPadding, imgui.NewVec2(8, 8))
	imgui.PushStyleVarFloat(imgui.StyleVarWindowRounding, 5)
	imgui.SetNextWindowBgAlpha(0.85)

	if imgui.BeginV("##StatusBar", nil, flags) {
		sb.renderBars()
	}
	imgui.End()

	imgui.PopStyleVar()
	imgui.PopStyleVar()
}

func (sb *StatusBar) renderBars() {
	e := sb.entity

	// HP Bar
	hpPercent := e.HPPercent()
	hpColor := sb.hpColor(hpPercent)
	sb.renderBar("HP", e.HP, e.MaxHP, hpPercent, hpColor)

	imgui.Spacing()

	// SP Bar
	spPercent := e.SPPercent()
	spColor := imgui.NewVec4(0.2, 0.4, 1.0, 1.0) // Blue
	sb.renderBar("SP", e.SP, e.MaxSP, spPercent, spColor)
}

func (sb *StatusBar) renderBar(label string, current, max int, percent float32, color imgui.Vec4) {
	// Label
	imgui.Text(label)
	imgui.SameLine()

	// Custom colored progress bar
	imgui.PushStyleColorVec4(imgui.ColPlotHistogram, color)

	barSize := imgui.NewVec2(-1, 0)
	if sb.Compact {
		barSize.Y = 12
	} else {
		barSize.Y = 16
	}

	var overlay string
	if sb.ShowNumeric {
		overlay = fmt.Sprintf("%d / %d", current, max)
	}
	imgui.ProgressBarV(percent, barSize, overlay)

	imgui.PopStyleColor()
}

func (sb *StatusBar) hpColor(percent float32) imgui.Vec4 {
	if percent > 0.5 {
		// Green to Yellow gradient
		t := (percent - 0.5) * 2
		return imgui.NewVec4(1.0-t*0.5, 0.8+t*0.2, 0.2, 1.0)
	}
	// Yellow to Red gradient
	t := percent * 2
	return imgui.NewVec4(1.0, t*0.8, 0.2*t, 1.0)
}

// EntityHPBar renders a floating HP bar above an entity.
type EntityHPBar struct {
	// Settings
	BarWidth  float32
	BarHeight float32
	ShowName  bool
}

// NewEntityHPBar creates a new entity HP bar renderer.
func NewEntityHPBar() *EntityHPBar {
	return &EntityHPBar{
		BarWidth:  60,
		BarHeight: 6,
		ShowName:  true,
	}
}

// RenderForEntity renders an HP bar for a single entity at screen position.
func (hb *EntityHPBar) RenderForEntity(e *entity.Entity, screenX, screenY float32) {
	if e == nil || !e.ShowHP && !e.ShowName {
		return
	}

	windowWidth := hb.BarWidth + 10
	windowHeight := float32(0)
	if e.ShowName {
		windowHeight += 18
	}
	if e.ShowHP {
		windowHeight += hb.BarHeight + 4
	}

	// Center above entity
	posX := screenX - windowWidth/2
	posY := screenY - windowHeight - 10

	imgui.SetNextWindowPos(imgui.NewVec2(posX, posY))
	imgui.SetNextWindowSize(imgui.NewVec2(windowWidth, windowHeight))

	flags := imgui.WindowFlagsNoTitleBar | imgui.WindowFlagsNoResize |
		imgui.WindowFlagsNoMove | imgui.WindowFlagsNoScrollbar |
		imgui.WindowFlagsNoInputs | imgui.WindowFlagsNoFocusOnAppearing

	imgui.PushStyleVarVec2(imgui.StyleVarWindowPadding, imgui.NewVec2(4, 2))
	imgui.SetNextWindowBgAlpha(0.6)

	windowID := fmt.Sprintf("##EntityHP%d", e.ID)
	if imgui.BeginV(windowID, nil, flags) {
		// Name
		if e.ShowName && e.Name != "" {
			nameColor := imgui.NewVec4(e.NameColor[0], e.NameColor[1], e.NameColor[2], e.NameColor[3])
			imgui.TextColored(nameColor, e.Name)
		}

		// HP Bar
		if e.ShowHP {
			percent := e.HPPercent()
			color := hb.hpColor(percent)

			imgui.PushStyleColorVec4(imgui.ColPlotHistogram, color)
			imgui.PushStyleColorVec4(imgui.ColFrameBg, imgui.NewVec4(0.2, 0.2, 0.2, 0.8))
			imgui.ProgressBarV(percent, imgui.NewVec2(hb.BarWidth, hb.BarHeight), "")
			imgui.PopStyleColor()
			imgui.PopStyleColor()
		}
	}
	imgui.End()
	imgui.PopStyleVar()
}

func (hb *EntityHPBar) hpColor(percent float32) imgui.Vec4 {
	if percent > 0.5 {
		return imgui.NewVec4(0.2, 0.9, 0.2, 1.0) // Green
	} else if percent > 0.25 {
		return imgui.NewVec4(1.0, 0.8, 0.2, 1.0) // Yellow
	}
	return imgui.NewVec4(1.0, 0.2, 0.2, 1.0) // Red
}
