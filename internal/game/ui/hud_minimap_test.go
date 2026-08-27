package ui

import (
	"image"
	"math"
	"testing"

	"github.com/Faultbox/midgard-ro/internal/engine/charsprite"
)

// TestMinimapProjectCentersTheMiddleCell is the check that catches the mistake
// this projection exists to avoid. RO's minimap image covers a square of
// max(width, height) cells with the shorter axis letterboxed — Prontera's
// 312x392 arrives as 512x512 — so treating the image as the map's own aspect
// squashes it and walks the marker away from the player.
func TestMinimapProjectCentersTheMiddleCell(t *testing.T) {
	const box = minimapSize

	tests := []struct {
		name           string
		cellsX, cellsY int
	}{
		{"prontera, taller than wide", 312, 392},
		{"wider than tall", 400, 200},
		{"square", 256, 256},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			px, py := minimapProject(tt.cellsX/2, tt.cellsY/2, tt.cellsX, tt.cellsY, 1, 0, 0)

			if math.Abs(float64(px)-box/2) > 0.5 {
				t.Errorf("the middle cell sits at x=%v, want the box center %v", px, box/2)
			}
			if math.Abs(float64(py)-box/2) > 0.5 {
				t.Errorf("the middle cell sits at y=%v, want the box center %v", py, box/2)
			}
		})
	}
}

// TestMinimapProjectFlipsY: cell Y counts north, screen Y counts down. Getting
// this wrong is not obvious on a square map — the marker just moves the wrong
// way as you walk.
func TestMinimapProjectFlipsY(t *testing.T) {
	_, southY := minimapProject(100, 10, 312, 392, 1, 0, 0)
	_, northY := minimapProject(100, 380, 312, 392, 1, 0, 0)

	if !(northY < southY) {
		t.Errorf("walking north moved the marker from y=%v to y=%v; north should be up", southY, northY)
	}
}

// TestMinimapProjectStaysInTheBox: every cell has to land inside the image, or
// the marker draws over the map next to it.
func TestMinimapProjectStaysInTheBox(t *testing.T) {
	const box = minimapSize
	const cellsX, cellsY = 312, 392

	corners := [][2]int{{0, 0}, {cellsX - 1, 0}, {0, cellsY - 1}, {cellsX - 1, cellsY - 1}}
	for _, c := range corners {
		px, py := minimapProject(c[0], c[1], cellsX, cellsY, 1, 0, 0)
		if px < 0 || px > box || py < 0 || py > box {
			t.Errorf("cell (%d,%d) projects to (%v,%v), outside the %v box", c[0], c[1], px, py, box)
		}
	}
}

// TestMinimapProjectLetterboxesTheShortAxis: the short axis should be inset by
// half the difference, leaving the map centered rather than pinned to an edge.
func TestMinimapProjectLetterboxesTheShortAxis(t *testing.T) {
	const box = 128.0
	// 312 wide in a 392 square: the map occupies 312/392 of the width, so the
	// left edge sits (392-312)/2/392 of the way in.
	wantLeft := float32((392.0 - 312.0) / 2 / 392.0 * box)

	px, _ := minimapProject(0, 0, 312, 392, 1, 0, 0)
	if math.Abs(float64(px-wantLeft)) > 0.5 {
		t.Errorf("cell x=0 projects to %v, want %v — the short axis is not centered", px, wantLeft)
	}
}

func TestMinimapProjectHandlesAnEmptyMap(t *testing.T) {
	if px, py := minimapProject(5, 5, 0, 0, 1, 0, 0); px != 0 || py != 0 {
		t.Errorf("a map with no cells projected to (%v,%v), want the origin", px, py)
	}
}

func TestMinimapImagePath(t *testing.T) {
	tests := []struct {
		mapName string
		want    string
	}{
		{"prontera.gat", minimapPath + `prontera.bmp`},
		{"prontera", minimapPath + `prontera.bmp`},
		{"prt_in.gat", minimapPath + `prt_in.bmp`},
		{"", ""},
	}

	for _, tt := range tests {
		if got := minimapImagePath(tt.mapName); got != tt.want {
			t.Errorf("minimapImagePath(%q) = %q, want %q", tt.mapName, got, tt.want)
		}
	}
}

// TestMinimapZoomKeepsThePlayerCentered: zooming in shows a slice of the map
// around the player, and the whole point is that the marker stays put while
// the map moves under it.
func TestMinimapZoomKeepsThePlayerCentered(t *testing.T) {
	const cellsX, cellsY = 312, 392

	for _, zoom := range minimapZooms[1:] {
		px, py := minimapProject(100, 200, cellsX, cellsY, zoom, 100, 200)

		if px != minimapSize/2 || py != minimapSize/2 {
			t.Errorf("at zoom %v the centered cell drew at (%v,%v), want the box center %v",
				zoom, px, py, minimapSize/2)
		}
	}
}

