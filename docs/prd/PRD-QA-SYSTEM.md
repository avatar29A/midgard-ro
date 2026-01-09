# PRD: QA AI Automation System

**Version**: 1.0  
**Status**: Draft  
**Date**: January 9, 2025  
**Authors**: Boris (CEO/Game Designer), Ilon (CTO)

---

## 1. Overview

### 1.1 Purpose

This document defines requirements for an AI-powered Quality Assurance system integrated into the OpenRO game client. The system enables automated testing through an event-driven architecture that unifies player input, AI agents, bots, and test runners.

### 1.2 Goals

| Goal | Description | Priority |
|------|-------------|----------|
| **Unified Event System** | Single event architecture for player, AI, bots, and tests | P0 |
| **Automated E2E Testing** | Use case-driven end-to-end test execution | P0 |
| **Visual Verification** | Screenshot-based validation with AI analysis | P1 |
| **Bot Support** | Headless game mode for automated agents | P1 |
| **Development Loop** | SPEC → DEV → TEST → VERIFY cycle integration | P0 |

### 1.3 Non-Goals (Q1)

- Real-time video analysis
- Distributed test execution
- Performance/load testing
- Fully automated CI/CD integration (manual trigger is acceptable)

---

## 2. System Architecture

### 2.1 High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                    QA AI Automation System                       │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  EVENT PRODUCERS                                                │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐           │
│  │  Player  │ │  Claude  │ │    QA    │ │   Bot    │           │
│  │  Input   │ │  Agent   │ │  Runner  │ │(Headless)│           │
│  └────┬─────┘ └────┬─────┘ └────┬─────┘ └────┬─────┘           │
│       │            │            │            │                   │
│       └────────────┴─────┬──────┴────────────┘                   │
│                          ▼                                       │
│                   ┌─────────────┐                                │
│                   │  EVENT BUS  │                                │
│                   └──────┬──────┘                                │
│                          │                                       │
│       ┌──────────────────┼──────────────────┐                   │
│       ▼                  ▼                  ▼                   │
│  ┌─────────┐      ┌─────────┐      ┌─────────┐                 │
│  │  Game   │      │ Network │      │   QA    │                 │
│  │  Logic  │      │ Client  │      │Validator│                 │
│  └─────────┘      └─────────┘      └─────────┘                 │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

### 2.2 Core Principle

**One event, multiple sources, multiple consumers.**

The same `MoveTo` event can be triggered by:
- Player clicking on the map
- QA test runner executing a use case step
- Claude agent via external API
- Bot AI making decisions

The game logic processes ALL events identically, regardless of source.

---

## 3. Functional Requirements

### 3.1 Event System

#### FR-EVT-001: Event Bus
- **SHALL** provide publish/subscribe mechanism for game events
- **SHALL** support multiple handlers per event type
- **SHALL** maintain event history for debugging/replay
- **SHALL** identify event source (player, agent, test, bot, system)

#### FR-EVT-002: Core Events
The system **SHALL** support these event categories:

| Category | Events |
|----------|--------|
| **Movement** | MoveTo, MoveStarted, MoveCompleted, MoveFailed |
| **Combat** | Attack, AttackStarted, CastSkill, SkillCasted |
| **Interaction** | InteractNPC, DialogOpened, DialogOption, UseItem |
| **State** | PlayerMoved, InventoryChanged, QuestUpdated |
| **Entity** | EntitySpawned, EntityDespawned |

#### FR-EVT-003: Event Metadata
Each event **SHALL** contain:
- Event type identifier
- Timestamp
- Source (player/agent/test/bot/system/network)
- Event-specific payload

### 3.2 External API (Debug/Test Interface)

#### FR-API-001: Protocol
- **SHALL** expose JSON-RPC over HTTP interface
- **SHALL** be disabled by default, enabled via config flag
- **SHALL** listen on configurable port (default: 7890)

#### FR-API-002: Commands
The API **SHALL** support:

| Method | Description |
|--------|-------------|
| `game.moveTo` | Publish MoveTo event |
| `game.attack` | Publish Attack event |
| `game.castSkill` | Publish CastSkill event |
| `game.useItem` | Publish UseItem event |
| `game.interactNPC` | Publish InteractNPC event |
| `game.dialogOption` | Select dialog option |

