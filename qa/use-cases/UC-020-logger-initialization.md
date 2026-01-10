# UC-020: Logger Initialization

## Description
Tests logger initialization with different configurations (log levels, console/file output). The logger is a critical component used throughout the application.

## Preconditions
- None (logger is self-contained)

## Test Steps

### Console Output Only
1. Call `logger.Init("info", "")` (empty log file path)
2. Verify `logger.Log` is not nil
3. Verify `logger.Sugar` is not nil
4. Call `logger.Info("test message")`
5. Verify message appears on console

### File Output
1. Call `logger.Init("debug", "test.log")`
2. Verify log file is created
3. Call `logger.Info("test message")`
4. Read log file and verify message is present

### Different Log Levels
1. Call `logger.Init("debug", "")` and verify Debug messages appear
2. Call `logger.Init("info", "")` and verify Info messages appear but Debug do not
3. Call `logger.Init("warn", "")` and verify only Warn and Error appear
4. Call `logger.Init("error", "")` and verify only Error and Fatal appear

## Expected Results
- Logger initializes without errors
- Log level filtering works correctly
- Console output is formatted with colors
- File output is formatted without colors
- Both outputs can be enabled simultaneously

## Priority
High

## Related
- PRD Section: 6.2 Main Game Loop (logging)
- ADR: ADR-003-configuration-and-logging.md
- Code: `/Users/borisglebov/git/Faultbox/midgard-ro/internal/logger/logger.go::Init()`
- Test: `/Users/borisglebov/git/Faultbox/midgard-ro/internal/logger/logger_test.go::TestLogLevels`

## Use Cases
- Application startup logging
- Debug information during development
- Error logging for troubleshooting
- Performance monitoring
