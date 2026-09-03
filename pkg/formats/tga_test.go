package formats

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Faultbox/midgard-ro/pkg/grf"
)

// tgaHeader builds one, with no id field and no color map.
func tgaHeader(imageType byte, width, height, bits int, descriptor byte) []byte {
	h := make([]byte, tgaHeaderLen)
	h[2] = imageType
	h[12], h[13] = byte(width), byte(width>>8)
	h[14], h[15] = byte(height), byte(height>>8)
	h[16] = byte(bits)
	h[17] = descriptor

	return h
}

// TestDecodeTGAChannelOrder: the format stores blue first, and a decoder that
// reads it the natural way round produces a picture rather than an error —
// every effect in the game in the wrong color.
func TestDecodeTGAChannelOrder(t *testing.T) {
	// One pixel, stored blue-green-red-alpha, top row first.
	data := append(tgaHeader(tgaTrueColor, 1, 1, 32, tgaTopToBottomBit),
		0x10, 0x20, 0x30, 0x40)

	img, err := DecodeTGA(data)
	if err != nil {
		t.Fatalf("decoding: %v", err)
	}

	got := img.At(0, 0).(color.NRGBA)
	if want := (color.NRGBA{R: 0x30, G: 0x20, B: 0x10, A: 0x40}); got != want {
		t.Errorf("pixel = %v, want %v", got, want)
	}
}

// TestDecodeTGARowOrder: a TGA's rows run bottom to top unless the descriptor
// says otherwise — the other way a wrong decoder produces a picture instead of
// an error, upside down.
func TestDecodeTGARowOrder(t *testing.T) {
	// Two rows of one pixel: first stored black, second white.
	pixels := []byte{0, 0, 0, 255, 255, 255, 255, 255}

	bottomUp, err := DecodeTGA(append(tgaHeader(tgaTrueColor, 1, 2, 32, 0), pixels...))
	if err != nil {
		t.Fatalf("decoding bottom-up: %v", err)
	}

	topDown, err := DecodeTGA(append(tgaHeader(tgaTrueColor, 1, 2, 32, tgaTopToBottomBit), pixels...))
	if err != nil {
		t.Fatalf("decoding top-down: %v", err)
	}

	// The first pixel stored is the bottom row of one and the top row of the
	// other, so the two images are each other flipped.
	if bottomUp.At(0, 1) != topDown.At(0, 0) || bottomUp.At(0, 0) != topDown.At(0, 1) {
		t.Errorf("the two row orders are not flips of each other: %v/%v and %v/%v",
			bottomUp.At(0, 0), bottomUp.At(0, 1), topDown.At(0, 0), topDown.At(0, 1))
	}

	if r, _, _, _ := bottomUp.At(0, 1).RGBA(); r != 0 {
		t.Errorf("the first pixel stored should be the bottom row, got %v at the bottom", bottomUp.At(0, 1))
	}
}

// TestDecodeTGARunLength: a run-length packet counts one less than it means,
// and the archive holds forty-one files that would be short by that much.
func TestDecodeTGARunLength(t *testing.T) {
	// One run of four red pixels, then two stored plainly.
	data := append(tgaHeader(tgaRLETrueColor, 6, 1, 32, tgaTopToBottomBit),
		0x83, 0x00, 0x00, 0xff, 0xff, // run of 4: blue=0 green=0 red=ff alpha=ff
		0x01, 0xff, 0x00, 0x00, 0xff, // two raw: blue
		0x00, 0xff, 0x00, 0xff,
	)

	img, err := DecodeTGA(data)
	if err != nil {
		t.Fatalf("decoding: %v", err)
	}

	for x := 0; x < 4; x++ {
		if got := img.At(x, 0).(color.NRGBA); got.R != 0xff || got.B != 0 {
			t.Errorf("pixel %d of the run = %v, want red", x, got)
		}
	}
	if got := img.At(4, 0).(color.NRGBA); got.B != 0xff {
		t.Errorf("first raw pixel = %v, want blue", got)
	}
	if got := img.At(5, 0).(color.NRGBA); got.G != 0xff {
		t.Errorf("second raw pixel = %v, want green", got)
	}
}

