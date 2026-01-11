// Package formats provides parsers for Ragnarok Online file formats.
package formats

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
)

// GND format errors.
var (
	ErrInvalidGNDMagic       = errors.New("invalid GND magic: expected 'GRGN'")
	ErrUnsupportedGNDVersion = errors.New("unsupported GND version")
	ErrTruncatedGNDData      = errors.New("truncated GND data")
)

// GNDVersion represents the GND file version.
type GNDVersion struct {
	Major uint8
	Minor uint8
}

// String returns the version as "Major.Minor".
func (v GNDVersion) String() string {
	return fmt.Sprintf("%d.%d", v.Major, v.Minor)
}

// GNDSurface represents a textured surface with UV coordinates.
type GNDSurface struct {
	U          [4]float32 // Texture U coordinates for 4 corners
	V          [4]float32 // Texture V coordinates for 4 corners
	TextureID  int16      // -1 = no texture
	LightmapID int16
	Color      [4]uint8 // BGRA vertex color
}

// GNDTile represents a single tile in the ground mesh.
type GNDTile struct {
	Altitude     [4]float32 // Corner heights (bottom-left, bottom-right, top-left, top-right)
	TopSurface   int32      // Surface ID for top face (-1 = none)
	FrontSurface int32      // Surface ID for front face (-1 = none)
	RightSurface int32      // Surface ID for right face (-1 = none)
}

// GNDLightmap represents lightmap data for a surface.
type GNDLightmap struct {
	Brightness []uint8 // Grayscale brightness values
	ColorRGB   []uint8 // RGB color values
}

// GND represents a parsed Ground file.
type GND struct {
	Version        GNDVersion
	Width          uint32
	Height         uint32
	Zoom           float32
	Textures       []string
	Lightmaps      []GNDLightmap
	LightmapWidth  uint32
	LightmapHeight uint32
	Surfaces       []GNDSurface
	Tiles          []GNDTile
}

// GetTile returns the tile at the given coordinates.
// Returns nil if coordinates are out of bounds.
func (g *GND) GetTile(x, y int) *GNDTile {
	if x < 0 || y < 0 || x >= int(g.Width) || y >= int(g.Height) {
		return nil
	}
	return &g.Tiles[y*int(g.Width)+x]
}

// GetAltitudeRange returns the minimum and maximum altitude in the ground mesh.
func (g *GND) GetAltitudeRange() (min, max float32) {
	if len(g.Tiles) == 0 {
		return 0, 0
	}

	min = g.Tiles[0].Altitude[0]
	max = g.Tiles[0].Altitude[0]

	for _, tile := range g.Tiles {
		for _, h := range tile.Altitude {
			if h < min {
				min = h
			}
			if h > max {
				max = h
			}
		}
	}

	return min, max
}

// ParseGND parses a GND file from raw bytes.
func ParseGND(data []byte) (*GND, error) {
	if len(data) < 18 {
		return nil, ErrTruncatedGNDData
	}

	// Check magic "GRGN"
	if string(data[0:4]) != "GRGN" {
		return nil, ErrInvalidGNDMagic
	}

	// Version is stored as [major, minor]
	version := GNDVersion{
		Major: data[4],
		Minor: data[5],
	}

	// Supported versions: 1.5 - 1.9
	if version.Major != 1 || version.Minor < 5 || version.Minor > 9 {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedGNDVersion, version)
	}

	r := bytes.NewReader(data[6:])

	// Read dimensions and zoom
	var width, height uint32
	var zoom float32

	if err := binary.Read(r, binary.LittleEndian, &width); err != nil {
		return nil, fmt.Errorf("%w: reading width", ErrTruncatedGNDData)
	}
	if err := binary.Read(r, binary.LittleEndian, &height); err != nil {
		return nil, fmt.Errorf("%w: reading height", ErrTruncatedGNDData)
	}
	if err := binary.Read(r, binary.LittleEndian, &zoom); err != nil {
		return nil, fmt.Errorf("%w: reading zoom", ErrTruncatedGNDData)
	}

	// Validate dimensions
	if width == 0 || height == 0 || width > 1024 || height > 1024 {
		return nil, fmt.Errorf("invalid GND dimensions: %dx%d", width, height)
	}

	gnd := &GND{
		Version: version,
		Width:   width,
		Height:  height,
		Zoom:    zoom,
	}

	// Read textures
	var textureCount, textureNameLen uint32
	if err := binary.Read(r, binary.LittleEndian, &textureCount); err != nil {
		return nil, fmt.Errorf("%w: reading texture count", ErrTruncatedGNDData)
	}
	if err := binary.Read(r, binary.LittleEndian, &textureNameLen); err != nil {
		return nil, fmt.Errorf("%w: reading texture name length", ErrTruncatedGNDData)
	}

	gnd.Textures = make([]string, textureCount)
	for i := uint32(0); i < textureCount; i++ {
		nameBytes := make([]byte, textureNameLen)
		if _, err := r.Read(nameBytes); err != nil {
			return nil, fmt.Errorf("%w: reading texture %d name", ErrTruncatedGNDData, i)
		}
		// Find first null byte and extract name
		if idx := bytes.IndexByte(nameBytes, 0); idx >= 0 {
			gnd.Textures[i] = string(nameBytes[:idx])
		} else {
			gnd.Textures[i] = string(nameBytes)
		}
	}

	// Read lightmaps
	var lightmapCount, lightmapWidth, lightmapHeight, lightmapCells uint32
	if err := binary.Read(r, binary.LittleEndian, &lightmapCount); err != nil {
		return nil, fmt.Errorf("%w: reading lightmap count", ErrTruncatedGNDData)
	}
	if err := binary.Read(r, binary.LittleEndian, &lightmapWidth); err != nil {
		return nil, fmt.Errorf("%w: reading lightmap width", ErrTruncatedGNDData)
	}
	if err := binary.Read(r, binary.LittleEndian, &lightmapHeight); err != nil {
		return nil, fmt.Errorf("%w: reading lightmap height", ErrTruncatedGNDData)
	}
	if err := binary.Read(r, binary.LittleEndian, &lightmapCells); err != nil {
		return nil, fmt.Errorf("%w: reading lightmap cells", ErrTruncatedGNDData)
	}

	gnd.LightmapWidth = lightmapWidth
	gnd.LightmapHeight = lightmapHeight

	pixelCount := lightmapWidth * lightmapHeight * lightmapCells
	gnd.Lightmaps = make([]GNDLightmap, lightmapCount)
	for i := uint32(0); i < lightmapCount; i++ {
		gnd.Lightmaps[i].Brightness = make([]byte, pixelCount)
		if _, err := r.Read(gnd.Lightmaps[i].Brightness); err != nil {
			return nil, fmt.Errorf("%w: reading lightmap %d brightness", ErrTruncatedGNDData, i)
		}
		gnd.Lightmaps[i].ColorRGB = make([]byte, pixelCount*3)
		if _, err := r.Read(gnd.Lightmaps[i].ColorRGB); err != nil {
			return nil, fmt.Errorf("%w: reading lightmap %d color", ErrTruncatedGNDData, i)
		}
	}

	// Read surfaces
	var surfaceCount uint32
	if err := binary.Read(r, binary.LittleEndian, &surfaceCount); err != nil {
		return nil, fmt.Errorf("%w: reading surface count", ErrTruncatedGNDData)
	}

	gnd.Surfaces = make([]GNDSurface, surfaceCount)
	for i := uint32(0); i < surfaceCount; i++ {
		surface, err := parseGNDSurface(r)
		if err != nil {
			return nil, fmt.Errorf("parsing surface %d: %w", i, err)
		}
		gnd.Surfaces[i] = surface
	}

	// Read tiles
	tileCount := width * height
	gnd.Tiles = make([]GNDTile, tileCount)
	for i := uint32(0); i < tileCount; i++ {
		tile, err := parseGNDTile(r)
		if err != nil {
			return nil, fmt.Errorf("parsing tile %d: %w", i, err)
		}
		gnd.Tiles[i] = tile
	}

	return gnd, nil
}

