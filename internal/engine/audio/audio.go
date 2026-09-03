// Package audio provides audio playback for background music and sound effects.
package audio

import (
	"bytes"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gopxl/beep/v2"
	"github.com/gopxl/beep/v2/effects"
	"github.com/gopxl/beep/v2/speaker"
	"github.com/gopxl/beep/v2/wav"
)

// DefaultSampleRate is the default sample rate for audio playback.
const DefaultSampleRate = beep.SampleRate(44100)

// bufferDuration is how much audio the speaker keeps queued.
//
// The work behind it is cheap — decoding and resampling a track runs at well
// over 100x realtime — so the buffer is not sized for throughput. It is sized
// to survive the pauses between refills: the game loop shares this process, and
// a GC cycle or a scheduler delay longer than the buffer starves the device and
// crackles. A 33ms buffer was not enough slack next to map rendering.
const bufferDuration = time.Second / 10

// resampleQuality is beep's sinc window size. RO audio is 22.05 kHz and the
// device runs at 44.1 kHz, so everything goes through the resampler.
const resampleQuality = 4

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
	bgmPath     string

	// bgmPlaying is atomic because the speaker sets it from its own thread,
	// which must never take m.mu — see the lock order in PlayBGM.
	bgmPlaying atomic.Bool

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
	err := speaker.Init(m.sampleRate, m.sampleRate.N(bufferDuration))
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
	m.StopBGM()

	speaker.Clear()

	m.mu.Lock()
	m.initialized = false
	m.mu.Unlock()
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
	if m.bgmVolume == nil {
		return
	}

	vol := m.masterVolume * m.bgmVolLevel
	if vol <= 0 {
		m.bgmVolume.Silent = true

		return
	}

	m.bgmVolume.Silent = false
	m.bgmVolume.Volume = volumeExponent(vol)
}

// volumeExponent converts a 0-1 volume to the number effects.Volume wants.
//
// That field is an exponent, not decibels: the gain is Base**Volume, so with
// Base 2 it is the base-2 logarithm of the gain we are asking for. Feeding it
// decibels instead makes everything roughly 30 dB too quiet — 0.56 lands on a
// gain of 0.03 rather than 0.56.
func volumeExponent(vol float64) float64 {
	if vol <= 0 {
		return -100 // effectively silent, though callers set Silent instead
	}

	return math.Log2(vol)
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

// PlayBGMFile plays background music from a file on disk, picking the decoder
// from its extension. Background music is not part of the GRF archives:
// Ragnarok clients ship it as .mp3 files in a BGM folder next to them.
// If loop is true, the music will loop indefinitely.
func (m *Manager) PlayBGMFile(path string, loop bool) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading background music %s: %w", path, err)
	}

	return m.PlayBGM(data, path, loop)
}

// PlayBGM plays background music from encoded audio data. The path selects the
// decoder by its extension and is reported by GetBGMPath; .mp3 and .wav are
// supported.
// If loop is true, the music will loop indefinitely.
// Lock order: this method may hold m.mu and then reach for the speaker, never
// the other way round. The speaker holds its own lock for the whole time it is
// pulling samples, so anything in the streamer chain that grabbed m.mu from
// that thread would deadlock against a caller doing the reverse. Nothing in the
// chain touches m.mu, which is what makes this safe.
func (m *Manager) PlayBGM(data []byte, path string, loop bool) error {
	m.mu.RLock()
	initialized, sampleRate := m.initialized, m.sampleRate
	m.mu.RUnlock()

	if !initialized {
		return fmt.Errorf("audio not initialized")
	}

	// Decoding is the slow part, so it happens before anything is locked or
	// stopped: a failure here leaves the current track playing.
	streamer, format, err := decode(data, path)
	if err != nil {
		return err
	}

	// Loop before resampling. A resampler reports the end of its source once
	// and stays ended, so looping underneath it would leave the resampler
	// spinning at the loop point — inside the speaker's lock.
	var source beep.Streamer = streamer

	if loop {
		looped, err := beep.Loop2(streamer)
		if err != nil {
			return fmt.Errorf("looping %s: %w", path, err)
		}

		source = looped
	}

	if format.SampleRate != sampleRate {
		source = beep.Resample(resampleQuality, format.SampleRate, sampleRate, source)
	}

	ctrl := &beep.Ctrl{Streamer: source}
	volume := &effects.Volume{Streamer: ctrl, Base: 2}

	m.StopBGM()

	m.mu.Lock()
	m.bgmStreamer = streamer
	m.bgmCtrl = ctrl
	m.bgmVolume = volume
	m.bgmPath = path
	m.updateBGMVolume()
	m.mu.Unlock()

	m.bgmPlaying.Store(true)

	speaker.Play(volume)

	return nil
}

