# ADR-005: QA Automation System

**Status**: Accepted  
**Date**: January 9, 2025  
**Decision Makers**: Boris (CEO), Ilon (CTO)

---

## Context

We need a Quality Assurance system that:

1. Enables automated end-to-end testing of game features
2. Integrates with the event-driven architecture (ADR-004)
3. Supports visual verification via AI (Claude)
4. Works with the development workflow (SPEC → DEV → TEST → VERIFY)
5. Allows future expansion to bot development and AI agent control

### Requirements Summary

| Requirement | Priority |
|-------------|----------|
| External API for game control | P0 |
| Use case-driven test execution | P0 |
| Screenshot capture at checkpoints | P0 |
| Event-based assertions | P0 |
| Report generation | P1 |
| AI-assisted visual verification | P1 |
| Headless mode for bots | P1 |

---

## Decision

We implement a **multi-layer QA automation system** with:

1. **Debug API**: JSON-RPC over HTTP for external control
2. **Use Case Runner**: YAML-driven test execution
3. **Snapshot System**: Screenshot + state capture
4. **Report Generator**: Markdown output with visual references

### Architecture Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                      QA Automation System                        │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌──────────────────┐                                           │
│  │   Use Case YAML  │ ─── Test definitions                      │
│  └────────┬─────────┘                                           │
│           │                                                      │
│           ▼                                                      │
│  ┌──────────────────┐     ┌──────────────────┐                 │
│  │   QA Runner      │────▶│   Debug API      │                 │
│  │                  │     │   (JSON-RPC)     │                 │
│  │  - Parse YAML    │     │                  │                 │
│  │  - Execute steps │     │  - game.*        │                 │
│  │  - Validate      │     │  - state.*       │                 │
│  │  - Report        │     │  - debug.*       │                 │
│  └────────┬─────────┘     └────────┬─────────┘                 │
│           │                        │                            │
│           │                        ▼                            │
│           │               ┌──────────────────┐                 │
│           │               │   Event Bus      │                 │
│           │               └────────┬─────────┘                 │
│           │                        │                            │
│           ▼                        ▼                            │
│  ┌──────────────────┐     ┌──────────────────┐                 │
│  │   Snapshot Mgr   │     │   Game Engine    │                 │
│  │                  │     │                  │                 │
│  │  - Screenshots   │◀────│  - Process events│                 │
│  │  - Game state    │     │  - Update state  │                 │
│  │  - Event history │     │  - Publish results│                │
│  └────────┬─────────┘     └──────────────────┘                 │
│           │                                                      │
│           ▼                                                      │
│  ┌──────────────────┐                                           │
│  │   Report Gen     │ ─── Markdown + screenshots                │
│  └──────────────────┘                                           │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

---

## Implementation Details

### 1. Debug API (JSON-RPC over HTTP)

#### Protocol Choice

| Option | Pros | Cons | Decision |
|--------|------|------|----------|
| JSON-RPC/HTTP | Simple, curl-testable, language-agnostic | Slightly verbose | ✅ Selected |
| gRPC | Fast, typed, streaming | Complex setup, requires protobuf | Rejected |
| REST | Familiar | Not ideal for commands | Rejected |
| WebSocket | Real-time | Overkill for request/response | Rejected |
| MCP | Claude-native | Newer, less mature | Future option |

#### API Design

```go
// internal/debug/rpc_server.go

type RPCServer struct {
    bus      *events.Bus
    game     *game.Game
    snapshot *SnapshotManager
    port     int
}

// Game Control Methods
type GameService struct {
    bus *events.Bus
}

func (s *GameService) MoveTo(params MoveToParams) (*Response, error)
func (s *GameService) Attack(params AttackParams) (*Response, error)
func (s *GameService) CastSkill(params CastSkillParams) (*Response, error)
func (s *GameService) UseItem(params UseItemParams) (*Response, error)
func (s *GameService) InteractNPC(params InteractParams) (*Response, error)
func (s *GameService) DialogOption(params DialogParams) (*Response, error)

// State Query Methods
type StateService struct {
    game *game.Game
}

func (s *StateService) GetPlayer() (*PlayerState, error)
func (s *StateService) GetInventory() (*InventoryState, error)
func (s *StateService) GetQuests() (*QuestState, error)
func (s *StateService) GetNearbyEntities(params RadiusParams) (*EntityList, error)
func (s *StateService) GetEventHistory(params HistoryParams) (*EventList, error)

// Debug Methods
type DebugService struct {
    snapshot *SnapshotManager
}

func (s *DebugService) TakeSnapshot(params SnapshotParams) (*SnapshotInfo, error)
func (s *DebugService) GetSnapshot(params GetSnapshotParams) (*Snapshot, error)
func (s *DebugService) ListSnapshots() (*SnapshotList, error)
```

