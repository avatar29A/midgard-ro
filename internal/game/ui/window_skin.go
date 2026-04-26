package ui

import (
	"fmt"

	"github.com/Faultbox/midgard-ro/internal/engine/ui2d"
)

// WindowSkin holds textures for window frame rendering.
type WindowSkin struct {
	Frame *ui2d.NineSlice
}

// RO UI texture base path (Korean folder name for "user interface").
// Note: there is no `basic_interface\` subfolder for win_msgbox — the file
// sits directly under 유저인터페이스. Other UI assets (login_interface, etc.)
// do live in subfolders. EUC-KR encoding is handled by assets.Manager.Load.
const skinBasePath = `data\texture\유저인터페이스\`

// LoadWindowSkin loads the RO window frame skin from GRF textures.
// Returns an error if the required textures are not available.
func LoadWindowSkin(tc *TextureCache) (*WindowSkin, error) {
	// RO uses a single 280×120 BMP that 9-slices into title bar (top),
	// body, and footer bar (bottom). Insets measured visually:
	// top includes the title bar with close icon; bottom is the thin footer.
	framePath := skinBasePath + `win_msgbox.bmp`
	info, err := tc.Load(framePath)
	if err != nil {
		return nil, fmt.Errorf("loading window frame skin: %w", err)
	}

	// Sample the rightmost 6px column of the title bar as the "clean strip"
	// overlay — that region of win_msgbox.bmp is empty gradient (no text or
	// icons), so stretching it across the whole title bar gives us a blank
	// canvas to render our own title text on.
	frame := &ui2d.NineSlice{
		TextureID:      info.ID,
		TexWidth:       info.Width,
		TexHeight:      info.Height,
		Left:           6,
		Right:          6,
		Top:            24,
		Bottom:         12,
		TitleStripSrcX: info.Width - 6,
		TitleStripSrcW: 6,
	}

	return &WindowSkin{Frame: frame}, nil
}
