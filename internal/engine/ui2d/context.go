package ui2d

import "fmt"

// Context is the main UI context that manages rendering and input.
type Context struct {
	renderer *Renderer
	input    *InputState

	// Active/hot widget tracking for interaction
	hotWidget    string
	activeWidget string

	// Window state
	windows map[string]*WindowState

	// Current window being drawn
	currentWindow *WindowState

	// Current listbox being drawn (nil if not in a listbox)
	currentListBox *ListBoxState

	// Default window skin (nine-slice frame texture)
	defaultSkin *NineSlice
	// Default input skin (nine-slice; nil falls back to procedural sunken bevel)
	defaultInputSkin *NineSlice

	// Layout state
	cursorX float32
	cursorY float32
	rowH    float32
}

// WindowState holds state for a UI window.
//
// Dragged becomes true the first time the user moves the window; once set,
// the caller's x/y arguments to BeginWindow are treated as initial-only and
// ignored, so the new position survives across frames.
type WindowState struct {
	ID      string
	X, Y    float32
	W, H    float32
	Open    bool
	Moving  bool
	Dragged bool
	Skin    *NineSlice // Per-window skin override (nil uses default)
}

// NewContext creates a new UI context.
func NewContext(width, height int) (*Context, error) {
	r, err := New(width, height)
	if err != nil {
		return nil, fmt.Errorf("create renderer: %w", err)
	}

	return &Context{
		renderer: r,
		input:    &InputState{},
		windows:  make(map[string]*WindowState),
	}, nil
}

// Close releases resources.
func (c *Context) Close() {
	if c.renderer != nil {
		c.renderer.Close()
	}
}

// Renderer returns the underlying renderer.
func (c *Context) Renderer() *Renderer {
	return c.renderer
}

// Resize updates the screen size.
func (c *Context) Resize(width, height int) {
	c.renderer.Resize(width, height)
}

// Input returns the input state for modification.
func (c *Context) Input() *InputState {
	return c.input
}

// SetDefaultWindowSkin sets the default nine-slice skin for all windows.
func (c *Context) SetDefaultWindowSkin(skin *NineSlice) {
	c.defaultSkin = skin
}

// SetDefaultInputSkin sets a nine-slice skin used to draw text inputs.
// When nil, inputs fall back to drawSunkenInput's procedural bevel.
func (c *Context) SetDefaultInputSkin(skin *NineSlice) {
	c.defaultInputSkin = skin
}

// Begin starts a new UI frame.
func (c *Context) Begin() {
	c.input.Update()
	c.renderer.Begin()
}

// End finishes the UI frame.
func (c *Context) End() {
	c.renderer.End()
	c.input.EndFrame()
}

// BeginWindow starts a new window.
// Returns false if the window is closed.
func (c *Context) BeginWindow(id string, x, y, w, h float32, title string) bool {
	// Get or create window state
	ws, ok := c.windows[id]
	if !ok {
		ws = &WindowState{
			ID:   id,
			X:    x,
			Y:    y,
			W:    w,
			H:    h,
			Open: true,
		}
		c.windows[id] = ws
	} else {
		// Always update size from parameters. Position parameters are only an
		// initial hint: once the user drags the window we stop overwriting
		// X/Y so the new position survives drop. Without this the window
		// snaps back to the caller's center-of-screen each frame.
		ws.W = w
		ws.H = h
		if !ws.Moving && !ws.Dragged {
			ws.X = x
			ws.Y = y
		}
	}

	if !ws.Open {
		return false
	}

	c.currentWindow = ws

	// Handle window dragging (title bar is top 25 pixels)
	titleBarH := float32(25)
	titleBarRect := Rect{ws.X, ws.Y, ws.W, titleBarH}

	if c.input.MouseLeftPressed && titleBarRect.Contains(c.input.MouseX, c.input.MouseY) {
		ws.Moving = true
		c.activeWidget = id + "_titlebar"
	}

	if ws.Moving && c.input.MouseLeftDown {
		ws.X += c.input.MouseDeltaX
		ws.Y += c.input.MouseDeltaY
		ws.Dragged = true
	}

	if c.input.MouseLeftReleased {
		ws.Moving = false
		if c.activeWidget == id+"_titlebar" {
			c.activeWidget = ""
		}
	}

	// Draw window background
	skin := ws.Skin
	if skin == nil {
		skin = c.defaultSkin
	}
	if skin != nil {
		// The GRF skin's optional clean title-strip overlay (see
		// NineSlice.TitleStripSrcW) hides the BMP's baked-in "메세지"
		// title and close icon, leaving us a blank title bar to stamp
		// our own title text onto below.
		skin.Draw(c.renderer, ws.X, ws.Y, ws.W, ws.H, ColorWhite)
	} else {
		c.renderer.DrawPanel(ws.X, ws.Y, ws.W, ws.H, ColorPanelBg, ColorPanelBorder)
		// Skinless fallback: title bar uses the darker panel border color
		// since ColorButtonNormal is now near-white (would look wrong here).
		c.renderer.DrawRect(ws.X+1, ws.Y+1, ws.W-2, titleBarH-1, ColorPanelBorder)
	}

	// Draw the per-window title text centered in the title bar. Drawing
	// the same glyph at +1px X gives a fake-bold effect that thickens
	// strokes on the light blue title bar without needing a bold font
	// face (which our TTF pipeline doesn't load). Centering uses ascent
	// (cap-height) and biases up a few pixels so the title sits visually
	// in the upper portion of the bar — the gradient's accent is at the
	// top of the bar, and centering the line box looks low.
	if title != "" {
		scale := float32(0.85)
		barH := titleBarH
		if skin != nil && skin.Top > 0 {
			barH = float32(skin.Top)
		}
		textW, _ := c.renderer.MeasureText(title, scale)
		ascent := c.renderer.FontAscent(scale)
		textX := ws.X + (ws.W-textW)/2
		textY := ws.Y + (barH-ascent)/2 - 6
		c.renderer.DrawText(textX, textY, title, scale, ColorTitleText)
		c.renderer.DrawText(textX+1, textY, title, scale, ColorTitleText)
	}

	// Set cursor for content (below title bar, with padding)
	c.cursorX = ws.X + 8
	c.cursorY = ws.Y + titleBarH + 8
	c.rowH = 0

	return true
}

