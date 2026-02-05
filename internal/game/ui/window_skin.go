package ui

import (
	"fmt"

	"github.com/Faultbox/midgard-ro/internal/engine/ui2d"
)

// WindowSkin holds textures for window frame rendering.
type WindowSkin struct {
	Frame *ui2d.NineSlice
}

// RO UI texture base path (Korean folder name for "user interface")
const skinBasePath = `data\texture\유저인터페이스\basic_interface\`

// LoadWindowSkin loads the RO window frame skin from GRF textures.
// Returns an error if the required textures are not available.
func LoadWindowSkin(tc *TextureCache) (*WindowSkin, error) {
	// RO uses a single window frame texture that works as a 9-slice
	framePath := skinBasePath + `win_msgbox.bmp`
	info, err := tc.Load(framePath)
	if err != nil {
		return nil, fmt.Errorf("loading window frame skin: %w", err)
	}

	// Standard RO window frame border insets (pixels from edges)
	frame := &ui2d.NineSlice{
		TextureID: info.ID,
		TexWidth:  info.Width,
		TexHeight: info.Height,
		Left:      8,
		Right:     8,
		Top:       8,
		Bottom:    8,
	}

	return &WindowSkin{Frame: frame}, nil
}
