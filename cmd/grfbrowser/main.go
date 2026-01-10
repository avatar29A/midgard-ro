// GRF Browser - A graphical tool for browsing Ragnarok Online GRF archives.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/AllenDang/cimgui-go/backend"
	"github.com/AllenDang/cimgui-go/backend/sdlbackend"
	"github.com/AllenDang/cimgui-go/imgui"
	"github.com/go-gl/gl/v4.1-core/gl"
	"golang.org/x/text/encoding/korean"
	"golang.org/x/text/transform"

	"github.com/Faultbox/midgard-ro/pkg/grf"
)

func main() {
	runtime.LockOSThread()

	// Parse command line arguments
	grfPath := flag.String("grf", "", "Path to GRF file to open")
	flag.Parse()

	// Create and run application
	app := NewApp()
	defer app.Close()

	// Open GRF if specified
	if *grfPath != "" {
		if err := app.OpenGRF(*grfPath); err != nil {
			fmt.Fprintf(os.Stderr, "Error opening GRF: %v\n", err)
		}
	}

	app.Run()
}

// App represents the GRF Browser application state.
type App struct {
	backend backend.Backend[sdlbackend.SDLWindowFlags]

	// GRF state
	archive     *grf.Archive
	grfPath     string
	fileTree    *FileNode
	flatFiles   []string
	totalFiles  int
	filterCount int

	// UI state
	searchText    string
	selectedPath  string
	expandedPaths map[string]bool

	// Filter state
	filterSprites    bool
	filterAnimations bool
	filterTextures   bool
	filterModels     bool
	filterMaps       bool
	filterAudio      bool
	filterOther      bool

	// Screenshot state (ADR-010: GUI testing infrastructure)
	screenshotDir       string    // Directory to save screenshots
	lastScreenshotMsg   string    // Status message for last screenshot
	showScreenshotMsg   bool      // Whether to show the notification
	screenshotMsgTime   time.Time // When notification was shown
	screenshotRequested bool      // Deferred capture flag (capture next frame)
}

// FileNode represents a node in the virtual file tree.
type FileNode struct {
	Name     string
	Path     string
	IsDir    bool
	Children []*FileNode
	Size     int64
}

// NewApp creates a new application instance.
func NewApp() *App {
	app := &App{
		expandedPaths:    make(map[string]bool),
		filterSprites:    true,
		filterAnimations: true,
		filterTextures:   true,
		filterModels:     true,
		filterMaps:       true,
		filterAudio:      true,
		filterOther:      true,
		screenshotDir:    "/tmp/grfbrowser",
	}

	// Ensure screenshot directory exists (ADR-010)
	if err := os.MkdirAll(app.screenshotDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not create screenshot dir: %v\n", err)
	}

	// Create backend using the proper wrapper
	var err error
	app.backend, err = backend.CreateBackend(sdlbackend.NewSDLBackend())
	if err != nil {
		panic(fmt.Sprintf("failed to create backend: %v", err))
	}

	app.backend.SetBgColor(imgui.NewVec4(0.1, 0.1, 0.12, 1.0))
	app.backend.CreateWindow("GRF Browser", 1280, 800)

	// Initialize OpenGL function pointers for screenshot capture (ADR-010)
	if err := gl.Init(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: OpenGL init failed (screenshots disabled): %v\n", err)
	}

	return app
}

// Close cleans up resources.
func (app *App) Close() {
	if app.archive != nil {
		app.archive.Close()
	}
}

// Run starts the main application loop.
func (app *App) Run() {
	app.backend.Run(app.render)
}