// EndWindow ends the current window.
func (c *Context) EndWindow() {
	c.currentWindow = nil
}

// CurrentWindowContentRect returns the rect inside the active window's
// chrome (under the title bar, padded). Returns the zero rect when no
// window is active. Tree-based screens use this as the root rect for
// RenderTree, so layouts compute the same body region the imperative
// cursor would have used.
func (c *Context) CurrentWindowContentRect() Rect {
	if c.currentWindow == nil {
		return Rect{}
	}
	ws := c.currentWindow
	pad := float32(8)
	titleBarH := float32(25)
	if c.defaultSkin != nil && c.defaultSkin.Top > 0 {
		titleBarH = float32(c.defaultSkin.Top)
	}
	return Rect{
		X: ws.X + pad,
		Y: ws.Y + titleBarH + pad,
		W: ws.W - pad*2,
		H: ws.H - titleBarH - pad*2,
	}
}

// Row starts a new row with the given height.
func (c *Context) Row(height float32) {
	if c.currentWindow == nil {
		return
	}
	c.cursorX = c.currentWindow.X + 8
	c.cursorY += c.rowH + 4
	c.rowH = height
}

// Button draws a button at the layout cursor and returns true if clicked.
// For absolute placement (the RO-style fixed-coordinate layout we use for
// skinned dialogs) call ButtonAt directly instead.
func (c *Context) Button(id string, width float32, label string) bool {
	if c.currentWindow == nil {
		return false
	}

	x := c.cursorX
	y := c.cursorY
	h := c.rowH
	if h == 0 {
		h = 28
	}
	if width == 0 {
		width = c.currentWindow.W - 16
	}

	fullID := c.currentWindow.ID + "_" + id
	clicked := c.ButtonAt(fullID, x, y, width, h, label)
	c.cursorX += width + 4
	return clicked
}

// Label draws a text label.
func (c *Context) Label(text string) {
	c.LabelColored(text, ColorText)
}

// LabelColored draws a text label with a specific color.
func (c *Context) LabelColored(text string, color Color) {
	if c.currentWindow == nil {
		return
	}

	// Draw text with scale 2.0 (16px font from 8px glyphs)
	scale := float32(1.0)
	c.renderer.DrawText(c.cursorX, c.cursorY, text, scale, color)

	// Advance cursor
	w, _ := c.renderer.MeasureText(text, scale)
	c.cursorX += w + 4
}

