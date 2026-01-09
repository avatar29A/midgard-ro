# Workflow Guide: Developing Midgard RO with Claude Code

**Version**: 1.0  
**Last Updated**: January 9, 2025

This document explains how to effectively work with Claude Code (Claude AI in development mode) for this project.

---

## 1. Session Management

### 1.1 Starting a Session

When you start a new Claude Code session, provide context:

```
"Hey Ilon, let's continue on midgard-ro. Last session we [brief summary]. 
Today I want to work on [specific goal]."
```

**Why This Matters**: Claude doesn't have memory between sessions. Brief context helps Claude understand where you left off.

### 1.2 Session Types

| Type | Duration | Goal |
|------|----------|------|
| **Sprint** | 2-4 hours | Complete a milestone or feature |
| **Fix** | 30-60 min | Bug fix or small improvement |
| **Review** | 30 min | Code review, refactoring |
| **Learn** | Variable | Explore concepts, Q&A |

### 1.3 Ending a Session

Ask Claude to summarize:
```
"Ilon, can you summarize what we accomplished and what's next?"
```

The summary should be saved to `docs/sessions/YYYY-MM-DD.md` for future reference.

---

## 2. Task Workflow

### 2.1 The UPID Cycle

For each feature or task, follow **UPID**:

1. **U**nderstand
   - What exactly are we building?
   - What's the expected behavior?
   - What are the edge cases?

2. **P**lan
   - Break into small steps
   - Identify dependencies
   - Estimate complexity

3. **I**mplement
   - Write code incrementally
   - Test each step
   - Commit logical chunks

4. **D**ocument
   - Update relevant docs
   - Add code comments
   - Create tests

### 2.2 Example Task Flow

**Task**: "Implement SDL3 window creation"

```
Step 1: UNDERSTAND
- Need to open a window with title, size
- Window should be resizable
- Must create OpenGL context

Step 2: PLAN
- Add SDL3 initialization to game.New()
- Create window with OpenGL flags
- Handle errors properly
- Add cleanup in game.Close()

Step 3: IMPLEMENT
[Claude writes code, you review]

Step 4: DOCUMENT
- Update CLAUDE.md if patterns changed
- Add comments explaining SDL3 setup
```

---

## 3. Code Review Protocol

### 3.1 Before Implementing

Ask Claude to explain the approach:
```
"Before we code this, explain your plan. What files will change? 
What's the general approach?"
```

### 3.2 During Implementation

Review code in chunks:
- Don't let Claude write 500 lines at once
- Ask for explanations of complex parts
- Question any magic numbers or unclear logic

### 3.3 Questions to Ask

- "Why did you choose this approach over X?"
- "What happens if this fails?"
- "Is this testable? How would we test it?"
- "Does this follow our architecture rules?"

---

## 4. Learning Integration

### 4.1 Concept Explanations

When Claude introduces new concepts, ask:
```
"Explain [concept] to me. Why is it important for our project?"
```

### 4.2 GameDev Fundamentals

Track concepts you've learned:

| Concept | Status | Notes |
|---------|--------|-------|
| Game Loop | ⏳ | Fixed timestep vs variable |
| Vertex Buffers | ❌ | |
| Shaders | ❌ | |
| Sprite Batching | ❌ | |
| Entity Component System | ❌ | |

### 4.3 Learning Sessions

Dedicate some sessions purely to learning:
```
"Today I want to understand OpenGL shaders. 
Can you explain them step by step with examples?"
```

---

## 5. File Organization

### 5.1 Session Logs

```
docs/
└── sessions/
    ├── 2025-01-09.md    # Session summaries
    ├── 2025-01-10.md
    └── ...
```

### 5.2 Research Notes

```
docs/
└── research/
    ├── grf-format.md         # GRF file format
    ├── hercules-protocol.md  # Packet docs
    ├── opengl-notes.md       # OpenGL learnings
    └── sdl3-notes.md         # SDL3 learnings
```

### 5.3 Decision Records

Create an ADR when:
- Choosing between multiple options
- Making a significant technical decision
- Changing an established pattern

```
docs/
└── adr/
    ├── ADR-001-graphics-stack.md
    ├── ADR-002-architecture.md
    └── ADR-003-[next-decision].md
```

