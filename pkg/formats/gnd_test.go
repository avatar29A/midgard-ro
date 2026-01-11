package formats

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// createTestGND creates a minimal valid GND file for testing.
func createTestGND(width, height uint32, textures []string) []byte {
	buf := new(bytes.Buffer)

	// Magic "GRGN"
	buf.WriteString("GRGN")

	// Version 1.7 (stored as major, minor)
	buf.WriteByte(1) // major
	buf.WriteByte(7) // minor

	// Dimensions and zoom
	binary.Write(buf, binary.LittleEndian, width)
	binary.Write(buf, binary.LittleEndian, height)
	binary.Write(buf, binary.LittleEndian, float32(10.0)) // zoom

	// Textures
	textureNameLen := uint32(80)
	binary.Write(buf, binary.LittleEndian, uint32(len(textures)))
	binary.Write(buf, binary.LittleEndian, textureNameLen)
	for _, tex := range textures {
		nameBytes := make([]byte, textureNameLen)
		copy(nameBytes, tex)
		buf.Write(nameBytes)
	}

	// Lightmaps (empty)
	binary.Write(buf, binary.LittleEndian, uint32(0)) // count
	binary.Write(buf, binary.LittleEndian, uint32(8)) // width
	binary.Write(buf, binary.LittleEndian, uint32(8)) // height
	binary.Write(buf, binary.LittleEndian, uint32(1)) // cells

	// Surfaces (empty)
	binary.Write(buf, binary.LittleEndian, uint32(0))

	// Tiles
	tileCount := width * height
	for i := uint32(0); i < tileCount; i++ {
		// 4 corner altitudes
		for j := 0; j < 4; j++ {
			binary.Write(buf, binary.LittleEndian, float32(0.0))
		}
		// 3 surface IDs
		binary.Write(buf, binary.LittleEndian, int32(-1)) // top
		binary.Write(buf, binary.LittleEndian, int32(-1)) // front
		binary.Write(buf, binary.LittleEndian, int32(-1)) // right
	}

	return buf.Bytes()
}

func TestParseGND_ValidFile(t *testing.T) {
	textures := []string{"texture1.bmp", "texture2.bmp"}
	data := createTestGND(4, 4, textures)

	gnd, err := ParseGND(data)
	if err != nil {
		t.Fatalf("ParseGND failed: %v", err)
	}

	if gnd.Version.Major != 1 || gnd.Version.Minor != 7 {
		t.Errorf("expected version 1.7, got %s", gnd.Version)
	}

	if gnd.Width != 4 {
		t.Errorf("expected width 4, got %d", gnd.Width)
	}

	if gnd.Height != 4 {
		t.Errorf("expected height 4, got %d", gnd.Height)
	}

	if gnd.Zoom != 10.0 {
		t.Errorf("expected zoom 10.0, got %f", gnd.Zoom)
	}

	if len(gnd.Textures) != 2 {
		t.Errorf("expected 2 textures, got %d", len(gnd.Textures))
	}

	if len(gnd.Tiles) != 16 {
		t.Errorf("expected 16 tiles, got %d", len(gnd.Tiles))
	}
}

func TestParseGND_TextureNames(t *testing.T) {
	textures := []string{"GROUND01.BMP", "WALL_STONE.BMP", "water\\blue.bmp"}
	data := createTestGND(2, 2, textures)

	gnd, err := ParseGND(data)
	if err != nil {
		t.Fatalf("ParseGND failed: %v", err)
	}

	if len(gnd.Textures) != 3 {
		t.Fatalf("expected 3 textures, got %d", len(gnd.Textures))
	}

	for i, expected := range textures {
		if gnd.Textures[i] != expected {
			t.Errorf("texture %d: expected %q, got %q", i, expected, gnd.Textures[i])
		}
	}
}

func TestParseGND_InvalidMagic(t *testing.T) {
	data := []byte("XXXX\x01\x07\x04\x00\x00\x00\x04\x00\x00\x00")

	_, err := ParseGND(data)
	if err == nil {
		t.Error("expected error for invalid magic")
	}
}

func TestParseGND_TruncatedData(t *testing.T) {
	_, err := ParseGND([]byte("GRGN"))
	if err == nil {
		t.Error("expected error for truncated data")
	}
}

func TestParseGND_UnsupportedVersion(t *testing.T) {
	buf := new(bytes.Buffer)
	buf.WriteString("GRGN")
	buf.WriteByte(2) // major = 2 (unsupported)
	buf.WriteByte(0) // minor
	binary.Write(buf, binary.LittleEndian, uint32(4))
	binary.Write(buf, binary.LittleEndian, uint32(4))
	binary.Write(buf, binary.LittleEndian, float32(10.0))

	_, err := ParseGND(buf.Bytes())
	if err == nil {
		t.Error("expected error for unsupported version")
	}
}

func TestGND_GetTile(t *testing.T) {
	data := createTestGND(4, 4, nil)
	gnd, _ := ParseGND(data)

	// Valid tile
	tile := gnd.GetTile(2, 3)
	if tile == nil {
		t.Error("GetTile(2, 3) returned nil for valid coordinates")
	}

	// Out of bounds
	if gnd.GetTile(-1, 0) != nil {
		t.Error("GetTile(-1, 0) should return nil")
	}
	if gnd.GetTile(0, -1) != nil {
		t.Error("GetTile(0, -1) should return nil")
	}
	if gnd.GetTile(4, 0) != nil {
		t.Error("GetTile(4, 0) should return nil")
	}
	if gnd.GetTile(0, 4) != nil {
		t.Error("GetTile(0, 4) should return nil")
	}
}

func TestGND_GetAltitudeRange(t *testing.T) {
	data := createTestGND(2, 2, nil)
	gnd, _ := ParseGND(data)

	// Manually set some altitudes
	gnd.Tiles[0].Altitude = [4]float32{-10, -5, 0, 5}
	gnd.Tiles[1].Altitude = [4]float32{10, 20, 30, 40}

	min, max := gnd.GetAltitudeRange()
	if min != -10 {
		t.Errorf("expected min -10, got %f", min)
	}
	if max != 40 {
		t.Errorf("expected max 40, got %f", max)
	}
}

func TestGND_GetAltitudeRange_Empty(t *testing.T) {
	gnd := &GND{}
	min, max := gnd.GetAltitudeRange()
	if min != 0 || max != 0 {
		t.Errorf("expected (0, 0) for empty GND, got (%f, %f)", min, max)
	}
}

func TestGNDVersion_String(t *testing.T) {
	tests := []struct {
		version  GNDVersion
		expected string
	}{
		{GNDVersion{1, 5}, "1.5"},
		{GNDVersion{1, 7}, "1.7"},
		{GNDVersion{1, 9}, "1.9"},
	}

	for _, tc := range tests {
		if tc.version.String() != tc.expected {
			t.Errorf("expected %q, got %q", tc.expected, tc.version.String())
		}
	}
}

func TestGND_CountSurfacesByTexture(t *testing.T) {
	gnd := &GND{
		Surfaces: []GNDSurface{
			{TextureID: 0},
			{TextureID: 0},
			{TextureID: 1},
			{TextureID: -1}, // no texture
			{TextureID: 0},
		},
	}

	counts := gnd.CountSurfacesByTexture()

	if counts[0] != 3 {
		t.Errorf("expected 3 surfaces with texture 0, got %d", counts[0])
	}
	if counts[1] != 1 {
		t.Errorf("expected 1 surface with texture 1, got %d", counts[1])
	}
	if _, exists := counts[-1]; exists {
		t.Error("should not count surfaces with no texture (-1)")
	}
}
