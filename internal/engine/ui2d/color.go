package ui2d

// Color represents an RGBA color with float components (0.0 to 1.0).
type Color struct {
	R, G, B, A float32
}

// Predefined colors for UI theming.
var (
	// Transparent
	ColorTransparent = Color{0, 0, 0, 0}

	// Basic colors
	ColorWhite = Color{1, 1, 1, 1}
	ColorBlack = Color{0, 0, 0, 1}
	ColorRed   = Color{1, 0, 0, 1}
	ColorGreen = Color{0, 1, 0, 1}
	ColorBlue  = Color{0, 0, 1, 1}

	// UI theme colors (RO-inspired)
	ColorPanelBg     = Color{0.08, 0.08, 0.12, 0.75}
	ColorPanelBorder = Color{0.3, 0.3, 0.4, 1}
	// Buttons: faint-gray fill that's distinguishable from the win_msgbox
	// body (which is pure white, RGB 255). Hover tints the fill subtly blue
	// to echo the RO title bar accent (rgb 53,93,204). Pressed deepens the
	// blue. Border is mid-gray so it reads on the white surface. Reference:
	// browser-default button look in the mannoeu/ragnarok-login recreation.
	// Buttons read as raised 3D widgets on the pure-white BMP body via a
	// classic Win95-style bevel. Fill is light gray (not near-white) so the
	// white highlight registers against it; highlight on top/left, dark
	// shadow on bottom/right. Border is the fallback outline if a caller
	// renders without bevels. Hover/active tint blue (RO accent rgb 53,93,204).
	ColorButtonNormal  = Color{0.84, 0.84, 0.86, 1}
	ColorButtonHover   = Color{0.78, 0.84, 0.95, 1}
	ColorButtonActive  = Color{0.55, 0.70, 0.90, 1}
	ColorButtonBorder  = Color{0.40, 0.40, 0.45, 1}
	ColorButtonBevelHi = Color{1.00, 1.00, 1.00, 1}
	ColorButtonBevelLo = Color{0.30, 0.30, 0.35, 1}
	ColorInputBg       = Color{0.05, 0.05, 0.08, 1}
	ColorInputBorder   = Color{0.2, 0.2, 0.3, 1}
	// ColorText is the default text color, tuned for legibility on the cream
	// win_msgbox.bmp body (which is the dominant text surface).
	ColorText       = Color{0.1, 0.1, 0.15, 1}
	ColorTextOnDark = Color{0.9, 0.9, 0.9, 1}
	ColorTextDim    = Color{0.4, 0.4, 0.5, 1}
	ColorHighlight  = Color{0.2, 0.6, 0.9, 1}
)

// RGBA creates a color from 8-bit RGBA values (0-255).
func RGBA(r, g, b, a uint8) Color {
	return Color{
		R: float32(r) / 255.0,
		G: float32(g) / 255.0,
		B: float32(b) / 255.0,
		A: float32(a) / 255.0,
	}
}

// RGB creates a color from 8-bit RGB values with full alpha.
func RGB(r, g, b uint8) Color {
	return Color{
		R: float32(r) / 255.0,
		G: float32(g) / 255.0,
		B: float32(b) / 255.0,
		A: 1.0,
	}
}

// WithAlpha returns a copy of the color with a different alpha value.
func (c Color) WithAlpha(a float32) Color {
	return Color{c.R, c.G, c.B, a}
}

// Darken returns a darker version of the color.
func (c Color) Darken(factor float32) Color {
	return Color{
		R: c.R * (1 - factor),
		G: c.G * (1 - factor),
		B: c.B * (1 - factor),
		A: c.A,
	}
}

// Lighten returns a lighter version of the color.
func (c Color) Lighten(factor float32) Color {
	return Color{
		R: c.R + (1-c.R)*factor,
		G: c.G + (1-c.G)*factor,
		B: c.B + (1-c.B)*factor,
		A: c.A,
	}
}
