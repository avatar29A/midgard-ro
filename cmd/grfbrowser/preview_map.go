// Map preview (GAT, GND, RSW) for GRF Browser.
package main

import (
	"fmt"
	"image"
	"image/color"
	"os"
	"strings"

	"github.com/AllenDang/cimgui-go/backend"
	"github.com/AllenDang/cimgui-go/imgui"

	"github.com/Faultbox/midgard-ro/pkg/formats"
)

// loadGATPreview loads a GAT file for preview.
func (app *App) loadGATPreview(path string) {
	data, err := app.archive.Read(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading GAT file: %v\n", err)
		return
	}

	gat, err := formats.ParseGAT(data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing GAT: %v\n", err)
		return
	}

	app.previewGAT = gat
	app.previewGATZoom = 1.0

	// Create visualization texture
	app.createGATTexture()
}

// createGATTexture creates a texture from GAT data for visualization.
func (app *App) createGATTexture() {
	if app.previewGAT == nil {
		return
	}

	gat := app.previewGAT
	width := int(gat.Width)
	height := int(gat.Height)

	// Create RGBA image
	rgba := image.NewRGBA(image.Rect(0, 0, width, height))

	// Color map for cell types
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			cell := gat.GetCell(x, y)
			if cell == nil {
				continue
			}

			var c color.RGBA
			switch cell.Type {
			case formats.GATWalkable:
				c = color.RGBA{R: 100, G: 200, B: 100, A: 255} // Green
			case formats.GATBlocked:
				c = color.RGBA{R: 200, G: 80, B: 80, A: 255} // Red
			case formats.GATWater:
				c = color.RGBA{R: 80, G: 150, B: 220, A: 255} // Blue
			case formats.GATWalkableWater:
				c = color.RGBA{R: 100, G: 200, B: 200, A: 255} // Cyan
			case formats.GATSnipeable:
				c = color.RGBA{R: 220, G: 200, B: 80, A: 255} // Yellow
			case formats.GATBlockedSnipe:
				c = color.RGBA{R: 220, G: 150, B: 80, A: 255} // Orange
			default:
				c = color.RGBA{R: 128, G: 128, B: 128, A: 255} // Gray
			}

			// Flip Y for display (GAT origin is bottom-left)
			rgba.SetRGBA(x, height-1-y, c)
		}
	}

	// Create texture
	app.previewGATTex = backend.NewTextureFromRgba(rgba)
}

// renderGATPreview renders the GAT visualization.
func (app *App) renderGATPreview() {
	if app.previewGAT == nil {
		imgui.TextDisabled("Failed to load GAT file")
		return
	}

	gat := app.previewGAT

	// Info
	imgui.Text(fmt.Sprintf("Size: %d x %d cells", gat.Width, gat.Height))
	imgui.Text(fmt.Sprintf("Version: %s", gat.Version))

	// Cell type counts
	counts := gat.CountByType()
	imgui.Text(fmt.Sprintf("Walkable: %d | Blocked: %d | Water: %d",
		counts[formats.GATWalkable]+counts[formats.GATWalkableWater],
		counts[formats.GATBlocked]+counts[formats.GATBlockedSnipe],
		counts[formats.GATWater]+counts[formats.GATWalkableWater]))

	// Altitude range
	minAlt, maxAlt := gat.GetAltitudeRange()
	imgui.Text(fmt.Sprintf("Altitude: %.1f to %.1f", minAlt, maxAlt))

	imgui.Separator()

	// Zoom controls (buttons only, keyboard +/- handled globally in handleKeyboardShortcuts)
	imgui.Text("Zoom:")
	imgui.SameLine()
	if imgui.Button("-##gatzoom") && app.previewGATZoom > 0.25 {
		app.previewGATZoom -= 0.25
	}
	imgui.SameLine()
	imgui.Text(fmt.Sprintf("%.0f%%", app.previewGATZoom*100))
	imgui.SameLine()
	if imgui.Button("+##gatzoom") && app.previewGATZoom < 8.0 {
		app.previewGATZoom += 0.25
	}
	imgui.SameLine()
	if imgui.Button("Fit##gatzoom") {
		// Calculate zoom to fit in available space
		avail := imgui.ContentRegionAvail()
		zoomX := avail.X / float32(gat.Width)
		zoomY := avail.Y / float32(gat.Height)
		app.previewGATZoom = min(zoomX, zoomY)
		if app.previewGATZoom < 0.1 {
			app.previewGATZoom = 0.1
		}
	}

	imgui.Separator()

	// Legend
	imgui.TextColored(imgui.NewVec4(0.4, 0.8, 0.4, 1), "Walkable")
	imgui.SameLine()
	imgui.TextColored(imgui.NewVec4(0.8, 0.3, 0.3, 1), "Blocked")
	imgui.SameLine()
	imgui.TextColored(imgui.NewVec4(0.3, 0.6, 0.9, 1), "Water")
	imgui.SameLine()
	imgui.TextColored(imgui.NewVec4(0.9, 0.8, 0.3, 1), "Snipeable")

	imgui.Separator()

	// Display texture
	if app.previewGATTex != nil {
		w := float32(gat.Width) * app.previewGATZoom
		h := float32(gat.Height) * app.previewGATZoom

		// Scrollable child region for large maps
		if imgui.BeginChildStrV("GATView", imgui.NewVec2(0, 0), imgui.ChildFlagsBorders, imgui.WindowFlagsHorizontalScrollbar) {
			imgui.ImageWithBgV(
				app.previewGATTex.ID,
				imgui.NewVec2(w, h),
				imgui.NewVec2(0, 0),
				imgui.NewVec2(1, 1),
				imgui.NewVec4(0.1, 0.1, 0.1, 1.0), // Dark background
				imgui.NewVec4(1, 1, 1, 1),         // No tint
			)
		}
		imgui.EndChild()
	}
}

