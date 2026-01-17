package sprite

// GenerateProceduralPlayer creates a simple humanoid sprite texture.
// Returns RGBA pixel data for a basic colored player marker.
// width and height are the texture dimensions.
func GenerateProceduralPlayer(width, height int) []byte {
	pixels := make([]byte, width*height*4)

	// Fill with humanoid shape in blue tones
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			idx := (y*width + x) * 4
			// Create a simple humanoid shape
			centerX := width / 2
			distFromCenter := x - centerX
			if distFromCenter < 0 {
				distFromCenter = -distFromCenter
			}

			// Head (top 1/4)
			if y < height/4 && distFromCenter < width/4 {
				pixels[idx+0] = 100 // R
				pixels[idx+1] = 150 // G
				pixels[idx+2] = 255 // B
				pixels[idx+3] = 255 // A
			} else if y >= height/4 && y < height*3/4 && distFromCenter < width/3 {
				// Body (middle half)
				pixels[idx+0] = 50  // R
				pixels[idx+1] = 100 // G
				pixels[idx+2] = 200 // B
				pixels[idx+3] = 255 // A
			} else if y >= height*3/4 && distFromCenter < width/4 {
				// Legs (bottom quarter)
				pixels[idx+0] = 50  // R
				pixels[idx+1] = 80  // G
				pixels[idx+2] = 150 // B
				pixels[idx+3] = 255 // A
			}
			// else: transparent (pixels remain 0)
		}
	}

	return pixels
}

// GenerateBillboardQuadVertices creates vertex data for a billboard sprite quad.
// Returns vertices in format: [x, y, u, v] for each of 4 corners.
// The quad is positioned from (0,0) to (1,1) in local space.
func GenerateBillboardQuadVertices() []float32 {
	return []float32{
		// Position (x, y)  TexCoord (u, v)
		-0.5, 1.0, 0.0, 0.0,
		0.5, 1.0, 1.0, 0.0,
		-0.5, 0.0, 0.0, 1.0,
		0.5, 0.0, 1.0, 1.0,
	}
}

// DefaultProceduralWidth is the default width for procedural player sprites.
const DefaultProceduralWidth = 32

// DefaultProceduralHeight is the default height for procedural player sprites.
const DefaultProceduralHeight = 64

// DefaultProceduralScale is the default scale for procedural player sprites.
const DefaultProceduralScale = 0.4

// GenerateCircularShadow creates a circular shadow texture with soft falloff.
// Returns RGBA pixel data suitable for GPU upload.
// size is the texture dimensions (size x size pixels).
// maxOpacity is the maximum alpha at the center (0.0-1.0, typically 0.25).
func GenerateCircularShadow(size int, maxOpacity float32) []byte {
	pixels := make([]byte, size*size*4)

	center := float32(size) / 2
	radius := float32(size)/2 - 1

	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			idx := (y*size + x) * 4
			// Calculate distance from center (circle equation)
			dx := (float32(x) - center) / radius
			dy := (float32(y) - center) / radius
			dist := dx*dx + dy*dy

			if dist <= 1.0 {
				// Inside circle - shadow with soft falloff from center
				alpha := (1.0 - dist) * maxOpacity
				pixels[idx+0] = 0 // R
				pixels[idx+1] = 0 // G
				pixels[idx+2] = 0 // B
				pixels[idx+3] = byte(alpha * 255)
			}
			// Outside circle: pixels remain 0 (fully transparent)
		}
	}

	return pixels
}

// GenerateShadowQuadVertices creates vertex data for a shadow quad on the XZ plane.
// Returns vertices in format: [x, z, u, v] for each of 4 corners.
// shadowSize is half the width/height in world units.
func GenerateShadowQuadVertices(shadowSize float32) []float32 {
	return []float32{
		// Position (x, z)  TexCoord (u, v)
		-shadowSize, -shadowSize, 0.0, 0.0,
		shadowSize, -shadowSize, 1.0, 0.0,
		-shadowSize, shadowSize, 0.0, 1.0,
		shadowSize, shadowSize, 1.0, 1.0,
	}
}

// DefaultShadowSize is the default shadow texture size in pixels.
const DefaultShadowSize = 24

// DefaultShadowOpacity is the default maximum opacity for shadows.
const DefaultShadowOpacity = 0.25

// DefaultShadowWorldSize is the default shadow size in world units.
const DefaultShadowWorldSize = 4.0
