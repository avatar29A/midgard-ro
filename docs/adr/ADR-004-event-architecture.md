# ADR-004: Event-Driven Architecture

**Status**: Accepted  
**Date**: January 9, 2025  
**Decision Makers**: Boris (CEO), Ilon (CTO)

---

## Context

We need an architecture that supports multiple input sources (player, AI agent, test runner, bots) interacting with the game engine uniformly. The system must:

1. Allow identical game logic regardless of input source
2. Enable automated testing through external API
3. Support headless (bot) mode without renderer
4. Facilitate future AI agent integration (Claude)
5. Keep code maintainable and decoupled

### Problem Statement

Traditional game architectures tightly couple input handling with game logic:

```go
// BAD: Tightly coupled
func (g *Game) HandleKeyPress(key int) {
    if key == KEY_W {
        g.player.Move(0, -1)  // Logic embedded in input handler
    }
}
```

This approach:
- Makes automated testing difficult
- Requires different code paths for bots vs players
- Couples rendering with logic
- Prevents external control (AI agents)

---

## Decision

**We adopt an Event-Driven Architecture using a central Event Bus.**

All game interactions are expressed as **Events**:
- Events are published by **Producers** (player input, API, bots, system)
- Events are consumed by **Handlers** (game logic, network, logger, QA)
- The game logic processes events identically regardless of source

### Core Pattern

```
┌────────────┐     ┌────────────┐     ┌────────────┐
│  Producer  │────▶│  Event Bus │────▶│  Consumer  │
└────────────┘     └────────────┘     └────────────┘

Producers:                             Consumers:
- Player Input                         - Game Logic
- External API                         - Network Client  
- Bot AI                               - Event Logger
- Test Runner                          - QA Validator
- Network (server responses)           - Audio System
```

---

## Implementation

### Event Structure

```go
// pkg/events/events.go

package events

type Event interface {
    EventType() string
    Timestamp() time.Time
    Source() EventSource
}

type EventSource string

const (
    SourcePlayer  EventSource = "player"   // Human keyboard/mouse
    SourceAgent   EventSource = "agent"    // Claude or other AI
    SourceBot     EventSource = "bot"      // Headless bot
    SourceTest    EventSource = "test"     // QA test runner
    SourceNetwork EventSource = "network"  // Server responses
    SourceSystem  EventSource = "system"   // Internal game events
)

type BaseEvent struct {
    Type string      `json:"type"`
    Time time.Time   `json:"timestamp"`
    Src  EventSource `json:"source"`
}

func NewBase(eventType string, source EventSource) BaseEvent {
    return BaseEvent{
        Type: eventType,
        Time: time.Now(),
        Src:  source,
    }
}
```

### Event Categories

#### Command Events (Input)

Events that request actions:

```go
// Movement
type MoveTo struct {
    BaseEvent
    TargetX int    `json:"target_x"`
    TargetY int    `json:"target_y"`
    MapName string `json:"map_name,omitempty"`
}

// Combat
type Attack struct {
    BaseEvent
    TargetID uint32 `json:"target_id"`
}

type CastSkill struct {
    BaseEvent
    SkillID  int    `json:"skill_id"`
    TargetID uint32 `json:"target_id,omitempty"`
    TargetX  int    `json:"target_x,omitempty"`
    TargetY  int    `json:"target_y,omitempty"`
}

// Interaction
type InteractNPC struct {
    BaseEvent
    NPCID uint32 `json:"npc_id"`
}

type UseItem struct {
    BaseEvent
    ItemID   int    `json:"item_id"`
    TargetID uint32 `json:"target_id,omitempty"`
}
```

#### Response Events (Output)

Events that report results:

```go
// Movement responses
type MoveStarted struct {
    BaseEvent
    FromX, FromY int
    ToX, ToY     int
    Path         []Point
}

type MoveCompleted struct {
    BaseEvent
    FinalX, FinalY int
}

type MoveFailed struct {
    BaseEvent
    Reason string // "blocked", "too_far", "invalid"
}

// State changes
type InventoryChanged struct {
    BaseEvent
    ItemID   int
    Quantity int
    Action   string // "add", "remove", "update"
}

type QuestUpdated struct {
    BaseEvent
    QuestID  int
    Status   string // "started", "progress", "completed"
    Progress int
}
```

### Event Bus

