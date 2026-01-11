// GRF Browser - A graphical tool for browsing Ragnarok Online GRF archives.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg" // JPEG decoder
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
	"github.com/sqweek/dialog"
	_ "golang.org/x/image/bmp" // BMP decoder registration
	"golang.org/x/text/encoding/korean"
	"golang.org/x/text/transform"

	"github.com/Faultbox/midgard-ro/pkg/formats"
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
	searchText           string
	selectedPath         string // Display path (UTF-8)
	selectedOriginalPath string // Archive path (for file reading)
	expandedPaths        map[string]bool
	// TODO (Stage 5): TAB key to cycle focus between Search/Tree/Preview panels
	// Requires research into ImGui keyboard navigation activation

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

	// File dialog state (must open on main thread)
	pendingGRFPath string // Path selected from file dialog, processed on main thread

	// Sprite preview state (ADR-009 Stage 3)
	previewSPR      *formats.SPR       // Currently loaded sprite
	previewACT      *formats.ACT       // Currently loaded animation
	previewTextures []*backend.Texture // Textures for each sprite frame
	previewFrame    int                // Current frame index
	previewAction   int                // Current action index (for ACT)
	previewPlaying  bool               // Animation playing state
	previewLastTime time.Time          // Last frame update time
	previewPath     string             // Path of currently previewed file
	previewZoom     float32            // Zoom level for preview

	// Image preview state (ADR-009 Stage 4)
	previewImage   *backend.Texture // Texture for image preview
	previewImgSize [2]int           // Original image dimensions [width, height]

	// Text preview state (ADR-009 Stage 4)
	previewText string // Text content for text viewer

	// Hex preview state (ADR-009 Stage 4)
	previewHex     []byte // Raw bytes for hex viewer
	previewHexSize int64  // Original file size
}

// FileNode represents a node in the virtual file tree.
type FileNode struct {
	Name         string // Display name (UTF-8)
	Path         string // Display path (UTF-8)
	OriginalPath string // Archive path (original encoding for lookups)
	IsDir        bool
	Children     []*FileNode
	Size         int64
}

// koreanGlyphRanges defines the Unicode ranges for Korean text rendering.
// Format: pairs of [start, end] values terminated by 0.
// Includes:
// - Basic Latin (0x0020-0x00FF) for ASCII and extended Latin
// - Hangul Syllables (0xAC00-0xD7AF) for Korean characters
var koreanGlyphRanges = []imgui.Wchar{
	0x0020, 0x00FF, // Basic Latin + Latin Supplement
	0x3000, 0x30FF, // CJK Symbols and Punctuation, Hiragana, Katakana
	0x3130, 0x318F, // Hangul Compatibility Jamo
	0xAC00, 0xD7AF, // Hangul Syllables
	0, // Terminator
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
		previewZoom:      1.0,
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

	// Set up font loading hook BEFORE creating window (ADR-009 Stage 2: Korean font support)
	app.backend.SetAfterCreateContextHook(func() {
		app.loadKoreanFont()
	})

	app.backend.SetBgColor(imgui.NewVec4(0.1, 0.1, 0.12, 1.0))
	app.backend.CreateWindow("GRF Browser", 1280, 800)

	// Initialize OpenGL function pointers for screenshot capture (ADR-010)
	if err := gl.Init(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: OpenGL init failed (screenshots disabled): %v\n", err)
	}

	return app
}

// loadKoreanFont loads a font with Korean glyph support.
// Called from SetAfterCreateContextHook after ImGui context is created.
func (app *App) loadKoreanFont() {
	io := imgui.CurrentIO()
	fonts := io.Fonts()

	// Try different font paths (cross-platform support)
	fontPaths := []string{
		"/Library/Fonts/Arial Unicode.ttf",                       // macOS (symlink)
		"/System/Library/Fonts/Supplemental/Arial Unicode.ttf",   // macOS (actual)
		"C:\\Windows\\Fonts\\malgun.ttf",                         // Windows (Malgun Gothic)
		"C:\\Windows\\Fonts\\gulim.ttc",                          // Windows (Gulim)
		"/usr/share/fonts/truetype/noto/NotoSansCJK-Regular.ttc", // Linux
		"/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc", // Linux alt
	}

	var fontPath string
	for _, path := range fontPaths {
		if _, err := os.Stat(path); err == nil {
			fontPath = path
			break
		}
	}

	if fontPath == "" {
		fmt.Fprintf(os.Stderr, "Warning: No Korean font found, using default font\n")
		return
	}

	// Create font config
	fontCfg := imgui.NewFontConfig()
	defer fontCfg.Destroy()

	// Load font with Korean glyph ranges
	font := fonts.AddFontFromFileTTFV(fontPath, 16.0, fontCfg, &koreanGlyphRanges[0])
	if font == nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to load Korean font from %s\n", fontPath)
		return
	}

	fmt.Printf("Loaded Korean font: %s\n", fontPath)
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

