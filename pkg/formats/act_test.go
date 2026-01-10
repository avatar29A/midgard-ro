package formats

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func TestParseACT_InvalidMagic(t *testing.T) {
	data := make([]byte, 20)
	copy(data, "XX") // Invalid magic
	_, err := ParseACT(data)
	if err != ErrInvalidACTMagic {
		t.Errorf("expected ErrInvalidACTMagic, got %v", err)
	}
}

func TestParseACT_TruncatedData(t *testing.T) {
	data := []byte("AC") // Too short
	_, err := ParseACT(data)
	if err != ErrTruncatedACTData {
		t.Errorf("expected ErrTruncatedACTData, got %v", err)
	}
}

func TestParseACT_UnsupportedVersion(t *testing.T) {
	data := make([]byte, 20)
	copy(data, "AC")
	data[2] = 0x99 // Invalid version
	data[3] = 0x01
	_, err := ParseACT(data)
	if err == nil {
		t.Error("expected error for unsupported version")
	}
}

func TestParseACT_Version200(t *testing.T) {
	act := buildSyntheticACT(0x200)

	parsed, err := ParseACT(act)
	if err != nil {
		t.Fatalf("failed to parse v0x200 ACT: %v", err)
	}

	if parsed.Version != 0x200 {
		t.Errorf("expected version 0x200, got 0x%X", parsed.Version)
	}

	if len(parsed.Actions) != 1 {
		t.Errorf("expected 1 action, got %d", len(parsed.Actions))
	}

	if len(parsed.Actions[0].Frames) != 2 {
		t.Errorf("expected 2 frames, got %d", len(parsed.Actions[0].Frames))
	}

	// v0x200 should have no events or intervals
	if len(parsed.Events) != 0 {
		t.Errorf("expected no events for v0x200, got %d", len(parsed.Events))
	}
	if len(parsed.Intervals) != 0 {
		t.Errorf("expected no intervals for v0x200, got %d", len(parsed.Intervals))
	}
}

func TestParseACT_Version205(t *testing.T) {
	act := buildSyntheticACT(0x205)

	parsed, err := ParseACT(act)
	if err != nil {
		t.Fatalf("failed to parse v0x205 ACT: %v", err)
	}

	if parsed.Version != 0x205 {
		t.Errorf("expected version 0x205, got 0x%X", parsed.Version)
	}

	// Check layer properties
	if len(parsed.Actions) > 0 && len(parsed.Actions[0].Frames) > 0 {
		frame := parsed.Actions[0].Frames[0]
		if len(frame.Layers) > 0 {
			layer := frame.Layers[0]
			if layer.SpriteID != 0 {
				t.Errorf("expected sprite ID 0, got %d", layer.SpriteID)
			}
			if layer.ScaleX != 1.0 {
				t.Errorf("expected scale X 1.0, got %f", layer.ScaleX)
			}
			if layer.ScaleY != 1.5 {
				t.Errorf("expected scale Y 1.5, got %f", layer.ScaleY)
			}
			if layer.Width != 32 {
				t.Errorf("expected width 32, got %d", layer.Width)
			}
			if layer.Height != 32 {
				t.Errorf("expected height 32, got %d", layer.Height)
			}
		}

		// Check anchor points
		if len(frame.AnchorPoints) != 1 {
			t.Errorf("expected 1 anchor point, got %d", len(frame.AnchorPoints))
		} else {
			anchor := frame.AnchorPoints[0]
			if anchor.X != 10 || anchor.Y != 20 {
				t.Errorf("expected anchor (10,20), got (%d,%d)", anchor.X, anchor.Y)
			}
		}
	}

	// Check events
	if len(parsed.Events) != 1 {
		t.Errorf("expected 1 event, got %d", len(parsed.Events))
	} else if parsed.Events[0] != "atk" {
		t.Errorf("expected event 'atk', got '%s'", parsed.Events[0])
	}

	// Check intervals
	if len(parsed.Intervals) != 1 {
		t.Errorf("expected 1 interval, got %d", len(parsed.Intervals))
	} else if parsed.Intervals[0] != 100.0 {
		t.Errorf("expected interval 100.0, got %f", parsed.Intervals[0])
	}
}

func TestLayer_IsMirrored(t *testing.T) {
	tests := []struct {
		flags    uint32
		expected bool
	}{
		{0, false},
		{1, true},
		{2, false},
		{3, true},
		{0xFFFFFFFF, true},
	}

	for _, tt := range tests {
		layer := Layer{Flags: tt.flags}
		if layer.IsMirrored() != tt.expected {
			t.Errorf("flags=%d: expected IsMirrored()=%v, got %v", tt.flags, tt.expected, layer.IsMirrored())
		}
	}
}

func TestACTVersion_String(t *testing.T) {
	tests := []struct {
		version  ACTVersion
		expected string
	}{
		{0x200, "2.0"},
		{0x201, "2.1"},
		{0x205, "2.5"},
	}

	for _, tt := range tests {
		if tt.version.String() != tt.expected {
			t.Errorf("version 0x%X: expected %s, got %s", tt.version, tt.expected, tt.version.String())
		}
	}
}