#### Request/Response Format

```json
// Request
{
    "jsonrpc": "2.0",
    "method": "game.moveTo",
    "params": {
        "x": 156,
        "y": 187,
        "map": "prontera"
    },
    "id": 1
}

// Response
{
    "jsonrpc": "2.0",
    "result": {
        "success": true,
        "event_id": "evt_12345"
    },
    "id": 1
}
```

#### Security Configuration

```yaml
# config.yaml
debug:
  enabled: false          # Disabled by default
  api:
    port: 7890
    bind: "127.0.0.1"    # Localhost only
    timeout: 30s
```

### 2. Screenshot Capture

#### Implementation

```go
// internal/debug/screenshot.go

package debug

import (
    "image/png"
    "os/exec"
    "path/filepath"
    "time"
)

type ScreenshotManager struct {
    outputDir string
    windowID  string // Cached window ID
}

func NewScreenshotManager(outputDir string) *ScreenshotManager {
    return &ScreenshotManager{
        outputDir: outputDir,
    }
}

// Capture using macOS screencapture command
func (m *ScreenshotManager) Capture(name string) (string, error) {
    timestamp := time.Now().Format("20060102-150405")
    filename := fmt.Sprintf("%s_%s.png", name, timestamp)
    filepath := filepath.Join(m.outputDir, filename)
    
    // Capture specific window by ID (silent, no shadow)
    cmd := exec.Command("screencapture",
        "-l", m.windowID,  // Window ID
        "-x",              // Silent (no sound)
        "-o",              // No shadow
        filepath,
    )
    
    if err := cmd.Run(); err != nil {
        // Fallback: capture entire screen
        cmd = exec.Command("screencapture", "-x", filepath)
        if err := cmd.Run(); err != nil {
            return "", fmt.Errorf("screenshot failed: %w", err)
        }
    }
    
    return filepath, nil
}

// GetWindowID finds our game window
func (m *ScreenshotManager) FindWindowID(windowTitle string) error {
    // Use AppleScript to find window ID
    script := fmt.Sprintf(`
        tell application "System Events"
            set windowID to id of first window of 
                (first process whose name contains "%s")
        end tell
        return windowID
    `, windowTitle)
    
    cmd := exec.Command("osascript", "-e", script)
    output, err := cmd.Output()
    if err != nil {
        return fmt.Errorf("could not find window: %w", err)
    }
    
    m.windowID = strings.TrimSpace(string(output))
    return nil
}
```

#### Alternative: In-Engine Screenshot

For more reliable captures, we can also capture directly from OpenGL:

```go
// internal/engine/renderer/screenshot.go

func (r *Renderer) CaptureFrame() (image.Image, error) {
    width, height := r.window.GetSize()
    
    pixels := make([]byte, width*height*4)
    gl.ReadPixels(0, 0, int32(width), int32(height),
        gl.RGBA, gl.UNSIGNED_BYTE, gl.Ptr(pixels))
    
    img := image.NewRGBA(image.Rect(0, 0, width, height))
    // Flip vertically (OpenGL origin is bottom-left)
    for y := 0; y < height; y++ {
        for x := 0; x < width; x++ {
            srcIdx := ((height - 1 - y) * width + x) * 4
            dstIdx := (y * width + x) * 4
            copy(img.Pix[dstIdx:dstIdx+4], pixels[srcIdx:srcIdx+4])
        }
    }
    
    return img, nil
}
```

### 3. Snapshot System

#### Snapshot Structure

