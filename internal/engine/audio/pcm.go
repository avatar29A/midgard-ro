package audio

import (
	"encoding/binary"
	"fmt"

	"github.com/gopxl/beep/v2"
	"github.com/tosone/minimp3"
)

// bytesPerSample is the width of one channel of decoded PCM.
const bytesPerSample = 2

// pcmStreamer serves already decoded 16-bit PCM to beep.
//
// Holding the whole track keeps decoding off the audio callback, which only has
// a buffer's worth of time to work with, and makes seeking — how a track loops —
// an index assignment that cannot fail. A four minute track costs about 23 MB,
// and only one plays at a time.
type pcmStreamer struct {
	data     []byte
	channels int
	pos      int
}

func newPCMStreamer(data []byte, channels int) *pcmStreamer {
	if channels < 1 {
		channels = 1
	}

	frame := channels * bytesPerSample

	return &pcmStreamer{
		data:     data[:len(data)-len(data)%frame],
		channels: channels,
	}
}

func (s *pcmStreamer) Stream(samples [][2]float64) (int, bool) {
	frame := s.channels * bytesPerSample

	n := 0
	for n < len(samples) {
		offset := (s.pos + n) * frame
		if offset+frame > len(s.data) {
			break
		}

		left := sampleAt(s.data, offset)

		right := left
		if s.channels > 1 {
			right = sampleAt(s.data, offset+bytesPerSample)
		}

		samples[n] = [2]float64{left, right}
		n++
	}

	s.pos += n

	return n, n > 0
}

// sampleAt reads one little-endian 16-bit sample as a value in [-1, 1].
func sampleAt(data []byte, offset int) float64 {
	return float64(int16(binary.LittleEndian.Uint16(data[offset:]))) / 32768
}

func (s *pcmStreamer) Err() error { return nil }

func (s *pcmStreamer) Len() int {
	return len(s.data) / (s.channels * bytesPerSample)
}

func (s *pcmStreamer) Position() int { return s.pos }

func (s *pcmStreamer) Seek(p int) error {
	if p < 0 || p > s.Len() {
		return fmt.Errorf("seek to %d is outside the track's %d samples", p, s.Len())
	}

	s.pos = p

	return nil
}

func (s *pcmStreamer) Close() error { return nil }

// decodeMP3 decodes a whole MP3 into PCM.
//
// beep's own mp3 decoder is not used: it wraps go-mp3, which decodes MPEG-1
// correctly but gets MPEG-2 Layer III audibly wrong, and every Ragnarok
// background music file is MPEG-2 at 22.05 kHz. minimp3 decodes them to the
// sample.
func decodeMP3(data []byte) (beep.StreamSeekCloser, beep.Format, error) {
	decoded, pcm, err := minimp3.DecodeFull(data)
	if err != nil {
		return nil, beep.Format{}, fmt.Errorf("decoding mp3: %w", err)
	}

	if decoded.SampleRate <= 0 || decoded.Channels <= 0 {
		return nil, beep.Format{}, fmt.Errorf("mp3 reports %d Hz and %d channels",
			decoded.SampleRate, decoded.Channels)
	}

	format := beep.Format{
		SampleRate:  beep.SampleRate(decoded.SampleRate),
		NumChannels: decoded.Channels,
		Precision:   bytesPerSample,
	}

	return newPCMStreamer(pcm, decoded.Channels), format, nil
}