#### FR-API-003: Queries
The API **SHALL** support:

| Method | Description |
|--------|-------------|
| `state.getPlayer` | Get player position, stats, state |
| `state.getInventory` | Get inventory contents |
| `state.getQuests` | Get active quests and progress |
| `state.getNearbyEntities` | Get entities within radius |
| `state.getEventHistory` | Get recent events |

#### FR-API-004: Snapshots
The API **SHALL** support:

| Method | Description |
|--------|-------------|
| `debug.takeSnapshot` | Capture screenshot + game state |
| `debug.getSnapshot` | Retrieve specific snapshot |
| `debug.listSnapshots` | List available snapshots |

### 3.3 Use Case System

#### FR-UC-001: Use Case Format
- **SHALL** define use cases in YAML format
- **SHALL** support preconditions, steps, and validations
- **SHALL** support checkpoint markers for snapshots

#### FR-UC-002: Use Case Structure
```yaml
name: "Use Case Name"
description: "Description"
version: "1.0"

preconditions:
  - condition_type: value

steps:
  - id: "step_id"
    action: "action_type"
    params:
      key: value
    checkpoint: true/false
    expected:
      assertion_type: value
    timeout: "duration"

validation:
  visual_checks:
    - step: "step_id"
      check: "description for AI"
```

#### FR-UC-003: Supported Actions
| Action | Description |
|--------|-------------|
| `move_to` | Move player to coordinates |
| `interact_npc` | Talk to NPC |
| `dialog_option` | Select dialog option |
| `use_item` | Use item from inventory |
| `attack` | Attack target |
| `cast_skill` | Use skill |
| `wait` | Wait for condition |
| `hunt_monster` | Kill N monsters (composite) |

### 3.4 QA Runner

#### FR-QA-001: Test Execution
- **SHALL** parse use case YAML files
- **SHALL** execute steps sequentially
- **SHALL** wait for step completion before proceeding
- **SHALL** respect timeouts per step

#### FR-QA-002: Checkpoints
- **SHALL** capture snapshot at checkpoint steps
- **SHALL** store screenshots in organized directory structure
- **SHALL** store game state alongside screenshots

#### FR-QA-003: Validation
- **SHALL** validate expected conditions after each step
- **SHALL** support state-based assertions (position, inventory, quests)
- **SHALL** support event-based assertions (specific event occurred)
- **SHALL** collect visual checks for AI analysis

### 3.5 Visual Verification

#### FR-VIS-001: Screenshot Capture
- **SHALL** capture game window on demand
- **SHALL** capture at checkpoint steps automatically
- **SHALL** save as PNG format
- **SHALL** include timestamp and step ID in filename

#### FR-VIS-002: AI Analysis Preparation
- **SHALL** generate analysis context for each checkpoint
- **SHALL** include: screenshot path, expected visual state, game state
- **SHALL** output in format suitable for Claude analysis

#### FR-VIS-003: Report Generation
- **SHALL** generate Markdown reports
- **SHALL** include: test summary, step results, screenshots, findings
- **SHALL** embed screenshot references

### 3.6 Bot Support (Headless Mode)

#### FR-BOT-001: Headless Execution
- **SHALL** support running game without renderer
- **SHALL** process events identically to normal mode
- **SHALL** support network connection to server

#### FR-BOT-002: Bot Integration
- **SHALL** allow bot AI to publish events
- **SHALL** provide game state queries for bot decisions
- **SHALL** support configurable bot behaviors

---

## 4. Non-Functional Requirements

### 4.1 Performance

| Requirement | Target |
|-------------|--------|
| Event publish latency | < 1ms |
| Snapshot capture time | < 100ms |
| API response time | < 50ms |

### 4.2 Reliability

- Event bus **SHALL NOT** drop events under normal operation
- API **SHALL** handle malformed requests gracefully
- Test runner **SHALL** continue after non-fatal step failures

### 4.3 Security

- Debug API **SHALL** only bind to localhost by default
- Debug API **SHALL** be disabled in release builds
- No authentication required for localhost (development tool)

### 4.4 Usability

- Use case YAML **SHALL** be human-readable
- Reports **SHALL** be viewable in any Markdown reader
- Errors **SHALL** include actionable information

---

## 5. User Stories

### 5.1 Developer Testing

