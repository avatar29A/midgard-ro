// Package ui provides game user interface components.
package ui

import (
	"github.com/AllenDang/cimgui-go/imgui"

	"github.com/Faultbox/midgard-ro/pkg/formats"
)

// Minimap renders a small map overview with player position.
type Minimap struct {
	// Map data
	gat       *formats.GAT
	mapName   string
	mapWidth  int
	mapHeight int

	// Player position (tile coordinates)
	playerX int
	playerY int

	// Display settings
	Size      float32 // Size of minimap (width = height)
	ShowGrid  bool    // Show grid lines
	ShowZones bool    // Show zone markers (NPCs, warps)

	// Zoom level (1.0 = full map, 2.0 = zoomed in 2x)
	Zoom float32

	// Markers
	markers []MinimapMarker
}

// MinimapMarker represents a point of interest on the minimap.
type MinimapMarker struct {
	X, Y  int           // Tile position
	Type  MarkerType    // Type of marker
	Color imgui.Vec4    // Display color
	Label string        // Optional label
}

// MarkerType defines the type of minimap marker.
type MarkerType uint8

const (
	MarkerTypePlayer MarkerType = iota
	MarkerTypeParty
	MarkerTypeGuild
	MarkerTypeNPC
	MarkerTypeWarp
	MarkerTypeMonster
	MarkerTypeItem
)

// NewMinimap creates a new minimap.
func NewMinimap() *Minimap {
	return &Minimap{
		Size:      150,
		ShowGrid:  false,
		ShowZones: true,
		Zoom:      1.0,
		markers:   make([]MinimapMarker, 0),
	}
}

// SetMapData sets the current map data for the minimap.
func (m *Minimap) SetMapData(gat *formats.GAT, mapName string) {
	m.gat = gat
	m.mapName = mapName
	if gat != nil {
		m.mapWidth = int(gat.Width)
		m.mapHeight = int(gat.Height)
	} else {
		m.mapWidth = 0
		m.mapHeight = 0
	}
}

// SetPlayerPosition updates the player position on the minimap.
func (m *Minimap) SetPlayerPosition(tileX, tileY int) {
	m.playerX = tileX
	m.playerY = tileY
}

// AddMarker adds a marker to the minimap.
func (m *Minimap) AddMarker(marker MinimapMarker) {
	m.markers = append(m.markers, marker)
}

// ClearMarkers removes all markers from the minimap.
func (m *Minimap) ClearMarkers() {
	m.markers = m.markers[:0]
}

// Render renders the minimap at the specified position.
func (m *Minimap) Render(x, y float32) {
	windowSize := m.Size + 20 // Padding

	imgui.SetNextWindowPos(imgui.NewVec2(x, y))
	imgui.SetNextWindowSize(imgui.NewVec2(windowSize, windowSize+25)) // Extra for title

	flags := imgui.WindowFlagsNoResize | imgui.WindowFlagsNoMove |
		imgui.WindowFlagsNoScrollbar | imgui.WindowFlagsNoCollapse

	imgui.PushStyleVarFloat(imgui.StyleVarWindowRounding, 5)
	imgui.SetNextWindowBgAlpha(0.85)

	title := "Minimap"
	if m.mapName != "" {
		title = m.mapName
	}

	if imgui.BeginV(title+"###Minimap", nil, flags) {
		m.renderMap()
	}
	imgui.End()

	imgui.PopStyleVar()
}

