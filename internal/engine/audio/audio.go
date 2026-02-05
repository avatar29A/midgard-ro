// Package audio provides audio playback for background music and sound effects.
package audio

import (
	"bytes"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/gopxl/beep/v2"
	"github.com/gopxl/beep/v2/effects"
	"github.com/gopxl/beep/v2/speaker"
	"github.com/gopxl/beep/v2/wav"
)

// DefaultSampleRate is the default sample rate for audio playback.
const DefaultSampleRate = beep.SampleRate(44100)

// Manager handles audio playback for the game.
type Manager struct {
	mu sync.RWMutex

	// State
	initialized bool
	sampleRate  beep.SampleRate

	// BGM
	bgmStreamer beep.StreamSeekCloser
	bgmCtrl     *beep.Ctrl
	bgmVolume   *effects.Volume
	bgmPlaying  bool
	bgmPath     string

	// Volume settings (0.0 to 1.0)
	masterVolume float64
	bgmVolLevel  float64
	sfxVolLevel  float64

	// SFX mixer for concurrent sound effects
	sfxMixer *beep.Mixer
}

// New creates a new audio manager.
func New() *Manager {
	return &Manager{
		masterVolume: 1.0,
		bgmVolLevel:  0.7,
		sfxVolLevel:  1.0,
		sfxMixer:     &beep.Mixer{},
	}
}

// Init initializes the audio system.
func (m *Manager) Init() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.initialized {
		return nil
	}

	m.sampleRate = DefaultSampleRate
	err := speaker.Init(m.sampleRate, m.sampleRate.N(time.Second/30))
	if err != nil {
		return fmt.Errorf("init speaker: %w", err)
	}

	// Start SFX mixer
	speaker.Play(m.sfxMixer)

	m.initialized = true
	return nil
}

// Close shuts down the audio system.
func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.stopBGMInternal()
	speaker.Clear()
	m.initialized = false
}

// IsInitialized returns whether the audio system is initialized.
func (m *Manager) IsInitialized() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.initialized
}

// SetMasterVolume sets the master volume (0.0 to 1.0).
func (m *Manager) SetMasterVolume(vol float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.masterVolume = clamp(vol, 0, 1)
	m.updateBGMVolume()
}

// SetBGMVolume sets the BGM volume (0.0 to 1.0).
func (m *Manager) SetBGMVolume(vol float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.bgmVolLevel = clamp(vol, 0, 1)
	m.updateBGMVolume()
}

// SetSFXVolume sets the SFX volume (0.0 to 1.0).
func (m *Manager) SetSFXVolume(vol float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sfxVolLevel = clamp(vol, 0, 1)
}

// GetMasterVolume returns the master volume.
func (m *Manager) GetMasterVolume() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.masterVolume
}

// GetBGMVolume returns the BGM volume.
func (m *Manager) GetBGMVolume() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.bgmVolLevel
}

// GetSFXVolume returns the SFX volume.
func (m *Manager) GetSFXVolume() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sfxVolLevel
}

func (m *Manager) updateBGMVolume() {
	if m.bgmVolume != nil {
		// Volume uses dB scale, convert from 0-1 to dB
		// Silent = -10, Full = 0
		vol := m.masterVolume * m.bgmVolLevel
		if vol <= 0 {
			m.bgmVolume.Silent = true
		} else {
			m.bgmVolume.Silent = false
			// Convert to dB scale: 0 = silent, 1 = full volume
			m.bgmVolume.Volume = volumeToDb(vol)
		}
	}
}

// volumeToDb converts a 0-1 volume to decibel scale.
func volumeToDb(vol float64) float64 {
	if vol <= 0 {
		return -100 // Effectively silent
	}
	// Use log scale: vol=1 -> 0dB, vol=0.5 -> -6dB, vol=0.25 -> -12dB
	return 20 * log10(vol)
}

func log10(x float64) float64 {
	if x <= 0 {
		return -100
	}
	// log10(x) = ln(x) / ln(10)
	return ln(x) / 2.302585092994046
}

func ln(x float64) float64 {
	// Simple natural log approximation for audio purposes
	if x <= 0 {
		return -100
	}
	if x == 1 {
		return 0
	}
	// Use Taylor series or math.Log if available
	// For simplicity, use a basic approximation
	n := 0
	for x >= 2 {
		x /= 2
		n++
	}
	for x < 0.5 {
		x *= 2
		n--
	}
	// x is now between 0.5 and 2
	// ln(x) ≈ 2 * (x-1)/(x+1) for x near 1
	y := (x - 1) / (x + 1)
	result := 2 * y
	y2 := y * y
	term := y
	for i := 1; i < 10; i++ {
		term *= y2
		result += 2 * term / float64(2*i+1)
	}
	return result + float64(n)*0.693147180559945
}

func clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