// StopBGM stops the current background music.
func (m *Manager) StopBGM() {
	m.mu.Lock()
	streamer := m.bgmStreamer
	sfxMixer := m.sfxMixer
	initialized := m.initialized
	m.bgmStreamer = nil
	m.bgmCtrl = nil
	m.bgmVolume = nil
	m.bgmPath = ""
	m.mu.Unlock()

	m.bgmPlaying.Store(false)

	// Clearing takes the sound effect mixer with it, so it goes back after.
	speaker.Clear()

	if initialized {
		speaker.Play(sfxMixer)
	}

	if streamer != nil {
		_ = streamer.Close()
	}
}

// PauseBGM pauses the current background music.
func (m *Manager) PauseBGM() {
	m.setPaused(true)
}

// ResumeBGM resumes the paused background music.
func (m *Manager) ResumeBGM() {
	m.setPaused(false)
}

// setPaused flips the control the speaker reads, which is why the speaker's own
// lock is taken for the write.
func (m *Manager) setPaused(paused bool) {
	m.mu.RLock()
	ctrl := m.bgmCtrl
	m.mu.RUnlock()

	if ctrl == nil {
		return
	}

	speaker.Lock()
	ctrl.Paused = paused
	speaker.Unlock()

	m.bgmPlaying.Store(!paused)
}

// IsBGMPlaying returns whether BGM is currently playing.
func (m *Manager) IsBGMPlaying() bool {
	return m.bgmPlaying.Load()
}

// GetBGMPath returns the path of the currently playing BGM.
func (m *Manager) GetBGMPath() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.bgmPath
}

// PlaySFX plays a sound effect from WAV data, at whatever the effects volume
// is set to.
func (m *Manager) PlaySFX(data []byte) error {
	return m.PlaySFXAt(data, 1)
}

// PlaySFXAt plays one quieter than that, for a sound that comes from
// somewhere: a monster walking at the far edge of the screen should not be as
// loud as one at your feet. Gain is a fraction of the effects volume, and one
// is as loud as PlaySFX.
func (m *Manager) PlaySFXAt(data []byte, gain float64) error {
	if gain <= 0 {
		return nil
	}

	m.mu.RLock()
	initialized := m.initialized
	sfxVol := m.masterVolume * m.sfxVolLevel * clamp(gain, 0, 1)
	m.mu.RUnlock()

	if !initialized {
		return fmt.Errorf("audio not initialized")
	}

	streamer, format, err := decode(data, ".wav")
	if err != nil {
		return err
	}

	// Resample if needed
	var resampled beep.Streamer
	if format.SampleRate != m.sampleRate {
		resampled = beep.Resample(resampleQuality, format.SampleRate, m.sampleRate, streamer)
	} else {
		resampled = streamer
	}

	// Apply volume to the effect
	volStreamer := &effects.Volume{
		Streamer: resampled,
		Base:     2,
		Volume:   volumeExponent(sfxVol),
		Silent:   sfxVol <= 0,
	}

	// beep.Mixer is not synchronized, and the speaker is ranging over these
	// same streamers on its own thread, so the mixer may only be touched under
	// the speaker's lock.
	speaker.Lock()
	m.sfxMixer.Add(volStreamer)
	speaker.Unlock()

	return nil
}

// readSeekNopCloser gives a seekable reader the Close the decoders ask for.
//
// io.NopCloser would do the same job but hides Seek, and go-mp3 only indexes
// the frames when its source is an io.Seeker. Without that index Len reports 0
// and seeking back to the start — which is how a track loops — divides by a
// zero frame size and panics.
type readSeekNopCloser struct {
	*bytes.Reader
}

func (readSeekNopCloser) Close() error { return nil }

// decode turns encoded audio into a seekable stream, choosing the decoder from
// the path's extension. Looping needs the stream to be seekable, which is why
// both decoders return a StreamSeekCloser.
func decode(data []byte, path string) (beep.StreamSeekCloser, beep.Format, error) {
	reader := readSeekNopCloser{bytes.NewReader(data)}

	switch ext := strings.ToLower(filepath.Ext(path)); ext {
	case ".mp3":
		streamer, format, err := decodeMP3(data)
		if err != nil {
			return nil, beep.Format{}, fmt.Errorf("%s: %w", path, err)
		}

		return streamer, format, nil

	case ".wav", "":
		streamer, format, err := wav.Decode(reader)
		if err != nil {
			return nil, beep.Format{}, fmt.Errorf("decode wav %s: %w", path, err)
		}

		return streamer, format, nil

	default:
		return nil, beep.Format{}, fmt.Errorf("unsupported audio format %q for %s", ext, path)
	}
}
