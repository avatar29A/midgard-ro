package ui2d

// defaultInputTextScale keeps body text comfortably inside a 22-28px field.
const defaultInputTextScale = float32(0.85)

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

			c.playClickSound()
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
		c.renderer.DrawRect(x+r, y, w-2*r, 1, Color{R: 1, G: 1, B: 1, A: 0.85})     // top shine
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

// ButtonOptions customizes how a button behaves. The zero value is the
// ordinary button: it clicks and it makes a noise doing so.
type ButtonOptions struct {
	// Silent suppresses the click sound. Not every control should announce
	// itself — chrome that folds a panel away, or a strip of buttons that
	// only open other windows, is part of arranging the interface rather
	// than an action taken in the game.
	Silent bool
}

// ImageButtonAt is the RO-style 3-state textured button. Pass texture IDs for
// the normal / hover / pressed states; zero-IDs fall back to the normal
// texture. Returns true on click. The texture is drawn 1:1 over the
// (x, y, w, h) rect with white tint.
func (c *Context) ImageButtonAt(id string, x, y, w, h float32, normalTex, overTex, pressedTex uint32) bool {
	clicked, _ := c.ImageButtonAtEx(id, x, y, w, h, normalTex, overTex, pressedTex)

	return clicked
}

// ImageButtonAtOpts is ImageButtonAt with its behavior spelled out.
func (c *Context) ImageButtonAtOpts(id string, x, y, w, h float32, normalTex, overTex, pressedTex uint32, opts ButtonOptions) bool {
	clicked, _ := c.ImageButtonAtExOpts(id, x, y, w, h, normalTex, overTex, pressedTex, opts)

	return clicked
}

// ImageButtonAtEx is ImageButtonAt that also reports the texture it drew.
// Callers that need to paint over artwork baked into the button — a caption in
// the wrong language — need to know which of the three states is on screen.
func (c *Context) ImageButtonAtEx(id string, x, y, w, h float32, normalTex, overTex, pressedTex uint32) (bool, uint32) {
	return c.ImageButtonAtExOpts(id, x, y, w, h, normalTex, overTex, pressedTex, ButtonOptions{})
}

// ImageButtonAtExOpts is ImageButtonAtEx with its behavior spelled out.
func (c *Context) ImageButtonAtExOpts(id string, x, y, w, h float32, normalTex, overTex, pressedTex uint32, opts ButtonOptions) (bool, uint32) {
	rect := Rect{x, y, w, h}
	hovered := rect.Contains(c.input.MouseX, c.input.MouseY)
	clicked := false

	if hovered {
		c.hotWidget = id
		if c.input.MouseLeftPressed || c.input.MouseLeftClicked {
			c.activeWidget = id
			clicked = true
			c.input.MouseLeftClicked = false

			if !opts.Silent {
				c.playClickSound()
			}
		}
	}
	if c.activeWidget == id && c.input.MouseLeftReleased {
		c.activeWidget = ""
	}

	// Not every button in the archive ships hover and pressed art —
	// btn_connect does, btn_ok does not. Where it is missing the state is
	// shaded instead, so every button reacts the same way.
	tex := normalTex
	overlay := Color{}

	switch {
	case c.activeWidget == id:
		if pressedTex != 0 && pressedTex != normalTex {
			tex = pressedTex
		} else {
			overlay = ColorSkinPressed
		}
	case hovered:
		if overTex != 0 && overTex != normalTex {
			tex = overTex
		} else {
			overlay = ColorSkinHover
		}
	}

	if tex != 0 {
		c.renderer.DrawImage(tex, x, y, w, h, ColorWhite)
	}

	// Solids draw over images and under text, so this shades the button
	// without touching its caption.
	if overlay.A > 0 {
		c.renderer.DrawRect(x, y, w, h, overlay)
	}

	return clicked, tex
}

// TextInputAt draws an editable text field at absolute coords.
// Returns (current value, changed, submitted-on-enter).
func (c *Context) TextInputAt(id string, x, y, w, h float32, value string) (string, bool, bool) {
	return c.textFieldAt(id, x, y, w, h, value, false, true, defaultInputTextScale)
}

// PasswordInputAt draws a masked text field at absolute coords.
func (c *Context) PasswordInputAt(id string, x, y, w, h float32, value string) (string, bool, bool) {
	return c.textFieldAt(id, x, y, w, h, value, true, true, defaultInputTextScale)
}

