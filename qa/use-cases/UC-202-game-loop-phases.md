# UC-202: Game Loop Phases

## Description
Tests that the game loop executes all phases in the correct order each frame: Input → Update → Render → Present.

## Preconditions
- Game is initialized and running

## Test Steps
1. Start game loop
2. In first iteration, verify phases execute in order:
   - Phase 1: `input.Update()` is called
   - Phase 2: `game.update(dt)` is called
   - Phase 3: `game.render()` is called
   - Phase 4: `window.SwapBuffers()` is called
3. Verify this order is maintained every frame
4. Verify each phase completes before next begins

## Expected Results
- Phases execute in fixed order every frame
- Input is processed before game state updates
- Game state is updated before rendering
- Rendering completes before buffer swap
- No phase is skipped
- Phases don't overlap or run out of order

## Priority
Critical

## Related
- PRD Section: 6.2 Main Game Loop
- ADR: ADR-002-architecture.md
- Code: `/Users/borisglebov/git/Faultbox/midgard-ro/internal/game/game.go::Run()`
- Test: None (code inspection + runtime verification)

## Game Loop Structure
```go
for running {
    // Calculate delta time
    dt := calculateDelta()

    // 1. INPUT PHASE
    if input.Update() { quit }
    processEvents()

    // 2. UPDATE PHASE
    update(dt)

    // 3. RENDER PHASE
    render()

    // 4. PRESENT PHASE
    swapBuffers()
}
```

## What Each Phase Does
- **Input**: Polls SDL events, updates input state
- **Update**: Updates game logic, entities, physics
- **Render**: Draws frame to back buffer
- **Present**: Swaps buffers, shows frame on screen