// loadGNDPreview loads a GND file for preview.
func (app *App) loadGNDPreview(path string) {
	data, err := app.archive.Read(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading GND file: %v\n", err)
		return
	}

	gnd, err := formats.ParseGND(data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing GND: %v\n", err)
		return
	}

	app.previewGND = gnd
	app.previewGNDZoom = 1.0

	// Create height map visualization texture
	app.createGNDTexture()
}

// createGNDTexture creates a height map texture from GND data for visualization.
func (app *App) createGNDTexture() {
	if app.previewGND == nil {
		return
	}

	gnd := app.previewGND
	width := int(gnd.Width)
	height := int(gnd.Height)

	// Create RGBA image - height map visualization
	rgba := image.NewRGBA(image.Rect(0, 0, width, height))

	// Get altitude range for normalization
	minAlt, maxAlt := gnd.GetAltitudeRange()
	altRange := maxAlt - minAlt
	if altRange == 0 {
		altRange = 1 // Avoid division by zero
	}

	// Color map: low altitude = dark blue, high altitude = white/yellow
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			tile := gnd.GetTile(x, y)
			if tile == nil {
				continue
			}

			// Average altitude of the tile
			avgAlt := (tile.Altitude[0] + tile.Altitude[1] + tile.Altitude[2] + tile.Altitude[3]) / 4.0

			// Normalize to 0-1 (note: RO uses negative Y for height)
			normalized := (maxAlt - avgAlt) / altRange

			// Color gradient: blue (low) -> green -> yellow -> white (high)
			var r, g, b uint8
			if normalized < 0.25 {
				// Dark blue to blue
				t := normalized * 4
				r = uint8(20 * t)
				g = uint8(50 + 50*t)
				b = uint8(100 + 100*t)
			} else if normalized < 0.5 {
				// Blue to green
				t := (normalized - 0.25) * 4
				r = uint8(20 + 80*t)
				g = uint8(100 + 100*t)
				b = uint8(200 - 100*t)
			} else if normalized < 0.75 {
				// Green to yellow
				t := (normalized - 0.5) * 4
				r = uint8(100 + 155*t)
				g = uint8(200 + 55*t)
				b = uint8(100 - 50*t)
			} else {
				// Yellow to white
				t := (normalized - 0.75) * 4
				r = uint8(255)
				g = uint8(255)
				b = uint8(50 + 205*t)
			}

			// Highlight tiles with no top surface (cliffs/holes)
			if tile.TopSurface < 0 {
				r, g, b = 80, 20, 20 // Dark red for no surface
			}

			rgba.Set(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
		}
	}

	// Create texture
	app.previewGNDTex = backend.NewTextureFromRgba(rgba)
}

