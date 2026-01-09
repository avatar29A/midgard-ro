// Package input handles SDL2 input events.
package input

import (
	"github.com/veandco/go-sdl2/sdl"
)

// Event types for game use
type EventType int

const (
	EventNone EventType = iota
	EventQuit
	EventWindowResize
	EventKeyDown
	EventKeyUp
	EventMouseMove
	EventMouseDown
	EventMouseUp
)

// Event represents a processed input event.
type Event struct {
	Type   EventType
	Key    sdl.Scancode
	Width  int
	Height int
	MouseX int
	MouseY int
	Button uint8
}

// Input handles all input processing.
type Input struct {
	events []Event
}

// New creates a new input handler.
func New() *Input {
	return &Input{
		events: make([]Event, 0, 16),
	}
}

// Update polls SDL events and converts them to game events.
// Returns true if the game should quit.
func (i *Input) Update() bool {
	i.events = i.events[:0] // Clear previous events

	for event := sdl.PollEvent(); event != nil; event = sdl.PollEvent() {
		switch e := event.(type) {
		case *sdl.QuitEvent:
			i.events = append(i.events, Event{Type: EventQuit})
			return true

		case *sdl.WindowEvent:
			if e.Event == sdl.WINDOWEVENT_RESIZED {
				i.events = append(i.events, Event{
					Type:   EventWindowResize,
					Width:  int(e.Data1),
					Height: int(e.Data2),
				})
			}

		case *sdl.KeyboardEvent:
			if e.Type == sdl.KEYDOWN {
				i.events = append(i.events, Event{
					Type: EventKeyDown,
					Key:  e.Keysym.Scancode,
				})
			} else if e.Type == sdl.KEYUP {
				i.events = append(i.events, Event{
					Type: EventKeyUp,
					Key:  e.Keysym.Scancode,
				})
			}

		case *sdl.MouseMotionEvent:
			i.events = append(i.events, Event{
				Type:   EventMouseMove,
				MouseX: int(e.X),
				MouseY: int(e.Y),
			})

		case *sdl.MouseButtonEvent:
			if e.Type == sdl.MOUSEBUTTONDOWN {
				i.events = append(i.events, Event{
					Type:   EventMouseDown,
					MouseX: int(e.X),
					MouseY: int(e.Y),
					Button: e.Button,
				})
			} else if e.Type == sdl.MOUSEBUTTONUP {
				i.events = append(i.events, Event{
					Type:   EventMouseUp,
					MouseX: int(e.X),
					MouseY: int(e.Y),
					Button: e.Button,
				})
			}
		}
	}

	return false
}

// Events returns the events from the last Update.
func (i *Input) Events() []Event {
	return i.events
}

// IsKeyPressed checks if a specific key was pressed this frame.
func (i *Input) IsKeyPressed(scancode sdl.Scancode) bool {
	for _, e := range i.events {
		if e.Type == EventKeyDown && e.Key == scancode {
			return true
		}
	}
	return false
}
