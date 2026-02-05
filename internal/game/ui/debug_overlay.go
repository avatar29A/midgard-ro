// Package ui provides game user interface components.
package ui

import (
	"fmt"
	"runtime"

	"github.com/AllenDang/cimgui-go/imgui"
)

// DebugOverlay renders debug information on screen.
type DebugOverlay struct {
	// Frame timing
	frameCount    int
	fps           float64
	frameTime     float64 // ms
	fpsUpdateTime float64 // seconds since last FPS update
	frameAccum    int

	// Memory stats
	memStats      runtime.MemStats
	memUpdateTime float64

	// Player info
	PlayerX, PlayerY, PlayerZ float32
	PlayerTileX, PlayerTileY  int
	PlayerDirection           uint8

	// Map info
	MapName   string
	MapWidth  int
	MapHeight int

	// Entity counts
	EntityCount   int
	PlayerCount   int
	MonsterCount  int
	NPCCount      int
	ItemCount     int

	// Network stats
	PacketsSent     int
	PacketsReceived int
	BytesSent       int64
	BytesReceived   int64
	Ping            int

	// Render stats
	DrawCalls    int
	Triangles    int
	TextureSwitches int

	// Display toggles
	ShowFPS       bool
	ShowPosition  bool
	ShowEntityInfo bool
	ShowNetworkInfo bool
	ShowRenderInfo bool
	ShowMemory    bool
	Enabled       bool
}

// NewDebugOverlay creates a new debug overlay.
func NewDebugOverlay() *DebugOverlay {
	return &DebugOverlay{
		ShowFPS:       true,
		ShowPosition:  true,
		ShowEntityInfo: false,
		ShowNetworkInfo: false,
		ShowRenderInfo: false,
		ShowMemory:    false,
		Enabled:       true,
	}
}

// Update updates the debug overlay state.
// deltaMs is the frame time in milliseconds.
func (d *DebugOverlay) Update(deltaMs float64) {
	d.frameCount++
	d.frameTime = deltaMs
	d.frameAccum++
	d.fpsUpdateTime += deltaMs / 1000.0

	// Update FPS every 0.5 seconds
	if d.fpsUpdateTime >= 0.5 {
		d.fps = float64(d.frameAccum) / d.fpsUpdateTime
		d.frameAccum = 0
		d.fpsUpdateTime = 0
	}

	// Update memory stats every 2 seconds
	d.memUpdateTime += deltaMs / 1000.0
	if d.memUpdateTime >= 2.0 {
		runtime.ReadMemStats(&d.memStats)
		d.memUpdateTime = 0
	}
}

// Render renders the debug overlay.
func (d *DebugOverlay) Render() {
	if !d.Enabled {
		return
	}

	// Position at top-left corner
	imgui.SetNextWindowPos(imgui.NewVec2(10, 10))
	imgui.SetNextWindowSize(imgui.NewVec2(250, 0)) // Auto height

	flags := imgui.WindowFlagsNoTitleBar | imgui.WindowFlagsNoResize |
		imgui.WindowFlagsNoMove | imgui.WindowFlagsNoScrollbar |
		imgui.WindowFlagsNoSavedSettings | imgui.WindowFlagsNoFocusOnAppearing |
		imgui.WindowFlagsNoInputs

	imgui.PushStyleVarVec2(imgui.StyleVarWindowPadding, imgui.NewVec2(8, 8))
	imgui.SetNextWindowBgAlpha(0.6)

	if imgui.BeginV("##DebugOverlay", nil, flags) {
		if d.ShowFPS {
			d.renderFPS()
		}

		if d.ShowPosition {
			d.renderPosition()
		}

		if d.ShowEntityInfo {
			d.renderEntityInfo()
		}

		if d.ShowNetworkInfo {
			d.renderNetworkInfo()
		}

		if d.ShowRenderInfo {
			d.renderRenderInfo()
		}

		if d.ShowMemory {
			d.renderMemory()
		}
	}
	imgui.End()

	imgui.PopStyleVar()
}

