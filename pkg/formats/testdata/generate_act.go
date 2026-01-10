//go:build ignore

// This program generates a test ACT file for unit tests.
// Run with: go run generate_act.go
package main

import (
	"bytes"
	"encoding/binary"
	"os"
)

func main() {
	// Generate a v0x205 ACT file with 2 actions
	var buf bytes.Buffer

	// Header (16 bytes)
	buf.WriteString("AC")
	buf.WriteByte(0x05)                                    // minor
	buf.WriteByte(0x02)                                    // major (version 2.5)
	binary.Write(&buf, binary.LittleEndian, uint16(2))     // 2 actions
	buf.Write(make([]byte, 10))                            // reserved

	// Action 0: idle (1 frame, 1 layer)
	binary.Write(&buf, binary.LittleEndian, uint32(1)) // 1 frame
	writeFrame(&buf, 0, 0, true)

	// Action 1: walk (2 frames, 1 layer each)
	binary.Write(&buf, binary.LittleEndian, uint32(2)) // 2 frames
	writeFrame(&buf, 1, 1, false)                       // frame 0, sprite 1, event=1
	writeFrame(&buf, 2, -1, false)                      // frame 1, sprite 2, no event

	// Events (1 event)
	binary.Write(&buf, binary.LittleEndian, int32(1))
	eventName := make([]byte, 40)
	copy(eventName, "step.wav")
	buf.Write(eventName)

	// Intervals (1 per action)
	binary.Write(&buf, binary.LittleEndian, float32(200.0)) // idle: 200ms
	binary.Write(&buf, binary.LittleEndian, float32(100.0)) // walk: 100ms

	// Write to file
	if err := os.WriteFile("test.act", buf.Bytes(), 0644); err != nil {
		panic(err)
	}

	println("Generated test.act:", buf.Len(), "bytes")
	println("  - 2 actions (idle: 1 frame, walk: 2 frames)")
	println("  - 1 event (step.wav)")
	println("  - 2 intervals (200ms, 100ms)")
}

func writeFrame(buf *bytes.Buffer, spriteID, eventID int, withAnchor bool) {
	// Range1 + Range2 (32 bytes, unused)
	buf.Write(make([]byte, 32))

	// 1 layer
	binary.Write(buf, binary.LittleEndian, uint32(1))

	// Layer
	binary.Write(buf, binary.LittleEndian, int32(0))           // X
	binary.Write(buf, binary.LittleEndian, int32(0))           // Y
	binary.Write(buf, binary.LittleEndian, int32(spriteID))    // sprite ID
	binary.Write(buf, binary.LittleEndian, uint32(0))          // flags
	buf.Write([]byte{255, 255, 255, 255})                      // color RGBA
	binary.Write(buf, binary.LittleEndian, float32(1.0))       // scale X
	binary.Write(buf, binary.LittleEndian, float32(1.0))       // scale Y (v0x204+)
	binary.Write(buf, binary.LittleEndian, float32(0.0))       // rotation
	binary.Write(buf, binary.LittleEndian, int32(0))           // sprite type
	binary.Write(buf, binary.LittleEndian, int32(32))          // width (v0x205+)
	binary.Write(buf, binary.LittleEndian, int32(32))          // height (v0x205+)

	// Event ID
	binary.Write(buf, binary.LittleEndian, int32(eventID))

	// Anchor points (v0x203+)
	if withAnchor {
		binary.Write(buf, binary.LittleEndian, uint32(1))  // 1 anchor
		buf.Write(make([]byte, 4))                         // padding
		binary.Write(buf, binary.LittleEndian, int32(0))   // X
		binary.Write(buf, binary.LittleEndian, int32(-16)) // Y (head position)
		binary.Write(buf, binary.LittleEndian, int32(0))   // attribute
	} else {
		binary.Write(buf, binary.LittleEndian, uint32(0)) // no anchors
	}
}
