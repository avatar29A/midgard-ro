// GRF Browser - A graphical tool for browsing Ragnarok Online GRF archives.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/AllenDang/cimgui-go/backend"
	"github.com/AllenDang/cimgui-go/backend/sdlbackend"
	"github.com/AllenDang/cimgui-go/imgui"
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
	}

	// Create backend using the proper wrapper
	var err error
	app.backend, err = backend.CreateBackend(sdlbackend.NewSDLBackend())
	if err != nil {
		panic(fmt.Sprintf("failed to create backend: %v", err))
	}

	app.backend.SetBgColor(imgui.NewVec4(0.1, 0.1, 0.12, 1.0))
	app.backend.CreateWindow("GRF Browser", 1280, 800)

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
	// Handle keyboard shortcuts
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
