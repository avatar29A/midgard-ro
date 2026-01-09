# Contributing to Midgard RO

## Pull Request Workflow

### Creating a PR

1. Create a feature branch from `main`
2. Make your changes with clear, atomic commits
3. Push and create a PR via `gh pr create`
4. Wait for reviewers

### Review Process

Every PR requires approval from:

| Reviewer | Purpose |
|----------|---------|
| **Developer** (Boris) | Architecture, logic, code quality |
| **GitHub Copilot** | Automated code analysis |

### Handling Review Comments

**Rules for the contributor (Claude):**

1. **Don't blindly agree** - If unsure about a suggestion, ask for clarification
2. **Add comments** - Reply in the discussion thread if you need more context
3. **Wait for CI** - All checks must pass before merge
4. **Resolve discussions** - Mark completed items as resolved

### Merge Criteria

A PR can be merged when:

- [ ] All CI checks pass (Build, Test, Lint)
- [ ] Developer approval received
- [ ] Copilot review completed (if enabled)
- [ ] All discussions resolved
- [ ] No unaddressed comments

### Commands Reference

```bash
# Check PR status
gh pr checks <PR_NUMBER>

# View PR comments
gh pr view <PR_NUMBER> --comments

# View review comments (line-specific)
gh api repos/avatar29A/midgard-ro/pulls/<PR_NUMBER>/comments

# Merge when ready
gh pr merge <PR_NUMBER> --merge
```

## Code Style

See `CLAUDE.md` for coding conventions and project structure.

## CI Pipeline

All PRs trigger:
- **Build** - Compile all binaries
- **Test** - Run tests with race detector
- **Lint** - golangci-lint static analysis

See `.github/workflows/ci.yml` for details.