// OpenGRF opens a GRF archive file.
func (app *App) OpenGRF(path string) error {
	// Close existing archive
	if app.archive != nil {
		app.archive.Close()
	}

	// Open new archive
	archive, err := grf.Open(path)
	if err != nil {
		return fmt.Errorf("failed to open GRF: %w", err)
	}

	app.archive = archive
	app.grfPath = path
	app.flatFiles = archive.List()
	app.totalFiles = len(app.flatFiles)
	app.fileTree = app.buildFileTree()
	app.filterCount = app.totalFiles
	app.selectedPath = ""
	app.expandedPaths = make(map[string]bool)

	// Update window title
	app.backend.SetWindowTitle(fmt.Sprintf("GRF Browser - %s", filepath.Base(path)))

	return nil
}

// buildFileTree creates a virtual folder structure from flat file list.
func (app *App) buildFileTree() *FileNode {
	root := &FileNode{
		Name:     "root",
		Path:     "",
		IsDir:    true,
		Children: make([]*FileNode, 0),
	}

	// Map to track created directories
	dirs := make(map[string]*FileNode)
	dirs[""] = root

	// Sort files for consistent ordering
	sortedFiles := make([]string, len(app.flatFiles))
	copy(sortedFiles, app.flatFiles)
	sort.Strings(sortedFiles)

	for _, filePath := range sortedFiles {
		// Apply filters
		if !app.matchesFilter(filePath) {
			continue
		}

		// Apply search
		if app.searchText != "" && !app.matchesSearch(filePath) {
			continue
		}

		// Normalize path and convert from EUC-KR to UTF-8
		normalizedPath := strings.ReplaceAll(filePath, "\\", "/")
		normalizedPath = euckrToUTF8(normalizedPath)
		parts := strings.Split(normalizedPath, "/")

		// Create parent directories
		currentPath := ""
		parent := root

		for i, part := range parts {
			if i < len(parts)-1 {
				// Directory
				if currentPath != "" {
					currentPath += "/"
				}
				currentPath += part

				if existing, ok := dirs[currentPath]; ok {
					parent = existing
				} else {
					newDir := &FileNode{
						Name:     part,
						Path:     currentPath,
						IsDir:    true,
						Children: make([]*FileNode, 0),
					}
					parent.Children = append(parent.Children, newDir)
					dirs[currentPath] = newDir
					parent = newDir
				}
			} else {
				// File
				fileNode := &FileNode{
					Name:  part,
					Path:  normalizedPath,
					IsDir: false,
				}
				parent.Children = append(parent.Children, fileNode)
			}
		}
	}

	// Sort children at each level
	app.sortTree(root)

	return root
}

// sortTree recursively sorts children (directories first, then alphabetically).
func (app *App) sortTree(node *FileNode) {
	sort.Slice(node.Children, func(i, j int) bool {
		// Directories first
		if node.Children[i].IsDir != node.Children[j].IsDir {
			return node.Children[i].IsDir
		}
		// Then alphabetically
		return strings.ToLower(node.Children[i].Name) < strings.ToLower(node.Children[j].Name)
	})

	for _, child := range node.Children {
		if child.IsDir {
			app.sortTree(child)
		}
	}
}

// matchesFilter checks if a file matches the current type filters.
func (app *App) matchesFilter(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))

	switch ext {
	case ".spr":
		return app.filterSprites
	case ".act":
		return app.filterAnimations
	case ".bmp", ".tga", ".jpg", ".png", ".imf":
		return app.filterTextures
	case ".rsm":
		return app.filterModels
	case ".rsw", ".gat", ".gnd":
		return app.filterMaps
	case ".wav", ".mp3":
		return app.filterAudio
	default:
		return app.filterOther
	}
}

// matchesSearch checks if a file matches the search pattern.
func (app *App) matchesSearch(path string) bool {
	if app.searchText == "" {
		return true
	}
	return strings.Contains(strings.ToLower(path), strings.ToLower(app.searchText))
}

// countFilteredFiles counts files matching current filters.
func (app *App) countFilteredFiles() int {
	count := 0
	for _, path := range app.flatFiles {
		if app.matchesFilter(path) && app.matchesSearch(path) {
			count++
		}
	}
	return count
}