**As a developer**, I want to define a quest flow as a use case, so that I can automatically verify the quest works correctly after code changes.

**Acceptance Criteria:**
- Can write use case in YAML
- Can run use case with single command
- Get pass/fail result with details

### 5.2 Visual Regression

**As a developer**, I want to capture screenshots at key moments, so that I can verify visual appearance hasn't regressed.

**Acceptance Criteria:**
- Screenshots captured automatically at checkpoints
- Screenshots organized by test run
- Can compare with previous runs

### 5.3 AI-Assisted Verification

**As a QA tester**, I want to use Claude to analyze screenshots, so that I can verify complex visual states without manual inspection.

**Acceptance Criteria:**
- Report includes visual check descriptions
- Screenshots accessible alongside descriptions
- Can paste to Claude for analysis

### 5.4 Bot Development

**As a developer**, I want to run the game in headless mode, so that I can develop and test bot AI without graphics overhead.

**Acceptance Criteria:**
- Game runs without window
- Bot can publish events
- Bot can query game state

### 5.5 Claude Agent Integration

**As an AI agent (Claude)**, I want to control the game via API, so that I can perform automated testing or demonstrate gameplay.

**Acceptance Criteria:**
- API accepts standard game commands
- Can query game state
- Can receive event notifications

---

## 6. Directory Structure

```
midgard-ro/
├── internal/
│   └── debug/                    # Debug/Test API
│       ├── api.go               # API interface definition
│       ├── rpc_server.go        # JSON-RPC server
│       ├── snapshot.go          # Snapshot management
│       └── screenshot.go        # Screenshot capture
│
├── pkg/
│   └── events/                  # Event system
│       ├── events.go            # Event definitions
│       ├── bus.go               # Event bus implementation
│       └── bus_test.go          # Tests
│
├── qa/                          # QA System
│   ├── usecases/               # Test scenarios
│   │   └── *.yaml
│   ├── specs/                  # Visual specifications
│   │   └── *.md
│   ├── captures/               # Test artifacts (.gitignore)
│   │   └── YYYY-MM-DD/
│   ├── reports/                # Generated reports
│   │   └── *.md
│   └── runner/                 # Test runner
│       ├── runner.go
│       ├── validator.go
│       └── reporter.go
│
├── cmd/
│   ├── client/                 # Normal game client
│   └── bot/                    # Headless bot client
│
└── docs/
    ├── prd/
    │   └── PRD-QA-SYSTEM.md   # This document
    └── adr/
        ├── ADR-004-event-architecture.md
        └── ADR-005-qa-automation.md
```

---

## 7. Integration with Development Workflow

### 7.1 Development Loop

```
┌─────────────────────────────────────────────────────────────┐
│                    Development Loop                          │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  1. SPEC                                                    │
│     └── Write/update use case YAML                          │
│                                                              │
│  2. IMPLEMENT                                               │
│     └── Write code with Claude Code assistance              │
│                                                              │
│  3. UNIT TEST                                               │
│     └── go test ./...                                       │
│                                                              │
│  4. RUN USE CASE                                            │
│     └── qa-runner run usecases/my-feature.yaml             │
│                                                              │
│  5. VERIFY                                                  │
│     ├── Check automated assertions                          │
│     └── Review screenshots with Claude (visual checks)      │
│                                                              │
│  6. ITERATE                                                 │
│     └── If failed, go to step 2                            │
│                                                              │
│  7. COMMIT                                                  │
│     └── All tests pass, commit changes                      │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### 7.2 Manual Steps (Q1)

For Q1, these steps remain manual:
- Running qa-runner command
- Pasting screenshots to Claude for visual analysis
- Reviewing and acting on reports

### 7.3 Future Automation (Post-Q1)

Potential future enhancements:
- CI/CD integration (run on PR)
- Direct Claude API calls for visual analysis
- MCP protocol for richer agent integration
- Automatic regression detection

---

## 8. Example Use Case

```yaml
# qa/usecases/quest-elder-trunk.yaml

name: "Elder Trunk Quest"
description: |
  Player finds Quest Master Hans in Prontera, accepts the Elder Trunk quest,
  travels to prt_fild01, kills Elder Willows to collect 100 trunks,
  returns to Hans to complete the quest.
version: "1.0"
author: "Boris"
created: "2025-01-09"