// renderGNDPreview renders the GND visualization.
func (app *App) renderGNDPreview() {
	if app.previewGND == nil {
		imgui.TextDisabled("Failed to load GND file")
		return
	}

	gnd := app.previewGND

	// Info
	imgui.Text(fmt.Sprintf("Size: %d x %d tiles", gnd.Width, gnd.Height))
	imgui.Text(fmt.Sprintf("Version: %s", gnd.Version))
	imgui.Text(fmt.Sprintf("Zoom Factor: %.1f", gnd.Zoom))
	imgui.Text(fmt.Sprintf("Textures: %d", len(gnd.Textures)))
	imgui.Text(fmt.Sprintf("Surfaces: %d", len(gnd.Surfaces)))
	imgui.Text(fmt.Sprintf("Lightmaps: %d (%dx%d)", len(gnd.Lightmaps), gnd.LightmapWidth, gnd.LightmapHeight))

	// Altitude range
	minAlt, maxAlt := gnd.GetAltitudeRange()
	imgui.Text(fmt.Sprintf("Altitude: %.1f to %.1f", minAlt, maxAlt))

	imgui.Separator()

	// Zoom controls
	imgui.Text("Zoom:")
	imgui.SameLine()
	if imgui.Button("-##gndzoom") && app.previewGNDZoom > 0.25 {
		app.previewGNDZoom -= 0.25
	}
	imgui.SameLine()
	imgui.Text(fmt.Sprintf("%.0f%%", app.previewGNDZoom*100))
	imgui.SameLine()
	if imgui.Button("+##gndzoom") && app.previewGNDZoom < 8.0 {
		app.previewGNDZoom += 0.25
	}
	imgui.SameLine()
	if imgui.Button("Fit##gndzoom") {
		avail := imgui.ContentRegionAvail()
		zoomX := avail.X / float32(gnd.Width)
		zoomY := avail.Y / float32(gnd.Height)
		app.previewGNDZoom = min(zoomX, zoomY)
		if app.previewGNDZoom < 0.1 {
			app.previewGNDZoom = 0.1
		}
	}

	imgui.Separator()

	// Texture list (collapsible)
	if imgui.TreeNodeExStrV("Textures", imgui.TreeNodeFlagsNone) {
		for i, tex := range gnd.Textures {
			imgui.Text(fmt.Sprintf("%d: %s", i, tex))
		}
		imgui.TreePop()
	}

	imgui.Separator()

	// Display height map texture
	if app.previewGNDTex != nil {
		w := float32(gnd.Width) * app.previewGNDZoom
		h := float32(gnd.Height) * app.previewGNDZoom

		// Scrollable child region for large maps
		if imgui.BeginChildStrV("GNDView", imgui.NewVec2(0, 0), imgui.ChildFlagsBorders, imgui.WindowFlagsHorizontalScrollbar) {
			imgui.ImageWithBgV(
				app.previewGNDTex.ID,
				imgui.NewVec2(w, h),
				imgui.NewVec2(0, 0),
				imgui.NewVec2(1, 1),
				imgui.NewVec4(0.1, 0.1, 0.1, 1.0), // Dark background
				imgui.NewVec4(1, 1, 1, 1),         // No tint
			)
		}
		imgui.EndChild()
	}
}

// loadRSWPreview loads a RSW file for preview.
func (app *App) loadRSWPreview(path string) {
	data, err := app.archive.Read(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading RSW file: %v\n", err)
		return
	}

	rsw, err := formats.ParseRSW(data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing RSW: %v\n", err)
		return
	}

	app.previewRSW = rsw

	// Auto-reload 3D view if already in 3D mode
	if app.map3DViewMode {
		app.initMap3DView()
	}
}

