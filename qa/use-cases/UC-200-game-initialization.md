# UC-200: Game Initialization

## Description
Tests the main game initialization sequence, which creates and wires up all subsystems (window, renderer, input, config, logger).

## Preconditions
- Valid config file or default config
- SDL2 libraries installed
- OpenGL 4.1+ drivers available

## Test Steps
1. Load configuration via `config.Load()`
2. Initialize logger via `logger.Init()`
3. Call `game.New(config)`
4. Verify window is created successfully
5. Verify renderer is created successfully
6. Verify input handler is created successfully
7. Verify no errors are returned
8. Verify game state is set to `running = false` (not started yet)

## Expected Results
- Configuration loads without errors
- Logger initializes correctly
- Window appears on screen
- OpenGL context is created
- Renderer is initialized with shaders
- Input system is ready
- No crashes or panics during initialization
- Resources can be cleaned up with `game.Close()`

## Priority
Critical

## Related
- PRD Section: 6.1 High-Level Components, 6.2 Main Game Loop
- ADR: ADR-002-architecture.md
- Code: `/Users/borisglebov/git/Faultbox/midgard-ro/internal/game/game.go::New()`
- Code: `/Users/borisglebov/git/Faultbox/midgard-ro/cmd/client/main.go`
- Test: None (integration test)

## Initialization Order
CRITICAL - Must happen in this order:
1. Config loading
2. Logger initialization
3. Window creation (creates OpenGL context)
4. Renderer creation (requires OpenGL context)
5. Input handler creation

## Failure Cases
- If window creation fails → renderer should not be created
- If renderer creation fails → window should be cleaned up
- All failures should be gracefully handled with proper error messages