func (m *Minimap) renderMap() {
	if m.mapWidth == 0 || m.mapHeight == 0 {
		imgui.Text("No map loaded")
		return
	}

	// Get draw list for custom rendering
	drawList := imgui.WindowDrawList()
	cursorPos := imgui.CursorScreenPos()

	// Calculate scale to fit map in minimap size
	scaleX := m.Size / float32(m.mapWidth) * m.Zoom
	scaleY := m.Size / float32(m.mapHeight) * m.Zoom

	// Use the smaller scale to maintain aspect ratio
	scale := scaleX
	if scaleY < scaleX {
		scale = scaleY
	}

	// Calculate offset to center the map
	mapDisplayWidth := float32(m.mapWidth) * scale
	mapDisplayHeight := float32(m.mapHeight) * scale
	offsetX := (m.Size - mapDisplayWidth) / 2
	offsetY := (m.Size - mapDisplayHeight) / 2

	// Draw background
	bgMin := imgui.NewVec2(cursorPos.X+offsetX, cursorPos.Y+offsetY)
	bgMax := imgui.NewVec2(cursorPos.X+offsetX+mapDisplayWidth, cursorPos.Y+offsetY+mapDisplayHeight)
	drawList.AddRectFilledV(bgMin, bgMax, imgui.ColorU32Vec4(imgui.NewVec4(0.1, 0.15, 0.2, 1.0)), 0, 0)

	// Draw grid if enabled
	if m.ShowGrid {
		m.renderGrid(drawList, cursorPos, offsetX, offsetY, scale)
	}

	// Draw markers
	for _, marker := range m.markers {
		m.renderMarker(drawList, cursorPos, offsetX, offsetY, scale, marker)
	}

	// Draw player position
	m.renderPlayer(drawList, cursorPos, offsetX, offsetY, scale)

	// Draw border
	drawList.AddRectV(bgMin, bgMax, imgui.ColorU32Vec4(imgui.NewVec4(0.5, 0.5, 0.5, 1.0)), 0, 0, 1)

	// Reserve space for the minimap
	imgui.Dummy(imgui.NewVec2(m.Size, m.Size))
}

func (m *Minimap) renderGrid(drawList *imgui.DrawList, cursorPos imgui.Vec2, offsetX, offsetY, scale float32) {
	gridColor := imgui.ColorU32Vec4(imgui.NewVec4(0.3, 0.3, 0.3, 0.5))
	gridSpacing := 10 // Draw a line every 10 tiles

	// Vertical lines
	for x := 0; x <= m.mapWidth; x += gridSpacing {
		px := cursorPos.X + offsetX + float32(x)*scale
		drawList.AddLineV(
			imgui.NewVec2(px, cursorPos.Y+offsetY),
			imgui.NewVec2(px, cursorPos.Y+offsetY+float32(m.mapHeight)*scale),
			gridColor,
			1,
		)
	}

	// Horizontal lines
	for y := 0; y <= m.mapHeight; y += gridSpacing {
		py := cursorPos.Y + offsetY + float32(y)*scale
		drawList.AddLineV(
			imgui.NewVec2(cursorPos.X+offsetX, py),
			imgui.NewVec2(cursorPos.X+offsetX+float32(m.mapWidth)*scale, py),
			gridColor,
			1,
		)
	}
}

func (m *Minimap) renderMarker(drawList *imgui.DrawList, cursorPos imgui.Vec2, offsetX, offsetY, scale float32, marker MinimapMarker) {
	px := cursorPos.X + offsetX + float32(marker.X)*scale
	py := cursorPos.Y + offsetY + float32(m.mapHeight-marker.Y)*scale // Flip Y

	markerSize := float32(3)
	color := imgui.ColorU32Vec4(marker.Color)

	switch marker.Type {
	case MarkerTypeParty:
		// Circle for party members
		drawList.AddCircleFilledV(imgui.NewVec2(px, py), markerSize, color, 8)
	case MarkerTypeGuild:
		// Square for guild members
		drawList.AddRectFilledV(
			imgui.NewVec2(px-markerSize, py-markerSize),
			imgui.NewVec2(px+markerSize, py+markerSize),
			color, 0, 0,
		)
	case MarkerTypeNPC:
		// Diamond for NPCs
		m.drawDiamond(drawList, px, py, markerSize, color)
	case MarkerTypeWarp:
		// Triangle for warps
		m.drawTriangle(drawList, px, py, markerSize, color)
	case MarkerTypeMonster:
		// Small red dot for monsters
		drawList.AddCircleFilledV(imgui.NewVec2(px, py), 2, color, 6)
	case MarkerTypeItem:
		// Small blue dot for items
		drawList.AddCircleFilledV(imgui.NewVec2(px, py), 2, color, 6)
	default:
		// Default: small circle
		drawList.AddCircleFilledV(imgui.NewVec2(px, py), markerSize, color, 8)
	}
}

