package states

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Faultbox/midgard-ro/pkg/encoding"
	"github.com/Faultbox/midgard-ro/pkg/formats"
	"github.com/Faultbox/midgard-ro/pkg/grf"
)

// TestSpriteFrameNames: a frame is asked for the way a file is, and the two
// have to be told apart again on the way in. A path read as a frame loads
// nothing; a frame read as a path loads the whole sprite as an image.
func TestSpriteFrameNames(t *testing.T) {
	key := spriteFrameKey(frozenSprite, 3)

	path, frame, ok := SpriteFrameOf(key)
	if !ok {
		t.Fatalf("%q does not read back as a frame", key)
	}
	if path != frozenSprite || frame != 3 {
		t.Errorf("%q read back as %q frame %d", key, path, frame)
	}

	for _, plain := range []string{
		`data\texture\effect\lens1.tga`,
		`data\texture\effect\불화살1.tga`,
		"",
	} {
		if _, _, ok := SpriteFrameOf(plain); ok {
			t.Errorf("%q read as a sprite frame", plain)
		}
	}
}

// TestAShardKeepsItsProportions: the frames are jagged shapes of quite
// different shapes, and one given another's proportions is a slab.
func TestAShardKeepsItsProportions(t *testing.T) {
	for frame, size := range frozenFrameSize {
		p := frozenPart(frame, 10)

		want := size[0] / size[1]
		if got := p.halfW / p.halfH; got < want-0.01 || got > want+0.01 {
			t.Errorf("frame %d is drawn %v wide for its height, want %v", frame, got, want)
		}
		if p.halfH != 5 {
			t.Errorf("frame %d is %v tall, want half of the ten asked for", frame, p.halfH*2)
		}
	}
}

// TestTheFrozenFramesAreWhatTheArchiveHolds: the sizes are written down here
// because the quads are laid out before anything has loaded a texture. The
// archive is the authority, and this is what holds them to it.
//
// Needs the client's GRFs:
//
//	MIDGARD_GRF=/path/to/data go test ./internal/game/states/
func TestTheFrozenFramesAreWhatTheArchiveHolds(t *testing.T) {
	dir := os.Getenv("MIDGARD_GRF")
	if dir == "" {
		t.Skip("set MIDGARD_GRF to the directory holding data.grf to run this")
	}

	var data []byte
	for _, name := range []string{"data.grf", "rdata.grf"} {
		a, err := grf.Open(filepath.Join(dir, name))
		if err != nil {
			continue
		}

		// The archive keeps its paths in the Korean encoding.
		if found, err := a.Read(string(encoding.UTF8ToEUCKR(frozenSprite))); err == nil {
			data = found
		}
		a.Close()

		if data != nil {
			break
		}
	}

	if data == nil {
		t.Fatalf("%s is not in the archive", frozenSprite)
	}

	spr, err := formats.ParseSPR(data)
	if err != nil {
		t.Fatalf("parsing %s: %v", frozenSprite, err)
	}

	if len(spr.Images) != len(frozenFrameSize) {
		t.Fatalf("the sprite has %d frames and this file names %d",
			len(spr.Images), len(frozenFrameSize))
	}

	for i, img := range spr.Images {
		want := frozenFrameSize[i]
		if float32(img.Width) != want[0] || float32(img.Height) != want[1] {
			t.Errorf("frame %d is %dx%d in the archive, written down here as %vx%v",
				i, img.Width, img.Height, want[0], want[1])
		}
	}

	// The block a target is sealed in is the big one, and the shards are the
	// small ones. Pointing at the wrong frame draws a shard where the block
	// should be, which is a target standing in the open with a chip of ice
	// beside it.
	block := spr.Images[frozenBlock]
	for i := frozenShard; i < frozenShard+frozenShards; i++ {
		if spr.Images[i].Height >= block.Height {
			t.Errorf("frame %d is %d tall against the block's %d; it is not a shard",
				i, spr.Images[i].Height, block.Height)
		}
	}
}
