package formats

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
)

// ACT format errors.
var (
	ErrInvalidACTMagic       = errors.New("invalid ACT magic: expected 'AC'")
	ErrUnsupportedACTVersion = errors.New("unsupported ACT version")
	ErrTruncatedACTData      = errors.New("truncated ACT data")
)

// ACTVersion represents the ACT file version.
type ACTVersion uint16

// String returns the version as "Major.Minor".
func (v ACTVersion) String() string {
	major := v >> 8
	minor := v & 0xFF
	return fmt.Sprintf("%d.%d", major, minor)
}

// ACT represents a parsed animation file.
type ACT struct {
	Version   ACTVersion
	Actions   []Action
	Events    []string  // Event names (sound files, "atk", etc.)
	Intervals []float32 // Animation delay per action (ms)
}

// Action represents an animation sequence.
type Action struct {
	Frames []Frame
}

// Frame represents a single animation frame.
type Frame struct {
	Layers       []Layer
	EventID      int32 // -1 = no event
	AnchorPoints []AnchorPoint
}

// Layer represents a sprite layer in a frame.
type Layer struct {
	X          int32
	Y          int32
	SpriteID   int32    // -1 = invalid/skip
	Flags      uint32   // bit 0 = mirror Y
	Color      [4]uint8 // RGBA tint
	ScaleX     float32
	ScaleY     float32
	Rotation   float32 // degrees
	SpriteType int32   // 0=indexed, 1=RGBA
	Width      int32   // v0x205+
	Height     int32   // v0x205+
}

// IsMirrored returns true if the layer should be Y-mirrored.
func (l *Layer) IsMirrored() bool {
	return l.Flags&1 != 0
}

// AnchorPoint represents an attachment point for equipment.
type AnchorPoint struct {
	X         int32
	Y         int32
	Attribute int32
}

// ParseACT parses an ACT file from raw bytes.
func ParseACT(data []byte) (*ACT, error) {
	if len(data) < 16 {
		return nil, ErrTruncatedACTData
	}

	// Check magic "AC"
	if data[0] != 'A' || data[1] != 'C' {
		return nil, ErrInvalidACTMagic
	}

	// Version is stored as Minor, Major (reversed)
	version := ACTVersion(uint16(data[3])<<8 | uint16(data[2]))

	// Check supported versions (0x200 - 0x205)
	if version < 0x200 || version > 0x205 {
		return nil, fmt.Errorf("%w: 0x%X", ErrUnsupportedACTVersion, version)
	}

	// Action count
	actionCount := binary.LittleEndian.Uint16(data[4:6])

	r := bytes.NewReader(data[16:]) // Skip header

	act := &ACT{
		Version: version,
		Actions: make([]Action, 0, actionCount),
	}

	// Parse actions
	for i := uint16(0); i < actionCount; i++ {
		action, err := parseAction(r, version)
		if err != nil {
			return nil, fmt.Errorf("parsing action %d: %w", i, err)
		}
		act.Actions = append(act.Actions, action)
	}

	// Parse events (v0x201+)
	if version >= 0x201 {
		var eventCount int32
		if err := binary.Read(r, binary.LittleEndian, &eventCount); err != nil {
			// No events at end of file is OK
			eventCount = 0
		}

		for i := int32(0); i < eventCount; i++ {
			name, err := parseEventName(r)
			if err != nil {
				return nil, fmt.Errorf("parsing event %d: %w", i, err)
			}
			act.Events = append(act.Events, name)
		}
	}

	// Parse intervals (v0x202+)
	if version >= 0x202 {
		act.Intervals = make([]float32, actionCount)
		for i := uint16(0); i < actionCount; i++ {
			var interval float32
			if readErr := binary.Read(r, binary.LittleEndian, &interval); readErr != nil {
				// Missing intervals at EOF is OK, use defaults (0)
				break
			}
			act.Intervals[i] = interval
		}
	}

	return act, nil
}

// ParseACTFile parses an ACT file from disk.
func ParseACTFile(path string) (*ACT, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading ACT file: %w", err)
	}
	return ParseACT(data)
}

// parseAction parses a single action.
func parseAction(r *bytes.Reader, version ACTVersion) (Action, error) {
	var frameCount uint32
	if err := binary.Read(r, binary.LittleEndian, &frameCount); err != nil {
		return Action{}, fmt.Errorf("%w: reading frame count", ErrTruncatedACTData)
	}

	action := Action{
		Frames: make([]Frame, 0, frameCount),
	}

	for i := uint32(0); i < frameCount; i++ {
		frame, err := parseFrame(r, version)
		if err != nil {
			return Action{}, fmt.Errorf("parsing frame %d: %w", i, err)
		}
		action.Frames = append(action.Frames, frame)
	}

	return action, nil
}