// TextInputBareAt is TextInputAt without the drawn field chrome, for inputs
// whose background is part of a skin texture — the original RO windows bake
// their input wells into the window art.
func (c *Context) TextInputBareAt(id string, x, y, w, h, scale float32, value string) (string, bool, bool) {
	return c.textFieldAt(id, x, y, w, h, value, false, false, scale)
}

// PasswordInputBareAt is PasswordInputAt without the drawn field chrome.
func (c *Context) PasswordInputBareAt(id string, x, y, w, h, scale float32, value string) (string, bool, bool) {
	return c.textFieldAt(id, x, y, w, h, value, true, false, scale)
}

// textFieldAt is the shared implementation for TextInputAt / PasswordInputAt.
func (c *Context) textFieldAt(id string, x, y, w, h float32, value string, masked, chrome bool, scale float32) (string, bool, bool) {
	rect := Rect{x, y, w, h}
	hovered := rect.Contains(c.input.MouseX, c.input.MouseY)
	changed := false
	submitted := false

	// Claim a pending Tab before this call can hand one on, so a lone field
	// does not tab to itself and a field never steals the focus it just gave
	// away. Fields drawn in call order are the tab order; a Tab from the last
	// one survives into the next frame and wraps onto the first.
	if c.focusNext {
		c.activeWidget = id
		c.focusNext = false
		c.selectAll = ""
	}
	focused := c.activeWidget == id

	if c.input.MouseLeftPressed {
		switch {
		case hovered:
			c.activeWidget = id
			focused = true
			// A double click selects the whole value, which is the only way
			// to clear a field in one gesture without a caret to drag.
			if c.DoubleClickedIn(id, rect) {
				c.selectAll = id
			} else {
				c.selectAll = ""
			}
		case focused:
			c.activeWidget = ""
			focused = false
			c.selectAll = ""
		}
	}

	selected := focused && c.selectAll == id && value != ""

	if focused {
		if len(c.input.TextInput) > 0 {
			if selected {
				value = ""
				selected = false
				c.selectAll = ""
			}
			value += c.input.TextInput
			changed = true
		}
		if c.input.KeyBackspacePressed || c.input.KeyDeletePressed {
			switch {
			case selected:
				value = ""
				selected = false
				c.selectAll = ""
				changed = true
			case c.input.KeyBackspacePressed && value != "":
				value = trimLastRune(value)
				changed = true
			}
		}
		if c.input.KeyEnterPressed {
			submitted = true
		}
		if c.input.KeyEscapePressed {
			c.activeWidget = ""
			c.selectAll = ""
		}
		if c.input.KeyTabPressed {
			c.activeWidget = ""
			c.selectAll = ""
			c.focusNext = true
			focused = false
			selected = false
		}
	}

	if chrome {
		c.drawInput(x, y, w, h, focused)
	}

	displayed := value
	if masked {
		displayed = maskRunes(value)
	}

	// There is no scissor in the renderer, so overflow is handled by drawing
	// only the tail that fits: the caret sits at the end of the value, and
	// keeping it visible is what matters while typing.
	const pad = 4
	inner := w - 2*pad
	displayed = fitTextTail(c.renderer, displayed, scale, inner)

	// Body text scaled down so glyphs sit comfortably inside the field —
	// at scale=1.0 cap-height was visually too tall for a 22-28px input.
	// Center on cap-height (ascent) for optical centering rather than the
	// line-height that includes leading + descender padding.
	ascent := c.renderer.FontAscent(scale)
	textY := y + (h-ascent)/2
	textW, _ := c.renderer.MeasureText(displayed, scale)

	// The caret and the selection follow the text, not the field: sized to
	// the field they stay full height when the text is scaled down, which
	// leaves a caret taller than the characters it sits among.
	caretH := ascent + 2
	caretY := y + (h-caretH)/2

	if selected {
		c.renderer.DrawRect(x+pad, caretY, textW, caretH, ColorSelection)
	}
	c.renderer.DrawText(x+pad, textY, displayed, scale, ColorText)

	if focused && !selected {
		c.renderer.DrawRect(x+pad+textW, caretY, 2, caretH, ColorText)
	}

	return value, changed, submitted
}

// fitTextTail returns the longest suffix of s that measures within maxW, so a
// value longer than its field scrolls instead of spilling past the edge.
func fitTextTail(r *Renderer, s string, scale, maxW float32) string {
	if s == "" || maxW <= 0 {
		return s
	}
	if w, _ := r.MeasureText(s, scale); w <= maxW {
		return s
	}
	runes := []rune(s)
	for i := 1; i < len(runes); i++ {
		tail := string(runes[i:])
		if w, _ := r.MeasureText(tail, scale); w <= maxW {
			return tail
		}
	}
	return ""
}