// renderRSWPreview renders the RSW world info panel.
func (app *App) renderRSWPreview() {
	if app.previewRSW == nil {
		imgui.TextDisabled("Failed to load RSW file")
		return
	}

	rsw := app.previewRSW

	// In 3D mode, just render the 3D view filling available space
	if app.map3DViewMode {
		app.renderMap3DView()
		return
	}

	if imgui.Button("3D View") {
		app.initMap3DView()
	}
	imgui.SameLine()
	imgui.TextDisabled("2D Info")
	imgui.SameLine()
	// Max models slider
	imgui.SetNextItemWidth(100)
	maxModels := int32(app.maxModelsLimit)
	if imgui.SliderInt("Max Models", &maxModels, 100, 5000) {
		app.maxModelsLimit = int(maxModels)
	}

	imgui.Separator()

	// Basic info
	imgui.Text(fmt.Sprintf("Version: %s", rsw.Version))
	imgui.Separator()

	// File references
	if imgui.TreeNodeExStrV("File References", imgui.TreeNodeFlagsDefaultOpen) {
		if rsw.GndFile != "" {
			imgui.Text(fmt.Sprintf("GND: %s", rsw.GndFile))
		}
		if rsw.GatFile != "" {
			imgui.Text(fmt.Sprintf("GAT: %s", rsw.GatFile))
		}
		if rsw.IniFile != "" {
			imgui.Text(fmt.Sprintf("INI: %s", rsw.IniFile))
		}
		if rsw.SrcFile != "" {
			imgui.Text(fmt.Sprintf("SRC: %s", rsw.SrcFile))
		}
		imgui.TreePop()
	}

	imgui.Separator()

	// Water settings
	if imgui.TreeNodeExStrV("Water Settings", imgui.TreeNodeFlagsDefaultOpen) {
		imgui.Text(fmt.Sprintf("Level: %.2f", rsw.Water.Level))
		imgui.Text(fmt.Sprintf("Type: %d", rsw.Water.Type))
		imgui.Text(fmt.Sprintf("Wave Height: %.2f", rsw.Water.WaveHeight))
		imgui.Text(fmt.Sprintf("Wave Speed: %.2f", rsw.Water.WaveSpeed))
		imgui.Text(fmt.Sprintf("Wave Pitch: %.2f", rsw.Water.WavePitch))
		imgui.Text(fmt.Sprintf("Anim Speed: %d", rsw.Water.AnimSpeed))
		imgui.TreePop()
	}

	imgui.Separator()

	// Light settings
	if imgui.TreeNodeExStrV("Light Settings", imgui.TreeNodeFlagsDefaultOpen) {
		imgui.Text(fmt.Sprintf("Longitude: %d", rsw.Light.Longitude))
		imgui.Text(fmt.Sprintf("Latitude: %d", rsw.Light.Latitude))
		imgui.Text(fmt.Sprintf("Diffuse: (%.2f, %.2f, %.2f)", rsw.Light.Diffuse[0], rsw.Light.Diffuse[1], rsw.Light.Diffuse[2]))
		imgui.Text(fmt.Sprintf("Ambient: (%.2f, %.2f, %.2f)", rsw.Light.Ambient[0], rsw.Light.Ambient[1], rsw.Light.Ambient[2]))
		imgui.Text(fmt.Sprintf("Shadow Opacity: %.2f", rsw.Light.Opacity))
		imgui.TreePop()
	}

	imgui.Separator()

	// Object statistics
	counts := rsw.CountByType()
	if imgui.TreeNodeExStrV("Objects", imgui.TreeNodeFlagsDefaultOpen) {
		imgui.Text(fmt.Sprintf("Total: %d objects", len(rsw.Objects)))
		imgui.Text(fmt.Sprintf("Models: %d", counts[formats.RSWObjectModel]))
		imgui.Text(fmt.Sprintf("Lights: %d", counts[formats.RSWObjectLight]))
		imgui.Text(fmt.Sprintf("Sounds: %d", counts[formats.RSWObjectSound]))
		imgui.Text(fmt.Sprintf("Effects: %d", counts[formats.RSWObjectEffect]))
		imgui.TreePop()
	}

	imgui.Separator()

	// Model list (collapsible)
	models := rsw.GetModels()
	if len(models) > 0 {
		if imgui.TreeNodeExStrV(fmt.Sprintf("Model List (%d)", len(models)), imgui.TreeNodeFlagsNone) {
			// Use clipper for large lists
			for i, model := range models {
				if i > 100 {
					imgui.Text(fmt.Sprintf("... and %d more", len(models)-100))
					break
				}
				name := model.Name
				if name == "" {
					name = model.ModelName
				}
				// Convert EUC-KR to UTF-8 for Korean names
				imgui.Text(fmt.Sprintf("%d: %s", i, euckrToUTF8(name)))
			}
			imgui.TreePop()
		}
	}

	// Sound list (collapsible)
	sounds := rsw.GetSounds()
	if len(sounds) > 0 {
		if imgui.TreeNodeExStrV(fmt.Sprintf("Sound List (%d)", len(sounds)), imgui.TreeNodeFlagsNone) {
			for i, sound := range sounds {
				if i > 50 {
					imgui.Text(fmt.Sprintf("... and %d more", len(sounds)-50))
					break
				}
				imgui.Text(fmt.Sprintf("%d: %s", i, euckrToUTF8(sound.File)))
			}
			imgui.TreePop()
		}
	}

	// Light source list (collapsible)
	lights := rsw.GetLights()
	if len(lights) > 0 {
		if imgui.TreeNodeExStrV(fmt.Sprintf("Light Sources (%d)", len(lights)), imgui.TreeNodeFlagsNone) {
			for i, light := range lights {
				if i > 50 {
					imgui.Text(fmt.Sprintf("... and %d more", len(lights)-50))
					break
				}
				imgui.Text(fmt.Sprintf("%d: %s (range: %.1f)", i, euckrToUTF8(light.Name), light.Range))
			}
			imgui.TreePop()
		}
	}

	// Effect list (collapsible)
	effects := rsw.GetEffects()
	if len(effects) > 0 {
		if imgui.TreeNodeExStrV(fmt.Sprintf("Effects (%d)", len(effects)), imgui.TreeNodeFlagsNone) {
			for i, effect := range effects {
				if i > 50 {
					imgui.Text(fmt.Sprintf("... and %d more", len(effects)-50))
					break
				}
				imgui.Text(fmt.Sprintf("%d: %s (ID: %d)", i, euckrToUTF8(effect.Name), effect.EffectID))
			}
			imgui.TreePop()
		}
	}

	// Quadtree info
	if len(rsw.Quadtree) > 0 {
		imgui.Separator()
		imgui.Text(fmt.Sprintf("Quadtree nodes: %d", len(rsw.Quadtree)))
	}
}

// initMap3DView initializes the 3D map viewer with GND data.
func (app *App) initMap3DView() {
	if app.previewRSW == nil {
		return
	}

	// Get GND file path from RSW
	gndPath := "data/" + app.previewRSW.GndFile
	if !app.archive.Contains(gndPath) {
		fmt.Fprintf(os.Stderr, "GND file not found: %s\n", gndPath)
		return
	}

	// Load GND data
	gndData, err := app.archive.Read(gndPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading GND: %v\n", err)
		return
	}

	gnd, err := formats.ParseGND(gndData)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing GND: %v\n", err)
		return
	}

	// Create MapViewer if needed
	if app.mapViewer == nil {
		mv, err := NewMapViewer(512, 512)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating map viewer: %v\n", err)
			return
		}
		app.mapViewer = mv
	}

	// Apply settings from App
	app.mapViewer.MaxModels = app.maxModelsLimit
	app.mapViewer.Brightness = app.terrainBrightness

	// Texture loader function
	texLoader := func(path string) ([]byte, error) {
		return app.archive.Read(path)
	}

	// Load map into viewer
	if err := app.mapViewer.LoadMap(gnd, app.previewRSW, texLoader); err != nil {
		fmt.Fprintf(os.Stderr, "Error loading map: %v\n", err)
		return
	}

	// Print loading diagnostics
	app.mapViewer.PrintDiagnostics()

	app.map3DViewMode = true
}

