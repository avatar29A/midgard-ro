package states

import (
	"strconv"
	"strings"
)

// Effects the archive draws rather than describes.
//
// Some of RO's effects are neither an STR animation nor a handful of quads
// cut from a texture: they are sprites, drawn frame by frame like a monster.
// The archive holds four hundred and fifty-eight of them under
// data/sprite/이팩트, and the ice a frozen target is sealed in is one.
//
// Reaching for a texture instead is how Frost Diver came out as a row of pale
// rectangles. ice.tga, which the spikes were cut from, is a seamless surface
// with no silhouette at all — it is meant to be wrapped around a shape, not
// drawn as one. The sprite already is the shape.

// The frozen sprite. Its name is the Korean children's game — freeze tag —
// and its frames run from the block that closes over a target down to the
// shards that come up out of the ground before it.
const frozenSprite = `data\sprite\이팩트\얼음땡.spr`

// Frames of it, by what they are.
//
// The line uses the two large ones rather than the five small ones. The small
// frames are a dozen pixels across, and a dozen pixels blown up to a couple of
// cells is a staircase however it is filtered: at the size the ice has to
// stand, the big frames are drawn about life size and the small ones six
// times over. What is lost is variety, and there is enough of that in the
// sizes and the lean.
const (
	frozenBlock  = 0 // the cluster a sealed target stands inside
	frozenShard  = 0 // the first of the ones the line is built from
	frozenShards = 2
)

// frozenFrameSize is how big each frame's art is, in pixels.
//
// Written down rather than read, because the quads are laid out before
// anything has loaded a texture and a shard drawn square is not a shard. The
// archive is the authority and a test holds these to it.
var frozenFrameSize = [][2]float32{
	{104, 121},
	{42, 83},
	{14, 35},
	{16, 34},
	{17, 22},
	{18, 22},
	{10, 13},
}

// spriteFrameKey names one frame of a sprite the way a path names a file, so
// anything that draws through the texture cache can ask for either.
func spriteFrameKey(sprPath string, frame int) string {
	return sprPath + "#" + strconv.Itoa(frame)
}

// SpriteFrameOf splits such a name back up, and says whether it was one.
func SpriteFrameOf(key string) (sprPath string, frame int, ok bool) {
	at := strings.LastIndex(key, "#")
	if at < 0 {
		return key, 0, false
	}

	frame, err := strconv.Atoi(key[at+1:])
	if err != nil {
		return key, 0, false
	}

	return key[:at], frame, true
}

// frozenPart is a quad for one frame of the frozen sprite, a given number of
// world units tall and as wide as the art says it should be.
//
// The width from the art rather than chosen: these are jagged shapes of
// different proportions, and a shard given the block's proportions is a slab.
func frozenPart(frame int, height float32) burstParticle {
	size := frozenFrameSize[frame]

	return burstParticle{
		halfH:    height / 2,
		halfW:    height / 2 * size[0] / size[1],
		texture:  spriteFrameKey(frozenSprite, frame),
		tint:     [3]float32{1, 1, 1},
		maxAlpha: 1,
	}
}
