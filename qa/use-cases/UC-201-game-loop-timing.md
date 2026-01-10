# UC-201: Game Loop Timing

## Description
Tests the main game loop timing mechanism, including delta time calculation, frame counting, and FPS tracking.

## Preconditions
- Game is initialized successfully
- ShowFPS config is enabled

## Test Steps
1. Start game loop with `game.Run()`
2. Let game run for at least 5 seconds
3. Verify delta time (`dt`) is calculated each frame
4. Verify delta time is reasonable (0.016s for 60 FPS, 0.033s for 30 FPS)
5. Verify frame counter increments each frame
6. Verify FPS is logged every second (when ShowFPS is true)
7. Verify FPS is approximately stable (55-65 FPS with VSync)

## Expected Results
- Delta time is calculated correctly between frames
- Delta time is positive and reasonable (< 1 second)
- Frame counter resets every second
- FPS counter logs correctly when enabled
- No timing drift over long periods
- Game loop runs smoothly without stuttering

## Priority
High

## Related
- PRD Section: 6.2 Main Game Loop, 7.1 Milestone 1 (game loop with timing)
- ADR: ADR-002-architecture.md
- Code: `/Users/borisglebov/git/Faultbox/midgard-ro/internal/game/game.go::Run()`
- Test: None (runtime behavior)

## Performance Expectations
- With VSync enabled: ~60 FPS (or display refresh rate)
- With VSync disabled: FPS should be much higher
- Delta time should be consistent frame-to-frame
- No sudden spikes in delta time (indicates freeze)

## Manual Verification
1. Run with `show_fps: true` in config
2. Observe console output showing FPS every second
3. Verify FPS is stable and within expected range
4. Verify delta time is logged and looks reasonable