// Track last mouse position for drag delta calculation
var mapViewerLastMousePos imgui.Vec2
var mapViewerWasDragging bool // Track if we were dragging camera to prevent click-to-move on release

// renderMap3DView renders the 3D map view filling available space.
func (app *App) renderMap3DView() {
	if app.mapViewer == nil {
		imgui.TextDisabled("Map viewer not initialized")
		return
	}

	// Handle keyboard input for camera movement
	var forward, right, up float32
	if imgui.IsKeyDown(imgui.KeyW) {
		forward = 1
	}
	if imgui.IsKeyDown(imgui.KeyS) {
		forward = -1
	}
	if imgui.IsKeyDown(imgui.KeyD) {
		right = 1
	}
	if imgui.IsKeyDown(imgui.KeyA) {
		right = -1
	}
	if imgui.IsKeyDown(imgui.KeyE) {
		up = 1
	}
	if imgui.IsKeyDown(imgui.KeyQ) {
		up = -1
	}

	if app.mapViewer.PlayMode {
		// Always call in Play mode to update IsMoving state
		app.mapViewer.HandlePlayMovement(forward, right, up)
	} else if forward != 0 || right != 0 || up != 0 {
		app.mapViewer.HandleOrbitMovement(forward, right, up)
	}

	// Get available space and resize render target to match
	avail := imgui.ContentRegionAvail()
	width := avail.X
	height := avail.Y
	if width < 100 {
		width = 100
	}
	if height < 100 {
		height = 100
	}

	// Resize render target to match display size (prevents blurry scaling)
	app.mapViewer.Resize(int32(width), int32(height))

	// Update model animations if playing
	if app.mapViewer.IsModelAnimationPlaying() {
		// Use 16ms as approximate frame delta (60 FPS)
		app.mapViewer.UpdateModelAnimation(16.0)
	}

	// Update player movement for click-to-move (in Play mode)
	if app.mapViewer.PlayMode {
		app.mapViewer.UpdatePlayerMovement(16.0) // ~60fps delta
	}

	// Render the map
	texID := app.mapViewer.Render()

	// Display with flipped V coordinates (OpenGL to ImGui)
	texRef := imgui.NewTextureRefTextureID(imgui.TextureID(texID))
	imgui.ImageWithBgV(
		*texRef,
		imgui.NewVec2(width, height),
		imgui.NewVec2(0, 1), // UV flipped
		imgui.NewVec2(1, 0),
		imgui.NewVec4(0.1, 0.1, 0.1, 1.0), // Dark background
		imgui.NewVec4(1, 1, 1, 1),         // No tint
	)

	// Get item position for click-to-select
	itemMin := imgui.ItemRectMin()

	// Handle mouse input on the image
	if imgui.IsItemHovered() {
		// Mouse drag for rotation
		mousePos := imgui.MousePos()
		if imgui.IsMouseDragging(imgui.MouseButtonLeft) {
			deltaX := mousePos.X - mapViewerLastMousePos.X
			deltaY := mousePos.Y - mapViewerLastMousePos.Y
			app.mapViewer.HandleMouseDrag(deltaX, deltaY)
			mapViewerWasDragging = true // Track that we were dragging
		}
		mapViewerLastMousePos = mousePos

		// Mouse wheel for zoom
		wheel := imgui.CurrentIO().MouseWheel()
		if wheel != 0 {
			app.mapViewer.HandleMouseWheel(wheel)
		}

		// Convert screen coords to local image coords
		localX := mousePos.X - itemMin.X
		localY := mousePos.Y - itemMin.Y

		// Double-click to select model
		if imgui.IsMouseDoubleClicked(imgui.MouseButtonLeft) {
			// Pick model at screen position
			modelIdx := app.mapViewer.PickModelAtScreen(localX, localY, width, height)
			if modelIdx >= 0 {
				app.mapViewer.SelectedIdx = modelIdx
				app.showPropertiesPanel = true
			}
		}

		// Single click handling (only if we weren't dragging)
		if imgui.IsMouseReleased(imgui.MouseButtonLeft) {
			if mapViewerWasDragging {
				// Was dragging camera, don't trigger click action
				mapViewerWasDragging = false
			} else if app.mapViewer.PlayMode {
				// In Play mode: click to move
				app.mapViewer.HandlePlayModeClick(localX, localY, width, height)
			} else {
				// In Orbit mode: deselect if click didn't hit any model
				modelIdx := app.mapViewer.PickModelAtScreen(localX, localY, width, height)
				if modelIdx < 0 {
					app.mapViewer.SelectedIdx = -1
					app.showPropertiesPanel = false
				}
			}
		}
	}
}

