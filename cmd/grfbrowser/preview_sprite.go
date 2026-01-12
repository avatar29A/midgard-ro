// Sprite and animation preview for GRF Browser.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/AllenDang/cimgui-go/backend"
	"github.com/AllenDang/cimgui-go/imgui"

	"github.com/Faultbox/midgard-ro/pkg/formats"
)

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

	// Timeline slider for multi-frame sprites
	if len(spr.Images) > 1 {
		frame := int32(app.previewFrame)
		imgui.SetNextItemWidth(200)
		if imgui.SliderIntV("##SprTimeline", &frame, 0, int32(len(spr.Images)-1), "%d", imgui.SliderFlagsNone) {
			app.previewFrame = int(frame)
		}
	}

	// Zoom controls
	imgui.Text("Zoom:")
	imgui.SameLine()
	if imgui.Button("-##zoom") && app.previewZoom > 0.5 {
		app.previewZoom -= 0.5
	}
	imgui.SameLine()
	imgui.Text(fmt.Sprintf("%.1fx", app.previewZoom))
	imgui.SameLine()
	if imgui.Button("+##zoom") && app.previewZoom < 8.0 {
		app.previewZoom += 0.5
	}
	imgui.SameLine()
	if imgui.Button("1:1") {
		app.previewZoom = 1.0
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

			// Apply speed multiplier
			if app.previewSpeed > 0 {
				intervalMs = intervalMs / app.previewSpeed
			}

			elapsed := time.Since(app.previewLastTime).Milliseconds()
			if elapsed >= int64(intervalMs) {
				nextFrame := app.previewFrame + 1
				if nextFrame >= len(action.Frames) {
					if app.previewLooping {
						nextFrame = 0
					} else {
						// Stop at last frame
						app.previewPlaying = false
						nextFrame = len(action.Frames) - 1
					}
				}
				app.previewFrame = nextFrame
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

	// Stop button (reset to frame 0)
	if imgui.ButtonV("Stop", imgui.NewVec2(-1, 0)) {
		app.previewPlaying = false
		app.previewFrame = 0
	}

	imgui.Text("(Space to toggle)")

	imgui.Separator()

	// Frame info and timeline
	if len(act.Actions) > 0 && app.previewAction < len(act.Actions) {
		action := act.Actions[app.previewAction]
		frameCount := len(action.Frames)
		imgui.Text(fmt.Sprintf("Frame: %d / %d", app.previewFrame+1, frameCount))

		// Frame navigation buttons
		if imgui.Button("<##frame") && app.previewFrame > 0 {
			app.previewFrame--
		}
		imgui.SameLine()
		if imgui.Button(">##frame") && app.previewFrame < frameCount-1 {
			app.previewFrame++
		}

		// Timeline slider
		if frameCount > 1 {
			frame := int32(app.previewFrame)
			imgui.SetNextItemWidth(-1)
			if imgui.SliderIntV("##Timeline", &frame, 0, int32(frameCount-1), "%d", imgui.SliderFlagsNone) {
				app.previewFrame = int(frame)
			}
		}
	}

	imgui.Separator()

	// Speed control
	imgui.Text("Speed:")
	imgui.SetNextItemWidth(-1)
	imgui.SliderFloatV("##Speed", &app.previewSpeed, 0.1, 3.0, "%.1fx", imgui.SliderFlagsNone)

	// Loop toggle
	imgui.Checkbox("Loop", &app.previewLooping)

	imgui.Separator()
	imgui.Text("Actions:")

	// Scrollable action list
	totalActions := len(act.Actions)
	if imgui.BeginChildStrV("ActionList", imgui.NewVec2(0, 0), imgui.ChildFlagsBorders, 0) {
		for i := 0; i < totalActions; i++ {
			action := act.Actions[i]
			name := getActionName(i, totalActions)
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

// Direction names for 8-direction sprites (standard RO order).
var directionNames = []string{
	"S", "SW", "W", "NW", "N", "NE", "E", "SE",
}

// Monster action type names (typically 5-8 action types with 8 directions each).
// Monsters have: Idle, Walk, Attack, Damage, Die (and sometimes more attacks).
var monsterActionNames = []string{
	"Idle",     // 0
	"Walk",     // 1
	"Attack",   // 2
	"Damage",   // 3
	"Die",      // 4
	"Attack 2", // 5 (some monsters)
	"Attack 3", // 6 (some monsters)
	"Special",  // 7 (some monsters)
}

// Player action type names (typically 13+ action types with 8 directions each).
// Players have more actions including Sit, Pick Up, Standby, multiple attacks, casting.
var playerActionNames = []string{
	"Idle",        // 0
	"Walk",        // 1
	"Sit",         // 2
	"Pick Up",     // 3
	"Standby",     // 4
	"Attack 1",    // 5
	"Damage",      // 6
	"Die",         // 7
	"Dead",        // 8
	"Attack 2",    // 9
	"Attack 3",    // 10
	"Skill Cast",  // 11
	"Skill Ready", // 12
	"Freeze",      // 13
}

// getActionName returns a standard RO action name for the given index.
// For sprites with 8 directions per action, shows "ActionType Dir" format.
// Uses different naming based on total action count to detect monster vs player sprites.
func getActionName(index, totalActions int) string {
	// Check if this looks like an 8-direction sprite (multiple of 8 actions)
	if totalActions >= 8 && totalActions%8 == 0 {
		actionType := index / 8
		direction := index % 8
		dirName := directionNames[direction]

		// Determine sprite type based on action count:
		// - Monster sprites: typically 40-64 actions (5-8 action types)
		// - Player sprites: typically 80+ actions (10+ action types)
		var typeName string
		actionTypeCount := totalActions / 8

		if actionTypeCount <= 8 {
			// Likely a monster sprite
			if actionType < len(monsterActionNames) {
				typeName = monsterActionNames[actionType]
			} else {
				typeName = fmt.Sprintf("Action%d", actionType)
			}
		} else {
			// Likely a player sprite
			if actionType < len(playerActionNames) {
				typeName = playerActionNames[actionType]
			} else {
				typeName = fmt.Sprintf("Action%d", actionType)
			}
		}

		return fmt.Sprintf("%s %s", typeName, dirName)
	}

	// Non-8-direction sprite (items, effects, etc.) - just show index
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
