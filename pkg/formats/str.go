package formats

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
)

// STR format errors.
var (
	ErrInvalidSTRMagic       = errors.New("invalid STR magic: expected 'STRM'")
	ErrUnsupportedSTRVersion = errors.New("unsupported STR version")
	ErrTruncatedSTRData      = errors.New("truncated STR data")
)

// STR is a parsed effect animation — the format RO keeps its non-sprite
// effects in, from a level-up flash to a skill's magic circle.
//
// An effect is a stack of layers, each a quad with its own texture and its own
// list of key frames. The layers are drawn in order over the same origin, so
// what looks like one animation is a few dozen quads fading past each other.
//
// Nothing here is in world units. The coordinates are the screen offsets the
// original client draws at, around an origin of (320, 240) — the middle of its
// 640x480 window — so an effect is anchored by projecting its owner's position
// and drawing the quads around that point.
type STR struct {
	// FPS is the rate the key frame numbers are counted at, and MaxKey how
	// many frames the whole effect runs for.
	FPS    int
	MaxKey int

	Layers []STRLayer
}

// DurationMs is how long the effect runs, in milliseconds.
//
// The file's own figures rather than a number chosen here: an effect that is
// sequenced after another — a job level-up behind a base one — has to wait
// exactly as long as the first one lasts, and the first one knows.
func (s *STR) DurationMs() float32 {
	if s == nil || s.FPS <= 0 {
		return 0
	}

	return float32(s.MaxKey) * 1000 / float32(s.FPS)
}

// STRLayer is one quad of an effect, with the textures it may draw and the key
// frames that move it.
type STRLayer struct {
	// Textures are file names, relative to the effect texture directory. A
	// layer usually has one; the ones that have several flip between them
	// through the AnimFrame of their key frames.
	Textures []string

	Keys []STRKey
}

// Key frame kinds. A file alternates them: a basic frame states where the quad
// is, and the morph frame after it says how much to move it each frame until
// the next basic one.
const (
	STRKeyBasic = 0
	STRKeyMorph = 1
)

// STRKey is one key frame of a layer.
//
// The morph frames carry per-frame deltas in the same fields the basic frames
// carry absolute values in, which is why every field is kept rather than
// resolved on the way in: what a number means depends on the kind beside it.
type STRKey struct {
	// Frame is which frame of the effect this applies from, and Kind whether
	// it states a position or a change to one.
	Frame int
	Kind  int

	// X and Y are the quad's origin, in the original client's screen space.
	X, Y float32

	// U holds the texture coordinates as the file stores them: four u values
	// then four v values, one pair per corner.
	U [8]float32

	// XY holds the four corner offsets: four x values then four y values. A
	// rotated or sheared quad is stored this way rather than as a size and an
	// angle, so the corners are used as they come.
	XY [8]float32

	// AnimFrame is which of the layer's textures to draw, as a float because a
	// morph frame steps it fractionally.
	AnimFrame float32
	AnimType  int

	// Delay is the frame interval for the texture flip, and Angle the
	// rotation. The file stores the angle in 1024ths of a turn.
	Delay float32
	Angle float32

	// Color is RGBA, each 0..255.
	Color [4]float32

	// SrcAlpha and DestAlpha are Direct3D blend factors. Two is D3DBLEND_ONE,
	// which is what makes an effect add to what is behind it rather than cover
	// it, and is how nearly every RO effect is drawn.
	SrcAlpha  int
	DestAlpha int

	MTPreset int
}

// AngleDegrees is the rotation in degrees. The file counts a turn as 1024.
func (k STRKey) AngleDegrees() float32 {
	return k.Angle * 360 / 1024
}

// Additive reports whether this key draws by adding to what is behind it.
//
// D3DBLEND_ONE as the destination factor is the marker. It is worth asking
// rather than assuming: an effect drawn with ordinary alpha where it wanted
// additive looks like a grey rectangle over the scene.
func (k STRKey) Additive() bool {
	const d3dBlendOne = 2

	return k.DestAlpha == d3dBlendOne || k.DestAlpha == d3dBlendDestAlpha
}