```go
// pkg/events/bus.go

package events

type Handler func(event Event)

type Bus struct {
    mu         sync.RWMutex
    handlers   map[string][]Handler
    allHandler []Handler  // Wildcard handlers
    history    []Event
    maxHistory int
}

func NewBus(historySize int) *Bus {
    return &Bus{
        handlers:   make(map[string][]Handler),
        history:    make([]Event, 0, historySize),
        maxHistory: historySize,
    }
}

// Subscribe to specific event type
func (b *Bus) Subscribe(eventType string, handler Handler) {
    b.mu.Lock()
    defer b.mu.Unlock()
    b.handlers[eventType] = append(b.handlers[eventType], handler)
}

// Subscribe to ALL events (logging, QA)
func (b *Bus) SubscribeAll(handler Handler) {
    b.mu.Lock()
    defer b.mu.Unlock()
    b.allHandler = append(b.allHandler, handler)
}

// Publish event to all subscribers
func (b *Bus) Publish(event Event) {
    b.mu.Lock()
    
    // Store history
    if len(b.history) >= b.maxHistory {
        b.history = b.history[1:]
    }
    b.history = append(b.history, event)
    
    // Collect handlers
    handlers := make([]Handler, 0)
    handlers = append(handlers, b.handlers[event.EventType()]...)
    handlers = append(handlers, b.allHandler...)
    b.mu.Unlock()
    
    // Execute outside lock
    for _, h := range handlers {
        h(event)
    }
}

// Get recent events for debugging/replay
func (b *Bus) History(count int) []Event {
    b.mu.RLock()
    defer b.mu.RUnlock()
    
    if count > len(b.history) {
        count = len(b.history)
    }
    
    result := make([]Event, count)
    copy(result, b.history[len(b.history)-count:])
    return result
}
```

### Integration Examples

#### Player Input → Event

```go
// internal/engine/input/input.go

type InputHandler struct {
    bus    *events.Bus
    camera *Camera
}

func (h *InputHandler) OnMapClick(screenX, screenY int) {
    worldX, worldY := h.camera.ScreenToWorld(screenX, screenY)
    
    h.bus.Publish(&events.MoveTo{
        BaseEvent: events.NewBase("move_to", events.SourcePlayer),
        TargetX:   worldX,
        TargetY:   worldY,
    })
}

func (h *InputHandler) OnAttackKey() {
    target := h.game.GetSelectedTarget()
    if target == nil {
        return
    }
    
    h.bus.Publish(&events.Attack{
        BaseEvent: events.NewBase("attack", events.SourcePlayer),
        TargetID:  target.ID,
    })
}
```

#### External API → Event

```go
// internal/debug/api_handler.go

type APIHandler struct {
    bus *events.Bus
}

func (h *APIHandler) HandleMoveTo(params MoveToParams) Response {
    h.bus.Publish(&events.MoveTo{
        BaseEvent: events.NewBase("move_to", events.SourceAgent),
        TargetX:   params.X,
        TargetY:   params.Y,
        MapName:   params.Map,
    })
    return Response{Success: true}
}
```

#### Game Logic Consumes Events

```go
// internal/game/game.go

func (g *Game) setupEventHandlers() {
    g.bus.Subscribe("move_to", g.onMoveTo)
    g.bus.Subscribe("attack", g.onAttack)
    g.bus.Subscribe("cast_skill", g.onCastSkill)
    g.bus.Subscribe("interact_npc", g.onInteractNPC)
    // ... more handlers
}

func (g *Game) onMoveTo(e events.Event) {
    evt := e.(*events.MoveTo)
    
    // Pathfinding
    path := g.pathfinder.FindPath(
        g.player.Position(),
        evt.TargetX, evt.TargetY,
    )
    
    if path == nil {
        g.bus.Publish(&events.MoveFailed{
            BaseEvent: events.NewBase("move_failed", events.SourceSystem),
            Reason:    "no_path",
        })
        return
    }
    
    // Start movement
    g.player.StartMove(path)
    
    g.bus.Publish(&events.MoveStarted{
        BaseEvent: events.NewBase("move_started", events.SourceSystem),
        FromX:     g.player.X,
        FromY:     g.player.Y,
        ToX:       evt.TargetX,
        ToY:       evt.TargetY,
        Path:      path,
    })
}
```

#### Network Client Listens

```go
// internal/network/client.go

func (c *Client) setupEventHandlers() {
    // Send movement to server when player moves
    c.bus.Subscribe("move_started", func(e events.Event) {
        evt := e.(*events.MoveStarted)
        c.SendPacket(&packets.RequestMove{
            X: evt.ToX,
            Y: evt.ToY,
        })
    })
    
    // Send attack to server
    c.bus.Subscribe("attack", func(e events.Event) {
        evt := e.(*events.Attack)
        c.SendPacket(&packets.RequestAttack{
            TargetID: evt.TargetID,
        })
    })
}
```

#### QA Validator Observes

```go
// qa/runner/validator.go

type Validator struct {
    bus      *events.Bus
    expected []Expectation
    results  []Result
}

func (v *Validator) Setup() {
    v.bus.SubscribeAll(func(e events.Event) {
        v.checkExpectations(e)
    })
}

func (v *Validator) ExpectMoveCompleted(x, y int, timeout time.Duration) {
    v.expected = append(v.expected, Expectation{
        EventType: "move_completed",
        Timeout:   timeout,
        Check: func(e events.Event) bool {
            evt := e.(*events.MoveCompleted)
            return evt.FinalX == x && evt.FinalY == y
        },
    })
}
```

---

## Consequences

### Positive