// render is called each frame to draw the UI.
func (app *App) render() {
	// Deferred screenshot capture (ADR-010: GUI testing)
	// Capture at start of frame to get previous frame's rendered content
	if app.screenshotRequested {
		app.screenshotRequested = false
		app.captureScreenshot()
	}

	// Check for remote commands (ADR-010 Phase 3)
	app.checkAndExecuteCommand()

	// Handle keyboard shortcuts
	// F12 = request screenshot (captured next frame to get rendered content)
	if imgui.IsKeyChordPressed(imgui.KeyChord(imgui.KeyF12)) {
		app.screenshotRequested = true
	}

	// Ctrl+D = dump GUI state as JSON (ADR-010 Phase 2)
	ctrlD := imgui.KeyChord(imgui.ModCtrl) | imgui.KeyChord(imgui.KeyD)
	if imgui.IsKeyChordPressed(ctrlD) {
		app.dumpState()
	}

	// Ctrl+C = copy filename only
	// Cmd+Ctrl+C = copy full path (macOS friendly)
	if app.selectedPath != "" {
		ctrlC := imgui.KeyChord(imgui.ModCtrl) | imgui.KeyChord(imgui.KeyC)
		cmdCtrlC := imgui.KeyChord(imgui.ModSuper) | imgui.KeyChord(imgui.ModCtrl) | imgui.KeyChord(imgui.KeyC)

		if imgui.IsKeyChordPressed(cmdCtrlC) {
			imgui.SetClipboardText(app.selectedPath)
		} else if imgui.IsKeyChordPressed(ctrlC) {
			imgui.SetClipboardText(filepath.Base(app.selectedPath))
		}
	}

	// Main menu bar
	if imgui.BeginMainMenuBar() {
		if imgui.BeginMenu("File") {
			if imgui.MenuItemBool("Open GRF...") {
				// File dialog will be implemented in Stage 2
				// For now, use: ./grfbrowser -grf path/to/file.grf
				_ = true // Placeholder to avoid empty branch warning
			}
			imgui.Separator()
			if imgui.MenuItemBool("Exit") {
				os.Exit(0)
			}
			imgui.EndMenu()
		}
		imgui.EndMainMenuBar()
	}

	// Get viewport work area (excludes menu bar)
	viewport := imgui.MainViewport()
	workPos := viewport.WorkPos()
	workSize := viewport.WorkSize()

	// Layout dimensions
	leftPanelWidth := float32(350)
	statusBarHeight := float32(30)
	contentHeight := workSize.Y - statusBarHeight

	// Window flags for fixed panels
	flags := imgui.WindowFlagsNoMove | imgui.WindowFlagsNoResize | imgui.WindowFlagsNoCollapse

	// Left panel - File browser
	imgui.SetNextWindowPos(workPos)
	imgui.SetNextWindowSize(imgui.NewVec2(leftPanelWidth, contentHeight))
	if imgui.BeginV("Files", nil, flags) {
		app.renderSearchAndFilter()
		imgui.Separator()
		app.renderFileTree()
	}
	imgui.End()

	// Right panel - Preview
	imgui.SetNextWindowPos(imgui.NewVec2(workPos.X+leftPanelWidth, workPos.Y))
	imgui.SetNextWindowSize(imgui.NewVec2(workSize.X-leftPanelWidth, contentHeight))
	if imgui.BeginV("Preview", nil, flags) {
		app.renderPreview()
	}
	imgui.End()

	// Status bar at bottom
	imgui.SetNextWindowPos(imgui.NewVec2(workPos.X, workPos.Y+contentHeight))
	imgui.SetNextWindowSize(imgui.NewVec2(workSize.X, statusBarHeight))
	statusFlags := flags | imgui.WindowFlagsNoTitleBar | imgui.WindowFlagsNoScrollbar
	if imgui.BeginV("##StatusBar", nil, statusFlags) {
		app.renderStatusBar()
	}
	imgui.End()

	// Screenshot notification overlay (ADR-010)
	// Shows for 2 seconds after capture
	if app.showScreenshotMsg && time.Since(app.screenshotMsgTime) < 2*time.Second {
		notifyFlags := imgui.WindowFlagsNoTitleBar | imgui.WindowFlagsNoResize |
			imgui.WindowFlagsNoMove | imgui.WindowFlagsNoScrollbar |
			imgui.WindowFlagsAlwaysAutoResize | imgui.WindowFlagsNoFocusOnAppearing
		imgui.SetNextWindowPos(imgui.NewVec2(workPos.X+10, workPos.Y+10))
		imgui.SetNextWindowBgAlpha(0.85)
		if imgui.BeginV("##ScreenshotNotify", nil, notifyFlags) {
			imgui.Text(app.lastScreenshotMsg)
		}
		imgui.End()
	} else if app.showScreenshotMsg {
		app.showScreenshotMsg = false
	}
}

