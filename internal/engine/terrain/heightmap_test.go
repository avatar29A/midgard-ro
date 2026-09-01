package terrain

import "testing"

// slope is a one-tile heightmap rising from west to east, with the corners in
// the order the mesh lays them out: south-west, south-east, north-west,
// north-east.
func slope(sw, se, nw, ne float32) *Heightmap {
	return &Heightmap{
		Corners:  [][][4]float32{{{sw, se, nw, ne}}},
		TilesX:   1,
		TilesZ:   1,
		TileZoom: 10,
	}
}

// TestHeightAtCorners: each corner reads back as itself, negated. The mesh
// puts a corner's world Y at minus its stored altitude, and a query that
// forgot the sign stands everything under the map.
func TestHeightAtCorners(t *testing.T) {
	h := slope(-10, -20, -30, -40)

	for _, tc := range []struct {
		x, z float32
		want float32
	}{
		{0, 0, 10},   // south-west
		{10, 0, 20},  // south-east
		{0, 10, 30},  // north-west
		{10, 10, 40}, // north-east
	} {
		// The far corners belong to the next tile along, which this map does
		// not have, so they are sampled a hair inside.
		x, z := tc.x, tc.z
		if x >= 10 {
			x = 9.999
		}
		if z >= 10 {
			z = 9.999
		}

		got := h.HeightAt(x, z)
		if diff := got - tc.want; diff > 0.02 || diff < -0.02 {
			t.Errorf("HeightAt(%v, %v) = %v, want %v", tc.x, tc.z, got, tc.want)
		}
	}
}

// TestHeightAtRisesAcrossTheTile: the whole point. Sampled as one height per
// tile this was flat all the way across and then jumped; a character walking
// it rose and fell in tile-wide steps.
func TestHeightAtRisesAcrossTheTile(t *testing.T) {
	h := slope(-10, -20, -10, -20)

	last := h.HeightAt(0, 5)
	for x := float32(1); x < 10; x++ {
		got := h.HeightAt(x, 5)
		if got <= last {
			t.Fatalf("height at x=%v is %v, no higher than %v a step back", x, got, last)
		}

		last = got
	}

	// Halfway across is halfway up, which a mesh quad's own surface is.
	if got := h.HeightAt(5, 5); got < 14.9 || got > 15.1 {
		t.Errorf("halfway across reads %v, want 15", got)
	}
}

// TestHeightAtIsFlatOnAFlatTile: a tile whose corners agree has one height,
// wherever it is sampled.
func TestHeightAtIsFlatOnAFlatTile(t *testing.T) {
	h := slope(-25, -25, -25, -25)

	for _, p := range [][2]float32{{0, 0}, {3, 7}, {9.9, 9.9}, {5, 5}} {
		if got := h.HeightAt(p[0], p[1]); got != 25 {
			t.Errorf("HeightAt%v = %v, want 25 everywhere on a flat tile", p, got)
		}
	}
}

// TestHeightAtOffTheMap: outside is zero, which is what a flat map gives, and
// a nil heightmap answers rather than panicking — the ground is asked for
// before a map has loaded.
func TestHeightAtOffTheMap(t *testing.T) {
	h := slope(-10, -10, -10, -10)

	for _, p := range [][2]float32{{-1, 5}, {5, -1}, {10.5, 5}, {5, 10.5}} {
		if got := h.HeightAt(p[0], p[1]); got != 0 {
			t.Errorf("HeightAt%v = %v, want 0 off the map", p, got)
		}
	}

	var none *Heightmap
	if got := none.HeightAt(5, 5); got != 0 {
		t.Errorf("a nil heightmap gave %v", got)
	}

	if got := (&Heightmap{}).HeightAt(5, 5); got != 0 {
		t.Errorf("an empty heightmap gave %v", got)
	}
}

// TestHeightAtCornerOrderMatchesTheMesh: mesh.go puts Altitude[0] at the
// tile's origin, [1] one tile east, [2] one tile north and [3] on the far
// corner. Reading them in any other order tilts the ground the wrong way,
// which on a real map is a character sunk into one slope and floating over
// the next.
func TestHeightAtCornerOrderMatchesTheMesh(t *testing.T) {
	// Only the south-east corner is raised.
	h := slope(0, -50, 0, 0)

	if got := h.HeightAt(9.9, 0); got < 49 {
		t.Errorf("the east end of the south edge reads %v, want it raised", got)
	}
	if got := h.HeightAt(0, 9.9); got != 0 {
		t.Errorf("the west end of the north edge reads %v, want it flat", got)
	}
}
