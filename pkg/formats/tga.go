package formats

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
)

// Targa images.
//
// Ten and a half thousand of the archive's effect textures are TGA and none of
// them could be read: Go's image package has no decoder for the format, and
// the loader's image.Decode answered "unknown format" for every one. An effect
// built out of them drew nothing at all, and silently — the skill fired, the
// quads were laid out, and the screen stayed empty.
//
// The format has no magic number to recognize it by, only a trailing signature
// that the older revision does not carry, so nothing here registers itself
// with the image package. DecodeImage tries the standard decoders first and
// falls back to this, which is unambiguous and costs nothing on a BMP.

// TGA errors.
var (
	ErrTruncatedTGAData = errors.New("truncated TGA data")
	ErrUnsupportedTGA   = errors.New("unsupported TGA")
)

// TGA image types. The archive holds four of them: true-color uncompressed is
// all but four hundred of its files, and the rest are grey masks, a handful of
// run-length ones, and a single color-mapped image nothing references.
const (
	tgaColorMapped    = 1
	tgaTrueColor      = 2
	tgaGrayscale      = 3
	tgaRLETrueColor   = 10
	tgaRLEGrayscale   = 11
	tgaHeaderLen      = 18
	tgaTopToBottomBit = 0x20
	tgaRightToLeftBit = 0x10
)

// DecodeImage reads an image the archive holds, whatever format it is in.
//
// The standard decoders first, then TGA. That order rather than sniffing:
// a TGA has no header to recognize it by — the signature that would is at the
// end of the file and only the later revision writes it — so guessing from the
// front would mistake other formats for it.
func DecodeImage(data []byte) (image.Image, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err == nil {
		return img, nil
	}

	tga, tgaErr := DecodeTGA(data)
	if tgaErr == nil {
		return tga, nil
	}

	// The first error, which is the one for the format the file most likely
	// is. A BMP that will not decode should not be reported as a bad TGA.
	return nil, err
}

// DecodeTGA reads a Targa image.
func DecodeTGA(data []byte) (image.Image, error) {
	if len(data) < tgaHeaderLen {
		return nil, ErrTruncatedTGAData
	}

	var (
		idLength     = int(data[0])
		colorMapType = data[1]
		imageType    = data[2]
		mapLength    = int(binary.LittleEndian.Uint16(data[5:]))
		mapEntryBits = int(data[7])
		width        = int(binary.LittleEndian.Uint16(data[12:]))
		height       = int(binary.LittleEndian.Uint16(data[14:]))
		pixelBits    = int(data[16])
		descriptor   = data[17]
	)

	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("%w: %dx%d", ErrUnsupportedTGA, width, height)
	}

	off := tgaHeaderLen + idLength

	var palette []byte
	if colorMapType == 1 {
		entryBytes := mapEntryBits / 8
		size := mapLength * entryBytes

		if off+size > len(data) {
			return nil, ErrTruncatedTGAData
		}

		palette = data[off : off+size]
		off += size
	}

	pixels := data[min(off, len(data)):]

	img := image.NewNRGBA(image.Rect(0, 0, width, height))

	var err error
	switch imageType {
	case tgaTrueColor, tgaGrayscale:
		err = tgaReadRaw(img, pixels, width, height, pixelBits, descriptor)
	case tgaRLETrueColor, tgaRLEGrayscale:
		err = tgaReadRLE(img, pixels, width, height, pixelBits, descriptor)
	case tgaColorMapped:
		err = tgaReadMapped(img, pixels, palette, width, height, pixelBits, mapEntryBits, descriptor)
	default:
		return nil, fmt.Errorf("%w: image type %d", ErrUnsupportedTGA, imageType)
	}

	if err != nil {
		return nil, err
	}

	return img, nil
}

// tgaPut writes one pixel, putting it where the descriptor says it goes.
//
// A TGA's rows run bottom to top unless it says otherwise, which is the one
// thing about the format that silently produces a picture rather than an
// error: read the natural way round, every image is upside down.
func tgaPut(img *image.NRGBA, i, width, height int, descriptor byte, r, g, b, a uint8) {
	x, y := i%width, i/width

	if descriptor&tgaTopToBottomBit == 0 {
		y = height - 1 - y
	}
	if descriptor&tgaRightToLeftBit != 0 {
		x = width - 1 - x
	}

	if y < 0 || y >= height {
		return
	}

	o := img.PixOffset(x, y)
	img.Pix[o], img.Pix[o+1], img.Pix[o+2], img.Pix[o+3] = r, g, b, a
}

