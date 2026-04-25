// Package ui provides game user interface components.
package ui

import (
	"fmt"
	"runtime"
	"time"

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
	PlayerHasDest             bool
	PlayerDestX, PlayerDestZ  float32
	PlayerIsMoving            bool
	PlayerAction              int

	// Camera info
	CamX, CamY, CamZ float32
	CamDistance      float32
	CamYaw, CamPitch float32

	// Scene / rendering pipeline diagnostics
	SceneFBWidth  int32  // Scene framebuffer width
	SceneFBHeight int32  // Scene framebuffer height
	SceneTexID    uint32 // Color attachment texture ID
	LastGLError   uint32 // 0 = NO_ERROR; non-zero = problem
	TerrainY      float32
	HasGAT        bool

	// Map info
	MapName   string
	MapWidth  int
	MapHeight int

	// Entity counts
	EntityCount  int
	PlayerCount  int
	MonsterCount int
	NPCCount     int
	ItemCount    int

	// Network stats
	PacketsSent     uint64
	PacketsReceived uint64
	BytesSent       uint64
	BytesReceived   uint64
	LastSentID      uint16
	LastSentAgo     time.Duration
	LastSentLen     int
	LastRecvID      uint16
	LastRecvAgo     time.Duration
	LastRecvLen     int

	// Render stats
	DrawCalls       int
	Triangles       int
	TextureSwitches int

	// Display toggles
	ShowFPS         bool
	ShowPosition    bool
	ShowCamera      bool
	ShowScene       bool
	ShowEntityInfo  bool
	ShowNetworkInfo bool
	ShowRenderInfo  bool
	ShowMemory      bool
	Enabled         bool
}

// NewDebugOverlay creates a new debug overlay.
//
// Default: hidden. Toggle with F3 (wired in game.go). Most sections default
// to ON when the overlay opens so the most-useful fields are visible at a
// glance during diagnosis.
func NewDebugOverlay() *DebugOverlay {
	return &DebugOverlay{
		ShowFPS:         true,
		ShowPosition:    true,
		ShowCamera:      true,
		ShowScene:       true,
		ShowEntityInfo:  false,
		ShowNetworkInfo: true,
		ShowRenderInfo:  false,
		ShowMemory:      false,
		Enabled:         false, // Toggle with F3
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
	imgui.SetNextWindowSize(imgui.NewVec2(310, 0)) // Auto height

	flags := imgui.WindowFlagsNoTitleBar | imgui.WindowFlagsNoResize |
		imgui.WindowFlagsNoMove | imgui.WindowFlagsNoScrollbar |
		imgui.WindowFlagsNoSavedSettings | imgui.WindowFlagsNoFocusOnAppearing |
		imgui.WindowFlagsNoInputs

	imgui.PushStyleVarVec2(imgui.StyleVarWindowPadding, imgui.NewVec2(8, 8))
	imgui.SetNextWindowBgAlpha(0.6)

	if imgui.BeginV("##DebugOverlay", nil, flags) {
		imgui.TextDisabled("F3 to toggle")
		if d.ShowFPS {
			d.renderFPS()
		}

		if d.ShowPosition {
			d.renderPosition()
		}

		if d.ShowCamera {
			d.renderCamera()
		}

		if d.ShowScene {
			d.renderScene()
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
	imgui.Text(fmt.Sprintf("Map:  %s", d.MapName))
	imgui.Text(fmt.Sprintf("Pos:  %.1f, %.1f, %.1f", d.PlayerX, d.PlayerY, d.PlayerZ))
	imgui.Text(fmt.Sprintf("Tile: %d, %d   Dir: %d", d.PlayerTileX, d.PlayerTileY, d.PlayerDirection))

	moveState := "idle"
	if d.PlayerIsMoving {
		moveState = "MOVING"
	}
	if d.PlayerHasDest {
		imgui.Text(fmt.Sprintf("State:%s  Dest: %.0f, %.0f", moveState, d.PlayerDestX, d.PlayerDestZ))
	} else {
		imgui.Text(fmt.Sprintf("State:%s", moveState))
	}
}

func (d *DebugOverlay) renderCamera() {
	imgui.Separator()
	imgui.Text("Camera")
	imgui.Text(fmt.Sprintf("  Pos: %.1f, %.1f, %.1f", d.CamX, d.CamY, d.CamZ))
	imgui.Text(fmt.Sprintf("  Dist: %.1f  Yaw: %.2f  Pitch: %.2f", d.CamDistance, d.CamYaw, d.CamPitch))
}

func (d *DebugOverlay) renderScene() {
	imgui.Separator()
	imgui.Text("Scene / GL")
	imgui.Text(fmt.Sprintf("  FB:    %dx%d   Tex: %d", d.SceneFBWidth, d.SceneFBHeight, d.SceneTexID))
	imgui.Text(fmt.Sprintf("  TerrY: %.1f  GAT: %v", d.TerrainY, d.HasGAT))
	if d.LastGLError != 0 {
		// Highlight non-zero GL errors in red — that's the "smoking gun" we
		// missed in our last debug session.
		imgui.TextColored(imgui.NewVec4(1, 0.2, 0.2, 1), fmt.Sprintf("  GL ERR: 0x%04x", d.LastGLError))
	} else {
		imgui.Text("  GL Err: NONE")
	}
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
	imgui.Text(fmt.Sprintf("  Sent: %d pkts (%s)", d.PacketsSent, formatBytes(int64(d.BytesSent))))
	imgui.Text(fmt.Sprintf("  Recv: %d pkts (%s)", d.PacketsReceived, formatBytes(int64(d.BytesReceived))))
	if d.LastSentID != 0 {
		imgui.Text(fmt.Sprintf("  -> 0x%04X (%dB) %s ago", d.LastSentID, d.LastSentLen, formatAgo(d.LastSentAgo)))
	}
	if d.LastRecvID != 0 {
		imgui.Text(fmt.Sprintf("  <- 0x%04X (%dB) %s ago", d.LastRecvID, d.LastRecvLen, formatAgo(d.LastRecvAgo)))
	}
}

func formatAgo(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
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
		imgui.Checkbox("Show Camera", &d.ShowCamera)
		imgui.Checkbox("Show Scene/GL", &d.ShowScene)
		imgui.Checkbox("Show Entity Info", &d.ShowEntityInfo)
		imgui.Checkbox("Show Network Info", &d.ShowNetworkInfo)
		imgui.Checkbox("Show Render Info", &d.ShowRenderInfo)
		imgui.Checkbox("Show Memory", &d.ShowMemory)
	}
}

// Toggle flips Enabled — wired to F3 in game.go.
func (d *DebugOverlay) Toggle() {
	d.Enabled = !d.Enabled
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
