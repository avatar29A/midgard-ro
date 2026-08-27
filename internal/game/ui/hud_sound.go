package ui

import "github.com/Faultbox/midgard-ro/internal/engine/ui2d"

// The Sound Configuration dialog: a level and an on switch for the music and
// for the effects, laid out as the original has them.
//
// The switch is not a second setting behind the slider. Turning a channel off
// silences it and leaves the slider where it was, so turning it back on
// returns to the level you had rather than to whatever the slider would
// otherwise have been dragged to.

// SoundSettings is what the dialog holds, and what the caller applies.
type SoundSettings struct {
	BGMVolume float32
	SFXVolume float32
	BGMOn     bool
	SFXOn     bool
}

const (
	soundWindowID = "hud_sound_config"

	soundW float32 = 330
	soundH float32 = 104

	soundRowH   float32 = 30
	soundLabelX float32 = 12
	soundNameX  float32 = 76
	soundBarX   float32 = 130
	soundBarW   float32 = 130
	soundBoxX   float32 = 274
	soundBox    float32 = 14
)

// soundRows are the channels the dialog offers, in the original's order.
var soundRows = []struct {
	id    string
	label string
}{
	{"bgm", "BGM"},
	{"effect", "Effect"},
}

// SoundOpen reports whether the dialog is showing.
func (b *UI2DBackend) SoundOpen() bool {
	return b.soundOpen
}

// TakeSoundSettings returns the settings when they have changed, and reports
// false when they have not. The interface has no audio device of its own, so
// applying them is the caller's job.
func (b *UI2DBackend) TakeSoundSettings() (SoundSettings, bool) {
	if !b.soundDirty {
		return SoundSettings{}, false
	}

	b.soundDirty = false

	return b.sound, true
}

// SetSoundSettings seeds the dialog from what is actually playing, so it opens
// showing the levels in force rather than its own defaults.
func (b *UI2DBackend) SetSoundSettings(s SoundSettings) {
	if b.soundSeeded {
		return
	}

	b.sound = s
	b.soundSeeded = true
}

// drawSoundConfig draws the dialog.
func (b *UI2DBackend) drawSoundConfig(screenW, screenH float32) {
	if !b.soundOpen {
		return
	}

	// Opens above the menu it was opened from, so both are readable at once.
	openX := (screenW - soundW) / 2
	openY := (screenH-escMenuH)/2 - soundH - 12
	if openY < 0 {
		openY = 0
	}

	if !b.ctx.BeginWindow(soundWindowID, openX, openY, soundW, soundH, "Sound Configuration") {
		// Minimized is not closed: the title bar is drawn and the dialog is
		// still open, so only a real close puts it away.
		if !b.ctx.WindowMinimized(soundWindowID) {
			b.soundOpen = false
		}

		return
	}

	// Read back after BeginWindow: before, the position is last frame's, and
	// the contents trail the frame while it is dragged.
	x, y := openX, openY
	if rect, ok := b.ctx.WindowRect(soundWindowID); ok {
		x, y = rect.X, rect.Y
	}

	b.ctx.CaptureMouse(ui2d.Rect{X: x, Y: y, W: soundW, H: soundH})

	r := b.ctx.Renderer()
	top := y + ui2d.FrameTitleH + 8

	// The group's name, once, against the first row.
	_, capH := r.MeasureText("Sound", 1)
	r.DrawText(x+soundLabelX, top+(soundRowH-capH)/2, "Sound", 1, ui2d.ColorText)

	for i, row := range soundRows {
		rowY := top + float32(i)*soundRowH

		_, nameH := r.MeasureText(row.label, 1)
		r.DrawText(x+soundNameX, rowY+(soundRowH-nameH)/2, row.label, 1, ui2d.ColorText)

		level, on := b.soundChannel(i)

		if v, changed := b.ctx.SliderAt("sound_"+row.id, x+soundBarX, rowY, soundBarW, soundRowH, level); changed {
			b.setSoundChannel(i, v, on)
		}

		boxY := rowY + (soundRowH-soundBox)/2
		if want := b.ctx.CheckboxAt("sound_on_"+row.id, x+soundBoxX, boxY, soundBox, "on", on); want != on {
			b.setSoundChannel(i, level, want)
		}
	}

	b.ctx.EndWindow()
}

// soundChannel returns one row's level and switch.
func (b *UI2DBackend) soundChannel(index int) (float32, bool) {
	if index == 0 {
		return b.sound.BGMVolume, b.sound.BGMOn
	}

	return b.sound.SFXVolume, b.sound.SFXOn
}

// setSoundChannel records one row's level and switch, and marks the pair for
// the caller to pick up.
func (b *UI2DBackend) setSoundChannel(index int, level float32, on bool) {
	if index == 0 {
		b.sound.BGMVolume, b.sound.BGMOn = level, on
	} else {
		b.sound.SFXVolume, b.sound.SFXOn = level, on
	}

	b.soundDirty = true
}
