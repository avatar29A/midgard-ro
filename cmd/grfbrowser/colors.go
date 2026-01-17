// Color constants for GRF Browser application.
package main

import "image/color"

// GAT cell type colors for visualization
var (
	GATColorWalkable  = color.RGBA{R: 100, G: 200, B: 100, A: 255} // Green - walkable
	GATColorBlocked   = color.RGBA{R: 200, G: 80, B: 80, A: 255}   // Red - blocked
	GATColorWater     = color.RGBA{R: 80, G: 150, B: 220, A: 255}  // Blue - water (not walkable)
	GATColorWaterWalk = color.RGBA{R: 100, G: 180, B: 180, A: 255} // Cyan - water (walkable)
	GATColorCliff     = color.RGBA{R: 180, G: 140, B: 100, A: 255} // Brown - cliff/snipable
	GATColorUnknown   = color.RGBA{R: 128, G: 128, B: 128, A: 255} // Gray - unknown type
)

// UI background colors
var (
	BackgroundColor   = [4]float32{0.1, 0.1, 0.12, 1.0}
	PreviewBackground = [4]float32{0.15, 0.15, 0.15, 1.0}
	DebugTextColor    = color.RGBA{R: 255, G: 255, B: 255, A: 255}
)

// Model viewer colors
var (
	AxisColorX = [4]float32{1.0, 0.0, 0.0, 1.0} // Red for X axis
	AxisColorY = [4]float32{0.0, 1.0, 0.0, 1.0} // Green for Y axis
	AxisColorZ = [4]float32{0.0, 0.0, 1.0, 1.0} // Blue for Z axis
)

// Default lighting colors
var (
	DefaultAmbientColor = [3]float32{0.55, 0.50, 0.50}
	DefaultDiffuseColor = [3]float32{1.00, 1.00, 1.00}
)
