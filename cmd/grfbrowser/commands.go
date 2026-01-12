// Command and screenshot handling for GRF Browser (ADR-010).
package main

import (
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/AllenDang/cimgui-go/imgui"
	"github.com/go-gl/gl/v4.1-core/gl"
)

// GUIState represents the current GUI state for JSON export (ADR-010 Phase 2).
type GUIState struct {
	Timestamp     string   `json:"timestamp"`
	GRFPath       string   `json:"grfPath"`
	SelectedPath  string   `json:"selectedPath"`
	SearchText    string   `json:"searchText"`
	ExpandedPaths []string `json:"expandedPaths"`
	Filters       struct {
		Sprites    bool `json:"sprites"`
		Animations bool `json:"animations"`
		Textures   bool `json:"textures"`
		Models     bool `json:"models"`
		Maps       bool `json:"maps"`
		Audio      bool `json:"audio"`
		Other      bool `json:"other"`
	} `json:"filters"`
	Stats struct {
		TotalFiles    int `json:"totalFiles"`
		FilteredFiles int `json:"filteredFiles"`
	} `json:"stats"`
}

// Command represents a remote command for GUI automation (ADR-010 Phase 3).
type Command struct {
	Action string          `json:"action"`
	Path   string          `json:"path,omitempty"`
	Value  string          `json:"value,omitempty"`
	Filter map[string]bool `json:"filter,omitempty"`
}

// captureScreenshot captures the current frame to a PNG file.
func (app *App) captureScreenshot() {
	// Get actual framebuffer size (handles HiDPI/Retina correctly)
	// DisplaySize is logical pixels, DisplayFramebufferScale is the multiplier
	io := imgui.CurrentIO()
	displaySize := io.DisplaySize()
	fbScale := io.DisplayFramebufferScale()
	width := int(displaySize.X * fbScale.X)
	height := int(displaySize.Y * fbScale.Y)

	if width <= 0 || height <= 0 {
		app.lastScreenshotMsg = "Screenshot failed: invalid viewport"
		app.showScreenshotMsg = true
		app.screenshotMsgTime = time.Now()
		return
	}

	// Read pixels from OpenGL framebuffer
	// Read from front buffer (what's currently displayed) since we capture at frame start
	gl.ReadBuffer(gl.FRONT)
	pixels := make([]byte, width*height*4) // RGBA
	gl.ReadPixels(0, 0, int32(width), int32(height), gl.RGBA, gl.UNSIGNED_BYTE, gl.Ptr(pixels))
	gl.ReadBuffer(gl.BACK) // Restore default

	// Create image (flip vertically - OpenGL has origin at bottom-left)
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			srcIdx := ((height-1-y)*width + x) * 4
			dstIdx := (y*width + x) * 4
			img.Pix[dstIdx+0] = pixels[srcIdx+0] // R
			img.Pix[dstIdx+1] = pixels[srcIdx+1] // G
			img.Pix[dstIdx+2] = pixels[srcIdx+2] // B
			img.Pix[dstIdx+3] = pixels[srcIdx+3] // A
		}
	}

	// Generate filename with timestamp
	timestamp := time.Now().Format("20060102-150405")
	filename := fmt.Sprintf("screenshot-%s.png", timestamp)
	savePath := filepath.Join(app.screenshotDir, filename)

	// Save to file
	file, err := os.Create(savePath)
	if err != nil {
		app.lastScreenshotMsg = fmt.Sprintf("Screenshot failed: %v", err)
		app.showScreenshotMsg = true
		app.screenshotMsgTime = time.Now()
		return
	}
	defer file.Close()

	if err := png.Encode(file, img); err != nil {
		app.lastScreenshotMsg = fmt.Sprintf("Screenshot failed: %v", err)
		app.showScreenshotMsg = true
		app.screenshotMsgTime = time.Now()
		return
	}

	// Also save as "latest.png" for easy access by automation
	latestPath := filepath.Join(app.screenshotDir, "latest.png")
	if latestFile, err := os.Create(latestPath); err == nil {
		_ = png.Encode(latestFile, img)
		latestFile.Close()
	}

	// Show notification
	app.lastScreenshotMsg = fmt.Sprintf("Saved: %s", filename)
	app.showScreenshotMsg = true
	app.screenshotMsgTime = time.Now()

	// Print to console for automation scripts
	fmt.Printf("Screenshot saved: %s\n", savePath)
}

// showNotification displays a brief overlay notification message.
func (app *App) showNotification(msg string) {
	app.lastScreenshotMsg = msg
	app.showScreenshotMsg = true
	app.screenshotMsgTime = time.Now()
}

