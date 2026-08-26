package packets

import "testing"

// TestPosRoundTrip walks every coordinate a map can hold. RO packs x and y
// into 10 bits each, so a shift that is wrong by one still produces a
// plausible in-range coordinate — the entity just stands somewhere else.
// Sweeping the whole range is what makes that visible.
func TestPosRoundTrip(t *testing.T) {
	for x := 0; x < 1024; x += 7 {
		for y := 0; y < 1024; y += 13 {
			for dir := 0; dir < 8; dir++ {
				gotX, gotY, gotDir := DecodePos(EncodePos(x, y, dir))
				if gotX != x || gotY != y || gotDir != dir {
					t.Fatalf("DecodePos(EncodePos(%d,%d,%d)) = (%d,%d,%d)",
						x, y, dir, gotX, gotY, gotDir)
				}
			}
		}
	}
}

func TestPos2RoundTrip(t *testing.T) {
	for x0 := 0; x0 < 1024; x0 += 61 {
		for y0 := 0; y0 < 1024; y0 += 67 {
			for x1 := 0; x1 < 1024; x1 += 71 {
				for y1 := 0; y1 < 1024; y1 += 73 {
					gotX0, gotY0, gotX1, gotY1, _, _ := DecodePos2(EncodePos2(x0, y0, x1, y1, 8, 8))
					if gotX0 != x0 || gotY0 != y0 || gotX1 != x1 || gotY1 != y1 {
						t.Fatalf("DecodePos2 round trip of (%d,%d)->(%d,%d) = (%d,%d)->(%d,%d)",
							x0, y0, x1, y1, gotX0, gotY0, gotX1, gotY1)
					}
				}
			}
		}
	}
}

func TestPos2SubCellOffsets(t *testing.T) {
	_, _, _, _, sx, sy := DecodePos2(EncodePos2(10, 20, 30, 40, 12, 5))
	if sx != 12 || sy != 5 {
		t.Errorf("sub-cell offsets = (%d,%d), want (12,5)", sx, sy)
	}
}

// TestPosAgainstServerBytes pins the layout against bytes packed by hand from
// rAthena's WBUFPOS, so the round-trip tests above cannot both be wrong in the
// same direction.
func TestPosAgainstServerBytes(t *testing.T) {
	// x=153, y=244, dir=4:
	//   p[0] = 153>>2            = 38   = 0x26
	//   p[1] = (153<<6)|((244>>4)&0x3f) = 0x40|0x0F = 0x4F
	//   p[2] = (244<<4)|4        = 0x44
	x, y, dir := DecodePos([]byte{0x26, 0x4F, 0x44})
	if x != 153 || y != 244 || dir != 4 {
		t.Errorf("DecodePos = (%d,%d,%d), want (153,244,4)", x, y, dir)
	}
}

func TestPosShortBuffer(t *testing.T) {
	if x, y, dir := DecodePos([]byte{1, 2}); x != 0 || y != 0 || dir != 0 {
		t.Error("a short buffer should decode to zeros, not read past the end")
	}
	if x0, _, _, _, _, _ := DecodePos2([]byte{1, 2, 3}); x0 != 0 {
		t.Error("a short buffer should decode to zeros, not read past the end")
	}
}
