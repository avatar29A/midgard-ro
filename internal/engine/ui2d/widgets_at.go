package ui2d

// Absolute-positioning widget API.
//
// Classic RO dialogs are hand-laid out: each window is a fixed-size BMP with
// widgets at known pixel coordinates. The `*At` family takes screen-absolute
// (x, y, w, h) and is independent of any cursor/Row/Spacer flow — call them
// from anywhere, with or without a current window.

// ButtonAt draws a procedural button at absolute coords. Returns true on
// click. Used as a fallback when no ImageButton textures are supplied.
func (c *Context) ButtonAt(id string, x, y, w, h float32, label string) bool {
	rect := Rect{x, y, w, h}
	hovered := rect.Contains(c.input.MouseX, c.input.MouseY)
	clicked := false

	if hovered {
		c.hotWidget = id
		if c.input.MouseLeftPressed || c.input.MouseLeftClicked {
			c.activeWidget = id
			clicked = true
			c.input.MouseLeftClicked = false
		}
	}
	if c.activeWidget == id && c.input.MouseLeftReleased {
		c.activeWidget = ""
	}

	pressed := c.activeWidget == id

	// Borderless glass button (radius 6). The silhouette is defined by
	// the body gradient + corner knockouts only — no traced outline,
	// since the staircase outline always reads as pixelated. Volume
	// comes from a stronger drop shadow + a stronger body gradient
	// (light at top, mid at bottom) plus a thin inner highlight strip.
	knock := [...]float32{6, 3, 2, 1, 1, 1}
	const r = 6

	// 1. Drop shadow — slightly stronger than before to compensate for
	// the missing border. Three fading rows below the button silhouette.
	if !pressed {
		c.renderer.DrawRect(x+r, y+h, w-2*r, 1, Color{R: 0, G: 0, B: 0, A: 0.42})
		c.renderer.DrawRect(x+r+1, y+h+1, w-2*r-2, 1, Color{R: 0, G: 0, B: 0, A: 0.26})
		c.renderer.DrawRect(x+r+2, y+h+2, w-2*r-4, 1, Color{R: 0, G: 0, B: 0, A: 0.12})
	}

	// 2. Glass body gradient over the full rect. Without a border the
	// gradient does all the visual work — bumped opacity so the
	// silhouette reads cleanly against the panel.
	bodyTop := Color{R: 1, G: 1, B: 1, A: 0.75}
	bodyBot := Color{R: 0.85, G: 0.88, B: 0.92, A: 0.55}
	if hovered && !pressed {
		bodyTop = Color{R: 0.82, G: 0.90, B: 1.0, A: 0.85}
		bodyBot = Color{R: 0.55, G: 0.72, B: 0.95, A: 0.65}
	}
	if pressed {
		bodyTop = Color{R: 0.20, G: 0.30, B: 0.50, A: 0.35}
		bodyBot = Color{R: 0.40, G: 0.55, B: 0.80, A: 0.50}
	}
	c.renderer.DrawRectGradient(x, y, w, h, bodyTop, bodyBot)

	// 3. Corner knockouts with the panel bg color. Same staircase as
	// before — defines the rounded silhouette.
	bg := ColorInputBg
	knockCorner := func(cx, cy float32, dx, dy float32) {
		for row, width := range knock {
			startX := cx
			if dx < 0 {
				startX = cx - width + 1
			}
			c.renderer.DrawRect(startX, cy+float32(row)*dy, width, 1, bg)
		}
	}
	knockCorner(x, y, 1, 1)
	knockCorner(x+w-1, y, -1, 1)
	knockCorner(x, y+h-1, 1, -1)
	knockCorner(x+w-1, y+h-1, -1, -1)

	// 4. Inner highlights — top edge shine + bottom edge dark stripe.
	// These are along the straight portions only (the corner cells are
	// already curved by the knockouts). Without a border, the bottom
	// stripe acts as the "lower edge" cue that sells the 3D look.
	if !pressed {
		c.renderer.DrawRect(x+r, y, w-2*r, 1, Color{R: 1, G: 1, B: 1, A: 0.85})    // top shine
		c.renderer.DrawRect(x+r, y+h-1, w-2*r, 1, Color{R: 0, G: 0, B: 0, A: 0.32}) // bottom shadow
	} else {
		// Pressed: invert — slight dark line at top, light at bottom.
		c.renderer.DrawRect(x+r, y, w-2*r, 1, Color{R: 0, G: 0, B: 0, A: 0.30})
	}

	// Label centered using line-box height (factors descenders for
	// mixed-case labels). Pressed nudges down 1px to follow the surface.
	scale := float32(0.9)
	textW, textH := c.renderer.MeasureText(label, scale)
	textY := y + (h-textH)/2
	if pressed {
		textY++
	}
	c.renderer.DrawText(x+(w-textW)/2, textY, label, scale, ColorText)
	return clicked
}

