// 3D model preview (RSM) for GRF Browser (ADR-012 Stage 2/3).
package main

import (
	"fmt"
	"os"

	"github.com/AllenDang/cimgui-go/imgui"

	"github.com/Faultbox/midgard-ro/pkg/formats"
)

// lastMousePos tracks previous mouse position for drag delta calculation.
var lastMousePos imgui.Vec2

// loadRSMPreview loads a RSM file for preview.
func (app *App) loadRSMPreview(path string) {
	data, err := app.archive.Read(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading RSM file: %v\n", err)
		return
	}

	rsm, err := formats.ParseRSM(data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing RSM: %v\n", err)
		return
	}

	app.previewRSM = rsm

	// Initialize 3D viewer if needed (ADR-012 Stage 3)
	if app.modelViewer == nil {
		var err error
		app.modelViewer, err = NewModelViewer(512, 512)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating model viewer: %v\n", err)
			return
		}
	}

	// Load model into 3D viewer with texture loader
	// Note: loadTextures() already builds the full path (data/texture/...)
	textureLoader := func(fullPath string) ([]byte, error) {
		return app.archive.Read(fullPath)
	}
	if err := app.modelViewer.LoadModel(rsm, textureLoader, app.magentaTransparency); err != nil {
		fmt.Fprintf(os.Stderr, "Error loading model: %v\n", err)
	}
}

// renderRSMPreview renders the RSM model 3D view and info panel.
func (app *App) renderRSMPreview() {
	if app.previewRSM == nil {
		imgui.TextDisabled("Failed to load RSM file")
		return
	}

	rsm := app.previewRSM

	// 3D Preview section (ADR-012 Stage 3)
	if app.modelViewer != nil {
		// Render 3D view to texture
		textureID := app.modelViewer.Render()

		// Get viewer dimensions
		viewerW := float32(app.modelViewer.width)
		viewerH := float32(app.modelViewer.height)

		// Calculate display size to fit in available space
		avail := imgui.ContentRegionAvail()
		maxPreviewHeight := avail.Y * 0.5 // Use up to 50% of space for 3D view
		if maxPreviewHeight > viewerH {
			maxPreviewHeight = viewerH
		}

		// Maintain aspect ratio
		aspectRatio := viewerW / viewerH
		displayW := maxPreviewHeight * aspectRatio
		displayH := maxPreviewHeight
		if displayW > avail.X {
			displayW = avail.X
			displayH = displayW / aspectRatio
		}

		// Center the image
		startX := imgui.CursorPosX()
		if displayW < avail.X {
			imgui.SetCursorPosX(startX + (avail.X-displayW)/2)
		}

		// Display rendered texture (flip V for OpenGL)
		texRef := imgui.NewTextureRefTextureID(imgui.TextureID(textureID))
		imgui.ImageWithBgV(
			*texRef,
			imgui.NewVec2(displayW, displayH),
			imgui.NewVec2(0, 1), // UV flipped
			imgui.NewVec2(1, 0),
			imgui.NewVec4(0.15, 0.15, 0.15, 1.0), // Dark background
			imgui.NewVec4(1, 1, 1, 1),            // White tint (no tint)
		)

		// Handle mouse input when hovering the image
		if imgui.IsItemHovered() {
			// Mouse drag for rotation
			mousePos := imgui.MousePos()
			if imgui.IsMouseDragging(imgui.MouseButtonLeft) {
				deltaX := mousePos.X - lastMousePos.X
				deltaY := mousePos.Y - lastMousePos.Y
				app.modelViewer.HandleMouseDrag(deltaX, deltaY)
			}
			lastMousePos = mousePos

			// Mouse wheel for zoom
			wheel := imgui.CurrentIO().MouseWheel()
			if wheel != 0 {
				app.modelViewer.HandleMouseWheel(wheel)
			}
		}

		// Controls row
		if imgui.Button("Reset View") {
			app.modelViewer.Reset()
		}
		imgui.SameLine()
		imgui.TextDisabled("(Drag to rotate, scroll to zoom)")

		// Magenta transparency checkbox
		if imgui.Checkbox("Magenta Transparency", &app.magentaTransparency) {
			// Reload model with new transparency setting
			textureLoader := func(fullPath string) ([]byte, error) {
				return app.archive.Read(fullPath)
			}
			if err := app.modelViewer.LoadModel(rsm, textureLoader, app.magentaTransparency); err != nil {
				fmt.Fprintf(os.Stderr, "Error reloading model: %v\n", err)
			}
		}
		if imgui.IsItemHovered() {
			imgui.SetTooltip("Treat RGB(255,0,255) as transparent")
		}
	}

	imgui.Separator()

	// Basic info
	imgui.Text(fmt.Sprintf("Version: %s", rsm.Version))
	imgui.Text(fmt.Sprintf("Animation: %d ms", rsm.AnimLength))
	imgui.Text(fmt.Sprintf("Shading: %s", rsm.Shading))
	imgui.Text(fmt.Sprintf("Alpha: %.2f", rsm.Alpha))

	imgui.Separator()

	// Statistics
	imgui.Text(fmt.Sprintf("Nodes: %d", len(rsm.Nodes)))
	imgui.Text(fmt.Sprintf("Total Vertices: %d", rsm.GetTotalVertexCount()))
	imgui.Text(fmt.Sprintf("Total Faces: %d", rsm.GetTotalFaceCount()))
	imgui.Text(fmt.Sprintf("Volume Boxes: %d", len(rsm.VolumeBoxes)))

	if rsm.HasAnimation() {
		imgui.TextColored(imgui.NewVec4(0.4, 0.8, 0.4, 1), "Has Animation")
	}

	imgui.Separator()

	// Texture list
	if len(rsm.Textures) > 0 {
		if imgui.TreeNodeExStrV(fmt.Sprintf("Textures (%d)", len(rsm.Textures)), imgui.TreeNodeFlagsNone) {
			for i, tex := range rsm.Textures {
				imgui.Text(fmt.Sprintf("%d: %s", i, tex))
			}
			imgui.TreePop()
		}
	}

	// Node hierarchy (collapsed by default now that we have 3D view)
	if len(rsm.Nodes) > 0 {
		if imgui.TreeNodeExStrV(fmt.Sprintf("Node Hierarchy (%d)", len(rsm.Nodes)), imgui.TreeNodeFlagsNone) {
			// Find and render root node first
			root := rsm.GetRootNode()
			if root != nil {
				app.renderRSMNodeTree(rsm, root, 0)
			} else {
				// If no explicit root, render all top-level nodes
				for i := range rsm.Nodes {
					node := &rsm.Nodes[i]
					if node.Parent == "" {
						app.renderRSMNodeTree(rsm, node, 0)
					}
				}
			}
			imgui.TreePop()
		}
	}

	// Volume boxes (collapsible)
	if len(rsm.VolumeBoxes) > 0 {
		if imgui.TreeNodeExStrV(fmt.Sprintf("Volume Boxes (%d)", len(rsm.VolumeBoxes)), imgui.TreeNodeFlagsNone) {
			for i, box := range rsm.VolumeBoxes {
				imgui.Text(fmt.Sprintf("%d: Size(%.1f, %.1f, %.1f) Pos(%.1f, %.1f, %.1f)",
					i,
					box.Size[0], box.Size[1], box.Size[2],
					box.Position[0], box.Position[1], box.Position[2]))
			}
			imgui.TreePop()
		}
	}
}

