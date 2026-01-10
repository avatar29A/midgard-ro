# UC-022: Logger Structured Fields

## Description
Tests structured logging with zap fields. Structured logging allows for better log analysis and filtering compared to plain text.

## Preconditions
- Logger is initialized

## Test Steps

### String Fields
1. Call `logger.Info("user login", zap.String("username", "testuser"))`
2. Verify output includes key-value pair: `username=testuser`

### Int Fields
1. Call `logger.Info("window created", zap.Int("width", 1280), zap.Int("height", 720))`
2. Verify output includes: `width=1280 height=720`

### Error Fields
1. Create error: `err := fmt.Errorf("connection failed")`
2. Call `logger.Error("network error", zap.Error(err))`
3. Verify output includes error message

### Multiple Fields
1. Call `logger.Info("renderer init", zap.String("version", "4.1"), zap.Int("width", 1280))`
2. Verify all fields are present in output

## Expected Results
- All field types are logged correctly
- Fields are formatted as key=value pairs
- Multiple fields on same line work correctly
- Field values are properly escaped if needed

## Priority
Medium

## Related
- PRD Section: 6.2 Main Game Loop
- ADR: ADR-003-configuration-and-logging.md
- Code: `/Users/borisglebov/git/Faultbox/midgard-ro/internal/logger/logger.go`
- Test: None (manual verification)

## Use Cases
- Debugging with context
- Performance metrics
- Error tracking with details
- Audit logging