// d3dBlendDestAlpha is D3DBLEND_DESTALPHA, which the effects in the archive
// use for the same purpose: drawn against a scene with no destination alpha to
// speak of, it reads as an add.
const d3dBlendDestAlpha = 7

// STR entry sizes.
const (
	strHeaderLen  = 36
	strTextureLen = 128
	strKeyLen     = 124
)

// ParseSTR reads an effect animation.
func ParseSTR(data []byte) (*STR, error) {
	if len(data) < strHeaderLen {
		return nil, ErrTruncatedSTRData
	}

	if string(data[:4]) != "STRM" {
		return nil, ErrInvalidSTRMagic
	}

	version := binary.LittleEndian.Uint32(data[4:])
	if version != 0x94 {
		return nil, fmt.Errorf("%w: %#x", ErrUnsupportedSTRVersion, version)
	}

	str := &STR{
		FPS:    int(binary.LittleEndian.Uint32(data[8:])),
		MaxKey: int(binary.LittleEndian.Uint32(data[12:])),
	}

	layers := int(binary.LittleEndian.Uint32(data[16:]))

	// Then sixteen reserved bytes, which the header length accounts for.
	off := strHeaderLen

	str.Layers = make([]STRLayer, 0, layers)
	for i := 0; i < layers; i++ {
		layer, next, err := parseSTRLayer(data, off)
		if err != nil {
			return nil, fmt.Errorf("layer %d: %w", i, err)
		}

		str.Layers = append(str.Layers, layer)
		off = next
	}

	return str, nil
}

// parseSTRLayer reads one layer and returns where the next one starts.
func parseSTRLayer(data []byte, off int) (STRLayer, int, error) {
	var layer STRLayer

	if off+4 > len(data) {
		return layer, 0, ErrTruncatedSTRData
	}

	textures := int(binary.LittleEndian.Uint32(data[off:]))
	off += 4

	if off+textures*strTextureLen > len(data) {
		return layer, 0, ErrTruncatedSTRData
	}

	layer.Textures = make([]string, 0, textures)
	for i := 0; i < textures; i++ {
		layer.Textures = append(layer.Textures, cString(data[off:off+strTextureLen]))
		off += strTextureLen
	}

	if off+4 > len(data) {
		return layer, 0, ErrTruncatedSTRData
	}

	keys := int(binary.LittleEndian.Uint32(data[off:]))
	off += 4

	if off+keys*strKeyLen > len(data) {
		return layer, 0, ErrTruncatedSTRData
	}

	layer.Keys = make([]STRKey, 0, keys)
	for i := 0; i < keys; i++ {
		layer.Keys = append(layer.Keys, parseSTRKey(data[off:off+strKeyLen]))
		off += strKeyLen
	}

	return layer, off, nil
}

// parseSTRKey reads one 124-byte key frame.
func parseSTRKey(entry []byte) STRKey {
	key := STRKey{
		Frame: int(int32(binary.LittleEndian.Uint32(entry[0:]))),
		Kind:  int(binary.LittleEndian.Uint32(entry[4:])),
		X:     f32(entry, 8),
		Y:     f32(entry, 12),
	}

	for i := range key.U {
		key.U[i] = f32(entry, 16+4*i)
	}
	for i := range key.XY {
		key.XY[i] = f32(entry, 48+4*i)
	}

	key.AnimFrame = f32(entry, 80)
	key.AnimType = int(binary.LittleEndian.Uint32(entry[84:]))
	key.Delay = f32(entry, 88)
	key.Angle = f32(entry, 92)

	for i := range key.Color {
		key.Color[i] = f32(entry, 96+4*i)
	}

	key.SrcAlpha = int(binary.LittleEndian.Uint32(entry[112:]))
	key.DestAlpha = int(binary.LittleEndian.Uint32(entry[116:]))
	key.MTPreset = int(binary.LittleEndian.Uint32(entry[120:]))

	return key
}

// f32 reads a little-endian float at an offset.
func f32(data []byte, off int) float32 {
	return math.Float32frombits(binary.LittleEndian.Uint32(data[off:]))
}

// cString reads a NUL-terminated name out of a fixed-width field.
func cString(field []byte) string {
	for i, b := range field {
		if b == 0 {
			return string(field[:i])
		}
	}

	return string(field)
}
