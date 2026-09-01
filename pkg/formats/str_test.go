package formats

import (
	"bytes"
	"encoding/binary"
	"errors"
	"math"
	"testing"
)

// strTestFPS is the rate every effect in the archive is authored at.
const strTestFPS = 60

// strBuilder writes a file in the layout ParseSTR reads, so a test can state
// exactly what is in one.
type strBuilder struct {
	buf bytes.Buffer
}

func newSTRBuilder(maxKey, layers int) *strBuilder {
	b := &strBuilder{}

	b.buf.WriteString("STRM")
	b.u32(0x94)
	b.u32(strTestFPS)
	b.u32(uint32(maxKey))
	b.u32(uint32(layers))
	b.buf.Write(make([]byte, 16)) // reserved

	return b
}

func (b *strBuilder) u32(v uint32) {
	_ = binary.Write(&b.buf, binary.LittleEndian, v)
}

func (b *strBuilder) f32(v float32) {
	b.u32(math.Float32bits(v))
}

func (b *strBuilder) layer(textures []string, keys []STRKey) {
	b.u32(uint32(len(textures)))

	for _, name := range textures {
		field := make([]byte, strTextureLen)
		copy(field, name)
		b.buf.Write(field)
	}

	b.u32(uint32(len(keys)))

	for _, k := range keys {
		b.u32(uint32(k.Frame))
		b.u32(uint32(k.Kind))
		b.f32(k.X)
		b.f32(k.Y)

		for _, v := range k.U {
			b.f32(v)
		}
		for _, v := range k.XY {
			b.f32(v)
		}

		b.f32(k.AnimFrame)
		b.u32(uint32(k.AnimType))
		b.f32(k.Delay)
		b.f32(k.Angle)

		for _, v := range k.Color {
			b.f32(v)
		}

		b.u32(uint32(k.SrcAlpha))
		b.u32(uint32(k.DestAlpha))
		b.u32(uint32(k.MTPreset))
	}
}

func (b *strBuilder) bytes() []byte { return b.buf.Bytes() }

// TestParseSTRReadsLayersAndKeys: the field order is what decides whether an
// effect plays at all, and getting it wrong reads plausible numbers out of the
// wrong places rather than failing.
func TestParseSTRReadsLayersAndKeys(t *testing.T) {
	b := newSTRBuilder(160, 1)
	b.layer([]string{"beam.bmp", "spark.bmp"}, []STRKey{
		{
			Frame: 10, Kind: STRKeyBasic,
			X: 320, Y: 81,
			U:         [8]float32{0, 0, 1, 1, 0, 0, 1, 1},
			XY:        [8]float32{-13, 11, 31, -32, 0, 0, 0, 0},
			AnimFrame: 1,
			Angle:     512,
			Color:     [4]float32{255, 128, 64, 32},
			SrcAlpha:  5, DestAlpha: 2,
		},
		{Frame: 10, Kind: STRKeyMorph, Y: 32.25},
	})

	str, err := ParseSTR(b.bytes())
	if err != nil {
		t.Fatalf("ParseSTR: %v", err)
	}

	if str.FPS != strTestFPS || str.MaxKey != 160 {
		t.Errorf("fps/maxKey = %d/%d, want 60/160", str.FPS, str.MaxKey)
	}
	if got := str.DurationMs(); got < 2666 || got > 2667 {
		t.Errorf("DurationMs = %v, want 160 frames at 60fps", got)
	}
	if len(str.Layers) != 1 {
		t.Fatalf("parsed %d layers, want 1", len(str.Layers))
	}

	layer := str.Layers[0]
	if len(layer.Textures) != 2 || layer.Textures[0] != "beam.bmp" {
		t.Errorf("textures = %v", layer.Textures)
	}
	if len(layer.Keys) != 2 {
		t.Fatalf("parsed %d keys, want 2", len(layer.Keys))
	}

	key := layer.Keys[0]
	if key.Frame != 10 || key.Kind != STRKeyBasic {
		t.Errorf("frame/kind = %d/%d, want 10/basic", key.Frame, key.Kind)
	}
	if key.X != 320 || key.Y != 81 {
		t.Errorf("pos = %v,%v want 320,81", key.X, key.Y)
	}
	if key.Color != [4]float32{255, 128, 64, 32} {
		t.Errorf("color = %v", key.Color)
	}
	if key.AnimFrame != 1 {
		t.Errorf("animFrame = %v, want 1", key.AnimFrame)
	}
	if got := key.AngleDegrees(); got != 180 {
		t.Errorf("AngleDegrees = %v, want 180 — the file counts a turn as 1024", got)
	}
	if !key.Additive() {
		t.Error("a key with D3DBLEND_ONE as its destination should draw additively")
	}

	if layer.Keys[1].Kind != STRKeyMorph || layer.Keys[1].Y != 32.25 {
		t.Errorf("morph key = %+v", layer.Keys[1])
	}
}

// TestParseSTRRejectsRubbish: a file that is not an effect, or is one the
// reader does not know, has to be refused rather than read as noise.
func TestParseSTRRejectsRubbish(t *testing.T) {
	if _, err := ParseSTR([]byte("nope")); !errors.Is(err, ErrTruncatedSTRData) {
		t.Errorf("a four-byte file gave %v", err)
	}

	short := newSTRBuilder(10, 0).bytes()
	copy(short, "XXXX")

	if _, err := ParseSTR(short); !errors.Is(err, ErrInvalidSTRMagic) {
		t.Errorf("bad magic gave %v", err)
	}

	wrongVersion := newSTRBuilder(10, 0).bytes()
	binary.LittleEndian.PutUint32(wrongVersion[4:], 0x95)

	if _, err := ParseSTR(wrongVersion); !errors.Is(err, ErrUnsupportedSTRVersion) {
		t.Errorf("unknown version gave %v", err)
	}
}

// TestParseSTRRejectsATruncatedLayer: a layer that claims more keys than the
// file holds must not be read past its end.
func TestParseSTRRejectsATruncatedLayer(t *testing.T) {
	b := newSTRBuilder(10, 1)
	b.layer(nil, []STRKey{{Frame: 1}})

	data := b.bytes()

	if _, err := ParseSTR(data[:len(data)-8]); !errors.Is(err, ErrTruncatedSTRData) {
		t.Errorf("a truncated layer gave %v", err)
	}
}
