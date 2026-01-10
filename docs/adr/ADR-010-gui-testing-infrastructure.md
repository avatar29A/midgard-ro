# ADR-010: GUI Testing Infrastructure

## Status
Proposed

## Context

The GRF Browser (ADR-009) uses ImGui for its interface, making traditional GUI testing challenging. When debugging GUI issues with AI assistance (Claude), we face limitations:

1. **Communication gap**: Describing visual state in text is error-prone and slow
2. **No automated verification**: Manual testing only, no regression detection
3. **Limited interaction**: Claude cannot directly interact with the GUI to reproduce issues

### Current Testing Approach

| Aspect | Current State | Problem |
|--------|---------------|---------|
| Visual verification | Manual only | Time-consuming, subjective |
| State inspection | None | Must add debug logging manually |
| Interaction replay | None | Cannot reproduce user actions |
| Regression testing | Manual use cases | No automated visual regression |

### Existing Solutions Considered

| Solution | Pros | Cons |
|----------|------|------|
| [imgui_test_engine](https://github.com/ocornut/imgui_test_engine) | Official, comprehensive | C++ only, complex Go integration |
| Applitools/Percy | Visual regression | Overkill, external service |
| Custom screenshot comparison | Simple | No interaction capability |
| State serialization | Machine-readable | No visual verification |

## Decision

Implement a **hybrid testing infrastructure** with three components:

### 1. Screenshot Capture (Phase 1)

Enable Claude to "see" the GUI by capturing framebuffer to PNG.

```
┌─────────────────────────────────────────────────────────┐
│                    GRF Browser                          │
│                                                         │
│       [F12 pressed]                                     │
│            │                                            │
│            ▼                                            │
│   ┌─────────────────────────────────────┐              │
│   │  OpenGL glReadPixels()              │              │
│   │  → Flip vertically (GL origin)      │              │
│   │  → Encode PNG                       │              │
│   │  → Save to /tmp/grfbrowser/         │              │
│   └─────────────────────────────────────┘              │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

**Features:**
- **F12 hotkey** triggers screenshot capture
- **Timestamped files**: `screenshot-20260110-143052.png`
- **Latest symlink**: Always points to most recent capture
- **On-screen notification**: Confirms capture success
- **Console output**: Path printed for automation scripts

**Implementation:**
```go
// App struct additions
screenshotDir     string    // Default: /tmp/grfbrowser
lastScreenshotMsg string    // UI notification text
showScreenshotMsg bool      // Show notification overlay
screenshotMsgTime time.Time // Auto-hide after 2s

// F12 handler in render()
if imgui.IsKeyChordPressed(imgui.KeyChord(imgui.KeyF12)) {
    app.captureScreenshot()
}

// Screenshot capture using OpenGL
func (app *App) captureScreenshot() {
    pixels := make([]byte, width*height*4)
    gl.ReadPixels(0, 0, w, h, gl.RGBA, gl.UNSIGNED_BYTE, gl.Ptr(pixels))
    // Flip Y, encode PNG, save
}
```

### 2. State Dump (Phase 2 - Future)

Export GUI state as JSON for machine verification.

```json
{
  "timestamp": "2026-01-10T14:30:52Z",
  "grfPath": "data.grf",
  "selectedPath": "data/sprite/monster.spr",
  "expandedPaths": ["data", "data/sprite"],
  "searchText": "monster",
  "filters": {
    "sprites": true,
    "animations": true,
    "textures": false
  },
  "stats": {
    "totalFiles": 18841,
    "filteredFiles": 256
  }
}
```

**Features:**
- **F11 hotkey** dumps current state
- **Auto-dump** on screenshot (optional)
- **JSON format** for easy parsing

### 3. Command Interface (Phase 3 - Future)

Allow external tools to send commands to the GUI.

```
┌──────────────┐     ┌─────────────────────────────────────┐
│  Test Script │────▶│  /tmp/grfbrowser/commands.json      │
│  (Claude)    │     │  {"action":"click","path":"data/"}  │
└──────────────┘     └─────────────────────────────────────┘
                                    │
                                    ▼
                     ┌─────────────────────────────────────┐
                     │            GRF Browser              │
                     │  • Reads command file               │
                     │  • Executes action                  │
                     │  • Writes state dump                │
                     │  • Takes screenshot                 │
                     └─────────────────────────────────────┘
```

**Supported commands:**
- `select_file`: Select a file in tree
- `expand_folder`: Expand a folder node
- `set_search`: Set search text
- `toggle_filter`: Enable/disable type filter
- `screenshot`: Capture current frame

## Consequences

### Positive
- Claude can visually inspect GUI via screenshots
- State verification without visual interpretation
- Foundation for automated GUI testing
- Debug sessions become reproducible

### Negative
- Screenshot comparison is not pixel-perfect across platforms
- State dump requires maintenance as UI evolves
- Command interface adds complexity

### Risks
- OpenGL context issues when reading framebuffer during ImGui render
- Performance impact of frequent screenshots
- File I/O blocking the render loop

## Implementation Phases

| Phase | Scope | Deliverables |
|-------|-------|--------------|
| **1** | Screenshot | F12 capture, PNG save, notification |
| **2** | State dump | F11 dump, JSON export, auto-dump option |
| **3** | Commands | File watcher, action executor, response cycle |

## File Structure

```
/tmp/grfbrowser/
├── screenshot-20260110-143052.png
├── screenshot-20260110-143055.png
├── latest.png -> screenshot-20260110-143055.png
├── state.json              (Phase 2)
└── commands.json           (Phase 3)
```

## Testing Strategy

### Phase 1 Verification
1. Launch GRF Browser with test GRF
2. Press F12
3. Verify file created in `/tmp/grfbrowser/`
4. Open PNG and confirm it matches screen
5. Verify console output shows path
6. Verify notification appears in UI

### Integration with Claude
```bash
# Claude can request screenshot
echo "Take a screenshot and show me"
# User presses F12
# Claude reads /tmp/grfbrowser/latest.png

# Claude analyzes the image
cat /tmp/grfbrowser/latest.png | claude "What files are visible in the tree?"
```

## References

- [ADR-009: GRF Browser Tool](ADR-009-grf-browser-tool.md)
- [ADR-005: QA Automation](ADR-005-qa-automation.md)
- [imgui_test_engine](https://github.com/ocornut/imgui_test_engine) - Official ImGui testing
- [OpenGL glReadPixels](https://registry.khronos.org/OpenGL-Refpages/gl4/html/glReadPixels.xhtml)
