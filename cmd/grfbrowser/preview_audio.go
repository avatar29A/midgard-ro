// Audio preview for GRF Browser.
package main

import (
	"bytes"
	"fmt"
	"os"
	"time"

	"github.com/AllenDang/cimgui-go/imgui"
	"github.com/gopxl/beep/v2"
	"github.com/gopxl/beep/v2/speaker"
	"github.com/gopxl/beep/v2/wav"
)

// loadAudioPreview loads a WAV file for audio preview.
func (app *App) loadAudioPreview(path string) {
	data, err := app.archive.Read(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading audio file: %v\n", err)
		return
	}

	// Decode WAV from memory
	streamer, format, err := wav.Decode(bytes.NewReader(data))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error decoding WAV: %v\n", err)
		return
	}

	// Initialize speaker once (use common sample rate)
	speakerInitOnce.Do(func() {
		err := speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/10))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error initializing speaker: %v\n", err)
			return
		}
		speakerInited = true
	})

	if !speakerInited {
		streamer.Close()
		return
	}

	app.audioStreamer = streamer
	app.audioFormat = format
	app.audioLength = streamer.Len()
	app.audioSampleRate = format.SampleRate
	app.audioPlaying = false
	app.audioCtrl = nil
}

// renderAudioPreview renders the audio player with controls.
func (app *App) renderAudioPreview() {
	if app.audioStreamer == nil {
		imgui.TextDisabled("Failed to load audio")
		return
	}

	// Audio info
	duration := app.audioSampleRate.D(app.audioLength)
	imgui.Text(fmt.Sprintf("Format: %d Hz, %d ch", app.audioFormat.SampleRate, app.audioFormat.NumChannels))
	imgui.Text(fmt.Sprintf("Duration: %.1f sec", duration.Seconds()))

	imgui.Separator()

	// Play/Stop buttons
	if app.audioPlaying {
		if imgui.ButtonV("Stop", imgui.NewVec2(80, 0)) {
			app.stopAudio()
		}
	} else {
		if imgui.ButtonV("Play", imgui.NewVec2(80, 0)) {
			app.playAudio()
		}
	}

	imgui.SameLine()

	// Progress bar
	var progress float32
	var currentPos int
	if app.audioStreamer != nil && app.audioLength > 0 {
		currentPos = app.audioStreamer.Position()
		progress = float32(currentPos) / float32(app.audioLength)
	}

	currentTime := app.audioSampleRate.D(currentPos)
	imgui.Text(fmt.Sprintf("%.1f / %.1f", currentTime.Seconds(), duration.Seconds()))

	// Progress bar (full width)
	imgui.ProgressBarV(progress, imgui.NewVec2(-1, 0), "")

	// Check if playback finished
	if app.audioPlaying && currentPos >= app.audioLength {
		app.audioPlaying = false
	}
}

// playAudio starts audio playback.
func (app *App) playAudio() {
	if app.audioStreamer == nil || !speakerInited {
		return
	}

	// Reset to beginning
	if err := app.audioStreamer.Seek(0); err != nil {
		fmt.Fprintf(os.Stderr, "Error seeking audio: %v\n", err)
		return
	}

	// Create control wrapper for pause/resume
	app.audioCtrl = &beep.Ctrl{Streamer: app.audioStreamer, Paused: false}
	app.audioPlaying = true

	// Play with callback when done
	speaker.Play(beep.Seq(app.audioCtrl, beep.Callback(func() {
		app.audioPlaying = false
	})))
}

// stopAudio stops audio playback and releases resources.
func (app *App) stopAudio() {
	if speakerInited {
		speaker.Clear()
	}
	app.audioPlaying = false
	app.audioCtrl = nil
	if app.audioStreamer != nil {
		app.audioStreamer.Close()
		app.audioStreamer = nil
	}
}
