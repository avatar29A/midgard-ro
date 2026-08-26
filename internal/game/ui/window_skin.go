package ui

import (
	"fmt"

	"github.com/Faultbox/midgard-ro/internal/engine/ui2d"
)

// WindowSkin holds textures for window frame rendering.
type WindowSkin struct {
	Frame *ui2d.NineSlice
}

// LoadInputSkin loads the RO `name-edit.bmp` (101×20) as a 9-slice for text
// input fields. The BMP has a thin border + recessed body with a subtle
// vertical gradient — exactly the look our procedural sunken bevel was
// faking. Insets chosen by eye: 3px on each edge captures the border + first
// gradient pixel, leaving the smooth body region as the stretchable center.
func LoadInputSkin(tc *TextureCache) (*ui2d.NineSlice, error) {
	path := skinBasePath + `login_interface\name-edit.bmp`
	info, err := tc.Load(path)
	if err != nil {
		return nil, fmt.Errorf("loading input skin: %w", err)
	}
	return &ui2d.NineSlice{
		TextureID: info.ID,
		TexWidth:  info.Width,
		TexHeight: info.Height,
		Left:      3,
		Right:     3,
		Top:       3,
		Bottom:    3,
	}, nil
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
		TextureID: info.ID,
		TexWidth:  info.Width,
		TexHeight: info.Height,
		Left:      6,
		Right:     6,
		Top:       24,
		Bottom:    12,
		// Sample a single column, not a band. The strip is stretched across the
		// whole title bar, so any horizontal variation inside the sampled
		// region gets magnified from a few pixels to a few hundred — which is
		// what split the bar into two visibly different shades.
		TitleStripSrcX: info.Width - 2,
		TitleStripSrcW: 1,
	}

	return &WindowSkin{Frame: frame}, nil
}

// basicInterfacePath holds the chrome the original assembles its windows from.
const basicInterfacePath = skinBasePath + `basic_interface\`

// LoadNativeWindowFrame loads the original window chrome. A missing texture
// returns nil, which leaves windows on the themed skin rather than half-drawn.
func LoadNativeWindowFrame(tc *TextureCache) (*ui2d.WindowFrame, error) {
	ids := make(map[string]uint32)

	for _, name := range []string{
		"titlebar_left.bmp", "titlebar_mid.bmp", "titlebar_right.bmp",
		"sys_mini_off.bmp", "sys_mini_on.bmp",
		"sys_close_off.bmp", "sys_close_on.bmp",
		"btnbar_mid2.bmp",
	} {
		info, err := tc.Load(basicInterfacePath + name)
		if err != nil {
			return nil, fmt.Errorf("loading window chrome %s: %w", name, err)
		}

		ids[name] = info.ID
	}

	// The grip sits beside the interface folder rather than inside
	// basic_interface, unlike everything else here.
	grip, err := tc.Load(skinBasePath + `btn_resize.bmp`)
	if err != nil {
		return nil, fmt.Errorf("loading resize grip: %w", err)
	}

	return &ui2d.WindowFrame{
		TitleLeft:   ids["titlebar_left.bmp"],
		TitleMid:    ids["titlebar_mid.bmp"],
		TitleRight:  ids["titlebar_right.bmp"],
		SysMiniOff:  ids["sys_mini_off.bmp"],
		SysMiniOn:   ids["sys_mini_on.bmp"],
		SysCloseOff: ids["sys_close_off.bmp"],
		SysCloseOn:  ids["sys_close_on.bmp"],
		FooterMid:   ids["btnbar_mid2.bmp"],
		Grip:        grip.ID,
	}, nil
}
