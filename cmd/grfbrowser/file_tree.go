// File tree management for GRF Browser.
package main

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/AllenDang/cimgui-go/imgui"
)

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
// Supports wildcard patterns: *.bmp, item_*.spr, etc.
func (app *App) matchesSearch(path string) bool {
	if app.searchText == "" {
		return true
	}

	search := strings.ToLower(app.searchText)
	pathLower := strings.ToLower(path)

	// Check if search contains wildcards
	if strings.ContainsAny(search, "*?") {
		// Use glob matching on filename only for patterns like *.bmp
		filename := filepath.Base(pathLower)
		if matched, _ := filepath.Match(search, filename); matched {
			return true
		}
		// Also try matching against full path for patterns like data/*.bmp
		if matched, _ := filepath.Match(search, pathLower); matched {
			return true
		}
		return false
	}

	// Default: substring search
	return strings.Contains(pathLower, search)
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
			icon := getFileIcon(child.Name)

			imgui.TreeNodeExStrV(icon+" "+child.Name, flags)

			// Auto-select when navigating with arrows (IsItemFocused), or on click/Enter
			if imgui.IsItemClicked() || imgui.IsItemFocused() {
				app.selectedPath = child.Path
				app.selectedOriginalPath = child.OriginalPath
			}
		}
	}
}