func (d *DebugOverlay) renderFPS() {
	// FPS with color coding
	fpsColor := imgui.NewVec4(0.2, 1.0, 0.2, 1.0) // Green
	if d.fps < 30 {
		fpsColor = imgui.NewVec4(1.0, 0.2, 0.2, 1.0) // Red
	} else if d.fps < 60 {
		fpsColor = imgui.NewVec4(1.0, 1.0, 0.2, 1.0) // Yellow
	}

	imgui.TextColored(fpsColor, fmt.Sprintf("FPS: %.1f", d.fps))
	imgui.SameLine()
	imgui.TextDisabled(fmt.Sprintf("(%.2f ms)", d.frameTime))
}

func (d *DebugOverlay) renderPosition() {
	imgui.Separator()
	imgui.Text(fmt.Sprintf("Map: %s", d.MapName))
	imgui.Text(fmt.Sprintf("Pos: %.1f, %.1f, %.1f", d.PlayerX, d.PlayerY, d.PlayerZ))
	imgui.Text(fmt.Sprintf("Tile: %d, %d", d.PlayerTileX, d.PlayerTileY))
	imgui.Text(fmt.Sprintf("Dir: %d", d.PlayerDirection))
}

func (d *DebugOverlay) renderEntityInfo() {
	imgui.Separator()
	imgui.Text("Entities")
	imgui.Text(fmt.Sprintf("  Total: %d", d.EntityCount))
	imgui.Text(fmt.Sprintf("  Players: %d", d.PlayerCount))
	imgui.Text(fmt.Sprintf("  Monsters: %d", d.MonsterCount))
	imgui.Text(fmt.Sprintf("  NPCs: %d", d.NPCCount))
	imgui.Text(fmt.Sprintf("  Items: %d", d.ItemCount))
}

func (d *DebugOverlay) renderNetworkInfo() {
	imgui.Separator()
	imgui.Text("Network")
	imgui.Text(fmt.Sprintf("  Ping: %d ms", d.Ping))
	imgui.Text(fmt.Sprintf("  Sent: %d pkts (%s)", d.PacketsSent, formatBytes(d.BytesSent)))
	imgui.Text(fmt.Sprintf("  Recv: %d pkts (%s)", d.PacketsReceived, formatBytes(d.BytesReceived)))
}

func (d *DebugOverlay) renderRenderInfo() {
	imgui.Separator()
	imgui.Text("Render")
	imgui.Text(fmt.Sprintf("  Draw Calls: %d", d.DrawCalls))
	imgui.Text(fmt.Sprintf("  Triangles: %d", d.Triangles))
	imgui.Text(fmt.Sprintf("  Tex Switches: %d", d.TextureSwitches))
}

func (d *DebugOverlay) renderMemory() {
	imgui.Separator()
	imgui.Text("Memory")
	imgui.Text(fmt.Sprintf("  Alloc: %s", formatBytes(int64(d.memStats.Alloc))))
	imgui.Text(fmt.Sprintf("  Total: %s", formatBytes(int64(d.memStats.TotalAlloc))))
	imgui.Text(fmt.Sprintf("  Sys: %s", formatBytes(int64(d.memStats.Sys))))
	imgui.Text(fmt.Sprintf("  GC: %d", d.memStats.NumGC))
}

// RenderSettings renders a settings panel for the debug overlay.
func (d *DebugOverlay) RenderSettings() {
	if imgui.CollapsingHeaderTreeNodeFlagsV("Debug Overlay", imgui.TreeNodeFlagsDefaultOpen) {
		imgui.Checkbox("Enabled", &d.Enabled)
		imgui.Checkbox("Show FPS", &d.ShowFPS)
		imgui.Checkbox("Show Position", &d.ShowPosition)
		imgui.Checkbox("Show Entity Info", &d.ShowEntityInfo)
		imgui.Checkbox("Show Network Info", &d.ShowNetworkInfo)
		imgui.Checkbox("Show Render Info", &d.ShowRenderInfo)
		imgui.Checkbox("Show Memory", &d.ShowMemory)
	}
}

// formatBytes formats byte count to human readable string.
func formatBytes(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/GB)
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/MB)
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/KB)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