// TextInput draws a text input field.
// Returns (current value, changed, submitted).
func (c *Context) TextInput(id string, width float32, value string) (string, bool, bool) {
	if c.currentWindow == nil {
		return value, false, false
	}

	x := c.cursorX
	y := c.cursorY
	h := c.rowH
	if h == 0 {
		h = 28
	}
	if width == 0 {
		width = c.currentWindow.W - 16
	}

	fullID := c.currentWindow.ID + "_" + id
	rect := Rect{x, y, width, h}

	// Check interaction
	hovered := rect.Contains(c.input.MouseX, c.input.MouseY)
	focused := c.activeWidget == fullID
	changed := false
	submitted := false

	if hovered && c.input.MouseLeftPressed {
		c.activeWidget = fullID
	}

	// Handle text input when focused
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

	c.drawInput(x, y, width, h, focused)

	// Draw text value
	scale := float32(1.0)
	_, textH := c.renderer.MeasureText("Ag", scale) // representative height
	textY := y + (h-textH)/2
	c.renderer.DrawText(x+4, textY, value, scale, ColorText)

	// Draw cursor when focused
	if focused {
		textW, _ := c.renderer.MeasureText(value, scale)
		cursorX := x + 4 + textW
		c.renderer.DrawRect(cursorX, y+4, 2, h-8, ColorText)
	}

	// Advance cursor
	c.cursorX += width + 4

	return value, changed, submitted
}

// drawInput renders the input background. With a skin set, it 9-slices the
// skin texture (RO's `name-edit.bmp`) for an authentic look; without a skin,
// it falls back to the procedural sunken bevel below. Focus tints the border
// either by recoloring the skin (subtle blue tint) or by drawing the focus
// outline over the skin's neutral border.
func (c *Context) drawInput(x, y, width, h float32, focused bool) {
	if c.defaultInputSkin != nil {
		tint := ColorWhite
		if focused {
			// A light-blue tint on the skin pulls in the RO accent without
			// hiding the recessed look.
			tint = Color{R: 0.85, G: 0.9, B: 1.0, A: 1}
		}
		c.defaultInputSkin.Draw(c.renderer, x, y, width, h, tint)
		if focused {
			c.renderer.DrawRectOutline(x, y, width, h, 1, ColorInputBorderFocus)
		}
		return
	}
	drawSunkenInput(c.renderer, x, y, width, h, focused)
}

// drawSunkenInput renders a text-input field as a recessed (sunken) box on
// the white BMP body: white fill plus a 1-pixel inverse bevel — dark on
// top/left, light on bottom/right — so it reads as inset rather than raised.
// Focused fields tint the border blue (RO accent).
func drawSunkenInput(r *Renderer, x, y, width, h float32, focused bool) {
	r.DrawRect(x, y, width, h, ColorInputBg)
	border := ColorInputBorder
	if focused {
		border = ColorInputBorderFocus
	}
	// Inverse bevel: shadow on top/left, highlight on bottom/right.
	r.DrawRect(x+1, y, width-2, 1, border)                 // top edge (shadow)
	r.DrawRect(x, y+1, 1, h-2, border)                     // left edge
	r.DrawRect(x+1, y+h-1, width-2, 1, ColorButtonBevelHi) // bottom edge (highlight)
	r.DrawRect(x+width-1, y+1, 1, h-2, ColorButtonBevelHi) // right edge
}

// Spacer adds vertical space.
func (c *Context) Spacer(height float32) {
	c.cursorY += height
}

// Separator draws a horizontal separator line.
func (c *Context) Separator() {
	if c.currentWindow == nil {
		return
	}
	c.cursorY += c.rowH + 4
	c.rowH = 0
	x := c.currentWindow.X + 8
	w := c.currentWindow.W - 16
	c.renderer.DrawRect(x, c.cursorY, w, 1, ColorPanelBorder)
	c.cursorY += 8
	c.cursorX = x
}

// SameLine keeps the cursor on the same line (for horizontal layouts).
func (c *Context) SameLine() {
	// Don't advance Y; cursorX is already updated by previous widget
}

// ProgressBar draws a progress bar.
func (c *Context) ProgressBar(fraction float32, width, height float32, label string) {
	if c.currentWindow == nil {
		return
	}

	x := c.cursorX
	y := c.cursorY
	if height == 0 {
		height = 20
	}
	if width == 0 {
		width = c.currentWindow.W - 16
	}

	// Clamp fraction
	if fraction < 0 {
		fraction = 0
	}
	if fraction > 1 {
		fraction = 1
	}

	// Background
	c.renderer.DrawRect(x, y, width, height, ColorInputBg)
	c.renderer.DrawRectOutline(x, y, width, height, 1, ColorPanelBorder)

	// Progress fill
	fillWidth := (width - 2) * fraction
	if fillWidth > 0 {
		c.renderer.DrawRect(x+1, y+1, fillWidth, height-2, ColorHighlight)
	}

	// Label (centered)
	if label != "" {
		scale := float32(1.0)
		textW, textH := c.renderer.MeasureText(label, scale)
		textX := x + (width-textW)/2
		textY := y + (height-textH)/2
		c.renderer.DrawText(textX, textY, label, scale, ColorText)
	}

	// Advance cursor
	c.cursorX = c.currentWindow.X + 8
	c.cursorY += height + 4
}

