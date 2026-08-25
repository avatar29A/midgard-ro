// Package charsprite resolves RO character sprite paths, loads the SPR/ACT
// pairs behind them, and bakes head+body into per-frame composites ready for
// upload to the GPU.
//
// It is deliberately GL-free: everything here is pixels and metadata, so it
// can be exercised in tests without a rendering context. Callers own the
// texture upload (see internal/engine/playerrender).
package charsprite

import (
	"fmt"

	"github.com/Faultbox/midgard-ro/internal/engine/sprite"
	"github.com/Faultbox/midgard-ro/pkg/formats"
)

// Loader reads a file out of the GRF archives. It matches the signature the
// game already passes around for textures; the asset manager handles the
// EUC-KR encoding of the Korean paths below.
type Loader func(path string) ([]byte, error)

// Action indices within an ACT file. Each action occupies 8 consecutive
// entries, one per direction: actionIndex = action*8 + direction.
const (
	ActionIdle = 0
	ActionWalk = 1

	// LoadedActions is how many actions we bake composites for. Idle and walk
	// are all the game can currently drive; sit/attack/etc. follow the same
	// layout when they're needed.
	LoadedActions = 2

	// Directions is the number of facings every action provides.
	Directions = 8

	// MaxAnimationFrames bounds how much of one action is baked.
	//
	// Every frame becomes a full-size composited texture, which is enormously
	// wasteful for a single-piece sprite: nothing is being composited, and the
	// same handful of images are re-baked at different offsets. A Kafra's idle
	// runs 99 frames, so baking it whole costs 27 MB and 1584 texture uploads
	// in the frame she first comes into view — a visible stall.
	//
	// The real fix is to stop pre-compositing sprites that have nothing to
	// composite, and draw the frames from their own images with per-frame
	// offsets. Until then this bounds the damage, and Sheet.Dropped reports
	// when it bites rather than leaving a shortened loop to be discovered.
	MaxAnimationFrames = 40

	// HeadStraight is the head direction for looking dead ahead. RO gives a
	// character three head poses — turned each way and straight — which the
	// head sprite stores as its three "frames". The server can change this
	// per entity; until that is wired up everyone looks straight ahead.
	HeadStraight = 0
)

// Spec identifies which character sprites to load.
type Spec struct {
	// Kind selects the sprite family. The zero value is a player.
	Kind Kind

	Job       int  // rAthena job/class id
	Female    bool // sex M/F selects the sprite folder and filename suffix
	HairStyle int  // head sprite number

	// HeadDirection is which of the three head poses to bake (see
	// HeadStraight). Zero value looks straight ahead.
	HeadDirection int
}

// Sheet is a complete set of pre-composited frames for one character, keyed
// by action*8+direction. Every frame is padded to the same Width x Height so
// they can all share one billboard quad without the sprite jumping between
// frames.
type Sheet struct {
	Width  int
	Height int
	Frames map[int][]Frame

	// Dropped counts frames left unbaked by MaxAnimationFrames, so a
	// shortened animation is reported rather than silently shortened.
	Dropped int
}

// Frame is one baked animation frame: RGBA pixels at the sheet's dimensions.
type Frame struct {
	Pixels []byte
}

// Assets is everything loaded for one character.
type Assets struct {
	BodySPR *formats.SPR
	BodyACT *formats.ACT
	HeadSPR *formats.SPR
	HeadACT *formats.ACT
	Sheet   *Sheet

	// BodyPath is the archive path the body actually loaded from, for logs.
	BodyPath string
	// HeadPath is empty when no head sprite was found.
	HeadPath string
}

// FrameCount returns how many frames the given action/direction has.
func (s *Sheet) FrameCount(action, direction int) int {
	if s == nil {
		return 0
	}
	return len(s.Frames[action*Directions+direction])
}

// Load resolves the sprite paths for spec, reads them from the archives and
// bakes the composite sheet. A missing head is not fatal — the body renders
// on its own — but a missing body is.
func Load(load Loader, spec Spec) (*Assets, error) {
	candidates := spec.BodyPathCandidates()
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no sprite known for job %d", spec.Job)
	}

	var (
		bodySPR      *formats.SPR
		bodyACT      *formats.ACT
		bodySPRPath  string
		err          error
		firstAttempt = candidates[0][0]
	)
	for _, candidate := range candidates {
		bodySPR, bodyACT, err = loadPair(load, candidate[0], candidate[1])
		if err == nil {
			bodySPRPath = candidate[0]
			break
		}
	}
	if err != nil {
		return nil, fmt.Errorf("body sprite %q: %w", firstAttempt, err)
	}

	a := &Assets{
		BodySPR:  bodySPR,
		BodyACT:  bodyACT,
		BodyPath: bodySPRPath,
	}

	// The head is a separate sprite anchored to the body. Losing it costs us
	// a face, not the character, so failure here is only worth a note.
	headSPRPath, headACTPath := spec.HeadPaths()
	if headSPR, headACT, headErr := loadPair(load, headSPRPath, headACTPath); headErr == nil {
		a.HeadSPR = headSPR
		a.HeadACT = headACT
		a.HeadPath = headSPRPath
	}

	a.Sheet = BuildSheet(a.BodySPR, a.BodyACT, a.HeadSPR, a.HeadACT, spec.HeadDirection, spec.Kind)
	if a.Sheet == nil {
		return nil, fmt.Errorf("body sprite %q produced no frames", bodySPRPath)
	}

	return a, nil
}

