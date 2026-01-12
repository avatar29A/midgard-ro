// 3D model preview (RSM) for GRF Browser (ADR-012 Stage 2).
package main

import (
	"fmt"
	"os"

	"github.com/AllenDang/cimgui-go/imgui"

	"github.com/Faultbox/midgard-ro/pkg/formats"
)

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
}

// renderRSMPreview renders the RSM model info panel.
func (app *App) renderRSMPreview() {
	if app.previewRSM == nil {
		imgui.TextDisabled("Failed to load RSM file")
		return
	}

	rsm := app.previewRSM

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
		if imgui.TreeNodeExStrV(fmt.Sprintf("Textures (%d)", len(rsm.Textures)), imgui.TreeNodeFlagsDefaultOpen) {
			for i, tex := range rsm.Textures {
				imgui.Text(fmt.Sprintf("%d: %s", i, tex))
			}
			imgui.TreePop()
		}
	}

	imgui.Separator()

	// Node hierarchy
	if len(rsm.Nodes) > 0 {
		if imgui.TreeNodeExStrV(fmt.Sprintf("Node Hierarchy (%d)", len(rsm.Nodes)), imgui.TreeNodeFlagsDefaultOpen) {
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
		imgui.Separator()
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