// captureScreenshot captures the current frame to a PNG file.
// Press F12 to trigger. Used for automated GUI testing with Claude (ADR-010).
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

// Command represents a remote command for GUI automation (ADR-010 Phase 3).
type Command struct {
	Action string          `json:"action"`
	Path   string          `json:"path,omitempty"`
	Value  string          `json:"value,omitempty"`
	Filter map[string]bool `json:"filter,omitempty"`
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

// renderSearchAndFilter renders the search box and filter checkboxes.
func (app *App) renderSearchAndFilter() {
	// Search input
	imgui.Text("Search:")
	imgui.SameLine()

	imgui.SetNextItemWidth(-1)
	if imgui.InputTextWithHint("##search", "Filter files...", &app.searchText, 0, nil) {
		app.rebuildTree()
	}

	// Filter checkboxes in two columns using table
	if imgui.TreeNodeExStrV("Filters", imgui.TreeNodeFlagsDefaultOpen) {
		changed := false

		if imgui.BeginTable("filterTable", 2) {
			imgui.TableNextRow()
			imgui.TableNextColumn()
			if imgui.Checkbox("Sprites", &app.filterSprites) {
				changed = true
			}
			imgui.TableNextColumn()
			if imgui.Checkbox("Models", &app.filterModels) {
				changed = true
			}

			imgui.TableNextRow()
			imgui.TableNextColumn()
			if imgui.Checkbox("Animations", &app.filterAnimations) {
				changed = true
			}
			imgui.TableNextColumn()
			if imgui.Checkbox("Maps", &app.filterMaps) {
				changed = true
			}

			imgui.TableNextRow()
			imgui.TableNextColumn()
			if imgui.Checkbox("Textures", &app.filterTextures) {
				changed = true
			}
			imgui.TableNextColumn()
			if imgui.Checkbox("Other", &app.filterOther) {
				changed = true
			}

			imgui.TableNextRow()
			imgui.TableNextColumn()
			if imgui.Checkbox("Audio", &app.filterAudio) {
				changed = true
			}

			imgui.EndTable()
		}

		if changed {
			app.rebuildTree()
		}

		imgui.TreePop()
	}
}

// rebuildTree rebuilds the file tree after filter/search changes.
func (app *App) rebuildTree() {
	if app.archive != nil {
		app.fileTree = app.buildFileTree()
		app.filterCount = app.countFilteredFiles()
	}
}

// renderFileTree renders the file tree view.
func (app *App) renderFileTree() {
	if app.archive == nil {
		imgui.TextDisabled("No GRF loaded")
		imgui.TextDisabled("Use File > Open GRF...")
		return
	}

	// File tree in child window for scrolling
	if imgui.BeginChildStrV("FileTreeChild", imgui.NewVec2(0, 0), imgui.ChildFlagsBorders, imgui.WindowFlagsHorizontalScrollbar) {
		app.renderTreeNode(app.fileTree)
	}
	imgui.EndChild()
}

// renderTreeNode recursively renders a tree node.
func (app *App) renderTreeNode(node *FileNode) {
	for _, child := range node.Children {
		if child.IsDir {
			// Directory node
			flags := imgui.TreeNodeFlagsOpenOnArrow | imgui.TreeNodeFlagsOpenOnDoubleClick

			// Check if expanded
			if app.expandedPaths[child.Path] {
				flags |= imgui.TreeNodeFlagsDefaultOpen
			}

			// Folder icon (text-based for font compatibility)
			open := imgui.TreeNodeExStrV("[+] "+child.Name, flags)

			// Track expansion state
			if open {
				app.expandedPaths[child.Path] = true
				app.renderTreeNode(child)
				imgui.TreePop()
			} else {
				app.expandedPaths[child.Path] = false
			}
		} else {
			// File node (leaf)
			flags := imgui.TreeNodeFlagsLeaf | imgui.TreeNodeFlagsNoTreePushOnOpen

			// Selection state
			if child.Path == app.selectedPath {
				flags |= imgui.TreeNodeFlagsSelected
			}

			// File icon based on type
			icon := app.getFileIcon(child.Name)

			imgui.TreeNodeExStrV(icon+" "+child.Name, flags)

			if imgui.IsItemClicked() {
				app.selectedPath = child.Path
			}
		}
	}
}

// getFileIcon returns an icon for a file based on its extension.
func (app *App) getFileIcon(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))

	switch ext {
	case ".spr":
		return "[SPR]"
	case ".act":
		return "[ACT]"
	case ".bmp", ".tga", ".jpg", ".png", ".imf":
		return "[IMG]"
	case ".rsm":
		return "[3D]"
	case ".rsw":
		return "[MAP]"
	case ".gat":
		return "[GAT]"
	case ".gnd":
		return "[GND]"
	case ".wav", ".mp3":
		return "[SND]"
	case ".txt", ".xml", ".lua":
		return "[TXT]"
	default:
		return "[?]"
	}
}

