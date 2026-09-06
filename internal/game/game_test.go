package game

import (
	"math"
	"testing"
)

// TestMinimapArrowFollowsTheCamera: the map is north-up and the arrow shows
// where the camera looks, so its facing is the yaw snapped to the eight the
// arrow is baked in — and the index chosen is the one whose baked bearing,
// (d+4)*45°, points that way.
func TestMinimapArrowFollowsTheCamera(t *testing.T) {
	const deg = math.Pi / 180

	for _, tc := range []struct {
		yaw  float64
		want uint8
	}{
		{0, 4},         // looking north: arrow up
		{90 * deg, 6},  // east: arrow right
		{180 * deg, 0}, // south: arrow down
		{270 * deg, 2}, // west: arrow left
		{45 * deg, 5},  // northeast
		{-90 * deg, 2}, // west from the other side
		{360 * deg, 4}, // a full turn is north again
	} {
		if got := minimapDirFromYaw(float32(tc.yaw)); got != tc.want {
			t.Errorf("minimapDirFromYaw(%.0f°) = %d, want %d", tc.yaw/deg, got, tc.want)
		}
	}
}