```go
// internal/debug/snapshot.go

type Snapshot struct {
    ID         string          `json:"id"`
    Timestamp  time.Time       `json:"timestamp"`
    StepID     string          `json:"step_id,omitempty"`
    
    // Visual
    Screenshot string          `json:"screenshot_path"`
    
    // Game State
    State      GameStateData   `json:"state"`
    
    // Events leading to this point
    RecentEvents []events.Event `json:"recent_events"`
}

type GameStateData struct {
    Player     PlayerSnapshot    `json:"player"`
    Map        MapSnapshot       `json:"map"`
    Inventory  []ItemSnapshot    `json:"inventory"`
    Quests     []QuestSnapshot   `json:"quests"`
    Entities   []EntitySnapshot  `json:"nearby_entities"`
    UIState    UISnapshot        `json:"ui"`
}

type PlayerSnapshot struct {
    X, Y       int    `json:"x"`
    MapName    string `json:"map"`
    HP, MaxHP  int    `json:"hp"`
    SP, MaxSP  int    `json:"sp"`
    Level      int    `json:"level"`
    Status     string `json:"status"` // "idle", "moving", "attacking"
}

type SnapshotManager struct {
    outputDir  string
    screenshots *ScreenshotManager
    game       *game.Game
    bus        *events.Bus
}

func (m *SnapshotManager) TakeSnapshot(stepID string) (*Snapshot, error) {
    // Capture screenshot
    screenshotPath, err := m.screenshots.Capture(stepID)
    if err != nil {
        return nil, err
    }
    
    // Capture game state
    state := m.captureGameState()
    
    // Get recent events
    recentEvents := m.bus.History(20)
    
    snapshot := &Snapshot{
        ID:           generateID(),
        Timestamp:    time.Now(),
        StepID:       stepID,
        Screenshot:   screenshotPath,
        State:        state,
        RecentEvents: recentEvents,
    }
    
    // Save metadata
    if err := m.saveMetadata(snapshot); err != nil {
        return nil, err
    }
    
    return snapshot, nil
}
```

### 4. Use Case Runner

#### Use Case Format

```yaml
# qa/usecases/example.yaml
name: "Example Use Case"
description: "Description of what this tests"
version: "1.0"

preconditions:
  - player_level: ">= 1"
  - current_map: "prontera"

steps:
  - id: "step_1"
    description: "What this step does"
    action: "move_to"
    params:
      x: 100
      y: 100
    checkpoint: true
    timeout: "30s"
    expected:
      position_near:
        x: 100
        y: 100
        radius: 3

validation:
  visual_checks:
    - step: "step_1"
      description: "What AI should verify in screenshot"
```

#### Runner Implementation

```go
// qa/runner/runner.go

type Runner struct {
    api        *APIClient      // Talks to debug API
    validator  *Validator
    reporter   *Reporter
}

func (r *Runner) Run(usecasePath string) (*Report, error) {
    // Parse use case
    uc, err := ParseUseCase(usecasePath)
    if err != nil {
        return nil, err
    }
    
    // Check preconditions
    if err := r.checkPreconditions(uc.Preconditions); err != nil {
        return nil, fmt.Errorf("preconditions not met: %w", err)
    }
    
    report := NewReport(uc.Name)
    
    // Execute steps
    for _, step := range uc.Steps {
        result := r.executeStep(step)
        report.AddStepResult(result)
        
        if result.Failed && step.Critical {
            break
        }
    }
    
    // Add visual checks for AI review
    report.VisualChecks = uc.Validation.VisualChecks
    
    return report, nil
}

func (r *Runner) executeStep(step Step) StepResult {
    result := StepResult{
        StepID:    step.ID,
        StartTime: time.Now(),
    }
    
    // Execute action via API
    resp, err := r.executeAction(step.Action, step.Params)
    if err != nil {
        result.Failed = true
        result.Error = err.Error()
        return result
    }
    
    // Wait for completion with timeout
    ctx, cancel := context.WithTimeout(context.Background(), step.Timeout)
    defer cancel()
    
    if err := r.waitForExpected(ctx, step.Expected); err != nil {
        result.Failed = true
        result.Error = err.Error()
    }
    
    // Take checkpoint if requested
    if step.Checkpoint {
        snapshot, _ := r.api.TakeSnapshot(step.ID)
        result.Snapshot = snapshot
    }
    
    result.EndTime = time.Now()
    return result
}
```