// renderPreview renders the preview panel for the selected file.
func (app *App) renderPreview() {
	if app.selectedPath == "" {
		imgui.TextDisabled("Select a file to preview")
		return
	}

	imgui.Text("Selected:")
	imgui.TextWrapped(app.selectedPath)
	imgui.Separator()

	// Show file extension info
	ext := strings.ToLower(filepath.Ext(app.selectedPath))
	imgui.Text("Type: " + app.getFileTypeName(ext))

	imgui.Separator()
	imgui.TextDisabled("Preview coming in Stage 3...")
}

// getFileTypeName returns a human-readable file type name.
func (app *App) getFileTypeName(ext string) string {
	switch ext {
	case ".spr":
		return "Sprite Image"
	case ".act":
		return "Animation Data"
	case ".bmp", ".tga", ".jpg", ".png":
		return "Texture Image"
	case ".imf":
		return "Image Format (IMF)"
	case ".rsm":
		return "3D Model"
	case ".rsw":
		return "Map Resource"
	case ".gat":
		return "Ground Altitude"
	case ".gnd":
		return "Ground Mesh"
	case ".wav", ".mp3":
		return "Audio File"
	case ".txt":
		return "Text File"
	case ".xml":
		return "XML File"
	case ".lua":
		return "Lua Script"
	default:
		return "Unknown"
	}
}

// euckrToUTF8 converts EUC-KR encoded string to UTF-8.
func euckrToUTF8(s string) string {
	decoder := korean.EUCKR.NewDecoder()
	result, _, err := transform.String(decoder, s)
	if err != nil {
		return s // Return original if conversion fails
	}
	return result
}

// renderStatusBar renders the status bar at the bottom.
func (app *App) renderStatusBar() {
	if app.archive != nil {
		imgui.Text(fmt.Sprintf("%d files total | %d filtered | Selected: %s",
			app.totalFiles, app.filterCount, app.selectedPath))
	} else {
		imgui.Text("No GRF loaded")
	}
}
