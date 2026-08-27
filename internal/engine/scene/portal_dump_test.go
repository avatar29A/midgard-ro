package scene

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"testing"
)

// TestDumpPortalTexture writes the generated warp texture, and the same
// texture blended over a light and a dark floor, so the pattern can be
// judged without running the client. Off unless PORTAL_DUMP names a file.
func TestDumpPortalTexture(t *testing.T) {
	out := os.Getenv("PORTAL_DUMP")
	if out == "" {
		t.Skip("set PORTAL_DUMP to write the texture")
	}

	const size = 256
	pix := portalPixels(size)

	floors := []color.RGBA{
		{200, 195, 185, 255}, // Prontera pavement
		{200, 195, 185, 255},
		{200, 195, 185, 255},
		{60, 58, 62, 255}, // dark stone, as in the reference capture
	}
	strengths := []float64{0.9, 0.7, 0.5, 0.9}
	img := image.NewRGBA(image.Rect(0, 0, size*len(floors), size))
	for f, floor := range floors {
		for y := 0; y < size; y++ {
			for x := 0; x < size; x++ {
				i := (y*size + x) * 4
				a := float64(pix[i+3]) / 255 * strengths[f]
				// Blended, as the renderer draws it.
				add := func(base, src byte) uint8 {
					return uint8(a*float64(src) + (1-a)*float64(base))
				}
				img.Set(f*size+x, y, color.RGBA{
					add(floor.R, pix[i]), add(floor.G, pix[i+1]), add(floor.B, pix[i+2]), 255,
				})
			}
		}
	}

	file, err := os.Create(out)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if err := png.Encode(file, img); err != nil {
		t.Fatal(err)
	}
}