// PasswordInput draws a password input field with masked characters.
// Returns (current value, changed, submitted).
func (c *Context) PasswordInput(id string, width float32, value string) (string, bool, bool) {
	if c.currentWindow == nil {
		return value, false, false
	}

	x := c.cursorX
	y := c.cursorY
	h := c.rowH
	if h == 0 {
		h = 28
	}
	if width == 0 {
		width = c.currentWindow.W - 16
	}

	fullID := c.currentWindow.ID + "_" + id
	rect := Rect{x, y, width, h}

	// Check interaction
	hovered := rect.Contains(c.input.MouseX, c.input.MouseY)
	focused := c.activeWidget == fullID
	changed := false
	submitted := false

	if hovered && c.input.MouseLeftPressed {
		c.activeWidget = fullID
	}

	// Handle text input when focused
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

	// Draw input field
	c.drawInput(x, y, width, h, focused)

	// Draw masked text (dots instead of characters)
	scale := float32(1.0)
	maskedText := ""
	for range value {
		maskedText += "*"
	}
	_, textH := c.renderer.MeasureText("Ag", scale)
	textY := y + (h-textH)/2
	c.renderer.DrawText(x+4, textY, maskedText, scale, ColorText)

	// Draw cursor when focused
	if focused {
		textW, _ := c.renderer.MeasureText(maskedText, scale)
		cursorX := x + 4 + textW
		c.renderer.DrawRect(cursorX, y+4, 2, h-8, ColorText)
	}

	// Advance cursor
	c.cursorX += width + 4

	return value, changed, submitted
}

// Selectable draws a selectable item and returns true if clicked.
func (c *Context) Selectable(id string, label string, selected bool) bool {
	if c.currentWindow == nil {
		return false
	}

	x := c.cursorX
	y := c.cursorY
	h := c.rowH
	if h == 0 {
		h = 24
	}

	// Use listbox width if inside a listbox, otherwise window width
	var width float32
	if c.currentListBox != nil {
		width = c.currentListBox.W - 8 // Account for padding
	} else {
		width = c.currentWindow.W - 16
	}

	fullID := c.currentWindow.ID + "_" + id
	rect := Rect{x, y, width, h}

	// Check interaction - click on press for better responsiveness
	hovered := rect.Contains(c.input.MouseX, c.input.MouseY)
	clicked := false

	if hovered {
		c.hotWidget = fullID
		if c.input.MouseLeftPressed {
			c.activeWidget = fullID
			clicked = true // Click immediately on press
		}
	}

	// Clear active state on release
	if c.activeWidget == fullID && c.input.MouseLeftReleased {
		c.activeWidget = ""
	}

	// Draw background
	var bgColor Color
	if selected {
		bgColor = ColorHighlight.WithAlpha(0.5)
	} else if c.activeWidget == fullID {
		bgColor = ColorButtonActive
	} else if hovered {
		bgColor = ColorButtonHover
	} else {
		bgColor = ColorTransparent
	}

	if bgColor.A > 0 {
		c.renderer.DrawRect(x, y, width, h, bgColor)
	}

	// Draw label
	scale := float32(1.0)
	_, textH := c.renderer.MeasureText(label, scale)
	textY := y + (h-textH)/2
	c.renderer.DrawText(x+4, textY, label, scale, ColorText)

	// Advance cursor to next row
	if c.currentListBox != nil {
		c.cursorX = c.currentListBox.X + 4
	} else {
		c.cursorX = c.currentWindow.X + 8
	}
	c.cursorY += h

	return clicked
}

// ListBoxState holds state for a list box widget.
type ListBoxState struct {
	ScrollY float32
	X, Y    float32
	W, H    float32
	Active  bool
}

// BeginListBox starts a list box region.
func (c *Context) BeginListBox(id string, width, height float32) {
	if c.currentWindow == nil {
		return
	}

	// Start on a new row (reset X to window left edge)
	x := c.currentWindow.X + 8
	y := c.cursorY

	if width == 0 {
		width = c.currentWindow.W - 16
	}
	if height == 0 {
		height = 200
	}

	// Draw list box background
	c.renderer.DrawRect(x, y, width, height, ColorInputBg)
	c.renderer.DrawRectOutline(x, y, width, height, 1, ColorPanelBorder)

	// Store listbox bounds
	c.currentListBox = &ListBoxState{
		X:      x,
		Y:      y,
		W:      width,
		H:      height,
		Active: true,
	}

	// Position cursor inside listbox
	c.cursorX = x + 4
	c.cursorY = y + 4
	c.rowH = 24
}

