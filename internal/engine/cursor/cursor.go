// Package cursor draws the game's mouse cursor from the original client's
// sprite.
//
// RO keeps its cursor in one sprite with an action per state, most of them
// animated: the arrow has 11 frames, the target ring 20. A static image would
// be the wrong cursor for eight of the fourteen states, so the sprite is baked
// into per-state frame lists and stepped on the clock.
package cursor

import (
	"fmt"
	"time"

	"github.com/Faultbox/midgard-ro/internal/engine/sprite"
	"github.com/Faultbox/midgard-ro/pkg/formats"
)

// SPRPath and ACTPath are where the cursor lives in the archives.
const (
	SPRPath = "data/sprite/cursors.spr"
	ACTPath = "data/sprite/cursors.act"
)

// State is which cursor the game is showing. The values are the sprite's own
// action indices, so they double as the lookup.
type State int

// The states the original client uses. The sprite carries a few more actions
// than these, unnamed and unused.
const (
	StateDefault State = 0  // the arrow
	StateTalk    State = 1  // over an NPC
	StateClick   State = 2  // clicking
	StateLock    State = 3  // over a locked target
	StateRotate  State = 4  // rotating the camera
	StateAttack  State = 5  // over an attackable target
	StateWarp    State = 7  // over a portal
	StatePick    State = 9  // over an item
	StateTarget  State = 10 // skill targeting
)

const (
	// intervalTick is what one unit of an ACT interval is worth. RO counts
	// animation in ticks of 25ms, not milliseconds: the character sprite's
	// walk interval of 3 is the 75ms the walk animation actually runs at.
	// Reading the field as milliseconds runs everything 25x too fast.
	intervalTick = 25 * time.Millisecond

	// defaultFrameDelay is used when the sprite gives an action no interval of
	// its own; without it a frame list would advance every tick.
	defaultFrameDelay = 100 * time.Millisecond
)

// speed scales a state's frame delay. The original runs the arrow at half rate
// and the targeting ring at double, which is what makes them read as an idle
// shimmer and an urgent pulse rather than one uniform flicker.
var speed = map[State]float64{
	StateDefault: 2.0,
	StateTarget:  0.5,
}

// Frame is one baked cursor image.
type Frame struct {
	Texture uint32

	// Width and Height are the image's size.
	Width, Height float32

	// OffsetX and OffsetY place the image against the mouse: the sprite's own
	// origin is the point the user is pointing at.
	OffsetX, OffsetY float32
}

// action is one state's animation.
type action struct {
	frames []Frame
	delay  time.Duration
}

// Cursor is the drawable cursor. It is not safe for concurrent use; it belongs
// to the frame loop.
type Cursor struct {
	actions map[State]*action

	state   State
	frame   int
	elapsed time.Duration
}

// Uploader turns RGBA pixels into a texture. The engine's renderer provides
// one; taking it as a function keeps this package away from the UI.
type Uploader func(width, height int, pixels []byte) uint32

// Load reads the cursor sprite and bakes every state's frames.
func Load(read func(path string) ([]byte, error), upload Uploader) (*Cursor, error) {
	if read == nil || upload == nil {
		return nil, fmt.Errorf("cursor needs an archive reader and an uploader")
	}

	sprData, err := read(SPRPath)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", SPRPath, err)
	}

	actData, err := read(ACTPath)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", ACTPath, err)
	}

	spr, err := formats.ParseSPR(sprData)
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", SPRPath, err)
	}

	act, err := formats.ParseACT(actData)
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", ACTPath, err)
	}

	c := &Cursor{actions: make(map[State]*action, len(act.Actions))}

	for index := range act.Actions {
		if baked := bake(spr, act, index, upload); baked != nil {
			c.actions[State(index)] = baked
		}
	}

	if len(c.actions) == 0 {
		return nil, fmt.Errorf("cursor sprite has no usable actions")
	}

	return c, nil
}

// bake composites and uploads every frame of one action.
func bake(spr *formats.SPR, act *formats.ACT, index int, upload Uploader) *action {
	frames := make([]Frame, 0, len(act.Actions[index].Frames))

	for frame := range act.Actions[index].Frames {
		// The cursor is a single-piece sprite with no directions, so the
		// action index goes in the direction slot: compositing looks up
		// action*8+direction, and the action here is zero.
		r := sprite.CompositeSprites(spr, act, nil, nil, 0, index, frame, 0)
		if r.Width == 0 || r.Height == 0 {
			continue
		}

		frames = append(frames, Frame{
			Texture: upload(r.Width, r.Height, r.Pixels),
			Width:   float32(r.Width),
			Height:  float32(r.Height),
			OffsetX: float32(r.OffsetX),
			OffsetY: float32(r.OffsetY),
		})
	}

	if len(frames) == 0 {
		return nil
	}

	return &action{frames: frames, delay: frameDelay(act, index)}
}

// frameDelay is how long one frame of an action holds, at the rate the
// original plays that state.
func frameDelay(act *formats.ACT, index int) time.Duration {
	delay := defaultFrameDelay

	if index < len(act.Intervals) && act.Intervals[index] > 0 {
		delay = time.Duration(float64(act.Intervals[index]) * float64(intervalTick))
	}

	if mult, ok := speed[State(index)]; ok {
		delay = time.Duration(float64(delay) * mult)
	}

	if delay <= 0 {
		delay = defaultFrameDelay
	}

	return delay
}

// SetState switches cursor, restarting its animation. Setting the state it is
// already in does nothing, so a state held across frames keeps animating.
func (c *Cursor) SetState(s State) {
	if c == nil || c.state == s {
		return
	}

	if _, ok := c.actions[s]; !ok {
		s = StateDefault
	}

	c.state = s
	c.frame = 0
	c.elapsed = 0
}

// State returns the cursor being shown.
func (c *Cursor) State() State {
	if c == nil {
		return StateDefault
	}

	return c.state
}

// Update advances the animation.
func (c *Cursor) Update(dt time.Duration) {
	if c == nil {
		return
	}

	a := c.actions[c.state]
	if a == nil || len(a.frames) < 2 {
		return
	}

	c.elapsed += dt
	for c.elapsed >= a.delay {
		c.elapsed -= a.delay
		c.frame = (c.frame + 1) % len(a.frames)
	}
}

// Frame returns what to draw, and whether there is anything to draw.
// FrameOf is the current frame of a state other than the one the pointer is
// showing.
//
// The marker over a locked target is drawn from the same sprite as the
// cursor, in world space rather than under the pointer, so it needs a frame
// from a state that is not the current one. It rides the same clock, which
// keeps the two in step and costs nothing.
func (c *Cursor) FrameOf(s State) (Frame, bool) {
	if c == nil {
		return Frame{}, false
	}

	a := c.actions[s]
	if a == nil || len(a.frames) == 0 {
		return Frame{}, false
	}

	return a.frames[c.frame%len(a.frames)], true
}

func (c *Cursor) Frame() (Frame, bool) {
	if c == nil {
		return Frame{}, false
	}

	a := c.actions[c.state]
	if a == nil || len(a.frames) == 0 {
		return Frame{}, false
	}

	return a.frames[c.frame%len(a.frames)], true
}