// tgaPixel reads one pixel of the given depth. The channels are stored blue
// first, which is the other thing about the format that produces a picture
// rather than an error — read the natural way round, everything is the wrong
// color.
func tgaPixel(p []byte, bits int) (r, g, b, a uint8, ok bool) {
	switch bits {
	case 32:
		if len(p) < 4 {
			return 0, 0, 0, 0, false
		}

		return p[2], p[1], p[0], p[3], true

	case 24:
		if len(p) < 3 {
			return 0, 0, 0, 0, false
		}

		return p[2], p[1], p[0], 255, true

	case 16, 15:
		if len(p) < 2 {
			return 0, 0, 0, 0, false
		}

		// Five bits a channel, scaled to eight by repeating the top bits.
		v := binary.LittleEndian.Uint16(p)
		r = uint8((v>>10)&0x1f) << 3
		g = uint8((v>>5)&0x1f) << 3
		b = uint8(v&0x1f) << 3
		a = 255

		if bits == 16 && v&0x8000 == 0 {
			a = 0
		}

		return r | r>>5, g | g>>5, b | b>>5, a, true

	case 8:
		if len(p) < 1 {
			return 0, 0, 0, 0, false
		}

		return p[0], p[0], p[0], 255, true
	}

	return 0, 0, 0, 0, false
}

// tgaBytesPerPixel is how many bytes one pixel of the given depth takes.
func tgaBytesPerPixel(bits int) int {
	switch bits {
	case 32:
		return 4
	case 24:
		return 3
	case 16, 15:
		return 2
	case 8:
		return 1
	}

	return 0
}

// tgaReadRaw reads an uncompressed image.
func tgaReadRaw(img *image.NRGBA, pixels []byte, width, height, bits int, descriptor byte) error {
	step := tgaBytesPerPixel(bits)
	if step == 0 {
		return fmt.Errorf("%w: %d bits a pixel", ErrUnsupportedTGA, bits)
	}

	if len(pixels) < width*height*step {
		return ErrTruncatedTGAData
	}

	for i := 0; i < width*height; i++ {
		r, g, b, a, ok := tgaPixel(pixels[i*step:], bits)
		if !ok {
			return ErrTruncatedTGAData
		}

		tgaPut(img, i, width, height, descriptor, r, g, b, a)
	}

	return nil
}

// tgaReadRLE reads a run-length encoded image.
//
// Packets of a byte and then pixels: with the top bit set the byte counts a
// run of one repeated pixel, and without it a stretch of pixels stored one
// after another. Either way the count is one less than it means.
func tgaReadRLE(img *image.NRGBA, pixels []byte, width, height, bits int, descriptor byte) error {
	step := tgaBytesPerPixel(bits)
	if step == 0 {
		return fmt.Errorf("%w: %d bits a pixel", ErrUnsupportedTGA, bits)
	}

	total := width * height
	at, read := 0, 0

	for at < total {
		if read >= len(pixels) {
			return ErrTruncatedTGAData
		}

		packet := pixels[read]
		read++

		count := int(packet&0x7f) + 1
		if count > total-at {
			count = total - at
		}

		if packet&0x80 != 0 {
			r, g, b, a, ok := tgaPixel(pixels[min(read, len(pixels)):], bits)
			if !ok {
				return ErrTruncatedTGAData
			}
			read += step

			for n := 0; n < count; n++ {
				tgaPut(img, at, width, height, descriptor, r, g, b, a)
				at++
			}

			continue
		}

		for n := 0; n < count; n++ {
			r, g, b, a, ok := tgaPixel(pixels[min(read, len(pixels)):], bits)
			if !ok {
				return ErrTruncatedTGAData
			}
			read += step

			tgaPut(img, at, width, height, descriptor, r, g, b, a)
			at++
		}
	}

	return nil
}

// tgaReadMapped reads a color-mapped image: each byte is an index into the
// palette that came before the pixels.
func tgaReadMapped(img *image.NRGBA, pixels, palette []byte, width, height, bits, entryBits int, descriptor byte) error {
	if bits != 8 {
		return fmt.Errorf("%w: %d-bit color-mapped", ErrUnsupportedTGA, bits)
	}

	entry := tgaBytesPerPixel(entryBits)
	if entry == 0 {
		return fmt.Errorf("%w: %d-bit palette entries", ErrUnsupportedTGA, entryBits)
	}

	if len(pixels) < width*height {
		return ErrTruncatedTGAData
	}

	for i := 0; i < width*height; i++ {
		at := int(pixels[i]) * entry
		if at+entry > len(palette) {
			return ErrTruncatedTGAData
		}

		r, g, b, a, ok := tgaPixel(palette[at:], entryBits)
		if !ok {
			return ErrTruncatedTGAData
		}

		tgaPut(img, i, width, height, descriptor, r, g, b, a)
	}

	return nil
}
