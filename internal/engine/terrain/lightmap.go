package terrain

import (
	"github.com/Faultbox/midgard-ro/pkg/formats"
)

// BuildLightmapAtlas creates a lightmap atlas from GND lightmap data.
// Returns atlas data ready for GPU upload.
func BuildLightmapAtlas(gnd *formats.GND) *LightmapAtlas {
	if len(gnd.Lightmaps) == 0 {
		// Create a simple white lightmap if none exist
		return &LightmapAtlas{
			Data:        createWhiteLightmap(8),
			Size:        8,
			TilesPerRow: 1,
			TileWidth:   8,
			TileHeight:  8,
		}
	}

	// Calculate atlas size (square, power of 2)
	lmWidth := int(gnd.LightmapWidth)
	lmHeight := int(gnd.LightmapHeight)
	if lmWidth == 0 {
		lmWidth = 8
	}
	if lmHeight == 0 {
		lmHeight = 8
	}

	// Calculate how many lightmaps fit per row
	numLightmaps := len(gnd.Lightmaps)
	tilesPerRow := 1
	for tilesPerRow*tilesPerRow < numLightmaps {
		tilesPerRow *= 2
	}

	atlasSize := tilesPerRow * lmWidth
	// Round up to next power of 2
	pow2 := 64
	for pow2 < atlasSize {
		pow2 *= 2
	}
	atlasSize = pow2
	if atlasSize > 4096 {
		atlasSize = 4096
	}

	tilesPerRowFinal := int32(atlasSize / lmWidth)

	// Create RGBA atlas (4 bytes per pixel)
	atlasData := make([]byte, atlasSize*atlasSize*4)

	// Fill with default (white color, full brightness)
	for i := 0; i < len(atlasData); i += 4 {
		atlasData[i] = 255   // R
		atlasData[i+1] = 255 // G
		atlasData[i+2] = 255 // B
		atlasData[i+3] = 255 // A (brightness/shadow)
	}

	// Copy each lightmap into the atlas
	for i, lm := range gnd.Lightmaps {
		tileX := i % int(tilesPerRowFinal)
		tileY := i / int(tilesPerRowFinal)

		baseX := tileX * lmWidth
		baseY := tileY * lmHeight

		// Copy lightmap pixels
		for y := range lmHeight {
			for x := range lmWidth {
				srcIdx := y*lmWidth + x
				dstX := baseX + x
				dstY := baseY + y

				if dstX >= atlasSize || dstY >= atlasSize {
					continue
				}

				dstIdx := (dstY*atlasSize + dstX) * 4

				// Get brightness (shadow intensity) for alpha channel
				var brightness uint8 = 255
				if srcIdx < len(lm.Brightness) {
					brightness = lm.Brightness[srcIdx]
				}

				// Get RGB color tint
				var r, g, b uint8 = 0, 0, 0
				if srcIdx*3+2 < len(lm.ColorRGB) {
					r = lm.ColorRGB[srcIdx*3]
					g = lm.ColorRGB[srcIdx*3+1]
					b = lm.ColorRGB[srcIdx*3+2]
				}

				// Store: RGB = color tint, A = shadow intensity
				atlasData[dstIdx] = r
				atlasData[dstIdx+1] = g
				atlasData[dstIdx+2] = b
				atlasData[dstIdx+3] = brightness
			}
		}
	}

	return &LightmapAtlas{
		Data:        atlasData,
		Size:        int32(atlasSize),
		TilesPerRow: tilesPerRowFinal,
		TileWidth:   lmWidth,
		TileHeight:  lmHeight,
	}
}

// CalculateLightmapUV returns UV coordinates for a lightmap in the atlas.
// cornerIdx: 0=BL, 1=BR, 2=TL, 3=TR
//
// Uses half-pixel insets to center UV sampling and avoid boundary bleeding.
func CalculateLightmapUV(atlas *LightmapAtlas, lightmapID int16, cornerIdx int) [2]float32 {
	if atlas == nil || lightmapID < 0 || atlas.TilesPerRow == 0 {
		return [2]float32{0.5, 0.5} // Center of first tile as fallback
	}

	// Position of lightmap tile in atlas
	tileX := int(lightmapID) % int(atlas.TilesPerRow)
	tileY := int(lightmapID) / int(atlas.TilesPerRow)

	// Calculate UV with half-pixel inset to avoid edge bleeding
	atlasSize := float32(atlas.Size)
	tileW := float32(atlas.TileWidth) / atlasSize
	tileH := float32(atlas.TileHeight) / atlasSize

	baseU := float32(tileX*atlas.TileWidth) / atlasSize
	baseV := float32(tileY*atlas.TileHeight) / atlasSize

	// Half-pixel inset
	halfPixelU := 0.5 / atlasSize
	halfPixelV := 0.5 / atlasSize
	innerU1 := baseU + halfPixelU
	innerU2 := baseU + tileW - halfPixelU
	innerV1 := baseV + halfPixelV
	innerV2 := baseV + tileH - halfPixelV

	// Corner UVs within the tile
	// GND UV order: [0]=BL, [1]=BR, [2]=TL, [3]=TR
	switch cornerIdx {
	case 0: // Bottom-left
		return [2]float32{innerU1, innerV2}
	case 1: // Bottom-right
		return [2]float32{innerU2, innerV2}
	case 2: // Top-left
		return [2]float32{innerU1, innerV1}
	case 3: // Top-right
		return [2]float32{innerU2, innerV1}
	}
	return [2]float32{0.5, 0.5}
}

// createWhiteLightmap creates a white RGBA lightmap of given size.
func createWhiteLightmap(size int) []byte {
	data := make([]byte, size*size*4)
	for i := 0; i < len(data); i++ {
		data[i] = 255
	}
	return data
}