---

## 6. Common Commands

### 6.1 Development

```bash
# Run the client
go run ./cmd/client

# Run all tests
go test ./...

# Run specific package tests
go test ./pkg/grf/...

# Run with verbose output
go test -v ./...

# Check for issues
go vet ./...

# Format code
gofmt -w .
```

### 6.2 Git Workflow

```bash
# Create feature branch
git checkout -b feature/sdl3-window

# Commit with meaningful message
git add .
git commit -m "feat(engine): add SDL3 window creation"

# Push branch
git push -u origin feature/sdl3-window
```

### 6.3 Commit Message Format

```
type(scope): description

Types:
- feat: new feature
- fix: bug fix
- docs: documentation
- refactor: code refactoring
- test: adding tests
- chore: maintenance

Examples:
- feat(renderer): add sprite batching
- fix(network): handle connection timeout
- docs(prd): update milestone 2 requirements
```

---

## 7. Problem Solving

### 7.1 When Stuck

1. **Describe the problem clearly**
   ```
   "I'm trying to X, but Y is happening. I expected Z."
   ```

2. **Share error messages**
   - Full error output
   - Stack traces if available

3. **Show what you tried**
   - What approaches failed?
   - What did you search for?

### 7.2 Debugging Together

```
"Let's debug this together. Can you add some logging to 
understand what's happening at each step?"
```

### 7.3 When Claude Seems Wrong

- Ask for clarification
- Request sources or documentation
- Test the suggestion in isolation
- Trust but verify!

---

## 8. Quality Checklist

Before considering a feature "done":

- [ ] Code compiles without warnings
- [ ] Tests pass (if applicable)
- [ ] Error cases handled
- [ ] Code follows project conventions
- [ ] Comments added for complex logic
- [ ] Related docs updated
- [ ] Committed with proper message

---

## 9. Red Flags to Watch For

### 9.1 Code Smells

- Functions longer than 50 lines
- Deep nesting (>3 levels)
- Unclear variable names
- Magic numbers without constants
- Commented-out code
- Copy-pasted code

### 9.2 Architecture Violations

- `pkg/` importing from `internal/`
- `internal/engine/` importing from `internal/game/`
- Circular dependencies
- Business logic in renderer
- Network code in entity

### 9.3 Session Red Flags

- Claude suggesting major rewrites unprompted
- Implementing features not in current milestone
- Skipping error handling "for now"
- Not writing tests

---

## 10. Useful Prompts

### For Starting Features
```
"Let's implement [feature]. First, show me the plan, 
then we'll code it step by step."
```

### For Understanding Code
```
"Walk me through this code. What does each part do?"
```

### For Refactoring
```
"This code works but feels messy. How can we improve it 
while keeping the same behavior?"
```

### For Debugging
```
"This isn't working as expected. Let's add debug output 
and trace through the logic."
```

### For Learning
```
"Explain [concept] like I'm new to programming. 
Then show me how it applies to our project."
```

---

## 11. Session Template

Use this template when starting sessions:

```markdown
# Session: [Date]

## Context
- Previous session: [what was done]
- Current milestone: [milestone number and name]
- Branch: [current git branch]

## Goals Today
1. [Goal 1]
2. [Goal 2]

## Notes
[Add notes during session]

## Completed
- [ ] [Task 1]
- [ ] [Task 2]

## Next Session
- [What to do next]
```

---

## 12. Milestone Tracking

Update this section as milestones complete:

| Milestone | Target | Status | Notes |
|-----------|--------|--------|-------|
| 1. Window & Triangle | Week 1 | 🔴 Not Started | |
| 2. Textured Rendering | Week 2 | 🔴 Not Started | |
| 3. GRF & Sprites | Week 3-4 | 🔴 Not Started | |
| 4. Map Rendering | Week 5-6 | 🔴 Not Started | |
| 5. Network Foundation | Week 7-8 | 🔴 Not Started | |
| 6. Game World | Week 9-10 | 🔴 Not Started | |
| 7. Polish | Week 11-12 | 🔴 Not Started | |

---

*Remember: The goal is learning AND building. Take time to understand what we're creating!*