// parseGNDSurface parses a single GND surface.
func parseGNDSurface(r *bytes.Reader) (GNDSurface, error) {
	var surface GNDSurface

	// Read UV coordinates
	for i := 0; i < 4; i++ {
		if err := binary.Read(r, binary.LittleEndian, &surface.U[i]); err != nil {
			return GNDSurface{}, fmt.Errorf("%w: reading U[%d]", ErrTruncatedGNDData, i)
		}
	}
	for i := 0; i < 4; i++ {
		if err := binary.Read(r, binary.LittleEndian, &surface.V[i]); err != nil {
			return GNDSurface{}, fmt.Errorf("%w: reading V[%d]", ErrTruncatedGNDData, i)
		}
	}

	// Read texture and lightmap IDs
	if err := binary.Read(r, binary.LittleEndian, &surface.TextureID); err != nil {
		return GNDSurface{}, fmt.Errorf("%w: reading texture ID", ErrTruncatedGNDData)
	}
	if err := binary.Read(r, binary.LittleEndian, &surface.LightmapID); err != nil {
		return GNDSurface{}, fmt.Errorf("%w: reading lightmap ID", ErrTruncatedGNDData)
	}

	// Read vertex color (BGRA)
	if _, err := r.Read(surface.Color[:]); err != nil {
		return GNDSurface{}, fmt.Errorf("%w: reading color", ErrTruncatedGNDData)
	}

	return surface, nil
}

// parseGNDTile parses a single GND tile.
func parseGNDTile(r *bytes.Reader) (GNDTile, error) {
	var tile GNDTile

	// Read corner altitudes
	for i := 0; i < 4; i++ {
		if err := binary.Read(r, binary.LittleEndian, &tile.Altitude[i]); err != nil {
			return GNDTile{}, fmt.Errorf("%w: reading altitude[%d]", ErrTruncatedGNDData, i)
		}
	}

	// Read surface IDs
	if err := binary.Read(r, binary.LittleEndian, &tile.TopSurface); err != nil {
		return GNDTile{}, fmt.Errorf("%w: reading top surface", ErrTruncatedGNDData)
	}
	if err := binary.Read(r, binary.LittleEndian, &tile.FrontSurface); err != nil {
		return GNDTile{}, fmt.Errorf("%w: reading front surface", ErrTruncatedGNDData)
	}
	if err := binary.Read(r, binary.LittleEndian, &tile.RightSurface); err != nil {
		return GNDTile{}, fmt.Errorf("%w: reading right surface", ErrTruncatedGNDData)
	}

	return tile, nil
}

// ParseGNDFile parses a GND file from disk.
func ParseGNDFile(path string) (*GND, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading GND file: %w", err)
	}
	return ParseGND(data)
}

// CountSurfacesByTexture returns the count of surfaces using each texture.
func (g *GND) CountSurfacesByTexture() map[int]int {
	counts := make(map[int]int)
	for _, surface := range g.Surfaces {
		if surface.TextureID >= 0 {
			counts[int(surface.TextureID)]++
		}
	}
	return counts
}