func TestParseACT_GeneratedFile(t *testing.T) {
	testFile := filepath.Join("testdata", "test.act")
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Skip("testdata/test.act not found, run: go run testdata/generate_act.go")
	}

	act, err := ParseACTFile(testFile)
	if err != nil {
		t.Fatalf("failed to parse test ACT file: %v", err)
	}

	// Verify basic structure
	if act.Version != 0x205 {
		t.Errorf("expected version 0x205, got 0x%X", act.Version)
	}

	if len(act.Actions) != 2 {
		t.Errorf("expected 2 actions, got %d", len(act.Actions))
	}

	// Action 0: idle (1 frame)
	if len(act.Actions[0].Frames) != 1 {
		t.Errorf("action 0: expected 1 frame, got %d", len(act.Actions[0].Frames))
	}

	// Action 1: walk (2 frames)
	if len(act.Actions) > 1 && len(act.Actions[1].Frames) != 2 {
		t.Errorf("action 1: expected 2 frames, got %d", len(act.Actions[1].Frames))
	}

	// Check events
	if len(act.Events) != 1 || act.Events[0] != "step.wav" {
		t.Errorf("expected event 'step.wav', got %v", act.Events)
	}

	// Check intervals
	if len(act.Intervals) < 2 {
		t.Errorf("expected 2 intervals, got %d", len(act.Intervals))
	}
}

// buildSyntheticACT creates a synthetic ACT file for testing.
func buildSyntheticACT(version uint16) []byte {
	var buf bytes.Buffer

	// Header (16 bytes)
	buf.WriteString("AC")
	buf.WriteByte(byte(version & 0xFF))                // minor
	buf.WriteByte(byte((version >> 8) & 0xFF))         // major
	binary.Write(&buf, binary.LittleEndian, uint16(1)) // 1 action
	buf.Write(make([]byte, 10))                        // reserved

	// Action 0: 2 frames
	binary.Write(&buf, binary.LittleEndian, uint32(2))

	// Frame 0
	writeFrame(&buf, version, 1, true)

	// Frame 1
	writeFrame(&buf, version, 1, false)

	// Events (v0x201+)
	if version >= 0x201 {
		binary.Write(&buf, binary.LittleEndian, int32(1)) // 1 event
		eventName := make([]byte, 40)
		copy(eventName, "atk")
		buf.Write(eventName)
	}

	// Intervals (v0x202+)
	if version >= 0x202 {
		binary.Write(&buf, binary.LittleEndian, float32(100.0))
	}

	return buf.Bytes()
}

// writeFrame writes a frame to the buffer.
func writeFrame(buf *bytes.Buffer, version uint16, layerCount int, withAnchor bool) {
	// Range1 + Range2 (32 bytes, unused)
	buf.Write(make([]byte, 32))

	// Layer count
	binary.Write(buf, binary.LittleEndian, uint32(layerCount))

	// Layers
	for i := 0; i < layerCount; i++ {
		writeLayer(buf, version, i)
	}

	// Event ID
	binary.Write(buf, binary.LittleEndian, int32(-1))

	// Anchor points (v0x203+)
	if version >= 0x203 {
		if withAnchor {
			binary.Write(buf, binary.LittleEndian, uint32(1)) // 1 anchor
			buf.Write(make([]byte, 4))                        // padding
			binary.Write(buf, binary.LittleEndian, int32(10)) // X
			binary.Write(buf, binary.LittleEndian, int32(20)) // Y
			binary.Write(buf, binary.LittleEndian, int32(0))  // attribute
		} else {
			binary.Write(buf, binary.LittleEndian, uint32(0)) // no anchors
		}
	}
}

// writeLayer writes a layer to the buffer.
func writeLayer(buf *bytes.Buffer, version uint16, spriteID int) {
	binary.Write(buf, binary.LittleEndian, int32(0))        // X
	binary.Write(buf, binary.LittleEndian, int32(0))        // Y
	binary.Write(buf, binary.LittleEndian, int32(spriteID)) // sprite ID
	binary.Write(buf, binary.LittleEndian, uint32(0))       // flags
	buf.Write([]byte{255, 255, 255, 255})                   // color RGBA
	binary.Write(buf, binary.LittleEndian, float32(1.0))    // scale X

	if version >= 0x204 {
		binary.Write(buf, binary.LittleEndian, float32(1.5)) // scale Y
	}

	binary.Write(buf, binary.LittleEndian, float32(0.0)) // rotation
	binary.Write(buf, binary.LittleEndian, int32(0))     // sprite type

	if version >= 0x205 {
		binary.Write(buf, binary.LittleEndian, int32(32)) // width
		binary.Write(buf, binary.LittleEndian, int32(32)) // height
	}
}