// renderRSMNodeTree recursively renders a node and its children.
func (app *App) renderRSMNodeTree(rsm *formats.RSM, node *formats.RSMNode, depth int) {
	if node == nil || depth > 10 { // Prevent infinite recursion
		return
	}

	// Build node label with stats
	label := fmt.Sprintf("%s (V:%d F:%d)", node.Name, len(node.Vertices), len(node.Faces))

	// Check if node has children
	children := rsm.GetChildNodes(node.Name)
	hasChildren := len(children) > 0

	flags := imgui.TreeNodeFlagsNone
	if !hasChildren {
		flags |= imgui.TreeNodeFlagsLeaf | imgui.TreeNodeFlagsNoTreePushOnOpen
	}

	isOpen := imgui.TreeNodeExStrV(label, flags)

	// Show node details on hover
	if imgui.IsItemHovered() {
		imgui.BeginTooltip()
		imgui.Text(fmt.Sprintf("Name: %s", node.Name))
		if node.Parent != "" {
			imgui.Text(fmt.Sprintf("Parent: %s", node.Parent))
		}
		imgui.Text(fmt.Sprintf("Textures: %d", len(node.TextureIDs)))
		imgui.Text(fmt.Sprintf("Vertices: %d", len(node.Vertices)))
		imgui.Text(fmt.Sprintf("Faces: %d", len(node.Faces)))
		imgui.Text(fmt.Sprintf("Position: (%.2f, %.2f, %.2f)", node.Position[0], node.Position[1], node.Position[2]))
		imgui.Text(fmt.Sprintf("Scale: (%.2f, %.2f, %.2f)", node.Scale[0], node.Scale[1], node.Scale[2]))

		// Animation info
		if len(node.RotKeys) > 0 {
			imgui.Text(fmt.Sprintf("Rot Keyframes: %d", len(node.RotKeys)))
		}
		if len(node.PosKeys) > 0 {
			imgui.Text(fmt.Sprintf("Pos Keyframes: %d", len(node.PosKeys)))
		}
		if len(node.ScaleKeys) > 0 {
			imgui.Text(fmt.Sprintf("Scale Keyframes: %d", len(node.ScaleKeys)))
		}
		imgui.EndTooltip()
	}

	// Render children if node is open
	if isOpen && hasChildren {
		for _, child := range children {
			app.renderRSMNodeTree(rsm, child, depth+1)
		}
		imgui.TreePop()
	}
}