// openFileDialog shows a native file dialog to select a GRF file.
func (app *App) openFileDialog() {
	// Run in goroutine to not block the UI
	// NOTE: SDL/Cocoa window operations must happen on main thread,
	// so we just set pendingGRFPath here and process it in render()
	go func() {
		filename, err := dialog.File().
			Filter("GRF Archives", "grf", "gpf").
			Filter("All Files", "*").
			Title("Open GRF Archive").
			Load()

		if err != nil {
			// User canceled or error occurred
			if err != dialog.ErrCancelled {
				fmt.Fprintf(os.Stderr, "File dialog error: %v\n", err)
			}
			return
		}

		// Queue the file to be opened on main thread
		app.pendingGRFPath = filename
	}()
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
	app.selectedOriginalPath = ""
	app.expandedPaths = make(map[string]bool)

	// Clear any existing preview
	app.clearPreview()

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

		// Keep original path for archive lookups
		originalPath := strings.ReplaceAll(filePath, "\\", "/")

		// Convert to UTF-8 for display
		displayPath := euckrToUTF8(originalPath)

		// Apply search against UTF-8 display path (supports Korean input)
		if app.searchText != "" && !app.matchesSearch(displayPath) {
			continue
		}
		parts := strings.Split(displayPath, "/")

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
				// File - store both display path and original path for archive lookup
				fileNode := &FileNode{
					Name:         part,
					Path:         displayPath,
					OriginalPath: originalPath,
					IsDir:        false,
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
		if !app.matchesFilter(path) {
			continue
		}
		// Convert to UTF-8 for search matching (supports Korean input)
		displayPath := euckrToUTF8(strings.ReplaceAll(path, "\\", "/"))
		if app.matchesSearch(displayPath) {
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

	// Process pending file dialog result (must be on main thread for SDL/Cocoa)
	if app.pendingGRFPath != "" {
		path := app.pendingGRFPath
		app.pendingGRFPath = ""
		if err := app.OpenGRF(path); err != nil {
			fmt.Fprintf(os.Stderr, "Error opening GRF: %v\n", err)
		}
	}

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

	// NOTE: TAB key navigates within tree (ImGui default behavior)
	// TODO (Stage 5): Research ImGui ConfigFlags to implement custom TAB panel cycling

	// Ctrl+C = copy filename only
	// Cmd+Ctrl+C = copy full path (macOS friendly)
	if app.selectedPath != "" {
		ctrlC := imgui.KeyChord(imgui.ModCtrl) | imgui.KeyChord(imgui.KeyC)
		cmdCtrlC := imgui.KeyChord(imgui.ModSuper) | imgui.KeyChord(imgui.ModCtrl) | imgui.KeyChord(imgui.KeyC)

		if imgui.IsKeyChordPressed(cmdCtrlC) {
			imgui.SetClipboardText(app.selectedPath)
			app.showNotification("Copied: " + app.selectedPath)
		} else if imgui.IsKeyChordPressed(ctrlC) {
			name := filepath.Base(app.selectedPath)
			imgui.SetClipboardText(name)
			app.showNotification("Copied: " + name)
		}
	}

	// Space to toggle Play/Pause for animations (when not in text input)
	if app.previewACT != nil && !imgui.IsAnyItemActive() {
		if imgui.IsKeyChordPressed(imgui.KeyChord(imgui.KeySpace)) {
			app.previewPlaying = !app.previewPlaying
			if app.previewPlaying {
				app.previewLastTime = time.Now()
			}
		}
	}

	// Zoom controls: +/- to zoom, 0 to reset (works when preview has content)
	if app.previewSPR != nil {
		// + or = key to zoom in
		if imgui.IsKeyChordPressed(imgui.KeyChord(imgui.KeyEqual)) || imgui.IsKeyChordPressed(imgui.KeyChord(imgui.KeyKeypadAdd)) {
			if app.previewZoom < 8.0 {
				app.previewZoom += 0.5
			}
		}
		// - key to zoom out
		if imgui.IsKeyChordPressed(imgui.KeyChord(imgui.KeyMinus)) || imgui.IsKeyChordPressed(imgui.KeyChord(imgui.KeyKeypadSubtract)) {
			if app.previewZoom > 0.5 {
				app.previewZoom -= 0.5
			}
		}
		// 0 key to reset zoom
		if imgui.IsKeyChordPressed(imgui.KeyChord(imgui.Key0)) || imgui.IsKeyChordPressed(imgui.KeyChord(imgui.KeyKeypad0)) {
			app.previewZoom = 1.0
		}
	}

	// Main menu bar
	if imgui.BeginMainMenuBar() {
		if imgui.BeginMenu("File") {
			if imgui.MenuItemBool("Open GRF...") {
				app.openFileDialog()
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
	rightPanelWidth := float32(200) // Actions panel for animations
	statusBarHeight := float32(30)
	contentHeight := workSize.Y - statusBarHeight

	// Show actions panel only for ACT files
	showActionsPanel := app.previewACT != nil

	// Window flags for fixed panels
	flags := imgui.WindowFlagsNoMove | imgui.WindowFlagsNoResize | imgui.WindowFlagsNoCollapse

	// Left panel - File browser (contains Search and Tree)
	imgui.SetNextWindowPos(workPos)
	imgui.SetNextWindowSize(imgui.NewVec2(leftPanelWidth, contentHeight))
	if imgui.BeginV("Files", nil, flags) {
		app.renderSearchAndFilter()
		imgui.Separator()
		app.renderFileTree()
	}
	imgui.End()

	// Calculate preview panel width (shrinks when actions panel is shown)
	previewWidth := workSize.X - leftPanelWidth
	if showActionsPanel {
		previewWidth -= rightPanelWidth
	}

	// Center panel - Preview
	imgui.SetNextWindowPos(imgui.NewVec2(workPos.X+leftPanelWidth, workPos.Y))
	imgui.SetNextWindowSize(imgui.NewVec2(previewWidth, contentHeight))
	if imgui.BeginV("Preview", nil, flags) {
		app.renderPreview()
	}
	imgui.End()

	// Right panel - Actions (only for ACT files)
	if showActionsPanel {
		imgui.SetNextWindowPos(imgui.NewVec2(workPos.X+leftPanelWidth+previewWidth, workPos.Y))
		imgui.SetNextWindowSize(imgui.NewVec2(rightPanelWidth, contentHeight))
		if imgui.BeginV("Actions", nil, flags) {
			app.renderActionsPanel()
		}
		imgui.End()
	}

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

// showNotification displays a brief overlay notification message.
func (app *App) showNotification(msg string) {
	app.lastScreenshotMsg = msg
	app.showScreenshotMsg = true
	app.screenshotMsgTime = time.Now()
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

		// Select All / Unselect All buttons
		if imgui.Button("All") {
			app.filterSprites = true
			app.filterAnimations = true
			app.filterTextures = true
			app.filterModels = true
			app.filterMaps = true
			app.filterAudio = true
			app.filterOther = true
			changed = true
		}
		imgui.SameLine()
		if imgui.Button("None") {
			app.filterSprites = false
			app.filterAnimations = false
			app.filterTextures = false
			app.filterModels = false
			app.filterMaps = false
			app.filterAudio = false
			app.filterOther = false
			changed = true
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
		if app.fileTree != nil {
			app.renderTreeNode(app.fileTree)
		}
	}
	imgui.EndChild()
}

// renderTreeNode recursively renders a tree node.
func (app *App) renderTreeNode(node *FileNode) {
	if node == nil {
		return
	}
	for _, child := range node.Children {
		if child.IsDir {
			// Directory node
			flags := imgui.TreeNodeFlagsOpenOnArrow | imgui.TreeNodeFlagsOpenOnDoubleClick | imgui.TreeNodeFlagsSpanAvailWidth

			// Selection state for directories
			if child.Path == app.selectedPath {
				flags |= imgui.TreeNodeFlagsSelected
			}

			// Check if expanded
			isExpanded := app.expandedPaths[child.Path]
			if isExpanded {
				flags |= imgui.TreeNodeFlagsDefaultOpen
			}

			// Folder icon (text-based for font compatibility)
			open := imgui.TreeNodeExStrV("[+] "+child.Name, flags)

			// Select directory when focused (for highlighting)
			if imgui.IsItemFocused() {
				app.selectedPath = child.Path
				app.selectedOriginalPath = ""
			}

			// Toggle expand/collapse with Space when focused
			if imgui.IsItemFocused() && imgui.IsKeyChordPressed(imgui.KeyChord(imgui.KeySpace)) {
				// Toggle expansion state - will be applied on next frame rebuild
				app.expandedPaths[child.Path] = !isExpanded
			}

			// Track expansion state
			if open {
				app.expandedPaths[child.Path] = true
				app.renderTreeNode(child)
				imgui.TreePop()
			} else if !imgui.IsItemFocused() || !imgui.IsKeyChordPressed(imgui.KeyChord(imgui.KeySpace)) {
				// Only set to false if not manually toggling
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

			// Auto-select when navigating with arrows (IsItemFocused), or on click/Enter
			if imgui.IsItemClicked() || imgui.IsItemFocused() {
				app.selectedPath = child.Path
				app.selectedOriginalPath = child.OriginalPath
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

	// Load preview if path changed
	if app.previewPath != app.selectedPath {
		app.loadPreview(app.selectedPath)
	}

	imgui.Separator()

	// Render based on file type
	switch ext {
	case ".spr":
		app.renderSpritePreview()
	case ".act":
		app.renderAnimationPreview()
	case ".bmp", ".tga", ".jpg", ".jpeg", ".png":
		app.renderImagePreview()
	case ".txt", ".xml", ".lua", ".ini", ".cfg":
		app.renderTextPreview()
	default:
		app.renderHexPreview()
	}
}

// loadPreview loads the preview for the given display path.
// Uses selectedOriginalPath for archive reads (handles EUC-KR paths).
func (app *App) loadPreview(displayPath string) {
	// Clear previous preview
	app.clearPreview()
	app.previewPath = displayPath

	if app.archive == nil {
		return
	}

	// Use original path for archive reads (EUC-KR encoded for Korean paths)
	archivePath := app.selectedOriginalPath
	if archivePath == "" {
		archivePath = displayPath // Fallback for ASCII paths
	}

	ext := strings.ToLower(filepath.Ext(displayPath))
	switch ext {
	case ".spr":
		app.loadSpritePreview(archivePath)
	case ".act":
		app.loadAnimationPreview(archivePath)
	case ".bmp", ".tga", ".jpg", ".jpeg", ".png":
		app.loadImagePreview(archivePath)
	case ".txt", ".xml", ".lua", ".ini", ".cfg":
		app.loadTextPreview(archivePath)
	default:
		// Load as hex for unknown formats
		app.loadHexPreview(archivePath)
	}
}

// clearPreview releases preview resources.
func (app *App) clearPreview() {
	// Release sprite textures
	for _, tex := range app.previewTextures {
		if tex != nil {
			tex.Release()
		}
	}
	app.previewTextures = nil
	app.previewSPR = nil
	app.previewACT = nil
	app.previewFrame = 0
	app.previewAction = 0
	app.previewPlaying = false

	// Release image texture (Stage 4)
	if app.previewImage != nil {
		app.previewImage.Release()
		app.previewImage = nil
	}
	app.previewImgSize = [2]int{0, 0}

	// Clear text and hex preview (Stage 4)
	app.previewText = ""
	app.previewHex = nil
	app.previewHexSize = 0
}

// loadSpritePreview loads a SPR file for preview.
func (app *App) loadSpritePreview(path string) {
	data, err := app.archive.Read(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading sprite: %v\n", err)
		return
	}

	spr, err := formats.ParseSPR(data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing sprite: %v\n", err)
		return
	}

	app.previewSPR = spr

	// Create textures for all images
	app.previewTextures = make([]*backend.Texture, len(spr.Images))
	for i, img := range spr.Images {
		rgba := sprImageToRGBA(&img)
		app.previewTextures[i] = backend.NewTextureFromRgba(rgba)
	}
}

// loadAnimationPreview loads an ACT file for preview.
func (app *App) loadAnimationPreview(path string) {
	data, err := app.archive.Read(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading animation: %v\n", err)
		return
	}

	act, err := formats.ParseACT(data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing animation: %v\n", err)
		return
	}

	app.previewACT = act

	// Try to load corresponding SPR file (try both .spr and .SPR extensions)
	basePath := strings.TrimSuffix(path, filepath.Ext(path))
	sprPath := ""
	for _, ext := range []string{".spr", ".SPR", ".Spr"} {
		candidate := basePath + ext
		if app.archive.Contains(candidate) {
			sprPath = candidate
			break
		}
	}

	if sprPath != "" {
		app.loadSpritePreview(sprPath)
	} else {
		fmt.Fprintf(os.Stderr, "SPR file not found for: %s\n", path)
		// Debug: List files in same directory to help find the SPR
		dir := filepath.Dir(path)
		fmt.Fprintf(os.Stderr, "Looking for SPR files in: %s\n", dir)
		for _, f := range app.flatFiles {
			fNorm := strings.ReplaceAll(f, "\\", "/")
			if strings.HasPrefix(fNorm, dir) && strings.HasSuffix(strings.ToLower(fNorm), ".spr") {
				fmt.Fprintf(os.Stderr, "  Found SPR: %s\n", fNorm)
			}
		}
	}

	app.previewLastTime = time.Now()
}

// decodeTGA decodes a TGA image file.
// Supports uncompressed true-color (type 2) and RLE compressed (type 10) TGA files,
// which are the formats commonly used in Ragnarok Online.
func decodeTGA(data []byte) (image.Image, error) {
	if len(data) < 18 {
		return nil, fmt.Errorf("TGA data too short")
	}

	// TGA header
	idLength := int(data[0])
	colorMapType := data[1]
	imageType := data[2]
	// colorMapSpec: bytes 3-7 (skip for now)
	// imageSpec: bytes 8-17
	xOrigin := int(data[8]) | int(data[9])<<8
	yOrigin := int(data[10]) | int(data[11])<<8
	width := int(data[12]) | int(data[13])<<8
	height := int(data[14]) | int(data[15])<<8
	bpp := int(data[16])
	descriptor := data[17]

	_ = xOrigin
	_ = yOrigin

	// Check supported formats
	if colorMapType != 0 {
		return nil, fmt.Errorf("color-mapped TGA not supported")
	}
	if imageType != 2 && imageType != 10 {
		return nil, fmt.Errorf("unsupported TGA type %d (only uncompressed/RLE true-color supported)", imageType)
	}
	if bpp != 24 && bpp != 32 {
		return nil, fmt.Errorf("unsupported TGA bit depth %d (only 24/32 supported)", bpp)
	}

	// Skip ID field
	offset := 18 + idLength
	if offset > len(data) {
		return nil, fmt.Errorf("TGA data truncated")
	}
	pixelData := data[offset:]

	// Create image
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	bytesPerPixel := bpp / 8

	// Check if image is flipped (bit 5 of descriptor = top-to-bottom)
	topToBottom := (descriptor & 0x20) != 0

	if imageType == 2 {
		// Uncompressed
		expectedSize := width * height * bytesPerPixel
		if len(pixelData) < expectedSize {
			return nil, fmt.Errorf("TGA pixel data truncated")
		}

		for y := 0; y < height; y++ {
			destY := y
			if !topToBottom {
				destY = height - 1 - y
			}
			for x := 0; x < width; x++ {
				i := (y*width + x) * bytesPerPixel
				b := pixelData[i]
				g := pixelData[i+1]
				r := pixelData[i+2]
				a := uint8(255)
				if bytesPerPixel == 4 {
					a = pixelData[i+3]
				}
				img.SetRGBA(x, destY, color.RGBA{R: r, G: g, B: b, A: a})
			}
		}
	} else {
		// RLE compressed (type 10)
		pixelCount := width * height
		pixelIdx := 0
		dataIdx := 0

		for pixelIdx < pixelCount && dataIdx < len(pixelData) {
			packet := pixelData[dataIdx]
			dataIdx++
			count := int(packet&0x7F) + 1

			if packet&0x80 != 0 {
				// RLE packet - repeat single pixel
				if dataIdx+bytesPerPixel > len(pixelData) {
					break
				}
				b := pixelData[dataIdx]
				g := pixelData[dataIdx+1]
				r := pixelData[dataIdx+2]
				a := uint8(255)
				if bytesPerPixel == 4 {
					a = pixelData[dataIdx+3]
				}
				dataIdx += bytesPerPixel

				for i := 0; i < count && pixelIdx < pixelCount; i++ {
					x := pixelIdx % width
					y := pixelIdx / width
					destY := y
					if !topToBottom {
						destY = height - 1 - y
					}
					img.SetRGBA(x, destY, color.RGBA{R: r, G: g, B: b, A: a})
					pixelIdx++
				}
			} else {
				// Raw packet - read count pixels
				for i := 0; i < count && pixelIdx < pixelCount; i++ {
					if dataIdx+bytesPerPixel > len(pixelData) {
						break
					}
					b := pixelData[dataIdx]
					g := pixelData[dataIdx+1]
					r := pixelData[dataIdx+2]
					a := uint8(255)
					if bytesPerPixel == 4 {
						a = pixelData[dataIdx+3]
					}
					dataIdx += bytesPerPixel

					x := pixelIdx % width
					y := pixelIdx / width
					destY := y
					if !topToBottom {
						destY = height - 1 - y
					}
					img.SetRGBA(x, destY, color.RGBA{R: r, G: g, B: b, A: a})
					pixelIdx++
				}
			}
		}
	}

	return img, nil
}

// loadImagePreview loads an image file (BMP, TGA, JPG, PNG) for preview.
func (app *App) loadImagePreview(path string) {
	data, err := app.archive.Read(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading image: %v\n", err)
		return
	}

	// Decode image - use image.Decode which auto-detects format
	// BMP is registered via golang.org/x/image/bmp import
	// JPEG and PNG are in standard library
	var img image.Image
	ext := strings.ToLower(filepath.Ext(path))

	if ext == ".tga" {
		// TGA needs special handling (not in standard library)
		img, err = decodeTGA(data)
	} else {
		img, _, err = image.Decode(bytes.NewReader(data))
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error decoding image %s: %v\n", ext, err)
		return
	}

	// Convert to RGBA
	bounds := img.Bounds()
	rgba := image.NewRGBA(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			rgba.Set(x, y, img.At(x, y))
		}
	}

	// Create texture
	app.previewImage = backend.NewTextureFromRgba(rgba)
	app.previewImgSize = [2]int{bounds.Dx(), bounds.Dy()}
}

// loadTextPreview loads a text file for preview.
func (app *App) loadTextPreview(path string) {
	data, err := app.archive.Read(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading text file: %v\n", err)
		return
	}

	// Try to convert from EUC-KR to UTF-8 if it looks like Korean
	text := string(data)
	if hasHighBytes(data) {
		decoder := korean.EUCKR.NewDecoder()
		if decoded, _, err := transform.String(decoder, text); err == nil {
			text = decoded
		}
	}

	// Limit preview size to avoid performance issues
	const maxPreviewSize = 64 * 1024 // 64KB
	if len(text) > maxPreviewSize {
		text = text[:maxPreviewSize] + "\n\n... (truncated)"
	}

	app.previewText = text
}

// loadHexPreview loads raw bytes for hex preview.
func (app *App) loadHexPreview(path string) {
	data, err := app.archive.Read(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file: %v\n", err)
		return
	}

	app.previewHexSize = int64(len(data))

	// Limit hex preview to first 4KB
	const maxHexSize = 4 * 1024
	if len(data) > maxHexSize {
		app.previewHex = data[:maxHexSize]
	} else {
		app.previewHex = data
	}
}

// hasHighBytes checks if data contains non-ASCII bytes (potential EUC-KR).
func hasHighBytes(data []byte) bool {
	for _, b := range data {
		if b > 127 {
			return true
		}
	}
	return false
}

// sprImageToRGBA converts an SPR image to *image.RGBA.
func sprImageToRGBA(img *formats.SPRImage) *image.RGBA {
	rgba := image.NewRGBA(image.Rect(0, 0, int(img.Width), int(img.Height)))

	// Copy pixel data
	for y := 0; y < int(img.Height); y++ {
		for x := 0; x < int(img.Width); x++ {
			i := (y*int(img.Width) + x) * 4
			rgba.SetRGBA(x, y, color.RGBA{
				R: img.Pixels[i],
				G: img.Pixels[i+1],
				B: img.Pixels[i+2],
				A: img.Pixels[i+3],
			})
		}
	}

	return rgba
}

// renderSpritePreview renders the sprite preview with frame navigation.
func (app *App) renderSpritePreview() {
	if app.previewSPR == nil || len(app.previewTextures) == 0 {
		imgui.TextDisabled("Failed to load sprite")
		return
	}

	spr := app.previewSPR
	imgui.Text(fmt.Sprintf("Version: %s", spr.Version))
	imgui.Text(fmt.Sprintf("Images: %d", len(spr.Images)))

	if spr.Palette != nil {
		imgui.Text("Palette: Yes (256 colors)")
	}

	imgui.Separator()

	// Frame navigation
	imgui.Text(fmt.Sprintf("Frame: %d / %d", app.previewFrame+1, len(spr.Images)))
	imgui.SameLine()

	if imgui.Button("<") && app.previewFrame > 0 {
		app.previewFrame--
	}
	imgui.SameLine()
	if imgui.Button(">") && app.previewFrame < len(spr.Images)-1 {
		app.previewFrame++
	}

	// Zoom controls
	imgui.SameLine()
	imgui.Text("  Zoom:")
	imgui.SameLine()
	if imgui.Button("-") && app.previewZoom > 0.5 {
		app.previewZoom -= 0.5
	}
	imgui.SameLine()
	imgui.Text(fmt.Sprintf("%.1fx", app.previewZoom))
	imgui.SameLine()
	if imgui.Button("+") && app.previewZoom < 8.0 {
		app.previewZoom += 0.5
	}

	imgui.Separator()

	// Display current frame centered in available space
	if app.previewFrame < len(app.previewTextures) {
		tex := app.previewTextures[app.previewFrame]
		if tex != nil {
			img := spr.Images[app.previewFrame]
			w := float32(img.Width) * app.previewZoom
			h := float32(img.Height) * app.previewZoom

			// Center the image both horizontally and vertically
			avail := imgui.ContentRegionAvail()
			startX := imgui.CursorPosX()
			startY := imgui.CursorPosY()
			if w < avail.X {
				imgui.SetCursorPosX(startX + (avail.X-w)/2)
			}
			if h < avail.Y {
				imgui.SetCursorPosY(startY + (avail.Y-h)/2)
			}

			// Draw with checkerboard background to show transparency
			imgui.ImageWithBgV(
				tex.ID,
				imgui.NewVec2(w, h),
				imgui.NewVec2(0, 0),
				imgui.NewVec2(1, 1),
				imgui.NewVec4(0.2, 0.2, 0.2, 1.0), // Dark gray background
				imgui.NewVec4(1, 1, 1, 1),         // White tint (no tint)
			)
		}
	}
}

// renderAnimationPreview renders the animation preview (frame display only, controls in Actions panel).
func (app *App) renderAnimationPreview() {
	if app.previewACT == nil {
		imgui.TextDisabled("Failed to load animation")
		return
	}

	act := app.previewACT

	// Update animation timing
	if len(act.Actions) > 0 {
		action := act.Actions[app.previewAction]

		if app.previewPlaying && len(action.Frames) > 0 {
			// Get interval from ACT file (default 4 ticks if not specified)
			// ACT intervals are in game ticks; multiply by 24ms per tick for real time
			interval := float32(4.0) // default 4 ticks
			if app.previewAction < len(act.Intervals) && act.Intervals[app.previewAction] > 0 {
				interval = act.Intervals[app.previewAction]
			}

			// Convert ticks to milliseconds (24ms per tick is standard RO timing)
			// Apply minimum floor of 100ms for readability
			intervalMs := interval * 24.0
			if intervalMs < 100.0 {
				intervalMs = 100.0
			}

			elapsed := time.Since(app.previewLastTime).Milliseconds()
			if elapsed >= int64(intervalMs) {
				app.previewFrame = (app.previewFrame + 1) % len(action.Frames)
				app.previewLastTime = time.Now()
			}
		}

		// Render current frame layers
		if app.previewFrame < len(action.Frames) && app.previewSPR != nil {
			frame := action.Frames[app.previewFrame]
			app.renderACTFrame(&frame)
		} else if app.previewSPR == nil {
			imgui.TextDisabled("No sprite loaded (SPR file not found)")
		}
	}
}

// renderActionsPanel renders the Actions panel for ACT files.
func (app *App) renderActionsPanel() {
	if app.previewACT == nil {
		return
	}

	act := app.previewACT

	// Playback controls at top
	if app.previewPlaying {
		if imgui.ButtonV("Pause", imgui.NewVec2(-1, 0)) {
			app.previewPlaying = false
		}
	} else {
		if imgui.ButtonV("Play", imgui.NewVec2(-1, 0)) {
			app.previewPlaying = true
			app.previewLastTime = time.Now()
		}
	}

	imgui.Text("(Space to toggle)")

	imgui.Separator()

	// Frame info
	if len(act.Actions) > 0 && app.previewAction < len(act.Actions) {
		action := act.Actions[app.previewAction]
		imgui.Text(fmt.Sprintf("Frame: %d / %d", app.previewFrame+1, len(action.Frames)))

		// Frame navigation
		if imgui.Button("<##frame") && app.previewFrame > 0 {
			app.previewFrame--
		}
		imgui.SameLine()
		if imgui.Button(">##frame") && app.previewFrame < len(action.Frames)-1 {
			app.previewFrame++
		}
	}

	imgui.Separator()
	imgui.Text("Actions:")

	// Scrollable action list
	if imgui.BeginChildStrV("ActionList", imgui.NewVec2(0, 0), imgui.ChildFlagsBorders, 0) {
		for i := 0; i < len(act.Actions); i++ {
			action := act.Actions[i]
			name := getActionName(i)
			label := fmt.Sprintf("%d: %s (%d)", i, name, len(action.Frames))

			isSelected := i == app.previewAction
			if imgui.SelectableBoolV(label, isSelected, 0, imgui.NewVec2(0, 0)) {
				app.previewAction = i
				app.previewFrame = 0
			}
		}
	}
	imgui.EndChild()
}

// getActionName returns a standard RO action name for the given index.
func getActionName(index int) string {
	// Standard RO action indices (may vary by sprite type)
	names := []string{
		"Idle",        // 0
		"Walk",        // 1
		"Sit",         // 2
		"Pick Up",     // 3
		"Standby",     // 4
		"Attack 1",    // 5
		"Damage",      // 6
		"Die",         // 7
		"Attack 2",    // 8
		"Attack 3",    // 9
		"Dead",        // 10
		"Skill Cast",  // 11
		"Skill Ready", // 12
		"Freeze",      // 13
	}

	if index < len(names) {
		return names[index]
	}
	return fmt.Sprintf("Action %d", index)
}

// renderACTFrame renders a single ACT frame with all its layers.
func (app *App) renderACTFrame(frame *formats.Frame) {
	if len(frame.Layers) == 0 {
		imgui.TextDisabled("Empty frame")
		return
	}

	// For now, just render the first valid layer's sprite
	validLayerFound := false
	allLayersEmpty := true

	for _, layer := range frame.Layers {
		if layer.SpriteID >= 0 {
			allLayersEmpty = false
			break
		}
	}

	// Show informative message for empty frames (common in garment/accessory ACT files)
	if allLayersEmpty {
		imgui.TextDisabled("Frame has no sprites")
		imgui.TextDisabled("(Accessory/garment overlay - uses base sprite)")
		return
	}

	for _, layer := range frame.Layers {
		if layer.SpriteID < 0 {
			continue
		}

		// Calculate actual sprite index based on sprite type
		// Type 0 = indexed (palette), Type 1 = RGBA (true-color)
		// RGBA sprites are stored after indexed sprites in the SPR file
		spriteIndex := int(layer.SpriteID)
		if layer.SpriteType == 1 && app.previewSPR != nil {
			spriteIndex += app.previewSPR.IndexedCount
		}

		if spriteIndex >= len(app.previewTextures) {
			continue
		}

		tex := app.previewTextures[spriteIndex]
		if tex == nil {
			continue
		}

		validLayerFound = true
		img := app.previewSPR.Images[spriteIndex]
		w := float32(img.Width) * app.previewZoom * layer.ScaleX
		h := float32(img.Height) * app.previewZoom * layer.ScaleY

		// Center the image both horizontally and vertically
		avail := imgui.ContentRegionAvail()
		startX := imgui.CursorPosX()
		startY := imgui.CursorPosY()
		if w < avail.X {
			imgui.SetCursorPosX(startX + (avail.X-w)/2)
		}
		if h < avail.Y {
			imgui.SetCursorPosY(startY + (avail.Y-h)/2)
		}

		// Apply layer color tint
		tint := imgui.NewVec4(
			float32(layer.Color[0])/255.0,
			float32(layer.Color[1])/255.0,
			float32(layer.Color[2])/255.0,
			float32(layer.Color[3])/255.0,
		)

		imgui.ImageWithBgV(
			tex.ID,
			imgui.NewVec2(w, h),
			imgui.NewVec2(0, 0),
			imgui.NewVec2(1, 1),
			imgui.NewVec4(0.2, 0.2, 0.2, 1.0),
			tint,
		)

		// Only render first valid layer for now (proper compositing would need DrawList)
		break
	}

	if !validLayerFound {
		imgui.TextDisabled("No renderable sprites in frame")
	}
}

// renderImagePreview renders an image (BMP, TGA, JPG, PNG) with zoom controls.
func (app *App) renderImagePreview() {
	if app.previewImage == nil {
		imgui.TextDisabled("Failed to load image")
		return
	}

	imgui.Text(fmt.Sprintf("Size: %d x %d", app.previewImgSize[0], app.previewImgSize[1]))

	// Zoom controls
	imgui.Text("Zoom:")
	imgui.SameLine()
	if imgui.Button("-##imgzoom") && app.previewZoom > 0.25 {
		app.previewZoom -= 0.25
	}
	imgui.SameLine()
	imgui.Text(fmt.Sprintf("%.0f%%", app.previewZoom*100))
	imgui.SameLine()
	if imgui.Button("+##imgzoom") && app.previewZoom < 4.0 {
		app.previewZoom += 0.25
	}
	imgui.SameLine()
	if imgui.Button("Reset##imgzoom") {
		app.previewZoom = 1.0
	}

	imgui.Separator()

	// Display image centered
	w := float32(app.previewImgSize[0]) * app.previewZoom
	h := float32(app.previewImgSize[1]) * app.previewZoom

	avail := imgui.ContentRegionAvail()
	startX := imgui.CursorPosX()
	startY := imgui.CursorPosY()
	if w < avail.X {
		imgui.SetCursorPosX(startX + (avail.X-w)/2)
	}
	if h < avail.Y {
		imgui.SetCursorPosY(startY + (avail.Y-h)/2)
	}

	imgui.ImageWithBgV(
		app.previewImage.ID,
		imgui.NewVec2(w, h),
		imgui.NewVec2(0, 0),
		imgui.NewVec2(1, 1),
		imgui.NewVec4(0.2, 0.2, 0.2, 1.0),
		imgui.NewVec4(1, 1, 1, 1),
	)
}

// renderTextPreview renders a text file with scrolling.
func (app *App) renderTextPreview() {
	if app.previewText == "" {
		imgui.TextDisabled("Empty file or failed to load")
		return
	}

	imgui.Text(fmt.Sprintf("Size: %d bytes", len(app.previewText)))
	imgui.Separator()

	// Scrollable text area
	flags := imgui.WindowFlagsHorizontalScrollbar
	if imgui.BeginChildStrV("TextPreview", imgui.NewVec2(0, 0), imgui.ChildFlagsBorders, flags) {
		imgui.TextUnformatted(app.previewText)
	}
	imgui.EndChild()
}

// renderHexPreview renders a hex dump of binary data.
func (app *App) renderHexPreview() {
	if app.previewHex == nil {
		imgui.TextDisabled("Failed to load file")
		return
	}

	imgui.Text(fmt.Sprintf("File size: %d bytes", app.previewHexSize))
	if int64(len(app.previewHex)) < app.previewHexSize {
		imgui.SameLine()
		imgui.TextDisabled(fmt.Sprintf("(showing first %d bytes)", len(app.previewHex)))
	}
	imgui.Separator()

	// Scrollable hex view
	flags := imgui.WindowFlagsHorizontalScrollbar
	if imgui.BeginChildStrV("HexPreview", imgui.NewVec2(0, 0), imgui.ChildFlagsBorders, flags) {
		// Render hex dump in classic format: offset | hex bytes | ascii
		const bytesPerLine = 16
		for offset := 0; offset < len(app.previewHex); offset += bytesPerLine {
			// Offset
			line := fmt.Sprintf("%08X  ", offset)

			// Hex bytes
			for i := 0; i < bytesPerLine; i++ {
				if offset+i < len(app.previewHex) {
					line += fmt.Sprintf("%02X ", app.previewHex[offset+i])
				} else {
					line += "   "
				}
				if i == 7 {
					line += " "
				}
			}

			// ASCII representation
			line += " |"
			for i := 0; i < bytesPerLine && offset+i < len(app.previewHex); i++ {
				b := app.previewHex[offset+i]
				if b >= 32 && b < 127 {
					line += string(b)
				} else {
					line += "."
				}
			}
			line += "|"

			imgui.Text(line)
		}
	}
	imgui.EndChild()
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
// Note: GRF files use EUC-KR encoding for Korean filenames.
func euckrToUTF8(s string) string {
	// Check if string contains non-ASCII bytes that might be EUC-KR
	hasHighBytes := false
	for i := 0; i < len(s); i++ {
		if s[i] > 127 {
			hasHighBytes = true
			break
		}
	}

	// Only decode if there are high bytes (potential EUC-KR)
	if !hasHighBytes {
		return s
	}

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