// EndListBox ends a list box region.
func (c *Context) EndListBox() {
	if c.currentWindow == nil {
		return
	}
	// Position cursor after the listbox
	if c.currentListBox != nil {
		c.cursorX = c.currentWindow.X + 8
		c.cursorY = c.currentListBox.Y + c.currentListBox.H + 4
		c.currentListBox = nil
	}
}

// ButtonDisabled draws a disabled button (no interaction).
func (c *Context) ButtonDisabled(id string, width float32, label string) {
	if c.currentWindow == nil {
		return
	}

	x := c.cursorX
	y := c.cursorY
	h := c.rowH
	if h == 0 {
		h = 28
	}
	if width == 0 {
		width = c.currentWindow.W - 16
	}

	// Draw button in disabled state
	c.renderer.DrawRect(x, y, width, h, ColorButtonNormal.Darken(0.3))
	c.renderer.DrawRectOutline(x, y, width, h, 1, ColorButtonBorder.Darken(0.3))

	// Draw button label centered (dimmed)
	scale := float32(1.0)
	textW, textH := c.renderer.MeasureText(label, scale)
	textX := x + (width-textW)/2
	textY := y + (h-textH)/2
	c.renderer.DrawText(textX, textY, label, scale, ColorTextDim)

	// Advance cursor
	c.cursorX += width + 4
}

// Checkbox draws a checkbox.
func (c *Context) Checkbox(id string, label string, checked bool) bool {
	if c.currentWindow == nil {
		return checked
	}

	x := c.cursorX
	y := c.cursorY
	boxSize := float32(18)

	fullID := c.currentWindow.ID + "_" + id
	rect := Rect{x, y, boxSize, boxSize}

	// Check interaction
	hovered := rect.Contains(c.input.MouseX, c.input.MouseY)

	if hovered && c.input.MouseLeftPressed {
		c.activeWidget = fullID
	}

	if c.activeWidget == fullID && c.input.MouseLeftReleased {
		if hovered {
			checked = !checked
		}
		c.activeWidget = ""
	}

	// Draw checkbox box
	bgColor := ColorInputBg
	if hovered {
		bgColor = ColorButtonHover
	}
	c.renderer.DrawRect(x, y, boxSize, boxSize, bgColor)
	c.renderer.DrawRectOutline(x, y, boxSize, boxSize, 1, ColorPanelBorder)

	// Draw check mark if checked
	if checked {
		// Draw a simple check (filled inner square)
		innerMargin := float32(4)
		c.renderer.DrawRect(
			x+innerMargin, y+innerMargin,
			boxSize-innerMargin*2, boxSize-innerMargin*2,
			ColorHighlight,
		)
	}

	// Draw label
	scale := float32(1.0)
	_, textH := c.renderer.MeasureText(label, scale)
	textY := y + (boxSize-textH)/2
	c.renderer.DrawText(x+boxSize+8, textY, label, scale, ColorText)

	// Advance cursor
	labelW, _ := c.renderer.MeasureText(label, scale)
	c.cursorX += boxSize + 8 + labelW + 8

	return checked
}

// LabelCentered draws centered text and advances the cursor down by the
// text's measured height. Unlike Label, which only advances X (so callers
// can chain inline labels in a Row), LabelCentered owns the whole row and
// must move Y so the next Spacer/Separator/Row doesn't draw on top of it.
func (c *Context) LabelCentered(text string) {
	if c.currentWindow == nil {
		return
	}

	scale := float32(1.0)
	textW, textH := c.renderer.MeasureText(text, scale)
	windowContentWidth := c.currentWindow.W - 16
	x := c.currentWindow.X + 8 + (windowContentWidth-textW)/2
	if x < c.currentWindow.X+8 {
		x = c.currentWindow.X + 8
	}

	c.renderer.DrawText(x, c.cursorY, text, scale, ColorText)
	c.cursorY += textH
}

// GetScreenSize returns the current screen dimensions.
func (c *Context) GetScreenSize() (float32, float32) {
	w, h := c.renderer.GetScreenSize()
	return float32(w), float32(h)
}

// Rect is a simple rectangle struct.
type Rect struct {
	X, Y, W, H float32
}

// Contains checks if a point is inside the rectangle.
func (r Rect) Contains(x, y float32) bool {
	return x >= r.X && x < r.X+r.W && y >= r.Y && y < r.Y+r.H
}
