package ui

import (
	"github.com/Faultbox/midgard-ro/internal/engine/ui2d"
)

// The buttons a level leaves at the foot of the screen.
//
// The original does not open anything by itself when you advance. It puts a
// button in the corner and waits: the base level's on the left, the job
// level's on the right. Pressing one opens the window with the points to
// spend in it, and takes the button away.
const (
	levelUpOff = basicInterfacePath + "lv_up_off.bmp"
	levelUpOn  = basicInterfacePath + "lv_up_on.bmp"

	// levelUpSize is the button's own size in the archive.
	levelUpSize = float32(43)

	// levelUpMargin is how far the buttons sit from the corner they are in.
	levelUpMargin = float32(10)
)

// LevelUpButtons says which of the two are waiting to be pressed.
type LevelUpButtons struct {
	Base bool
	Job  bool
}

// LevelUpAction is one of them being pressed.
type LevelUpAction struct {
	// Base distinguishes the two. Pressed is what makes the action real,
	// since Base false is a job level rather than nothing.
	Base    bool
	Pressed bool
}

// TakeLevelUpAction returns a pressed button and clears it.
func (b *UI2DBackend) TakeLevelUpAction() (LevelUpAction, bool) {
	action := b.levelUpAction
	if !action.Pressed {
		return LevelUpAction{}, false
	}

	b.levelUpAction = LevelUpAction{}

	return action, true
}

// drawLevelUpButtons puts a button in each corner that has a level waiting.
func (b *UI2DBackend) drawLevelUpButtons(buttons LevelUpButtons, screenW, screenH float32) {
	y := screenH - levelUpSize - levelUpMargin

	if buttons.Base {
		if b.drawLevelUpButton("hud_levelup_base", levelUpMargin, y) {
			b.levelUpAction = LevelUpAction{Base: true, Pressed: true}
			b.OpenWindow(WindowEquip)
		}
	}

	if buttons.Job {
		x := screenW - levelUpSize - levelUpMargin
		if b.drawLevelUpButton("hud_levelup_job", x, y) {
			b.levelUpAction = LevelUpAction{Pressed: true}
			b.OpenWindow(WindowSkill)
		}
	}
}

// drawLevelUpButton draws one, lit under the pointer, and reports a press.
func (b *UI2DBackend) drawLevelUpButton(id string, x, y float32) bool {
	box := ui2d.Rect{X: x, Y: y, W: levelUpSize, H: levelUpSize}

	asset := levelUpOff
	if box.Contains(b.ctx.Input().MouseX, b.ctx.Input().MouseY) {
		asset = levelUpOn
	}

	tex, err := b.texCache.Load(asset)
	if err != nil {
		return false
	}

	b.ctx.Renderer().DrawImage(tex.ID, box.X, box.Y, box.W, box.H, ui2d.ColorWhite)

	// The whole button claims the pointer, or the press would fall through
	// and walk the character to whatever is behind it.
	b.ctx.CaptureMouse(box)

	return b.ctx.InvisibleButtonAt(id, box.X, box.Y, box.W, box.H)
}
