package audio

import (
	"encoding/binary"
	"testing"

	"github.com/gopxl/beep/v2"
)

// testPCM builds interleaved stereo PCM with a recognizable ramp.
func testPCM(frames int) []byte {
	data := make([]byte, frames*2*bytesPerSample)
	for i := range frames {
		binary.LittleEndian.PutUint16(data[i*4:], uint16(int16(i)))
		binary.LittleEndian.PutUint16(data[i*4+2:], uint16(int16(-i)))
	}

	return data
}

func TestPCMStreamer(t *testing.T) {
	s := newPCMStreamer(testPCM(100), 2)

	if s.Len() != 100 {
		t.Fatalf("Len = %d, want 100", s.Len())
	}

	buf := make([][2]float64, 60)

	n, ok := s.Stream(buf)
	if n != 60 || !ok {
		t.Fatalf("first Stream = (%d, %v), want (60, true)", n, ok)
	}

	if s.Position() != 60 {
		t.Errorf("Position = %d, want 60", s.Position())
	}

	// The tail is short, and then the stream is done.
	n, ok = s.Stream(buf)
	if n != 40 || !ok {
		t.Fatalf("second Stream = (%d, %v), want (40, true)", n, ok)
	}

	if n, ok := s.Stream(buf); n != 0 || ok {
		t.Fatalf("drained Stream = (%d, %v), want (0, false)", n, ok)
	}

	if err := s.Seek(0); err != nil {
		t.Fatalf("Seek(0): %v", err)
	}

	if n, ok := s.Stream(buf); n != 60 || !ok {
		t.Fatalf("Stream after Seek = (%d, %v), want (60, true)", n, ok)
	}

	if err := s.Seek(-1); err == nil {
		t.Error("Seek(-1) succeeded, want an error")
	}

	if err := s.Seek(s.Len() + 1); err == nil {
		t.Error("Seek past the end succeeded, want an error")
	}
}

func TestPCMStreamerMono(t *testing.T) {
	// One channel has to reach both ears rather than being read as a stereo
	// pair, which would halve the length and swap channels every frame.
	data := make([]byte, 2*bytesPerSample)
	var loud int16 = 16384

	binary.LittleEndian.PutUint16(data[0:], uint16(loud))
	binary.LittleEndian.PutUint16(data[2:], uint16(-loud))

	s := newPCMStreamer(data, 1)
	if s.Len() != 2 {
		t.Fatalf("Len = %d, want 2", s.Len())
	}

	buf := make([][2]float64, 2)
	if n, _ := s.Stream(buf); n != 2 {
		t.Fatalf("Stream = %d, want 2", n)
	}

	if buf[0][0] != buf[0][1] || buf[1][0] != buf[1][1] {
		t.Errorf("mono did not reach both channels: %v", buf)
	}

	if buf[0][0] <= 0 || buf[1][0] >= 0 {
		t.Errorf("mono samples came out wrong: %v", buf)
	}
}

// Looping has to happen underneath the resampler. A resampler reports the end
// of its source once and stays ended, so looping above it used to leave the
// loop spinning forever — inside the lock the speaker holds while pulling
// samples, which froze the whole game.
func TestLoopedResampleKeepsProducing(t *testing.T) {
	const (
		sourceRate = beep.SampleRate(22050)
		deviceRate = beep.SampleRate(44100)
		frames     = 512 // far shorter than what we will ask for
	)

	source := newPCMStreamer(testPCM(frames), 2)

	looped, err := beep.Loop2(source)
	if err != nil {
		t.Fatalf("Loop2: %v", err)
	}

	chain := beep.Resample(resampleQuality, sourceRate, deviceRate, looped)

	buf := make([][2]float64, 256)

	want := frames * 8 // several times around the loop
	got := 0

	for range 1000 {
		n, ok := chain.Stream(buf)
		got += n

		if !ok {
			t.Fatalf("stream ended after %d samples; a looping track must not end", got)
		}

		if n == 0 {
			t.Fatalf("stream stalled at %d samples with no progress", got)
		}

		if got >= want {
			return
		}
	}

	t.Fatalf("only produced %d of %d samples before giving up", got, want)
}