// TestMinimapZoomSpreadsTheMap: a cell a fixed distance from the player should
// sit further from the marker the further you zoom in.
func TestMinimapZoomSpreadsTheMap(t *testing.T) {
	const cellsX, cellsY = 312, 392

	near, _ := minimapProject(120, 200, cellsX, cellsY, 1, 100, 200)
	far, _ := minimapProject(120, 200, cellsX, cellsY, 4, 100, 200)

	centerAt1, _ := minimapProject(100, 200, cellsX, cellsY, 1, 100, 200)

	spread1 := near - centerAt1
	spread4 := far - minimapSize/2

	if !(spread4 > spread1*3) {
		t.Errorf("zooming 4x spread a cell from %v to %v; it should be about four times further",
			spread1, spread4)
	}
}

// TestMinimapViewStaysInsideTheImage: the window must never run off the edge,
// or walking into a corner shows blank space beside the map.
func TestMinimapViewStaysInsideTheImage(t *testing.T) {
	const cellsX, cellsY = 312, 392

	corners := [][2]int{{0, 0}, {cellsX - 1, 0}, {0, cellsY - 1}, {cellsX - 1, cellsY - 1}}

	for _, zoom := range minimapZooms {
		for _, c := range corners {
			u0, v0, u1, v1 := minimapViewUV(c[0], c[1], cellsX, cellsY, zoom)

			if u0 < 0 || v0 < 0 || u1 > 1.0001 || v1 > 1.0001 {
				t.Errorf("at zoom %v cell (%d,%d) views (%v,%v)-(%v,%v), outside the image",
					zoom, c[0], c[1], u0, v0, u1, v1)
			}
			if u1 <= u0 || v1 <= v0 {
				t.Errorf("at zoom %v cell (%d,%d) views an empty rect", zoom, c[0], c[1])
			}
		}
	}
}

// TestMinimapViewIsWholeAtZoomOne: zoom 1 shows the entire map, which is what
// the minimap has always done and what the - button returns to.
func TestMinimapViewIsWholeAtZoomOne(t *testing.T) {
	u0, v0, u1, v1 := minimapViewUV(150, 200, 312, 392, 1)
	if u0 != 0 || v0 != 0 || u1 != 1 || v1 != 1 {
		t.Errorf("zoom 1 views (%v,%v)-(%v,%v), want the whole image", u0, v0, u1, v1)
	}
}

// TestRotateRGBAKeepsTheImage: a rotation that lost pixels would thin the
// arrow at some facings and not others, which reads as flicker while turning.
func TestRotateRGBAKeepsTheImage(t *testing.T) {
	const size = 12

	src := image.NewRGBA(image.Rect(0, 0, size, size))
	// A solid block in the middle, well inside the circle the rotation sweeps.
	for y := 4; y < 8; y++ {
		for x := 4; x < 8; x++ {
			copy(src.Pix[src.PixOffset(x, y):][:4], []byte{255, 0, 0, 255})
		}
	}

	count := func(img *image.RGBA) int {
		n := 0
		for i := 3; i < len(img.Pix); i += 4 {
			if img.Pix[i] != 0 {
				n++
			}
		}

		return n
	}

	want := count(src)
	for dir := 0; dir < charsprite.Directions; dir++ {
		got := count(rotateRGBA(src, arrowAngleFor(dir)))

		// Nearest sampling can drop or double a pixel at the edges; a
		// rotation that halved the block would be a real fault.
		if got < want*3/4 || got > want*5/4 {
			t.Errorf("facing %d kept %d of %d pixels", dir, got, want)
		}
	}
}

// TestRotateRGBASizeIsStable: every facing has to draw at the same size, or
// the arrow grows and shrinks as the player turns.
func TestRotateRGBASizeIsStable(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 12, 12))

	for dir := 0; dir < charsprite.Directions; dir++ {
		got := rotateRGBA(src, arrowAngleFor(dir)).Bounds()
		if got.Dx() != 12 || got.Dy() != 12 {
			t.Errorf("facing %d rotated to %dx%d, want 12x12", dir, got.Dx(), got.Dy())
		}
	}
}

// TestArrowAnglesAreDistinct: eight facings should give eight headings, a
// whole turn between them and no two the same.
func TestArrowAnglesAreDistinct(t *testing.T) {
	seen := map[int]bool{}

	for dir := 0; dir < charsprite.Directions; dir++ {
		deg := int(arrowAngleFor(dir)*180/math.Pi) % 360
		if seen[deg] {
			t.Errorf("facing %d repeats a heading already used (%d degrees)", dir, deg)
		}
		seen[deg] = true
	}

	if len(seen) != charsprite.Directions {
		t.Errorf("got %d distinct headings, want %d", len(seen), charsprite.Directions)
	}
}