// TestDecodeTGAGrayscale: two hundred and sixty-five of the archive's are
// eight-bit grey.
func TestDecodeTGAGrayscale(t *testing.T) {
	data := append(tgaHeader(tgaGrayscale, 2, 1, 8, tgaTopToBottomBit), 0x00, 0x80)

	img, err := DecodeTGA(data)
	if err != nil {
		t.Fatalf("decoding: %v", err)
	}

	if got := img.At(1, 0).(color.NRGBA); got.R != 0x80 || got.G != 0x80 || got.B != 0x80 || got.A != 0xff {
		t.Errorf("grey pixel = %v, want an opaque half-grey", got)
	}
}

// TestDecodeTGARejectsRubbish: a truncated or unknown file says so rather than
// returning half a picture.
func TestDecodeTGARejectsRubbish(t *testing.T) {
	if _, err := DecodeTGA(nil); err == nil {
		t.Error("an empty file decoded")
	}
	if _, err := DecodeTGA(tgaHeader(tgaTrueColor, 4, 4, 32, 0)); err == nil {
		t.Error("a header with no pixels decoded")
	}
	if _, err := DecodeTGA(tgaHeader(99, 1, 1, 32, 0)); err == nil {
		t.Error("an unknown image type decoded")
	}
	if _, err := DecodeTGA(tgaHeader(tgaTrueColor, 0, 0, 32, 0)); err == nil {
		t.Error("an image of no size decoded")
	}
}

// TestDecodeImagePrefersTheStandardDecoders: a PNG is a PNG, and a file that
// is neither reports the standard decoder's complaint rather than a TGA one —
// a bad BMP called a bad TGA sends you looking in the wrong place.
func TestDecodeImagePrefersTheStandardDecoders(t *testing.T) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}

	if _, err := DecodeImage(buf.Bytes()); err != nil {
		t.Errorf("a PNG did not decode: %v", err)
	}

	_, err := DecodeImage([]byte("not a picture at all"))
	if err == nil {
		t.Fatal("rubbish decoded")
	}
	if strings.Contains(err.Error(), "TGA") {
		t.Errorf("rubbish is reported as a TGA fault: %v", err)
	}
}

// TestDecodeImageReadsTheArchivesTGAs: the ones the effects are built from,
// against the real files. Every one of these was unreadable before there was a
// decoder, which is why Cold Bolt landed and nothing appeared.
//
// Needs the client's GRFs:
//
//	MIDGARD_GRF=/path/to/data go test ./pkg/formats/
func TestDecodeImageReadsTheArchivesTGAs(t *testing.T) {
	dir := os.Getenv("MIDGARD_GRF")
	if dir == "" {
		t.Skip("set MIDGARD_GRF to the directory holding data.grf to run this")
	}

	archive, err := grf.Open(filepath.Join(dir, "data.grf"))
	if err != nil {
		t.Fatalf("opening data.grf: %v", err)
	}
	defer archive.Close()

	for _, tc := range []struct {
		name string
		w, h int
	}{
		{"lens1.tga", 32, 128},
		{"lens2.tga", 32, 128},
		{"smoke.tga", 64, 64},
		{"alpha_down.tga", 64, 64},
		{"alpha_center.tga", 32, 32},
	} {
		data, err := archive.Read(`data\texture\effect\` + tc.name)
		if err != nil {
			t.Errorf("%s is not in the archive: %v", tc.name, err)

			continue
		}

		img, err := DecodeImage(data)
		if err != nil {
			t.Errorf("%s did not decode: %v", tc.name, err)

			continue
		}

		if got := img.Bounds().Dx(); got != tc.w {
			t.Errorf("%s is %d wide, want %d", tc.name, got, tc.w)
		}
		if got := img.Bounds().Dy(); got != tc.h {
			t.Errorf("%s is %d tall, want %d", tc.name, got, tc.h)
		}

		// A texture that decoded to nothing but black would draw nothing,
		// which is the fault this is here to catch.
		var brightest uint32
		bounds := img.Bounds()
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				r, g, b, _ := img.At(x, y).RGBA()
				if v := max(r, max(g, b)); v > brightest {
					brightest = v
				}
			}
		}

		if brightest == 0 {
			t.Errorf("%s decoded to an entirely black image", tc.name)
		}
	}
}
