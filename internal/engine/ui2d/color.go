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
	ColorPanelBg      = Color{0.08, 0.08, 0.12, 0.95}
	ColorPanelBorder  = Color{0.3, 0.3, 0.4, 1}
	ColorButtonNormal = Color{0.15, 0.15, 0.2, 1}
	ColorButtonHover  = Color{0.25, 0.25, 0.35, 1}
	ColorButtonActive = Color{0.1, 0.3, 0.5, 1}
	ColorInputBg      = Color{0.05, 0.05, 0.08, 1}
	ColorInputBorder  = Color{0.2, 0.2, 0.3, 1}
	ColorText         = Color{0.9, 0.9, 0.9, 1}
	ColorTextDim      = Color{0.5, 0.5, 0.6, 1}
	ColorHighlight    = Color{0.2, 0.6, 0.9, 1}
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