// ImageButtonAt is the RO-style 3-state textured button. Pass texture IDs for
// the normal / hover / pressed states; zero-IDs fall back to the normal
// texture. Returns true on click. The texture is drawn 1:1 over the
// (x, y, w, h) rect with white tint.
func (c *Context) ImageButtonAt(id string, x, y, w, h float32, normalTex, overTex, pressedTex uint32) bool {
	rect := Rect{x, y, w, h}
	hovered := rect.Contains(c.input.MouseX, c.input.MouseY)
	clicked := false

	if hovered {
		c.hotWidget = id
		if c.input.MouseLeftPressed || c.input.MouseLeftClicked {
			c.activeWidget = id
			clicked = true
			c.input.MouseLeftClicked = false
		}
	}
	if c.activeWidget == id && c.input.MouseLeftReleased {
		c.activeWidget = ""
	}

	tex := normalTex
	switch {
	case c.activeWidget == id && pressedTex != 0:
		tex = pressedTex
	case hovered && overTex != 0:
		tex = overTex
	}
	if tex != 0 {
		c.renderer.DrawImage(tex, x, y, w, h, ColorWhite)
	}
	return clicked
}

// TextInputAt draws an editable text field at absolute coords.
// Returns (current value, changed, submitted-on-enter).
func (c *Context) TextInputAt(id string, x, y, w, h float32, value string) (string, bool, bool) {
	return c.textFieldAt(id, x, y, w, h, value, false)
}

// PasswordInputAt draws a masked text field at absolute coords.
func (c *Context) PasswordInputAt(id string, x, y, w, h float32, value string) (string, bool, bool) {
	return c.textFieldAt(id, x, y, w, h, value, true)
}

// textFieldAt is the shared implementation for TextInputAt / PasswordInputAt.
func (c *Context) textFieldAt(id string, x, y, w, h float32, value string, masked bool) (string, bool, bool) {
	rect := Rect{x, y, w, h}
	hovered := rect.Contains(c.input.MouseX, c.input.MouseY)
	focused := c.activeWidget == id
	changed := false
	submitted := false

	if hovered && c.input.MouseLeftPressed {
		c.activeWidget = id
		focused = true
	} else if !hovered && c.input.MouseLeftPressed {
		// Clicking outside an active text field releases focus.
		if focused {
			c.activeWidget = ""
			focused = false
		}
	}

	if focused {
		if len(c.input.TextInput) > 0 {
			value += c.input.TextInput
			changed = true
		}
		if c.input.KeyBackspacePressed && len(value) > 0 {
			value = value[:len(value)-1]
			changed = true
		}
		if c.input.KeyEnterPressed {
			submitted = true
		}
		if c.input.KeyEscapePressed {
			c.activeWidget = ""
		}
	}

	c.drawInput(x, y, w, h, focused)

	displayed := value
	if masked {
		displayed = ""
		for range value {
			displayed += "*"
		}
	}

	// Body text scaled down so glyphs sit comfortably inside the field —
	// at scale=1.0 cap-height was visually too tall for a 22-28px input.
	// Center on cap-height (ascent) for optical centering rather than the
	// line-height that includes leading + descender padding.
	scale := float32(0.85)
	ascent := c.renderer.FontAscent(scale)
	textY := y + (h-ascent)/2
	c.renderer.DrawText(x+4, textY, displayed, scale, ColorText)

	if focused {
		textW, _ := c.renderer.MeasureText(displayed, scale)
		cursorX := x + 4 + textW
		c.renderer.DrawRect(cursorX, y+4, 2, h-8, ColorText)
	}

	return value, changed, submitted
}