// dumpState exports the current GUI state as JSON.
// Press F11 to trigger. Used for automated GUI testing with Claude (ADR-010 Phase 2).
func (app *App) dumpState() {
	// Build list of expanded paths
	expandedList := make([]string, 0)
	for path, expanded := range app.expandedPaths {
		if expanded {
			expandedList = append(expandedList, path)
		}
	}
	sort.Strings(expandedList)

	// Create state object
	state := GUIState{
		Timestamp:     time.Now().Format(time.RFC3339),
		GRFPath:       app.grfPath,
		SelectedPath:  app.selectedPath,
		SearchText:    app.searchText,
		ExpandedPaths: expandedList,
	}
	state.Filters.Sprites = app.filterSprites
	state.Filters.Animations = app.filterAnimations
	state.Filters.Textures = app.filterTextures
	state.Filters.Models = app.filterModels
	state.Filters.Maps = app.filterMaps
	state.Filters.Audio = app.filterAudio
	state.Filters.Other = app.filterOther
	state.Stats.TotalFiles = app.totalFiles
	state.Stats.FilteredFiles = app.filterCount

	// Marshal to JSON
	jsonData, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		app.lastScreenshotMsg = fmt.Sprintf("State dump failed: %v", err)
		app.showScreenshotMsg = true
		app.screenshotMsgTime = time.Now()
		return
	}

	// Save to file
	statePath := filepath.Join(app.screenshotDir, "state.json")
	if err := os.WriteFile(statePath, jsonData, 0644); err != nil {
		app.lastScreenshotMsg = fmt.Sprintf("State dump failed: %v", err)
		app.showScreenshotMsg = true
		app.screenshotMsgTime = time.Now()
		return
	}

	// Show notification (reuse screenshot notification)
	app.lastScreenshotMsg = "State saved: state.json"
	app.showScreenshotMsg = true
	app.screenshotMsgTime = time.Now()

	// Print to console for automation scripts
	fmt.Printf("State saved: %s\n", statePath)
}

// checkAndExecuteCommand polls for command file and executes if found.
// Called each frame from render(). Commands are single-shot (file deleted after execution).
func (app *App) checkAndExecuteCommand() {
	cmdPath := filepath.Join(app.screenshotDir, "command.json")

	// Check if command file exists
	data, err := os.ReadFile(cmdPath)
	if err != nil {
		return // No command file, normal case
	}

	// Delete file immediately to prevent re-execution
	os.Remove(cmdPath)

	// Parse command
	var cmd Command
	if err := json.Unmarshal(data, &cmd); err != nil {
		fmt.Fprintf(os.Stderr, "Invalid command: %v\n", err)
		return
	}

	// Execute command
	app.executeCommand(cmd)
}

// executeCommand executes a single command.
func (app *App) executeCommand(cmd Command) {
	switch cmd.Action {
	case "select_file":
		app.selectedPath = cmd.Path
		app.selectedOriginalPath = cmd.Path // Command uses original path format
		app.lastScreenshotMsg = fmt.Sprintf("Selected: %s", cmd.Path)

	case "expand_folder":
		app.expandedPaths[cmd.Path] = true
		app.lastScreenshotMsg = fmt.Sprintf("Expanded: %s", cmd.Path)

	case "collapse_folder":
		app.expandedPaths[cmd.Path] = false
		app.lastScreenshotMsg = fmt.Sprintf("Collapsed: %s", cmd.Path)

	case "set_search":
		app.searchText = cmd.Value
		app.rebuildTree()
		app.lastScreenshotMsg = fmt.Sprintf("Search: %s", cmd.Value)

	case "clear_search":
		app.searchText = ""
		app.rebuildTree()
		app.lastScreenshotMsg = "Search cleared"

	case "set_filter":
		if cmd.Filter != nil {
			if v, ok := cmd.Filter["sprites"]; ok {
				app.filterSprites = v
			}
			if v, ok := cmd.Filter["animations"]; ok {
				app.filterAnimations = v
			}
			if v, ok := cmd.Filter["textures"]; ok {
				app.filterTextures = v
			}
			if v, ok := cmd.Filter["models"]; ok {
				app.filterModels = v
			}
			if v, ok := cmd.Filter["maps"]; ok {
				app.filterMaps = v
			}
			if v, ok := cmd.Filter["audio"]; ok {
				app.filterAudio = v
			}
			if v, ok := cmd.Filter["other"]; ok {
				app.filterOther = v
			}
			app.rebuildTree()
		}
		app.lastScreenshotMsg = "Filters updated"

	case "screenshot":
		app.screenshotRequested = true
		return // Skip notification, screenshot will show its own

	case "dump_state":
		app.dumpState()
		return // Skip notification, dumpState shows its own

	default:
		app.lastScreenshotMsg = fmt.Sprintf("Unknown command: %s", cmd.Action)
	}

	app.showScreenshotMsg = true
	app.screenshotMsgTime = time.Now()
	fmt.Printf("Command executed: %s\n", cmd.Action)
}
