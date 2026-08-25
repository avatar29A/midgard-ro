package audio

import (
	"math"
	"testing"
)

// effects.Volume computes the gain as Base**Volume, so what we hand it has to
// come back out as the volume that was asked for. Passing decibels instead —
// which is what this used to do — turns 0.56 into a gain of 0.03.
func TestVolumeExponentRoundTrips(t *testing.T) {
	for _, vol := range []float64{1.0, 0.8, 0.7, 0.56, 0.5, 0.25, 0.01} {
		gain := math.Pow(2, volumeExponent(vol))
		if math.Abs(gain-vol) > 1e-9 {
			t.Errorf("volumeExponent(%v) gives gain %v, want %v", vol, gain, vol)
		}
	}

	// Silence has no logarithm; callers set Silent, but the value must stay
	// finite and far below audible.
	if got := volumeExponent(0); got > -50 || math.IsInf(got, 0) {
		t.Errorf("volumeExponent(0) = %v, want a finite value well below -50", got)
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