// parseFrame parses a single frame.
func parseFrame(r *bytes.Reader, version ACTVersion) (Frame, error) {
	// Skip Range1 and Range2 (unused, 16 bytes each)
	if _, err := r.Seek(32, io.SeekCurrent); err != nil {
		return Frame{}, fmt.Errorf("%w: skipping ranges", ErrTruncatedACTData)
	}

	var layerCount uint32
	if err := binary.Read(r, binary.LittleEndian, &layerCount); err != nil {
		return Frame{}, fmt.Errorf("%w: reading layer count", ErrTruncatedACTData)
	}

	frame := Frame{
		Layers:  make([]Layer, 0, layerCount),
		EventID: -1,
	}

	// Parse layers
	for i := uint32(0); i < layerCount; i++ {
		layer, err := parseLayer(r, version)
		if err != nil {
			return Frame{}, fmt.Errorf("parsing layer %d: %w", i, err)
		}
		frame.Layers = append(frame.Layers, layer)
	}

	// Event ID (v0x200+, always present)
	if err := binary.Read(r, binary.LittleEndian, &frame.EventID); err != nil {
		return Frame{}, fmt.Errorf("%w: reading event ID", ErrTruncatedACTData)
	}

	// Anchor points (v0x203+)
	if version >= 0x203 {
		var anchorCount uint32
		if err := binary.Read(r, binary.LittleEndian, &anchorCount); err != nil {
			return Frame{}, fmt.Errorf("%w: reading anchor count", ErrTruncatedACTData)
		}

		for i := uint32(0); i < anchorCount; i++ {
			anchor, err := parseAnchorPoint(r)
			if err != nil {
				return Frame{}, fmt.Errorf("parsing anchor %d: %w", i, err)
			}
			frame.AnchorPoints = append(frame.AnchorPoints, anchor)
		}
	}

	return frame, nil
}

// parseLayer parses a single layer.
func parseLayer(r *bytes.Reader, version ACTVersion) (Layer, error) {
	layer := Layer{
		Color: [4]uint8{255, 255, 255, 255}, // Default white tint
	}

	// X, Y position
	if err := binary.Read(r, binary.LittleEndian, &layer.X); err != nil {
		return Layer{}, fmt.Errorf("%w: reading X", ErrTruncatedACTData)
	}
	if err := binary.Read(r, binary.LittleEndian, &layer.Y); err != nil {
		return Layer{}, fmt.Errorf("%w: reading Y", ErrTruncatedACTData)
	}

	// Sprite index
	if err := binary.Read(r, binary.LittleEndian, &layer.SpriteID); err != nil {
		return Layer{}, fmt.Errorf("%w: reading sprite ID", ErrTruncatedACTData)
	}

	// Flags
	if err := binary.Read(r, binary.LittleEndian, &layer.Flags); err != nil {
		return Layer{}, fmt.Errorf("%w: reading flags", ErrTruncatedACTData)
	}

	// Color RGBA
	if _, err := io.ReadFull(r, layer.Color[:]); err != nil {
		return Layer{}, fmt.Errorf("%w: reading color", ErrTruncatedACTData)
	}

	// X Scale
	if err := binary.Read(r, binary.LittleEndian, &layer.ScaleX); err != nil {
		return Layer{}, fmt.Errorf("%w: reading X scale", ErrTruncatedACTData)
	}

	// Handle NaN/Inf scale values
	if math.IsNaN(float64(layer.ScaleX)) || math.IsInf(float64(layer.ScaleX), 0) {
		layer.ScaleX = 1.0
	}

	// Y Scale (v0x204+) or same as X
	if version >= 0x204 {
		if err := binary.Read(r, binary.LittleEndian, &layer.ScaleY); err != nil {
			return Layer{}, fmt.Errorf("%w: reading Y scale", ErrTruncatedACTData)
		}
		if math.IsNaN(float64(layer.ScaleY)) || math.IsInf(float64(layer.ScaleY), 0) {
			layer.ScaleY = 1.0
		}
	} else {
		layer.ScaleY = layer.ScaleX
	}

	// Rotation
	if err := binary.Read(r, binary.LittleEndian, &layer.Rotation); err != nil {
		return Layer{}, fmt.Errorf("%w: reading rotation", ErrTruncatedACTData)
	}

	// Sprite type
	if err := binary.Read(r, binary.LittleEndian, &layer.SpriteType); err != nil {
		return Layer{}, fmt.Errorf("%w: reading sprite type", ErrTruncatedACTData)
	}

	// Width/Height (v0x205+)
	if version >= 0x205 {
		if err := binary.Read(r, binary.LittleEndian, &layer.Width); err != nil {
			return Layer{}, fmt.Errorf("%w: reading width", ErrTruncatedACTData)
		}
		if err := binary.Read(r, binary.LittleEndian, &layer.Height); err != nil {
			return Layer{}, fmt.Errorf("%w: reading height", ErrTruncatedACTData)
		}
	}

	return layer, nil
}

// parseAnchorPoint parses a single anchor point.
func parseAnchorPoint(r *bytes.Reader) (AnchorPoint, error) {
	// Skip unknown/padding (4 bytes)
	if _, err := r.Seek(4, io.SeekCurrent); err != nil {
		return AnchorPoint{}, fmt.Errorf("%w: skipping anchor padding", ErrTruncatedACTData)
	}

	var anchor AnchorPoint
	if err := binary.Read(r, binary.LittleEndian, &anchor.X); err != nil {
		return AnchorPoint{}, fmt.Errorf("%w: reading anchor X", ErrTruncatedACTData)
	}
	if err := binary.Read(r, binary.LittleEndian, &anchor.Y); err != nil {
		return AnchorPoint{}, fmt.Errorf("%w: reading anchor Y", ErrTruncatedACTData)
	}
	if err := binary.Read(r, binary.LittleEndian, &anchor.Attribute); err != nil {
		return AnchorPoint{}, fmt.Errorf("%w: reading anchor attribute", ErrTruncatedACTData)
	}

	return anchor, nil
}

// parseEventName reads a 40-byte null-terminated event name.
func parseEventName(r *bytes.Reader) (string, error) {
	buf := make([]byte, 40)
	if _, err := io.ReadFull(r, buf); err != nil {
		return "", fmt.Errorf("%w: reading event name", ErrTruncatedACTData)
	}

	// Find null terminator
	end := bytes.IndexByte(buf, 0)
	if end == -1 {
		end = 40
	}

	return string(buf[:end]), nil
}
