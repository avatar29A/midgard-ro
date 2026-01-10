# QA Documentation

This directory contains all Quality Assurance documentation and resources for the Midgard RO project.

## Directory Structure

```
qa/
├── use-cases/        # Use case documentation for features
├── reports/          # QA test reports and verification results
└── README.md         # This file
```

## Use Cases

Use cases are stored in `use-cases/` and document the expected behavior and testing procedures for implemented features.

### Use Case Format

Each use case follows this standard format:

```markdown
# UC-[NUMBER]: [Title]

## Description
[Brief description of what this use case tests]

## Preconditions
- [List of required conditions before test]
- [e.g., "GRF archive file exists at data/test.grf"]
- [e.g., "SDL2 libraries are installed"]

## Test Steps
1. [Step 1 - clear, executable action]
2. [Step 2 - clear, executable action]
3. [Step 3 - clear, executable action]

## Expected Results
- [What should happen after each step or overall]
- [Be specific and measurable]

## Priority
[Critical/High/Medium/Low]

## Related
- PRD Section: [reference to docs/prd/PRD.md section]
- ADR: [reference to relevant ADR if applicable]
- Code: [relevant file paths]
- Test: [path to automated test if exists]
```

### Use Case Numbering

- UC-001 to UC-099: Core infrastructure (GRF, math, logging, config)
- UC-100 to UC-199: Engine layer (window, renderer, input, audio)
- UC-200 to UC-299: Game layer (game loop, states, entities, world)
- UC-300 to UC-399: Network layer (client, packets)
- UC-400 to UC-499: Asset layer (asset loading, caching)

### Priority Levels

- **Critical**: Core functionality that blocks all other features
- **High**: Essential features for MVP
- **Medium**: Important but not blocking
- **Low**: Nice-to-have or edge cases

## Test Reports

Test reports are stored in `reports/` and document test execution results.

### Report Types

1. **Regression Test Report**: Results from running a suite of use cases
2. **PR Verification Report**: QA verification of a pull request
3. **PR Reverification Report**: Re-testing after fixes

### Regression Test Report Format

```markdown
# Regression Test Report
**Date:** [YYYY-MM-DD]
**Scope:** [What was tested - e.g., "Full pkg/ layer regression"]
**Tested By:** Manual QA Agent / [Name]

## Summary
- Total Cases: [N]
- Passed: [N]
- Failed: [N]
- Blocked: [N]

## Test Environment
- OS: [macOS 14.6 / Linux / Windows]
- Go Version: [1.22.x]
- SDL2 Version: [if applicable]

## Results

| UC ID | Title | Status | Notes |
|-------|-------|--------|-------|
| UC-001 | ... | PASS | ... |
| UC-002 | ... | FAIL | See details below |

## Failed Cases Details

### UC-XXX: [Title]
**Failure Reason:** [What went wrong]
**Steps to Reproduce:**
1. [Step that failed]

**Expected:** [What should have happened]
**Actual:** [What actually happened]
**Severity:** [Critical/High/Medium/Low]

## Recommendations
[Any observations or suggestions for improvements]
```

### PR Verification Report Format

```markdown
# QA Verification Report - PR #[NUMBER]
**Date:** [YYYY-MM-DD]
**PR Title:** [Title]
**PR Link:** [GitHub URL]
**Verified By:** Manual QA Agent / [Name]

## Verification Status: [PASS/FAIL/BLOCKED]

## Scope
[What this PR implements/changes]

## Tested Use Cases
| UC ID | Status | Notes |
|-------|--------|-------|
| UC-XXX | PASS | ... |

## Bugs Found

### BUG-[N]: [Title]
**Severity:** [Critical/High/Medium/Low]
**Steps to Reproduce:**
1. [Step 1]
2. [Step 2]

**Expected Result:** [What should happen]
**Actual Result:** [What actually happens]
**Evidence:** [Code reference, error message, screenshot reference]

## Code Review Notes
- [Observations about code quality, architecture, etc.]

## Recommendations
[Suggestions for fixes or improvements]

## Checklist
- [ ] All CI checks pass
- [ ] Code follows project conventions (see CLAUDE.md)
- [ ] Tests cover new functionality
- [ ] Error handling is complete
- [ ] Documentation updated if needed
- [ ] No new dependencies without discussion
- [ ] Layer dependencies respected (pkg/ has no internal imports, etc.)
```

## Running Tests

### Automated Unit Tests

```bash
# Run all tests
go test ./...

# Run with verbose output
go test -v ./...

# Run specific package
go test -v ./pkg/grf/...

# Run with coverage
go test -cover ./...

# Run with race detection
go test -race ./...
```

### Manual Use Case Testing

1. Navigate to `qa/use-cases/`
2. Select the use case to test
3. Follow the test steps exactly as written
4. Document results in a test report
5. Store report in `qa/reports/` with date: `YYYY-MM-DD-[description].md`

## Best Practices

### Writing Good Use Cases

1. **Be Specific**: Write clear, unambiguous steps
2. **Be Executable**: Anyone should be able to follow the steps
3. **Be Measurable**: Expected results should be verifiable
4. **Cover Edge Cases**: Include error scenarios, not just happy paths
5. **Keep Updated**: Update use cases when behavior changes

### Example of Good vs Bad Steps

**Bad:**
```
1. Test the GRF reader
2. Make sure it works
```

**Good:**
```
1. Open the GRF archive at `pkg/grf/testdata/test.grf`
2. Call archive.List() and verify it returns 4 files
3. Call archive.Read("data/test.txt") and verify content equals "Hello, GRF!"
4. Close the archive and verify no errors occur
```

### Testing Principles

1. **Test Early**: Write use cases alongside or before implementation
2. **Test Often**: Run regression tests before major changes
3. **Test Thoroughly**: Cover happy paths, edge cases, and error conditions
4. **Document Everything**: Keep detailed notes of what was tested and results
5. **Respect Layers**: Test each architectural layer independently when possible

## Hard-to-Test Areas

Some areas require special consideration:

### OpenGL Rendering
- Cannot easily automate visual testing
- Use manual verification with screenshots
- Document expected visual appearance in use cases
- Consider using frame buffer dumps for comparison

### SDL2 Integration
- Window creation requires display
- Input events require simulation
- Use headless testing when possible
- Document manual testing procedures

### Network Protocols
- Requires running Hercules server
- Use mock servers for unit tests
- Document server setup in use case preconditions

## QA Workflow

### For New Features

1. Read PRD section for requirements
2. Write use cases before or during implementation
3. Implement feature with unit tests
4. Execute use cases manually
5. Document any issues found

### For Pull Requests

1. Review PR description and code changes
2. Identify affected use cases
3. Run automated tests: `go test ./...`
4. Execute relevant use cases manually
5. Create verification report
6. Report bugs or approve PR

### For Releases

1. Run full regression suite (all use cases)
2. Run all automated tests with race detection
3. Test on all target platforms
4. Create regression report
5. Document known issues

## Contact

For questions about QA processes or use case writing, consult:
- CLAUDE.md for project guidelines
- docs/WORKFLOW.md for development workflow
- ADR-005-qa-automation.md for QA architecture decisions