// renderMapControlsPanel renders the map controls in the right panel.
func (app *App) renderMapControlsPanel() {
	if app.mapViewer == nil {
		return
	}

	// Camera section
	imgui.Text("Camera")
	imgui.Separator()

	// Zoom slider with label (use FollowCam in play mode, OrbitCam otherwise)
	imgui.Text("Zoom:")
	var zoom float32
	if app.mapViewer.PlayMode {
		zoom = app.mapViewer.FollowCam.Distance
	} else {
		zoom = app.mapViewer.OrbitCam.Distance
	}
	imgui.SetNextItemWidth(-1)
	if imgui.SliderFloatV("##Zoom", &zoom, 50, 2000, "%.0f", imgui.SliderFlagsNone) {
		if app.mapViewer.PlayMode {
			app.mapViewer.FollowCam.Distance = zoom
		} else {
			app.mapViewer.OrbitCam.Distance = zoom
		}
	}

	// Camera mode buttons
	if app.mapViewer.PlayMode {
		if imgui.ButtonV("Orbit Mode", imgui.NewVec2(-1, 0)) {
			app.mapViewer.TogglePlayMode()
		}
	} else {
		if imgui.ButtonV("Play", imgui.NewVec2(-1, 0)) {
			// Load player character if not already loaded
			if app.mapViewer.Player == nil && app.archive != nil {
				var spritePath string
				var headPath string

				// Search for Novice body sprite (midgarts approach)
				// Path: data/sprite/인간족/몸통/남/초보자_남.spr
				// EUC-KR bytes from midgarts:
				// 인간족 = {0xC0, 0xCE, 0xB0, 0xA3, 0xC1, 0xB7}
				// 몸통 = {0xB8, 0xF6, 0xC5, 0xEB}
				// 초보자 = {0xC3, 0xCA, 0xBA, 0xB8, 0xC0, 0xDA}
				// 남 = {0xB3, 0xB2}
				fmt.Println("Searching for Novice body sprite...")
				humanFolderBytes := []byte{0xC0, 0xCE, 0xB0, 0xA3, 0xC1, 0xB7} // 인간족
				bodyFolderBytes := []byte{0xB8, 0xF6, 0xC5, 0xEB}              // 몸통
				noviceBytes := []byte{0xC3, 0xCA, 0xBA, 0xB8, 0xC0, 0xDA}      // 초보자
				maleBytes := []byte{0xB3, 0xB2}                                // 남

				// First: look for Novice specifically in 인간족/몸통/남 folder (NOT costume subfolder)
				// Base sprite path: data/sprite/인간족/몸통/남/초보자_남.spr (6 parts)
				// Costume path: data/sprite/인간족/몸통/남/costume_1/초보자_남_1.spr (7 parts)
				for _, f := range app.flatFiles {
					if strings.HasSuffix(f, ".spr") && strings.HasPrefix(f, "data/sprite/") {
						// Skip costume variants
						if strings.Contains(f, "costume") {
							continue
						}
						// MUST have 인간족 folder AND 몸통 folder AND 초보자 name
						if strings.Contains(f, string(humanFolderBytes)) &&
							strings.Contains(f, string(bodyFolderBytes)) &&
							strings.Contains(f, string(noviceBytes)) {
							// Check path depth: base sprite has exactly 6 parts
							parts := strings.Split(f, "/")
							if len(parts) == 6 {
								spritePath = f
								fmt.Printf("  Found Novice body: %s\n", f)
								break
							}
						}
					}
				}

				// Fallback: any body sprite in 인간족/몸통/남 folder
				if spritePath == "" {
					fmt.Println("  Novice not found, searching for any body in human folder...")
					for _, f := range app.flatFiles {
						if strings.HasSuffix(f, ".spr") &&
							strings.HasPrefix(f, "data/sprite/") &&
							strings.Contains(f, string(humanFolderBytes)) &&
							strings.Contains(f, string(bodyFolderBytes)) &&
							strings.Contains(f, string(maleBytes)) {
							spritePath = f
							fmt.Printf("  Found body: %s\n", f)
							break
						}
					}
				}

				// Final fallback to b_novice
				if spritePath == "" {
					fmt.Println("  No body sprite found, trying b_novice fallback...")
					for _, f := range app.flatFiles {
						fLower := strings.ToLower(f)
						if strings.HasSuffix(fLower, "/b_novice.spr") {
							spritePath = f
							fmt.Printf("  Fallback to b_novice: %s\n", f)
							break
						}
					}
				}

				// Search for head sprite #1
				// Head sprites are in paths like data/sprite/인간족/머리통/남/1_남.spr
				// 머리통 = {0xB8, 0xD3, 0xB8, 0xAE, 0xC5, 0xEB} (head folder)
				fmt.Println("Searching for head sprites...")
				headFolderBytes := []byte{0xB8, 0xD3, 0xB8, 0xAE, 0xC5, 0xEB} // 머리통

				// Look for head #1 in 인간족/머리통/남 folder
				for _, f := range app.flatFiles {
					if strings.HasPrefix(f, "data/sprite/") &&
						strings.HasSuffix(f, ".spr") &&
						strings.Contains(f, string(humanFolderBytes)) &&
						strings.Contains(f, string(headFolderBytes)) &&
						strings.Contains(f, string(maleBytes)) {
						parts := strings.Split(f, "/")
						if len(parts) >= 5 {
							filename := parts[len(parts)-1]
							// Look for head #1: exactly "1_" at start
							if len(filename) >= 4 && filename[0] == '1' && filename[1] == '_' {
								headPath = f
								fmt.Printf("  Found head #1: %s\n", f)
								break
							}
						}
					}
				}

				texLoader := func(path string) ([]byte, error) {
					return app.archive.Read(path)
				}

				if spritePath != "" {
					// Found sprite, pass the path to loader
					actPath := spritePath[:len(spritePath)-4] + ".act"
					headActPath := ""
					if headPath != "" {
						headActPath = headPath[:len(headPath)-4] + ".act"
					}
					if err := app.mapViewer.LoadPlayerCharacterFromPath(texLoader, spritePath, actPath, headPath, headActPath); err != nil {
						fmt.Fprintf(os.Stderr, "Error loading player sprite: %v\n", err)
					}
				} else {
					// Fallback to default search
					if err := app.mapViewer.LoadPlayerCharacter(texLoader); err != nil {
						fmt.Fprintf(os.Stderr, "Error loading player: %v\n", err)
					}
				}
			}
			app.mapViewer.TogglePlayMode()
		}
	}

	// Character section (only in Play mode)
	if app.mapViewer.PlayMode && app.mapViewer.Player != nil {
		imgui.Spacing()
		imgui.Spacing()
		imgui.Text("Character")
		imgui.Separator()

		imgui.Text("Speed:")
		speed := app.mapViewer.Player.MoveSpeed
		imgui.SetNextItemWidth(-1)
		if imgui.SliderFloatV("##CharSpeed", &speed, 10.0, 150.0, "%.0f", imgui.SliderFlagsNone) {
			app.mapViewer.Player.MoveSpeed = speed
		}
	}

	imgui.Spacing()
	imgui.Spacing()

	if imgui.ButtonV("Reset Camera", imgui.NewVec2(-1, 0)) {
		app.mapViewer.Reset()
	}

	imgui.Spacing()
	imgui.Spacing()

	// Fog section
	imgui.Text("Fog")
	imgui.Separator()

	fogEnabled := app.mapViewer.FogEnabled
	if imgui.Checkbox("Enabled", &fogEnabled) {
		app.mapViewer.FogEnabled = fogEnabled
	}

	imgui.Text("Near:")
	fogNear := app.mapViewer.FogNear
	imgui.SetNextItemWidth(-1)
	if imgui.SliderFloatV("##FogNear", &fogNear, 10, 500, "%.0f", imgui.SliderFlagsNone) {
		app.mapViewer.FogNear = fogNear
	}

	imgui.Text("Far:")
	fogFar := app.mapViewer.FogFar
	imgui.SetNextItemWidth(-1)
	if imgui.SliderFloatV("##FogFar", &fogFar, 100, 2000, "%.0f", imgui.SliderFlagsNone) {
		app.mapViewer.FogFar = fogFar
	}

	imgui.Spacing()
	imgui.Spacing()

	// Lighting section
	imgui.Text("Lighting")
	imgui.Separator()

	imgui.Text("Brightness:")
	brightness := app.terrainBrightness
	imgui.SetNextItemWidth(-1)
	if imgui.SliderFloatV("##Brightness", &brightness, 0.5, 3.0, "%.2f", imgui.SliderFlagsNone) {
		app.terrainBrightness = brightness
		app.mapViewer.Brightness = brightness
	}

	imgui.Spacing()
	imgui.Spacing()

	// Model section
	imgui.Text("Models")
	imgui.Separator()

	imgui.Text("Max Models:")
	maxModels := int32(app.maxModelsLimit)
	imgui.SetNextItemWidth(-1)
	if imgui.SliderIntV("##MaxModels", &maxModels, 100, 5000, "%d", imgui.SliderFlagsNone) {
		app.maxModelsLimit = int(maxModels)
	}

	if imgui.ButtonV("Reload Map", imgui.NewVec2(-1, 0)) {
		app.mapViewer.MaxModels = app.maxModelsLimit
		app.initMap3DView()
	}

	// Debug: Force all two-sided
	forceTwo := app.mapViewer.ForceAllTwoSided
	if imgui.Checkbox("Force Two-Sided", &forceTwo) {
		app.mapViewer.ForceAllTwoSided = forceTwo
		// Need to reload to apply
		app.initMap3DView()
	}
	imgui.SameLineV(0, 5)
	imgui.TextDisabled("(?)")
	if imgui.IsItemHovered() {
		imgui.SetTooltip("Render all faces from both sides (reloads map)")
	}

	imgui.Spacing()
	imgui.Spacing()

	// Animation section
	imgui.Text("Animation")
	imgui.Separator()

	animCount := app.mapViewer.GetAnimatedModelCount()
	imgui.Text(fmt.Sprintf("Animated Models: %d", animCount))

	if animCount > 0 {
		if app.mapViewer.IsModelAnimationPlaying() {
			if imgui.ButtonV("Pause", imgui.NewVec2(-1, 0)) {
				app.mapViewer.PauseModelAnimation()
			}
		} else {
			if imgui.ButtonV("Play", imgui.NewVec2(-1, 0)) {
				app.mapViewer.PlayModelAnimation()
			}
		}
	}

	imgui.Spacing()
	imgui.Spacing()

	// View toggle
	if imgui.ButtonV("2D Info View", imgui.NewVec2(-1, 0)) {
		app.map3DViewMode = false
	}

	imgui.Spacing()
	imgui.Spacing()

	// Scene Models section
	imgui.Text("Scene Models")
	imgui.Separator()

	// Model stats
	visCount := app.mapViewer.GetVisibleCount()
	totalCount := len(app.mapViewer.ModelGroups)
	imgui.Text(fmt.Sprintf("%d groups, %d/%d visible", totalCount, visCount, len(app.mapViewer.models)))

	// Filter input
	imgui.Text("Filter:")
	imgui.SetNextItemWidth(-1)
	if imgui.InputTextWithHint("##modelfilter", "Filter models...", &app.modelFilterText, 0, nil) {
		app.mapViewer.ModelFilter = app.modelFilterText
	}

	// All/None buttons
	if imgui.Button("All##models") {
		app.mapViewer.SetAllModelsVisible(true)
	}
	imgui.SameLine()
	if imgui.Button("None##models") {
		app.mapViewer.SetAllModelsVisible(false)
	}

	imgui.Spacing()

	// Model groups tree
	filterLower := strings.ToLower(app.modelFilterText)
	if imgui.BeginChildStrV("ModelTree", imgui.NewVec2(0, 0), imgui.ChildFlagsBorders, 0) {
		for groupIdx, group := range app.mapViewer.ModelGroups {
			// Apply filter
			if filterLower != "" && !strings.Contains(strings.ToLower(group.RSMName), filterLower) {
				continue
			}

			// Tree node for group (RSM name)
			nodeFlags := imgui.TreeNodeFlagsOpenOnArrow | imgui.TreeNodeFlagsOpenOnDoubleClick
			if len(group.Instances) == 1 {
				nodeFlags |= imgui.TreeNodeFlagsLeaf
			}

			// Checkbox for group visibility
			visible := group.AllVisible
			if imgui.Checkbox(fmt.Sprintf("##grp%d", groupIdx), &visible) {
				app.mapViewer.SetGroupVisibility(groupIdx, visible)
			}
			imgui.SameLine()

			// Group node label with instance count (convert Korean names)
			displayName := euckrToUTF8(group.RSMName)
			label := fmt.Sprintf("%s (%d)", displayName, len(group.Instances))
			nodeOpen := imgui.TreeNodeExStrV(fmt.Sprintf("%s##grp%d", label, groupIdx), nodeFlags)

			if nodeOpen {
				// Show instances
				for i, modelIdx := range group.Instances {
					model := app.mapViewer.GetModel(modelIdx)
					if model == nil {
						continue
					}

					// Instance checkbox
					instVisible := model.Visible
					if imgui.Checkbox(fmt.Sprintf("##inst%d_%d", groupIdx, i), &instVisible) {
						model.Visible = instVisible
						// Update group visibility state
						allVis := true
						for _, idx := range group.Instances {
							m := app.mapViewer.GetModel(idx)
							if m != nil && !m.Visible {
								allVis = false
								break
							}
						}
						app.mapViewer.ModelGroups[groupIdx].AllVisible = allVis
					}
					imgui.SameLine()

					// Instance label - show ID and position
					instLabel := fmt.Sprintf("Instance %d (%.0f, %.0f, %.0f)##inst%d_%d",
						model.instanceID,
						model.position[0], model.position[1], model.position[2],
						groupIdx, i)

					// Selectable for clicking
					selected := app.mapViewer.SelectedIdx == modelIdx
					if imgui.SelectableBoolPtrV(instLabel, &selected, imgui.SelectableFlagsAllowDoubleClick, imgui.NewVec2(0, 0)) {
						app.mapViewer.SelectedIdx = modelIdx
						app.showPropertiesPanel = true

						// Double-click to focus camera
						if imgui.IsMouseDoubleClicked(imgui.MouseButtonLeft) {
							app.mapViewer.FocusOnModel(modelIdx)
						}
					}
				}
				imgui.TreePop()
			}
		}
	}
	imgui.EndChild()
}