preconditions:
  - player_level: ">= 10"
  - current_map: "prontera"
  - inventory_slots_free: ">= 10"

steps:
  - id: "navigate_to_npc"
    description: "Walk to Quest Master Hans"
    action: "move_to"
    params:
      map: "prontera"
      x: 156
      y: 187
    checkpoint: true
    timeout: "30s"
    expected:
      position_near:
        x: 156
        y: 187
        radius: 3

  - id: "verify_npc_present"
    description: "Confirm NPC is visible"
    action: "wait"
    params:
      condition: "entity_nearby"
      entity_type: "npc"
      entity_name: "Quest Master Hans"
      radius: 5
    timeout: "5s"
    expected:
      entity_found: true

  - id: "talk_to_npc"
    description: "Initiate conversation with Hans"
    action: "interact_npc"
    params:
      npc_name: "Quest Master Hans"
    expected:
      dialog_opened: true

  - id: "accept_quest"
    description: "Accept the Elder Trunk quest"
    action: "dialog_option"
    params:
      option_index: 0  # "Accept Quest"
    expected:
      quest_started: "elder_trunk_100"

  - id: "travel_to_field"
    description: "Travel to hunting grounds"
    action: "move_to"
    params:
      map: "prt_fild01"
      x: 200
      y: 150
    checkpoint: true
    timeout: "60s"
    expected:
      current_map: "prt_fild01"

  - id: "hunt_monsters"
    description: "Kill Elder Willows and collect trunks"
    action: "hunt_monster"
    params:
      monster_name: "Elder Willow"
      kill_count: 100
      collect_item: "Trunk"
    checkpoint: true
    timeout: "10m"
    expected:
      inventory_contains:
        item: "Trunk"
        count: ">= 100"

  - id: "return_to_npc"
    description: "Return to Quest Master Hans"
    action: "move_to"
    params:
      map: "prontera"
      x: 156
      y: 187
    timeout: "60s"

  - id: "complete_quest"
    description: "Turn in quest items"
    action: "interact_npc"
    params:
      npc_name: "Quest Master Hans"
    checkpoint: true
    expected:
      quest_completed: "elder_trunk_100"
      experience_gained: "> 0"

validation:
  visual_checks:
    - step: "navigate_to_npc"
      description: "NPC sprite 'Quest Master Hans' should be visible near player"
    
    - step: "travel_to_field"
      description: "Map should show field terrain with trees and grass"
    
    - step: "hunt_monsters"
      description: "Combat effects visible, item drops on ground or in inventory"
    
    - step: "complete_quest"
      description: "Quest completion notification visible on screen"

metadata:
  estimated_duration: "15m"
  difficulty: "easy"
  tags: ["quest", "combat", "npc"]
```

---

## 9. Success Metrics

| Metric | Target | Measurement |
|--------|--------|-------------|
| Use case coverage | 80% of Q1 features | Count of use cases vs features |
| Test execution time | < 5min per use case | Timer in runner |
| False positive rate | < 5% | Manual review of failures |
| Developer adoption | Used for all new features | Process compliance |

---

## 10. Risks and Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| Event system adds overhead | Performance degradation | Benchmark, optimize critical path |
| Visual checks too subjective | Unreliable results | Clear specifications, multiple checks |
| Use case maintenance burden | Outdated tests | Tie to feature PRDs |
| Claude API cost | Budget overrun | Manual analysis for Q1 |

---

## 11. Timeline

| Phase | Scope | Target |
|-------|-------|--------|
| **Phase 1** | Event bus + basic events | Week 1 |
| **Phase 2** | Debug API (JSON-RPC) | Week 2 |
| **Phase 3** | Screenshot capture | Week 2 |
| **Phase 4** | Use case runner (basic) | Week 3 |
| **Phase 5** | Report generation | Week 3 |
| **Phase 6** | First real use cases | Week 4 |

---

## 12. Open Questions

1. **Event Persistence**: Should we persist events to file for long-term replay/analysis?
2. **Network Mocking**: Should the QA system support mocked network responses?
3. **Parallel Tests**: Is parallel test execution needed for Q1?

---

## 13. References

- ADR-004: Event-Driven Architecture
- ADR-005: QA Automation System
- WORKFLOW.md: Development workflow guide

---

*Document Status: Draft - Pending Review*