### 5. Report Generation

#### Report Format

```go
// qa/runner/reporter.go

type Report struct {
    Name          string
    Timestamp     time.Time
    Duration      time.Duration
    Passed        bool
    Steps         []StepResult
    VisualChecks  []VisualCheck
}

func (r *Reporter) GenerateMarkdown(report *Report) string {
    var sb strings.Builder
    
    sb.WriteString(fmt.Sprintf("# Test Report: %s\n\n", report.Name))
    sb.WriteString(fmt.Sprintf("**Date**: %s\n", report.Timestamp.Format(time.RFC3339)))
    sb.WriteString(fmt.Sprintf("**Duration**: %s\n", report.Duration))
    sb.WriteString(fmt.Sprintf("**Status**: %s\n\n", statusEmoji(report.Passed)))
    
    sb.WriteString("## Step Results\n\n")
    sb.WriteString("| Step | Status | Duration | Notes |\n")
    sb.WriteString("|------|--------|----------|-------|\n")
    
    for _, step := range report.Steps {
        sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n",
            step.StepID,
            statusEmoji(!step.Failed),
            step.Duration(),
            step.Error,
        ))
    }
    
    sb.WriteString("\n## Checkpoints\n\n")
    for _, step := range report.Steps {
        if step.Snapshot != nil {
            sb.WriteString(fmt.Sprintf("### %s\n\n", step.StepID))
            sb.WriteString(fmt.Sprintf("![%s](%s)\n\n", 
                step.StepID, step.Snapshot.Screenshot))
            sb.WriteString(fmt.Sprintf("**Player Position**: (%d, %d) on %s\n\n",
                step.Snapshot.State.Player.X,
                step.Snapshot.State.Player.Y,
                step.Snapshot.State.Player.MapName,
            ))
        }
    }
    
    sb.WriteString("## Visual Checks (For AI Review)\n\n")
    for _, check := range report.VisualChecks {
        sb.WriteString(fmt.Sprintf("- **Step %s**: %s\n", 
            check.Step, check.Description))
    }
    
    return sb.String()
}
```

#### Example Output

```markdown
# Test Report: Elder Trunk Quest

**Date**: 2025-01-09T14:30:00Z
**Duration**: 3m 45s
**Status**: ✅ PASSED

## Step Results

| Step | Status | Duration | Notes |
|------|--------|----------|-------|
| navigate_to_npc | ✅ | 5.2s | |
| verify_npc_present | ✅ | 0.3s | |
| talk_to_npc | ✅ | 1.1s | |
| accept_quest | ✅ | 0.5s | |
| travel_to_field | ✅ | 12.3s | |
| hunt_monsters | ✅ | 2m 30s | |
| return_to_npc | ✅ | 10.1s | |
| complete_quest | ✅ | 1.2s | |

## Checkpoints

### navigate_to_npc

![navigate_to_npc](captures/2025-01-09/navigate_to_npc_143005.png)

**Player Position**: (156, 187) on prontera

### hunt_monsters

![hunt_monsters](captures/2025-01-09/hunt_monsters_143235.png)

**Player Position**: (200, 150) on prt_fild01
**Inventory**: Trunk x100

## Visual Checks (For AI Review)

- **Step navigate_to_npc**: NPC sprite 'Quest Master Hans' should be visible near player
- **Step travel_to_field**: Map should show field terrain with trees and grass
- **Step hunt_monsters**: Combat effects visible, item drops on ground or in inventory
- **Step complete_quest**: Quest completion notification visible on screen
```

---

## Directory Structure