// PlayBGM plays background music from WAV data.
// If loop is true, the music will loop indefinitely.
func (m *Manager) PlayBGM(data []byte, path string, loop bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.initialized {
		return fmt.Errorf("audio not initialized")
	}

	// Stop current BGM
	m.stopBGMInternal()

	// Decode WAV
	streamer, format, err := wav.Decode(io.NopCloser(bytes.NewReader(data)))
	if err != nil {
		return fmt.Errorf("decode wav: %w", err)
	}

	// Resample if needed
	var resampled beep.Streamer
	if format.SampleRate != m.sampleRate {
		resampled = beep.Resample(4, format.SampleRate, m.sampleRate, streamer)
	} else {
		resampled = streamer
	}

	// Create looping streamer if needed
	var finalStreamer beep.Streamer
	if loop {
		// For looping, we need a seekable streamer
		finalStreamer = &loopStreamer{
			streamer: streamer,
			resampled: resampled,
			loop:      true,
		}
	} else {
		finalStreamer = resampled
	}

	// Create control wrapper
	m.bgmCtrl = &beep.Ctrl{Streamer: finalStreamer, Paused: false}

	// Create volume wrapper
	m.bgmVolume = &effects.Volume{
		Streamer: m.bgmCtrl,
		Base:     2,
		Volume:   0,
		Silent:   false,
	}
	m.updateBGMVolume()

	m.bgmStreamer = streamer
	m.bgmPath = path
	m.bgmPlaying = true

	// Play with callback when done
	speaker.Play(beep.Seq(m.bgmVolume, beep.Callback(func() {
		m.mu.Lock()
		m.bgmPlaying = false
		m.mu.Unlock()
	})))

	return nil
}

// StopBGM stops the current background music.
func (m *Manager) StopBGM() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stopBGMInternal()
}

func (m *Manager) stopBGMInternal() {
	if m.bgmCtrl != nil {
		m.bgmCtrl.Paused = true
	}
	speaker.Clear()
	// Re-add SFX mixer after clearing
	if m.initialized {
		speaker.Play(m.sfxMixer)
	}
	m.bgmPlaying = false
	if m.bgmStreamer != nil {
		m.bgmStreamer.Close()
		m.bgmStreamer = nil
	}
	m.bgmCtrl = nil
	m.bgmVolume = nil
	m.bgmPath = ""
}

// PauseBGM pauses the current background music.
func (m *Manager) PauseBGM() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.bgmCtrl != nil {
		m.bgmCtrl.Paused = true
		m.bgmPlaying = false
	}
}

// ResumeBGM resumes the paused background music.
func (m *Manager) ResumeBGM() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.bgmCtrl != nil {
		m.bgmCtrl.Paused = false
		m.bgmPlaying = true
	}
}

// IsBGMPlaying returns whether BGM is currently playing.
func (m *Manager) IsBGMPlaying() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.bgmPlaying
}

// GetBGMPath returns the path of the currently playing BGM.
func (m *Manager) GetBGMPath() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.bgmPath
}

// PlaySFX plays a sound effect from WAV data.
func (m *Manager) PlaySFX(data []byte) error {
	m.mu.RLock()
	initialized := m.initialized
	sfxVol := m.masterVolume * m.sfxVolLevel
	m.mu.RUnlock()

	if !initialized {
		return fmt.Errorf("audio not initialized")
	}

	// Decode WAV
	streamer, format, err := wav.Decode(io.NopCloser(bytes.NewReader(data)))
	if err != nil {
		return fmt.Errorf("decode wav: %w", err)
	}

	// Resample if needed
	var resampled beep.Streamer
	if format.SampleRate != m.sampleRate {
		resampled = beep.Resample(4, format.SampleRate, m.sampleRate, streamer)
	} else {
		resampled = streamer
	}

	// Apply volume
	volStreamer := &effects.Volume{
		Streamer: resampled,
		Base:     2,
		Volume:   volumeToDb(sfxVol),
		Silent:   sfxVol <= 0,
	}

	// Add to mixer (concurrent playback)
	m.sfxMixer.Add(volStreamer)

	return nil
}

// loopStreamer wraps a streamer to make it loop.
type loopStreamer struct {
	streamer  beep.StreamSeekCloser
	resampled beep.Streamer
	loop      bool
}

func (l *loopStreamer) Stream(samples [][2]float64) (n int, ok bool) {
	filled := 0
	for filled < len(samples) {
		n, ok := l.resampled.Stream(samples[filled:])
		filled += n
		if !ok {
			if l.loop {
				// Reset to beginning
				if err := l.streamer.Seek(0); err != nil {
					return filled, false
				}
				continue
			}
			return filled, false
		}
	}
	return filled, true
}

func (l *loopStreamer) Err() error {
	return l.streamer.Err()
}