// trimLastRune drops the final rune, not the final byte: names arrive as UTF-8
// and a byte-wise backspace would leave a broken sequence behind.
func trimLastRune(s string) string {
	runes := []rune(s)
	if len(runes) == 0 {
		return s
	}
	return string(runes[:len(runes)-1])
}

func maskRunes(s string) string {
	masked := make([]rune, 0, len(s))
	for range s {
		masked = append(masked, '*')
	}
	return string(masked)
}

// InvisibleButtonAt is a hit area with no drawing of its own, for regions whose
// appearance comes from a skin texture — a character slot painted into the
// window it sits in.
func (c *Context) InvisibleButtonAt(id string, x, y, w, h float32) bool {
	rect := Rect{x, y, w, h}
	if !rect.Contains(c.input.MouseX, c.input.MouseY) {
		return false
	}

	c.hotWidget = id

	if !c.input.MouseLeftPressed && !c.input.MouseLeftClicked {
		return false
	}

	c.activeWidget = id
	c.input.MouseLeftClicked = false

	c.playClickSound()

	return true
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

// WindowRect is where a window actually is.
//
// BeginWindow takes a position, but only as an opening hint: once the window
// has been dragged it keeps its own, and the caller's is ignored. Anything
// drawn inside at the position it passed in therefore stays behind when the
// frame is moved, which is what this is for.
func (c *Context) WindowRect(id string) (Rect, bool) {
	ws, ok := c.windows[id]
	if !ok {
		return Rect{}, false
	}

	return Rect{X: ws.X, Y: ws.Y, W: ws.W, H: ws.H}, true
}

// OpenWindow reopens a window that was closed or minimized from its own
// title bar.
//
// Both set a flag on the window's remembered state, and BeginWindow returns
// false while either is set — for good, since the state outlives the window.
// Anything that offers a way to open a window again has to clear them, or the
// window opens once per session and never more.
func (c *Context) OpenWindow(id string) {
	if ws, ok := c.windows[id]; ok {
		ws.Open = true
		ws.Minimized = false
	}
}

// WindowClosed reports whether a window has been closed from its own X.
//
// BeginWindow returns false for a minimized window as well as a closed one,
// and the two mean opposite things: the minimized one is still on screen and
// still the caller's to draw next frame. So the question to ask of a false
// return is this one and not "is it minimized" — minimizing and then closing
// leaves both flags set, and a caller watching the minimized flag would never
// notice the close.
func (c *Context) WindowClosed(id string) bool {
	ws, ok := c.windows[id]

	return ok && !ws.Open
}

// FillEllipseImageLayer fills an ellipse in the image pass, so it sits behind
// images drawn after it rather than over them.
//
// Built from horizontal rows whose width follows the ellipse's chord, the way
// the slider's knob is: the renderer draws rectangles and nothing rounder.
func (c *Context) FillEllipseImageLayer(x, y, w, h float32, color Color) {
	rx, ry := w/2, h/2
	cx, cy := x+rx, y+ry

	for dy := -ry; dy <= ry; dy++ {
		// The chord at this height, as a fraction of the full width.
		frac := 1 - (dy*dy)/(ry*ry)
		if frac <= 0 {
			continue
		}

		half := rx * sqrt32(frac)
		if half < 0.5 {
			continue
		}

		c.renderer.FillImageLayer(cx-half, cy+dy, half*2, 1, color)
	}
}

// CheckboxAt is Checkbox at a position of the caller's choosing, for a dialog
// laid out to match the original rather than by the cursor.
func (c *Context) CheckboxAt(id string, x, y, size float32, label string, checked bool) bool {
	rect := Rect{x, y, size, size}
	hovered := rect.Contains(c.input.MouseX, c.input.MouseY)

	if hovered && c.input.MouseLeftPressed {
		c.activeWidget = id
	}

	if c.activeWidget == id && c.input.MouseLeftReleased {
		if hovered {
			checked = !checked
		}
		c.activeWidget = ""
	}

	// A white box in a thin dark border, which is what the original's is: the
	// panel-colored fill and heavy border read as a button, not a checkbox.
	bg := ColorCheckFace
	if hovered {
		bg = ColorCheckFaceHot
	}
	c.renderer.DrawRect(x, y, size, size, bg)
	c.renderer.DrawRectOutline(x, y, size, size, 1, ColorCheckBorder)

	if checked {
		const inset float32 = 3
		c.renderer.DrawRect(x+inset, y+inset, size-inset*2, size-inset*2, ColorCheckMark)
	}

	if label != "" {
		_, capH := c.renderer.MeasureText(label, 1)
		c.renderer.DrawText(x+size+6, y+(size-capH)/2, label, 1, ColorText)
	}

	return checked
}

// SliderAt is a horizontal slider between 0 and 1, with the arrow caps the
// original draws at each end.
//
// The value follows the pointer while the knob is held rather than moving by
// how far the pointer traveled: a drag that leaves the track and comes back
// then picks up where the pointer is, instead of somewhere behind it.
func (c *Context) SliderAt(id string, x, y, w, h, value float32) (float32, bool) {
	const (
		cap   float32 = 9
		knobW float32 = 9
	)

	trackX := x + cap
	trackW := w - 2*cap

	// The knob's center travels the track inset by half its own width, so it
	// stops flush with each end rather than hanging over it.
	span := trackW - knobW
	if span < 1 {
		span = 1
	}

	rect := Rect{x, y, w, h}
	if c.input.MouseLeftPressed && rect.Contains(c.input.MouseX, c.input.MouseY) {
		c.activeWidget = id
	}

	changed := false
	if c.activeWidget == id {
		if c.input.MouseLeftDown {
			want := (c.input.MouseX - trackX - knobW/2) / span
			want = clamp01(want)

			if want != value {
				value = want
				changed = true
			}
		} else {
			c.activeWidget = ""
		}
	}

	value = clamp01(value)

	// A pale track in a thin border, an outlined arrow at each end, and a
	// round knob — the original's is a Windows trackbar, not a painted RO
	// widget, and a solid blue bar is not what it looks like.
	trackY := y + h/2 - 5
	c.renderer.DrawRect(trackX, trackY, trackW, 10, ColorTrackFace)
	c.renderer.DrawRectOutline(trackX, trackY, trackW, 10, 1, ColorTrackBorder)

	c.drawSliderCap(x, y+h/2, cap, true)
	c.drawSliderCap(x+w-cap, y+h/2, cap, false)

	c.drawSliderKnob(trackX+value*span+knobW/2, y+h/2, knobW/2)

	return value, changed
}

// drawSliderCap draws one of the triangular ends, filling [x, x+size] and
// pointing away from the track.
func (c *Context) drawSliderCap(x, midY, size float32, left bool) {
	// Stepped columns rather than a real triangle: the renderer draws
	// rectangles, and at this size the steps read as the arrow the original
	// has. Each column is tallest at the track side and a pixel at the point.
	//
	// Outlined rather than solid: a pale face with an edge, so it matches the
	// track it sits against instead of being a block of color beside it.
	steps := int(size)
	for i := 0; i < steps; i++ {
		half := float32(i+1) / float32(steps) * size / 2
		if half < 1 {
			half = 1
		}

		// i counts from the point outward, so it maps to the far column on a
		// left-pointing cap and the near one on a right-pointing cap.
		col := x + size - float32(i) - 1
		if left {
			col = x + float32(i)
		}

		c.renderer.DrawRect(col, midY-half, 1, half*2, ColorTrackFace)
		c.renderer.DrawRect(col, midY-half, 1, 1, ColorSliderEdge)
		c.renderer.DrawRect(col, midY+half-1, 1, 1, ColorSliderEdge)

		// The point itself, so the tip is edged rather than open.
		if i == 0 {
			c.renderer.DrawRect(col, midY-half, 1, half*2, ColorSliderEdge)
		}
	}
}

// drawSliderKnob draws the round grip. The renderer has no circle, so it is
// built from rows whose width follows the chord of one.
func (c *Context) drawSliderKnob(cx, cy, radius float32) {
	for dy := -radius; dy <= radius; dy++ {
		halfW := sqrt32(radius*radius - dy*dy)
		if halfW < 0.5 {
			continue
		}

		c.renderer.DrawRect(cx-halfW, cy+dy, halfW*2, 1, ColorKnobFace)
	}
}

// sqrt32 is a Newton step or two, which is plenty for a knob a few pixels
// across and avoids pulling math in for one call.
func sqrt32(v float32) float32 {
	if v <= 0 {
		return 0
	}

	guess := v
	for i := 0; i < 8; i++ {
		guess = 0.5 * (guess + v/guess)
	}

	return guess
}

func clamp01(v float32) float32 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}

	return v
}