| Benefit | Description |
|---------|-------------|
| **Decoupling** | Input, logic, network, rendering are independent |
| **Testability** | QA runner publishes events, validates responses |
| **Bot Support** | Bots publish same events as players |
| **AI Integration** | External API maps directly to events |
| **Replay** | Event history enables debugging/replay |
| **Extensibility** | Add new consumers without changing producers |

### Negative

| Drawback | Mitigation |
|----------|------------|
| **Indirection** | Clear event naming, documentation |
| **Debugging** | Event logger, history inspection |
| **Performance** | Benchmark critical paths, optimize if needed |
| **Learning curve** | Good examples, consistent patterns |

### Neutral

- Event bus is a well-known pattern with established practices
- Similar to what Unity, Unreal, and other engines use internally
- Network packets already follow a similar pattern

---

## Event Naming Convention

### Format
```
<action>_<status>
```

### Examples
| Event | Description |
|-------|-------------|
| `move_to` | Command: request movement |
| `move_started` | Response: movement began |
| `move_completed` | Response: arrived at destination |
| `move_failed` | Response: couldn't move |
| `attack` | Command: request attack |
| `attack_started` | Response: attack animation began |
| `inventory_changed` | State: inventory updated |
| `quest_updated` | State: quest progress changed |

---

## Package Organization

```
pkg/events/
├── events.go       # Event interface, BaseEvent, EventSource
├── bus.go          # Event bus implementation
├── bus_test.go     # Bus tests
├── movement.go     # Movement-related events
├── combat.go       # Combat-related events
├── interaction.go  # NPC/item interaction events
├── state.go        # State change events
└── entity.go       # Entity spawn/despawn events
```

---

## Testing Strategy

### Unit Tests

```go
// pkg/events/bus_test.go

func TestBus_Publish(t *testing.T) {
    bus := NewBus(100)
    
    received := make(chan Event, 1)
    bus.Subscribe("move_to", func(e Event) {
        received <- e
    })
    
    evt := &MoveTo{
        BaseEvent: NewBase("move_to", SourcePlayer),
        TargetX:   100,
        TargetY:   200,
    }
    
    bus.Publish(evt)
    
    select {
    case r := <-received:
        assert.Equal(t, evt, r)
    case <-time.After(time.Second):
        t.Fatal("event not received")
    }
}

func TestBus_SubscribeAll(t *testing.T) {
    bus := NewBus(100)
    
    var events []Event
    bus.SubscribeAll(func(e Event) {
        events = append(events, e)
    })
    
    bus.Publish(&MoveTo{BaseEvent: NewBase("move_to", SourcePlayer)})
    bus.Publish(&Attack{BaseEvent: NewBase("attack", SourcePlayer)})
    
    assert.Len(t, events, 2)
}
```

### Integration Tests

```go
// internal/game/game_test.go

func TestGame_MoveToEvent(t *testing.T) {
    game := NewTestGame() // Uses test bus, mock dependencies
    
    // Publish event as if from player
    game.Bus().Publish(&events.MoveTo{
        BaseEvent: events.NewBase("move_to", events.SourcePlayer),
        TargetX:   100,
        TargetY:   100,
    })
    
    // Wait for move to complete
    game.WaitForEvent("move_completed", time.Second)
    
    assert.Equal(t, 100, game.Player().X)
    assert.Equal(t, 100, game.Player().Y)
}
```

---

## Migration Path

### Phase 1: Core Events
1. Implement `pkg/events` package
2. Add bus to game initialization
3. Convert player input to events

### Phase 2: Game Logic
1. Move logic from input handlers to event handlers
2. Add response events (completed, failed)
3. Update network client to use events

### Phase 3: QA Integration
1. Add debug API that publishes events
2. Implement event history
3. Create QA validator

### Phase 4: Bot Support
1. Create headless game mode
2. Implement bot AI publishing events
3. Test full cycle

---

## Alternatives Considered

### 1. Direct Method Calls

```go
// Player input calls game directly
inputHandler.OnClick() -> game.MovePlayer(x, y)
```

**Rejected**: Doesn't support external control, tight coupling.

### 2. Command Pattern (Objects)

```go
type Command interface {
    Execute(game *Game)
}

type MoveCommand struct {
    X, Y int
}
```

**Rejected**: Similar to events but less flexible, no built-in pub/sub.

### 3. Actor Model

Each entity is an actor with its own mailbox.

**Rejected**: Too complex for our needs, harder to debug.

### 4. Reactive Streams (RxGo)

Full reactive programming with observables.

**Rejected**: Overkill, adds dependency, steeper learning curve.

---

## References

- [Game Programming Patterns - Event Queue](https://gameprogrammingpatterns.com/event-queue.html)
- [ECS and Events in Game Development](https://www.gamedev.net/tutorials/programming/general-and-gameplay-programming/understanding-component-entity-systems-r3013/)
- [Unity Event System](https://docs.unity3d.com/Manual/EventSystem.html)
- ADR-002: Architecture (Layered Design)
- PRD-QA-SYSTEM: QA Automation Requirements

---

*Document Status: Accepted*