```
midgard-ro/
├── internal/
│   └── debug/                   # Debug subsystem
│       ├── api.go              # API interface
│       ├── rpc_server.go       # JSON-RPC implementation
│       ├── screenshot.go       # Screenshot capture
│       ├── snapshot.go         # Snapshot management
│       └── debug.go            # Debug mode initialization
│
├── qa/                          # QA system (separate from game code)
│   ├── runner/                 # Test runner
│   │   ├── runner.go          # Main runner
│   │   ├── parser.go          # YAML parser
│   │   ├── validator.go       # Assertion checker
│   │   ├── reporter.go        # Report generator
│   │   └── api_client.go      # Debug API client
│   │
│   ├── usecases/              # Test definitions
│   │   ├── login-flow.yaml
│   │   ├── character-creation.yaml
│   │   ├── prontera-walk.yaml
│   │   └── quest-elder-trunk.yaml
│   │
│   ├── specs/                 # Visual specifications
│   │   ├── prontera-fountain.md
│   │   ├── login-screen.md
│   │   └── character-select.md
│   │
│   ├── captures/              # Test artifacts (gitignored)
│   │   └── 2025-01-09/
│   │       ├── step_001.png
│   │       └── step_001.json
│   │
│   └── reports/               # Generated reports
│       └── 2025-01-09-quest-test.md
│
├── cmd/
│   ├── client/               # Normal game
│   ├── bot/                  # Headless bot
│   └── qa-runner/            # QA CLI tool
│       └── main.go
```

---

## Usage Examples

### Running a Test

```bash
# Start game with debug API enabled
./midgard --debug.enabled=true --debug.api.port=7890

# In another terminal, run test
./qa-runner run qa/usecases/quest-elder-trunk.yaml

# Output:
# Running: Elder Trunk Quest
# Step navigate_to_npc: ✅ (5.2s)
# Step verify_npc_present: ✅ (0.3s)
# ...
# 
# Report saved: qa/reports/2025-01-09-elder-trunk.md
```

### Manual API Testing

```bash
# Move player
curl -X POST http://localhost:7890/rpc -d '{
    "jsonrpc": "2.0",
    "method": "game.moveTo",
    "params": {"x": 156, "y": 187},
    "id": 1
}'

# Get player state
curl -X POST http://localhost:7890/rpc -d '{
    "jsonrpc": "2.0",
    "method": "state.getPlayer",
    "params": {},
    "id": 2
}'

# Take snapshot
curl -X POST http://localhost:7890/rpc -d '{
    "jsonrpc": "2.0",
    "method": "debug.takeSnapshot",
    "params": {"name": "manual_check"},
    "id": 3
}'
```

### AI Visual Verification Workflow

1. Run test, generates report with screenshots
2. Open report in editor
3. For each visual check:
   - Open screenshot
   - Paste to Claude with check description
   - Record Claude's assessment
4. Update report with findings

---

## Future Enhancements (Post-Q1)

| Enhancement | Description | Priority |
|-------------|-------------|----------|
| **MCP Protocol** | Replace/augment JSON-RPC with MCP for Claude | Medium |
| **Direct Claude API** | Automated visual verification | Medium |
| **CI Integration** | Run tests on PR/commit | Low |
| **Parallel Tests** | Run multiple tests simultaneously | Low |
| **Network Mocking** | Test without real server | Medium |
| **Regression Detection** | Compare screenshots across runs | Medium |

---

## Consequences

### Positive

| Benefit | Description |
|---------|-------------|
| **Automated Testing** | E2E tests run without manual intervention |
| **Visual Record** | Screenshots document each test state |
| **AI Integration Ready** | Reports formatted for Claude analysis |
| **Developer Productivity** | Catch regressions early |
| **Documentation** | Use cases serve as feature documentation |

### Negative

| Drawback | Mitigation |
|----------|------------|
| **Maintenance** | Use cases need updates when features change |
| **Setup Overhead** | Initial configuration required |
| **Platform-Specific** | Screenshot capture is macOS-specific for now |

### Neutral

- Debug API adds small code footprint
- Reports require manual AI verification for Q1
- Use cases are human-readable YAML

---

## References

- ADR-004: Event-Driven Architecture
- PRD-QA-SYSTEM: Full requirements document
- WORKFLOW.md: Development process guide
- [JSON-RPC 2.0 Specification](https://www.jsonrpc.org/specification)
- [macOS screencapture documentation](https://ss64.com/mac/screencapture.html)

---

*Document Status: Accepted*
