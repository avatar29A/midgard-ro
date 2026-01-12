// GRF Browser - A graphical tool for browsing Ragnarok Online GRF archives.
package main

import (
	"flag"
	"fmt"
	_ "image/jpeg" // JPEG decoder
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/AllenDang/cimgui-go/backend"
	"github.com/AllenDang/cimgui-go/backend/sdlbackend"
	"github.com/AllenDang/cimgui-go/imgui"
	"github.com/go-gl/gl/v4.1-core/gl"
	"github.com/gopxl/beep/v2"
	"github.com/sqweek/dialog"
	_ "golang.org/x/image/bmp" // BMP decoder registration

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

	// Audio preview state (ADR-009 Stage 4)
	audioStreamer   beep.StreamSeekCloser // Audio stream
	audioFormat     beep.Format           // Audio format (sample rate, channels)
	audioCtrl       *beep.Ctrl            // Playback control (pause/resume)
	audioPlaying    bool                  // Is audio currently playing
	audioLength     int                   // Total samples
	audioSampleRate beep.SampleRate       // Sample rate for duration calc

	// GAT preview state (ADR-011)
	previewGAT     *formats.GAT     // Loaded GAT data
	previewGATTex  *backend.Texture // Rendered texture for GAT visualization
	previewGATZoom float32          // Zoom level for GAT view

	// GND preview state (ADR-011 Stage 2)
	previewGND     *formats.GND     // Loaded GND data
	previewGNDTex  *backend.Texture // Rendered height map texture
	previewGNDZoom float32          // Zoom level for GND view

	// RSW preview state (ADR-011 Stage 3)
	previewRSW *formats.RSW // Loaded RSW data

	// RSM preview state (ADR-012 Stage 2/3)
	previewRSM          *formats.RSM // Loaded RSM 3D model data
	modelViewer         *ModelViewer // 3D model renderer (ADR-012 Stage 3)
	magentaTransparency bool         // Enable magenta (255,0,255) as transparency key
}

var (
	speakerInitOnce sync.Once
	speakerInited   bool
)

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
		expandedPaths:       make(map[string]bool),
		filterSprites:       true,
		filterAnimations:    true,
		filterTextures:      true,
		filterModels:        true,
		filterMaps:          true,
		filterAudio:         true,
		filterOther:         true,
		screenshotDir:       "/tmp/grfbrowser",
		previewZoom:         1.0,
		magentaTransparency: true, // Enable magenta key transparency by default
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
	if app.modelViewer != nil {
		app.modelViewer.Destroy()
		app.modelViewer = nil
	}
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

	// GAT zoom controls: +/- to zoom, 0 to reset
	if app.previewGAT != nil {
		// + or = key to zoom in
		if imgui.IsKeyChordPressed(imgui.KeyChord(imgui.KeyEqual)) || imgui.IsKeyChordPressed(imgui.KeyChord(imgui.KeyKeypadAdd)) {
			if app.previewGATZoom < 8.0 {
				app.previewGATZoom += 0.25
			}
		}
		// - key to zoom out
		if imgui.IsKeyChordPressed(imgui.KeyChord(imgui.KeyMinus)) || imgui.IsKeyChordPressed(imgui.KeyChord(imgui.KeyKeypadSubtract)) {
			if app.previewGATZoom > 0.25 {
				app.previewGATZoom -= 0.25
			}
		}
		// 0 key to reset zoom
		if imgui.IsKeyChordPressed(imgui.KeyChord(imgui.Key0)) || imgui.IsKeyChordPressed(imgui.KeyChord(imgui.KeyKeypad0)) {
			app.previewGATZoom = 1.0
		}
	}

	// GND zoom controls: +/- to zoom, 0 to reset
	if app.previewGND != nil {
		// + or = key to zoom in
		if imgui.IsKeyChordPressed(imgui.KeyChord(imgui.KeyEqual)) || imgui.IsKeyChordPressed(imgui.KeyChord(imgui.KeyKeypadAdd)) {
			if app.previewGNDZoom < 8.0 {
				app.previewGNDZoom += 0.25
			}
		}
		// - key to zoom out
		if imgui.IsKeyChordPressed(imgui.KeyChord(imgui.KeyMinus)) || imgui.IsKeyChordPressed(imgui.KeyChord(imgui.KeyKeypadSubtract)) {
			if app.previewGNDZoom > 0.25 {
				app.previewGNDZoom -= 0.25
			}
		}
		// 0 key to reset zoom
		if imgui.IsKeyChordPressed(imgui.KeyChord(imgui.Key0)) || imgui.IsKeyChordPressed(imgui.KeyChord(imgui.KeyKeypad0)) {
			app.previewGNDZoom = 1.0
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
	imgui.Text("Type: " + getFileTypeName(ext))

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
	case ".wav":
		app.renderAudioPreview()
	case ".gat":
		app.renderGATPreview()
	case ".gnd":
		app.renderGNDPreview()
	case ".rsw":
		app.renderRSWPreview()
	case ".rsm":
		app.renderRSMPreview()
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
	case ".wav":
		app.loadAudioPreview(archivePath)
	case ".gat":
		app.loadGATPreview(archivePath)
	case ".gnd":
		app.loadGNDPreview(archivePath)
	case ".rsw":
		app.loadRSWPreview(archivePath)
	case ".rsm":
		app.loadRSMPreview(archivePath)
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

	// Stop and release audio (Stage 4)
	app.stopAudio()

	// Clear GAT preview (ADR-011)
	app.previewGAT = nil
	if app.previewGATTex != nil {
		app.previewGATTex.Release()
		app.previewGATTex = nil
	}

	// Clear GND preview (ADR-011 Stage 2)
	app.previewGND = nil
	if app.previewGNDTex != nil {
		app.previewGNDTex.Release()
		app.previewGNDTex = nil
	}

	// Clear RSW preview (ADR-011 Stage 3)
	app.previewRSW = nil

	// Clear RSM preview (ADR-012 Stage 2/3)
	app.previewRSM = nil
	// Note: modelViewer is reused, not destroyed here - just clear mesh on next load
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
