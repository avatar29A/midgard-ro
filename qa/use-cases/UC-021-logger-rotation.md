# UC-021: Logger File Rotation

## Description
Tests log file rotation functionality using lumberjack. Ensures logs don't grow indefinitely and old logs are managed properly.

## Preconditions
- Filesystem access for creating log files

## Test Steps
1. Create `FileConfig` with `MaxSizeMB: 1`, `MaxBackups: 3`
2. Call `logger.InitWithFileConfig("info", fileCfg, false)`
3. Write enough log messages to exceed 1MB
4. Verify that log file is rotated (new file created)
5. Verify that old log files are kept (up to MaxBackups)
6. Write more logs to exceed MaxBackups
7. Verify that oldest files are deleted

## Expected Results
- Log files rotate when size limit is reached
- Old log files are preserved up to MaxBackups count
- Rotated files have timestamps in their names
- Oldest files are deleted when MaxBackups is exceeded
- No loss of log data during rotation

## Priority
Medium

## Related
- PRD Section: 6.2 Main Game Loop
- ADR: ADR-003-configuration-and-logging.md
- Code: `/Users/borisglebov/git/Faultbox/midgard-ro/internal/logger/logger.go::InitWithFileConfig()`
- Test: `/Users/borisglebov/git/Faultbox/midgard-ro/internal/logger/logger_test.go::TestLogRotation`

## Configuration Options
- `MaxSizeMB`: Maximum size in megabytes before rotation
- `MaxBackups`: Maximum number of old log files to keep
- `MaxAgeDays`: Maximum days to keep old log files
- `Compress`: Whether to compress rotated files