func (m *Minimap) drawDiamond(drawList *imgui.DrawList, x, y, size float32, color uint32) {
	drawList.AddQuadFilled(
		imgui.NewVec2(x, y-size),     // Top
		imgui.NewVec2(x+size, y),     // Right
		imgui.NewVec2(x, y+size),     // Bottom
		imgui.NewVec2(x-size, y),     // Left
		color,
	)
}

func (m *Minimap) drawTriangle(drawList *imgui.DrawList, x, y, size float32, color uint32) {
	drawList.AddTriangleFilled(
		imgui.NewVec2(x, y-size),       // Top
		imgui.NewVec2(x+size, y+size),  // Bottom right
		imgui.NewVec2(x-size, y+size),  // Bottom left
		color,
	)
}

func (m *Minimap) renderPlayer(drawList *imgui.DrawList, cursorPos imgui.Vec2, offsetX, offsetY, scale float32) {
	px := cursorPos.X + offsetX + float32(m.playerX)*scale
	py := cursorPos.Y + offsetY + float32(m.mapHeight-m.playerY)*scale // Flip Y

	// Player marker: white circle with green fill
	playerSize := float32(4)
	drawList.AddCircleFilledV(imgui.NewVec2(px, py), playerSize, imgui.ColorU32Vec4(imgui.NewVec4(0.2, 1.0, 0.2, 1.0)), 12)
	drawList.AddCircleV(imgui.NewVec2(px, py), playerSize, imgui.ColorU32Vec4(imgui.NewVec4(1.0, 1.0, 1.0, 1.0)), 12, 1)
}

// HandleInput handles mouse input for the minimap (zoom, click-to-move).
func (m *Minimap) HandleInput() (clicked bool, tileX, tileY int) {
	// Check if mouse is over the minimap window
	if !imgui.IsWindowHovered() {
		return false, 0, 0
	}

	// Zoom with scroll wheel
	scroll := imgui.CurrentIO().MouseWheel()
	if scroll != 0 {
		m.Zoom += scroll * 0.1
		if m.Zoom < 0.5 {
			m.Zoom = 0.5
		}
		if m.Zoom > 4.0 {
			m.Zoom = 4.0
		}
	}

	// Click to move (left mouse button)
	if imgui.IsMouseClickedBool(0) {
		mousePos := imgui.MousePos()
		windowPos := imgui.WindowPos()

		// Calculate click position relative to minimap
		relX := mousePos.X - windowPos.X - 10 // Account for padding
		relY := mousePos.Y - windowPos.Y - 30 // Account for title bar

		// Convert to tile coordinates
		scale := m.Size / float32(m.mapWidth) * m.Zoom
		if scale > m.Size/float32(m.mapHeight)*m.Zoom {
			scale = m.Size / float32(m.mapHeight) * m.Zoom
		}

		mapDisplayWidth := float32(m.mapWidth) * scale
		mapDisplayHeight := float32(m.mapHeight) * scale
		offsetX := (m.Size - mapDisplayWidth) / 2
		offsetY := (m.Size - mapDisplayHeight) / 2

		// Check if click is within map bounds
		if relX >= offsetX && relX <= offsetX+mapDisplayWidth &&
			relY >= offsetY && relY <= offsetY+mapDisplayHeight {

			tileX = int((relX - offsetX) / scale)
			tileY = m.mapHeight - int((relY-offsetY)/scale) // Flip Y

			// Clamp to map bounds
			if tileX < 0 {
				tileX = 0
			}
			if tileX >= m.mapWidth {
				tileX = m.mapWidth - 1
			}
			if tileY < 0 {
				tileY = 0
			}
			if tileY >= m.mapHeight {
				tileY = m.mapHeight - 1
			}

			return true, tileX, tileY
		}
	}

	return false, 0, 0
}