// BuildSheet composites every frame of every loaded action/direction and pads
// them to a common size. Padding centers horizontally and aligns to the
// bottom, so the character's feet sit at the same place in every frame — the
// billboard quad is foot-anchored, so that keeps them planted on the ground
// as the animation plays.
//
// headDir picks which of the three head poses to bake in (see HeadStraight).
//
// What "standing still" means depends on the kind. For a player it is a single
// frame, not three: RO's idle action stores one entry per head direction
// rather than an animation — the body art is identical across all three, only
// the head turns — so cycling them makes a standing character swivel their
// head forever. Walking is a real animation, and its head stays on the chosen
// pose while the legs cycle.
//
// A monster or NPC has no head to pose, and its idle action is a real
// animation: a Kafra has 99 idle frames of her standing and shifting. Baking
// only the first leaves every one of them frozen, which is what treating them
// like players did.
func BuildSheet(bodySPR *formats.SPR, bodyACT *formats.ACT, headSPR *formats.SPR, headACT *formats.ACT, headDir int, kind Kind) *Sheet {
	if bodySPR == nil || bodyACT == nil || len(bodyACT.Actions) == 0 {
		return nil
	}
	if headDir < 0 {
		headDir = HeadStraight
	}

	// frameIndices returns the (bodyFrame, headFrame) pairs to bake for an
	// action. A player's idle indexes the body by head direction too, because
	// the body's neck anchor moves with the head pose.
	frameIndices := func(action, available int) [][2]int {
		if action == ActionIdle && kind == KindPlayer {
			return [][2]int{{headDir, headDir}}
		}
		if available > MaxAnimationFrames {
			available = MaxAnimationFrames
		}
		pairs := make([][2]int, 0, available)
		for i := 0; i < available; i++ {
			pairs = append(pairs, [2]int{i, headDir})
		}
		return pairs
	}

	// Count what the cap leaves out, so the caller can report it. A player's
	// unbaked idle entries do not count: those are head poses deliberately
	// left alone, not animation that went missing.
	dropped := 0
	for action := 0; action < LoadedActions; action++ {
		if action == ActionIdle && kind == KindPlayer {
			continue
		}
		for dir := 0; dir < Directions; dir++ {
			idx := action*Directions + dir
			if idx >= len(bodyACT.Actions) {
				continue
			}
			if extra := len(bodyACT.Actions[idx].Frames) - MaxAnimationFrames; extra > 0 {
				dropped += extra
			}
		}
	}

	// First pass: the largest frame decides the sheet size.
	maxW, maxH := 0, 0
	for action := 0; action < LoadedActions; action++ {
		for dir := 0; dir < Directions; dir++ {
			idx := action*Directions + dir
			if idx >= len(bodyACT.Actions) {
				continue
			}
			for _, fp := range frameIndices(action, len(bodyACT.Actions[idx].Frames)) {
				r := sprite.CompositeSprites(bodySPR, bodyACT, headSPR, headACT, action, dir, fp[0], fp[1])
				if r.Width > maxW {
					maxW = r.Width
				}
				if r.Height > maxH {
					maxH = r.Height
				}
			}
		}
	}
	if maxW == 0 || maxH == 0 {
		return nil
	}

	// Second pass: bake each frame onto a canvas of that size.
	sheet := &Sheet{
		Width:   maxW,
		Height:  maxH,
		Frames:  make(map[int][]Frame, LoadedActions*Directions),
		Dropped: dropped,
	}

	for action := 0; action < LoadedActions; action++ {
		for dir := 0; dir < Directions; dir++ {
			idx := action*Directions + dir
			if idx >= len(bodyACT.Actions) {
				continue
			}
			available := len(bodyACT.Actions[idx].Frames)
			if available == 0 {
				continue
			}

			pairs := frameIndices(action, available)
			frames := make([]Frame, len(pairs))
			for i, fp := range pairs {
				r := sprite.CompositeSprites(bodySPR, bodyACT, headSPR, headACT, action, dir, fp[0], fp[1])
				if r.Pixels == nil || r.Width == 0 || r.Height == 0 {
					// Keep the slot so frame indices stay contiguous.
					frames[i] = Frame{Pixels: make([]byte, maxW*maxH*4)}
					continue
				}
				frames[i] = Frame{Pixels: pad(r, maxW, maxH)}
			}
			sheet.Frames[idx] = frames
		}
	}

	if len(sheet.Frames) == 0 {
		return nil
	}
	return sheet
}

// pad centers a composite horizontally on a maxW x maxH canvas and aligns it
// to the bottom edge.
func pad(r sprite.CompositeResult, maxW, maxH int) []byte {
	out := make([]byte, maxW*maxH*4)
	offsetX := (maxW - r.Width) / 2
	offsetY := maxH - r.Height

	for py := 0; py < r.Height; py++ {
		dstY := offsetY + py
		if dstY < 0 || dstY >= maxH {
			continue
		}
		for px := 0; px < r.Width; px++ {
			dstX := offsetX + px
			if dstX < 0 || dstX >= maxW {
				continue
			}
			src := (py*r.Width + px) * 4
			dst := (dstY*maxW + dstX) * 4
			copy(out[dst:dst+4], r.Pixels[src:src+4])
		}
	}
	return out
}

func loadPair(load Loader, sprPath, actPath string) (*formats.SPR, *formats.ACT, error) {
	if load == nil {
		return nil, nil, fmt.Errorf("no asset loader")
	}

	sprData, err := load(sprPath)
	if err != nil {
		return nil, nil, fmt.Errorf("reading %s: %w", sprPath, err)
	}
	spr, err := formats.ParseSPR(sprData)
	if err != nil {
		return nil, nil, fmt.Errorf("parsing %s: %w", sprPath, err)
	}

	actData, err := load(actPath)
	if err != nil {
		return nil, nil, fmt.Errorf("reading %s: %w", actPath, err)
	}
	act, err := formats.ParseACT(actData)
	if err != nil {
		return nil, nil, fmt.Errorf("parsing %s: %w", actPath, err)
	}

	return spr, act, nil
}
