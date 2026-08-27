package ui

import (
	"math"
	"testing"
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
			px, py := minimapProject(tt.cellsX/2, tt.cellsY/2, tt.cellsX, tt.cellsY)

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
	_, southY := minimapProject(100, 10, 312, 392)
	_, northY := minimapProject(100, 380, 312, 392)

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
		px, py := minimapProject(c[0], c[1], cellsX, cellsY)
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

	px, _ := minimapProject(0, 0, 312, 392)
	if math.Abs(float64(px-wantLeft)) > 0.5 {
		t.Errorf("cell x=0 projects to %v, want %v — the short axis is not centered", px, wantLeft)
	}
}

func TestMinimapProjectHandlesAnEmptyMap(t *testing.T) {
	if px, py := minimapProject(5, 5, 0, 0); px != 0 || py != 0 {
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
