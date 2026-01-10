# UC-105: Input Event Handling

## Description
Tests SDL2 input event handling including keyboard, mouse, and window events. Input is fundamental for user interaction.

## Preconditions
- Window is created and visible
- Input handler is created
- Application is running

## Test Steps

### Quit Event
1. Click window close button (X)
2. Verify `input.Update()` detects quit event
3. Verify application exits gracefully

### Keyboard Event - ESC to Quit
1. Press ESC key
2. Verify key down event is detected
3. Verify scancode is 41 (SDL_SCANCODE_ESCAPE)
4. Verify application sets `running = false`
5. Verify application exits gracefully

### Window Resize Event
1. Resize window
2. Verify `EventWindowResize` is generated
3. Verify event contains new width and height
4. Verify renderer is notified

### Event Queue
1. Generate multiple events (key presses, mouse moves)
2. Call `input.Update()` and `input.Events()`
3. Verify all events are captured in order
4. Verify event queue is cleared between frames

## Expected Results
- All SDL events are captured correctly
- Event types are mapped correctly
- Event data (key codes, dimensions, etc.) is accurate
- Event queue doesn't overflow or miss events
- Application responds to quit events properly

## Priority
High

## Related
- PRD Section: 6.2 Main Game Loop (input processing)
- ADR: ADR-002-architecture.md
- Code: `/Users/borisglebov/git/Faultbox/midgard-ro/internal/engine/input/input.go`
- Code: `/Users/borisglebov/git/Faultbox/midgard-ro/internal/game/game.go::Run()` (event handling)
- Test: None (requires manual interaction)

## Manual Verification Required
- Press ESC → application quits
- Click X button → application quits
- Resize window → triangle resizes correctly
- All quit methods exit cleanly (no crashes)