// SelectableAt draws a list-row entry at absolute coords. Returns true on
// click. `selected` controls the persistent highlight; the caller owns
// selection state and decides which row claims the highlight on click.
// Hover is shown via a lighter background; selected rows use the accent
// highlight color.
func (c *Context) SelectableAt(id string, x, y, w, h float32, label string, selected bool) bool {
	rect := Rect{x, y, w, h}
	hovered := rect.Contains(c.input.MouseX, c.input.MouseY)
	clicked := false

	if hovered {
		c.hotWidget = id
		if c.input.MouseLeftPressed {
			c.activeWidget = id
			clicked = true
		}
	}
	if c.activeWidget == id && c.input.MouseLeftReleased {
		c.activeWidget = ""
	}

	var bgColor Color
	switch {
	case selected:
		bgColor = ColorHighlight.WithAlpha(0.5)
	case c.activeWidget == id:
		bgColor = ColorButtonActive
	case hovered:
		bgColor = ColorButtonHover.WithAlpha(0.5)
	default:
		bgColor = ColorTransparent
	}
	if bgColor.A > 0 {
		c.renderer.DrawRect(x, y, w, h, bgColor)
	}

	scale := float32(1.0)
	ascent := c.renderer.FontAscent(scale)
	textY := y + (h-ascent)/2
	c.renderer.DrawText(x+6, textY, label, scale, ColorText)
	return clicked
}

// ProgressBarAt draws a horizontal progress bar at absolute coords.
// `fraction` is clamped to [0, 1]. `label` is rendered centered over the
// bar (typically the percentage). The bar uses the same recessed look as
// inputs so it visually reads as "this is filling up".
func (c *Context) ProgressBarAt(x, y, w, h float32, fraction float32, label string) {
	if fraction < 0 {
		fraction = 0
	}
	if fraction > 1 {
		fraction = 1
	}

	c.renderer.DrawRect(x, y, w, h, ColorInputBg)
	c.renderer.DrawRectOutline(x, y, w, h, 1, ColorPanelBorder)
	if fillW := (w - 2) * fraction; fillW > 0 {
		c.renderer.DrawRect(x+1, y+1, fillW, h-2, ColorHighlight)
	}

	if label != "" {
		// Smaller white caption — readable on the blue fill at high
		// progress without overwhelming the bar at small bar heights.
		scale := float32(0.75)
		textW, _ := c.renderer.MeasureText(label, scale)
		ascent := c.renderer.FontAscent(scale)
		textY := y + (h-ascent)/2 - 2
		c.renderer.DrawText(x+(w-textW)/2, textY, label, scale, ColorWhite)
	}
}

// LabelAt draws a text label at absolute coords using ColorText.
func (c *Context) LabelAt(x, y float32, text string) {
	c.LabelAtColored(x, y, text, ColorText)
}

// LabelAtColored draws a text label at absolute coords with a specific color.
func (c *Context) LabelAtColored(x, y float32, text string, color Color) {
	c.renderer.DrawText(x, y, text, 1.0, color)
}

// ImageAt blits a textured quad at absolute coords with the given tint.
func (c *Context) ImageAt(x, y, w, h float32, texID uint32, tint Color) {
	if texID == 0 {
		return
	}
	c.renderer.DrawImage(texID, x, y, w, h, tint)
}
