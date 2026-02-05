package audio

import (
	"testing"
)

func TestVolumeConversion(t *testing.T) {
	// Test volume to dB conversion
	tests := []struct {
		vol float64
		min float64
		max float64
	}{
		{1.0, -1, 1},      // Full volume should be ~0dB
		{0.5, -8, -4},     // Half volume should be around -6dB
		{0.25, -14, -10},  // Quarter volume should be around -12dB
		{0.0, -200, -90},  // Zero volume should be very negative
	}

	for _, tt := range tests {
		db := volumeToDb(tt.vol)
		if db < tt.min || db > tt.max {
			t.Errorf("volumeToDb(%f) = %f, want between %f and %f", tt.vol, db, tt.min, tt.max)
		}
	}
}

func TestClamp(t *testing.T) {
	tests := []struct {
		v, min, max, want float64
	}{
		{0.5, 0, 1, 0.5},
		{-1, 0, 1, 0},
		{2, 0, 1, 1},
		{0, 0, 1, 0},
		{1, 0, 1, 1},
	}

	for _, tt := range tests {
		got := clamp(tt.v, tt.min, tt.max)
		if got != tt.want {
			t.Errorf("clamp(%f, %f, %f) = %f, want %f", tt.v, tt.min, tt.max, got, tt.want)
		}
	}
}

func TestNewManager(t *testing.T) {
	m := New()
	if m == nil {
		t.Fatal("New() returned nil")
	}

	// Check default volumes
	if m.GetMasterVolume() != 1.0 {
		t.Errorf("default master volume = %f, want 1.0", m.GetMasterVolume())
	}
	if m.GetBGMVolume() != 0.7 {
		t.Errorf("default BGM volume = %f, want 0.7", m.GetBGMVolume())
	}
	if m.GetSFXVolume() != 1.0 {
		t.Errorf("default SFX volume = %f, want 1.0", m.GetSFXVolume())
	}
}

func TestSetVolume(t *testing.T) {
	m := New()

	m.SetMasterVolume(0.5)
	if m.GetMasterVolume() != 0.5 {
		t.Errorf("master volume = %f, want 0.5", m.GetMasterVolume())
	}

	// Test clamping
	m.SetMasterVolume(2.0)
	if m.GetMasterVolume() != 1.0 {
		t.Errorf("master volume = %f, want 1.0 (clamped)", m.GetMasterVolume())
	}

	m.SetMasterVolume(-1.0)
	if m.GetMasterVolume() != 0.0 {
		t.Errorf("master volume = %f, want 0.0 (clamped)", m.GetMasterVolume())
	}
}
